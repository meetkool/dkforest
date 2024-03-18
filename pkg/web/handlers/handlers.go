package handlers

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"image"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/odometer"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"golang.org/x/crypto/sha3"
)

var tempLoginCache = cache.New[TempLoginCaptcha](3 * time.Minute, 3 * time.Minute)
var tempLoginStore = captcha.NewMemoryStore(captcha.CollectNum, 3*time.Minute)

type TempLoginCaptcha struct {
	ID         string
	Img        string
	ValidUntil time.Time
}

func HomeHandler(c echo.Context) error {
	ctx := c.Request().Context()
	db := database.FromContext(ctx)

	if config.IsFirstUse.IsTrue() {
		return firstUseHandler(c)
	}

	user := c.Get("authUser").(*database.User)
	if user != nil {
		return c.Render(http.StatusOK, "home", nil)
	}

	if config.ProtectHome.IsTrue() {
		return protectHomeHandler(c)
	}

	return loginHandler(c)
}

func createSessionCookie(value string, sessionDuration time.Duration) *http.Cookie {
	return hutils.CreateCookie(hutils.AuthCookieName, value, int64(sessionDuration.Seconds()))
}

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
	
