package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type IgnoredUser struct {
	UserID        UserID
	IgnoredUserID UserID
	CreatedAt     time.Time
	User          User
	IgnoredUser   User
}

func GetIgnoredUsers(userID UserID) (out []IgnoredUser, err error) {
	err = DB.Where("user_id = ?", userID).Preload("IgnoredUser").Find(&out).Error
	return
}

// GetIgnoredByUsers get a list of people who ignore userID
func GetIgnoredByUsers(userID UserID) (out []IgnoredUser, err error) {
	err = DB.Where("ignored_user_id = ?", userID).Find(&out).Error
	return
}

func IgnoreUser(userID, ignoredUserID UserID) {
	ignore := IgnoredUser{UserID: userID, IgnoredUserID: ignoredUserID}
	if err := DB.Create(&ignore).Error; err != nil {
		logrus.Error(err)
	}
}

func UnIgnoreUser(userID, ignoredUserID UserID) {
	if err := DB.Delete(IgnoredUser{}, "user_id = ? AND ignored_user_id = ?", userID, ignoredUserID).Error; err != nil {
		logrus.Error(err)
	}
}
