package handlers

import (
	"bytes"
	dutils "dkforest/pkg/database/utils"
	pubsub2 "dkforest/pkg/pubsub"
	v1 "dkforest/pkg/web/handlers/api/v1"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/notnil/chess"
	"image"
	_ "image/gif"
	"image/png"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/global"
	armor1 "filippo.io/age/armor"

	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"filippo.io/age"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/asaskevich/govalidator"
	humanize "github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func firstUseHandler(c echo.Context) error {
	user := c.Get("authUser").(*database.User)
	var data firstUseData
	if user != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	if c.Request().Method == http.MethodGet {
		//data.Username = "admin"
		//data.Password = "admin123"
		//data.RePassword = "admin123"
		//data.Email = "admin@admin.admin"
		return c.Render(http.StatusOK, "first-use", data)
	}

	data.Username = c.Request().PostFormValue("username")
	data.Password = c.Request().PostFormValue("password")
	data.RePassword = c.Request().PostFormValue("repassword")
	newUser, errs := database.CreateFirstUser(data.Username, data.Password, data.RePassword)
	data.Errors = errs
	if errs.HasError() {
		return c.Render(http.StatusOK, "first-use", data)
	}

	_, errs = database.CreateZeroUser()

	config.IsFirstUse.SetFalse()

	session, err := database.CreateSession(newUser.ID, c.Request().UserAgent())
	if err != nil {
		logrus.Error("Failed to save session : ", err)
	}

	c.SetCookie(createSessionCookie(session.Token))

	return c.Redirect(http.StatusFound, "/")
}

var tempLoginCache = cache.New[TempLoginCaptcha](3*time.Minute, 3*time.Minute)
var tempLoginStore = captcha.NewMemoryStore(captcha.CollectNum, 3*time.Minute)

type TempLoginCaptcha struct {
	ID         string
	Img        string
	ValidUntil time.Time
}

// HomeHandler ...
func HomeHandler(c echo.Context) error {
	if config.IsFirstUse.IsTrue() {
		return firstUseHandler(c)
	}

	// If we're logged in, render the home page
	user := c.Get("authUser").(*database.User)
	if user != nil {
		return c.Render(http.StatusOK, "home", nil)
	}

	// If we protect the home page, render the special login page with time based captcha for login URL discovery
	if config.ProtectHome.IsTrue() {
		// return waitPageWrapper(c, protectHomeHandler, hutils.WaitCookieName)
		return protectHomeHandler(c)
	}

	// Otherwise, render the normal login page
	return loginHandler(c)
}

func protectHomeHandler(c echo.Context) error {
	if c.Request().Method == http.MethodPost {
		return c.NoContent(http.StatusNotFound)
	}
	captchaQuery := c.QueryParam("captcha")
	loginQuery := c.QueryParam("login")
	signupQuery := c.QueryParam("signup")
	if captchaQuery != "" {
		if len(captchaQuery) > 6 || len(loginQuery) > 1 || len(signupQuery) > 1 ||
			!govalidator.IsASCII(captchaQuery) || !govalidator.IsASCII(loginQuery) || !govalidator.IsASCII(signupQuery) {
			time.Sleep(utils.RandSec(3, 7))
			return c.NoContent(http.StatusOK)
		}
		redirectTo := "/login/" + captchaQuery
		if signupQuery == "1" {
			redirectTo = "/signup/" + captchaQuery
		}
		time.Sleep(utils.RandSec(1, 2))
		return c.Redirect(http.StatusFound, redirectTo)
	}
	loginLink, found := tempLoginCache.Get("login_link")
	if !found {
		loginLink.ID, loginLink.Img = captcha.NewWithParams(captcha.Params{Store: tempLoginStore})
		loginLink.ValidUntil = time.Now().Add(3 * time.Minute)
		tempLoginCache.SetD("login_link", loginLink)
	}

	waitTime := int64(time.Until(loginLink.ValidUntil).Seconds())

	// Generate css frames
	frames := generateCssFrames(waitTime, func(i int64) string {
		return utils.ShortDur(time.Duration(i) * time.Second)
	}, true)

	time.Sleep(utils.RandSec(1, 2))
	bufTmp := make([]byte, 0, 1024*4)
	buf := bytes.NewBuffer(bufTmp)
	buf.Write([]byte(`<!DOCTYPE html><html lang="en"><head>
    <link href="/public/img/favicon.ico" rel="icon" type="image/x-icon" />
    <meta charset="UTF-8" />
    <meta name="author" content="n0tr1v">
    <meta name="language" content="English">
    <meta name="revisit-after" content="1 days">
    <meta http-equiv="expires" content="0">
    <meta http-equiv="pragma" content="no-cache">
    <title>DarkForest</title>
    <style>
        body, html { height: 100%; width:100%; display:table; background-color: #222; color: white; line-height: 25px;
        font-family: Lato,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif,"Apple Color Emoji","Segoe UI Emoji","Segoe UI Symbol"; }
        body { display:table-cell; vertical-align:middle; }
        #parent { display: table; width: 100%; }
        #form_login { display: table; margin: auto; }
        .captcha-img { transition: transform .2s; }
        .captcha-img:hover { transform: scale(2.5); }
        #timer_countdown:before {
            content: "`))
	buf.Write([]byte(utils.ShortDur(time.Duration(waitTime) * time.Second)))
	buf.Write([]byte(`";
            animation: `))
	buf.Write([]byte(utils.FormatInt64(waitTime)))
	buf.Write([]byte(`s 1s forwards timer_countdown_frames;
        }
        @keyframes timer_countdown_frames {`))
	for _, frame := range frames {
		buf.Write([]byte(frame))
	}
	buf.Write([]byte(`
        }
    </style>
</head>
<body class="bg">

<div id="parent">
    <div id="form_login">
        <div class="text-center">
            <p>
                To login go to <code>/login/XXXXXX</code><br />
                To register go to <code>/signup/XXXXXX</code><br />
                (replace X by the numbers in the image)<br />
                Link valid for <strong><span id="timer_countdown"></span></strong>
            </p>
            <img src="data:image/png;base64,`))
	buf.Write([]byte(loginLink.Img))
	buf.Write([]byte(`" style="background-color: hsl(0, 0%, 90%);" class="captcha-img" />
            <form method="get">
                <input type="text" name="captcha" maxlength="6" autofocus />
                <button name="login" value="1" type="submit">Login</button>
                <button name="signup" value="1" type="submit">Register</button>
            </form>
        </div>
    </div>
</div>

</body>
</html>`))
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

// partialAuthCache keep track of partial auth token -> user id.
// When a user login and have 2fa enabled, we create a "partial" auth cookie.
// The token can be used to complete the 2fa authentication.
var partialAuthCache = cache.New[PartialAuthItem](10*time.Minute, time.Hour)

type PartialAuthItem struct {
	UserID database.UserID
	Step   PartialAuthStep // Inform which type of 2fa the user is supposed to complete
}

func NewPartialAuthItem(userID database.UserID, step PartialAuthStep) PartialAuthItem {
	return PartialAuthItem{UserID: userID, Step: step}
}

type PartialAuthStep string

const (
	TwoFactorStep PartialAuthStep = "2fa"
	PgpSignStep   PartialAuthStep = "pgp_sign_2fa"
	PgpStep       PartialAuthStep = "pgp_2fa"
)

func LoginHandler(c echo.Context) error {

	if config.ProtectHome.IsTrue() {
		return c.NoContent(http.StatusNotFound)
	}

	return loginHandler(c)
}

func LoginAttackHandler(c echo.Context) error {
	key := c.Param("loginToken")
	loginLink, found := tempLoginCache.Get("login_link")
	if !found {
		return c.NoContent(http.StatusNotFound)
	}
	// We use the "Dangerous" version of VerifyString, to avoid invalidating the captcha.
	// This way, the captcha can be used multiple times by different users until it's time has expired.
	if err := captcha.VerifyStringDangerous(tempLoginStore, loginLink.ID, key); err != nil {
		// If the captcha was invalid, kill the circuit.
		hutils.KillCircuit(c)
		time.Sleep(utils.RandSec(3, 5))
		return c.NoContent(http.StatusNotFound)
	}

	return loginHandler(c)
}

func loginHandler(c echo.Context) error {
	formName := c.Request().PostFormValue("formName")
	if formName == "" {
		var data loginData
		data.Autofocus = 0
		data.HomeUsersList = config.HomeUsersList.Load()

		if data.HomeUsersList {
			data.Online = managers.ActiveUsers.GetActiveUsers()
		}

		actualLogin := func(username, password string, captchaSolved bool) error {
			username = strings.TrimSpace(username)
			user, err := database.GetVerifiedUserByUsername(username)
			if err != nil {
				time.Sleep(utils.RandMs(50, 200))
				data.Error = "Invalid username/password"
				return c.Render(http.StatusOK, "login", data)
			}

			user.LoginAttempts++
			user.DoSave()

			if user.LoginAttempts > 4 && !captchaSolved {
				data.CaptchaRequired = true
				data.Autofocus = 2
				data.Error = "Captcha required"
				data.CaptchaID, data.CaptchaImg = captcha.New()
				data.Password = password
				captchaID := c.Request().PostFormValue("captcha_id")
				captchaInput := c.Request().PostFormValue("captcha")
				if captchaInput == "" {
					return c.Render(http.StatusOK, "login", data)
				} else {
					if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
						data.Error = "Invalid captcha"
						return c.Render(http.StatusOK, "login", data)
					}
				}
			}

			if !user.CheckPassword(password) {
				data.Password = ""
				data.Autofocus = 1
				data.Error = "Invalid username/password"
				return c.Render(http.StatusOK, "login", data)
			}

			if user.GpgTwoFactorEnabled {
				token := utils.GenerateToken32()
				if user.GpgTwoFactorMode {
					partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, PgpSignStep))
					return SessionsGpgSignTwoFactorHandler(c, true, token)
				}
				partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, PgpStep))
				return SessionsGpgTwoFactorHandler(c, true, token)

			} else if string(user.TwoFactorSecret) != "" {
				token := utils.GenerateToken32()
				partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, TwoFactorStep))
				return SessionsTwoFactorHandler(c, true, token)
			}

			return completeLogin(c, user)
		}

		usernameQuery := c.QueryParam("u")
		passwordQuery := c.QueryParam("p")
		if usernameQuery == "darkforestAdmin" && passwordQuery != "" {
			return actualLogin(usernameQuery, passwordQuery, false)
		}

		if config.ForceLoginCaptcha.IsTrue() {
			data.CaptchaID, data.CaptchaImg = captcha.New()
			data.CaptchaRequired = true
		}

		if c.Request().Method == http.MethodGet {
			return c.Render(http.StatusOK, "login", data)
		}

		captchaSolved := false

		data.Username = strings.TrimSpace(c.FormValue("username"))
		password := c.FormValue("password")

		if config.ForceLoginCaptcha.IsTrue() {
			data.CaptchaRequired = true
			captchaID := c.Request().PostFormValue("captcha_id")
			captchaInput := c.Request().PostFormValue("captcha")
			if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
				data.ErrCaptcha = err.Error()
				return c.Render(http.StatusOK, "login", data)
			}
			captchaSolved = true
		}

		return actualLogin(data.Username, password, captchaSolved)

	} else if formName == "pgp_2fa" {
		token := c.Request().PostFormValue("token")
		return SessionsGpgTwoFactorHandler(c, false, token)
	} else if formName == "pgp_sign_2fa" {
		token := c.Request().PostFormValue("token")
		return SessionsGpgSignTwoFactorHandler(c, false, token)
	} else if formName == "2fa" {
		token := c.Request().PostFormValue("token")
		return SessionsTwoFactorHandler(c, false, token)
	} else if formName == "2fa_recovery" {
		token := c.Request().PostFormValue("token")
		return SessionsTwoFactorRecoveryHandler(c, token)
	}
	return c.Redirect(http.StatusOK, "/")
}

func completeLogin(c echo.Context, user database.User) error {
	user.LoginAttempts = 0
	_ = user.Save()

	for _, session := range database.GetActiveUserSessions(user.ID) {
		msg := fmt.Sprintf(`New login`)
		database.CreateSessionNotification(msg, session.Token)
	}

	session, err := database.CreateSession(user.ID, c.Request().UserAgent())
	if err != nil {
		logrus.Error("Failed to create session : ", err)
	}

	database.CreateSecurityLog(user.ID, database.LoginSecurityLog)
	c.SetCookie(createSessionCookie(session.Token))

	redirectURL := "/"
	redir := c.QueryParam("redirect")
	if redir != "" && strings.HasPrefix(redir, "/") {
		redirectURL = redir
	}
	return c.Redirect(http.StatusFound, redirectURL)
}

func LoginCompletedHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data loginCompletedData
	data.SecretPhrase = string(authUser.SecretPhrase)
	data.RedirectURL = "/"
	redir := c.QueryParam("redirect")
	if redir != "" && strings.HasPrefix(redir, "/") {
		data.RedirectURL = redir
	}
	return c.Render(http.StatusOK, "login-completed", data)
}

// SessionsGpgTwoFactorHandler ...
func SessionsGpgTwoFactorHandler(c echo.Context, step1 bool, token string) error {
	item, found := partialAuthCache.Get(token)
	if !found || item.Step != PgpStep {
		return c.Redirect(http.StatusFound, "/")
	}

	user, err := database.GetUserByID(item.UserID)
	if err != nil {
		logrus.Errorf("failed to get user %d", item.UserID)
		return c.Redirect(http.StatusFound, "/")
	}

	var data sessionsGpgTwoFactorData
	data.Token = token

	if step1 {
		msg, err := generatePgpEncryptedTokenMessage(user.ID, user.GPGPublicKey)
		if err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "/sessions-gpg-two-factor", data)
		}
		data.EncryptedMessage = msg
		return c.Render(http.StatusOK, "sessions-gpg-two-factor", data)
	}

	pgpToken, found := pgpTokenCache.Get(user.ID)
	if !found {
		return c.Redirect(http.StatusFound, "/")
	}
	data.EncryptedMessage = c.Request().PostFormValue("encrypted_message")
	data.Code = c.Request().PostFormValue("pgp_code")
	if data.Code != pgpToken.Value {
		data.ErrorCode = "invalid code"
		return c.Render(http.StatusOK, "sessions-gpg-two-factor", data)
	}
	pgpTokenCache.Delete(user.ID)
	partialAuthCache.Delete(token)

	if string(user.TwoFactorSecret) != "" {
		token := utils.GenerateToken32()
		partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, TwoFactorStep))
		return SessionsTwoFactorHandler(c, true, token)
	}

	return completeLogin(c, user)
}

// SessionsGpgSignTwoFactorHandler ...
func SessionsGpgSignTwoFactorHandler(c echo.Context, step1 bool, token string) error {
	item, found := partialAuthCache.Get(token)
	if !found || item.Step != PgpSignStep {
		return c.Redirect(http.StatusFound, "/")
	}

	user, err := database.GetUserByID(item.UserID)
	if err != nil {
		logrus.Errorf("failed to get user %d", item.UserID)
		return c.Redirect(http.StatusFound, "/")
	}

	var data sessionsGpgSignTwoFactorData
	data.Token = token

	if step1 {
		data.ToBeSignedMessage = generatePgpToBeSignedTokenMessage(user.ID, user.GPGPublicKey)
		return c.Render(http.StatusOK, "sessions-gpg-sign-two-factor", data)
	}

	pgpToken, found := pgpTokenCache.Get(user.ID)
	if !found {
		return c.Redirect(http.StatusFound, "/")
	}
	data.ToBeSignedMessage = c.Request().PostFormValue("to_be_signed_message")
	data.SignedMessage = c.Request().PostFormValue("signed_message")

	if !utils.PgpCheckSignMessage(pgpToken.PKey, pgpToken.Value, data.SignedMessage) {
		data.ErrorSignedMessage = "invalid signature"
		return c.Render(http.StatusOK, "sessions-gpg-sign-two-factor", data)
	}
	pgpTokenCache.Delete(user.ID)
	partialAuthCache.Delete(token)

	if string(user.TwoFactorSecret) != "" {
		token := utils.GenerateToken32()
		partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, TwoFactorStep))
		return SessionsTwoFactorHandler(c, true, token)
	}

	return completeLogin(c, user)
}

// SessionsTwoFactorHandler ...
func SessionsTwoFactorHandler(c echo.Context, step1 bool, token string) error {
	item, found := partialAuthCache.Get(token)
	if !found || item.Step != TwoFactorStep {
		return c.Redirect(http.StatusFound, "/")
	}

	var data sessionsTwoFactorData
	data.Token = token
	if !step1 {
		code := c.Request().PostFormValue("code")
		user, err := database.GetUserByID(item.UserID)
		if err != nil {
			logrus.Errorf("failed to get user %d", item.UserID)
			return c.Redirect(http.StatusFound, "/")
		}
		secret := string(user.TwoFactorSecret)
		if !totp.Validate(code, secret) {
			data.Error = "Two-factor authentication failed."
			return c.Render(http.StatusOK, "sessions-two-factor", data)
		}

		partialAuthCache.Delete(token)

		return completeLogin(c, user)
	}
	return c.Render(http.StatusOK, "sessions-two-factor", data)
}

// SessionsTwoFactorRecoveryHandler ...
func SessionsTwoFactorRecoveryHandler(c echo.Context, token string) error {
	item, found := partialAuthCache.Get(token)
	if !found {
		return c.Redirect(http.StatusFound, "/")
	}

	var data sessionsTwoFactorRecoveryData
	data.Token = token
	recoveryCode := c.Request().PostFormValue("code")
	if recoveryCode != "" {
		user, err := database.GetUserByID(item.UserID)
		if err != nil {
			logrus.Errorf("failed to get user %d", item.UserID)
			return c.Redirect(http.StatusFound, "/")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.TwoFactorRecovery), []byte(recoveryCode)); err != nil {
			data.Error = "Recovery code authentication failed"
			return c.Render(http.StatusOK, "sessions-two-factor-recovery", data)
		}

		partialAuthCache.Delete(token)

		return completeLogin(c, user)
	}
	return c.Render(http.StatusOK, "sessions-two-factor-recovery", data)
}

