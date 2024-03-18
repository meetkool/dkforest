package template

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/yaml.v2"
)

type Templates struct {
	templates *template.Template
	funcMap   template.FuncMap
	e         *echo.Echo
}

type TemplateData struct {
	Bundle          *i18n.Bundle
	AcceptLanguage  string
	Lang            string
	Data            any
	VERSION         string
	SHA             string
	VersionHTML     template.HTML
	ShaHTML         template.HTML
	LogoASCII       template.HTML
	NullUsername    string
	DB              *database.DkfDB
	CSRF            string
	Master          bool
	Development     bool
	BaseKeywords    string
	TmplName        string
	AuthUser        *database.User
	InboxCount      int64
	WallpaperImg    string
	IsAprilFool2023 bool
	GitURL          string
	Reverse         func(string, ...any) string
}

func NewTemplateBuilder(e *echo.Echo) *Templates {
	t := &Templates{
		funcMap: template.FuncMap{
			"reverse": func(s string, args ...any) string {
				return reverse(s, args...)
			},
		},
	}
	t.e = e
	return t
}

func (t *Templates) AddFn(name string, fn any) {
	t.funcMap[name] = fn
}

func (t *Templates) Render(w io.Writer, name string, data any, c echo.Context) error {
	td := &TemplateData{
		TmplName: name,
		Data:     data,
		VERSION:  config.Global.AppVersion.Get().Original(),
	}
	td.Reverse = c.Echo().Reverse
	td.Bundle = c.Get("bundle").(*i18n.Bundle)
	td.DB = c.Get("database").(*database.DkfDB)
	td.CSRF = c.Get("csrf").(string)
	td.AcceptLanguage = c.Get("accept-language").(string)
	td.Lang = c.Get("lang").(string)
	td.BaseKeywords = strings.Join(getBaseKeywords(), ", ")
	td.Development = config.Development.Load()
	td.AuthUser = c.Get("authUser").(*database.User)
	td.VersionHTML = template.HTML(fmt.Sprintf("<!-- VERSION: %s -->", td.VERSION))
	td.ShaHTML = template.HTML(fmt.Sprintf("<!-- SHA: %s -->", config.Global.Sha.Get()))
	td.NullUsername = config.NullUsername
	td.WallpaperImg = "/public/img/login_bg.jpg"
	year, month, day := time.Now().UTC().Date()
	td.IsAprilFool2023 = year == 2023 && month == time.April && day == 1
	switch c.Get("clientFE").(clientFrontends.ClientFrontend) {
	case clientFrontends.TorClientFE:
		td.GitURL = config.DkfGitOnion
	case clientFrontends.I2PClientFE:
		td.GitURL = config.I2pGitOnion
	}

	if td.AuthUser != nil {
		var sessionToken string
		if authCookie, err := c.Cookie(hutils.AuthCookieName); err == nil {
			sessionToken = authCookie.Value
		}
		td.InboxCount = global.GetUserNotificationCount(td.DB, td.AuthUser.ID, sessionToken)
	}

	return t.templates.ExecuteTemplate(w, name, td)
}

func getBaseKeywords() []string {
	return []string{}
}

func (t *Templates) BuildTemplates() {
	t.templates = template.Must(template.New("").Funcs(t.funcMap).ParseGlob("views/pages/*.gohtml"))
}

func reverse(s string, args ...any) string {
	// implementation of the reverse function
}

