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

func (d *DkfDB) DeleteOldIgnoredUsers() {
	if err := d.db.Exec(`DELETE FROM ignored_users WHERE ignored_user_id IN
	(SELECT iu.ignored_user_id FROM ignored_users iu INNER JOIN users u ON u.id = iu.ignored_user_id WHERE u.last_seen_at < date('now', '-60 Day'))`).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetIgnoredUsers(userID UserID) (out []IgnoredUser, err error) {
	err = d.db.Where("user_id = ?", userID).Preload("IgnoredUser").Find(&out).Error
	return
}

func (d *DkfDB) GetIgnoredUsersUsernames(userID UserID) (out []Username, err error) {
	err = d.db.Model(&IgnoredUser{}).
		Joins("INNER JOIN users ON users.id = ignored_users.ignored_user_id").
		Where("ignored_users.user_id = ?", userID).
		Pluck("users.username", &out).
		Error
	return
}

func (d *DkfDB) GetIgnoredUsersIDs(userID UserID) (out []UserID, err error) {
	err = d.db.Model(&IgnoredUser{}).
		Joins("INNER JOIN users ON users.id = ignored_users.ignored_user_id").
		Where("ignored_users.user_id = ?", userID).
		Pluck("users.id", &out).
		Error
	return
}

// GetIgnoredByUsers get a list of people who ignore userID
func (d *DkfDB) GetIgnoredByUsers(userID UserID) (out []IgnoredUser, err error) {
	err = d.db.Where("ignored_user_id = ?", userID).Find(&out).Error
	return
}

func (d *DkfDB) IgnoreUser(userID, ignoredUserID UserID) {
	ignore := IgnoredUser{UserID: userID, IgnoredUserID: ignoredUserID}
	if err := d.db.Create(&ignore).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) UnIgnoreUser(userID, ignoredUserID UserID) {
	if err := d.db.Delete(IgnoredUser{}, "user_id = ? AND ignored_user_id = ?", userID, ignoredUserID).Error; err != nil {
		logrus.Error(err)
	}
}
