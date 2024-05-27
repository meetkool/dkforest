package template

import (
	"dkforest/pkg/global"
	"dkforest/pkg/web/clientFrontends"
	hutils "dkforest/pkg/web/handlers/utils"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"
	"time"

	"dkforest/bindata"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"github.com/labstack/echo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// Templates ...
type Templates struct {
	Templates map[string]*template.Template
	funcMap   FuncMap
	e         *echo.Echo
}

// NewTemplateBuilder ...
func NewTemplateBuilder(e *echo.Echo) *Templates {
	t := new(Templates)
	t.funcMap = make(FuncMap)
	t.e = e
	return t
}

// AddFn ...
func (t *Templates) AddFn(name string, fn any) {
	t.funcMap[name] = fn
}

type templateDataStruct struct {
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

// Render render a template
func (t *Templates) Render(w io.Writer, name string, data any, c echo.Context) error {
	tmpl := t.Templates[name]

	db := c.Get("database").(*database.DkfDB)
	clientFE := c.Get("clientFE").(clientFrontends.ClientFrontend)

	d := templateDataStruct{}
	d.TmplName = name
	d.Data = data
	d.VERSION = config.Global.AppVersion.Get().Original()
	if config.Development.IsTrue() {
		d.VERSION += utils.FormatInt64(time.Now().Unix())
	}
	d.Reverse = c.Echo().Reverse // {{ call .Reverse "name" }} || {{ call .Reverse "name" "params" }}
	d.Bundle = c.Get("bundle").(*i18n.Bundle)
	//d.SHA = config.Global.Sha()
	d.VersionHTML = template.HTML(fmt.Sprintf("<!-- VERSION: %s -->", config.Global.AppVersion.Get().Original()))
	d.ShaHTML = template.HTML(fmt.Sprintf("<!-- SHA: %s -->", config.Global.Sha.Get()))
	d.NullUsername = config.NullUsername
	d.CSRF, _ = c.Get("csrf").(string)
	d.DB = db
	d.AcceptLanguage = c.Get("accept-language").(string)
	d.Lang = c.Get("lang").(string)
	d.BaseKeywords = strings.Join(getBaseKeywords(), ", ")
	d.Development = config.Development.Load()
	d.AuthUser = c.Get("authUser").(*database.User)
	switch clientFE {
	case clientFrontends.TorClientFE:
		d.GitURL = config.DkfGitOnion
	case clientFrontends.I2PClientFE:
		d.GitURL = config.I2pGitOnion
	}

	d.WallpaperImg = "/public/img/login_bg.jpg"
	year, month, day := time.Now().UTC().Date()
	d.IsAprilFool2023 = year == 2023 && month == time.April && day == 1
	if strings.HasPrefix(c.QueryParam("redirect"), "/poker") {
		d.WallpaperImg = "/public/img/login_bg_poker.jpg"
	} else if month == time.December {
		d.WallpaperImg = "/public/img/login_bg_1.jpg"
	} else if d.IsAprilFool2023 {
		d.WallpaperImg = "/public/img/login_bg_2.jpg"
	}

	if d.AuthUser != nil {
		var sessionToken string
		if authCookie, err := c.Cookie(hutils.AuthCookieName); err == nil {
			sessionToken = authCookie.Value
		}
		d.InboxCount = global.GetUserNotificationCount(db, d.AuthUser.ID, sessionToken)
	}

	return tmpl.ExecuteTemplate(w, "base", d)
}

// Keywords use for html meta tag, for SEO
func getBaseKeywords() []string {
	return []string{}
}

// BuildTemplates build all templates
func (t *Templates) BuildTemplates() {
	t.Templates = make(map[string]*template.Template)

	bases := []string{
		"views/pages/captcha-tmpl.gohtml",
		"views/pages/pagination.gohtml",
		"views/pages/anti-prefill.gohtml"}
	buildTemplatesHelper("views/pages", t.Templates, "", bases, t.funcMap)
}

func buildTemplatesHelper(root string, tmpls map[string]*template.Template, prefix string, bases []string, fnsMap FuncMap) {
	if _, err := bindata.AssetInfo(root + prefix + "/index.gohtml"); err == nil {
		bases = append(bases, root+prefix+"/index.gohtml")
	}
	viewsPages, _ := bindata.AssetDir(root + prefix)
	for _, page := range viewsPages {
		// Recursively process folders
		ext := filepath.Ext(page)
		if !strings.HasSuffix(page, ".gohtml") {
			buildTemplatesHelper(root, tmpls, prefix+"/"+page, bases, fnsMap)
			continue
		}
		// Create template
		page = strings.TrimSuffix(page, ".gohtml")
		tmpl := New("_", bindata.Asset).Funcs(fnsMap)
		parseBases(tmpl, bases)
		tmpl = Must(tmpl.Parse(root + prefix + "/" + page + ext))
		// Add to templates collection
		tmplName := buildTemplateName(prefix, page)
		tmpls[tmplName] = tmpl.Tmpl
	}
}

func buildTemplateName(prefix, page string) string {
	tmplName := strings.TrimPrefix(prefix, "/")
	tmplName = strings.Join(strings.Split(tmplName, "/"), ".") + "." + page
	tmplName = strings.TrimPrefix(tmplName, ".")
	return tmplName
}

func parseBases(tmpl *Template, bases []string) {
	for _, b := range bases {
		tmpl = Must(tmpl.Parse(b))
	}
}
