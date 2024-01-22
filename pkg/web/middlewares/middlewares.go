package middlewares

import (
	"dkforest/bindata"
	"dkforest/pkg/cache"
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/clientFrontends"
	"dkforest/pkg/web/handlers"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/store/memory"
	"net"
	"net/http"
	"strings"
	"time"
)

// GzipMiddleware ...
var GzipMiddleware = middleware.GzipWithConfig(
	middleware.GzipConfig{
		Level: 5,
		Skipper: func(c echo.Context) bool {
			if c.Path() == "/bhcli/downloads/:filename" ||
				c.Path() == "/vip/downloads/:filename" ||
				c.Path() == "/vip/challenges/re-1/:filename" ||
				c.Path() == "/chess/:key" ||
				c.Path() == "/chess/:key/analyze" ||
				c.Path() == "/poker/:roomID/stream" ||
				c.Path() == "/poker/:roomID/logs" ||
				c.Path() == "/poker/:roomID/bet" ||
				c.Path() == "/api/v1/chat/messages/:roomName/stream" ||
				c.Path() == "/api/v1/chat/messages/:roomName/stream/menu" ||
				c.Path() == "/uploads/:filename" ||
				c.Path() == "/" {
				return true
			}
			return false
		},
	},
)

// BodyLimit ...
var BodyLimit = middleware.BodyLimitWithConfig(middleware.BodyLimitConfig{
	Limit: "1M",
	Skipper: func(c echo.Context) bool {
		if c.Path() == "/api/v1/chat/top-bar/:roomName" {
			return true
		}
		return false
	},
})

// CaptchaMiddleware ...
func CaptchaMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var data captchaMiddlewareData
			data.CaptchaDescription = "Captcha required"
			data.CaptchaID, data.CaptchaImg = captcha.New()
			const captchaRequiredTmpl = "captcha-required"
			if c.Request().Method == http.MethodPost {
				captchaID := c.Request().PostFormValue("captcha_id")
				captchaInput := c.Request().PostFormValue("captcha")
				if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
					data.ErrCaptcha = err.Error()
					return c.Render(http.StatusOK, captchaRequiredTmpl, data)
				}
				return next(c)
			}
			return c.Render(http.StatusOK, captchaRequiredTmpl, data)
		}
	}
}

// GenericRateLimitMiddleware rate limit on userID if authenticated, or circuitID otherwise
// This rate limiter should be used for endpoints that are accessible by both unauthenticated and authenticated users.
func GenericRateLimitMiddleware(period time.Duration, limit int64) echo.MiddlewareFunc {
	rate := limiter.Rate{Period: period, Limit: limit}
	store := memory.NewStore()
	limiterInstance := limiter.New(store, rate)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := "ip_" + c.RealIP()
			if authUser, ok := c.Get("authUser").(*database.User); ok && authUser != nil {
				key = "userid_" + authUser.ID.String()
			} else if conn, ok := c.Request().Context().Value("conn").(net.Conn); ok {
				circuitID := config.ConnMap.Get(conn)
				key = "circuitid_" + utils.FormatInt64(circuitID)
			}
			context, err := limiterInstance.Get(c.Request().Context(), key)
			if err != nil {
				return next(c)
			}
			c.Response().Header().Add("X-RateLimit-Limit", utils.FormatInt64(context.Limit))
			c.Response().Header().Add("X-RateLimit-Remaining", utils.FormatInt64(context.Remaining))
			c.Response().Header().Add("X-RateLimit-Reset", utils.FormatInt64(context.Reset))
			if context.Reached {
				return c.Render(http.StatusTooManyRequests, "flash", handlers.FlashResponse{Message: "Rate limit exceeded", Redirect: c.Request().URL.String(), Type: "alert-warning"})
			}
			return next(c)
		}
	}
}

func CircuitRateLimitMiddleware(period time.Duration, limit int64, kill bool) echo.MiddlewareFunc {
	rate := limiter.Rate{Period: period, Limit: limit}
	store := memory.NewStore()
	limiterInstance := limiter.New(store, rate)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			if conn, ok := c.Request().Context().Value("conn").(net.Conn); ok {
				circuitID := config.ConnMap.Get(conn)

				context, err := limiterInstance.Get(c.Request().Context(), utils.FormatInt64(circuitID))
				if err != nil {
					return next(c)
				}
				c.Response().Header().Add("X-RateLimit-Limit", utils.FormatInt64(context.Limit))
				c.Response().Header().Add("X-RateLimit-Remaining", utils.FormatInt64(context.Remaining))
				c.Response().Header().Add("X-RateLimit-Reset", utils.FormatInt64(context.Reset))
				if context.Reached {
					if kill {
						config.ConnMap.CloseCircuit(circuitID)
						return c.NoContent(http.StatusOK)
					}
					return c.Render(http.StatusTooManyRequests, "flash", handlers.FlashResponse{Message: "Rate limit exceeded", Redirect: c.Request().URL.String(), Type: "alert-warning"})
				}
			}
			return next(c)
		}
	}
}

