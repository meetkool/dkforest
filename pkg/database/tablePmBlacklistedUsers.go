package database

import (
	"github.com/sirupsen/logrus"
)

type PmBlacklistedUsers struct {
	UserID            UserID
	BlacklistedUserID UserID
	BlacklistedUser   User
}

func (d *DkfDB) DeleteOldPmBlacklistedUsers() {
	if err := d.db.Exec(`DELETE FROM pm_blacklisted_users WHERE blacklisted_user_id IN (SELECT id FROM users u WHERE u.last_seen_at < date('now', '-90 Day'))`).Error; err != nil {
		logrus.Error(err)
	}
}

// IsUserPmBlacklisted returns either or not toUserID blacklisted fromUserID
func (d *DkfDB) IsUserPmBlacklisted(fromUserID, toUserID UserID) bool {
	var count int64
	d.db.Model(&PmBlacklistedUsers{}).Where("blacklisted_user_id = ? AND user_id = ?", fromUserID, toUserID).Count(&count)
	return count == 1
}

// GetPmBlacklistedUsers returns a list of userID blacklisted users
func (d *DkfDB) GetPmBlacklistedUsers(userID UserID) (out []PmBlacklistedUsers, err error) {
	err = d.db.Where("user_id = ?", userID).Preload("BlacklistedUser").Find(&out).Error
	return
}

// GetPmBlacklistedByUsers returns a list of users that are blacklisting userID
func (d *DkfDB) GetPmBlacklistedByUsers(userID UserID) (out []PmBlacklistedUsers, err error) {
	err = d.db.Where("blacklisted_user_id = ?", userID).Find(&out).Error
	return
}

// ToggleBlacklistedUser returns true if the user was added to the blacklist
func (d *DkfDB) ToggleBlacklistedUser(userID, blacklistedUserID UserID) bool {
	if d.IsUserPmBlacklisted(blacklistedUserID, userID) {
		d.RmBlacklistedUser(userID, blacklistedUserID)
		return false
	}
	d.AddBlacklistedUser(userID, blacklistedUserID)
	return true
}

func (d *DkfDB) AddBlacklistedUser(userID, blacklistedUserID UserID) {
	ignore := PmBlacklistedUsers{UserID: userID, BlacklistedUserID: blacklistedUserID}
	if err := d.db.Create(&ignore).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) RmBlacklistedUser(userID, blacklistedUserID UserID) {
	if err := d.db.Delete(PmBlacklistedUsers{}, "user_id = ? AND blacklisted_user_id = ?", userID, blacklistedUserID).Error; err != nil {
		logrus.Error(err)
	}
}
