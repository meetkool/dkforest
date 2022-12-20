package database

import (
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

func GetGistByUUID(uuid string) (out Gist, err error) {
	err = DB.First(&out, "uuid = ?", uuid).Error
	return
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
func (g *Gist) DoSave() {
	if err := DB.Save(g).Error; err != nil {
		logrus.Error(err)
	}
}