// AuthRateLimitMiddleware ...
func AuthRateLimitMiddleware(period time.Duration, limit int64) echo.MiddlewareFunc {
	rate := limiter.Rate{Period: period, Limit: limit}
	store := memory.NewStore()
	limiterInstance := limiter.New(store, rate)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authUser := c.Get("authUser").(*database.User)
			context, err := limiterInstance.Get(c.Request().Context(), utils.FormatInt64(int64(authUser.ID)))
			if err != nil {
				// fmt.Errorf("could not get context for IP %s - %v", c.RealIP(), err)
				return next(c)
			}
			c.Response().Header().Add("X-RateLimit-Limit", utils.FormatInt64(context.Limit))
			c.Response().Header().Add("X-RateLimit-Remaining", utils.FormatInt64(context.Remaining))
			c.Response().Header().Add("X-RateLimit-Reset", utils.FormatInt64(context.Reset))
			if context.Reached {
				return c.Render(http.StatusTooManyRequests, "flash", handlers.FlashResponse{Message: "Rate limit exceeded", Redirect: c.Request().URL.String(), Type: "alert-warning"})
				//return c.JSON(429, map[string]string{"message": fmt.Sprintf("Rate limit exceeded for %s", authUser.Username)})
			}
			return next(c)
		}
	}
}

// CSRFMiddleware ...
func CSRFMiddleware() echo.MiddlewareFunc {
	csrfConfig := CSRFConfig{
		TokenLookup:    "form:csrf",
		CookieDomain:   config.Global.CookieDomain.Get(),
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSecure:   config.Global.CookieSecure.Get(),
		CookieMaxAge:   utils.OneMonthSecs,
		SameSite:       http.SameSiteLaxMode,
		Skipper: func(c echo.Context) bool {
			apiKey := c.Request().Header.Get("DKF_API_KEY")
			return (apiKey != "" && strings.HasPrefix(c.Path(), "/api/v1/")) ||
				c.Path() == "/api/v1/battleship" ||
				c.Path() == "/api/v1/werewolf" ||
				c.Path() == "/chess/:key" ||
				c.Path() == "/poker/:roomID/sit/:pos" ||
				c.Path() == "/poker/:roomID/unsit" ||
				c.Path() == "/poker/:roomID/bet" ||
				c.Path() == "/poker/:roomID/logs" ||
				c.Path() == "/poker/:roomID/deal"
		},
	}
	return CSRFWithConfig(csrfConfig)
}

// I18nMiddleware ...
func I18nMiddleware(bundle *i18n.Bundle, defaultLang string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.HasPrefix(c.Path(), "/sse/") {
				return next(c)
			}
			accept := c.Request().Header.Get("Accept-Language")

			// This is how the language is chosen:
			// - User preference (if set)
			// - App lang flag (if set)
			// - Browser accept-language header
			// - Default en

			lang := ""
			user, _ := c.Get("authUser").(*database.User)
			if user != nil && user.Lang != "" {
				lang = user.Lang
			} else if defaultLang != "" {
				lang = defaultLang
			}
			c.Set("lang", lang)
			c.Set("accept-language", accept)
			c.Set("bundle", bundle)
			return next(c)
		}
	}
}

func SetDatabaseMiddleware(db *database.DkfDB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ctx.Set("database", db)
			return next(ctx)
		}
	}
}

func SetClientFEMiddleware(clientFE clientFrontends.ClientFrontend) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			ctx.Set("clientFE", clientFE)
			return next(ctx)
		}
	}
}

// SetUserMiddleware Get user and put it into echo context.
// - Get auth-token from cookie
// - If exists, get user from database
// - If found, set user in echo context
// - Otherwise, empty user will be put in context
func SetUserMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		db := ctx.Get("database").(*database.DkfDB)
		var nilUser *database.User
		var user database.User

		if apiKey := ctx.Request().Header.Get("DKF_API_KEY"); apiKey != "" {
			// Login using DKF_API_KEY
			if err := db.GetUserByApiKey(&user, apiKey); err == nil {
				ctx.Set("authUser", &user)
				return next(ctx)
			}
		} else if authCookie, err := ctx.Cookie(hutils.AuthCookieName); err == nil {
			// Login using auth cookie
			if err := db.GetUserBySessionKey(&user, authCookie.Value); err == nil {
				ctx.Set("authUser", &user)
				return next(ctx)
			} else {
			}
		}

		ctx.Set("authUser", nilUser)
		return next(ctx)
	}
}

