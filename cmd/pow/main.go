package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Build with `go build -o pow main.go`
// Use `./pow my_username -d 7`
func main() {
	username := flag.String("username", "user", "the username to hash")
	difficulty := flag.Int("difficulty", 5, "the difficulty level of the hash")
	flag.Parse()

	prefix := strings.Repeat("0", *difficulty)
	var nonce int
	for {
		h := sha256.Sum256([]byte(*username + ":" + strconv.Itoa(nonce)))
		hashed := hex.EncodeToString(h[:])
		if strings.HasPrefix(hashed, prefix) {
		
