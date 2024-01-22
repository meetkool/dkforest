package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/base64"
	"flag"
	"fmt"
	"math/big"
	"os"
)

func main() {
	var privKeyFile string
	var fileName string
	flag.StringVar(&privKeyFile, "s", "hs_ed25519_secret_key", "tor private key file")
	flag.StringVar(&fileName, "c", "certificate.txt", "certificate to sign")
	flag.Parse()
	msg, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	pemKeyBytes, _ := os.ReadFile(privKeyFile)
	identityPrivKey := loadTorKeyFromDisk(pemKeyBytes)
	signature := sign(identityPrivKey, msg)
	fmt.Println(base64.StdEncoding.EncodeToString(signature))
}

func loadTorKeyFromDisk(keyBytes []byte) ed25519.PrivateKey {
	if !bytes.Equal(keyBytes[:29], []byte("== ed25519v1-secret: type0 ==")) {
		panic("Tor key does not start with Tor header")
	}
	expandedSk := keyBytes[32:]
	if len(expandedSk) != 64 {
		panic("Tor private key has the wrong length")
	}
	return expandedSk
}

func sign(identityPrivKey, msg []byte) []byte {
	return signatureWithESK(msg, identityPrivKey, publickeyFromESK(identityPrivKey))
}

var (
	b  = 256
	l  = biAdd(biExp(bi(2), bi(252)), biFromStr("27742317777372353535851937790883648493"))
	by = biMul(bi(4), inv(bi(5)))
	bx = xrecover(by)
	q  = biSub(biExp(bi(2), bi(255)), bi(19))
	bB = []*big.Int{biMod(bx, q), biMod(by, q)}
	I  = expmod(bi(2), biDiv(biSub(q, bi(1)), bi(4)), q)
	d  = bi(0).Mul(bi(-121665), inv(bi(121666)))
)

func publickeyFromESK(h []byte) ed25519.PublicKey {
	a := decodeInt(h[:32])
	A := scalarmult(bB, a)
	return encodepoint(A)
}

func signatureWithESK(msg, blindedEsk, blindedKey []byte) []byte {
	a := decodeInt(blindedEsk[:32])
	lines := make([][]byte, 0)
	for i := b / 8; i < b/4; i++ {
		lines = append(lines, blindedEsk[i:i+1])
	}
	toHint := append(bytes.Join(lines, []byte("")), msg...)
	r := hint(toHint)
	R := scalarmult(bB, r)
	S := biMod(biAdd(r, biMul(hint([]byte(string(encodepoint(R))+string(blindedKey)+string(msg))), a)), l)
	return append(encodepoint(R), encodeint(S)...)
}

func edwards(P, Q []*big.Int) []*big.Int {
	x1, y1 := P[0], P[1]
	x2, y2 := Q[0], Q[1]
	x3 := biMul(biAdd(biMul(x1, y2), biMul(x2, y1)), inv(biAdd(bi(1), biMul(biMul(biMul(biMul(d, x1), x2), y1), y2))))
	y3 := biMul(biAdd(biMul(y1, y2), biMul(x1, x2)), inv(biSub(bi(1), biMul(biMul(biMul(biMul(d, x1), x2), y1), y2))))
	return []*big.Int{biMod(x3, q), biMod(y3, q)}
}

func scalarmult(P []*big.Int, e *big.Int) []*big.Int {
	if e.Cmp(bi(0)) == 0 {
		return []*big.Int{bi(0), bi(1)}
	}
	Q := scalarmult(P, biDiv(e, bi(2)))
	Q = edwards(Q, Q)
	if biAnd(e, bi(1)).Int64() == 1 {
		Q = edwards(Q, P)
	}
	return Q
}

func encodepoint(P []*big.Int) []byte {
	x, y := P[0], P[1]
	bits := make([]uint8, 0)
	for i := 0; i < b-1; i++ {
		bits = append(bits, uint8(biAnd(biRsh(y, uint(i)), bi(1)).Int64()))
	}
	bits = append(bits, uint8(biAnd(x, bi(1)).Int64()))
	by := make([]uint8, 0)
	for i := 0; i < b/8; i++ {
		sum := uint8(0)
		for j := 0; j < 8; j++ {
			sum += bits[i*8+j] << j
		}
		by = append(by, sum)
	}
	return by
}

func hint(m []byte) *big.Int {
	shaSum := sha512.Sum512(m)
	h := shaSum[:]
	sum := bi(0)
	for i := 0; i < 2*b; i++ {
		sum = biAdd(sum, biMul(biExp(bi(2), bi(int64(i))), bi(int64(Bit(h, int64(i))))))
	}
	return sum
}

func encodeint(y *big.Int) []byte {
	bits := make([]*big.Int, 0)
	for i := 0; i < b; i++ {
		bits = append(bits, biAnd(biRsh(y, uint(i)), bi(1)))
	}
	final := make([]byte, 0)
	for i := 0; i < b/8; i++ {
		sum := bi(0)
		for j := 0; j < 8; j++ {
			sum = biAdd(sum, biLsh(bits[i*8+j], uint(j)))
		}
		final = append(final, byte(sum.Uint64()))
	}
	return final
}

func decodeInt(s []uint8) *big.Int {
	sum := bi(0)
	for i := 0; i < b; i++ {
		e := biExp(bi(2), bi(int64(i)))
		m := bi(int64(Bit(s, int64(i))))
		sum = sum.Add(sum, biMul(e, m))
	}
	return sum
}

func xrecover(y *big.Int) *big.Int {
	xx := biMul(biSub(biMul(y, y), bi(1)), inv(biAdd(biMul(biMul(d, y), y), bi(1))))
	x := expmod(xx, biDiv(biAdd(q, bi(3)), bi(8)), q)
	if biMod(biSub(biMul(x, x), xx), q).Int64() != 0 {
		x = biMod(biMul(x, I), q)
	}
	if biMod(x, bi(2)).Int64() != 0 {
		x = biSub(q, x)
	}
	return x
}

func expmod(b, e, m *big.Int) *big.Int {
	if e.Cmp(bi(0)) == 0 {
		return bi(1)
	}
	t := biMod(biExp(expmod(b, biDiv(e, bi(2)), m), bi(2)), m)
	if biAnd(e, bi(1)).Int64() == 1 {
		t = biMod(biMul(t, b), m)
	}
	return t
}

func biFromStr(v string) (out *big.Int) {
	out = new(big.Int)
	_, _ = fmt.Sscan(v, out)
	return
}

func inv(x *big.Int) *big.Int           { return expmod(x, biSub(q, bi(2)), q) }
func Bit(h []uint8, i int64) uint8      { return (h[i/8] >> (i % 8)) & 1 }
func bi(v int64) *big.Int               { return big.NewInt(v) }
func biAdd(a, b *big.Int) *big.Int      { return bi(0).Add(a, b) }
func biSub(a, b *big.Int) *big.Int      { return bi(0).Sub(a, b) }
func biMul(a, b *big.Int) *big.Int      { return bi(0).Mul(a, b) }
func biDiv(a, b *big.Int) *big.Int      { return bi(0).Div(a, b) }
func biAnd(a, b *big.Int) *big.Int      { return bi(0).And(a, b) }
func biMod(a, b *big.Int) *big.Int      { return bi(0).Mod(a, b) }
func biExp(a, b *big.Int) *big.Int      { return bi(0).Exp(a, b, nil) }
func biLsh(a *big.Int, b uint) *big.Int { return bi(0).Lsh(a, b) }
func biRsh(a *big.Int, b uint) *big.Int { return bi(0).Rsh(a, b) }