type RateLimit[K comparable, V any] struct {
	cache *cache.Cache[K, V]
	value V
}

func NewRateLimit[K comparable](defaultExpiration time.Duration) *RateLimit[K, struct{}] {
	return NewRateLimitV[K, struct{}](defaultExpiration)
}

func NewRateLimitV[K comparable, V any](defaultExpiration time.Duration) *RateLimit[K, V] {
	return &RateLimit[K, V]{
		cache: cache.NewWithKey[K, V](defaultExpiration, time.Minute),
	}
}

func (l *RateLimit[K, V]) RateLimit(k K, clb func()) {
	if !l.cache.Has(k) {
		clb()
		l.cache.SetD(k, l.value)
	}
}

func (l *RateLimit[K, V]) RateLimitV(k K, clb func() (V, error)) (V, bool, error) {
	var err error
	if !l.cache.Has(k) {
		l.value, err = clb()
		l.cache.SetD(k, l.value)
		return l.value, true, err
	}
	return l.value, false, err
}

var lastSeenRL = NewRateLimit[database.UserID](time.Second)

// IsAuthMiddleware will ensure user is authenticated.
// - Find user from context
// - If user is empty, redirect to home
func IsAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("authUser").(*database.User)
		db := c.Get("database").(*database.DkfDB)
		if user == nil {
			if strings.HasPrefix(c.Path(), "/api/") {
				return c.String(http.StatusUnauthorized, "unauthorized")
			}
			referralToken := c.QueryParam("r")
			if strings.HasPrefix(c.Path(), "/poker") && referralToken != "" {
				if len(referralToken) == 9 {
					hutils.CreatePokerReferralCookie(c, referralToken)
				}
			}
			return c.Redirect(http.StatusFound, "/?redirect="+c.Request().URL.String())
		}

		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		lastSeenRL.RateLimit(user.ID, func() {
			now := time.Now()
			db.DB().Exec("UPDATE users SET last_seen_at = ?, updated_at = ? WHERE id = ?", now, now, int64(user.ID))
		})

		// Prevent clickjacking by setting the header on every logged in page
		if !strings.Contains(c.Path(), "/chess/:key/form") &&
			!strings.Contains(c.Path(), "/chess/:key/stats") &&
			!strings.Contains(c.Path(), "/api/v1/chat/messages") &&
			!strings.Contains(c.Path(), "/api/v1/chat/messages/:roomName/stream") &&
			!strings.Contains(c.Path(), "/api/v1/chat/messages/:roomName/stream/menu") &&
			!strings.Contains(c.Path(), "/api/v1/chat/top-bar") &&
			!strings.Contains(c.Path(), "/api/v1/chat/controls") &&
			!strings.Contains(c.Path(), "/poker/:roomID/stream") &&
			!strings.Contains(c.Path(), "/poker/:roomID/sit/:pos") &&
			!strings.Contains(c.Path(), "/poker/:roomID/unsit") &&
			!strings.Contains(c.Path(), "/poker/:roomID/bet") &&
			!strings.Contains(c.Path(), "/poker/:roomID/logs") &&
			!strings.Contains(c.Path(), "/poker/:roomID/deal") {
			c.Response().Header().Set("X-Frame-Options", "DENY")
		}

		return next(c)
	}
}

func ForceCaptchaMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("authUser").(*database.User)
		if user.CaptchaRequired && c.Path() != "/captcha-required" {
			return c.Redirect(http.StatusFound, "/captcha-required")
		}
		return next(c)
	}
}

// HellbannedCookieMiddleware if a user is HB and doesn't have the cookie, creates it.
// We use this cookie to auto HB new account created by this person.
func HellbannedCookieMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("authUser").(*database.User)
		if user != nil && user.IsHellbanned {
			if _, err := c.Cookie(hutils.HBCookieName); err != nil {
				cookie := hutils.CreateCookie(hutils.HBCookieName, utils.GenerateToken3(), utils.OneMonthSecs)
				c.SetCookie(cookie)
			}
		}
		return next(c)
	}
}

// ClubMiddleware ...
func ClubMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("authUser").(*database.User)
		if user != nil && (!user.IsAdmin && !user.IsClubMember) {
			var data unauthorizedData
			data.Message = `To access this section, you need an official invitation from the team.`
			return c.Render(http.StatusOK, "unauthorized", data)
		}
		return next(c)
	}
}

