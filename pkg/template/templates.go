package template

import (
	"dkforest/pkg/global"
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
	"github.com/sirupsen/logrus"
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
	Bundle         *i18n.Bundle
	AcceptLanguage string
	Lang           string
	Data           any
	VERSION        string
	SHA            string
	VersionHTML    template.HTML
	ShaHTML        template.HTML
	LogoASCII      template.HTML
	CSRF           string
	Master         bool
	Development    bool
	BaseKeywords   string
	TmplName       string
	AuthUser       *database.User
	InboxCount     int64
	WallpaperImg   string
}

// Render render a template
func (t *Templates) Render(w io.Writer, name string, data any, c echo.Context) error {
	tmpl := t.Templates[name]

	d := templateDataStruct{}
	d.TmplName = name
	d.Data = data
	d.VERSION = config.Global.GetVersion().Original()
	if config.Development.IsTrue() {
		d.VERSION += utils.FormatInt64(time.Now().Unix())
	}
	d.Bundle = c.Get("bundle").(*i18n.Bundle)
	//d.SHA = config.Global.Sha()
	d.VersionHTML = template.HTML(fmt.Sprintf("<!-- VERSION: %s -->", config.Global.GetVersion().Original()))
	d.ShaHTML = template.HTML(fmt.Sprintf("<!-- SHA: %s -->", config.Global.Sha()))
	d.CSRF = c.Get("csrf").(string)
	d.AcceptLanguage = c.Get("accept-language").(string)
	d.Lang = c.Get("lang").(string)
	d.BaseKeywords = strings.Join(getBaseKeywords(), ", ")
	d.Development = config.Development.Load()
	d.AuthUser = c.Get("authUser").(*database.User)

	d.WallpaperImg = "/public/img/login_bg.jpg"
	_, month, _ := time.Now().UTC().Date()
	if month == time.December {
		d.WallpaperImg = "/public/img/login_bg_1.jpg"
	}

	if d.AuthUser != nil {
		var sessionToken string
		if authCookie, err := c.Cookie(hutils.AuthCookieName); err == nil {
			sessionToken = authCookie.Value
		}
		d.InboxCount = global.GetUserNotificationCount(d.AuthUser.ID, sessionToken)
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

	var err error
	tmpl := New("_", bindata.Asset).Funcs(t.funcMap)
	tmpl, err = tmpl.Parse("views/pages/first-use.gohtml")
	if err != nil {
		logrus.Error(err)
	}
	t.Templates["first-use"] = tmpl.Tmpl
}

func buildTemplatesHelper(root string, tmpls map[string]*template.Template, prefix string, bases []string, fnsMap FuncMap) {
	if _, err := bindata.AssetInfo(root + prefix + "/index.gohtml"); err == nil {
		bases = append(bases, root+prefix+"/index.gohtml")
	}
	viewsPages, _ := bindata.AssetDir(root + prefix)
LOOP:
	for _, page := range viewsPages {
		for _, base := range bases {
			if root+prefix+"/"+page == base {
				continue LOOP
			}
		}
		ext := filepath.Ext(page)
		if !strings.HasSuffix(page, ".html") && !strings.HasSuffix(page, ".gohtml") {
			buildTemplatesHelper(root, tmpls, prefix+"/"+page, bases, fnsMap)
			continue
		}

		page = strings.TrimSuffix(page, ".html")
		page = strings.TrimSuffix(page, ".gohtml")
		tmpl := New("_", bindata.Asset).Funcs(fnsMap)

		var err error
		for _, b := range bases {
			tmpl, err = tmpl.Parse(b)
			if err != nil {
				logrus.Error(err)
			}
		}
		tmpl, err = tmpl.Parse(root + prefix + "/" + page + ext)
		if err != nil {
			logrus.Error(root+prefix+"/"+page+ext, err)
		}
		tmplName := strings.TrimPrefix(prefix, "/")
		tmplName = strings.Join(strings.Split(tmplName, "/"), ".") + "." + page
		tmplName = strings.TrimPrefix(tmplName, ".")
		tmpls[tmplName] = tmpl.Tmpl
	}
}
