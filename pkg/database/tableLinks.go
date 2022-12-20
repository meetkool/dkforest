package database

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"html"
	"strings"
	"time"
)

type Link struct {
	ID          int64
	UUID        string
	URL         string
	Title       string
	Description string
	Shorthand   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	Mirrors     []LinksMirror
}

func (l Link) DescriptionSafe() string {
	return html.EscapeString(l.Description)
}

func (l *Link) Save() error {
	return DB.Save(l).Error
}

func (l *Link) DoSave() {
	if err := DB.Debug().Save(l).Error; err != nil {
		logrus.Error(err)
	}
}

func CreateLink(url, title, description, shorthand string) (out Link, err error) {
	out = Link{UUID: uuid.New().String(), URL: url, Title: title, Description: description}
	if shorthand != "" {
		out.Shorthand = &shorthand
	}
	err = DB.FirstOrCreate(&out, "url = ?", url).Error
	return
}

func DeleteLinkByID(id int64) error {
	return DB.Where("id = ?", id).Delete(&Link{}).Error
}

func GetLinks() (out []Link, err error) {
	err = DB.Find(&out).Error
	return
}

func GetRecentLinks() (out []Link, err error) {
	err = DB.Order("id DESC").Limit(100).Find(&out).Error
	return
}

func GetLinkByShorthand(shorthand string) (out Link, err error) {
	err = DB.First(&out, "shorthand = ?", shorthand).Error
	return
}

func GetLinkByUUID(linkUUID string) (out Link, err error) {
	err = DB.First(&out, "uuid = ?", linkUUID).Error
	return
}

func GetLinkByID(linkID int64) (out Link, err error) {
	err = DB.First(&out, "id = ?", linkID).Error
	return
}

type LinksCategory struct {
	ID   int64
	Name string
}

func CreateLinksCategory(category string) (out LinksCategory, err error) {
	out = LinksCategory{Name: category}
	err = DB.FirstOrCreate(&out, "name = ?", category).Error
	return
}

type LinksTag struct {
	ID   int64
	Name string
}

func CreateLinksTag(tag string) (out LinksTag, err error) {
	out = LinksTag{Name: tag}
	err = DB.FirstOrCreate(&out, "name = ?", tag).Error
	return
}

type LinksTagsLink struct {
	LinkID int64
	TagID  int64
}

func AddLinkTag(linkID, tagID int64) (err error) {
	return DB.Create(&LinksTagsLink{LinkID: linkID, TagID: tagID}).Error
}

type LinksCategoriesLink struct {
	CategoryID int64
	LinkID     int64
}

func AddLinkCategory(linkID, categoryID int64) (err error) {
	return DB.Create(&LinksCategoriesLink{CategoryID: categoryID, LinkID: linkID}).Error
}

type CategoriesResult struct {
	Name  string
	Count int64
}

func GetCategories() (out []CategoriesResult, err error) {
	err = DB.Raw(`SELECT
c.name, count(cl.link_id) as count
FROM links_categories_links cl
INNER JOIN links_categories c ON c.id = cl.category_id
INNER JOIN links l ON l.id = cl.link_id AND l.deleted_at IS NULL
GROUP BY category_id
ORDER BY c.name`).Scan(&out).Error
	return
}

func GetLinkCategories(linkID int64) (out []LinksCategory, err error) {
	err = DB.Raw(`SELECT
c.id, c.name
FROM links_categories_links cl
INNER JOIN links_categories c ON c.id = cl.category_id
WHERE cl.link_id = ?
ORDER BY c.name`, linkID).Scan(&out).Error
	return
}

func GetLinkTags(linkID int64) (out []LinksTag, err error) {
	err = DB.Raw(`SELECT
t.id, t.name
FROM links_tags_links tl
INNER JOIN links_tags t ON t.id = tl.tag_id
WHERE tl.link_id = ?
ORDER BY t.name`, linkID).Scan(&out).Error
	return
}

func DeleteLinkCategories(linkID int64) error {
	return DB.Delete(&LinksCategoriesLink{}, "link_id = ?", linkID).Error
}

func DeleteLinkTags(linkID int64) error {
	return DB.Delete(&LinksTagsLink{}, "link_id = ?", linkID).Error
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
	reader := bytes.NewReader([]byte(l.PgpPublicKey))
	if block, err := armor.Decode(reader); err == nil {
		r := packet.NewReader(block.Body)
		if e, err := openpgp.ReadEntity(r); err == nil {
			return e.PrimaryKey.KeyIdString()
		}
	}
	return "n/a"
}

func (l LinksPgp) GetKeyFingerprint() string {
	reader := bytes.NewReader([]byte(l.PgpPublicKey))
	if block, err := armor.Decode(reader); err == nil {
		r := packet.NewReader(block.Body)
		if e, err := openpgp.ReadEntity(r); err == nil {
			fp := strings.ToUpper(hex.EncodeToString(e.PrimaryKey.Fingerprint))
			return fmt.Sprintf("%s %s %s %s %s  %s %s %s %s %s",
				fp[0:4], fp[4:8], fp[8:12], fp[12:16], fp[16:20],
				fp[20:24], fp[24:28], fp[28:32], fp[32:36], fp[36:40])
		}
	}
	return "n/a"
}

func CreateLinkPgp(linkID int64, title, description, publicKey string) (out LinksPgp, err error) {
	out = LinksPgp{
		LinkID:       linkID,
		Title:        title,
		Description:  description,
		PgpPublicKey: publicKey,
	}
	err = DB.Create(&out).Error
	return
}

func CreateLinkMirror(linkID int64, link string) (out LinksMirror, err error) {
	out = LinksMirror{
		LinkID:    linkID,
		MirrorURL: link,
	}
	err = DB.Create(&out).Error
	return
}

func GetLinkPgps(linkID int64) (out []LinksPgp, err error) {
	err = DB.Find(&out, "link_id = ?", linkID).Error
	return
}

func GetLinkMirrors(linkID int64) (out []LinksMirror, err error) {
	err = DB.Find(&out, "link_id = ?", linkID).Error
	return
}

func GetLinkPgpByID(id int64) (out LinksPgp, err error) {
	err = DB.First(&out, "id = ?", id).Error
	return
}

func GetLinkMirrorByID(id int64) (out LinksMirror, err error) {
	err = DB.First(&out, "id = ?", id).Error
	return
}

func DeleteLinkPgpByID(id int64) error {
	return DB.Where("id = ?", id).Delete(&LinksPgp{}).Error
}

func DeleteLinkMirrorByID(id int64) error {
	return DB.Where("id = ?", id).Delete(&LinksMirror{}).Error
}
