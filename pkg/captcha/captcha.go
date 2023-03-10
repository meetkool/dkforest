// Copyright 2011 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
//
// Server provides an http.Handler which can serve image and audio
// representations of captchas automatically from the URL. It can also be used
// to reload captchas.  Refer to Server function documentation for details, or
// take a look at the example in "capexample" subdirectory.
package captcha

import (
	"bytes"
	"dkforest/pkg/config"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"time"
)

const (
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

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

// SetCustomStore sets custom storage for captchas, replacing the default
// memory store. This function must be called before generating any captchas.
func SetCustomStore(s Store) {
	globalStore = s
}

func randomId(rnd *rand.Rand) string {
	b := make([]byte, 10)
	_, _ = rnd.Read(b)
	return hex.EncodeToString(b)
}

// randomDigits returns a byte slice of the given length containing
// pseudorandom numbers in range 0-9. The slice can be used as a captcha
// solution.
func randomDigits(length int, rnd *rand.Rand) (out []byte) {
	for i := 0; i < length; i++ {
		out = append(out, byte(rnd.Intn(10)))
	}
	return
}

type Params struct {
	Store Store
	Rnd   *rand.Rand
}

// New creates a new captcha with the standard length, saves it in the internal
// storage and returns its id.
func New() (string, string) {
	return newLen(Params{})
}

func NewWithParams(params Params) (id, b64 string) {
	return newLen(params)
}

func newLen(params Params) (id, b64 string) {
	r := rnd
	s := globalStore
	if params.Store != nil {
		s = params.Store
	}
	if params.Rnd != nil {
		r = params.Rnd
	}
	id = randomId(r)
	digits := randomDigits(6, r)
	s.Set(id, digits)

	var buf bytes.Buffer
	_ = writeImage(s, &buf, id, r)
	captchaImg := base64.StdEncoding.EncodeToString(buf.Bytes())

	return id, captchaImg
}

// WriteImage writes PNG-encoded image representation of the captcha with the given id.
func WriteImage(w io.Writer, id string, rnd *rand.Rand) error {
	return writeImage(globalStore, w, id, rnd)
}

func WriteImageWithStore(store Store, w io.Writer, id string, rnd *rand.Rand) error {
	return writeImage(store, w, id, rnd)
}

func writeImage(store Store, w io.Writer, id string, rnd *rand.Rand) error {
	d, err := store.Get(id, false)
	if err != nil {
		return err
	}
	_, err = NewImage(d, config.CaptchaDifficulty.Load(), rnd).WriteTo(w)
	return err
}

// Verify returns true if the given digits are the ones that were used to
// create the given captcha id.
//
// The function deletes the captcha with the given id from the internal
// storage, so that the same captcha can't be verified anymore.
func Verify(id string, digits []byte) error {
	return verify(globalStore, id, digits, true)
}

func VerifyDangerous(store Store, id string, digits []byte) error {
	return verify(store, id, digits, false)
}

func verify(store Store, id string, digits []byte, clear bool) error {
	if digits == nil || len(digits) == 0 {
		return ErrInvalidCaptcha
	}
	realID, err := store.Get(id, clear)
	if err != nil {
		return err
	}
	if !bytes.Equal(digits, realID) {
		// reverse digits
		for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
			digits[i], digits[j] = digits[j], digits[i]
		}
		if !bytes.Equal(digits, realID) {
			return ErrInvalidCaptcha
		}
	}
	return nil
}

// VerifyString is like Verify, but accepts a string of digits.  It removes
// spaces and commas from the string, but any other characters, apart from
// digits and listed above, will cause the function to return false.
func VerifyString(id, digits string) error {
	return verifyString(globalStore, id, digits, true)
}

func VerifyStringDangerous(store Store, id, digits string) error {
	return verifyString(store, id, digits, false)
}

func verifyString(store Store, id, digits string, clear bool) error {
	if digits == "" {
		return ErrInvalidCaptcha
	}
	ns := make([]byte, len(digits))
	for i := range ns {
		d := digits[i]
		switch {
		case '0' <= d && d <= '9':
			ns[i] = d - '0'
		case d == ' ' || d == ',':
			// ignore
		default:
			return ErrInvalidCaptcha
		}
	}
	return verify(store, id, ns, clear)
}
