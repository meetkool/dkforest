package main

import (
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	var privKeyFile, fileName string
	flag.StringVar(&privKeyFile, "s", "hs_ed25519_secret_key", "tor private key file")
	flag.StringVar(&fileName, "c", "certificate.txt", "certificate to sign")
	flag.Parse()
	msg, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	pemKeyBytes, err := ioutil.ReadFile(privKeyFile)
	if err != nil {
		fmt.Println("Error reading private key file:", err)
		return
	}
	identityPrivKey := loadTorKeyFromDisk(pemKeyBytes)
	signature := sign(identityPrivKey, msg)
	fmt.Println(base64.StdEncoding.EncodeToString(signature))
}

func loadTorKeyFromDisk(keyBytes []byte) ed25519.PrivateKey {
	if !bytes.Equal(keyBytes[:29], []byte("== ed25519v1-secret: type0 ==")) {
		fmt.Println("Tor key does not start with Tor header")
		os.Exit(1)
	}
	expandedSk := keyBytes[32:]
	if len(expandedSk) != 64 {
		fmt.Println("Tor private key has the wrong length")
		os.Exit(1)
	}
	return expandedSk
}

func sign(identityPrivKey, msg []byte) []byte {
	return ed25519.Sign(identityPrivKey, msg)
}
