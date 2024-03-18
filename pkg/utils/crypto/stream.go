package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"io/ioutil"
)

func NewCtrStram(encKey []byte) (cipher.Stream, cipher.Block, []byte, error) {
	block, err := aes.NewCipher(encKey)
	if err != nil {
	
