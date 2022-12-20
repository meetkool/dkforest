package utils

import (
	"dkforest/pkg/captcha"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"github.com/labstack/echo"
)

const (
	HBCookieName        = "dkft" // dkf troll
	WaitCookieName      = "wait-token"
	AuthCookieName      = "auth-token"
	AprilFoolCookieName = "april_fool"
	ByteRoadCookieName  = "challenge_byte_road_session"
)

var AccountTooYoungErr = errors.New("account must be at least 3 days old")

func CreateCookie(name, value string, maxAge int64) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Domain:   config.Global.CookieDomain(),
		Secure:   config.Global.CookieSecure(),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(maxAge),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Duration(maxAge) * time.Second),
	}
	return cookie
}

// CreateEncCookie return a cookie where the value has been json marshaled, encrypted and base64 encoded
func CreateEncCookie(name string, value any, maxAge int64) *http.Cookie {
	by, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	encryptedVal, err := utils.EncryptAES(by, []byte(config.Global.MasterKey()))
	if err != nil {
		return nil
	}
	valB64 := base64.URLEncoding.EncodeToString(encryptedVal)
	return CreateCookie(name, valB64, maxAge)
}

// EncCookie gets back the value of an encrypted cookie
func EncCookie[T any](c echo.Context, name string) (*http.Cookie, T, error) {
	var zero T
	cc, err := c.Cookie(name)
	if err != nil {
		return nil, zero, err
	}
	val, err := base64.URLEncoding.DecodeString(cc.Value)
	if err != nil {
		return nil, zero, err
	}
	v, err := utils.DecryptAES(val, []byte(config.Global.MasterKey()))
	if err != nil {
		return nil, zero, err
	}
	var out T
	if err := json.Unmarshal(v, &out); err != nil {
		return nil, zero, err
	}
	return cc, out, nil
}

func DeleteCookie(name string) *http.Cookie {
	return CreateCookie(name, "", -1)
}

func getGistCookieName(gistUUID string) string {
	return fmt.Sprintf("gist_%s_auth", gistUUID)
}

func getRoomCookieName(roomID int64) string {
	return fmt.Sprintf("room_%d_auth", roomID)
}

func getRoomKeyCookieName(roomID int64) string {
	return fmt.Sprintf("room_%d_key", roomID)
}

func GetRoomCookie(c echo.Context, roomID int64) (*http.Cookie, error) {
	return c.Cookie(getRoomCookieName(roomID))
}

func GetRoomKeyCookie(c echo.Context, roomID int64) (*http.Cookie, error) {
	return c.Cookie(getRoomKeyCookieName(roomID))
}

func DeleteRoomCookie(c echo.Context, roomID int64) {
	c.SetCookie(DeleteCookie(getRoomCookieName(roomID)))
	c.SetCookie(DeleteCookie(getRoomKeyCookieName(roomID)))
}

func CreateRoomCookie(c echo.Context, roomID int64, v, key string) {
	c.SetCookie(CreateCookie(getRoomCookieName(roomID), v, utils.OneDaySecs))
	c.SetCookie(CreateCookie(getRoomKeyCookieName(roomID), key, utils.OneDaySecs))
}

func GetGistCookie(c echo.Context, gistUUID string) (*http.Cookie, error) {
	return c.Cookie(getGistCookieName(gistUUID))
}

func DeleteGistCookie(c echo.Context, gistUUID string) {
	c.SetCookie(DeleteCookie(getGistCookieName(gistUUID)))
}

func CreateGistCookie(c echo.Context, gistUUID, v string) {
	c.SetCookie(CreateCookie(getGistCookieName(gistUUID), v, utils.OneDaySecs))
}

func GetAprilFoolCookie(c echo.Context) int {
	v, err := c.Cookie(AprilFoolCookieName)
	if err != nil {
		return 0
	}
	vv, err := strconv.Atoi(v.Value)
	if err != nil {
		return 0
	}
	return vv
}

func CreateAprilFoolCookie(c echo.Context, v int) {
	c.SetCookie(CreateCookie(AprilFoolCookieName, strconv.Itoa(v), utils.OneDaySecs))
}

// CaptchaVerifyString ensure that all captcha across the website makes HB life miserable.
func CaptchaVerifyString(c echo.Context, id, digits string) error {
	// Can bypass captcha in dev mode
	if config.Development.IsTrue() && digits == "000000" {
		return nil
	}
	if err := captcha.VerifyString(id, digits); err != nil {
		return errors.New("invalid answer")
	}
	// HB has 50% chance of having the captcha fails for no reason
	hbCookie, hbCookieErr := c.Cookie(HBCookieName)
	hasHBCookie := hbCookieErr == nil && hbCookie.Value != ""
	if hasHBCookie && utils.DiceRoll(50) {
		return errors.New("invalid answer")
	}
	return nil
}

func KillCircuit(c echo.Context) {
	if conn, ok := c.Request().Context().Value("conn").(net.Conn); ok {
		config.ConnMap.Close(conn)
	}
}
