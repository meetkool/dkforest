package handlers

import (
	"bytes"
	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	hutils "dkforest/pkg/web/handlers/utils"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

var signupCache = cache.New[SignupInfo](5*time.Minute, 5*time.Minute)

type SignupInfo struct {
	ScreenWidth     string
	ScreenHeight    string
	HelvaticaLoaded bool

	hasSolvedCaptcha bool
	UpdatedAt        string
}

// SignupHandler ...
func SignupHandler(c echo.Context) error {
	if config.ProtectHome.IsTrue() {
		return c.NoContent(http.StatusNotFound)
	}
	return tmpSignupHandler(c)
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
	db := c.Get("database").(*database.DkfDB)
	invitationToken := c.Param("invitationToken")
	invitationTokenQuery := c.QueryParam("invitationToken")
	if invitationTokenQuery != "" {
		invitationToken = invitationTokenQuery
	}
	if _, err := db.GetUnusedInvitationByToken(invitationToken); err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	return waitPageWrapper(c, signupHandler, hutils.WaitCookieName)
}

func tmpSignupHandler(c echo.Context) error {
	if config.SignupFakeEnabled.IsFalse() && config.SignupEnabled.IsFalse() {
		return c.Render(http.StatusOK, "standalone.signup-invite", nil)
	}
	return waitPageWrapper(c, signupHandler, hutils.WaitCookieName)
}

// The random wait time 0-15 seconds make sure the load is evenly distributed while under DDoS.
// Not all requests to the signup endpoint will get the captcha at the same time,
// so you cannot just refresh the page until you get a captcha that is easier to crack.
func signupHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	start := c.Get("start").(int64)
	signupToken := c.Get("signupToken").(string)
	var data signupData
	config.SignupPageLoad.Inc()

	data.Redirect = c.QueryParam("redirect")
	data.PowEnabled = config.PowEnabled.Load()
	data.CaptchaSec = 120
	data.Frames = generateCssFrames(data.CaptchaSec, nil, true)

	hbCookie, hbCookieErr := c.Cookie(hutils.HBCookieName)
	hasHBCookie := hbCookieErr == nil && hbCookie.Value != ""

	pokerReferralCookie, _ := hutils.GetPokerReferralCookie(c)

	signupInfo, _ := signupCache.Get(signupToken)

	data.HasSolvedCaptcha = signupInfo.hasSolvedCaptcha
	if !signupInfo.hasSolvedCaptcha {
		data.CaptchaID, data.CaptchaImg = captcha.New()
	}

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "standalone.signup", data)
	}

	// POST
	data.Username = strings.TrimSpace(c.Request().PostFormValue("username"))
	data.Password = c.Request().PostFormValue("password")
	data.RePassword = c.Request().PostFormValue("repassword")
	data.Pow = c.Request().PostFormValue("pow")
	captchaID := c.Request().PostFormValue("captcha_id")
	captchaInput := c.Request().PostFormValue("captcha")
	captchaInputImg := c.Request().PostFormValue("captcha_img")
	if !signupInfo.hasSolvedCaptcha {
		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			config.SignupFailed.Inc()
			return c.Render(http.StatusOK, "standalone.signup", data)
		}
	}
	data.Captcha = captchaInput
	data.CaptchaImg = captchaInputImg

	signupInfo.hasSolvedCaptcha = true
	data.HasSolvedCaptcha = signupInfo.hasSolvedCaptcha
	signupCache.SetD(signupToken, signupInfo)

	// verify POW
	if config.PowEnabled.IsTrue() {
		if !hutils.VerifyPow(data.Username, data.Pow, config.PowDifficulty) {
			data.ErrPow = "invalid proof of work"
			return c.Render(http.StatusOK, "standalone.signup", data)
		}
	}

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
	newUser, errs := db.CreateUser(data.Username, data.Password, data.RePassword, registrationDuration, string(signupInfoEnc))
	if errs.HasError() {
		data.Errors = errs
		return c.Render(http.StatusOK, "standalone.signup", data)
	}

	// Fuck with hellbanned users. New account also hellbanned
	if hasHBCookie {
		newUser.IsHellbanned = true
		newUser.DoSave(db)
	}

	if pokerReferralCookie != nil {
		if referredByUser, err := db.GetUserByPokerReferralToken(pokerReferralCookie.Value); err == nil {
			newUser.PokerReferredBy = &referredByUser.ID
			newUser.DoSave(db)
			c.SetCookie(hutils.DeleteCookie(hutils.PokerReferralName))
		}
	}

	invitationToken := c.Param("invitationToken")
	if invitationToken != "" {
		if invitation, err := db.GetUnusedInvitationByToken(invitationToken); err == nil {
			invitation.InviteeUserID = newUser.ID
			invitation.DoSave(db)
		}
	}

	// If more than 10 users were created in the past minute, auto disable signup for the website
	if db.GetRecentUsersCount() > 10 {
		settings := db.GetSettings()
		settings.SignupEnabled = false
		settings.DoSave(db)
		config.SignupEnabled.SetFalse()
		if userNull, err := db.GetUserByUsername(config.NullUsername); err == nil {
			db.NewAudit(userNull, fmt.Sprintf("auto turn off signup"))

			// Display message in chat
			txt := fmt.Sprintf("auto turn off registrations")
			if err := db.CreateSysMsg(txt, txt, "", config.GeneralRoomID, userNull.ID); err != nil {
				logrus.Error(err)
			}
		}
	}

	c.SetCookie(hutils.DeleteCookie(hutils.WaitCookieName))
	return c.Render(http.StatusOK, "flash", FlashResponse{"Your account has been created", "/login", "alert-success"})
}

func SignalCss1(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
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
	authUser.SetSignupMetadata(db, string(signupInfoEnc))
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