// VipMiddleware ...
func VipMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("authUser").(*database.User)
		if user != nil && user.GPGPublicKey == "" {
			var data unauthorizedData
			data.Message = `To access this section, you need to have a valid PGP public key linked to your profile.<br />
<a href="/settings/pgp">Add your PGP public key to your profile here</a>`
			return c.Render(http.StatusOK, "unauthorized", data)
		}
		return next(c)
	}
}

// IsModeratorMiddleware only moderators can access these routes.
func IsModeratorMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("authUser").(*database.User)
		if user == nil || !user.IsModerator() {
			if strings.HasPrefix(c.Path(), "/api") {
				if user == nil {
					return c.NoContent(http.StatusUnauthorized)
				} else if !user.IsModerator() {
					return c.NoContent(http.StatusForbidden)
				}
				return c.NoContent(http.StatusInternalServerError)
			}
			return c.Redirect(http.StatusFound, "/")
		}
		return next(c)
	}
}

// IsAdminMiddleware only administrators can access these routes.
func IsAdminMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("authUser").(*database.User)
		if user == nil || !user.IsAdmin {
			if strings.HasPrefix(c.Path(), "/api") {
				if user == nil {
					return c.NoContent(http.StatusUnauthorized)
				} else if !user.IsAdmin {
					return c.NoContent(http.StatusForbidden)
				}
				return c.NoContent(http.StatusInternalServerError)
			}
			return c.Redirect(http.StatusFound, "/")
		}
		return next(c)
	}
}

func AprilFoolMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.HasPrefix(c.Path(), "/api/v1/") {
				return next(c)
			}

			year, month, day := time.Now().UTC().Date()
			if year == 2022 && month == time.April && day == 1 {
				vv := hutils.GetAprilFoolCookie(c)
				if vv < 3 {
					hutils.CreateAprilFoolCookie(c, vv+1)
					return c.Render(http.StatusOK, "seized", nil)
				}
			}
			return next(c)
		}
	}
}

func DdosMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	stopFn := func(c echo.Context) error {
		hutils.KillCircuit(c)
		config.RejectedReqCounter.Incr()
		time.Sleep(utils.RandSec(5, 20))
		return c.NoContent(http.StatusOK)
	}
	return func(c echo.Context) error {
		config.RpsCounter.Incr()
		if authCookie, err := c.Cookie(hutils.AuthCookieName); err == nil {
			if len(authCookie.Value) > 64 {
				return stopFn(c)
			}
		}
		if csrfCookie, err := c.Cookie("_csrf"); err == nil {
			if len(csrfCookie.Value) > 32 {
				return stopFn(c)
			}
		}
		if len(c.QueryParam("captcha")) > 6 {
			return stopFn(c)
		}
		return next(c)
	}
}

// MaintenanceMiddleware ...
func MaintenanceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if config.MaintenanceAtom.IsFalse() {
			return next(c)
		}
		if strings.HasPrefix(c.Path(), "/admin/") ||
			strings.HasPrefix(c.Path(), "/master-admin/") ||
			strings.HasPrefix(c.Path(), "/api/v1/master-admin") {
			return next(c)
		}
		asset := bindata.MustAsset("views/pages/maintenance.gohtml")
		return c.HTML(http.StatusOK, string(asset))
	}
}

// MaybeAuthMiddleware let un-authenticated users access the page if MaybeAuthEnabled is enabled.
// Otherwise, the user needs to be authenticated to access the page.
func MaybeAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if config.MaybeAuthEnabled.IsFalse() {
			if user := c.Get("authUser").(*database.User); user == nil {
				return c.Redirect(http.StatusFound, "/")
			}
		}
		return next(c)
	}
}

// NoAuthMiddleware redirect to / is the user is authenticated
func NoAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if user := c.Get("authUser").(*database.User); user != nil {
			return c.Redirect(http.StatusFound, "/")
		}
		return next(c)
	}
}

// FirstUseMiddleware if first use, redirect to /
func FirstUseMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if config.IsFirstUse.IsTrue() && c.Path() != "/" {
			return c.Redirect(http.StatusFound, "/")
		}
		return next(c)
	}
}

// SecureMiddleware ...
var SecureMiddleware = middleware.SecureWithConfig(middleware.SecureConfig{
	XSSProtection:      "1; mode=block",
	ContentTypeNosniff: "nosniff",
	XFrameOptions:      "SAMEORIGIN",
	//HSTSMaxAge:         3600,
	//ContentSecurityPolicy: "default-src 'self'",
})

func SetUselessHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("X-Powered-By", "the almighty n0tr1v")
		return next(c)
	}
}
