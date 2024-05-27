package handlers

import (
	"bytes"
	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/odometer"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"encoding/base64"
	"fmt"
	humanize "github.com/dustin/go-humanize"
	"github.com/labstack/echo"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

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

func createSessionCookie(value string, sessionDuration time.Duration) *http.Cookie {
	return hutils.CreateCookie(hutils.AuthCookieName, value, int64(sessionDuration.Seconds()))
}

// FlashResponse ...
type FlashResponse struct {
	Message  string
	Redirect string
	Type     string
}

func AesNB64(in string) string {
	encryptedVal, _ := utils.EncryptAESMaster([]byte(in))
	return base64.URLEncoding.EncodeToString(encryptedVal)
}

func DAesB64(in string) ([]byte, error) {
	enc, err := base64.URLEncoding.DecodeString(in)
	if err != nil {
		return nil, err
	}
	encryptedVal, err := utils.DecryptAESMaster(enc)
	if err != nil {
		return nil, err
	}
	return encryptedVal, nil
}

func DAesB64Str(in string) (string, error) {
	encryptedVal, err := DAesB64(in)
	return string(encryptedVal), err
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
		waitTime := getWaitPageDuration()
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
		return c.Render(http.StatusOK, "standalone.wait", data)

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

// RecaptchaResponse ...
type RecaptchaResponse struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

// n: how many frames to generate.
// contentFn: callback to alter the content of the frames
// reverse: if true, will generate the frames like so: 5 4 3 2 1 0
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

func MemeHandler(c echo.Context) error {
	slug := c.Param("slug")
	db := c.Get("database").(*database.DkfDB)
	meme, err := db.GetMemeBySlug(slug)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	fi, by, err := meme.GetContent()
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	buf := bytes.NewReader(by)

	http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
	return nil
}

func NewsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data newsData
	category, _ := db.GetForumCategoryBySlug("news")
	data.News, _ = db.GetForumNews(category.ID)
	return c.Render(http.StatusOK, "news", data)
}

func BhcliHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "bhcli", nil)
}

func TorchessHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "torchess", nil)
}

func PowHelpHandler(c echo.Context) error {
	var data powHelperData
	data.Difficulty = config.PowDifficulty
	return c.Render(http.StatusOK, "pow-help", data)
}

func CaptchaHelpHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "captcha-help", nil)
}

func WerewolfHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "werewolf", nil)
}

func RoomsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data roomsData
	data.Rooms, _ = db.GetListedChatRooms(authUser.ID)
	return c.Render(http.StatusOK, "rooms", data)
}

func getTutorialStepDuration() int64 {
	secs := int64(15)
	if config.Development.IsTrue() {
		secs = 1
	}
	return secs
}

func getWaitPageDuration() int64 {
	secs := utils.RandI64(5, 15)
	if config.Development.IsTrue() {
		secs = 2
	}
	return secs
}

func ExternalLink1Handler(c echo.Context) error {
	original, _ := url.PathUnescape(c.Param("original"))
	var data externalLink1Data
	data.Link = original
	return c.Render(http.StatusOK, "external-link1", data)
}

func ExternalLinkHandler(c echo.Context) error {
	service := c.Param("service")
	original, _ := url.PathUnescape(c.Param("original"))
	baseURL := "/"
	if service == "invidious" {
		baseURL = utils.RandChoice(dutils.InvidiousURLs)
	} else if service == "libreddit" {
		baseURL = utils.RandChoice(dutils.LibredditURLs)
	} else if service == "wikiless" {
		baseURL = utils.RandChoice(dutils.WikilessURLs)
	} else if service == "nitter" {
		baseURL = utils.RandChoice(dutils.NitterURLs)
	} else if service == "rimgo" {
		baseURL = utils.RandChoice(dutils.RimgoURLs)
	} else {
		return c.String(http.StatusNotFound, "Not found")
	}
	return c.Redirect(http.StatusFound, baseURL+"/"+original)
}

func DonateHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "donate", nil)
}