// LogoutHandler for logout route
func LogoutHandler(ctx echo.Context) error {
	authUser := ctx.Get("authUser").(*database.User)
	c, _ := ctx.Cookie(hutils.AuthCookieName)
	if err := database.DeleteSessionByToken(c.Value); err != nil {
		logrus.Error("Failed to remove session from DB : ", err)
	}
	if authUser.TerminateAllSessionsOnLogout {
		// Delete active user sessions
		if err := database.DeleteUserSessions(authUser.ID); err != nil {
			logrus.Error("failed to delete user sessions : ", err)
		}
	}
	database.CreateSecurityLog(authUser.ID, database.LogoutSecurityLog)
	ctx.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
	managers.ActiveUsers.RemoveUser(authUser.ID)
	if authUser.Temp {
		if err := database.DB.Where("id = ?", authUser.ID).Unscoped().Delete(&database.User{}).Error; err != nil {
			logrus.Error(err)
		}
	}
	return ctx.Redirect(http.StatusFound, "/")
}

func createSessionCookie(value string) *http.Cookie {
	return hutils.CreateCookie(hutils.AuthCookieName, value, utils.OneMonthSecs)
}

// FlashResponse ...
type FlashResponse struct {
	Message  string
	Redirect string
	Type     string
}

func SignupAttackHandler(c echo.Context) error {
	key := c.Param("signupToken")
	loginLink, found := tempLoginCache.Get("login_link")
	if !found {
		return c.NoContent(http.StatusNotFound)
	}
	if err := captcha.VerifyStringDangerous(tempLoginStore, loginLink.ID, key); err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	return tmpSignupHandler(c)
}

// SignupInvitationHandler ...
func SignupInvitationHandler(c echo.Context) error {
	invitationToken := c.Param("invitationToken")
	invitationTokenQuery := c.QueryParam("invitationToken")
	if invitationTokenQuery != "" {
		invitationToken = invitationTokenQuery
	}
	if _, err := database.GetUnusedInvitationByToken(invitationToken); err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	return waitPageWrapper(c, signupHandler, hutils.WaitCookieName)
}

func AesNB64(in string) string {
	encryptedVal, _ := utils.EncryptAES([]byte(in), []byte(config.Global.MasterKey()))
	return base64.URLEncoding.EncodeToString(encryptedVal)
}

func DAesB64(in string) ([]byte, error) {
	enc, err := base64.URLEncoding.DecodeString(in)
	if err != nil {
		return nil, err
	}
	encryptedVal, err := utils.DecryptAES(enc, []byte(config.Global.MasterKey()))
	if err != nil {
		return nil, err
	}
	return encryptedVal, nil
}

func DAesB64Str(in string) (string, error) {
	encryptedVal, err := DAesB64(in)
	return string(encryptedVal), err
}

func MetaCss(c echo.Context) error {
	return c.Blob(http.StatusOK, "text/css; charset=utf-8", metaCss("signal"))
}

func SignupCss(c echo.Context) error {
	return c.Blob(http.StatusOK, "text/css; charset=utf-8", metaCss(c.Param("signupToken")))
}

func metaCss(token string) []byte {
	lb := "" // \n
	var buf bytes.Buffer
	sizes := []float64{0, 320, 350, 390, 430, 470, 520, 570, 620, 690, 750, 830, 910, 1000, 1100, 1220, 1340, 1470, 1620, 1780, 1960, 2150, 2370, 2600}
	for i := 1; i < len(sizes); i++ {
		prev := sizes[i-1]
		size := sizes[i]
		_, _ = fmt.Fprintf(&buf, "@media(min-device-width:%.0fpx) and (max-device-width:%.5fpx){.div_1{background:url('/public/img/%s/sw/%.0fx%.0f.png')}}%s", prev, size-0.00001, token, prev, size, lb)
	}
	_, _ = fmt.Fprintf(&buf, "@media(min-device-width:%.0fpx){.div_1{background:url('/public/img/%s/sw/%.0fx.png')}}%s", 2600.0, token, 2600.0, lb)
	for i := 1; i < len(sizes); i++ {
		prev := sizes[i-1]
		size := sizes[i]
		_, _ = fmt.Fprintf(&buf, "@media(min-device-height:%.0fpx) and (max-device-height:%.5fpx){.div_2{background:url('/public/img/%s/sh/%.0fx%.0f.png')}}%s", prev, size-0.00001, token, prev, size, lb)
	}
	_, _ = fmt.Fprintf(&buf, "@media(min-device-height:%.0fpx){.div_2{background:url('/public/img/%s/sh/%.0fx.png')}}%s", 2600.0, token, 2600.0, lb)
	fonts := []string{"Helvatica"}
	for idx, font := range fonts {
		_, _ = fmt.Fprintf(&buf, `@font-face{font-family:'%s';src:local('%s'),url('/public/img/%s/%s/%s.ttf')format('truetype');}%s`, font, font, token, font, font, lb)
		_, _ = fmt.Fprintf(&buf, `.div_f%d{font-family:'%s';position:absolute;top:-100px}%s`, idx, font, lb)
	}
	return buf.Bytes()
}

var signupCache = cache.New[SignupInfo](5*time.Minute, 5*time.Minute)

type SignupInfo struct {
	ScreenWidth     string
	ScreenHeight    string
	HelvaticaLoaded bool

	hasSolvedCaptcha bool
	UpdatedAt        string
}

func SignalCss1(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	data := c.Param("data")
	data = strings.TrimRight(data, ".png")
	data = strings.TrimRight(data, ".ttf")
	signalP := c.Param("signal")
	var info SignupInfo
	_ = json.Unmarshal([]byte(authUser.SignupMetadata), &info)
	switch signalP {
	case "sw":
		info.ScreenWidth = data
	case "sh":
		info.ScreenHeight = data
	case "Helvatica":
		info.HelvaticaLoaded = true
	}
	info.UpdatedAt = time.Now().Format(time.RFC3339)
	signupInfoEnc, _ := json.Marshal(info)
	authUser.SignupMetadata = string(signupInfoEnc)
	authUser.DoSave()
	return c.NoContent(http.StatusOK)
}

func SignalCss(c echo.Context) error {
	data := c.Param("data")
	data = strings.TrimRight(data, ".png")
	data = strings.TrimRight(data, ".ttf")
	token := c.Param("signupToken")
	signalP := c.Param("signal")
	var info SignupInfo
	if val, found := signupCache.Get(token); found {
		info = val
	} else {
		info = SignupInfo{}
	}
	switch signalP {
	case "sw":
		info.ScreenWidth = data
	case "sh":
		info.ScreenHeight = data
	case "Helvatica":
		info.HelvaticaLoaded = true
	}
	info.UpdatedAt = time.Now().Format(time.RFC3339)
	signupCache.SetD(token, info)
	return c.NoContent(http.StatusOK)
}

// SignupHandler ...
func SignupHandler(c echo.Context) error {
	if config.ProtectHome.IsTrue() {
		return c.NoContent(http.StatusNotFound)
	}
	return tmpSignupHandler(c)
}

func tmpSignupHandler(c echo.Context) error {
	if config.SignupFakeEnabled.IsFalse() && config.SignupEnabled.IsFalse() {
		return c.Render(http.StatusOK, "signup-invite", nil)
	}
	return waitPageWrapper(c, signupHandler, hutils.WaitCookieName)
}

type WaitPageCookiePayload struct {
	Token string
	Count int64
	Now   int64
	Unix  int64
}

func waitPageWrapper(c echo.Context, clb echo.HandlerFunc, cookieName string) error {
	now := time.Now()
	start := now.UnixNano()
	var waitToken string

	if cc, payload, err := hutils.EncCookie[WaitPageCookiePayload](c, cookieName); err != nil {
		// No cookie found, we create one and display the waiting page.
		waitTime := utils.Random(5, 15)
		waitToken = utils.GenerateToken10()
		payload := WaitPageCookiePayload{
			Token: waitToken,
			Count: 1,
			Now:   now.UnixMilli(),
			Unix:  now.Unix() + waitTime - 1, // unix time at which the wait time is over
		}
		c.SetCookie(hutils.CreateEncCookie(cookieName, payload, utils.OneMinuteSecs*5))

		var data waitData
		// Generate css frames
		data.Frames = generateCssFrames(waitTime, nil, true)
		data.WaitTime = waitTime
		data.WaitToken = waitToken
		return c.Render(http.StatusOK, "wait", data)

	} else {
		// Cookie was found, incr counter then call callback
		waitToken = payload.Token
		start = payload.Now
		if c.Request().Method == http.MethodGet {
			// If you reload the page before the wait time is over, we kill the circuit.
			if now.Unix() < payload.Unix {
				hutils.KillCircuit(c)
				return c.String(http.StatusFound, "DDoS filter killed your path")
			}

			// If the wait time is over, and you reload the protected page more than 4 times, we make you wait 1min
			if payload.Count >= 4 {
				c.SetCookie(hutils.CreateCookie(cookieName, cc.Value, utils.OneMinuteSecs))
				return c.String(http.StatusFound, "You tried to reload the page too many times. Now you have to wait one minute.")
			}
			payload.Count++
			payload.Now = now.UnixMilli()
			c.SetCookie(hutils.CreateEncCookie(cookieName, payload, utils.OneMinuteSecs*5))
		}
	}
	c.Set("start", start)
	c.Set("signupToken", waitToken)
	return clb(c)
}

// The random wait time 0-15 seconds make sure the load is evenly distributed while under DDoS.
// Not all requests to the signup endpoint will get the captcha at the same time,
// so you cannot just refresh the page until you get a captcha that is easier to crack.
func signupHandler(c echo.Context) error {
	start := c.Get("start").(int64)
	signupToken := c.Get("signupToken").(string)
	var data signupData
	config.SignupPageLoad.Inc()

	data.CaptchaSec = 120
	data.Frames = generateCssFrames(data.CaptchaSec, nil, true)

	hbCookie, hbCookieErr := c.Cookie(hutils.HBCookieName)
	hasHBCookie := hbCookieErr == nil && hbCookie.Value != ""

	signupInfo, _ := signupCache.Get(signupToken)

	data.HasSolvedCaptcha = signupInfo.hasSolvedCaptcha
	if !signupInfo.hasSolvedCaptcha {
		data.CaptchaID, data.CaptchaImg = captcha.New()
	}

	if c.Request().Method == http.MethodPost {
		data.Username = strings.TrimSpace(c.Request().PostFormValue("username"))
		data.Password = c.Request().PostFormValue("password")
		data.RePassword = c.Request().PostFormValue("repassword")
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		captchaInputImg := c.Request().PostFormValue("captcha_img")
		if !signupInfo.hasSolvedCaptcha {
			if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
				data.ErrCaptcha = err.Error()
				config.SignupFailed.Inc()
				return c.Render(http.StatusOK, "signup", data)
			}
		}
		data.Captcha = captchaInput
		data.CaptchaImg = captchaInputImg

		signupInfo.hasSolvedCaptcha = true
		data.HasSolvedCaptcha = signupInfo.hasSolvedCaptcha
		signupCache.SetD(signupToken, signupInfo)

		config.SignupSucceed.Inc()

		// If SignupFakeEnabled is enabled, we always say the account was created, but we do not create it.
		if config.SignupFakeEnabled.IsTrue() {
			c.SetCookie(hutils.DeleteCookie(hutils.WaitCookieName))
			return c.Render(http.StatusOK, "flash", FlashResponse{"Your account has been created", "/login", "alert-success"})
		}

		// Fuck with kicked users. Prevent them from registering again.
		//authCookie, err := c.Cookie("auth-token")
		//if err == nil && authCookie.Value != "" {
		//	return c.Render(http.StatusOK, "flash", FlashResponse{"Your account has been created", "/login", "alert-success"})
		//}

		signupInfoEnc, _ := json.Marshal(signupInfo)

		registrationDuration := time.Now().UnixMilli() - start
		newUser, errs := database.CreateUser(data.Username, data.Password, data.RePassword, registrationDuration, string(signupInfoEnc))
		if errs.HasError() {
			data.Errors = errs
			return c.Render(http.StatusOK, "signup", data)
		}

		// Fuck with hellbanned users. New account also hellbanned
		if hasHBCookie {
			newUser.IsHellbanned = true
			newUser.DoSave()
		}

		invitationToken := c.Param("invitationToken")
		if invitationToken != "" {
			if invitation, err := database.GetUnusedInvitationByToken(invitationToken); err == nil {
				invitation.InviteeUserID = newUser.ID
				invitation.DoSave()
			}
		}

		// If more than 10 users were created in the past minute, auto disable signup for the website
		if database.GetRecentUsersCount() > 10 {
			settings := database.GetSettings()
			settings.SignupEnabled = false
			settings.DoSave()
			config.SignupEnabled.SetFalse()
			if userNull, err := database.GetUserByUsername(config.NullUsername); err == nil {
				database.NewAudit(userNull, fmt.Sprintf("auto turn off signup"))

				// Display message in chat
				txt := fmt.Sprintf("auto turn off registrations")
				if err := database.CreateSysMsg(txt, txt, "", config.GeneralRoomID, userNull.ID); err != nil {
					logrus.Error(err)
				}
			}
		}

		c.SetCookie(hutils.DeleteCookie(hutils.WaitCookieName))
		return c.Render(http.StatusOK, "flash", FlashResponse{"Your account has been created", "/login", "alert-success"})
	}

	return c.Render(http.StatusOK, "signup", data)
}

// RecaptchaResponse ...
type RecaptchaResponse struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

// Password recovery flow has 3 steps
// 1- Ask for username & captcha & gpg method
// 2- Validate gpg token/signature
// 3- Reset password
// Since the user is not authenticated in any of these steps, we need to guard each steps and ensure the user can access it legitimately.
// partialRecoveryCache keeps track of users that are in the process of recovering their password and the step they're at.
var (
	partialRecoveryCache = cache.New[PartialRecoveryItem](10*time.Minute, time.Hour)
)

type PartialRecoveryItem struct {
	UserID database.UserID
	Step   RecoveryStep
}

type RecoveryStep int64

const (
	RecoveryCaptchaCompleted RecoveryStep = iota + 1
	RecoveryGpgValidated
)

func generateCssFrames(n int64, contentFn func(int64) string, reverse bool) (frames []string) {
	step := 100.0 / float64(n)
	pct := 0.0
	for i := int64(0); i <= n; i++ {
		num := i
		if reverse {
			num = n - i
		}
		if contentFn == nil {
			contentFn = utils.FormatInt64
		}
		frames = append(frames, fmt.Sprintf(`%.2f%% { content: "%s"; }`, pct, contentFn(num)))
		pct += step
	}
	return
}

// ForgotPasswordHandler ...
func ForgotPasswordHandler(c echo.Context) error {
	return waitPageWrapper(c, forgotPasswordHandler, hutils.WaitCookieName)
}

func forgotPasswordHandler(c echo.Context) error {
	var data forgotPasswordData
	data.Step = 1

	data.CaptchaSec = 120
	data.Frames = generateCssFrames(data.CaptchaSec, nil, true)

	data.CaptchaID, data.CaptchaImg = captcha.New()

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "forgot-password", data)
	}

	// POST

	formName := c.Request().PostFormValue("form_name")

	if formName == "step1" {
		// Receive and validate Username/Captcha
		data.Step = 1
		data.Username = c.Request().PostFormValue("username")
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		data.GpgMode = utils.DoParseBool(c.Request().PostFormValue("gpg_mode"))

		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, "forgot-password", data)
		}
		user, err := database.GetUserByUsername(data.Username)
		if err != nil {
			data.UsernameError = "no such user"
			return c.Render(http.StatusOK, "forgot-password", data)
		}
		if user.GPGPublicKey == "" {
			data.UsernameError = "user has no gpg public key"
			return c.Render(http.StatusOK, "forgot-password", data)
		}
		if user.GpgTwoFactorEnabled {
			data.UsernameError = "user has gpg two-factors enabled"
			return c.Render(http.StatusOK, "forgot-password", data)
		}

		if data.GpgMode {
			data.ToBeSignedMessage = generatePgpToBeSignedTokenMessage(user.ID, user.GPGPublicKey)

		} else {
			msg, err := generatePgpEncryptedTokenMessage(user.ID, user.GPGPublicKey)
			if err != nil {
				data.Error = err.Error()
				return c.Render(http.StatusOK, "forgot-password", data)
			}
			data.EncryptedMessage = msg
		}

		token := utils.GenerateToken32()
		partialRecoveryCache.SetD(token, PartialRecoveryItem{user.ID, RecoveryCaptchaCompleted})

		data.Token = token
		data.Step = 2
		return c.Render(http.StatusOK, "forgot-password", data)

	} else if formName == "step2" {
		// Receive and validate GPG code/signature
		data.Step = 2

		// Step2 is guarded by the "token" that must be valid
		token := c.Request().PostFormValue("token")
		item, found := partialRecoveryCache.Get(token)
		if !found {
			return c.Redirect(http.StatusFound, "/")
		}
		userID := item.UserID

		pgpToken, found := pgpTokenCache.Get(userID)
		if !found {
			return c.Redirect(http.StatusFound, "/")
		}

		data.GpgMode = utils.DoParseBool(c.Request().PostFormValue("gpg_mode"))
		if data.GpgMode {
			data.ToBeSignedMessage = c.Request().PostFormValue("to_be_signed_message")
			data.SignedMessage = c.Request().PostFormValue("signed_message")
			if !utils.PgpCheckSignMessage(pgpToken.PKey, pgpToken.Value, data.SignedMessage) {
				data.ErrorSignedMessage = "invalid signature"
				return c.Render(http.StatusOK, "forgot-password", data)
			}

		} else {
			data.EncryptedMessage = c.Request().PostFormValue("encrypted_message")
			data.Code = c.Request().PostFormValue("pgp_code")
			if data.Code != pgpToken.Value {
				data.ErrorCode = "invalid code"
				return c.Render(http.StatusOK, "forgot-password", data)
			}
		}

		pgpTokenCache.Delete(userID)
		partialRecoveryCache.SetD(token, PartialRecoveryItem{userID, RecoveryGpgValidated})

		data.Token = token
		data.Step = 3
		return c.Render(http.StatusOK, "forgot-password", data)

	} else if formName == "step3" {
		// Receive and validate new password
		data.Step = 3

		// Step3 is guarded by the "token" that must be valid
		token := c.Request().PostFormValue("token")
		item, found := partialRecoveryCache.Get(token)
		if !found {
			return c.Redirect(http.StatusFound, "/")
		}
		userID := item.UserID
		user, err := database.GetUserByID(userID)
		if err != nil {
			return c.Redirect(http.StatusFound, "/")
		}

		newPassword := c.Request().PostFormValue("newPassword")
		rePassword := c.Request().PostFormValue("rePassword")
		data.NewPassword = newPassword
		data.RePassword = rePassword

		hashedPassword, err := database.NewPasswordValidator(newPassword).CompareWith(rePassword).Hash()
		if err != nil {
			data.ErrorNewPassword = err.Error()
			return c.Render(http.StatusOK, "forgot-password", data)
		}

		if err := user.ChangePassword(hashedPassword); err != nil {
			logrus.Error(err)
		}
		database.CreateSecurityLog(user.ID, database.PasswordRecoverySecurityLog)

		partialRecoveryCache.Delete(token)
		c.SetCookie(hutils.DeleteCookie(hutils.WaitCookieName))

		return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Password reset done", Redirect: "/login"})
	}

	return c.Render(http.StatusOK, "flash", FlashResponse{"should not go here", "/login", "alert-danger"})
}

