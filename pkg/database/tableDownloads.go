package database

import (
	"github.com/sirupsen/logrus"
	"time"
)

// Download table that keep tracks of downloaded files by the users.
type Download struct {
	ID        int64
	UserID    UserID
	User      User
	Filename  string
	CreatedAt time.Time
}

func (d *DkfDB) DeleteOldDownloads() {
	if err := d.db.Delete(Download{}, "created_at < date('now', '-90 Day')").Error; err != nil {
		logrus.Error(err)
	}
}

// CreateDownload ...
func (d *DkfDB) CreateDownload(userID UserID, filename string) (out Download, err error) {
	out = Download{UserID: userID, Filename: filename}
	err = d.db.Create(&out).Error
	return
}

// UserNbDownloaded returns how many times a user downloaded a file
func (d *DkfDB) UserNbDownloaded(userID UserID, filename string) (out int64) {
	d.db.Table("downloads").Where("user_id = ? AND filename = ?", userID, filename).Count(&out)
	return
}

func (d *DkfDB) DeleteDownloadByID(downloadID int64) (err error) {
	err = d.db.Unscoped().Delete(Download{}, "id = ?", downloadID).Error
	return
}
