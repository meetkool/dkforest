package database

import (
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

// CreateDownload ...
func CreateDownload(userID UserID, filename string) (out Download, err error) {
	out = Download{UserID: userID, Filename: filename}
	err = DB.Create(&out).Error
	return
}

// UserNbDownloaded returns how many times a user downloaded a file
func UserNbDownloaded(userID UserID, filename string) (out int64) {
	DB.Table("downloads").Where("user_id = ? AND filename = ?", userID, filename).Count(&out)
	return
}