func ShopHandler(c echo.Context) error {
	getImgStr := func(img image.Image) string {
		buf := bytes.NewBuffer([]byte(""))
		_ = png.Encode(buf, img)
		return base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data shopData
	invoice, err := db.CreateXmrInvoice(authUser.ID, 1)
	if err != nil {
		logrus.Error(err)
	}
	b, _ := invoice.GetImage()
	data.Img = getImgStr(b)
	data.Invoice = invoice

	return c.Render(http.StatusOK, "shop", data)
}

type ValueTokenCache struct {
	Value string // Either age/pgp token or msg to sign
	PKey  string // age/pgp public key
}

var ageTokenCache = cache.NewWithKey[database.UserID, ValueTokenCache](2*time.Minute, time.Hour)
var pgpTokenCache = cache.NewWithKey[database.UserID, ValueTokenCache](2*time.Minute, time.Hour)

func generateTokenMsg(token string) string {
	msg := "The required code is below the line.\n"
	msg += "----------------------------------------------------------------------------------\n"
	msg += token + "\n"
	return msg
}

func generatePgpEncryptedTokenMessage(userID database.UserID, pkey string) (string, error) {
	token := utils.GenerateToken10()
	pgpTokenCache.SetD(userID, ValueTokenCache{Value: token, PKey: pkey})
	msg := generateTokenMsg(token)
	return utils.GeneratePgpEncryptedMessage(pkey, msg)
}

func generatePgpToBeSignedTokenMessage(userID database.UserID, pkey string) string {
	token := utils.GenerateToken10()
	msg := fmt.Sprintf("dkf_%s{%s}", time.Now().UTC().Format("2006.01.02"), token)
	pgpTokenCache.SetD(userID, ValueTokenCache{Value: msg, PKey: pkey})
	return msg
}

// twoFactorCache ...
var twoFactorCache = cache.NewWithKey[database.UserID, twoFactorObj](10*time.Minute, time.Hour)

type twoFactorObj struct {
	key      *otp.Key
	recovery string
}

func GpgTwoFactorAuthenticationToggleHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	var data gpgTwoFactorAuthenticationVerifyData
	data.IsEnabled = authUser.GpgTwoFactorEnabled
	data.GpgTwoFactorMode = authUser.GpgTwoFactorMode

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "two-factor-authentication-gpg", data)
	}

	password := c.Request().PostFormValue("password")
	if !authUser.CheckPassword(db, password) {
		data.ErrorPassword = "Invalid password"
		return c.Render(http.StatusOK, "two-factor-authentication-gpg", data)
	}

	// Disable
	if authUser.GpgTwoFactorEnabled {
		authUser.DisableGpg2FA(db)
		db.CreateSecurityLog(authUser.ID, database.Gpg2faDisabledSecurityLog)
		return c.Render(http.StatusOK, "flash", FlashResponse{"GPG Two-factor authentication disabled", "/settings/account", "alert-success"})
	}

	// Enable
	if authUser.GPGPublicKey == "" {
		return c.Render(http.StatusOK, "flash", FlashResponse{"You need to setup your PGP key first", "/settings/pgp", "alert-danger"})
	}
	// Delete active user sessions
	if err := db.DeleteUserSessions(authUser.ID); err != nil {
		logrus.Error(err)
	}
	c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
	authUser.GpgTwoFactorEnabled = true
	authUser.GpgTwoFactorMode = utils.DoParseBool(c.Request().PostFormValue("gpg_two_factor_mode"))
	authUser.DoSave(db)
	db.CreateSecurityLog(authUser.ID, database.Gpg2faEnabledSecurityLog)
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
	db := c.Get("database").(*database.DkfDB)
	if authUser.HasTotpEnabled() {
		return c.Redirect(http.StatusFound, "/settings/account")
	}
	var data twoFactorAuthenticationVerifyData
	if c.Request().Method == http.MethodPost {
		twoFactor, found := twoFactorCache.Get(authUser.ID)
		if !found {
			return c.Redirect(http.StatusFound, "/two-factor-authentication/verify")
		}
		password := c.Request().PostFormValue("password")
		if !authUser.CheckPassword(db, password) {
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
		if err := db.DeleteUserSessions(authUser.ID); err != nil {
			logrus.Error(err)
		}
		c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
		authUser.TwoFactorSecret = database.EncryptedString(twoFactor.key.Secret())
		authUser.TwoFactorRecovery = string(h)
		authUser.DoSave(db)
		db.CreateSecurityLog(authUser.ID, database.TotpEnabledSecurityLog)
		return c.Render(http.StatusOK, "flash", FlashResponse{"Two-factor authentication enabled", "/", "alert-success"})
	}
	key, _ := totp.Generate(totp.GenerateOpts{Issuer: "DarkForest", AccountName: string(authUser.Username)})
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
	db := c.Get("database").(*database.DkfDB)
	password := c.Request().PostFormValue("password")
	if !authUser.CheckPassword(db, password) {
		data.ErrorPassword = "Invalid password"
		return c.Render(http.StatusOK, "disable-totp", data)
	}
	authUser.DisableTotp2FA(db)
	db.CreateSecurityLog(authUser.ID, database.TotpDisabledSecurityLog)
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
			checksumBytes, err := os.ReadFile(path + ".checksum")
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
			checksumBytes, err := os.ReadFile(path + ".checksum")
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

func downloadFile(c echo.Context, folder, redirect string) error {
	if config.DownloadsEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Downloads are temporarily disabled", Redirect: "/", Type: "alert-danger"})
	}

	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if authUser == nil {
		return c.Redirect(http.StatusFound, "/login?redirect="+redirect)
	}

	filename := c.Param("filename")

	if !utils.FileExists(filepath.Join(folder, filename)) {
		logrus.Error(filename + " does not exists")
		return c.Redirect(http.StatusFound, redirect)
	}

	// Keep track of user downloads
	if _, err := db.CreateDownload(authUser.ID, filename); err != nil {
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

func CaptchaRequiredHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	var data captchaRequiredData
	data.CaptchaDescription = "Captcha required"
	data.CaptchaID, data.CaptchaImg = captcha.New()
	config.CaptchaRequiredGenerated.Inc()

	const captchaRequiredTmpl = "captcha-required"
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, captchaRequiredTmpl, data)
	}

	captchaID := c.Request().PostFormValue("captcha_id")
	captchaInput := c.Request().PostFormValue("captcha")
	if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
		data.ErrCaptcha = err.Error()
		config.CaptchaRequiredFailed.Inc()
		return c.Render(http.StatusOK, captchaRequiredTmpl, data)
	}
	config.CaptchaRequiredSuccess.Inc()
	authUser.SetCaptchaRequired(db, false)
	return c.Redirect(http.StatusFound, "/chat")
}

