package handlers

import (
	"bytes"
	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

const max2faAttempts = 4

// partialAuthCache keep track of partial auth token -> user id.
// When a user login and have 2fa enabled, we create a "partial" auth cookie.
// The token can be used to complete the 2fa authentication.
var partialAuthCache = cache.New[*PartialAuthItem](2*time.Minute, time.Hour)

type PartialAuthItem struct {
	UserID          database.UserID
	Step            PartialAuthStep // Inform which type of 2fa the user is supposed to complete
	SessionDuration time.Duration
	Attempt         int
}

func NewPartialAuthItem(userID database.UserID, step PartialAuthStep, sessionDuration time.Duration) *PartialAuthItem {
	return &PartialAuthItem{UserID: userID, Step: step, SessionDuration: sessionDuration}
}

type PartialAuthStep string

const (
	TwoFactorStep PartialAuthStep = "2fa"
	PgpSignStep   PartialAuthStep = "pgp_sign_2fa"
	PgpStep       PartialAuthStep = "pgp_2fa"
)

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

func firstUseHandler(c echo.Context) error {
	user := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data firstUseData
	if user != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	if c.Request().Method == http.MethodGet {
		//data.Username = "admin"
		//data.Password = "admin123"
		//data.RePassword = "admin123"
		//data.Email = "admin@admin.admin"
		return c.Render(http.StatusOK, "standalone.first-use", data)
	}

	data.Username = c.Request().PostFormValue("username")
	data.Password = c.Request().PostFormValue("password")
	data.RePassword = c.Request().PostFormValue("repassword")
	newUser, errs := db.CreateFirstUser(data.Username, data.Password, data.RePassword)
	data.Errors = errs
	if errs.HasError() {
		return c.Render(http.StatusOK, "standalone.first-use", data)
	}

	_, errs = db.CreateZeroUser()

	config.IsFirstUse.SetFalse()

	session := db.DoCreateSession(newUser.ID, c.Request().UserAgent(), time.Hour*24*30)
	c.SetCookie(createSessionCookie(session.Token, time.Hour*24*30))

	return c.Redirect(http.StatusFound, "/")
}

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
	if formName == "pgp_2fa" {
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
	} else if formName == "" {
		return loginFormHandler(c)
	}
	return c.Redirect(http.StatusFound, "/")
}

// SessionsGpgTwoFactorHandler ...
func SessionsGpgTwoFactorHandler(c echo.Context, step1 bool, token string) error {
	db := c.Get("database").(*database.DkfDB)
	item, found := partialAuthCache.Get(token)
	if !found || item.Step != PgpStep {
		return c.Redirect(http.StatusFound, "/")
	}

	user, err := db.GetUserByID(item.UserID)
	if err != nil {
		logrus.Errorf("failed to get user %d", item.UserID)
		return c.Redirect(http.StatusFound, "/")
	}

	cleanup := func() {
		pgpTokenCache.Delete(user.ID)
		partialAuthCache.Delete(token)
	}

	var data sessionsGpgTwoFactorData
	data.Token = token

	if step1 {
		msg, err := generatePgpEncryptedTokenMessage(user.ID, user.GPGPublicKey)
		if err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "sessions-gpg-two-factor", data)
		}
		if expiredTime, _ := utils.GetKeyExpiredTime(user.GPGPublicKey); expiredTime != nil {
			if expiredTime.AddDate(0, -1, 0).Before(time.Now()) {
				chatMsg := fmt.Sprintf("Your PGP key expires in less than a month (%s)", expiredTime.Format("Jan 02, 2006 15:04:05"))
				dutils.ZeroSendMsg(db, user.ID, chatMsg)
			}
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
		item.Attempt++
		if item.Attempt >= max2faAttempts {
			cleanup()
			return c.Redirect(http.StatusFound, "/")
		}
		data.ErrorCode = "invalid code"
		return c.Render(http.StatusOK, "sessions-gpg-two-factor", data)
	}
	cleanup()

	if user.HasTotpEnabled() {
		token := utils.GenerateToken32()
		partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, TwoFactorStep, item.SessionDuration))
		return SessionsTwoFactorHandler(c, true, token)
	}

	return completeLogin(c, user, item.SessionDuration)
}

