package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/asaskevich/govalidator"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
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

// DoParseInt64 same as ParseInt64 but ignore errors
func DoParseInt64(v string) (out int64) {
	out, _ = ParseInt64(v)
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

// ShortDisplayID generate a short display id
func ShortDisplayID(size int64) string {
	if size <= 4 || size > 20 {
		return ""
	}
	b := make([]byte, size)
	rand.Read(b)
	return hex.EncodeToString(b)[0:size]
}

// GenerateToken32 generate a random 32 bytes hex token
func GenerateToken32() string {
	return GenerateTokenN(32)
}

// GenerateToken10 ...
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
	by, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return "", err
	}
	decStr := string(by)
	return decStr, nil
}

func GeneratePgpEncryptedMessage(pkey, msg string) (string, error) {
	reader := bytes.NewReader([]byte(pkey))
	block, err := armor.Decode(reader)
	if err != nil {
		logrus.Error(err)
		return "", errors.New("invalid public key")
	}
	r := packet.NewReader(block.Body)
	e, err := openpgp.ReadEntity(r)
	if err != nil {
		logrus.Error(err)
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
	err := filepath.Walk(config.Global.ProjectLocalsPath(), func(path string, info os.FileInfo, err error) error {
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
	min := vals[0]
	for _, num := range vals {
		if num < min {
			min = num
		}
	}
	return min
}

// MaxInt returns the minimum int64 value
func MaxInt[T Ints](vals ...T) T {
	max := vals[0]
	for _, num := range vals {
		if num > max {
			max = num
		}
	}
	return max
}

type Ints interface {
	int | int64
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

// Random generates a number between min and max inclusively
func Random(min, max int64) int64 {
	if min == max {
		return min
	}
	if max < min {
		min, max = max, min
	}
	return rand.Int63n(max-min+1) + min
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

func RandInt(min, max int) int {
	if min == max {
		return min
	}
	if max < min {
		min, max = max, min
	}
	return int(rand.Int63n(int64(max-min+1))) + min
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

// RandMin generates random duration in hours
func RandHour(min, max int64) time.Duration {
	return randDur(min, max, time.Hour)
}

func randDur(min, max int64, dur time.Duration) time.Duration {
	return RandDuration(time.Duration(min)*dur, time.Duration(max)*dur)
}

// RandDuration generates random duration
func RandDuration(min, max time.Duration) time.Duration {
	n := Random(min.Nanoseconds(), max.Nanoseconds())
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
	var hour, min, sec int64
	if n, err := fmt.Sscanf(hourMinSec, "%d:%d:%d", &hour, &min, &sec); err != nil || n != 3 {
		return time.Time{}, errors.New("invalid format (should be 00:00:00)")
	}
	return GetNextDatetimeAt(hour, min, sec, clock)
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
	var hour, min, sec int64
	if n, err := fmt.Sscanf(hourMinSec, "%d:%d:%d", &hour, &min, &sec); err != nil || n != 3 {
		return time.Time{}, errors.New("invalid format (should be HH:MM:SS)")
	}
	return GetPrevDatetimeAt(hour, min, sec, clock)
}

// ParsePrevDatetimeAt2 given a string in this format "mm-dd HH:MM:SS" returns the previous Time object at that time.
func ParsePrevDatetimeAt2(monthDayHourMinSec string, clock clockwork.Clock) (time.Time, error) {
	if !monthDayHourMinSecRgx.MatchString(monthDayHourMinSec) {
		return time.Time{}, errors.New("invalid format (should be mm-dd HH:MM:SS)")
	}
	var month, day, hour, min, sec int64
	if n, err := fmt.Sscanf(monthDayHourMinSec, "%d-%d %d:%d:%d", &month, &day, &hour, &min, &sec); err != nil || n != 5 {
		return time.Time{}, errors.New("invalid format (should be mm-dd HH:MM:SS)")
	}
	return GetPrevDatetimeAt2(month, day, hour, min, sec, clock)
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
