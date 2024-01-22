package utils

import (
	"context"
	"crypto/sha256"
	"dkforest/pkg/captcha"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"github.com/labstack/echo"
)

const (
	HBCookieName        = "dkft" // dkf troll
	WaitCookieName      = "wait-token"
	PokerReferralName   = "poker-referral-token"
	AuthCookieName      = "auth-token"
	AprilFoolCookieName = "april_fool"
	ByteRoadCookieName  = "challenge_byte_road_session"
)

var ForumDisabledErr = errors.New("forum is temporarily disabled")
var AccountTooYoungErr = errors.New("account must be at least 3 days old")

func CreateCookie(name, value string, maxAge int64) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Domain:   config.Global.CookieDomain.Get(),
		Secure:   config.Global.CookieSecure.Get(),
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
	encryptedVal, err := utils.EncryptAESMaster(by)
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
	v, err := utils.DecryptAESMaster(val)
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

func getLastMsgCookieName(roomName string) string {
	return fmt.Sprintf("last_known_msg_%s", roomName)
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

func CreateLastMsgCookie(c echo.Context, roomName, v string) {
	c.SetCookie(CreateCookie(getLastMsgCookieName(roomName), v, utils.OneDaySecs))
}

func GetLastMsgCookie(c echo.Context, roomName string) (*http.Cookie, error) {
	return c.Cookie(getLastMsgCookieName(roomName))
}

func CreatePokerReferralCookie(c echo.Context, v string) {
	c.SetCookie(CreateCookie(PokerReferralName, v, utils.OneDaySecs))
}

func GetPokerReferralCookie(c echo.Context) (*http.Cookie, error) {
	return c.Cookie(PokerReferralName)
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
func CaptchaVerifyString(c echo.Context, id, answer string) error {
	// Can bypass captcha in dev mode
	if config.Development.IsTrue() && answer == "000000" {
		return nil
	}
	if err := captcha.VerifyString(id, answer); err != nil {
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

func VerifyPow(username, nonce string, difficulty int) bool {
	h := sha256.Sum256([]byte(username + ":" + nonce))
	hashed := hex.EncodeToString(h[:])
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hashed, prefix)
}

func setStreamingHeaders(c echo.Context) {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(http.StatusOK)
	c.Response().Header().Set("Transfer-Encoding", "chunked")
	c.Response().Header().Set("Connection", "keep-alive")
}

func closeSignalChan(c echo.Context) <-chan struct{} {
	ctx, cancel := context.WithCancel(context.Background())
	// Listen to the closing of HTTP connection via CloseNotifier
	notify := c.Request().Context().Done()
	notify1 := make(chan os.Signal, 1)
	signal.Notify(notify1, syscall.SIGINT, syscall.SIGTERM)
	utils.SGo(func() {
		select {
		case <-notify:
		case <-notify1:
		}
		cancel()
	})
	return ctx.Done()
}

func SetStreaming(c echo.Context) <-chan struct{} {
	setStreamingHeaders(c)
	return closeSignalChan(c)
}

func GetReferer(c echo.Context) string {
	return c.Request().Referer()
}

func RedirectReferer(c echo.Context) error {
	return c.Redirect(http.StatusFound, GetReferer(c))
}

func MetaRefreshNow() string {
	return MetaRefresh(0)
}

func MetaRefresh(delay int) string {
	return MetaRedirect(delay, "")
}

func MetaRedirectNow(redirectURL string) string {
	return MetaRedirect(0, redirectURL)
}

func MetaRedirect(delay int, redirectURL string) string {
	content := fmt.Sprintf(`%d`, delay)
	if redirectURL != "" {
		content += fmt.Sprintf(`; URL='%s'`, redirectURL)
	}
	return fmt.Sprintf(`<meta http-equiv="refresh" content="%s" />`, content)
}

const CssReset = `html, body, div, span, applet, object, iframe,
h1, h2, h3, h4, h5, h6, p, blockquote, pre,
a, abbr, acronym, address, big, cite, code,
del, dfn, em, img, ins, kbd, q, s, samp,
small, strike, strong, sub, sup, tt, var,
b, u, i, center,
dl, dt, dd, ol, ul, li,
fieldset, form, label, legend,
table, caption, tbody, tfoot, thead, tr, th, td,
article, aside, canvas, details, embed,
figure, figcaption, footer, header, hgroup,
menu, nav, output, ruby, section, summary,
time, mark, audio, video {
	margin: 0;
	padding: 0;
	border: 0;
	font-size: 100%;
	font: inherit;
	vertical-align: baseline;
}

article, aside, details, figcaption, figure,
footer, header, hgroup, menu, nav, section {
	display: block;
}
body {
	line-height: 1;
}
ol, ul {
	list-style: none;
}
blockquote, q {
	quotes: none;
}
blockquote:before, blockquote:after,
q:before, q:after {
	content: '';
	content: none;
}
table {
	border-collapse: collapse;
	border-spacing: 0;
}`

const HtmlCssReset = `<style>` + CssReset + `</style>`