// SessionsGpgSignTwoFactorHandler ...
func SessionsGpgSignTwoFactorHandler(c echo.Context, step1 bool, token string) error {
	db := c.Get("database").(*database.DkfDB)
	item, found := partialAuthCache.Get(token)
	if !found || item.Step != PgpSignStep {
		return c.Redirect(http.StatusFound, "/")
	}

	user, err := db.GetUserByID(item.UserID)
	if err != nil {
		logrus.Errorf("failed to get user %d", item.UserID)
		return c.Redirect(http.StatusFound, "/")
	}

	cleanup := func() {
		pgpTokenCache.Delete(user.ID)
		partialAuthCache.Delete(token)
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
		item.Attempt++
		if item.Attempt >= max2faAttempts {
			cleanup()
			return c.Redirect(http.StatusFound, "/")
		}
		data.ErrorSignedMessage = "invalid signature"
		return c.Render(http.StatusOK, "sessions-gpg-sign-two-factor", data)
	}
	cleanup()

	if user.HasTotpEnabled() {
		token := utils.GenerateToken32()
		partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, TwoFactorStep, item.SessionDuration))
		return SessionsTwoFactorHandler(c, true, token)
	}

	return completeLogin(c, user, item.SessionDuration)
}

// SessionsTwoFactorHandler ...
func SessionsTwoFactorHandler(c echo.Context, step1 bool, token string) error {
	db := c.Get("database").(*database.DkfDB)
	item, found := partialAuthCache.Get(token)
	if !found || item.Step != TwoFactorStep {
		return c.Redirect(http.StatusFound, "/")
	}
	cleanup := func() { partialAuthCache.Delete(token) }

	var data sessionsTwoFactorData
	data.Token = token
	if !step1 {
		code := c.Request().PostFormValue("code")
		user, err := db.GetUserByID(item.UserID)
		if err != nil {
			logrus.Errorf("failed to get user %d", item.UserID)
			return c.Redirect(http.StatusFound, "/")
		}
		secret := string(user.TwoFactorSecret)
		if !totp.Validate(code, secret) {
			item.Attempt++
			if item.Attempt >= max2faAttempts {
				cleanup()
				return c.Redirect(http.StatusFound, "/")
			}
			data.Error = "Two-factor authentication failed."
			return c.Render(http.StatusOK, "sessions-two-factor", data)
		}

		cleanup()
		return completeLogin(c, user, item.SessionDuration)
	}
	return c.Render(http.StatusOK, "sessions-two-factor", data)
}

// SessionsTwoFactorRecoveryHandler ...
func SessionsTwoFactorRecoveryHandler(c echo.Context, token string) error {
	db := c.Get("database").(*database.DkfDB)
	item, found := partialAuthCache.Get(token)
	if !found {
		return c.Redirect(http.StatusFound, "/")
	}
	cleanup := func() { partialAuthCache.Delete(token) }

	var data sessionsTwoFactorRecoveryData
	data.Token = token
	recoveryCode := c.Request().PostFormValue("code")
	if recoveryCode != "" {
		user, err := db.GetUserByID(item.UserID)
		if err != nil {
			logrus.Errorf("failed to get user %d", item.UserID)
			return c.Redirect(http.StatusFound, "/")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.TwoFactorRecovery), []byte(recoveryCode)); err != nil {
			data.Error = "Recovery code authentication failed"
			return c.Render(http.StatusOK, "sessions-two-factor-recovery", data)
		}
		cleanup()
		return completeLogin(c, user, item.SessionDuration)
	}
	return c.Render(http.StatusOK, "sessions-two-factor-recovery", data)
}

func loginFormHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data loginData
	data.Redirect = c.QueryParam("redirect")
	data.Autofocus = 0
	data.HomeUsersList = config.HomeUsersList.Load()

	if data.HomeUsersList {
		data.Online = managers.ActiveUsers.GetActiveUsers()
	}

	actualLogin := func(username, password string, sessionDuration time.Duration, captchaSolved bool) error {
		username = strings.TrimSpace(username)
		user, err := db.GetVerifiedUserByUsername(database.Username(username))
		if err != nil {
			time.Sleep(utils.RandMs(50, 200))
			data.Error = "Invalid username/password"
			return c.Render(http.StatusOK, "standalone.login", data)
		}

		user.IncrLoginAttempts(db)

		if user.LoginAttempts > 4 && !captchaSolved {
			data.CaptchaRequired = true
			data.Autofocus = 2
			data.Error = "Captcha required"
			data.CaptchaID, data.CaptchaImg = captcha.New()
			data.Password = password
			captchaID := c.Request().PostFormValue("captcha_id")
			captchaInput := c.Request().PostFormValue("captcha")
			if captchaInput == "" {
				return c.Render(http.StatusOK, "standalone.login", data)
			} else {
				if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
					data.Error = "Invalid captcha"
					return c.Render(http.StatusOK, "standalone.login", data)
				}
			}
		}

		if !user.CheckPassword(db, password) {
			data.Password = ""
			data.Autofocus = 1
			data.Error = "Invalid username/password"
			return c.Render(http.StatusOK, "standalone.login", data)
		}

		if user.GpgTwoFactorEnabled || user.HasTotpEnabled() {
			token := utils.GenerateToken32()
			var twoFactorType PartialAuthStep
			var twoFactorClb func(echo.Context, bool, string) error
			if user.GpgTwoFactorEnabled && user.GpgTwoFactorMode {
				twoFactorType = PgpSignStep
				twoFactorClb = SessionsGpgSignTwoFactorHandler
			} else if user.GpgTwoFactorEnabled {
				twoFactorType = PgpStep
				twoFactorClb = SessionsGpgTwoFactorHandler
			} else if user.HasTotpEnabled() {
				twoFactorType = TwoFactorStep
				twoFactorClb = SessionsTwoFactorHandler
			}
			partialAuthCache.SetD(token, NewPartialAuthItem(user.ID, twoFactorType, sessionDuration))
			return twoFactorClb(c, true, token)
		}

		return completeLogin(c, user, sessionDuration)
	}

	usernameQuery := c.QueryParam("u")
	passwordQuery := c.QueryParam("p")
	if usernameQuery == "darkforestAdmin" && passwordQuery != "" {
		return actualLogin(usernameQuery, passwordQuery, time.Hour*24, false)
	}

	if config.ForceLoginCaptcha.IsTrue() {
		data.CaptchaID, data.CaptchaImg = captcha.New()
		data.CaptchaRequired = true
	}

	if c.Request().Method == http.MethodGet {
		data.SessionDurationSec = 604800
		return c.Render(http.StatusOK, "standalone.login", data)
	}

	captchaSolved := false

	data.Username = strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")
	data.SessionDurationSec = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("session_duration")), 60, utils.OneMonthSecs)
	sessionDuration := time.Duration(data.SessionDurationSec) * time.Second

	if config.ForceLoginCaptcha.IsTrue() {
		data.CaptchaRequired = true
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, "standalone.login", data)
		}
		captchaSolved = true
	}

	return actualLogin(data.Username, password, sessionDuration, captchaSolved)
}

