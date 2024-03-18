package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type forumData struct {
	ForumCategories []database.ForumCategory
	ForumThreads    []database.ForumThread
}

type forumCategoryData struct {
	ForumThreads []database.ForumThread
}

type forumSearchData struct {
	Search           string
	AuthorFilter     string
	ForumThreads    []database.ForumThread
	ForumMessages    []database.ForumMessage
}

func ForumHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data forumData
	var err error
	data.ForumCategories, err = db.GetForumCategories()
	if err != nil {
		return err
	}
	data.ForumThreads, err = db.GetPublicForumCategoryThreads(authUser.ID, 1)
	if err != nil {
		return err
	}
	return c.Render(http.StatusOK, "forum", data)
}

func ForumCategoryHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	categorySlug := c.Param("categorySlug")
	var data forumCategoryData
	category, err := db.GetForumCategoryBySlug(categorySlug)
	if err != nil {
		return c.Redirect(http.StatusFound, "/forum")
	}
	data.ForumThreads, err = db.GetPublicForumCategoryThreads(authUser.ID, category.ID)
	if err != nil {
	