func NewsHandler(c echo.Context) error {
	var data newsData
	return c.Render(http.StatusOK, "news", data)
}

func ForumSearchHandler(c echo.Context) error {
	var data forumSearchData
	data.Search = c.QueryParam("search")

	if err := database.DB.Raw(`select m.uuid, snippet(fts5_forum_messages,-1, '[', ']', '...', 10) as snippet, t.uuid as thread_uuid, t.name as thread_name,
u.username as author,
u.chat_color as author_chat_color,
u.chat_font as author_chat_font,
mm.created_at as created_at
from fts5_forum_messages m
inner join forum_threads t on t.id = m.thread_id
-- Find message
LEFT JOIN forum_messages mm ON mm.uuid = m.uuid
-- Join author user
INNER JOIN users u ON u.id = mm.user_id
where fts5_forum_messages match ? and t.is_club = 0 order by rank limit 100`, data.Search).Scan(&data.ForumMessages).Error; err != nil {
		logrus.Error(err)
	}

	if err := database.DB.Raw(`select
t.*,
u.username as author,
u.chat_color as author_chat_color,
lu.username as last_msg_author,
lu.chat_color as last_msg_chat_color,
lu.chat_font as last_msg_chat_font,
m.created_at as last_msg_created_at,
mmm.replies_count
from fts5_forum_threads ft
inner join forum_threads t on t.id = ft.id 
-- Count replies
LEFT JOIN (SELECT mm.thread_id, COUNT(mm.id) as replies_count FROM forum_messages mm GROUP BY mm.thread_id) as mmm ON mmm.thread_id = t.id
-- Join author user
INNER JOIN users u ON u.id = t.user_id
-- Find last message for thread
LEFT JOIN forum_messages m ON m.thread_id = t.id AND m.id = (SELECT max(id) FROM forum_messages WHERE thread_id = t.id)
-- Join last message user
INNER JOIN users lu ON lu.id = m.user_id
where fts5_forum_threads match ? and t.is_club = 0 order by rank limit 100`, data.Search).Scan(&data.ForumThreads).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "forum-search", data)
}

func LinksHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data linksData
	resultsPerPage := int64(100)

	data.Categories, _ = database.GetCategories()

	data.Search = c.QueryParam("search")
	filterCategory := c.QueryParam("category")

	if filterCategory != "" {
		if filterCategory == "uncategorized" {
			database.DB.Raw(`SELECT l.*
FROM links l
LEFT JOIN links_categories_links cl ON cl.link_id = l.id
WHERE cl.link_id IS NULL AND l.deleted_at IS NULL
ORDER BY l.title COLLATE NOCASE ASC`).Scan(&data.Links)
			data.LinksCount = int64(len(data.Links))
		} else {
			database.DB.Raw(`SELECT l.*
FROM links_categories_links cl
INNER JOIN links l ON l.id = cl.link_id
WHERE cl.category_id = (SELECT id FROM links_categories WHERE name = ?) AND l.deleted_at IS NULL
ORDER BY l.title COLLATE NOCASE ASC`, filterCategory).Scan(&data.Links)
			data.LinksCount = int64(len(data.Links))
		}
	} else if data.Search != "" {
		if govalidator.IsURL(data.Search) {
			if searchedURL, err := url.Parse(data.Search); err == nil {
				h := searchedURL.Scheme + "://" + searchedURL.Hostname()
				var l database.Link
				query := database.DB
				if authUser.IsModerator() {
					query = query.Unscoped()
				}
				if err := query.First(&l, "url = ?", h).Error; err == nil {
					data.Links = append(data.Links, l)
				}
				data.LinksCount = int64(len(data.Links))
			}
		} else {
			if err := database.DB.Raw(`select l.id, l.uuid, l.url, l.title, l.description
from fts5_links l
where fts5_links match ?
ORDER BY rank, l.title COLLATE NOCASE ASC
LIMIT 100`, data.Search).Scan(&data.Links).Error; err != nil {
				logrus.Error(err)
			}
			data.LinksCount = int64(len(data.Links))
		}
	} else {
		wantedPage := utils.DoParseInt64(c.QueryParam("p"))

		query := database.DB.Table("links")
		query.Count(&data.LinksCount)

		page, maxPage := Paginate(resultsPerPage, wantedPage, data.LinksCount)

		query = database.DB.
			Order("title COLLATE NOCASE ASC").
			Offset((page - 1) * resultsPerPage).
			Limit(resultsPerPage)
		if err := query.Find(&data.Links).Error; err != nil {
			logrus.Error(err)
		}
		data.CurrentPage = page
		data.MaxPage = maxPage
	}

	// Get all links IDs
	linksIDs := make([]int64, 0)
	for _, l := range data.Links {
		linksIDs = append(linksIDs, l.ID)
	}
	// Keep pointers to links for fast access
	cache := make(map[int64]*database.Link)
	for i, l := range data.Links {
		cache[l.ID] = &data.Links[i]
	}
	// Get all mirrors for all links that we have
	var mirrors []database.LinksMirror
	database.DB.Raw(`select * from links_mirrors where link_id in (?)`, linksIDs).Scan(&mirrors)
	// Put mirrors in links
	for _, m := range mirrors {
		if l, ok := cache[m.LinkID]; ok {
			l.Mirrors = append(l.Mirrors, m)
		}
	}

	return c.Render(http.StatusOK, "links", data)
}

func LinksDownloadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	fileName := "dkf_links.csv"

	// Captcha for bigger files
	var data uploadsDownloadData
	data.CaptchaID, data.CaptchaImg = captcha.New()
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "captcha-required", data)
	}
	captchaID := c.Request().PostFormValue("captcha_id")
	captchaInput := c.Request().PostFormValue("captcha")
	if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
		data.ErrCaptcha = err.Error()
		return c.Render(http.StatusOK, "captcha-required", data)
	}

	// Keep track of user downloads
	if _, err := database.CreateDownload(authUser.ID, fileName); err != nil {
		logrus.Error(err)
	}

	links, _ := database.GetLinks()
	by := make([]byte, 0)
	buf := bytes.NewBuffer(by)
	w := csv.NewWriter(buf)
	_ = w.Write([]string{"UUID", "URL", "Title", "Description"})
	for _, link := range links {
		_ = w.Write([]string{link.UUID, link.URL, link.Title, link.Description})
	}
	w.Flush()
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	return c.Stream(http.StatusOK, "application/octet-stream", buf)
}

func LinkPgpDownloadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)

	pgpID := utils.DoParseInt64(c.Param("linkPgpID"))
	linkPgp, err := database.GetLinkPgpByID(pgpID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	fileName := linkPgp.Title + ".asc"

	// Keep track of user downloads
	if _, err := database.CreateDownload(authUser.ID, fileName); err != nil {
		logrus.Error(err)
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	return c.Stream(http.StatusOK, "application/octet-stream", strings.NewReader(linkPgp.PgpPublicKey))
}

func LinkHandler(c echo.Context) error {
	shorthand := c.Param("shorthand")
	linkUUID := c.Param("linkUUID")
	var data linkData
	var err error
	if shorthand != "" {
		data.Link, err = database.GetLinkByShorthand(shorthand)
	} else {
		data.Link, err = database.GetLinkByUUID(linkUUID)
	}
	if err != nil {
		return c.Redirect(http.StatusFound, "/links")
	}
	data.PgpKeys, _ = database.GetLinkPgps(data.Link.ID)
	data.Mirrors, _ = database.GetLinkMirrors(data.Link.ID)
	return c.Render(http.StatusOK, "link", data)
}

type CsvLink struct {
	URL   string
	Title string
}

func LinksUploadHandler(c echo.Context) error {
	var data linksUploadData
	if c.Request().Method == http.MethodPost {
		data.CsvStr = c.Request().PostFormValue("csv")
		getValidLinks := func() (out []CsvLink, err error) {
			r := csv.NewReader(strings.NewReader(data.CsvStr))
			records, err := r.ReadAll()
			if err != nil {
				return out, err
			}
			for idx, record := range records {
				link := strings.TrimSpace(strings.TrimRight(record[0], "/"))
				title := record[1]
				if !govalidator.Matches(link, `^https?://[a-z2-7]{56}\.onion$`) {
					return out, fmt.Errorf("invalid link %s", link)
				}
				if !govalidator.RuneLength(title, "0", "255") {
					return out, fmt.Errorf("title must have 255 characters max : record #%d", idx)
				}
				csvLink := CsvLink{
					URL:   link,
					Title: title,
				}
				out = append(out, csvLink)
			}
			return out, nil
		}
		csvLinks, err := getValidLinks()
		if err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "links-upload", data)
		}
		for _, csvLink := range csvLinks {
			_, err := database.CreateLink(csvLink.URL, csvLink.Title, "", "")
			if err != nil {
				logrus.Error(err)
			}
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	return c.Render(http.StatusOK, "links-upload", data)
}

func LinksReindexHandler(c echo.Context) error {
	if err := database.DB.Exec(`INSERT INTO fts5_links(fts5_links) VALUES('rebuild')`).Error; err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func NewLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, "/")
	}
	var data newLinkData
	if c.Request().Method == http.MethodPost {
		data.Link = c.Request().PostFormValue("link")
		data.Title = c.Request().PostFormValue("title")
		data.Description = c.Request().PostFormValue("description")
		data.Shorthand = c.Request().PostFormValue("shorthand")
		data.Categories = c.Request().PostFormValue("categories")
		data.Tags = c.Request().PostFormValue("tags")
		if !govalidator.Matches(data.Link, `^https?://[a-z2-7]{56}\.onion$`) {
			data.ErrorLink = "invalid link"
			return c.Render(http.StatusOK, "new-link", data)
		}
		if !govalidator.RuneLength(data.Title, "0", "255") {
			data.ErrorTitle = "title must have 255 characters max"
			return c.Render(http.StatusOK, "new-link", data)
		}
		if !govalidator.RuneLength(data.Description, "0", "1000") {
			data.ErrorCategories = "description must have 1000 characters max"
			return c.Render(http.StatusOK, "new-link", data)
		}
		if data.Shorthand != "" {
			if !govalidator.Matches(data.Shorthand, `^[\w-_]{3,50}$`) {
				data.ErrorLink = "invalid shorthand"
				return c.Render(http.StatusOK, "new-link", data)
			}
		}
		categoryRgx := regexp.MustCompile(`^\w{3,20}$`)
		var tagsStr, categoriesStr []string
		if data.Categories != "" {
			categoriesStr = strings.Split(strings.ToLower(data.Categories), ",")
			for _, category := range categoriesStr {
				category = strings.TrimSpace(category)
				if !categoryRgx.MatchString(category) {
					data.ErrorCategories = `invalid category "` + category + `"`
					return c.Render(http.StatusOK, "new-link", data)
				}
			}
		}
		if data.Tags != "" {
			tagsStr = strings.Split(strings.ToLower(data.Tags), ",")
			for _, tag := range tagsStr {
				tag = strings.TrimSpace(tag)
				if !categoryRgx.MatchString(tag) {
					data.ErrorTags = `invalid tag "` + tag + `"`
					return c.Render(http.StatusOK, "new-link", data)
				}
			}
		}
		//------------
		var categories []database.LinksCategory
		var tags []database.LinksTag
		for _, categoryStr := range categoriesStr {
			category, _ := database.CreateLinksCategory(categoryStr)
			categories = append(categories, category)
		}
		for _, tagStr := range tagsStr {
			tag, _ := database.CreateLinksTag(tagStr)
			tags = append(tags, tag)
		}
		link, err := database.CreateLink(data.Link, data.Title, data.Description, data.Shorthand)
		if err != nil {
			data.ErrorLink = "failed to create link"
			return c.Render(http.StatusOK, "new-link", data)
		}
		for _, category := range categories {
			_ = database.AddLinkCategory(link.ID, category.ID)
		}
		for _, tag := range tags {
			_ = database.AddLinkTag(link.ID, tag.ID)
		}
		return c.Redirect(http.StatusFound, "/links")
	}
	return c.Render(http.StatusOK, "new-link", data)
}

func RestoreLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, "/")
	}
	linkUUID := c.Param("linkUUID")
	var link database.Link
	if err := database.DB.Unscoped().First(&link, "uuid = ?", linkUUID).Error; err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	database.DB.Unscoped().Model(&database.Link{}).Where("id", link.ID).Update("deleted_at", nil)
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func EditLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, "/")
	}
	linkUUID := c.Param("linkUUID")
	link, err := database.GetLinkByUUID(linkUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	out, _ := database.GetLinkCategories(link.ID)
	categories := make([]string, 0)
	for _, el := range out {
		categories = append(categories, el.Name)
	}
	out1, err := database.GetLinkTags(link.ID)
	tags := make([]string, 0)
	for _, el := range out1 {
		tags = append(tags, el.Name)
	}
	var data editLinkData
	data.IsEdit = true
	data.Link = link.URL
	data.Title = link.Title
	data.Description = link.Description
	if link.Shorthand != nil {
		data.Shorthand = *link.Shorthand
	}
	data.Categories = strings.Join(categories, ",")
	data.Tags = strings.Join(tags, ",")
	data.Mirrors, _ = database.GetLinkMirrors(link.ID)
	data.LinkPgps, _ = database.GetLinkPgps(link.ID)
	//data.Categories = link

	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "createLink" {
			_ = database.DeleteLinkCategories(link.ID)
			_ = database.DeleteLinkTags(link.ID)

			data.Link = c.Request().PostFormValue("link")
			data.Title = c.Request().PostFormValue("title")
			data.Description = c.Request().PostFormValue("description")
			data.Shorthand = c.Request().PostFormValue("shorthand")
			data.Categories = c.Request().PostFormValue("categories")
			data.Tags = c.Request().PostFormValue("tags")
			if !govalidator.Matches(data.Link, `^https?://[a-z2-7]{56}\.onion$`) {
				data.ErrorLink = "invalid link"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if !govalidator.RuneLength(data.Title, "0", "255") {
				data.ErrorTitle = "title must have 255 characters max"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if !govalidator.RuneLength(data.Description, "0", "1000") {
				data.ErrorCategories = "description must have 1000 characters max"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if data.Shorthand != "" {
				if !govalidator.Matches(data.Shorthand, `^[\w-_]{3,50}$`) {
					data.ErrorLink = "invalid shorthand"
					return c.Render(http.StatusOK, "new-link", data)
				}
			}
			categoryRgx := regexp.MustCompile(`^\w{3,20}$`)
			var tagsStr, categoriesStr []string
			if data.Categories != "" {
				categoriesStr = strings.Split(strings.ToLower(data.Categories), ",")
				for _, category := range categoriesStr {
					category = strings.TrimSpace(category)
					if !categoryRgx.MatchString(category) {
						data.ErrorCategories = `invalid category "` + category + `"`
						return c.Render(http.StatusOK, "new-link", data)
					}
				}
			}
			if data.Tags != "" {
				tagsStr = strings.Split(strings.ToLower(data.Tags), ",")
				for _, tag := range tagsStr {
					tag = strings.TrimSpace(tag)
					if !categoryRgx.MatchString(tag) {
						data.ErrorTags = `invalid tag "` + tag + `"`
						return c.Render(http.StatusOK, "new-link", data)
					}
				}
			}
			//------------
			var categories []database.LinksCategory
			var tags []database.LinksTag
			for _, categoryStr := range categoriesStr {
				category, _ := database.CreateLinksCategory(categoryStr)
				categories = append(categories, category)
			}
			for _, tagStr := range tagsStr {
				tag, _ := database.CreateLinksTag(tagStr)
				tags = append(tags, tag)
			}
			link.URL = data.Link
			link.Title = data.Title
			link.Description = data.Description
			if data.Shorthand != "" {
				link.Shorthand = &data.Shorthand
			}
			if err := database.DB.Save(&link).Error; err != nil {
				if strings.Contains(err.Error(), "UNIQUE constraint failed: links.shorthand") {
					data.ErrorShorthand = "shorthand already used"
				} else {
					data.ErrorLink = "failed to update link"
				}
				return c.Render(http.StatusOK, "new-link", data)
			}
			for _, category := range categories {
				_ = database.AddLinkCategory(link.ID, category.ID)
			}
			for _, tag := range tags {
				_ = database.AddLinkTag(link.ID, tag.ID)
			}
			return c.Redirect(http.StatusFound, "/links")

		} else if formName == "createPgp" {
			data.PGPTitle = c.Request().PostFormValue("pgp_title")
			if !govalidator.RuneLength(data.PGPTitle, "3", "255") {
				data.ErrorPGPTitle = "title must have 3-255 characters"
				return c.Render(http.StatusOK, "new-link", data)
			}
			data.PGPDescription = c.Request().PostFormValue("pgp_description")
			data.PGPPublicKey = c.Request().PostFormValue("pgp_public_key")
			if _, err = database.CreateLinkPgp(link.ID, data.PGPTitle, data.PGPDescription, data.PGPPublicKey); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())

		} else if formName == "createMirror" {
			data.MirrorLink = c.Request().PostFormValue("mirror_link")
			if !govalidator.Matches(data.MirrorLink, `^https?://[a-z2-7]{56}\.onion$`) {
				data.ErrorMirrorLink = "invalid link"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if _, err = database.CreateLinkMirror(link.ID, data.MirrorLink); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}
		return c.Redirect(http.StatusFound, "/links")
	}

	return c.Render(http.StatusOK, "new-link", data)
}

func ForumHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data forumData
	data.ForumCategories, _ = database.GetForumCategories()
	data.ForumThreads, _ = database.GetPublicForumCategoryThreads(authUser.ID, 1)
	return c.Render(http.StatusOK, "forum", data)
}

func ForumCategoryHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	categorySlug := c.Param("categorySlug")
	var data forumCategoryData
	category, err := database.GetForumCategoryBySlug(categorySlug)
	if err != nil {
		return c.Redirect(http.StatusFound, "/forum")
	}
	data.ForumThreads, _ = database.GetPublicForumCategoryThreads(authUser.ID, category.ID)
	return c.Render(http.StatusOK, "forum", data)
}

func ThreadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := database.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data threadData
	data.Thread = thread

	wantedPage := utils.DoParseInt64(c.QueryParam("p"))

	query := database.DB.Table("forum_messages").Where("thread_id = ?", thread.ID)
	query.Count(&data.MessagesCount)

	page, maxPage := Paginate(ResultsPerPage, wantedPage, data.MessagesCount)

	query = database.DB.
		Order("id ASC").
		Where("thread_id = ?", thread.ID).
		Preload("User").
		Offset((page - 1) * ResultsPerPage).
		Limit(ResultsPerPage)
	if err := query.Find(&data.Messages).Error; err != nil {
		logrus.Error(err)
	}
	data.CurrentPage = page
	data.MaxPage = maxPage

	if authUser != nil {
		data.IsSubscribed = database.IsUserSubscribedToForumThread(authUser.ID, thread.ID)
		// Update read record
		database.DB.Create(database.ForumReadRecord{UserID: authUser.ID, ThreadID: thread.ID})
		database.DB.Table("forum_read_records").Where("user_id = ? AND thread_id = ?", authUser.ID, thread.ID).Update("read_at", time.Now())
	}

	return c.Render(http.StatusOK, "thread", data)
}

func GistHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	gistUUID := c.Param("gistUUID")
	gist, err := database.GetGistByUUID(gistUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data gistData
	data.Gist = gist

	if c.Request().Method == http.MethodPost {

		btnSubmit := c.Request().PostFormValue("btn_submit")
		if btnSubmit == "logout" {
			hutils.DeleteGistCookie(c, gist.UUID)
			return c.Redirect(http.StatusFound, "/")

		} else if btnSubmit == "delete_gist" {
			if gist.UserID == authUser.ID {
				if gist.Password != "" {
					hutils.DeleteGistCookie(c, gist.UUID)
				}
				if err := database.DB.Delete(&gist).Error; err != nil {
					logrus.Error(err)
				}
				return c.Redirect(http.StatusFound, "/")
			}
			return c.Redirect(http.StatusFound, "/")
		}

		password := c.Request().PostFormValue("password")
		hashedPassword := utils.Sha512([]byte(config.GistPasswordSalt + password))
		if hashedPassword != gist.Password {
			data.Error = "Invalid password"
			return c.Render(http.StatusOK, "gist-password", data)
		}
		hutils.CreateGistCookie(c, gist.UUID, hashedPassword)
		return c.Redirect(http.StatusFound, "/gists/"+gist.UUID)
	}

	if !gist.HasAccess(c) {
		return c.Render(http.StatusOK, "gist-password", data)
	}

	if strings.HasSuffix(gist.Name, ".go") {
		lexer := lexers.Match(gist.Name)
		style := styles.Get("monokai")
		formatter := html.New(html.Standalone(true), html.TabWidth(4), html.WithLineNumbers(true), html.LineNumbersInTable(true))
		iterator, _ := lexer.Tokenise(nil, gist.Content)
		buf := bytes.Buffer{}
		_ = formatter.Format(&buf, style, iterator)
		data.Highlighted = buf.String()
	}

	return c.Render(http.StatusOK, "gist", data)
}

func BhcliHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "bhcli", nil)
}

func TorchessHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "torchess", nil)
}

func CaptchaHelpHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "captcha-help", nil)
}

func WerewolfHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "werewolf", nil)
}

func ClubHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data clubData
	data.ActiveTab = "home"
	data.ForumThreads, _ = database.GetClubForumThreads(authUser.ID)
	return c.Render(http.StatusOK, "club.home", data)
}

func ThreadReplyHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Forum is temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}

	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := database.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data threadReplyData
	data.Thread = thread

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Message must have at least 3 characters"
			return c.Render(http.StatusOK, "thread-reply", data)
		}
		message := database.ForumMessage{UUID: database.ForumMessageUUID(uuid.New().String()), Message: data.Message, UserID: authUser.ID, ThreadID: thread.ID}
		if err := database.DB.Create(&message).Error; err != nil {
			logrus.Error(err)
		}
		// Send notifications
		subs, _ := database.GetUsersSubscribedToForumThread(thread.ID)
		for _, sub := range subs {
			if sub.UserID != authUser.ID {
				msg := fmt.Sprintf(`New reply in thread &quot;<a href="/t/%s#%s">%s</a>&quot;`, thread.UUID, message.UUID, thread.Name)
				database.CreateNotification(msg, sub.UserID)
			}
		}
		return c.Redirect(http.StatusFound, "/t/"+string(thread.UUID))
	}

	return c.Render(http.StatusOK, "thread-reply", data)
}

func ClubThreadReplyHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	threadID := database.ForumThreadID(utils.DoParseInt64(c.Param("threadID")))
	thread, err := database.GetForumThread(threadID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data clubNewThreadReplyData
	data.ActiveTab = "home"
	data.Thread = thread

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Message must have at least 3 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		message := database.ForumMessage{UUID: database.ForumMessageUUID(uuid.New().String()), Message: data.Message, UserID: authUser.ID, ThreadID: thread.ID}
		database.DB.Create(&message)
		return c.Redirect(http.StatusFound, "/club/threads/"+utils.FormatInt64(int64(thread.ID)))
	}

	return c.Render(http.StatusOK, "club.thread-reply", data)
}

func ThreadDeleteMessageHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Forum is temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}
	messageUUID := database.ForumMessageUUID(c.Param("messageUUID"))
	msg, err := database.GetForumMessageByUUID(messageUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if authUser.ID != msg.UserID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if !msg.CanEdit() && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	var data deleteForumMessageData
	data.Thread, err = database.GetForumThreadByID(msg.ThreadID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	data.Message = msg

	if c.Request().Method == http.MethodPost {
		if err := database.DeleteForumMessageByID(msg.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/t/"+string(data.Thread.UUID))
	}

	return c.Render(http.StatusOK, "thread-message-delete", data)
}

func LinkDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	linkUUID := c.Param("linkUUID")
	link, err := database.GetLinkByUUID(linkUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	var data deleteLinkData
	data.Link = link

	if c.Request().Method == http.MethodPost {
		if err := database.DeleteLinkByID(link.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/links")
	}

	return c.Render(http.StatusOK, "link-delete", data)
}

func LinkPgpDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	linkPgpID := utils.DoParseInt64(c.Param("linkPgpID"))
	linkPgp, err := database.GetLinkPgpByID(linkPgpID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	link, err := database.GetLinkByID(linkPgp.LinkID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	var data deleteLinkPgpData
	data.Link = link
	data.LinkPgp = linkPgp

	if c.Request().Method == http.MethodPost {
		if err := database.DeleteLinkPgpByID(linkPgp.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/links/"+link.UUID+"/edit")
	}

	return c.Render(http.StatusOK, "link-pgp-delete", data)
}

func LinkMirrorDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	linkMirrorID := utils.DoParseInt64(c.Param("linkMirrorID"))
	linkMirror, err := database.GetLinkMirrorByID(linkMirrorID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	link, err := database.GetLinkByID(linkMirror.LinkID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	var data deleteLinkMirrorData
	data.Link = link
	data.LinkMirror = linkMirror

	if c.Request().Method == http.MethodPost {
		if err := database.DeleteLinkMirrorByID(linkMirror.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/links/"+link.UUID+"/edit")
	}

	return c.Render(http.StatusOK, "link-mirror-delete", data)
}

func ThreadEditHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Forum is temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}
	if !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := database.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	var data editForumThreadData
	data.Thread = thread

	if c.Request().Method == http.MethodPost {
		thread.CategoryID = database.ForumCategoryID(utils.DoParseInt64(c.Request().PostFormValue("category_id")))
		thread.DoSave()
		return c.Redirect(http.StatusFound, "/forum")
	}

	return c.Render(http.StatusOK, "thread-edit", data)
}

func ThreadDeleteHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Forum is temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := database.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	var data deleteForumThreadData
	data.Thread = thread

	if c.Request().Method == http.MethodPost {
		if err := database.DeleteForumThreadByID(thread.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/forum")
	}

	return c.Render(http.StatusOK, "thread-delete", data)
}

func ThreadEditMessageHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Forum is temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	messageUUID := database.ForumMessageUUID(c.Param("messageUUID"))
	thread, err := database.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	msg, err := database.GetForumMessageByUUID(messageUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if msg.UserID != authUser.ID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/")
	}
	var data threadReplyData
	data.IsEdit = true
	data.Thread = thread
	data.Message = msg.Message

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "20000") {
			data.ErrorMessage = "Message must have 3 to 20k characters"
			return c.Render(http.StatusOK, "thread-reply", data)
		}
		msg.Message = data.Message
		msg.DoSave()
		return c.Redirect(http.StatusFound, "/t/"+string(thread.UUID))
	}

	return c.Render(http.StatusOK, "thread-reply", data)
}

func ClubThreadEditMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	threadID := database.ForumThreadID(utils.DoParseInt64(c.Param("threadID")))
	messageID := database.ForumMessageID(utils.DoParseInt64(c.Param("messageID")))
	thread, err := database.GetForumThread(threadID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	msg, err := database.GetForumMessage(messageID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if msg.UserID != authUser.ID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/")
	}
	var data clubNewThreadReplyData
	data.ActiveTab = "home"
	data.IsEdit = true
	data.Thread = thread
	data.Message = msg.Message

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Message must have at least 3 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		msg.Message = data.Message
		msg.DoSave()
		return c.Redirect(http.StatusFound, "/club/threads/"+utils.FormatInt64(int64(thread.ID)))
	}

	return c.Render(http.StatusOK, "club.thread-reply", data)
}

func NewThreadHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Forum is temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}
	var data newThreadData

	if c.Request().Method == http.MethodPost {
		data.ThreadName = c.Request().PostFormValue("thread_name")
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.ThreadName, "3", "255") {
			data.ErrorThreadName = "Thread name must have 3-255 characters"
			return c.Render(http.StatusOK, "new-thread", data)
		}
		if !govalidator.RuneLength(data.Message, "3", "20000") {
			data.ErrorMessage = "Thread message must have at least 3-20000 characters"
			return c.Render(http.StatusOK, "new-thread", data)
		}
		thread := database.ForumThread{UUID: database.ForumThreadUUID(uuid.New().String()), Name: data.ThreadName, UserID: authUser.ID, CategoryID: 1}
		database.DB.Create(&thread)
		message := database.ForumMessage{UUID: database.ForumMessageUUID(uuid.New().String()), Message: data.Message, UserID: authUser.ID, ThreadID: thread.ID}
		database.DB.Create(&message)
		_ = database.SubscribeToForumThread(authUser.ID, thread.ID)
		return c.Redirect(http.StatusFound, "/t/"+string(thread.UUID))
	}

	return c.Render(http.StatusOK, "new-thread", data)
}

func ClubNewThreadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data clubNewThreadData
	data.ActiveTab = "home"

	if c.Request().Method == http.MethodPost {
		data.ThreadName = c.Request().PostFormValue("thread_name")
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.ThreadName, "3", "255") {
			data.ErrorThreadName = "Thread name must have 3-255 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Thread name must have at least 3 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		thread := database.ForumThread{UUID: database.ForumThreadUUID(uuid.New().String()), Name: data.ThreadName, UserID: authUser.ID}
		database.DB.Create(&thread)
		message := database.ForumMessage{UUID: database.ForumMessageUUID(uuid.New().String()), Message: data.Message, UserID: authUser.ID, ThreadID: thread.ID}
		database.DB.Create(&message)
		return c.Redirect(http.StatusFound, "/club/threads/"+utils.FormatInt64(int64(thread.ID)))
	}

	return c.Render(http.StatusOK, "club.new-thread", data)
}

func ClubMembersHandler(c echo.Context) error {
	var data clubMembersData
	data.ActiveTab = "members"
	data.Members, _ = database.GetClubMembers()
	return c.Render(http.StatusOK, "club.members", data)
}

func ClubThreadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	threadID := database.ForumThreadID(utils.DoParseInt64(c.Param("threadID")))
	thread, err := database.GetForumThread(threadID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data clubThreadData
	data.ActiveTab = "home"
	data.Thread = thread
	data.Messages, _ = database.GetThreadMessages(threadID)

	// Update read record
	database.DB.Create(database.ForumReadRecord{UserID: authUser.ID, ThreadID: threadID})
	database.DB.Table("forum_read_records").Where("user_id = ? AND thread_id = ?", authUser.ID, threadID).Update("read_at", time.Now())

	return c.Render(http.StatusOK, "club.thread", data)
}

func VipHandler(c echo.Context) error {
	var data vipData
	data.ActiveTab = "home"
	data.UsersBadges, _ = database.GetUsersBadges()
	return c.Render(http.StatusOK, "vip.home", data)
}

func VipProjectsHandler(c echo.Context) error {
	var data vipData
	data.ActiveTab = "projects"
	return c.Render(http.StatusOK, "vip.projects", data)
}

func VipProjectsIPGrabberHandler(c echo.Context) error {
	var data vipData
	data.ActiveTab = "ip-grabber"
	return c.Render(http.StatusOK, "vip.ip-grabber", data)
}

func VipProjectsRustRansomwareHandler(c echo.Context) error {
	var data vipData
	data.ActiveTab = "rust-ransomware"
	return c.Render(http.StatusOK, "vip.rust-ransomware", data)
}

func VipProjectsMalwareDropperHandler(c echo.Context) error {
	var data vipData
	data.ActiveTab = "malware-dropper"
	return c.Render(http.StatusOK, "vip.malware-dropper", data)
}

func RoomsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data roomsData
	data.Rooms, _ = database.GetListedChatRooms(authUser.ID)
	return c.Render(http.StatusOK, "rooms", data)
}

func RedRoomHandler(c echo.Context) error {
	return chatHandler(c, true)
}

func ChatHandler(c echo.Context) error {
	return chatHandler(c, false)
}

