package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Build with `go build -o pow main.go`
// Use `./pow my_username`
func main() {
	username := os.Args[1]
	difficulty := 7
	prefix := strings.Repeat("0", difficulty)
	var nonce int
	for {
		h := sha256.Sum256([]byte(username + ":" + strconv.Itoa(nonce)))
		hashed := hex.EncodeToString(h[:])
		if strings.HasPrefix(hashed, prefix) {
			fmt.Printf("%s:%d -> %s\n", username, nonce, hashed)
			return
		}
		nonce++
	}
}
