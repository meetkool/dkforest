package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/md5"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"dkforest/pkg/hashset"
	"dkforest/pkg/utils/crypto"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/asaskevich/govalidator"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"dkforest/pkg/clockwork"
	"dkforest/pkg/config"
	humanize "github.com/dustin/go-humanize"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

// Constants ...
const (
	OneMinuteSecs = 60
	OneHourSecs   = OneMinuteSecs * 60
	OneDaySecs    = OneHourSecs * 24
	OneMonthSecs  = OneDaySecs * 30
)

// H is a hashmap
type H map[string]any

// SGo stands for Safe Go or Shit Go depending how you feel about goroutine panic handling
// Basically just a wrapper around the built-in keyword "go" with crash recovery
func SGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Error("unexpected crash", r)
				debug.PrintStack()
			}
		}()
		fn()
	}()
}

// EnsureRange ensure min is smaller or equal to max
func EnsureRange(min, max int64) (int64, int64) {
	if max < min {
		min, max = max, min
	}
	return min, max
}

// EnsureRangeDur ...
func EnsureRangeDur(min, max time.Duration) (time.Duration, time.Duration) {
	if max < min {
		min, max = max, min
	}
	return min, max
}

func castStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func doParseInt(s string) (out int) {
	out, _ = strconv.Atoi(s)
	return
}

// ParseInt64 shortcut for strconv.ParseInt base 10 64bit integer
func ParseInt64(v string) (int64, error) {
	return strconv.ParseInt(v, 10, 64)
}

// ParseInt shortcut for strconv.Atoi
func ParseInt(v string) (int, error) {
	return strconv.Atoi(v)
}

// ParseInt64OrDefault ...
func ParseInt64OrDefault(v string, d int64) (out int64) {
	var err error
	out, err = ParseInt64(v)
	if err != nil {
		out = d
	}
	return
}

// DoParseInt64 same as ParseInt64 but ignore errors
func DoParseInt64(v string) (out int64) {
	out, _ = ParseInt64(v)
	return
}

// DoParseInt same as ParseInt but ignore errors
func DoParseInt(v string) (out int) {
	out, _ = ParseInt(v)
	return
}

func ParseUint64(v string) (uint64, error) {
	p, err := strconv.ParseInt(v, 10, 64)
	return uint64(p), err
}

func DoParseUint64(v string) (out uint64) {
	out, _ = ParseUint64(v)
	return
}

// ParseF64 ...
func ParseF64(v string) (float64, error) {
	return strconv.ParseFloat(v, 64)
}

// DoParseF64 same as ParseF64 but ignore errors
func DoParseF64(v string) (out float64) {
	out, _ = ParseF64(v)
	return
}

// DoParsePInt64 ...
func DoParsePInt64(v string) (out *int64) {
	tmp, err := ParseInt64(v)
	if err != nil {
		return nil
	}
	return &tmp
}

// ParseBool ...
func ParseBool(v string) (bool, error) {
	return strconv.ParseBool(v)
}

// DoParseBool ...
func DoParseBool(v string) (out bool) {
	out, _ = ParseBool(v)
	return
}

// ParseMs parse string to milliseconds
func ParseMs(v string) (time.Duration, error) {
	tmp, err := ParseInt64(v)
	if err != nil {
		return 0, err
	}
	return time.Duration(tmp) * time.Millisecond, nil
}

// DoParseMs ...
func DoParseMs(v string) (out time.Duration) {
	out, _ = ParseMs(v)
	return
}

func refStr(s string) *string {
	return &s
}

// Sha1 returns sha1 hex sum as a string
func Sha1(in []byte) string {
	h := sha1.New()
	h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}

// Sha256 returns sha256 hex sum as a string
func Sha256(in []byte) string {
	h := sha256.New()
	h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}

// Sha512 returns sha512 hex sum as a string
func Sha512(in []byte) string {
	h := sha512.New()
	h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}

// MD5 returns md5 hex sum as a string
func MD5(in []byte) string {
	h := md5.New()
	h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}

func Crc32(in []byte) uint32 {
	h := crc32.NewIEEE()
	_, _ = h.Write(in)
	return h.Sum32()
}