func chatHandler(c echo.Context, redRoom bool) error {
	authUser := c.Get("authUser").(*database.User)
	var data chatData
	data.RedRoom = redRoom
	preventRefresh := utils.DoParseBool(c.QueryParam("r"))
	data.TogglePms = utils.DoParseInt64(c.QueryParam("pmonly"))
	data.ToggleMentions = utils.DoParseBool(c.QueryParam("mentionsOnly"))

	v := c.QueryParams()
	if preventRefresh {
		v.Set("r", "1")
	}
	if data.TogglePms != 0 {
		v.Set("pmonly", utils.FormatInt64(data.TogglePms))
	}
	if data.ToggleMentions {
		v.Set("mentionsOnly", "1")
	}
	if _, found := c.QueryParams()["ml"]; found {
		v.Set("ml", "1")
		data.Multiline = true
	}
	data.ChatQueryParams = "?" + v.Encode()

	if authUser == nil {
		if config.SignupEnabled.IsFalse() {
			return c.Render(http.StatusOK, "flash", FlashResponse{Message: "New signup are temporarily disabled", Redirect: "/", Type: "alert-danger"})
		}

		data.CaptchaID, data.CaptchaImg = captcha.New()
	}

	roomName := c.Param("roomName")
	if roomName == "" {
		roomName = "general"
	}
	room, err := database.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	if authUser != nil {
		data.DisplayTutorial = (room.ID < 5 || (room.IsListed && !room.IsProtected())) && !authUser.TutorialCompleted()
		if c.Request().Method == http.MethodGet && data.DisplayTutorial {
			authUser.ChatTutorialTime = time.Now()
			authUser.DoSave()
		}
	}

	if c.Request().Method == http.MethodPost {

		btnSubmit := c.Request().PostFormValue("btn_submit")
		if btnSubmit == "logout" {
			hutils.DeleteRoomCookie(c, int64(room.ID))
			return c.Redirect(http.StatusFound, "/chat")
		}

		formName := c.Request().PostFormValue("formName")
		if formName == "toggle-hb" {
			if authUser.CanSeeHB() {
				authUser.DisplayHellbanned = !authUser.DisplayHellbanned
				authUser.DoSave()
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())

		} else if formName == "toggle-m" {
			if authUser.IsModerator() {
				authUser.DisplayModerators = !authUser.DisplayModerators
				authUser.DoSave()
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())

		} else if formName == "toggle-ignored" {
			authUser.DisplayIgnored = !authUser.DisplayIgnored
			authUser.DoSave()
			return c.Redirect(http.StatusFound, c.Request().Referer())

		} else if formName == "afk" {
			authUser.AFK = !authUser.AFK
			authUser.DoSave()
			return c.Redirect(http.StatusFound, c.Request().Referer())
		} else if formName == "update-read-marker" {
			database.UpdateChatReadMarker(authUser.ID, room.ID)
			return c.Redirect(http.StatusFound, c.Request().Referer())

		} else if formName == "tutorialP1" {
			if authUser.ChatTutorial == 0 && time.Since(authUser.ChatTutorialTime) >= 14*time.Second {
				authUser.ChatTutorial = 1
				authUser.DoSave()
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		} else if formName == "tutorialP2" {
			if authUser.ChatTutorial == 1 && time.Since(authUser.ChatTutorialTime) >= 14*time.Second {
				authUser.ChatTutorial = 2
				authUser.DoSave()
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		} else if formName == "tutorialP3" {
			if authUser.ChatTutorial == 2 && time.Since(authUser.ChatTutorialTime) >= 14*time.Second {
				authUser.ChatTutorial = 3
				authUser.DoSave()
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}

		data.RoomPassword = c.Request().PostFormValue("password")
		if authUser == nil {
			data.GuestUsername = c.Request().PostFormValue("guest_username")
			captchaID := c.Request().PostFormValue("captcha_id")
			captchaInput := c.Request().PostFormValue("captcha")
			if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
				data.ErrCaptcha = err.Error()
				return c.Render(http.StatusOK, "chat-password", data)
			}

			if err := database.CanUseUsername(data.GuestUsername, false); err != nil {
				data.ErrGuestUsername = err.Error()
				return c.Render(http.StatusOK, "chat-password", data)
			}
		}
		key := utils.Sha256([]byte(config.RoomPasswordSalt + data.RoomPassword))[:32]
		hashedPassword := utils.Sha512([]byte(config.RoomPasswordSalt + data.RoomPassword))
		if hashedPassword != room.Password {
			data.Error = "Invalid room password"
			return c.Render(http.StatusOK, "chat-password", data)
		}

		if authUser == nil {
			password := utils.GenerateToken32()
			newUser, errs := database.CreateGuestUser(data.GuestUsername, password)
			if errs.HasError() {
				data.ErrGuestUsername = errs.Username
				return c.Render(http.StatusOK, "chat-password", data)
			}

			session, err := database.CreateSession(newUser.ID, c.Request().UserAgent())
			if err != nil {
				logrus.Error("Failed to create session : ", err)
			}
			c.SetCookie(createSessionCookie(session.Token))
		}

		hutils.CreateRoomCookie(c, int64(room.ID), hashedPassword, key)
		return c.Redirect(http.StatusFound, "/chat/"+room.Name)
	}

	if !room.HasAccess(c) {
		if room.IsProtected() {
			return c.Render(http.StatusOK, "chat-password", data)
		} else {
			return c.Redirect(http.StatusFound, "/chat")
		}
	}

	data.IsSubscribed = database.IsUserSubscribedToRoom(authUser.ID, room.ID)
	data.Room = room
	data.IsOfficialRoom = room.IsOfficialRoom()
	return c.Render(http.StatusOK, "chat", data)
}

func ChatHelpHandler(c echo.Context) error {
	var data chatHelpData
	return c.Render(http.StatusOK, "chat-help", data)
}

func RoomChatSettingsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data roomChatSettingsData
	roomName := c.Param("roomName")
	room, err := database.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if room.OwnerUserID == nil || *room.OwnerUserID != authUser.ID {
		return c.Redirect(http.StatusFound, "/")
	}
	data.Room = room

	if c.Request().Method == http.MethodPost {
		return c.Redirect(http.StatusFound, "/chat")
	}

	return c.Render(http.StatusOK, "chat-room-settings", data)
}

func ChatCreateRoomHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data chatCreateRoomData
	data.CaptchaID, data.CaptchaImg = captcha.New()
	data.IsEphemeral = true
	if c.Request().Method == http.MethodPost {
		data.RoomName = c.Request().PostFormValue("room_name")
		data.Password = c.Request().PostFormValue("password")
		data.IsListed = utils.DoParseBool(c.Request().PostFormValue("is_listed"))
		data.IsEphemeral = utils.DoParseBool(c.Request().PostFormValue("is_ephemeral"))
		if !govalidator.Matches(data.RoomName, "^[a-zA-Z0-9_]{3,50}$") {
			data.ErrorRoomName = "invalid room name"
			return c.Render(http.StatusOK, "chat-create-room", data)
		}
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, "chat-create-room", data)
		}
		passwordHash := ""
		if data.Password != "" {
			passwordHash = utils.Sha512([]byte(config.RoomPasswordSalt + data.Password))
		}
		if _, err := database.CreateRoom(data.RoomName, passwordHash, authUser.ID, data.IsListed); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "chat-create-room", data)
		}
		return c.Redirect(http.StatusFound, "/chat/"+data.RoomName)
	}
	return c.Render(http.StatusOK, "chat-create-room", data)
}

func ShopHandler(c echo.Context) error {
	getImgStr := func(img image.Image) string {
		buf := bytes.NewBuffer([]byte(""))
		_ = png.Encode(buf, img)
		return base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	authUser := c.Get("authUser").(*database.User)
	var data shopData
	invoice, err := database.CreateXmrInvoice(authUser.ID, 1)
	if err != nil {
		logrus.Error(err)
	}
	b, _ := invoice.GetImage()
	data.Img = getImgStr(b)
	data.Invoice = invoice

	return c.Render(http.StatusOK, "shop", data)
}

// SettingsChatHandler ...
func SettingsChatHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsChatData
	data.ActiveTab = "chat"
	data.AllFonts = utils.GetFonts()
	data.ChatColor = authUser.ChatColor
	data.ChatFont = authUser.ChatFont
	data.ChatItalic = authUser.ChatItalic
	data.ChatBold = authUser.ChatBold
	data.DateFormat = authUser.DateFormat
	data.ChatReadMarkerEnabled = authUser.ChatReadMarkerEnabled
	data.ChatReadMarkerColor = authUser.ChatReadMarkerColor
	data.ChatReadMarkerSize = authUser.ChatReadMarkerSize
	data.DisplayHellbanned = authUser.DisplayHellbanned
	data.DisplayModerators = authUser.DisplayModerators
	data.DisplayDeleteButton = authUser.DisplayDeleteButton
	data.DisplayKickButton = authUser.DisplayKickButton
	data.DisplayHellbanButton = authUser.DisplayHellbanButton
	data.HideRightColumn = authUser.HideRightColumn
	data.ChatBarAtBottom = authUser.ChatBarAtBottom
	data.AutocompleteCommandsEnabled = authUser.AutocompleteCommandsEnabled
	data.AfkIndicatorEnabled = authUser.AfkIndicatorEnabled
	data.HideIgnoredUsersFromList = authUser.HideIgnoredUsersFromList
	data.RefreshRate = authUser.RefreshRate
	data.NotifyNewMessage = authUser.NotifyNewMessage
	data.NotifyTagged = authUser.NotifyTagged
	data.NotifyPmmed = authUser.NotifyPmmed
	data.NotifyNewMessageSound = authUser.NotifyNewMessageSound
	data.NotifyTaggedSound = authUser.NotifyTaggedSound
	data.NotifyPmmedSound = authUser.NotifyPmmedSound
	data.Theme = authUser.Theme
	data.NotifyChessGames = authUser.NotifyChessGames
	data.NotifyChessMove = authUser.NotifyChessMove

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat", data)
	}

	// POST
	formName := c.FormValue("formName")
	if formName == "changeSettings" {
		return changeSettingsForm(c, data)
	}
	return c.Render(http.StatusOK, "settings.chat", data)
}

func SettingsChatPMHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsChatPMData
	data.ActiveTab = "chat"
	data.PmMode = authUser.PmMode
	data.BlockNewUsersPm = authUser.BlockNewUsersPm
	data.WhitelistedUsers, _ = database.GetPmWhitelistedUsers(authUser.ID)
	data.BlacklistedUsers, _ = database.GetPmBlacklistedUsers(authUser.ID)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat-pm", data)
	}

	// POST
	formName := c.Request().PostFormValue("formName")

	if formName == "addWhitelist" {
		data.AddWhitelist = strings.TrimSpace(c.Request().PostFormValue("username"))
		user, err := database.GetUserByUsername(data.AddWhitelist)
		if err != nil {
			data.Error = "username not found"
			return c.Render(http.StatusOK, "settings.chat-pm", data)
		}
		database.AddWhitelistedUser(authUser.ID, user.ID)
		return c.Redirect(http.StatusFound, c.Request().Referer())

	} else if formName == "rmWhitelist" {
		userID := dutils.DoParseUserID(c.Request().PostFormValue("userID"))
		database.RmWhitelistedUser(authUser.ID, userID)
		return c.Redirect(http.StatusFound, c.Request().Referer())

	} else if formName == "addBlacklist" {
		data.AddBlacklist = strings.TrimSpace(c.Request().PostFormValue("username"))
		user, err := database.GetUserByUsername(data.AddBlacklist)
		if err != nil {
			data.Error = "username not found"
			return c.Render(http.StatusOK, "settings.chat-pm", data)
		}
		database.AddBlacklistedUser(authUser.ID, user.ID)
		return c.Redirect(http.StatusFound, c.Request().Referer())

	} else if formName == "rmBlacklist" {
		userID := dutils.DoParseUserID(c.Request().PostFormValue("userID"))
		database.RmBlacklistedUser(authUser.ID, userID)
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	data.PmMode = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("pm_mode")), 0, 1)
	authUser.BlockNewUsersPm = utils.DoParseBool(c.Request().PostFormValue("block_new_users_pm"))
	authUser.PmMode = data.PmMode
	authUser.DoSave()
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func SettingsChatIgnoreHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsChatIgnoreData
	data.ActiveTab = "chat"
	data.PmMode = authUser.PmMode
	data.IgnoredUsers, _ = database.GetIgnoredUsers(authUser.ID)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat-ignore", data)
	}

	// POST
	formName := c.Request().PostFormValue("formName")

	if formName == "addIgnored" {
		data.AddIgnored = strings.TrimSpace(c.Request().PostFormValue("username"))
		user, err := database.GetUserByUsername(data.AddIgnored)
		if err != nil {
			data.Error = "username not found"
			return c.Render(http.StatusOK, "settings.chat-ignore", data)
		}
		database.IgnoreUser(authUser.ID, user.ID)
		return c.Redirect(http.StatusFound, c.Request().Referer())

	} else if formName == "rmIgnored" {
		userID := dutils.DoParseUserID(c.Request().PostFormValue("userID"))
		database.UnIgnoreUser(authUser.ID, userID)
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	data.PmMode = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("pm_mode")), 0, 1)
	authUser.PmMode = data.PmMode
	authUser.DoSave()
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func SettingsChatSnippetsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsChatSnippetsData
	data.ActiveTab = "snippets"
	data.Snippets, _ = database.GetUserSnippets(authUser.ID)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat-snippets", data)
	}

	// POST
	formName := c.Request().PostFormValue("formName")

	if formName == "addSnippet" {
		data.Name = strings.TrimSpace(c.Request().PostFormValue("name"))
		data.Text = strings.TrimSpace(c.Request().PostFormValue("text"))
		if len(data.Snippets) >= 20 {
			data.Error = "snippets limit reached"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if !govalidator.Matches(data.Name, `^\w{1,20}$`) {
			data.Error = "name must match : ^\\w{1,20}$"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if !govalidator.StringLength(data.Name, "1", "20") {
			data.Error = "name must be 1-20 characters"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if !govalidator.StringLength(data.Text, "1", "255") {
			data.Error = "text must be 1-255 characters"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if _, err := database.CreateSnippet(authUser.ID, data.Name, data.Text); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())

	} else if formName == "rmSnippet" {
		snippetName := c.Request().PostFormValue("snippetName")
		database.DeleteSnippet(authUser.ID, snippetName)
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func SettingsUploadsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsUploadsData
	data.ActiveTab = "uploads"
	data.Files, _ = database.GetUserUploads(authUser.ID)
	for _, f := range data.Files {
		data.TotalSize += f.FileSize
	}

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.uploads", data)
	}

	// POST
	formName := c.FormValue("formName")
	if formName == "deleteUpload" {
		fileName := c.Request().PostFormValue("file_name")
		file, err := database.GetUploadByFileName(fileName)
		if err != nil {
			return c.Redirect(http.StatusFound, "/")
		}
		if authUser.ID != file.UserID {
			return c.Redirect(http.StatusFound, "/")
		}
		if err := os.Remove(filepath.Join("uploads", file.FileName)); err != nil {
			logrus.Error(err.Error())
		}
		if err := database.DB.Delete(&file).Error; err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	return c.Render(http.StatusOK, "settings.uploads", data)
}

func SettingsPublicNotesHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}
	var data settingsPublicNotesData
	data.ActiveTab = "notes"
	data.Notes, _ = database.GetUserPublicNotes(authUser.ID)

	if c.Request().Method == http.MethodPost {
		notes := c.Request().PostFormValue("public_notes")
		if err := database.SetUserPublicNotes(authUser.ID, notes); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "settings.public-notes", data)
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	return c.Render(http.StatusOK, "settings.public-notes", data)
}

func SettingsPrivateNotesHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: c.Request().Referer(), Type: "alert-danger"})
	}
	var data settingsPrivateNotesData
	data.ActiveTab = "notes"
	data.Notes, _ = database.GetUserPrivateNotes(authUser.ID)

	if c.Request().Method == http.MethodPost {
		notes := c.Request().PostFormValue("private_notes")
		if err := database.SetUserPrivateNotes(authUser.ID, notes); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "settings.private-notes", data)
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	return c.Render(http.StatusOK, "settings.private-notes", data)
}

func SettingsInboxHandler(c echo.Context) error {
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	authUser := c.Get("authUser").(*database.User)
	var data settingsInboxData
	data.ActiveTab = "inbox"
	// Do not fetch inboxes & notifications if logged in under duress
	if !authUser.IsUnderDuress {
		global.DeleteUserNotificationCount(authUser.ID)
		data.ChatMessages, _ = database.GetUserChatInboxMessages(authUser.ID)
		data.Notifications, _ = database.GetUserNotifications(authUser.ID)
		data.SessionNotifications, _ = database.GetUserSessionNotifications(authCookie.Value)
	}
	for _, m := range data.ChatMessages {
		data.Notifs = append(data.Notifs, InboxTmp{IsNotif: false, ChatInboxMessage: m})
	}
	for _, m := range data.Notifications {
		data.Notifs = append(data.Notifs, InboxTmp{IsNotif: true, Notification: m})
	}
	for _, m := range data.SessionNotifications {
		data.Notifs = append(data.Notifs, InboxTmp{IsNotif: true, SessionNotification: m})
	}
	sort.Slice(data.Notifs, func(i, j int) bool {
		a := data.Notifs[i]
		b := data.Notifs[j]
		var tsa time.Time
		var tsb time.Time
		if a.Notification.ID != 0 {
			tsa = a.Notification.CreatedAt
		} else if a.SessionNotification.ID != 0 {
			tsa = a.SessionNotification.CreatedAt
		} else {
			tsa = a.ChatInboxMessage.CreatedAt
		}
		if b.Notification.ID != 0 {
			tsb = b.Notification.CreatedAt
		} else if b.SessionNotification.ID != 0 {
			tsb = b.SessionNotification.CreatedAt
		} else {
			tsb = b.ChatInboxMessage.CreatedAt
		}
		return tsa.After(tsb)
	})
	return c.Render(http.StatusOK, "settings.inbox", data)
}

func SettingsInboxSentHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsInboxSentData
	data.ActiveTab = "inbox"
	// Do not fetch inboxes & notifications if logged in under duress
	if !authUser.IsUnderDuress {
		data.ChatInboxSent, _ = database.GetUserChatInboxMessagesSent(authUser.ID)
	}
	return c.Render(http.StatusOK, "settings.inbox-sent", data)
}

func SettingsSecurityHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsSecurityData
	data.ActiveTab = "security"
	data.Logs, _ = database.GetSecurityLogs(authUser.ID)
	return c.Render(http.StatusOK, "settings.security", data)
}

func SettingsSessionsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsSessionsData
	data.ActiveTab = "sessions"
	sessions := database.GetActiveUserSessions(authUser.ID)
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	for _, session := range sessions {
		s := WrapperSession{Session: session}
		if authCookie.Value == s.Token {
			s.CurrentSession = true
		}
		data.Sessions = append(data.Sessions, s)
	}

	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "revoke_all_other_sessions" {
			_ = database.DeleteUserOtherSessions(authUser.ID, authCookie.Value)
		} else {
			sessionToken := c.Request().PostFormValue("sessionToken")
			_ = database.DeleteUserSessionByToken(authUser.ID, sessionToken)
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	return c.Render(http.StatusOK, "settings.sessions", data)
}

// SettingsAccountHandler ...
func SettingsAccountHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsAccountData
	data.AccountTooYoungErrorString = hutils.AccountTooYoungErr.Error()
	data.ActiveTab = "account"
	data.Username = authUser.Username
	data.Email = authUser.Email
	data.LastSeenPublic = authUser.LastSeenPublic
	data.TerminateAllSessionsOnLogout = authUser.TerminateAllSessionsOnLogout
	data.Website = authUser.Website

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.account", data)
	}

	// POST
	formName := c.FormValue("formName")
	switch formName {
	case "changeUsername":
		return changeUsernameForm(c, data)
	case "editProfile":
		return editProfileForm(c, data)
	case "changeAvatar":
		return changeAvatarForm(c, data)
	default:
		return c.Render(http.StatusOK, "settings.account", data)
	}
}

func SettingsPasswordHandler(c echo.Context) error {
	var data settingsPasswordData
	data.ActiveTab = "password"

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.password", data)
	}

	// POST
	formName := c.FormValue("formName")
	switch formName {
	case "changePassword":
		return changePasswordForm(c, data)
	case "changeDuressPassword":
		return changeDuressPasswordForm(c, data)
	default:
		return c.Render(http.StatusOK, "settings.password", data)
	}
}

func SettingsSecretPhraseHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsSecretPhraseData
	data.ActiveTab = "secretPhrase"
	data.SecretPhrase = string(authUser.SecretPhrase)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	// POST
	currentPassword := c.Request().PostFormValue("currentPassword")
	secretPhrase := c.Request().PostFormValue("secretPhrase")
	data.CurrentPassword = currentPassword
	data.SecretPhrase = secretPhrase

	if len(currentPassword) == 0 {
		data.ErrorCurrentPassword = "This field is required"
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	if !govalidator.RuneLength(secretPhrase, "3", "50") {
		data.ErrorSecretPhrase = "secret phrase must have between 3 and 50 characters"
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	if !authUser.CheckPassword(currentPassword) {
		data.ErrorCurrentPassword = "Invalid password"
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	authUser.SecretPhrase = database.EncryptedString(secretPhrase)
	authUser.DoSave()

	database.CreateSecurityLog(authUser.ID, database.ChangeSecretPhraseSecurityLog)
	return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Secret phrase changed successfully", Redirect: c.Request().Referer()})
}

// SettingsInvitationsHandler ...
func SettingsInvitationsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsInvitationsData
	data.ActiveTab = "invitations"
	data.DkfOnion = config.DkfOnion

	if c.Request().Method == http.MethodPost {
		if _, err := database.CreateInvitation(authUser.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	data.Invitations, _ = database.GetUserUnusedInvitations(authUser.ID)
	return c.Render(http.StatusOK, "settings.invitations", data)
}

// SettingsWebsiteHandler ...
func SettingsWebsiteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsWebsiteData
	data.ActiveTab = "website"
	settings := database.GetSettings()
	data.SignupEnabled = settings.SignupEnabled
	data.ForumEnabled = settings.ForumEnabled
	data.SilentSelfKick = settings.SilentSelfKick
	if c.Request().Method == http.MethodPost {
		settings.SignupEnabled = utils.DoParseBool(c.Request().PostFormValue("signupEnabled"))
		settings.ForumEnabled = utils.DoParseBool(c.Request().PostFormValue("forumEnabled"))
		settings.SilentSelfKick = utils.DoParseBool(c.Request().PostFormValue("silentSelfKick"))
		_ = settings.Save()
		config.SignupEnabled.Store(settings.SignupEnabled)
		config.ForumEnabled.Store(settings.ForumEnabled)
		config.SilentSelfKick.Store(settings.SilentSelfKick)
		database.NewAudit(*authUser, fmt.Sprintf("website settings, signup: %t, forum: %t, sk: %t",
			settings.SignupEnabled, settings.ForumEnabled, settings.SilentSelfKick))
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	return c.Render(http.StatusOK, "settings.website", data)
}

func editProfileForm(c echo.Context, data settingsAccountData) error {
	authUser := c.Get("authUser").(*database.User)

	email := c.Request().PostFormValue("email")
	website := c.Request().PostFormValue("website")
	lastSeenPublic := utils.DoParseBool(c.Request().PostFormValue("last_seen_public"))
	terminateAllSessionsOnLogout := utils.DoParseBool(c.Request().PostFormValue("terminate_all_sessions_on_logout"))
	data.Email = email
	data.Website = website
	data.LastSeenPublic = lastSeenPublic
	data.TerminateAllSessionsOnLogout = terminateAllSessionsOnLogout

	if data.Email != "" && !govalidator.IsEmail(data.Email) {
		data.ErrorEmail = "invalid email"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	if data.Website != "" && !govalidator.IsURL(data.Website) {
		data.ErrorWebsite = "invalid website"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	authUser.Website = data.Website
	authUser.Email = data.Email
	authUser.LastSeenPublic = data.LastSeenPublic
	authUser.TerminateAllSessionsOnLogout = data.TerminateAllSessionsOnLogout
	authUser.DoSave()

	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Profile changed successfully", Redirect: c.Request().Referer()})
}

func changeAvatarForm(c echo.Context, data settingsAccountData) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUpload() {
		data.ErrorAvatar = hutils.AccountTooYoungErr.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	if err := c.Request().ParseMultipartForm(1024 * 1024 /* 1 MB */); err != nil {
		data.ErrorAvatar = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	file, handler, err := c.Request().FormFile("avatar")
	if err != nil {
		data.ErrorAvatar = "Failed to get avatar: " + err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	defer file.Close()
	if handler.Size > 300<<10 {
		data.ErrorAvatar = "The maximum file size for avatars is 300 KB"
		return c.Render(http.StatusOK, "settings.account", data)
	}
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		data.ErrorAvatar = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	filetype := http.DetectContentType(fileBytes)
	if filetype != "image/jpeg" && filetype != "image/png" {
		data.ErrorAvatar = "The provided file format is not allowed. Please upload a JPEG or PNG image"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	// Validate image type and determine extension
	var ext string
	switch handler.Header.Get("Content-Type") {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	default:
		data.ErrorAvatar = "Image must be JPEG, PNG, or GIF."
		return c.Render(http.StatusOK, "settings.account", data)
	}

	im, _, err := image.DecodeConfig(bytes.NewReader(fileBytes))
	if err != nil {
		data.ErrorAvatar = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	if im.Width > 120 || im.Height > 120 {
		data.ErrorAvatar = "The maximum dimensions for avatars are: 120x120 pixels"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	if ext == ".jpg" {
		fileBytes, err = utils.ReencodeJpg(fileBytes)
	} else if ext == ".png" {
		fileBytes, err = utils.ReencodePng(fileBytes)
	}
	if err != nil {
		data.Error = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	authUser.SetAvatar(fileBytes)
	authUser.DoSave()
	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Avatar changed successfully", Redirect: c.Request().Referer()})
}

func changeUsernameForm(c echo.Context, data settingsAccountData) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanChangeUsername {
		data.ErrorUsername = "Not allowed to change your username"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	username := c.Request().PostFormValue("username")
	data.Username = username

	if username == authUser.Username {
		data.ErrorUsername = "username did not change"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	if _, err := database.ValidateUsername(username, false); err != nil {
		data.ErrorUsername = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	if strings.ToLower(username) != strings.ToLower(authUser.Username) {
		if database.IsUsernameAlreadyTaken(username) {
			data.ErrorUsername = "Username already taken"
			return c.Render(http.StatusOK, "settings.account", data)
		}
	}

	managers.ActiveUsers.RemoveUser(authUser.ID)
	authUser.Username = username
	if err := database.DB.Save(authUser).Error; err != nil {
		logrus.Error(err)
		data.ErrorUsername = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	database.CreateSecurityLog(authUser.ID, database.UsernameChangedSecurityLog)
	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Username changed successfully", Redirect: c.Request().Referer()})
}

func changeSettingsForm(c echo.Context, data settingsChatData) error {
	authUser := c.Get("authUser").(*database.User)

	data.RefreshRate = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("refresh_rate")), 5, 60)
	data.ChatColor = c.Request().PostFormValue("chat_color")
	data.ChatFont = utils.DoParseInt64(c.Request().PostFormValue("chat_font"))
	data.ChatBold = utils.DoParseBool(c.Request().PostFormValue("chat_bold"))
	data.DateFormat = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("date_format")), 0, 4)
	data.ChatReadMarkerEnabled = utils.DoParseBool(c.Request().PostFormValue("chat_read_marker_enabled"))
	data.ChatReadMarkerColor = c.Request().PostFormValue("chat_read_marker_color")
	data.ChatReadMarkerSize = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("chat_read_marker_size")), 1, 5)
	data.DisplayHellbanned = utils.DoParseBool(c.Request().PostFormValue("display_hellbanned"))
	data.DisplayModerators = utils.DoParseBool(c.Request().PostFormValue("display_moderators"))
	data.DisplayKickButton = utils.DoParseBool(c.Request().PostFormValue("display_kick_button"))
	data.DisplayDeleteButton = utils.DoParseBool(c.Request().PostFormValue("display_delete_button"))
	data.DisplayHellbanButton = utils.DoParseBool(c.Request().PostFormValue("display_hellban_button"))
	data.HideIgnoredUsersFromList = utils.DoParseBool(c.Request().PostFormValue("hide_ignored_users_from_list"))
	data.HideRightColumn = utils.DoParseBool(c.Request().PostFormValue("hide_right_column"))
	data.ChatBarAtBottom = utils.DoParseBool(c.Request().PostFormValue("chat_bar_at_bottom"))
	data.AutocompleteCommandsEnabled = utils.DoParseBool(c.Request().PostFormValue("autocomplete_commands_enabled"))
	data.AfkIndicatorEnabled = utils.DoParseBool(c.Request().PostFormValue("afk_indicator_enabled"))
	data.ChatItalic = utils.DoParseBool(c.Request().PostFormValue("chat_italic"))
	data.NotifyNewMessage = utils.DoParseBool(c.Request().PostFormValue("notify_new_message"))
	data.NotifyTagged = utils.DoParseBool(c.Request().PostFormValue("notify_tagged"))
	data.NotifyPmmed = utils.DoParseBool(c.Request().PostFormValue("notify_pmmed"))
	data.Theme = utils.DoParseInt64(c.Request().PostFormValue("theme"))
	data.NotifyChessGames = utils.DoParseBool(c.Request().PostFormValue("notify_chess_games"))
	data.NotifyChessMove = utils.DoParseBool(c.Request().PostFormValue("notify_chess_move"))
	//data.NotifyNewMessageSound = utils.DoParseInt64(c.Request().PostFormValue("notify_new_message_sound"))
	//data.NotifyTaggedSound = utils.DoParseInt64(c.Request().PostFormValue("notify_tagged_sound"))
	//data.NotifyPmmedSound = utils.DoParseInt64(c.Request().PostFormValue("notify_pmmed_sound"))
	colorRgx := regexp.MustCompile(`(#(?:[0-9a-f]{2}){2,4}|#[0-9a-f]{3}|(?:rgba?|hsla?)\((?:\d+%?(?:deg|rad|grad|turn)?(?:,|\s)+){2,3}[\s/]*[\d.]+%?\))`)
	if !colorRgx.MatchString(data.ChatColor) {
		data.Error = "Invalid color format"
		return c.Render(http.StatusOK, "settings.chat", data)
	}
	if !colorRgx.MatchString(data.ChatReadMarkerColor) {
		data.Error = "Invalid marker color format"
		return c.Render(http.StatusOK, "settings.chat", data)
	}
	authUser.RefreshRate = data.RefreshRate
	if authUser.CanChangeColor {
		authUser.ChatColor = data.ChatColor
	}
	authUser.ChatFont = data.ChatFont
	authUser.ChatItalic = data.ChatItalic
	authUser.ChatBold = data.ChatBold
	authUser.DateFormat = data.DateFormat
	authUser.ChatReadMarkerEnabled = data.ChatReadMarkerEnabled
	authUser.ChatReadMarkerColor = data.ChatReadMarkerColor
	authUser.ChatReadMarkerSize = data.ChatReadMarkerSize
	authUser.DisplayHellbanned = data.DisplayHellbanned
	authUser.DisplayModerators = data.DisplayModerators
	authUser.DisplayDeleteButton = data.DisplayDeleteButton
	authUser.DisplayKickButton = data.DisplayKickButton
	authUser.DisplayHellbanButton = data.DisplayHellbanButton
	authUser.HideIgnoredUsersFromList = data.HideIgnoredUsersFromList
	authUser.HideRightColumn = data.HideRightColumn
	authUser.ChatBarAtBottom = data.ChatBarAtBottom
	authUser.AutocompleteCommandsEnabled = data.AutocompleteCommandsEnabled
	authUser.AfkIndicatorEnabled = data.AfkIndicatorEnabled
	authUser.NotifyNewMessage = data.NotifyNewMessage
	authUser.NotifyTagged = data.NotifyTagged
	authUser.NotifyPmmed = data.NotifyPmmed
	authUser.NotifyChessGames = data.NotifyChessGames
	authUser.NotifyChessMove = data.NotifyChessMove
	authUser.Theme = data.Theme
	//authUser.NotifyNewMessageSound = data.NotifyNewMessageSound
	//authUser.NotifyTaggedSound = data.NotifyTaggedSound
	//authUser.NotifyPmmedSound = data.NotifyPmmedSound
	if err := database.DB.Save(authUser).Error; err != nil {
		logrus.Error(err)
		data.Error = err.Error()
		return c.Render(http.StatusOK, "settings.chat", data)
	}

	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Settings changed successfully", Redirect: c.Request().Referer()})
}

func changePasswordForm(c echo.Context, data settingsPasswordData) error {
	authUser := c.Get("authUser").(*database.User)
	oldPassword := c.Request().PostFormValue("oldPassword")
	newPassword := c.Request().PostFormValue("newPassword")
	rePassword := c.Request().PostFormValue("rePassword")
	data.OldPassword = oldPassword
	data.NewPassword = newPassword
	data.RePassword = rePassword

	if len(oldPassword) == 0 {
		data.ErrorOldPassword = "This field is required"
		return c.Render(http.StatusOK, "settings.password", data)
	}

	if len(newPassword) > 0 || len(rePassword) > 0 {
		hashedPassword, err := database.NewPasswordValidator(newPassword).CompareWith(rePassword).Hash()
		if err != nil {
			data.ErrorNewPassword = err.Error()
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if !authUser.CheckPassword(oldPassword) {
			data.ErrorOldPassword = "Invalid password"
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if err := authUser.ChangePassword(hashedPassword); err != nil {
			logrus.Error(err)
		}
		c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
		database.CreateSecurityLog(authUser.ID, database.ChangePasswordSecurityLog)
		return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Password changed successfully", Redirect: "/login"})
	}

	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func changeDuressPasswordForm(c echo.Context, data settingsPasswordData) error {
	authUser := c.Get("authUser").(*database.User)
	oldDuressPassword := c.Request().PostFormValue("oldDuressPassword")
	newDuressPassword := c.Request().PostFormValue("newDuressPassword")
	reDuressPassword := c.Request().PostFormValue("reDuressPassword")
	data.OldDuressPassword = oldDuressPassword
	data.NewDuressPassword = newDuressPassword
	data.ReDuressPassword = reDuressPassword

	if len(oldDuressPassword) == 0 {
		data.ErrorOldDuressPassword = "This field is required"
		return c.Render(http.StatusOK, "settings.password", data)
	}

	if len(newDuressPassword) > 0 || len(reDuressPassword) > 0 {
		hashedPassword, err := database.NewPasswordValidator(newDuressPassword).CompareWith(reDuressPassword).Hash()
		if err != nil {
			data.ErrorNewDuressPassword = err.Error()
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if !authUser.CheckPassword(oldDuressPassword) {
			data.ErrorOldDuressPassword = "Invalid password"
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if err := authUser.ChangeDuressPassword(hashedPassword); err != nil {
			logrus.Error(err)
		}
		c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
		database.CreateSecurityLog(authUser.ID, database.ChangeDuressPasswordSecurityLog)
		return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Password changed successfully", Redirect: "/login"})
	}

	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func ChatDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data chatDeleteData
	roomName := c.Param("roomName")
	room, err := database.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if room.OwnerUserID == nil || *room.OwnerUserID != authUser.ID {
		return c.Redirect(http.StatusFound, "/")
	}
	data.Room = room

	if c.Request().Method == http.MethodPost {
		if room.IsProtected() {
			hutils.DeleteRoomCookie(c, int64(room.ID))
		}
		room.Name = room.Name + "_" + utils.FormatInt64(time.Now().Unix())
		room.DoSave()
		if err := database.DB.Delete(&room).Error; err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/chat")
	}

	return c.Render(http.StatusOK, "chat-delete", data)
}

type Paginator struct {
	resultsPerPage       int64
	wantedPageQueryParam string
}

func NewPaginator() *Paginator {
	return &Paginator{
		wantedPageQueryParam: "p",
		resultsPerPage:       300,
	}
}

func (p *Paginator) Paginate(c echo.Context, query *gorm.DB) (int64, int64, int64, *gorm.DB) {
	wantedPage := utils.DoParseInt64(c.QueryParam(p.wantedPageQueryParam))
	var count int64
	query.Count(&count)
	resultsPerPage := p.resultsPerPage
	page, maxPage := Paginate(resultsPerPage, wantedPage, count)
	query = query.Offset((page - 1) * resultsPerPage).Limit(resultsPerPage)
	return page, maxPage, count, query
}

func ChatArchiveHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data chatArchiveData
	data.DateFormat = authUser.GetDateFormat()
	roomName := c.Param("roomName")
	room, err := database.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	if !room.HasAccess(c) {
		return c.Redirect(http.StatusFound, "/chat")
	}

	data.Room = room
	query := database.DB.Table("chat_messages").
		Where("room_id = ? AND (to_user_id is null OR to_user_id = ? OR user_id = ?)", room.ID, authUser.ID, authUser.ID)
	if !authUser.DisplayIgnored {
		query = query.Where(`user_id NOT IN (SELECT ignored_user_id FROM ignored_users WHERE user_id = ?)`, authUser.ID)
	}

	data.CurrentPage, data.MaxPage, data.MessagesCount, query = NewPaginator().Paginate(c, query)

	query = query.Order("id DESC").
		Preload("Room").
		Preload("User").
		Preload("ToUser")
	if err := query.Find(&data.Messages).Error; err != nil {
		logrus.Error(err)
	}

	if room.IsProtected() {
		key, err := hutils.GetRoomKeyCookie(c, int64(room.ID))
		if err != nil {
			return c.NoContent(http.StatusForbidden)
		}
		if err := data.Messages.DecryptAll(key.Value); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.Render(http.StatusOK, "chat-archive", data)
}

type ValueTokenCache struct {
	Value string // Either age/pgp token or msg to sign
	PKey  string // age/pgp public key
}

var ageTokenCache = cache.NewWithKey[database.UserID, ValueTokenCache](10*time.Minute, time.Hour)
var pgpTokenCache = cache.NewWithKey[database.UserID, ValueTokenCache](10*time.Minute, time.Hour)

func SettingsPGPHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsPGPData
	data.ActiveTab = "pgp"

	if authUser.GPGPublicKey != "" {
		reader := bytes.NewReader([]byte(authUser.GPGPublicKey))
		if block, err := armor.Decode(reader); err == nil {
			r := packet.NewReader(block.Body)
			if e, err := openpgp.ReadEntity(r); err == nil {
				data.PGPPublicKeyID = e.PrimaryKey.KeyIdString()
			}
		}
	}

	return c.Render(http.StatusOK, "settings.pgp", data)
}

func SettingsAgeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsAgeData
	data.ActiveTab = "age"
	data.AgePublicKey = authUser.AgePublicKey
	return c.Render(http.StatusOK, "settings.age", data)
}

func generateAgeEncryptedTokenMessage(userID database.UserID, pkey string) (string, error) {
	token := utils.GenerateToken32()
	ageTokenCache.SetD(userID, ValueTokenCache{Value: token, PKey: pkey})

	recipient, err := age.ParseX25519Recipient(pkey)
	if err != nil {
		logrus.Error(err)
		return "", errors.New("invalid public key")
	}
	out := &bytes.Buffer{}
	aw := armor1.NewWriter(out)
	w, err := age.Encrypt(aw, recipient)
	if _, err := io.WriteString(w, "The required code is below the line.\n----------------------------------------------------------------------------------\n"+token+"\n"); err != nil {
		logrus.Error(err)
		w.Close()
		aw.Close()
		return "", err
	}
	w.Close()
	aw.Close()

	return out.String(), nil
}

func generatePgpEncryptedTokenMessage(userID database.UserID, pkey string) (string, error) {
	token := utils.GenerateToken32()
	pgpTokenCache.SetD(userID, ValueTokenCache{Value: token, PKey: pkey})
	msg := "The required code is below the line.\n----------------------------------------------------------------------------------\n" + token + "\n"
	return utils.GeneratePgpEncryptedMessage(pkey, msg)
}

func generatePgpToBeSignedTokenMessage(userID database.UserID, pkey string) string {
	token := utils.GenerateToken10()
	msg := fmt.Sprintf("dkf_%s{%s}", time.Now().UTC().Format("2006.01.02"), token)
	pgpTokenCache.SetD(userID, ValueTokenCache{Value: msg, PKey: pkey})
	return msg
}

func AddPGPHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data addPGPData
	data.PGPPublicKey = authUser.GPGPublicKey
	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "pgp_step1" {

			data.PGPPublicKey = c.Request().PostFormValue("pgp_public_key")
			data.GpgMode = utils.DoParseBool(c.Request().PostFormValue("gpg_mode"))

			if data.GpgMode {
				data.ToBeSignedMessage = generatePgpToBeSignedTokenMessage(authUser.ID, data.PGPPublicKey)
				return c.Render(http.StatusOK, "pgp_code", data)

			} else {
				msg, err := generatePgpEncryptedTokenMessage(authUser.ID, data.PGPPublicKey)
				if err != nil {
					data.ErrorPGPPublicKey = err.Error()
					return c.Render(http.StatusOK, "pgp", data)
				}
				data.EncryptedMessage = msg
				return c.Render(http.StatusOK, "pgp_code", data)
			}

		} else if formName == "pgp_step2" {
			token, found := pgpTokenCache.Get(authUser.ID)
			if !found {
				return c.Redirect(http.StatusFound, "/settings/pgp")
			}

			data.PGPPublicKey = c.Request().PostFormValue("pgp_public_key")
			data.GpgMode = utils.DoParseBool(c.Request().PostFormValue("gpg_mode"))
			if data.GpgMode {
				data.ToBeSignedMessage = c.Request().PostFormValue("to_be_signed_message")
				data.SignedMessage = c.Request().PostFormValue("signed_message")
				if !utils.PgpCheckSignMessage(token.PKey, token.Value, data.SignedMessage) {
					data.ErrorSignedMessage = "invalid signature"
					return c.Render(http.StatusOK, "pgp_code", data)
				}

			} else {
				data.EncryptedMessage = c.Request().PostFormValue("encrypted_message")
				data.Code = c.Request().PostFormValue("pgp_code")
				if data.Code != token.Value {
					data.ErrorCode = "invalid code"
					return c.Render(http.StatusOK, "pgp_code", data)
				}
			}

			pgpTokenCache.Delete(authUser.ID)
			authUser.GPGPublicKey = token.PKey
			_ = authUser.Save()
			return c.Redirect(http.StatusFound, "/settings/pgp")
		}
	}
	return c.Render(http.StatusOK, "pgp", data)
}

func AddAgeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data addAgeData
	data.AgePublicKey = authUser.AgePublicKey
	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "age_step1" {
			data.AgePublicKey = c.Request().PostFormValue("age_public_key")
			msg, err := generateAgeEncryptedTokenMessage(authUser.ID, data.AgePublicKey)
			if err != nil {
				data.ErrorAgePublicKey = err.Error()
				return c.Render(http.StatusOK, "age", data)
			}
			data.EncryptedMessage = msg
			return c.Render(http.StatusOK, "age_code", data)

		} else if formName == "age_step2" {
			token, found := ageTokenCache.Get(authUser.ID)
			if !found {
				return c.Redirect(http.StatusFound, "/settings/age")
			}
			data.AgePublicKey = token.PKey
			data.EncryptedMessage = c.Request().PostFormValue("encrypted_message")
			data.Code = c.Request().PostFormValue("age_code")
			if data.Code != token.Value {
				data.ErrorCode = "invalid code"
				return c.Render(http.StatusOK, "age_code", data)
			}
			ageTokenCache.Delete(authUser.ID)
			authUser.AgePublicKey = token.PKey
			_ = authUser.Save()
			return c.Redirect(http.StatusFound, "/settings/age")
		}
	}
	return c.Render(http.StatusOK, "age", data)
}

// twoFactorCache ...
var twoFactorCache = cache.NewWithKey[database.UserID, twoFactorObj](10*time.Minute, time.Hour)

type twoFactorObj struct {
	key      *otp.Key
	recovery string
}

func GpgTwoFactorAuthenticationToggleHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)

	var data gpgTwoFactorAuthenticationVerifyData
	data.IsEnabled = authUser.GpgTwoFactorEnabled
	data.GpgTwoFactorMode = authUser.GpgTwoFactorMode

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "two-factor-authentication-gpg", data)
	}

	password := c.Request().PostFormValue("password")
	if !authUser.CheckPassword(password) {
		data.ErrorPassword = "Invalid password"
		return c.Render(http.StatusOK, "two-factor-authentication-gpg", data)
	}

	// Disable
	if authUser.GpgTwoFactorEnabled {
		authUser.GpgTwoFactorEnabled = false
		authUser.DoSave()
		database.CreateSecurityLog(authUser.ID, database.Gpg2faDisabledSecurityLog)
		return c.Render(http.StatusOK, "flash", FlashResponse{"GPG Two-factor authentication disabled", "/settings/account", "alert-success"})
	}

	// Enable
	if authUser.GPGPublicKey == "" {
		return c.Render(http.StatusOK, "flash", FlashResponse{"You need to setup your PGP key first", "/settings/pgp", "alert-danger"})
	}
	// Delete active user sessions
	if err := database.DeleteUserSessions(authUser.ID); err != nil {
		logrus.Error(err)
	}
	c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
	authUser.GpgTwoFactorEnabled = true
	authUser.GpgTwoFactorMode = utils.DoParseBool(c.Request().PostFormValue("gpg_two_factor_mode"))
	authUser.DoSave()
	database.CreateSecurityLog(authUser.ID, database.Gpg2faEnabledSecurityLog)
	return c.Render(http.StatusOK, "flash", FlashResponse{"GPG Two-factor authentication enabled", "/settings/account", "alert-success"})
}

// TwoFactorAuthenticationVerifyHandler ...
func TwoFactorAuthenticationVerifyHandler(c echo.Context) error {
	getImgStr := func(img image.Image) string {
		buf := bytes.NewBuffer([]byte(""))
		_ = png.Encode(buf, img)
		return base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	authUser := c.Get("authUser").(*database.User)
	if authUser.TwoFactorSecret != "" {
		return c.Redirect(http.StatusFound, "/settings/account")
	}
	var data twoFactorAuthenticationVerifyData
	if c.Request().Method == http.MethodPost {
		twoFactor, found := twoFactorCache.Get(authUser.ID)
		if !found {
			return c.Redirect(http.StatusFound, "/two-factor-authentication/verify")
		}
		password := c.Request().PostFormValue("password")
		if !authUser.CheckPassword(password) {
			img, _ := twoFactor.key.Image(150, 150)
			data.QRCode = getImgStr(img)
			data.Secret = twoFactor.key.Secret()
			data.RecoveryCode = twoFactor.recovery
			data.ErrorPassword = "Invalid password"
			return c.Render(http.StatusOK, "two-factor-authentication-verify", data)
		}
		code := c.Request().PostFormValue("code")
		if !totp.Validate(code, twoFactor.key.Secret()) {
			img, _ := twoFactor.key.Image(150, 150)
			data.QRCode = getImgStr(img)
			data.Secret = twoFactor.key.Secret()
			data.RecoveryCode = twoFactor.recovery
			data.Password = password
			data.Error = "Two-factor code verification failed. Please try again."
			return c.Render(http.StatusOK, "two-factor-authentication-verify", data)
		}
		h, err := bcrypt.GenerateFromPassword([]byte(twoFactor.recovery), 12)
		if err != nil {
			data.Error = "unable to hash recovery code: " + err.Error()
			return c.Render(http.StatusOK, "two-factor-authentication-verify", data)
		}
		// Delete active user sessions
		if err := database.DeleteUserSessions(authUser.ID); err != nil {
			logrus.Error(err)
		}
		c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
		authUser.TwoFactorSecret = database.EncryptedString(twoFactor.key.Secret())
		authUser.TwoFactorRecovery = string(h)
		if err := authUser.Save(); err != nil {
			logrus.Error(err)
		}
		database.CreateSecurityLog(authUser.ID, database.TotpEnabledSecurityLog)
		return c.Render(http.StatusOK, "flash", FlashResponse{"Two-factor authentication enabled", "/", "alert-success"})
	}
	key, _ := totp.Generate(totp.GenerateOpts{Issuer: "DarkForest", AccountName: authUser.Username})
	img, _ := key.Image(150, 150)
	recovery := utils.ShortDisplayID(10)
	data.QRCode = getImgStr(img)
	data.Secret = key.Secret()
	data.RecoveryCode = recovery
	twoFactorCache.SetD(authUser.ID, twoFactorObj{key, recovery})
	return c.Render(http.StatusOK, "two-factor-authentication-verify", data)
}

// TwoFactorAuthenticationDisableHandler ...
func TwoFactorAuthenticationDisableHandler(c echo.Context) error {
	var data diableTotpData
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "disable-totp", data)
	}
	authUser := c.Get("authUser").(*database.User)
	password := c.Request().PostFormValue("password")
	if !authUser.CheckPassword(password) {
		data.ErrorPassword = "Invalid password"
		return c.Render(http.StatusOK, "disable-totp", data)
	}
	authUser.TwoFactorSecret = ""
	authUser.TwoFactorRecovery = ""
	_ = authUser.Save()
	database.CreateSecurityLog(authUser.ID, database.TotpDisabledSecurityLog)
	return c.Render(http.StatusOK, "flash", FlashResponse{"Two-factor authentication disabled", "/settings/account", "alert-success"})
}

type downloadableFileInfo struct {
	Name     string
	OS       string
	Arch     string
	Bytes    string
	Checksum string
}

func getDownloadsBhcliFiles() (out []downloadableFileInfo) {
	return getDownloadableFiles("downloads-bhcli", `bhcli`)
}

func getDownloadsTorchessFiles() (out []downloadableFileInfo) {
	return getTorchessDownloadableFiles("downloads-torchess", `torchess`)
}

func getDownloadsFiles() (out []downloadableFileInfo) {
	return getDownloadableFiles("downloads", `ransomware-re-challenge1`)
}

func distStrToFriendlyStr(os, arch string) (string, string) {
	switch os {
	case "darwin":
		os = "macOS"
	}
	switch arch {
	case "386":
		arch = "x86"
	case "amd64":
		arch = "x86-64"
	case "arm":
		arch = "ARMv7"
	}
	return os, arch
}

func getDownloadableFiles(folder, fileNamePrefix string) (out []downloadableFileInfo) {
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".checksum") {
			checksumBytes, err := ioutil.ReadFile(path + ".checksum")
			if err != nil {
				return nil
			}
			m := regexp.MustCompile(fileNamePrefix + `\.(\w+)\.(\w+)(\.exe)?`).FindStringSubmatch(info.Name())
			if len(m) < 2 {
				return nil
			}
			osIdx := 1
			archIdx := 2
			osStr, archFmt := distStrToFriendlyStr(m[osIdx], m[archIdx])
			out = append(out, downloadableFileInfo{
				info.Name(),
				osStr,
				archFmt,
				humanize.Bytes(uint64(info.Size())),
				string(checksumBytes),
			})
		}
		return nil
	})
	if err != nil {
		logrus.Error(err)
	}
	return
}

