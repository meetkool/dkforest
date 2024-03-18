package database

import (
	"github.com/sirupsen/logrus"
)

type PmBlacklistedUsers struct {
	UserID            UserID
	BlacklistedUserID UserID
	BlacklistedUser   User
}

// DeleteOldPmBlacklistedUsers deletes old pm blacklisted users
func (d *DkfDB) DeleteOldPmBlacklistedUsers() {
	deleteQuery := `DELETE FROM pm_blacklisted_users WHERE blacklisted_user_id IN (SELECT id FROM users WHERE last_seen_at < date('now', '-90 Day'))`
	if err := d.db.Exec(deleteQuery).Error; err != nil {
		logrus.Error(err)
	}
}

// IsUserPmBlacklisted checks if a user is blacklisted
func (d *DkfDB) IsUserPmBlacklisted(fromUserID, toUserID UserID) bool {
	var count int64
	err := d.db.Model(&PmBlacklistedUsers{}).Where("blacklisted_user_id = ? AND user_id = ?", fromUserID, toUserID).Count(&count).Error
	if err != nil {
		logrus.Error(err)
		return false
	}
	return count == 1
}

// GetPmBlacklistedUsers returns a list of blacklisted users for a given user
func (d *DkfDB) GetPmBlacklistedUsers(userID UserID) ([]PmBlacklistedUsers, error) {
	var out []PmBlacklistedUsers
	err := d.db.Where("user_id = ?", userID).Preload("BlacklistedUser").Find(&out).Error
	return out, err
}

// GetPmBlacklistedByUsers returns a list of users that have blacklisted the given user
func (d *DkfDB) GetPmBlacklistedByUsers(userID UserID) ([]PmBlacklistedUsers, error) {
	var out []PmBlacklistedUsers
	err := d.db.Where("blacklisted_user_id = ?", userID).Find(&out).Error
	return out, err
}

// ToggleBlacklistedUser toggles a user's blacklist status
func (d *DkfDB) ToggleBlacklistedUser(userID, blacklistedUserID UserID) (bool, error) {
	isBlacklisted := d.IsUserPmBlacklisted(blacklistedUserID, userID)
	if isBlacklisted {
		err := d.RmBlacklistedUser(userID, blacklistedUserID)
		return false, err
	}
	err := d.AddBlacklistedUser(userID, blacklistedUserID)
	return true, err
}

// AddBlacklistedUser adds a user to the blacklist
func (d *DkfDB) AddBlacklistedUser(userID, blacklistedUserID UserID) error {
	ignore := PmBlacklistedUsers{UserID: userID, BlacklistedUserID: blacklistedUserID}
	return d.db.Create(&ignore).Error
}

// RmBlacklistedUser removes a user from the blacklist
func (d *DkfDB) RmBlacklistedUser(userID, blacklistedUserID UserID) error {
	return d.db.Delete(&PmBlacklistedUsers{}, "user_id = ? AND blacklisted_user_id = ?", userID