// ShortDisplayID generate a short display id
func ShortDisplayID(size int64) string {
	if size <= 4 || size > 20 {
		return ""
	}
	b := make([]byte, size)
	_, _ = cryptoRand.Read(b)
	return hex.EncodeToString(b)[0:size]
}

// GenerateToken32 generate a random 32 bytes hex token
// fe3aa9e2a3362ed6fb19295e76dca9b74c9edb415affe1a9b3d8be23b8608e23
func GenerateToken32() string {
	return GenerateTokenN(32)
}

// GenerateToken16 ...
func GenerateToken16() string {
	return GenerateTokenN(16)
}

// GenerateToken10 ...
// 0144387f11c617517a41
func GenerateToken10() string {
	return GenerateTokenN(10)
}

func GenerateToken3() string {
	return GenerateTokenN(3)
}

func GenerateTokenN(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func TruncStr2(s string, maxLen int, suffix string) (out string) {
	if len(s) <= maxLen {
		return s
	}
	out = s[0:maxLen]
	idx := strings.LastIndex(out, " ")
	if idx > -1 {
		if idx < maxLen-20 {
			out = s[0:maxLen]
		} else {
			out = out[0:idx]
		}
	}
	if suffix != "" {
		out += suffix
	}
	return
}

func TruncStr(s string, maxLen int, suffix string) (out string) {
	if len(s) <= maxLen {
		return s
	}
	out = s[0:maxLen]
	if suffix != "" {
		out += suffix
	}
	return
}

// FormatInt64 shortcut for strconv.FormatInt base 10
func FormatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}

// IP2Long ...
func IP2Long(ip string) uint32 {
	var long uint32
	_ = binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.BigEndian, &long)
	return long
}

// BacktoIP4 ...
func BacktoIP4(ipInt int64) string {
	// need to do two bit shifting and “0xff” masking
	b0 := FormatInt64((ipInt >> 24) & 0xff)
	b1 := FormatInt64((ipInt >> 16) & 0xff)
	b2 := FormatInt64((ipInt >> 8) & 0xff)
	b3 := FormatInt64(ipInt & 0xff)
	return b0 + "." + b1 + "." + b2 + "." + b3
}

// ShortDur ...
func ShortDur(v any) string {
	if d, ok := v.(time.Duration); ok {
		d = d.Round(time.Second)
		s := d.String()
		if strings.HasSuffix(s, "m0s") {
			s = s[:len(s)-2]
		}
		if strings.HasSuffix(s, "h0m") {
			s = s[:len(s)-2]
		}
		return s
	} else if d, ok := v.(time.Time); ok {
		return ShortDur(time.Until(d))
	} else if d, ok := v.(uint64); ok {
		return ShortDur(time.Duration(d) * time.Second)
	} else if d, ok := v.(int64); ok {
		return ShortDur(time.Duration(d) * time.Second)
	} else if d, ok := v.(int32); ok {
		return ShortDur(time.Duration(d) * time.Second)
	} else if d, ok := v.(int64); ok {
		return ShortDur(time.Duration(d) * time.Second)
	} else if d, ok := v.(float32); ok {
		return ShortDur(time.Duration(d) * time.Second)
	} else if d, ok := v.(float64); ok {
		return ShortDur(time.Duration(d) * time.Second)
	}
	return "n/a"
}

// Sanitize ...
func Sanitize(txt string) string {
	p := bluemonday.UGCPolicy()
	return p.Sanitize(txt)
}

// N2br ...
func N2br(txt string) string {
	return strings.Replace(txt, "\n", "<br />", -1)
}

// Dot ...
func Dot(in int64) string {
	return strings.Replace(humanize.Comma(in), ",", ".", -1)
}

