package handlers

import (
	"bytes"
	"dkforest/pkg/database"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

func GistHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	gistUUID := c.Param("gistUUID")
	gist, err := db.GetGistByUUID(gistUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data gistData
	data.Gist = gist

	if c.Request().Method == http.MethodPost {

		btnSubmit := c.Request().PostFormValue("btn_submit")
		if btnSubmit == "logout" {
			hutils.DeleteGistCookie(c, gist.UUID)
			return c.Redirect(http.StatusFound, "/")

		} else if btnSubmit == "delete_gist" {
			if gist.UserID == authUser.ID {
				if gist.Password != "" {
					hutils.DeleteGistCookie(c, gist.UUID)
				}
				if err := db.DB().Delete(&gist).Error; err != nil {
					logrus.Error(err)
				}
				return c.Redirect(http.StatusFound, "/")
			}
			return c.Redirect(http.StatusFound, "/")
		}

		password := c.Request().PostFormValue("password")
		hashedPassword := database.GetGistPasswordHash(password)
		if hashedPassword != gist.Password {
			data.Error = "Invalid password"
			return c.Render(http.StatusOK, "gist-password", data)
		}
		hutils.CreateGistCookie(c, gist.UUID, hashedPassword)
		return c.Redirect(http.StatusFound, "/gists/"+gist.UUID)
	}

	if !gist.HasAccess(c) {
		return c.Render(http.StatusOK, "gist-password", data)
	}

	if strings.HasSuffix(gist.Name, ".go") {
		lexer := lexers.Match(gist.Name)
		style := styles.Get("monokai")
		formatter := html.New(html.Standalone(true), html.TabWidth(4), html.WithLineNumbers(true), html.LineNumbersInTable(true))
		iterator, _ := lexer.Tokenise(nil, gist.Content)
		buf := bytes.Buffer{}
		_ = formatter.Format(&buf, style, iterator)
		data.Highlighted = buf.String()
	}

	return c.Render(http.StatusOK, "gist", data)
}