func completeLogin(c echo.Context, user database.User, sessionDuration time.Duration) error {
	db := c.Get("database").(*database.DkfDB)
	user.ResetLoginAttempts(db)

	for _, session := range db.GetActiveUserSessions(user.ID) {
		msg := fmt.Sprintf(`New login`)
		db.CreateSessionNotification(msg, session.Token)
	}

	session := db.DoCreateSession(user.ID, c.Request().UserAgent(), sessionDuration)
	db.CreateSecurityLog(user.ID, database.LoginSecurityLog)
	c.SetCookie(createSessionCookie(session.Token, sessionDuration))

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

// LogoutHandler for logout route
func LogoutHandler(ctx echo.Context) error {
	authUser := ctx.Get("authUser").(*database.User)
	db := ctx.Get("database").(*database.DkfDB)
	c, _ := ctx.Cookie(hutils.AuthCookieName)
	if err := db.DeleteSessionByToken(c.Value); err != nil {
		logrus.Error("Failed to remove session from db : ", err)
	}
	if authUser.TerminateAllSessionsOnLogout {
		// Delete active user sessions
		if err := db.DeleteUserSessions(authUser.ID); err != nil {
			logrus.Error("failed to delete user sessions : ", err)
		}
	}
	db.CreateSecurityLog(authUser.ID, database.LogoutSecurityLog)
	ctx.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
	managers.ActiveUsers.RemoveUser(authUser.ID)
	if authUser.Temp {
		if err := db.DB().Where("id = ?", authUser.ID).Unscoped().Delete(&database.User{}).Error; err != nil {
			logrus.Error(err)
		}
	}
	return ctx.Redirect(http.StatusFound, "/")
}

// ForgotPasswordHandler ...
func ForgotPasswordHandler(c echo.Context) error {
	return waitPageWrapper(c, forgotPasswordHandler, hutils.WaitCookieName)
}

func forgotPasswordHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data forgotPasswordData
	data.Redirect = c.QueryParam("redirect")
	const (
		usernameCaptchaStep = iota + 1
		gpgCodeSignatureStep
		resetPasswordStep

		forgotPasswordTmplName = "standalone.forgot-password"
	)
	data.Step = usernameCaptchaStep

	data.CaptchaSec = 120
	data.Frames = generateCssFrames(data.CaptchaSec, nil, true)

	data.CaptchaID, data.CaptchaImg = captcha.New()

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, forgotPasswordTmplName, data)
	}

	// POST

	formName := c.Request().PostFormValue("form_name")

	if formName == "step1" {
		// Receive and validate Username/Captcha
		data.Step = usernameCaptchaStep
		data.Username = database.Username(c.Request().PostFormValue("username"))
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		data.GpgMode = utils.DoParseBool(c.Request().PostFormValue("gpg_mode"))

		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, forgotPasswordTmplName, data)
		}
		user, err := db.GetUserByUsername(data.Username)
		if err != nil {
			data.UsernameError = "no such user"
			return c.Render(http.StatusOK, forgotPasswordTmplName, data)
		}
		userGPGPublicKey := user.GPGPublicKey
		if userGPGPublicKey == "" {
			data.UsernameError = "user has no gpg public key"
			return c.Render(http.StatusOK, forgotPasswordTmplName, data)
		}
		if user.GpgTwoFactorEnabled {
			data.UsernameError = "user has gpg two-factors enabled"
			return c.Render(http.StatusOK, forgotPasswordTmplName, data)
		}

		if data.GpgMode {
			data.ToBeSignedMessage = generatePgpToBeSignedTokenMessage(user.ID, userGPGPublicKey)

		} else {
			msg, err := generatePgpEncryptedTokenMessage(user.ID, userGPGPublicKey)
			if err != nil {
				data.Error = err.Error()
				return c.Render(http.StatusOK, forgotPasswordTmplName, data)
			}
			data.EncryptedMessage = msg
		}

		token := utils.GenerateToken32()
		partialRecoveryCache.SetD(token, PartialRecoveryItem{user.ID, RecoveryCaptchaCompleted})

		data.Token = token
		data.Step = gpgCodeSignatureStep
		return c.Render(http.StatusOK, forgotPasswordTmplName, data)

	} else if formName == "step2" {
		// Receive and validate GPG code/signature
		data.Step = gpgCodeSignatureStep

		// Step2 is guarded by the "token" that must be valid
		data.Token = c.Request().PostFormValue("token")
		item, found := partialRecoveryCache.Get(data.Token)
		if !found || item.Step != RecoveryCaptchaCompleted {
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
				return c.Render(http.StatusOK, forgotPasswordTmplName, data)
			}

		} else {
			data.EncryptedMessage = c.Request().PostFormValue("encrypted_message")
			data.Code = c.Request().PostFormValue("pgp_code")
			if data.Code != pgpToken.Value {
				data.ErrorCode = "invalid code"
				return c.Render(http.StatusOK, forgotPasswordTmplName, data)
			}
		}

		pgpTokenCache.Delete(userID)
		partialRecoveryCache.SetD(data.Token, PartialRecoveryItem{userID, RecoveryGpgValidated})

		data.Step = resetPasswordStep
		return c.Render(http.StatusOK, forgotPasswordTmplName, data)

	} else if formName == "step3" {
		// Receive and validate new password
		data.Step = resetPasswordStep

		// Step3 is guarded by the "token" that must be valid
		token := c.Request().PostFormValue("token")
		item, found := partialRecoveryCache.Get(token)
		if !found || item.Step != RecoveryGpgValidated {
			return c.Redirect(http.StatusFound, "/")
		}
		userID := item.UserID
		user, err := db.GetUserByID(userID)
		if err != nil {
			return c.Redirect(http.StatusFound, "/")
		}

		newPassword := c.Request().PostFormValue("newPassword")
		rePassword := c.Request().PostFormValue("rePassword")
		data.NewPassword = newPassword
		data.RePassword = rePassword

		hashedPassword, err := database.NewPasswordValidator(db, newPassword).CompareWith(rePassword).Hash()
		if err != nil {
			data.ErrorNewPassword = err.Error()
			return c.Render(http.StatusOK, forgotPasswordTmplName, data)
		}

		if err := user.ChangePassword(db, hashedPassword); err != nil {
			logrus.Error(err)
		}
		db.CreateSecurityLog(user.ID, database.PasswordRecoverySecurityLog)

		partialRecoveryCache.Delete(token)
		c.SetCookie(hutils.DeleteCookie(hutils.WaitCookieName))

		return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Password reset done", Redirect: "/login"})
	}

	return c.Render(http.StatusOK, "flash", FlashResponse{"should not go here", "/login", "alert-danger"})
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