// EncryptAES ...
func EncryptAES(plaintext []byte, key []byte) ([]byte, error) {
	gcm, ns, err := getGCMKeyBytes(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, ns)
	if _, err = io.ReadFull(cryptoRand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAES ...
func DecryptAES(ciphertext []byte, key []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return []byte(""), nil
	}

	gcm, nonceSize, err := getGCMKeyBytes(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptAESMaster same as EncryptAES but use the default master key
func EncryptAESMaster(plaintext []byte) ([]byte, error) {
	return EncryptAES(plaintext, []byte(config.Global.MasterKey.Get()))
}

// DecryptAESMaster same as DecryptAES but use the default master key
func DecryptAESMaster(ciphertext []byte) ([]byte, error) {
	return DecryptAES(ciphertext, []byte(config.Global.MasterKey.Get()))
}

func EncryptStream(password []byte, src io.Reader) (*crypto.StreamEncrypter, error) {
	return crypto.NewStreamEncrypter(password, nil, src)
}

func DecryptStream(password, iv []byte, src io.Reader) (*crypto.StreamDecrypter, error) {
	decrypter, err := crypto.NewStreamDecrypter(password, nil, crypto.StreamMeta{IV: iv}, src)
	if err != nil {
		return nil, err
	}
	return decrypter, nil
}

func GetGCM(key string) (cipher.AEAD, int, error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, 0, err
	}
	gcm, nonceSize, err := getGCMKeyBytes(keyBytes)
	if err != nil {
		return nil, 0, err
	}
	return gcm, nonceSize, nil
}

func getGCMKeyBytes(keyBytes []byte) (cipher.AEAD, int, error) {
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, 0, err
	}
	nonceSize := gcm.NonceSize()
	return gcm, nonceSize, nil
}

func GetKeyFingerprint(pkey string) string {
	if e := GetEntityFromPKey(pkey); e != nil {
		return FormatPgPFingerprint(e.PrimaryKey.Fingerprint)
	}
	return ""
}

func FormatPgPFingerprint(fpBytes []byte) string {
	fp := strings.ToUpper(hex.EncodeToString(fpBytes))
	return fmt.Sprintf("%s %s %s %s %s  %s %s %s %s %s",
		fp[0:4], fp[4:8], fp[8:12], fp[12:16], fp[16:20],
		fp[20:24], fp[24:28], fp[28:32], fp[32:36], fp[36:40])
}

func PgpCheckClearSignMessage(pkey, msg string) bool {
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(pkey))
	if err != nil {
		return false
	}
	b, _ := clearsign.Decode([]byte(msg))
	if b == nil {
		return false
	}
	if _, err = b.VerifySignature(keyring, nil); err != nil {
		return false
	}
	return true
}

func GetEntityFromPKey(pkey string) *openpgp.Entity {
	reader := bytes.NewReader([]byte(pkey))
	if block, err := armor.Decode(reader); err == nil {
		r := packet.NewReader(block.Body)
		if e, err := openpgp.ReadEntity(r); err == nil {
			return e
		}
	}
	return nil
}

func PgpCheckSignMessage(pkey, msg, signature string) bool {
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(pkey))
	if err != nil {
		return false
	}
	verify := func(msg string) bool {
		_, err := openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(msg), strings.NewReader(signature), nil)
		return err == nil
	}
	// Text editors often add an extra line break, so let's check with and without it.
	return verify(msg) || verify(msg+"\n")
}

func PgpDecryptMessage(secretKey, msg string) (string, error) {
	readerMsg := bytes.NewReader([]byte(msg))
	block, err := armor.Decode(readerMsg)
	if err != nil {
		logrus.Error(err)
		return "", errors.New("invalid msg")
	}
	//if block.Type != "MESSAGE" {
	//	return "", errors.New("invalid message type")
	//}
	reader := bytes.NewReader([]byte(secretKey))
	e, err := openpgp.ReadArmoredKeyRing(reader)
	if err != nil {
		logrus.Error(err)
		return "", errors.New("invalid secret key")
	}
	md, err := openpgp.ReadMessage(block.Body, e, nil, nil)
	if err != nil {
		logrus.Error(err)
		return "", errors.New("unable to read message")
	}
	by, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		return "", err
	}
	decStr := string(by)
	return decStr, nil
}

func GeneratePgpEncryptedMessage(pkey, msg string) (string, error) {
	e := GetEntityFromPKey(pkey)
	if e == nil {
		return "", errors.New("invalid public key")
	}
	buffer := &bytes.Buffer{}
	armoredWriter, _ := armor.Encode(buffer, "PGP MESSAGE", nil)
	w, err := openpgp.Encrypt(armoredWriter, []*openpgp.Entity{e}, nil, nil, nil)
	if err != nil {
		// openpgp: invalid argument: cannot encrypt a message to key id xxx because it has no encryption keys
		// Likely your key is expired or had expired subkeys. (https://github.com/keybase/keybase-issues/issues/2072#issuecomment-183702559)
		logrus.Error(err)
		return "", err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		logrus.Error(err)
		return "", err
	}
	err = w.Close()
	if err != nil {
		logrus.Error(err)
		return "", err
	}
	_ = armoredWriter.Close()

	return buffer.String(), nil
}

