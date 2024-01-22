package handlers

import (
	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"net/http"
	"time"
)

func VipHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data vipData
	data.ActiveTab = "home"
	data.UsersBadges, _ = db.GetUsersBadges()
	return c.Render(http.StatusOK, "vip.home", data)
}

func Stego1ChallengeHandler(c echo.Context) error {
	const flagHash = "05b456689a9f8de69416d21cbb97157588b8491d07551167a95b93a1c7d61e7b"
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
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
		if utils.Sha256([]byte(flag)) == flagHash {
			data.FlagMessage = "You found the flag!"
			_ = db.CreateUserBadge(authUser.ID, 3)
		} else {
			data.FlagMessage = "Invalid flag"
		}
		flagValidationCache.SetD(authUser.ID, true)
	}

	return c.Render(http.StatusOK, "vip.stego1", data)
}

func ForgotPasswordBypassChallengeHandler(c echo.Context) error {
	var data forgotPasswordBypassChallengeData
	data.ActiveTab = "home"
	return c.Render(http.StatusOK, "vip.forgot-password-bypass-challenge", data)
}

var byteRoadSignUpSessionCache = cache.New[bool](10*time.Minute, 10*time.Minute)
var byteRoadUsersCountCache = cache.NewWithKey[database.UserID, ByteRoadPayload](5*time.Minute, 10*time.Minute)

type ByteRoadPayload struct {
	Count     int64
	Usernames map[string]struct{}
}

func ByteRoadChallengeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	const byteRoadChallengeTmplName = "vip.byte-road-challenge"
	var data byteRoadChallengeData
	data.ActiveTab = "home"

	if payload, sessionExp, ok := byteRoadUsersCountCache.GetWithExpiration(authUser.ID); ok {
		data.SessionExp = time.Until(sessionExp)
		data.NbAccountsRegistered = payload.Count
		if payload.Count >= 100 {
			data.FlagFound = true
			return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)
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
				return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)
			}
			token := utils.GenerateToken32()
			setCookie(token)
			byteRoadSignUpSessionCache.SetD(token, true)
			data.CaptchaSolved = true
			return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)

		} else if formName == "register" {
			captchaSession, err := c.Cookie(hutils.ByteRoadCookieName)
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
				return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)
			}
			if !govalidator.IsASCII(data.Password) || len(data.Password) < 3 || len(data.Password) > 10 {
				data.CaptchaSolved = true
				data.Registered = false
				data.ErrRegistration = "Invalid password (3-10 ascii characters)"
				return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)
			}

			data.Registered = true

			if payload, found := byteRoadUsersCountCache.Get(authUser.ID); found {

				// Username already registered
				if _, found := payload.Usernames[data.Username]; found {
					data.CaptchaSolved = true
					data.Registered = false
					data.ErrRegistration = "Username is already registered"
					return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)
				}

				token := utils.GenerateToken32()
				setCookie(token)

				payload.Count++
				payload.Usernames[data.Username] = struct{}{}
				_ = byteRoadUsersCountCache.Update(authUser.ID, payload)
				if payload.Count >= 100 {
					data.FlagFound = true
					_ = db.CreateUserBadge(authUser.ID, 2)
				}
				return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)
			}

			token := utils.GenerateToken32()
			setCookie(token)

			payload := ByteRoadPayload{Count: 1, Usernames: map[string]struct{}{data.Username: {}}}
			byteRoadUsersCountCache.SetD(authUser.ID, payload)
			return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)

		}
	}
	return c.Render(http.StatusOK, byteRoadChallengeTmplName, data)
}

var flagValidationCache = cache.NewWithKey[database.UserID, bool](time.Minute, time.Hour)

// VipDownloadsHandler ...
func VipDownloadsHandler(c echo.Context) error {
	const flagHash = "fefc9d5db52b51aeefd4b098f0178a8bcb7f0816dcadaf1714604f01ef63a621"
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
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
		if utils.Sha256([]byte(flag)) == flagHash {
			data.FlagMessage = "You found the flag!"
			_ = db.CreateUserBadge(authUser.ID, 1)
		} else {
			data.FlagMessage = "Invalid flag"
		}
		flagValidationCache.SetD(authUser.ID, true)
	}

	return c.Render(http.StatusOK, "vip.re-1", data)
}

func VipDownloadFileHandler(c echo.Context) error {
	return downloadFile(c, "downloads", "/vip/re-1")
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

func VipProjectsMalwareDropperHandler(c echo.Context) error {
	var data vipData
	data.ActiveTab = "malware-dropper"
	return c.Render(http.StatusOK, "vip.malware-dropper", data)
}

func VipProjectsRustRansomwareHandler(c echo.Context) error {
	var data vipData
	data.ActiveTab = "rust-ransomware"
	return c.Render(http.StatusOK, "vip.rust-ransomware", data)
}
