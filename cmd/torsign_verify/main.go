package main

import (
	"crypto/ed25519"
	"encoding/base32"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	var onionAddr, certFile string
	flag.StringVar(&onionAddr, "onion-address", "", "the_public_onion_address.onion")
	flag.StringVar(&onionAddr, "a", "", "the_public_onion_address.onion")
	flag.StringVar(&certFile, "cert", "", "certificate file to validate")
	flag.StringVar(&certFile, "c", "", "certificate file to validate")
	flag.Parse()

	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		panic(err)
	}
	cert := string(certBytes)
	cert = strings.TrimSpace(cert)
	cert = strings.TrimPrefix(cert, "-----BEGIN SIGNED MESSAGE-----\n")
	cert = strings.TrimSuffix(cert, "\n-----END SIGNATURE-----")
	parts := strings.Split(cert, "\n-----BEGIN SIGNATURE-----\n")
	msg := []byte(parts[0])
	sig, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(parts[1], "\n", ""))
	pub := identityKeyFromAddress(onionAddr)
	if ed25519.Verify(pub, msg, sig) {
		fmt.Println("valid signature")
	} else {
		fmt.Println("invalid signature")
	}
}

func identityKeyFromAddress(onionAddr string) ed25519.PublicKey {
	trimmedAddr := strings.TrimSuffix(onionAddr, ".onion")
	upperAddr := strings.ToUpper(trimmedAddr)
	decodedAddr, _ := base32.StdEncoding.DecodeString(upperAddr)
	return decodedAddr[:32]
}
