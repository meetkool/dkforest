package database

import (
	"github.com/sirupsen/logrus"
)

type PmWhitelistedUsers struct {
	UserID            UserID
	WhitelistedUserID UserID
	WhitelistedUser   User
}

func (d *DkfDB) DeleteOldPmWhitelistedUsers() {
	if err := d.db.Exec(`DELETE FROM pm_whitelisted_users WHERE whitelisted_user_id IN (SELECT u.id FROM users u WHERE u.last_seen_at < date('now', '-90 Day'))`).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) IsUserPmWhitelisted(fromUserID, toUserID UserID) bool {
	var count int64
	d.db.Model(&PmWhitelistedUsers{}).Where("whitelisted_user_id = ? AND user_id = ?", fromUserID, toUserID).Count(&count)
	return count == 1
}

func (d *DkfDB) GetPmWhitelistedUsers(userID UserID) (out []PmWhitelistedUsers, err error) {
	err = d.db.Where("user_id = ?", userID).Preload("WhitelistedUser").Find(&out).Error
	return
}

// ToggleWhitelistedUser returns true if the user was added to the whitelist
func (d *DkfDB) ToggleWhitelistedUser(userID, whitelistedUserID UserID) bool {
	if d.IsUserPmWhitelisted(whitelistedUserID, userID) {
		d.RmWhitelistedUser(userID, whitelistedUserID)
		return false
	}
	d.AddWhitelistedUser(userID, whitelistedUserID)
	return true
}

func (d *DkfDB) AddWhitelistedUser(userID, whitelistedUserID UserID) {
	ignore := PmWhitelistedUsers{UserID: userID, WhitelistedUserID: whitelistedUserID}
	if err := d.db.Create(&ignore).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) RmWhitelistedUser(userID, whitelistedUserID UserID) {
	if err := d.db.Delete(PmWhitelistedUsers{}, "user_id = ? AND whitelisted_user_id = ?", userID, whitelistedUserID).Error; err != nil {
		logrus.Error(err)
	}
}
