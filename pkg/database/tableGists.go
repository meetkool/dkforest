package database

import (
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"time"

	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

type Gist struct {
	ID        int64
	UUID      string
	UserID    UserID
	User      User
	Name      string
	Content   string
	Password  string
	CreatedAt time.Time
}

func (d *DkfDB) GetGistByUUID(uuid string) (out Gist, err error) {
	err = d.db.First(&out, "uuid = ?", uuid).Error
	return
}

func GetGistPasswordHash(password string) string {
	return utils.Sha512(getGistSaltedPasswordBytes(password))
}

func getGistSaltedPasswordBytes(password string) []byte {
	return getSaltedPasswordBytes(config.GistPasswordSalt, password)
}

func (g *Gist) HasAccess(c echo.Context) bool {
	if g.Password == "" {
		return true
	}
	cookie, err := hutils.GetGistCookie(c, g.UUID)
	if err != nil {
		return false
	}
	if cookie.Value != g.Password {
		hutils.DeleteGistCookie(c, g.UUID)
		return false
	}
	return true
}

// DoSave user in the database, ignore error
func (g *Gist) DoSave(db *DkfDB) {
	if err := db.db.Save(g).Error; err != nil {
		logrus.Error(err)
	}
}
