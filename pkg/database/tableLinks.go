package database

import (
	"dkforest/pkg/utils"
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"html"
	"regexp"
	"time"
)

type Link struct {
	ID                int64
	UUID              string
	URL               string
	Title             string
	Description       string
	Shorthand         *string
	SignedCertificate string
	OwnerUserID       *UserID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
	Mirrors           []LinksMirror
	OwnerUser         *User
}

func (l Link) GenOwnershipCert(signerUsername Username) string {
	return fmt.Sprintf(""+
		"DarkForest ownership certificate\n"+
		"\n"+
		"For the following onion address:\n"+
		"%s\n"+
		"\n"+
		"Signed by: @%s\n"+
		"Signed on: %s",
		l.GetOnionAddr(),
		signerUsername,
		time.Now().UTC().Format("January 02, 2006"))
}

func (l Link) GetOnionAddr() string {
	var onionV3Rgx = regexp.MustCompile(`[a-z2-7]{56}\.onion`)
	return onionV3Rgx.FindString(l.URL)
}

func (l Link) DescriptionSafe() string {
	return html.EscapeString(l.Description)
}

func (l *Link) Save(db *DkfDB) error {
	return db.db.Save(l).Error
}

func (l *Link) DoSave(db *DkfDB) {
	if err := l.Save(db); err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) CreateLink(url, title, description, shorthand string) (out Link, err error) {
	out = Link{UUID: uuid.New().String(), URL: url, Title: title, Description: description}
	if shorthand != "" {
		out.Shorthand = &shorthand
	}
	err = d.db.FirstOrCreate(&out, "url = ?", url).Error
	return
}

func (d *DkfDB) DeleteLinkByID(id int64) error {
	return d.db.Where("id = ?", id).Delete(&Link{}).Error
}

func (d *DkfDB) GetLinks() (out []Link, err error) {
	err = d.db.Find(&out).Error
	return
}

func (d *DkfDB) GetRecentLinks() (out []Link, err error) {
	err = d.db.Order("id DESC").Limit(100).Find(&out).Error
	return
}

func (d *DkfDB) GetLinkByShorthand(shorthand string) (out Link, err error) {
	err = d.db.Preload("OwnerUser").First(&out, "shorthand = ?", shorthand).Error
	return
}

func (d *DkfDB) GetLinkByUUID(linkUUID string) (out Link, err error) {
	err = d.db.Preload("OwnerUser").First(&out, "uuid = ?", linkUUID).Error
	return
}

func (d *DkfDB) GetLinkByID(linkID int64) (out Link, err error) {
	err = d.db.First(&out, "id = ?", linkID).Error
	return
}

type LinksCategory struct {
	ID   int64
	Name string
}

func (d *DkfDB) CreateLinksCategory(category string) (out LinksCategory, err error) {
	out = LinksCategory{Name: category}
	err = d.db.FirstOrCreate(&out, "name = ?", category).Error
	return
}

type LinksTag struct {
	ID   int64
	Name string
}

func (d *DkfDB) CreateLinksTag(tag string) (out LinksTag, err error) {
	out = LinksTag{Name: tag}
	err = d.db.FirstOrCreate(&out, "name = ?", tag).Error
	return
}

type LinksTagsLink struct {
	LinkID int64
	TagID  int64
}

func (d *DkfDB) AddLinkTag(linkID, tagID int64) (err error) {
	return d.db.Create(&LinksTagsLink{LinkID: linkID, TagID: tagID}).Error
}

type LinksCategoriesLink struct {
	CategoryID int64
	LinkID     int64
}

func (d *DkfDB) AddLinkCategory(linkID, categoryID int64) (err error) {
	return d.db.Create(&LinksCategoriesLink{CategoryID: categoryID, LinkID: linkID}).Error
}

type CategoriesResult struct {
	Name  string
	Count int64
}

func (d *DkfDB) GetCategories() (out []CategoriesResult, err error) {
	err = d.db.Raw(`SELECT
c.name, count(cl.link_id) as count
FROM links_categories_links cl
INNER JOIN links_categories c ON c.id = cl.category_id
INNER JOIN links l ON l.id = cl.link_id AND l.deleted_at IS NULL
GROUP BY category_id
ORDER BY c.name`).Scan(&out).Error
	return
}