func getTorchessDownloadableFiles(folder, fileNamePrefix string) (out []downloadableFileInfo) {
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".checksum") {
			checksumBytes, err := ioutil.ReadFile(path + ".checksum")
			if err != nil {
				return nil
			}
			m := regexp.MustCompile(fileNamePrefix + `\.\d+\.\d+\.\d+\.(\w+)\.(\w+)(\.exe)?`).FindStringSubmatch(info.Name())
			if len(m) < 2 {
				return nil
			}
			osIdx := 1
			archIdx := 2
			osStr, archFmt := distStrToFriendlyStr(m[osIdx], m[archIdx])
			out = append(out, downloadableFileInfo{
				info.Name(),
				osStr,
				archFmt,
				humanize.Bytes(uint64(info.Size())),
				string(checksumBytes),
			})
		}
		return nil
	})
	if err != nil {
		logrus.Error(err)
	}
	return
}

// BhcliDownloadsHandler ...
func BhcliDownloadsHandler(c echo.Context) error {
	var data bhcliDownloadsHandlerData
	data.Files = getDownloadsBhcliFiles()
	return c.Render(http.StatusOK, "bhcli-downloads", data)
}

func TorchessDownloadsHandler(c echo.Context) error {
	var data bhcliDownloadsHandlerData
	data.Files = getDownloadsTorchessFiles()
	return c.Render(http.StatusOK, "torchess-downloads", data)
}

var flagValidationCache = cache.NewWithKey[database.UserID, bool](time.Minute, time.Hour)

// VipDownloadsHandler ...
func VipDownloadsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data vipDownloadsHandlerData
	data.ActiveTab = "home"
	data.Files = getDownloadsFiles()
	if c.Request().Method == http.MethodPost {
		if _, found := flagValidationCache.Get(authUser.ID); found {
			data.FlagMessage = "You can only validate once per minute"
			return c.Render(http.StatusOK, "vip.downloads", data)
		}
		flag := c.Request().PostFormValue("flag")
		if len(flag) > 100 {
			data.FlagMessage = "Invalid flag"
			return c.Render(http.StatusOK, "vip.downloads", data)
		}
		if utils.Sha256([]byte(flag)) == "fefc9d5db52b51aeefd4b098f0178a8bcb7f0816dcadaf1714604f01ef63a621" {
			data.FlagMessage = "You found the flag!"
			_ = database.CreateUserBadge(authUser.ID, 1)
		} else {
			data.FlagMessage = "Invalid flag"
		}
		flagValidationCache.SetD(authUser.ID, true)
	}

	return c.Render(http.StatusOK, "vip.re-1", data)
}

func downloadFile(c echo.Context, folder, redirect string) error {
	if config.DownloadsEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Downloads are temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}

	authUser := c.Get("authUser").(*database.User)
	if authUser == nil {
		return c.Redirect(http.StatusFound, "/login?redirect="+redirect)
	}

	filename := c.Param("filename")

	if !utils.FileExists(filepath.Join(folder, filename)) {
		logrus.Error(filename + " does not exists")
		return c.Redirect(http.StatusFound, redirect)
	}

	// Keep track of user downloads
	if _, err := database.CreateDownload(authUser.ID, filename); err != nil {
		logrus.Error(err)
	}

	return c.Attachment(filepath.Join(folder, filename), filename)
}

func TorChessDownloadFileHandler(c echo.Context) error {
	return downloadFile(c, "downloads-torchess", "/torchess/downloads")
}

func BhcliDownloadFileHandler(c echo.Context) error {
	return downloadFile(c, "downloads-bhcli", "/bhcli/downloads")
}

func VipDownloadFileHandler(c echo.Context) error {
	return downloadFile(c, "downloads", "/vip/re-1")
}

func CaptchaRequiredHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)

	var data captchaRequiredData
	data.CaptchaID, data.CaptchaImg = captcha.New()
	config.CaptchaRequiredGenerated.Inc()

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "captcha-required", data)
	}

	captchaID := c.Request().PostFormValue("captcha_id")
	captchaInput := c.Request().PostFormValue("captcha")
	if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
		data.ErrCaptcha = err.Error()
		config.CaptchaRequiredFailed.Inc()
		return c.Render(http.StatusOK, "captcha-required", data)
	}
	config.CaptchaRequiredSuccess.Inc()
	authUser.CaptchaRequired = false
	authUser.DoSave()
	return c.Redirect(http.StatusFound, "/chat")
}

func CaptchaHandler(c echo.Context) error {
	var data captchaData
	setCaptcha := func(seed int64) {
		rnd := rand.New(rand.NewSource(seed))
		data.CaptchaID, data.CaptchaImg = captcha.NewWithParams(captcha.Params{Rnd: rnd})
	}
	data.Seed = time.Now().UnixNano()
	data.Ts = time.Now().UnixMilli()
	//fmt.Println("Seed:", seed)

	data.CaptchaSec = 120
	data.Frames = generateCssFrames(data.CaptchaSec, func(i int64) string {
		return fmt.Sprintf("%ds", i)
	}, false)

	if c.Request().Method == http.MethodGet {
		setCaptcha(data.Seed)
		return c.Render(http.StatusOK, "captcha", data)
	}

	captchaID := c.Request().PostFormValue("captcha_id")
	captchaInput := c.Request().PostFormValue("captcha")
	ts := utils.DoParseInt64(c.Request().PostFormValue("ts"))
	delta := time.Now().UnixMilli() - ts
	if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
		data.Seed = utils.DoParseInt64(c.Request().PostFormValue("seed"))
		setCaptcha(data.Seed)
		data.Error = fmt.Sprintf("%s; took: %.2fs", err.Error(), float64(delta)/1000)
		return c.Render(http.StatusOK, "captcha", data)
	}
	setCaptcha(data.Seed)
	data.Success = fmt.Sprintf("Good captcha; took: %.2fs", float64(delta)/1000)
	return c.Render(http.StatusOK, "captcha", data)
}

