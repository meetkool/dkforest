package middlewares

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/random"
)

// CSRFConfig defines the config for CSRF middleware.
type CSRFConfig struct {
	// Skipper defines a function to skip middleware.
	Skipper middleware.Skipper

	// TokenLength is the length of the generated token.
	TokenLength uint8 `yaml:"token_length" validate:"min=1"`

	// TokenLookup is a string in the form of "<source>:<key>" that is used
	// to extract token from the request.
	// Possible values:
	// - "header:<name>"
	// - "form:<name>"
	// - "query:<name>"
	TokenLookup string `yaml:"token_lookup" validate:"required,tokenlookup"`

	// ContextKey is the context key to store generated CSRF token into context.
	ContextKey string `yaml:"context_key" validate:"required,contextkey"`

	// CookieName is the name of the CSRF cookie. This cookie will store CSRF token.
	CookieName string `yaml:"cookie_name" validate:"required,cookieName"`

	// CookieMaxAge is the max age (in seconds) of the CSRF cookie.
	CookieMaxAge int64 `yaml:"cookie_max_age" validate:"min=0"`

	// CookieSecure indicates if CSRF cookie is secure.
	CookieSecure bool `yaml:"cookie_secure"`

	// CookieHTTPOnly indicates if CSRF cookie is HTTP only.
	CookieHTTPOnly bool `yaml:"cookie_http_only"`

	// CookieSameSite is the SameSite attribute of the CSRF cookie.
	CookieSameSite http.SameSite `yaml:"cookie_same_site"`
}

const (
	defaultTokenLength      = 32
	defaultCookieName       = "_csrf"
	defaultCookieMaxAge     = 86400
	defaultContextKey       = "csrf"
	defaultTokenLookup      = "header:" + echo.HeaderXCSRFToken
	defaultCookieSecure     = false
	defaultCookieHTTPOnly   = false
	defaultCookieSameSite  = http.SameSiteNoneMode
	tokenLookupHeader      = "header"
	tokenLookupForm        = "form"
	tokenLookupQuery       = "query"
)

// validateTokenLookup checks if the TokenLookup string is valid.
func validateTokenLookup(lookup string) error {
	parts := strings.Split(lookup, ":")
	if len(parts) != 2 {
		return errors.New("invalid TokenLookup format")
	}
	switch parts[0] {
	case tokenLookupHeader, tokenLookupForm, tokenLookupQuery:
	default:
		return fmt.Errorf("invalid TokenLookup source: %s", parts[0])
	}
	return nil
}

// validateContextKey checks if the ContextKey string is valid.
func validateContextKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("invalid ContextKey")
	}
	return nil
}

// validateCookieName checks if the CookieName string is valid.
func validateCookieName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("invalid CookieName")
	}
	return nil
}

// NewCSRFConfig returns a new CSRFConfig with default values.
func NewCSRFConfig() *CSRFConfig {
	return &CSRFConfig{
		Skipper:      middleware.DefaultSkipper,
		TokenLength:  defaultTokenLength,
		TokenLookup:  defaultTokenLookup,
		ContextKey:   defaultContextKey,
		CookieName:   defaultCookieName,
		CookieMaxAge: defaultCookieMaxAge,
		CookieSecure: defaultCookieSecure,
		CookieHTTPOnly: defaultCookieHTTPOnly,
		CookieSameSite: defaultCookieSameSite,
	}
}

// CSRF returns a Cross-Site Request Forgery (CSRF) middleware.
// See: https://en.wikipedia.org/wiki/Cross-site_request_forgery
func CSRF() echo.MiddlewareFunc {
	c := NewCSRFConfig()
	return CSRFWithConfig(c)
}

// CSRFWithConfig returns a CSRF middleware with config.
// See `CSRF()`.
func CSRFWithConfig(config *CSRFConfig) echo.MiddlewareFunc {
	// Validate config
	if err := validate.Struct(config); err != nil {
		panic(err)
	}

	// Defaults
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}
	if config.TokenLength == 0 {
		config.TokenLength = defaultTokenLength
	}
	if config.CookieMaxAge == 0 {
		config.CookieMaxAge = defaultCookieMaxAge
	}
	if config.ContextKey == "" {
		config.ContextKey = defaultContextKey
	}
	if config.CookieName == "" {
		config.CookieName = defaultCookieName
	}
	if config.CookieSecure {
		config.CookieHTTPOnly = true
	}

	// Initialize
	parts := strings.Split(config.TokenLookup, ":")
	extractor := csrfTokenFromHeader(parts[1])
	switch parts[0] {
	case tokenLookupForm:
		extractor = csrfToken