func OdometerHandler(c echo.Context) error {
	var data odometerData
	data.Odometer = odometer.New("12345")
	return c.Render(http.StatusOK, "odometer", data)
}

func CaptchaHandler(c echo.Context) error {
	var data captchaData
	if c.QueryParam("a") != "" {
		data.ShowAnswer = true
	}
	setCaptcha := func(seed int64) {
		data.CaptchaID, data.Answer, data.CaptchaImg, data.CaptchaAnswerImg = captcha.NewWithSolution(seed)
		if !data.ShowAnswer {
			data.CaptchaAnswerImg = ""
		}
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
	username := database.Username(c.Param("username"))
	db := c.Get("database").(*database.DkfDB)
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data publicProfileData
	data.User = user
	data.UserStyle = user.GenerateChatStyle()
	data.PublicNotes, _ = db.GetUserPublicNotes(user.ID)
	data.GpgKeyExpiredTime, data.GpgKeyExpired = utils.GetKeyExpiredTime(user.GPGPublicKey)
	if data.GpgKeyExpiredTime != nil {
		data.GpgKeyExpiredSoon = data.GpgKeyExpiredTime.AddDate(0, -1, 0).Before(time.Now())
	}
	return c.Render(http.StatusOK, "public-profile", data)
}

func PublicUserProfilePGPHandler(c echo.Context) error {
	username := database.Username(c.Param("username"))
	db := c.Get("database").(*database.DkfDB)
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if user.GPGPublicKey == "" {
		return c.NoContent(http.StatusOK)
	}
	return c.String(http.StatusOK, user.GPGPublicKey)
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