func (d *DkfDB) GetLinkCategories(linkID int64) (out []LinksCategory, err error) {
	err = d.db.Raw(`SELECT
c.id, c.name
FROM links_categories_links cl
INNER JOIN links_categories c ON c.id = cl.category_id
WHERE cl.link_id = ?
ORDER BY c.name`, linkID).Scan(&out).Error
	return
}

func (d *DkfDB) GetLinkTags(linkID int64) (out []LinksTag, err error) {
	err = d.db.Raw(`SELECT
t.id, t.name
FROM links_tags_links tl
INNER JOIN links_tags t ON t.id = tl.tag_id
WHERE tl.link_id = ?
ORDER BY t.name`, linkID).Scan(&out).Error
	return
}

// LinksCategoriesLinks many-to-many table
type LinksCategoriesLinks struct {
	LinkID     int64
	CategoryID int64
}

func (d *DkfDB) GetTags() (out []LinksTag, err error) {
	err = d.db.Find(&out).Error
	return
}

func (d *DkfDB) GetLinksCategories() (out []LinksCategory, err error) {
	err = d.db.Find(&out).Error
	return
}

func (d *DkfDB) GetCategoriesLinks() (out []LinksCategoriesLinks, err error) {
	err = d.db.Find(&out).Error
	return
}

func (d *DkfDB) DeleteLinkCategories(linkID int64) error {
	return d.db.Delete(&LinksCategoriesLink{}, "link_id = ?", linkID).Error
}

func (d *DkfDB) DeleteLinkTags(linkID int64) error {
	return d.db.Delete(&LinksTagsLink{}, "link_id = ?", linkID).Error
}

type LinksMirror struct {
	ID        int64
	LinkID    int64
	Idx       int64
	MirrorURL string
}

type LinksPgp struct {
	ID           int64
	LinkID       int64
	Idx          int64
	Title        string
	Description  string
	PgpPublicKey string
}

func (l LinksPgp) GetKeyID() string {
	if e := utils.GetEntityFromPKey(l.PgpPublicKey); e != nil {
		return e.PrimaryKey.KeyIdString()
	}
	return "n/a"
}

func (l LinksPgp) GetKeyFingerprint() string {
	out := "n/a"
	if fingerprint := utils.GetKeyFingerprint(l.PgpPublicKey); fingerprint != "" {
		out = fingerprint
	}
	return out
}

func (d *DkfDB) CreateLinkPgp(linkID int64, title, description, publicKey string) (out LinksPgp, err error) {
	out = LinksPgp{
		LinkID:       linkID,
		Title:        title,
		Description:  description,
		PgpPublicKey: publicKey,
	}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) CreateLinkMirror(linkID int64, link string) (out LinksMirror, err error) {
	out = LinksMirror{
		LinkID:    linkID,
		MirrorURL: link,
	}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) GetLinkPgps(linkID int64) (out []LinksPgp, err error) {
	err = d.db.Find(&out, "link_id = ?", linkID).Error
	return
}

func (d *DkfDB) GetLinkMirrors(linkID int64) (out []LinksMirror, err error) {
	err = d.db.Find(&out, "link_id = ?", linkID).Error
	return
}

func (d *DkfDB) GetLinkPgpByID(id int64) (out LinksPgp, err error) {
	err = d.db.First(&out, "id = ?", id).Error
	return
}

func (d *DkfDB) GetLinkMirrorByID(id int64) (out LinksMirror, err error) {
	err = d.db.First(&out, "id = ?", id).Error
	return
}

func (d *DkfDB) DeleteLinkPgpByID(id int64) error {
	return d.db.Where("id = ?", id).Delete(&LinksPgp{}).Error
}

func (d *DkfDB) DeleteLinkMirrorByID(id int64) error {
	return d.db.Where("id = ?", id).Delete(&LinksMirror{}).Error
}