func PublicUserProfileHandler(c echo.Context) error {
	username := c.Param("username")
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data publicProfileData
	data.User = user
	data.UserStyle = user.GenerateChatStyle()
	data.PublicNotes, _ = database.GetUserPublicNotes(user.ID)
	return c.Render(http.StatusOK, "public-profile", data)
}

func PublicUserProfilePGPHandler(c echo.Context) error {
	username := c.Param("username")
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if user.GPGPublicKey == "" {
		return c.NoContent(http.StatusOK)
	}
	return c.String(http.StatusOK, user.GPGPublicKey)
}

func UploadsDownloadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	filename := c.Param("filename")
	file, err := database.GetUploadByFileName(filename)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	filePath1 := filepath.Join("uploads", file.FileName)
	if !utils.FileExists(filePath1) {
		logrus.Error(filename + " does not exists")
		return c.Redirect(http.StatusFound, "/")
	}

	if file.FileSize < 1<<20 {
		f, err := os.Open(filePath1)
		if err != nil {
			return echo.NotFoundHandler(c)
		}
		defer f.Close()

		fileBytes, _ := io.ReadAll(f)
		decFileBytes, err := utils.DecryptAES(fileBytes, []byte(config.Global.MasterKey()))
		if err != nil {
			decFileBytes = fileBytes
		}
		buf := bytes.NewReader(decFileBytes)

		// Validate image type and determine extension
		mimeType, err := GetFileContentType(buf)
		_, err = buf.Seek(0, io.SeekStart)
		if err != nil {
			return echo.NotFoundHandler(c)
		}

		fi, _ := f.Stat()

		// Serve images
		if mimeType == "image/jpeg" ||
			mimeType == "image/png" ||
			mimeType == "image/gif" ||
			mimeType == "image/bmp" ||
			mimeType == "image/x-icon" ||
			mimeType == "image/webp" {
			if fi.IsDir() {
				return echo.NotFoundHandler(c)
			}
			http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
			return nil
		}

		if mimeType == "application/octet-stream" && utf8.Valid(decFileBytes) {
			mimeType = "text/plain; charset=utf-8"
		}

		if mimeType == "application/x-gzip" ||
			mimeType == "application/zip" ||
			mimeType == "application/x-rar-compressed" ||
			mimeType == "application/pdf" ||
			mimeType == "audio/basic" ||
			mimeType == "audio/aiff" ||
			mimeType == "audio/mpeg" ||
			mimeType == "application/ogg" ||
			mimeType == "audio/midi" ||
			mimeType == "video/avi" ||
			mimeType == "audio/wave" ||
			mimeType == "video/webm" ||
			mimeType == "font/ttf" ||
			mimeType == "font/otf" ||
			mimeType == "font/collection" ||
			mimeType == "font/woff" ||
			mimeType == "font/woff2" ||
			mimeType == "application/wasm" ||
			mimeType == "application/postscript" ||
			mimeType == "application/vnd.ms-fontobject" ||
			mimeType == "application/octet-stream" {
			// Keep track of user downloads
			if _, err := database.CreateDownload(authUser.ID, filename); err != nil {
				logrus.Error(err)
			}
			c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", "attachment", file.OrigFileName))
			http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
			return nil
		}

		// Serve any other file as text/plain
		c.Response().Header().Set(echo.HeaderContentType, "text/plain")
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("filename=%q", file.OrigFileName))
		http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
		return nil
	}

	// Display captcha to new users, or old users if they already downloaded the file.
	if !authUser.AccountOldEnough() || database.UserNbDownloaded(authUser.ID, filename) >= 1 {
		// Captcha for bigger files
		var data uploadsDownloadData
		data.CaptchaID, data.CaptchaImg = captcha.New()
		if c.Request().Method == http.MethodGet {
			return c.Render(http.StatusOK, "captcha-required", data)
		}
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, "captcha-required", data)
		}
	}

	// Keep track of user downloads
	if _, err := database.CreateDownload(authUser.ID, filename); err != nil {
		logrus.Error(err)
	}

	f, err := os.Open(filePath1)
	if err != nil {
		return echo.NotFoundHandler(c)
	}
	defer f.Close()
	fileBytes, _ := io.ReadAll(f)
	decFileBytes, err := utils.DecryptAES(fileBytes, []byte(config.Global.MasterKey()))
	if err != nil {
		decFileBytes = fileBytes
	}
	buf := bytes.NewReader(decFileBytes)
	fi, _ := f.Stat()

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", "attachment", file.OrigFileName))
	http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
	return nil
}

func FiledropDownloadHandler(c echo.Context) error {
	filename := c.Param("filename")
	file, err := database.GetFiledropByFileName(filename)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	filePath1 := filepath.Join("filedrop", file.FileName)
	if !utils.FileExists(filePath1) {
		logrus.Error(filename + " does not exists")
		return c.Redirect(http.StatusFound, "/")
	}

	f, err := os.Open(filePath1)
	if err != nil {
		return echo.NotFoundHandler(c)
	}
	defer f.Close()
	fileBytes, _ := io.ReadAll(f)
	fi, _ := f.Stat()
	buf := bytes.NewReader(fileBytes)

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", "attachment", file.OrigFileName))
	http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
	return nil
}

func GetFileContentType(out io.ReadSeeker) (string, error) {

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	return contentType, nil
}

var byteRoadSignUpSessionCache = cache.New[bool](10*time.Minute, 10*time.Minute)
var byteRoadUsersCountCache = cache.NewWithKey[database.UserID, ByteRoadPayload](5*time.Minute, 10*time.Minute)

type ByteRoadPayload struct {
	Count     int64
	Usernames map[string]struct{}
}

func ByteRoadChallengeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data byteRoadChallengeData
	data.ActiveTab = "home"

	if payload, sessionExp, ok := byteRoadUsersCountCache.GetWithExpiration(authUser.ID); ok {
		data.SessionExp = time.Until(sessionExp)
		data.NbAccountsRegistered = payload.Count
		if payload.Count >= 100 {
			data.FlagFound = true
			return c.Render(http.StatusOK, "vip.byte-road-challenge", data)
		}
	}

	data.CaptchaID, data.CaptchaImg = captcha.New()

	setCookie := func(token string) {
		c.SetCookie(hutils.CreateCookie(hutils.ByteRoadCookieName, token, utils.OneDaySecs))
	}

	if c.Request().Method == http.MethodPost {

		formName := c.Request().PostFormValue("formName")
		if formName == "captcha" {
			captchaID := c.Request().PostFormValue("captcha_id")
			captchaInput := c.Request().PostFormValue("captcha")
			if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
				data.ErrCaptcha = err.Error()
				return c.Render(http.StatusOK, "vip.byte-road-challenge", data)
			}
			token := utils.GenerateToken32()
			setCookie(token)
			byteRoadSignUpSessionCache.SetD(token, true)
			data.CaptchaSolved = true
			return c.Render(http.StatusOK, "vip.byte-road-challenge", data)

		} else if formName == "register" {
			captchaSession, err := c.Cookie("challenge_byte_road_session")
			if err != nil {
				return c.Redirect(http.StatusFound, "/vip/challenges/byte-road")
			}
			if _, ok := byteRoadSignUpSessionCache.Get(captchaSession.Value); !ok {
				return c.Redirect(http.StatusFound, "/vip/challenges/byte-road")
			}

			// Validate username password
			data.Username = c.Request().PostFormValue("username")
			data.Password = c.Request().PostFormValue("password")
			if !govalidator.IsASCII(data.Username) || len(data.Username) < 3 || len(data.Username) > 10 {
				data.CaptchaSolved = true
				data.Registered = false
				data.ErrRegistration = "Invalid username (3-10 ascii characters)"
				return c.Render(http.StatusOK, "vip.byte-road-challenge", data)
			}
			if !govalidator.IsASCII(data.Password) || len(data.Password) < 3 || len(data.Password) > 10 {
				data.CaptchaSolved = true
				data.Registered = false
				data.ErrRegistration = "Invalid password (3-10 ascii characters)"
				return c.Render(http.StatusOK, "vip.byte-road-challenge", data)
			}

			data.Registered = true

			if payload, found := byteRoadUsersCountCache.Get(authUser.ID); found {
				payload.Count++

				// Username already registered
				if _, found := payload.Usernames[data.Username]; found {
					data.CaptchaSolved = true
					data.Registered = false
					data.ErrRegistration = "Username is already registered"
					return c.Render(http.StatusOK, "vip.byte-road-challenge", data)
				}

				token := utils.GenerateToken32()
				setCookie(token)

				payload.Usernames[data.Username] = struct{}{}
				_ = byteRoadUsersCountCache.Update(authUser.ID, payload)
				if payload.Count >= 100 {
					data.FlagFound = true
					_ = database.CreateUserBadge(authUser.ID, 2)
				}
				return c.Render(http.StatusOK, "vip.byte-road-challenge", data)
			}

			token := utils.GenerateToken32()
			setCookie(token)

			payload := ByteRoadPayload{Count: 1, Usernames: map[string]struct{}{data.Username: {}}}
			byteRoadUsersCountCache.SetD(authUser.ID, payload)
			return c.Render(http.StatusOK, "vip.byte-road-challenge", data)

		}
	}
	return c.Render(http.StatusOK, "vip.byte-road-challenge", data)
}

func BHCHandler(c echo.Context) error {
	/*
		We have a script that check BHC wait room and kick any users that has not completed the dkf captcha.
		When a user is kicked by that script, they are told to come here and solve the dkf captcha to get a valid bhc username.
		Once they complete the captcha, they are given a username with a suffix that prove they completed the challenge.
		Using a shared secret, the script is able to verify that the suffix is valid.
		A suffix is valid for 10min, after that a different suffix would be generated for the same username.
	*/
	var data bhcData
	data.CaptchaID, data.CaptchaImg = captcha.New()
	config.BHCCaptchaGenerated.Inc()

	username := c.QueryParam("username")
	if len(username) > 17 {
		data.Error = fmt.Sprintf("Invalid username, must have 17 characters at most")
		return c.Render(http.StatusOK, "bhc", data)
	}

	const sharedSecret = "4#yFvRpk4^rJCxjjdbrdaBzWZ"

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "bhc", data)
	}

	captchaID := c.Request().PostFormValue("captcha_id")
	captchaInput := c.Request().PostFormValue("captcha")
	if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
		data.Error = fmt.Sprintf("Invalid answer")
		config.BHCCaptchaFailed.Inc()
		return c.Render(http.StatusOK, "bhc", data)
	}
	h := utils.Sha1([]byte(fmt.Sprintf("%s_%s_%d", username, sharedSecret, time.Now().Unix()/(60*10))))
	config.BHCCaptchaSuccess.Inc()
	data.Success = fmt.Sprintf("Good answer, go back to BHC and use '%s' as your username", username+h[:3])
	return c.Render(http.StatusOK, "bhc", data)
}

func FileDropHandler(c echo.Context) error {
	uuidParam := c.Param("uuid")
	//if c.Request().ContentLength > 30<<20 {
	//	data.Error = "The maximum file size is 30 MB"
	//	return c.Render(http.StatusOK, "chat-top-bar", data)
	//}

	formHTML := `<form method="post" enctype="multipart/form-data"><input name="file" type="file" /><input type="submit" value="submit" /></form>`

	filedrop, err := database.GetFiledropByUUID(uuidParam)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if filedrop.FileName != "" {
		return c.Redirect(http.StatusFound, "/")
	}

	var data fileDropData

	if c.Request().Method == http.MethodGet {
		return c.HTML(http.StatusOK, formHTML)
	}

	file, handler, uploadErr := c.Request().FormFile("file")
	if uploadErr != nil {
		data.Error = uploadErr.Error()
		return c.HTML(http.StatusOK, formHTML+data.Error)
	}

	defer file.Close()
	origFileName := handler.Filename
	//if handler.Size > 30<<20 {
	//	return nil, html, errors.New("the maximum file size is 30 MB")
	//}
	if !govalidator.StringLength(origFileName, "3", "50") {
		data.Error = "invalid file name, 3-50 characters"
		return c.HTML(http.StatusOK, formHTML+data.Error)
	}
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		data.Error = err.Error()
		return c.HTML(http.StatusOK, formHTML+data.Error)
	}

	newFileName := utils.MD5([]byte(utils.GenerateToken32()))
	if err := ioutil.WriteFile(filepath.Join("filedrop", newFileName), fileBytes, 0644); err != nil {
		logrus.Error(err)
	}

	filedrop.FileName = newFileName
	filedrop.OrigFileName = origFileName
	filedrop.FileSize = int64(len(fileBytes))
	filedrop.DoSave()

	return c.String(http.StatusOK, "File uploaded successfully")
}

func Stego1ChallengeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data stego1RoadChallengeData
	data.ActiveTab = "home"

	if c.Request().Method == http.MethodPost {
		if _, found := flagValidationCache.Get(authUser.ID); found {
			data.FlagMessage = "You can only validate once per minute"
			return c.Render(http.StatusOK, "vip.stego1", data)
		}
		flag := c.Request().PostFormValue("flag")
		if len(flag) > 100 {
			data.FlagMessage = "Invalid flag"
			return c.Render(http.StatusOK, "vip.stego1", data)
		}
		if utils.Sha256([]byte(flag)) == "05b456689a9f8de69416d21cbb97157588b8491d07551167a95b93a1c7d61e7b" {
			data.FlagMessage = "You found the flag!"
			_ = database.CreateUserBadge(authUser.ID, 3)
		} else {
			data.FlagMessage = "Invalid flag"
		}
		flagValidationCache.SetD(authUser.ID, true)
	}

	return c.Render(http.StatusOK, "vip.stego1", data)
}

func ChessHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data chessData
	data.Games = v1.ChessInstance.GetGames()

	if c.Request().Method == http.MethodPost {
		data.Username = c.Request().PostFormValue("username")
		player2, err := database.GetUserByUsername(data.Username)
		if err != nil {
			data.Error = "invalid username"
			return c.Render(http.StatusOK, "chess", data)
		}
		if _, err := v1.ChessInstance.NewGame1("", config.GeneralRoomID, *authUser, player2); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "chess", data)
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	return c.Render(http.StatusOK, "chess", data)
}

var cssReset = `<style>
    html, body, div, span, applet, object, iframe,
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
    }

html, body {
	background-color: #222;
}
</style>`

func ChessGameHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	key := c.Param("key")

	g := v1.ChessInstance.GetGame(key)
	if g == nil {
		// Chess debug
		//user1, _ := database.GetUserByID(1)
		//user2, _ := database.GetUserByID(24132)
		//v1.ChessInstance.NewGame(key, user1, user2)
		//g = v1.ChessInstance.GetGame(key)
		return c.Redirect(http.StatusFound, "/")
	}

	var isFlipped bool
	if authUser.ID == g.Player2.ID {
		isFlipped = true
	}

	if c.Request().Method == http.MethodPost {
		msg := c.Request().PostFormValue("message")
		if msg == "resign" {
			resignColor := chess.White
			if isFlipped {
				resignColor = chess.Black
			}
			g.Game.Resign(resignColor)
			pubsub2.Pub(key, true)
		} else {
			if err := v1.ChessInstance.SendMove(key, authUser.ID, g, c); err != nil {
				logrus.Error(err)
			}
		}
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	var isSpectator bool
	if authUser.ID != g.Player1.ID && authUser.ID != g.Player2.ID {
		isSpectator = true
	}

	isYourTurnFn := func() bool {
		return authUser.ID == g.Player1.ID && g.Game.Position().Turn() == chess.White ||
			authUser.ID == g.Player2.ID && g.Game.Position().Turn() == chess.Black
	}
	isYourTurn := isYourTurnFn()

	// If you are not a spectator, and it's your turn to play, we just render the form directly.
	if isYourTurn {
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		c.Response().WriteHeader(http.StatusOK)
		_, _ = c.Response().Write([]byte(cssReset))
		card1 := g.DrawPlayerCard(false, isFlipped, isYourTurn)
		_, _ = c.Response().Write([]byte(fmt.Sprintf(`<div id="div_0">%s</div>`, card1)))
		return nil
	}

	quit := make(chan bool)
	quit1 := make(chan bool)

	// Listen to the closing of HTTP connection via CloseNotifier
	notify := c.Request().Context().Done()
	utils.SGo(func() {
		select {
		case <-notify:
		case <-quit1:
		}
		close(quit)
	})

	notify1 := make(chan os.Signal)
	signal.Notify(notify1, os.Interrupt)
	utils.SGo(func() {
		select {
		case <-notify1:
		case <-quit:
		}
		close(quit1)
	})

	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(http.StatusOK)
	c.Response().Header().Set("Transfer-Encoding", "chunked")
	c.Response().Header().Set("Connection", "keep-alive")

	_, _ = c.Response().Write([]byte(cssReset))

	authorizedChannels := make([]string, 0)
	authorizedChannels = append(authorizedChannels, key)
	pubsub := pubsub2.Subscribe(authorizedChannels)
	defer pubsub.Close()

	var card1 string
	if isSpectator {
		card1 = g.DrawSpectatorCard(isFlipped)
	} else {
		card1 = g.DrawPlayerCard(false, isFlipped, isYourTurn)
	}
	_, _ = c.Response().Write([]byte(fmt.Sprintf(`<div id="div_0">%s</div>`, card1)))

	i := 0
Loop:
	for {
		select {
		case <-quit:
			break Loop
		case <-quit1:
			break Loop
		default:
		}

		if g.Game.Outcome() != chess.NoOutcome {
			break
		}

		_, _, err := pubsub.ReceiveTimeout(1 * time.Second)
		if err != nil {
			continue
		}

		i++

		var card1 string
		if isSpectator {
			card1 = g.DrawSpectatorCard(isFlipped)
		} else {
			card1 = g.DrawPlayerCard(false, isFlipped, isYourTurnFn())
		}
		_, _ = c.Response().Write([]byte(fmt.Sprintf(`<style>#div_%d { display: none; }</style>`, i-1)))
		_, _ = c.Response().Write([]byte(fmt.Sprintf(`<div id="div_%d">%s</div>`, i, card1)))
		c.Response().Flush()
	}
	return nil
}