// LoadLocals ...
func LoadLocals(bundle *i18n.Bundle) error {
	err := filepath.Walk(config.Global.ProjectLocalsPath.Get(), func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if strings.HasSuffix(info.Name(), ".yaml") {
				if _, err = bundle.LoadMessageFile(path); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return err
}

// MustGetDefaultProjectPath ...
func MustGetDefaultProjectPath() string {
	osUser, err := user.Current()
	if err != nil {
		log.Fatal("Unable to find os user")
	}
	return filepath.Join(osUser.HomeDir, config.AppDirName)
}

// MinInt returns the minimum int64 value
func MinInt[T Ints](vals ...T) T {
	minV := vals[0]
	for _, num := range vals {
		if num < minV {
			minV = num
		}
	}
	return minV
}

// MaxInt returns the minimum int64 value
func MaxInt[T Ints](vals ...T) T {
	maxV := vals[0]
	for _, num := range vals {
		if num > maxV {
			maxV = num
		}
	}
	return maxV
}

func Shuffle[T any](s []T) {
	rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

func Shuffle1[T any](r *rand.Rand, s []T) {
	r.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

type Ints interface {
	int | int64 | ~uint64
}

// Clamp ensure the value is within a range
func Clamp[T Ints](val, min, max T) T {
	val = MinInt(val, max)
	val = MaxInt(val, min)
	return val
}

// RandChoice returns a random element from an array
func RandChoice[T any](arr []T) T {
	if len(arr) == 0 {
		panic("empty array")
	}
	return arr[rand.Intn(len(arr))]
}

func RandBool() bool {
	return RandInt(0, 1) == 1
}

// DiceRoll receive an int 0-100, returns true "pct" of the time
func DiceRoll(pct int) bool {
	if pct < 0 || pct > 100 {
		panic("invalid dice roll value")
	}
	return RandInt(0, 100) <= pct
}

// RandI64 generates a number between min and max inclusively
func RandI64(min, max int64) int64 {
	if min == max {
		return min
	}
	if max < min {
		min, max = max, min
	}
	return rand.Int63n(max-min+1) + min
}

func RandInt(min, max int) int {
	if min == max {
		return min
	}
	if max < min {
		min, max = max, min
	}
	return rand.Intn(max-min+1) + min
}

func RandFloat(min, max float64) float64 {
	if min == max {
		return min
	}
	if max < min {
		min, max = max, min
	}
	return rand.Float64()*(max-min) + min
}

// RandMs generates random duration in milliseconds
func RandMs(min, max int64) time.Duration {
	return randDur(min, max, time.Millisecond)
}

// RandSec generates random duration in seconds
func RandSec(min, max int64) time.Duration {
	return randDur(min, max, time.Second)
}

// RandMin generates random duration in minutes
func RandMin(min, max int64) time.Duration {
	return randDur(min, max, time.Minute)
}

// RandHour generates random duration in hours
func RandHour(min, max int64) time.Duration {
	return randDur(min, max, time.Hour)
}

func randDur(min, max int64, dur time.Duration) time.Duration {
	return RandDuration(time.Duration(min)*dur, time.Duration(max)*dur)
}

// RandDuration generates random duration
func RandDuration(min, max time.Duration) time.Duration {
	n := RandI64(min.Nanoseconds(), max.Nanoseconds())
	return time.Duration(n) * time.Nanosecond
}

// SendTelegram sends a telegram message to a chatID
func SendTelegram(telegramBotToken string, chatID int64, msg string) error {
	if chatID == 0 {
		return errors.New("invalid chat ID 0")
	}
	tgbot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		return err
	}
	if _, err := tgbot.Send(tgbotapi.NewMessage(chatID, msg)); err != nil {
		return err
	}
	return nil
}

func SendDiscord(webhookURL string, msg string) error {
	type Payload struct {
		Content string `json:"content"`
	}
	data := Payload{
		Content: msg,
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	body := bytes.NewReader(payloadBytes)
	req, err := http.NewRequest("POST", webhookURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Last4 display last 4 digits of a CC
func Last4(cc string) string {
	return strings.Repeat("*", 12) + cc[len(cc)-4:]
}

// Once ...
type Once struct {
	m    sync.Mutex
	done uint32
}

func (o *Once) Now() <-chan time.Time {
	return o.After(0)
}

// After if and only if After is being called for the first time for this instance of Once.
func (o *Once) After(duration time.Duration) <-chan time.Time {
	if atomic.LoadUint32(&o.done) == 1 {
		return nil
	}
	// Slow-path.
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		return time.After(duration)
	}
	return nil
}

// Today returns today's date
func Today() time.Time {
	return TodayWithClock(clockwork.NewRealClock())
}

// TodayWithClock returns today's date using a clock
func TodayWithClock(clock clockwork.Clock) time.Time {
	year, month, day := clock.Now().Date()
	today := time.Date(year, month, day, 0, 0, 0, 0, clock.Now().Location())
	return today
}

// FileExists https://stackoverflow.com/a/12518877/4196220
func FileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

// GenerateRSAKeyPair need to initialize rand.Seed
func GenerateRSAKeyPair() (pubKey, privKey []byte, err error) {
	privateKey, err := rsa.GenerateKey(cryptoRand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	PubASN1, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	encodedPrivateKey := x509.MarshalPKCS1PrivateKey(privateKey)
	privKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: encodedPrivateKey})
	pubKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: PubASN1})
	return pubKey, privKey, nil
}

func GetRandomChatColor() string {
	colors := []string{
		"#F5F5DC",
		"#8A2BE2",
		"#A52A2A",
		"#00FFFF",
		"#00BFFF",
		"#FFD700",
		"#808080",
		"#008000",
		"#FF69B4",
		"#ADD8E6",
		"#90EE90",
		"#32CD32",
		"#FF00FF",
		"#808000",
		"#FFA500",
		"#FF4500",
		"#FF0000",
		"#4169E1",
		"#2E8B57",
		"#A0522D",
		"#C0C0C0",
		"#D2B48C",
		"#008080",
		"#EE82EE",
		"#FFFFFF",
		"#FFFF00",
		"#9ACD32",
	}
	return colors[rand.Intn(len(colors))]
}

type Font struct {
	Display string
	Value   int64
	Style   string
}

func GetFonts() []Font {
	return []Font{
		{"* Room Default *", 0, ""},
		{"Arial", 2, "Arial,Helvetica,sans-serif;"},
		{"Book Antiqua", 4, "'Book Antiqua','MS Gothic',serif;"},
		{"Comic", 5, "'Comic Sans MS',Papyrus,sans-serif;"},
		{"Courier", 1, "'Courier New',Courier,monospace;"},
		{"Cursive", 6, "Cursive,Papyrus,sans-serif;"},
		{"Garamond", 8, "Fantasy,Futura,Papyrus,sans;"},
		{"Georgia", 3, "Georgia,'Times New Roman',Times,serif;"},
		{"Serif", 9, "'MS Serif','New York',serif;"},
		{"System", 10, "System,Chicago,sans-serif;"},
		{"Times New Roman", 11, "Times New Roman',Times,serif;"},
		{"Verdana", 12, "Verdana,Geneva,Arial,Helvetica,sans-serif;"},
	}
}

func GetSound(id int64) string {
	switch id {
	case 1:
		return "/public/mp3/sound1.mp3"
	case 2:
		return "/public/mp3/sound1.mp3"
	case 3:
		return "/public/mp3/sound1.mp3"
	case 4:
		return "/public/mp3/sound4.mp3"
	}
	return "/public/mp3/sound1.mp3"
}

func WordCount(str string) (int, map[string]int) {
	wordList := strings.Fields(str)
	counts := make(map[string]int)
	for _, word := range wordList {
		_, ok := counts[word]
		if ok {
			counts[word] += 1
		} else {
			counts[word] = 1
		}
	}
	return len(wordList), counts
}

var hourMinSecRgx = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
var monthDayHourMinSecRgx = regexp.MustCompile(`^\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)

// ParseNextDatetimeAt given a string in this format 00:00:00 returns the next Time object at that time.
func ParseNextDatetimeAt(hourMinSec string, clock clockwork.Clock) (time.Time, error) {
	if !hourMinSecRgx.MatchString(hourMinSec) {
		return time.Time{}, errors.New("invalid format (should be 00:00:00)")
	}
	var hour, minute, sec int64
	if n, err := fmt.Sscanf(hourMinSec, "%d:%d:%d", &hour, &minute, &sec); err != nil || n != 3 {
		return time.Time{}, errors.New("invalid format (should be 00:00:00)")
	}
	return GetNextDatetimeAt(hour, minute, sec, clock)
}

// GetNextDatetimeAt given a hour, minute, second, returns the next Time object at that time.
func GetNextDatetimeAt(hour, min, sec int64, clock clockwork.Clock) (time.Time, error) {
	if hour < 0 || hour > 23 {
		return time.Time{}, errors.New("hour must be between [0, 23]")
	}
	if min < 0 || min > 59 {
		return time.Time{}, errors.New("min must be between [0, 59]")
	}
	if sec < 0 || sec > 59 {
		return time.Time{}, errors.New("sec must be between [0, 59]")
	}
	secondsOfDay := func(datetime time.Time) int {
		return (datetime.Hour() * 60 * 60) + (datetime.Minute() * 60) + datetime.Second()
	}
	hourMinSec := fmt.Sprintf("%02d:%02d:%02d", hour, min, sec)
	timeNow := clock.Now()
	nowInSeconds := secondsOfDay(timeNow)
	parsedDate, _ := time.ParseInLocation("2 Jan 2006 15:04:05", timeNow.Format("2 Jan 2006")+" "+hourMinSec, timeNow.Location())
	if nowInSeconds >= secondsOfDay(parsedDate) {
		parsedDate = parsedDate.Add(24 * time.Hour)
	}
	return parsedDate, nil
}

// ParsePrevDatetimeAt given a string in this format HH:MM:SS returns the previous Time object at that time.
func ParsePrevDatetimeAt(hourMinSec string, clock clockwork.Clock) (time.Time, error) {
	if !hourMinSecRgx.MatchString(hourMinSec) {
		return time.Time{}, errors.New("invalid format (should be HH:MM:SS)")
	}
	var hour, minute, sec int64
	if n, err := fmt.Sscanf(hourMinSec, "%d:%d:%d", &hour, &minute, &sec); err != nil || n != 3 {
		return time.Time{}, errors.New("invalid format (should be HH:MM:SS)")
	}
	return GetPrevDatetimeAt(hour, minute, sec, clock)
}

// ParsePrevDatetimeAt2 given a string in this format "mm-dd HH:MM:SS" returns the previous Time object at that time.
func ParsePrevDatetimeAt2(monthDayHourMinSec string, clock clockwork.Clock) (time.Time, error) {
	if !monthDayHourMinSecRgx.MatchString(monthDayHourMinSec) {
		return time.Time{}, errors.New("invalid format (should be mm-dd HH:MM:SS)")
	}
	var month, day, hour, minute, sec int64
	if n, err := fmt.Sscanf(monthDayHourMinSec, "%d-%d %d:%d:%d", &month, &day, &hour, &minute, &sec); err != nil || n != 5 {
		return time.Time{}, errors.New("invalid format (should be mm-dd HH:MM:SS)")
	}
	return GetPrevDatetimeAt2(month, day, hour, minute, sec, clock)
}

// GetPrevDatetimeAt given a hour, minute, second, returns the previous Time object at that time.
func GetPrevDatetimeAt(hour, min, sec int64, clock clockwork.Clock) (time.Time, error) {
	if hour < 0 || hour > 23 {
		return time.Time{}, errors.New("hour must be between [0, 23]")
	}
	if min < 0 || min > 59 {
		return time.Time{}, errors.New("min must be between [0, 59]")
	}
	if sec < 0 || sec > 59 {
		return time.Time{}, errors.New("sec must be between [0, 59]")
	}
	secondsOfDay := func(datetime time.Time) int {
		return (datetime.Hour() * 60 * 60) + (datetime.Minute() * 60) + datetime.Second()
	}
	hourMinSec := fmt.Sprintf("%02d:%02d:%02d", hour, min, sec)
	timeNow := clock.Now()
	nowInSeconds := secondsOfDay(timeNow)
	parsedDate, _ := time.ParseInLocation("2 Jan 2006 15:04:05", timeNow.Format("2 Jan 2006")+" "+hourMinSec, timeNow.Location())
	if nowInSeconds < secondsOfDay(parsedDate) {
		parsedDate = parsedDate.Add(-24 * time.Hour)
	}
	return parsedDate, nil
}

// GetPrevDatetimeAt2 given a month, day, hour, minute, second, returns the previous Time object at that time.
func GetPrevDatetimeAt2(month, day, hour, min, sec int64, clock clockwork.Clock) (time.Time, error) {
	if month < 1 || month > 12 {
		return time.Time{}, errors.New("month must be between [1, 12]")
	}
	if day < 1 || day > 31 {
		return time.Time{}, errors.New("day must be between [1, 31]")
	}
	if hour < 0 || hour > 23 {
		return time.Time{}, errors.New("hour must be between [0, 23]")
	}
	if min < 0 || min > 59 {
		return time.Time{}, errors.New("min must be between [0, 59]")
	}
	if sec < 0 || sec > 59 {
		return time.Time{}, errors.New("sec must be between [0, 59]")
	}
	monthDayHourMinSec := fmt.Sprintf("%02d-%02d %02d:%02d:%02d", month, day, hour, min, sec)
	timeNow := clock.Now()
	parsedDate, _ := time.ParseInLocation("2006 01-02 15:04:05", timeNow.Format("2006")+" "+monthDayHourMinSec, timeNow.Location())
	return parsedDate, nil
}

// ReencodePng to remove metadata
func ReencodePng(in []byte) (out []byte, err error) {
	var buf bytes.Buffer
	img, _, err := image.Decode(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReencodeJpg to remove metadata
func ReencodeJpg(in []byte) (out []byte, err error) {
	var buf bytes.Buffer
	img, _, err := image.Decode(bytes.NewReader(in))
	if err != nil {
		return nil, err
	}
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func Must1(err error) {
	if err != nil {
		panic(err)
	}
}

func ValidateRuneLength(str string, min, max int) bool {
	return govalidator.RuneLength(str, strconv.Itoa(min), strconv.Itoa(max))
}

func Ternary[T any](predicate bool, a, b T) T {
	if predicate {
		return a
	}
	return b
}

func TernaryOrZero[T any](predicate bool, a T) T {
	var zero T
	return Ternary(predicate, a, zero)
}

func InArr[T comparable](needle T, haystack []T) bool {
	for _, el := range haystack {
		if el == needle {
			return true
		}
	}
	return false
}

// CountBools given booleans, returns how many are set to true
func CountBools(vals ...bool) (count int64) {
	for _, v := range vals {
		if v {
			count++
		}
	}
	return count
}

func CountUppercase(s string) (count, total int64) {
	for _, r := range s {
		if unicode.IsLetter(r) {
			total++
			if unicode.IsUpper(r) {
				count++
			}
		}
	}
	return
}

func VerifyTorSign(onionAddr, msg, pemSig string) bool {
	block, _ := pem.Decode([]byte(pemSig))
	if block == nil {
		return false
	}
	sig := block.Bytes
	pub := identityKeyFromAddress(onionAddr)
	return ed25519.Verify(pub, []byte(msg), sig)
}

func identityKeyFromAddress(onionAddr string) ed25519.PublicKey {
	trimmedAddr := strings.TrimSuffix(onionAddr, ".onion")
	upperAddr := strings.ToUpper(trimmedAddr)
	decodedAddr, _ := base32.StdEncoding.DecodeString(upperAddr)
	return decodedAddr[:32]
}

func Slice2Set[T any, U comparable](s []T, f func(T) U) *hashset.HashSet[U] {
	h := hashset.New[U]()
	for _, e := range s {
		h.Set(f(e))
	}
	return h
}

type CryptoRandSource struct{}

func NewCryptoRandSource() CryptoRandSource {
	return CryptoRandSource{}
}

func (_ CryptoRandSource) Int63() int64 {
	var b [8]byte
	_, _ = cryptoRand.Read(b[:])
	// mask off sign bit to ensure positive number
	return int64(binary.LittleEndian.Uint64(b[:]) & (1<<63 - 1))
}

func (_ CryptoRandSource) Seed(_ int64) {}

func False() bool { return false }
func True() bool  { return true }
