package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"github.com/dustin/go-humanize"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// GetKeyFingerprint returns the SHA-1 fingerprint of a public key
func GetKeyFingerprint(pkey string) string {
	return utils.GetKeyFingerprint(pkey)
}

// ShortNum returns a human-readable shortened number string
func ShortNum(n int64) string {
	if n < 1000 {
		return utils.FormatInt64(n)
	} else if n >= 1000 && n < 1000000 {
		return utils.FormatInt64(n/1000) + "k"
	} else if n >= 1000000 {
		return utils.FormatInt64(n/1000000) + "M"
	}
	return utils.FormatInt64(n)
}

// ShortNumPtr returns a human-readable shortened number string for a nullable int64
func ShortNumPtr(n *int64) string {
	if n == nil {
		return "-"
	}
	return ShortNum(*n)
}

// N returns a channel that yields a sequence of integers
func N(start, end int64) <-chan int64 {
	stream := make(chan int64)
	go func() {
		for i := start; i <= end; i++ {
			stream <- i
		}
		close(stream)
	}()
	return stream
}

// Mod returns true if the first number is divisible by the second number
func Mod(i, j int64) bool {
	return i%j == 0
}

// AddInt returns the sum of two integers
func AddInt(a, b int) int {
	return a + b
}

// Add returns the sum of two int64 numbers
func Add(a, b int64) int64 {
	return a + b
}

// MD5 returns the MD5 hash of a string
func MD5(v string) string {
	return utils.MD5([]byte(v))
}

// LimitTo returns a substring of a string with a maximum length
func LimitTo(limit int64, s string) string {
	if int64(len(s)) > limit {
		return s[0:limit]
	}
	return s
}

// DerefStr returns the value of a nullable string
func DerefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CSS returns a template.CSS string
func CSS(s string) template.CSS {
	return template.CSS(s)
}

// Attr returns a template.HTMLAttr string
func Attr(s string) template.HTMLAttr {
	return template.HTMLAttr(s)
}

// Safe returns a template.HTML string
func Safe(s string) template.HTML {
	return template.HTML(s)
}

// SafeURL returns a template.URL string
func SafeURL(s string) template.URL {
	return template.URL(s)
}

// SafeJsStr returns a template.JSStr string
func SafeJsStr(s string) template.JSStr {
	return template.JSStr(s)
}

// SafeJs returns a template.JS string
func SafeJs(s string) template.JS {
	return template.JS(s)
}

// Success returns true if the status code is a 2xx success
func Success(code int64) bool {
	return code/100 == 2
}

// Divide100 returns a number divided by 100 as a float64
func Divide100(val int64) float64 {
	return float64(val) / 100
}

// Divide1000 returns a number divided by 1000 as a float64
func Divide1000(val int64) float64 {
	return float64(val) / 1000
}

// Divide100M returns a number divided by 100,000,000 as a float64
func Divide100M(val int64) float64 {
	return float64(val) / 100_000_000
}

// Divide1T returns a number divided by 1,000,000,000,000 as a float64
func Divide1T(val database.Piconero) float64 {
	return float64(val) / 1_000_000_000_000
}

// FormatPiconero returns a string representation of a Piconero value
func FormatPiconero(val database.Piconero) string {
	return val.XmrStr()
}

// Int64Bytes returns a human-readable string representation of a byte count
func Int64Bytes(val int64) string {
	return humanize.Bytes(uint64(val))
}

// ToString returns a JSON-encoded string representation of a value
func ToString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ToI64 returns an
