package database

import (
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type IgnoredUser struct {
	gorm.Model
	UserID        UserID
	IgnoredUserID UserID
	User          User
	IgnoredUser   User
}

func (d *DkfDB) DeleteOldIgnoredUsers() {
	err := d.db.Exec(`DELETE FROM ignored_users WHERE ignored_user_id IN (
		SELECT iu.ignored_user_id FROM ignored_users iu INNER JOIN users u ON u.id = iu.ignored_user_id
		WHERE u.last_seen_at < date('now', '-60 Day')
	)`).Error
	if err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetIgnoredUsers(userID UserID) ([]IgnoredUser, error) {
	var ignoredUsers []IgnoredUser
	err := d.db.Where("user_id = ?", userID).Preload("IgnoredUser").Find(&ignoredUsers).Error
	return ignoredUsers, err
}

func (d *DkfDB) GetIgnoredUsersUsernames(userID UserID) ([]Username, error) {
	var usernames []Username
	err := d.db.Model(&IgnoredUser{}).
		Joins("INNER JOIN users ON users.id = ignored_users.ignored_user_id").
		Where("ignored_users.user_id = ?", userID).
		Pluck("users.username", &usernames).
		Error
	return usernames, err
}

func (d *DkfDB) GetIgnoredUsersIDs(userID UserID) ([]UserID, error) {
	var userIDs []UserID
	err := d.db.Model(&IgnoredUser{}).
		Joins("INNER JOIN users ON users.id = ignored_users.ignored_user_id").
		Where("ignored_users.user_id = ?", userID).
		Pluck("users.id", &userIDs).
		Error
	return userIDs, err
}

func (d *DkfDB) GetIgnoredByUsers(userID UserID) ([]IgnoredUser, error) {
	var ignoredByUsers []IgnoredUser
	err := d.db.Where("ignored_user_id = ?", userID).Find(&ignoredByUsers).Error
	return ignoredByUsers, err
}

func (d *DkfDB) IgnoreUser(userID, ignoredUserID UserID) error {
	ignore := IgnoredUser{UserID: userID, IgnoredUserID: ignoredUserID}
	return d.db.Create(&ignore).Error
}

func (d *DkfDB) UnIgnoreUser(userID, ignoredUserID UserID) error {
	return d.db.Delete(&IgnoredUser{}, "user_id = ? AND ignored_
