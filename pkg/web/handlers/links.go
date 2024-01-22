package handlers

import (
	"bytes"
	"dkforest/pkg/captcha"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"encoding/base64"
	"encoding/csv"
	"encoding/pem"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

func LinksHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data linksData

	data.Categories, _ = db.GetCategories()

	data.Search = c.QueryParam("search")
	filterCategory := c.QueryParam("category")

	if filterCategory != "" {
		if filterCategory == "uncategorized" {
			db.DB().Raw(`SELECT l.*
FROM links l
LEFT JOIN links_categories_links cl ON cl.link_id = l.id
WHERE cl.link_id IS NULL AND l.deleted_at IS NULL
ORDER BY l.title COLLATE NOCASE ASC`).Scan(&data.Links)
			data.LinksCount = int64(len(data.Links))
		} else {
			db.DB().Raw(`SELECT l.*
FROM links_categories_links cl
INNER JOIN links l ON l.id = cl.link_id
WHERE cl.category_id = (SELECT id FROM links_categories WHERE name = ?) AND l.deleted_at IS NULL
ORDER BY l.title COLLATE NOCASE ASC`, filterCategory).Scan(&data.Links)
			data.LinksCount = int64(len(data.Links))
		}
	} else if data.Search != "" {
		if govalidator.IsURL(data.Search) {
			if searchedURL, err := url.Parse(data.Search); err == nil {
				h := searchedURL.Scheme + "://" + searchedURL.Hostname()
				var l database.Link
				query := db.DB()
				if authUser.IsModerator() {
					query = query.Unscoped()
				}
				if err := query.First(&l, "url = ?", h).Error; err == nil {
					data.Links = append(data.Links, l)
				}
				data.LinksCount = int64(len(data.Links))
			}
		} else {
			if err := db.DB().Raw(`select l.id, l.uuid, l.url, l.title, l.description
from fts5_links l
where fts5_links match ?
ORDER BY rank, l.title COLLATE NOCASE ASC
LIMIT 100`, data.Search).Scan(&data.Links).Error; err != nil {
				logrus.Error(err)
			}
			data.LinksCount = int64(len(data.Links))
		}
	} else {
		if err := db.DB().Table("links").
			Scopes(func(query *gorm.DB) *gorm.DB {
				data.CurrentPage, data.MaxPage, data.LinksCount, query = NewPaginator().Paginate(c, query)
				return query
			}).Order("title COLLATE NOCASE ASC").Find(&data.Links).Error; err != nil {
			logrus.Error(err)
		}
	}

	// Get all links IDs
	linksIDs := make([]int64, 0)
	for _, l := range data.Links {
		linksIDs = append(linksIDs, l.ID)
	}
	// Keep pointers to links for fast access
	linksCache := make(map[int64]*database.Link)
	for i, l := range data.Links {
		linksCache[l.ID] = &data.Links[i]
	}
	// Get all mirrors for all links that we have
	var mirrors []database.LinksMirror
	db.DB().Raw(`select * from links_mirrors where link_id in (?)`, linksIDs).Scan(&mirrors)
	// Put mirrors in links
	for _, m := range mirrors {
		if l, ok := linksCache[m.LinkID]; ok {
			l.Mirrors = append(l.Mirrors, m)
		}
	}

	return c.Render(http.StatusOK, "links", data)
}

func LinksDownloadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	fileName := "dkf_links.csv"

	// Captcha for bigger files
	var data captchaRequiredData
	data.CaptchaDescription = "Captcha required"
	data.CaptchaID, data.CaptchaImg = captcha.New()
	const captchaRequiredTmpl = "captcha-required"
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, captchaRequiredTmpl, data)
	}
	captchaID := c.Request().PostFormValue("captcha_id")
	captchaInput := c.Request().PostFormValue("captcha")
	if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
		data.ErrCaptcha = err.Error()
		return c.Render(http.StatusOK, captchaRequiredTmpl, data)
	}

	// Keep track of user downloads
	if _, err := db.CreateDownload(authUser.ID, fileName); err != nil {
		logrus.Error(err)
	}

	// Get all categories and make a hashmap for fast access
	categories, _ := db.GetLinksCategories()
	categoriesMap := make(map[int64]string)
	for _, category := range categories {
		categoriesMap[category.ID] = category.Name
	}
	// Get all "categories links" associations between links and their categories
	categoriesLinks, _ := db.GetCategoriesLinks()
	// Build a map of all categories IDs for a given link ID
	categoriesLinksMap := make(map[int64][]int64)
	for _, cl := range categoriesLinks {
		categoriesLinksMap[cl.LinkID] = append(categoriesLinksMap[cl.LinkID], cl.CategoryID)
	}

	links, _ := db.GetLinks()
	by := make([]byte, 0)
	buf := bytes.NewBuffer(by)
	w := csv.NewWriter(buf)
	_ = w.Write([]string{"UUID", "URL", "Title", "Description", "Categories"})
	for _, link := range links {
		// Get all categories for the link
		categoryNames := make([]string, 0)
		categoryIDs := categoriesLinksMap[link.ID]
		for _, tagID := range categoryIDs {
			categoryNames = append(categoryNames, categoriesMap[tagID])
		}
		_ = w.Write([]string{link.UUID, link.URL, link.Title, link.Description, strings.Join(categoryNames, ",")})
	}
	w.Flush()
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	return c.Stream(http.StatusOK, "application/octet-stream", buf)
}

func LinkHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	shorthand := c.Param("shorthand")
	linkUUID := c.Param("linkUUID")
	var data linkData
	var err error
	if shorthand != "" {
		data.Link, err = db.GetLinkByShorthand(shorthand)
	} else {
		data.Link, err = db.GetLinkByUUID(linkUUID)
	}
	if err != nil {
		return c.Redirect(http.StatusFound, "/links")
	}
	data.PgpKeys, _ = db.GetLinkPgps(data.Link.ID)
	data.Mirrors, _ = db.GetLinkMirrors(data.Link.ID)
	return c.Render(http.StatusOK, "link", data)
}

func RestoreLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, "/")
	}
	linkUUID := c.Param("linkUUID")
	var link database.Link
	if err := db.DB().Unscoped().First(&link, "uuid = ?", linkUUID).Error; err != nil {
		return hutils.RedirectReferer(c)
	}
	db.NewAudit(*authUser, fmt.Sprintf("restore link %s", link.URL))
	db.DB().Unscoped().Model(&database.Link{}).Where("id = ?", link.ID).Update("deleted_at", nil)
	return hutils.RedirectReferer(c)
}

func ClaimLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	linkUUID := c.Param("linkUUID")
	link, err := db.GetLinkByUUID(linkUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data claimLinkData
	data.Link = link
	data.Certificate = link.GenOwnershipCert(authUser.Username)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "link-claim", data)
	}

	data.Signature = c.Request().PostFormValue("signature")

	b64Sig, err := base64.StdEncoding.DecodeString(data.Signature)
	if err != nil {
		data.Error = "invalid signature"
		return c.Render(http.StatusOK, "link-claim", data)
	}
	pemSign := string(pem.EncodeToMemory(&pem.Block{Type: "SIGNATURE", Bytes: b64Sig}))

	isValid := utils.VerifyTorSign(link.GetOnionAddr(), data.Certificate, pemSign)
	if !isValid {
		data.Error = "invalid signature"
		return c.Render(http.StatusOK, "link-claim", data)
	}

	signedCert := "-----BEGIN SIGNED MESSAGE-----\n" +
		data.Certificate + "\n" +
		pemSign

	link.SignedCertificate = signedCert
	link.OwnerUserID = &authUser.ID
	link.DoSave(db)

	return c.Redirect(http.StatusFound, "/links/"+link.UUID)
}

func ClaimDownloadCertificateLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	linkUUID := c.Param("linkUUID")
	link, err := db.GetLinkByUUID(linkUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	fileName := "certificate.txt"

	// Keep track of user downloads
	if _, err := db.CreateDownload(authUser.ID, fileName); err != nil {
		logrus.Error(err)
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	return c.Stream(http.StatusOK, "application/octet-stream", strings.NewReader(link.GenOwnershipCert(authUser.Username)))
}

func ClaimCertificateLinkHandler(c echo.Context) error {
	linkUUID := c.Param("linkUUID")
	db := c.Get("database").(*database.DkfDB)
	link, err := db.GetLinkByUUID(linkUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	return c.String(http.StatusOK, link.SignedCertificate)
}

func EditLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, "/")
	}
	linkUUID := c.Param("linkUUID")
	link, err := db.GetLinkByUUID(linkUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	out, _ := db.GetLinkCategories(link.ID)
	categories := make([]string, 0)
	for _, el := range out {
		categories = append(categories, el.Name)
	}
	out1, err := db.GetLinkTags(link.ID)
	tags := make([]string, 0)
	for _, el := range out1 {
		tags = append(tags, el.Name)
	}
	var data editLinkData
	data.IsEdit = true
	data.Link = link.URL
	data.Title = link.Title
	data.Description = link.Description
	if link.Shorthand != nil {
		data.Shorthand = *link.Shorthand
	}
	data.Categories = strings.Join(categories, ",")
	data.Tags = strings.Join(tags, ",")
	data.Mirrors, _ = db.GetLinkMirrors(link.ID)
	data.LinkPgps, _ = db.GetLinkPgps(link.ID)
	//data.Categories = link

	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "createLink" {
			_ = db.DeleteLinkCategories(link.ID)
			_ = db.DeleteLinkTags(link.ID)

			// If link is signed, we can no longer edit the link URL
			if link.SignedCertificate == "" {
				data.Link = c.Request().PostFormValue("link")
			}
			data.Title = c.Request().PostFormValue("title")
			data.Description = c.Request().PostFormValue("description")
			data.Shorthand = c.Request().PostFormValue("shorthand")
			data.Categories = c.Request().PostFormValue("categories")
			data.Tags = c.Request().PostFormValue("tags")
			if !govalidator.Matches(data.Link, `^https?://[a-z2-7]{56}\.onion$`) {
				data.ErrorLink = "invalid link"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if !govalidator.RuneLength(data.Title, "0", "255") {
				data.ErrorTitle = "title must have 255 characters max"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if !govalidator.RuneLength(data.Description, "0", "1000") {
				data.ErrorCategories = "description must have 1000 characters max"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if data.Shorthand != "" {
				if !govalidator.Matches(data.Shorthand, `^[\w-_]{3,50}$`) {
					data.ErrorLink = "invalid shorthand"
					return c.Render(http.StatusOK, "new-link", data)
				}
			}
			categoryRgx := regexp.MustCompile(`^\w{3,20}$`)
			var tagsStr, categoriesStr []string
			if data.Categories != "" {
				categoriesStr = strings.Split(strings.ToLower(data.Categories), ",")
				for _, category := range categoriesStr {
					category = strings.TrimSpace(category)
					if !categoryRgx.MatchString(category) {
						data.ErrorCategories = `invalid category "` + category + `"`
						return c.Render(http.StatusOK, "new-link", data)
					}
				}
			}
			if data.Tags != "" {
				tagsStr = strings.Split(strings.ToLower(data.Tags), ",")
				for _, tag := range tagsStr {
					tag = strings.TrimSpace(tag)
					if !categoryRgx.MatchString(tag) {
						data.ErrorTags = `invalid tag "` + tag + `"`
						return c.Render(http.StatusOK, "new-link", data)
					}
				}
			}
			//------------
			var categories []database.LinksCategory
			var tags []database.LinksTag
			for _, categoryStr := range categoriesStr {
				category, _ := db.CreateLinksCategory(categoryStr)
				categories = append(categories, category)
			}
			for _, tagStr := range tagsStr {
				tag, _ := db.CreateLinksTag(tagStr)
				tags = append(tags, tag)
			}
			link.URL = data.Link
			link.Title = data.Title
			link.Description = data.Description
			if data.Shorthand != "" {
				link.Shorthand = &data.Shorthand
			}
			if err := db.DB().Save(&link).Error; err != nil {
				if strings.Contains(err.Error(), "UNIQUE constraint failed: links.shorthand") {
					data.ErrorShorthand = "shorthand already used"
				} else {
					data.ErrorLink = "failed to update link"
				}
				return c.Render(http.StatusOK, "new-link", data)
			}
			for _, category := range categories {
				_ = db.AddLinkCategory(link.ID, category.ID)
			}
			for _, tag := range tags {
				_ = db.AddLinkTag(link.ID, tag.ID)
			}
			db.NewAudit(*authUser, fmt.Sprintf("updated link %s", link.URL))
			return c.Redirect(http.StatusFound, "/links")

		} else if formName == "createPgp" {
			data.PGPTitle = c.Request().PostFormValue("pgp_title")
			if !govalidator.RuneLength(data.PGPTitle, "3", "255") {
				data.ErrorPGPTitle = "title must have 3-255 characters"
				return c.Render(http.StatusOK, "new-link", data)
			}
			data.PGPDescription = c.Request().PostFormValue("pgp_description")
			data.PGPPublicKey = c.Request().PostFormValue("pgp_public_key")
			if _, err = db.CreateLinkPgp(link.ID, data.PGPTitle, data.PGPDescription, data.PGPPublicKey); err != nil {
				logrus.Error(err)
			}
			db.NewAudit(*authUser, fmt.Sprintf("create gpg for link %s", link.URL))
			return hutils.RedirectReferer(c)

		} else if formName == "createMirror" {
			data.MirrorLink = c.Request().PostFormValue("mirror_link")
			if !govalidator.Matches(data.MirrorLink, `^https?://[a-z2-7]{56}\.onion$`) {
				data.ErrorMirrorLink = "invalid link"
				return c.Render(http.StatusOK, "new-link", data)
			}
			if _, err = db.CreateLinkMirror(link.ID, data.MirrorLink); err != nil {
				logrus.Error(err)
			}
			db.NewAudit(*authUser, fmt.Sprintf("create mirror for link %s", link.URL))
			return hutils.RedirectReferer(c)
		}
		return c.Redirect(http.StatusFound, "/links")
	}

	return c.Render(http.StatusOK, "new-link", data)
}

func LinkDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	linkUUID := c.Param("linkUUID")
	link, err := db.GetLinkByUUID(linkUUID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}

	if !authUser.IsModerator() {
		return hutils.RedirectReferer(c)
	}

	var data deleteLinkData
	data.Link = link

	if c.Request().Method == http.MethodPost {
		db.NewAudit(*authUser, fmt.Sprintf("deleted link %s", link.URL))
		if err := db.DeleteLinkByID(link.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/links")
	}

	return c.Render(http.StatusOK, "link-delete", data)
}

func LinkPgpDownloadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	pgpID := utils.DoParseInt64(c.Param("linkPgpID"))
	linkPgp, err := db.GetLinkPgpByID(pgpID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	fileName := linkPgp.Title + ".asc"

	// Keep track of user downloads
	if _, err := db.CreateDownload(authUser.ID, fileName); err != nil {
		logrus.Error(err)
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	return c.Stream(http.StatusOK, "application/octet-stream", strings.NewReader(linkPgp.PgpPublicKey))
}

func LinkPgpDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	linkPgpID := utils.DoParseInt64(c.Param("linkPgpID"))
	linkPgp, err := db.GetLinkPgpByID(linkPgpID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	link, err := db.GetLinkByID(linkPgp.LinkID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}

	if !authUser.IsModerator() {
		return hutils.RedirectReferer(c)
	}

	var data deleteLinkPgpData
	data.Link = link
	data.LinkPgp = linkPgp

	if c.Request().Method == http.MethodPost {
		if err := db.DeleteLinkPgpByID(linkPgp.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/links/"+link.UUID+"/edit")
	}

	return c.Render(http.StatusOK, "link-pgp-delete", data)
}

func LinkMirrorDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	linkMirrorID := utils.DoParseInt64(c.Param("linkMirrorID"))
	linkMirror, err := db.GetLinkMirrorByID(linkMirrorID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/links")
	}
	link, err := db.GetLinkByID(linkMirror.LinkID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/links")
	}

	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, "/links")
	}

	var data deleteLinkMirrorData
	data.Link = link
	data.LinkMirror = linkMirror

	if c.Request().Method == http.MethodPost {
		if err := db.DeleteLinkMirrorByID(linkMirror.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/links/"+link.UUID+"/edit")
	}

	return c.Render(http.StatusOK, "link-mirror-delete", data)
}

type CsvLink struct {
	URL   string
	Title string
}

func LinksUploadHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data linksUploadData
	if c.Request().Method == http.MethodPost {
		data.CsvStr = c.Request().PostFormValue("csv")
		getValidLinks := func() (out []CsvLink, err error) {
			r := csv.NewReader(strings.NewReader(data.CsvStr))
			records, err := r.ReadAll()
			if err != nil {
				return out, err
			}
			for idx, record := range records {
				link := strings.TrimSpace(strings.TrimRight(record[0], "/"))
				title := record[1]
				if !govalidator.Matches(link, `^https?://[a-z2-7]{56}\.onion$`) {
					return out, fmt.Errorf("invalid link %s", link)
				}
				if !govalidator.RuneLength(title, "0", "255") {
					return out, fmt.Errorf("title must have 255 characters max : record #%d", idx)
				}
				csvLink := CsvLink{
					URL:   link,
					Title: title,
				}
				out = append(out, csvLink)
			}
			return out, nil
		}
		csvLinks, err := getValidLinks()
		if err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "links-upload", data)
		}
		for _, csvLink := range csvLinks {
			_, err := db.CreateLink(csvLink.URL, csvLink.Title, "", "")
			if err != nil {
				logrus.Error(err)
			}
		}
		return hutils.RedirectReferer(c)
	}
	return c.Render(http.StatusOK, "links-upload", data)
}

func NewLinkHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.IsModerator() {
		return c.Redirect(http.StatusFound, "/")
	}
	var data newLinkData
	if c.Request().Method == http.MethodPost {
		data.Link = c.Request().PostFormValue("link")
		data.Title = c.Request().PostFormValue("title")
		data.Description = c.Request().PostFormValue("description")
		data.Shorthand = c.Request().PostFormValue("shorthand")
		data.Categories = c.Request().PostFormValue("categories")
		data.Tags = c.Request().PostFormValue("tags")
		if !govalidator.Matches(data.Link, `^https?://[a-z2-7]{56}\.onion$`) {
			data.ErrorLink = "invalid link"
			return c.Render(http.StatusOK, "new-link", data)
		}
		if !govalidator.RuneLength(data.Title, "0", "255") {
			data.ErrorTitle = "title must have 255 characters max"
			return c.Render(http.StatusOK, "new-link", data)
		}
		if !govalidator.RuneLength(data.Description, "0", "1000") {
			data.ErrorCategories = "description must have 1000 characters max"
			return c.Render(http.StatusOK, "new-link", data)
		}
		if data.Shorthand != "" {
			if !govalidator.Matches(data.Shorthand, `^[\w-_]{3,50}$`) {
				data.ErrorLink = "invalid shorthand"
				return c.Render(http.StatusOK, "new-link", data)
			}
		}
		categoryRgx := regexp.MustCompile(`^\w{3,20}$`)
		var tagsStr, categoriesStr []string
		if data.Categories != "" {
			categoriesStr = strings.Split(strings.ToLower(data.Categories), ",")
			for _, category := range categoriesStr {
				category = strings.TrimSpace(category)
				if !categoryRgx.MatchString(category) {
					data.ErrorCategories = `invalid category "` + category + `"`
					return c.Render(http.StatusOK, "new-link", data)
				}
			}
		}
		if data.Tags != "" {
			tagsStr = strings.Split(strings.ToLower(data.Tags), ",")
			for _, tag := range tagsStr {
				tag = strings.TrimSpace(tag)
				if !categoryRgx.MatchString(tag) {
					data.ErrorTags = `invalid tag "` + tag + `"`
					return c.Render(http.StatusOK, "new-link", data)
				}
			}
		}
		//------------
		var categories []database.LinksCategory
		var tags []database.LinksTag
		for _, categoryStr := range categoriesStr {
			category, _ := db.CreateLinksCategory(categoryStr)
			categories = append(categories, category)
		}
		for _, tagStr := range tagsStr {
			tag, _ := db.CreateLinksTag(tagStr)
			tags = append(tags, tag)
		}
		link, err := db.CreateLink(data.Link, data.Title, data.Description, data.Shorthand)
		if err != nil {
			logrus.Error(err)
			data.ErrorLink = "failed to create link"
			return c.Render(http.StatusOK, "new-link", data)
		}
		for _, category := range categories {
			_ = db.AddLinkCategory(link.ID, category.ID)
		}
		for _, tag := range tags {
			_ = db.AddLinkTag(link.ID, tag.ID)
		}
		db.NewAudit(*authUser, fmt.Sprintf("create link %s", link.URL))
		return c.Redirect(http.StatusFound, "/links")
	}
	return c.Render(http.StatusOK, "new-link", data)
}

func LinksClaimInstructionsHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "links-claim-instructions", nil)
}

func LinksReindexHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	if err := db.DB().Exec(`INSERT INTO fts5_links(fts5_links) VALUES('rebuild')`).Error; err != nil {
		logrus.Error(err)
	}
	db.DB().Exec(`delete from fts5_links where rowid in (select id from links where deleted_at is not null)`)
	return hutils.RedirectReferer(c)
}
