package handlers

import (
	"crypto/sha256"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"github.com/patrickmn/go-cache"
	"net/http"
	"strings"
	"time"

	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
)

type vipData struct {
	ActiveTab    string
	UsersBadges  []database.UserBadge
	Files         []string
	FlagMessage   string
	SessionExp   time.Duration
	NbAccountsRegistered int64
	FlagFound    bool
	CaptchaID    string
	CaptchaImg   string
	ErrCaptcha   string
	Username     string
	Password     string
	Registered   bool
	ErrRegistration  string
	CaptchaSolved bool
	FlagHash     string
	DownloadFlagHash string
}

type stego1RoadChallengeData struct {
	ActiveTab    string
	FlagMessage  string
}

type forgotPasswordBypassChallengeData struct {
	ActiveTab string
}

type byteRoadPayload struct {
	Count     int64
	Usernames map[string]struct{}
}

type byteRoadChallengeData struct {
	ActiveTab    string
	SessionExp   time.Duration
	NbAccountsRegistered int64
	FlagFound    bool
	CaptchaID    string
	CaptchaImg   string
	ErrCaptcha   string
	Username     string
	Password     string
	Registered   bool
	ErrRegistration  string
	CaptchaSolved bool
	FlagHash     string
	DownloadFlagHash string
}

func VipHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	data := vipData{ActiveTab: "home"}
	usersBadges, err := db.GetUsersBadges()
	if err != nil {
		c.Error(err)
		return c.Render(http.StatusInternalServerError, "error", data)
	}
	data.UsersBadges = usersBadges
	return c.Render(http.StatusOK, "vip.home", data)
}

func Stego1ChallengeHandler(c echo.Context) error {
	const flagHash = "05b456689a9f8de69416d21cbb97157588b8491d07551167a95b93a1c7d61e7b"
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	data := stego1RoadChallengeData{ActiveTab: "home"}

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
			err := db.CreateUserBadge(authUser.ID, 3)
			if err != nil {
				c.Error(err)
				return c.Render(http.StatusInternalServerError, "error", data)
			}
		} else {
			data.FlagMessage = "Invalid flag"
		}
		flagValidationCache.SetD(authUser.ID, true)
	}

	return c.Render(http.StatusOK, "vip.stego1", data)
}

func ForgotPasswordBypassChallengeHandler(c echo.Context) error {
	data := forgotPasswordBypassChallengeData{ActiveTab: "home"}
	return c.Render(http.StatusOK, "vip.forgot-password-bypass-challenge", data)
}

var byteRoadSignUpSessionCache = cache.New[bool](10*time.Minute, 10*time.Minute)
var byteRoadUsersCountCache = cache.NewWithKey[database.UserID, byteRoadPayload](5*time.Minute, 10*time.Minute)

func ByteRoadChallengeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	const byteRoadChallengeTmplName = "vip.byte-road-challenge"
	data := byteRoadChallengeData{ActiveTab: "home"}

	if payload, sessionExp, ok := byteRoadUsersCountCache.GetWithExpiration(authUser.ID); ok {
		data.SessionExp = sessionExp
		data.NbAccountsRegistered = payload.Count
		if payload.Count >= 100 {
			data.FlagFound = true
			return c.Render(http
