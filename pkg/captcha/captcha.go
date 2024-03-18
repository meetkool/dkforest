// Package captcha implements generation and verification of image and audio
// CAPTCHAs.
//
// A captcha solution is the sequence of digits 0-9 with the defined length.
// There are two captcha representations: image and audio.
//
// An image representation is a PNG-encoded image with the solution printed on
// it in such a way that makes it hard for computers to solve it using OCR.
//
// An audio representation is a WAVE-encoded (8 kHz unsigned 8-bit) sound with
// the spoken solution (currently in English, Russian, Chinese, and Japanese).
// To make it hard for computers to solve audio captcha, the voice that
// pronounces numbers has random speed and pitch, and there is a randomly
// generated background noise mixed into the sound.
//
// This package doesn't require external files or libraries to generate captcha
// representations; it is self-contained.
//
// To make captchas one-time, the package includes a memory storage that stores
// captcha ids, their solutions, and expiration time. Used captchas are removed
// from the store immediately after calling Verify or VerifyString, while
// unused captchas (user loaded a page with captcha, but didn't submit the
// form) are collected automatically after the predefined expiration time.
// Developers can also provide custom store (for example, which saves captcha
// ids and solutions in database) by implementing Store interface and
// registering the object with SetCustomStore.
//
// Captchas are created by calling New, which returns the captcha id.  Their
// representations, though, are created on-the-fly by calling WriteImage or
// WriteAudio functions. Created representations are not stored anywhere, but
// subsequent calls to these functions with the same id will write the same
// captcha solution. Reload function will create a new different solution for
// the provided captcha, allowing users to "reload" captcha if they can't solve
// the displayed one without reloading the whole page.  Verify and VerifyString
// are used to verify that the given solution is the right one for the given
// captcha id.
package captcha

import (
	"bytes"
	"crypto/rand"
	"dkforest/pkg/config"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"strings"
	"time"
)

const (
	//alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	ALPHABET = "0123456789"
	// The number of captchas created that triggers garbage collection used
	// by default store.
	CollectNum = 100
	// Expiration time of captchas used by default store.
	Expiration = 10 * time.Minute
)

var (
	ErrNotFound       = errors.New("captcha: id not found")
	ErrInvalidCaptcha = errors.New("invalid captcha")
	ErrCaptchaExpired = errors.New("captcha expired")
	// globalStore is a shared storage for captchas, generated by New function.
	globalStore = NewMemoryStore(CollectNum, Expiration)
)

// SetCustomStore sets custom storage for captchas, replacing the default
// memory store. This function must be called before generating any captchas.
func SetCustomStore(s Store) {
	if _, ok := s.(memoryStore); !ok {
	
