package database

import (
	"github.com/sirupsen/logrus"
)

type PmWhitelistedUsers struct {
	UserID            UserID
	WhitelistedUserID UserID
	WhitelistedUser   User
}

func IsUserPmWhitelisted(fromUserID, toUserID UserID) bool {
	var count int64
	DB.Model(&PmWhitelistedUsers{}).Where("whitelisted_user_id = ? AND user_id = ?", fromUserID, toUserID).Count(&count)
	return count == 1
}

func GetPmWhitelistedUsers(userID UserID) (out []PmWhitelistedUsers, err error) {
	err = DB.Where("user_id = ?", userID).Preload("WhitelistedUser").Find(&out).Error
	return
}

// ToggleWhitelistedUser returns true if the user was added to the whitelist
func ToggleWhitelistedUser(userID, whitelistedUserID UserID) bool {
	if IsUserPmWhitelisted(whitelistedUserID, userID) {
		RmWhitelistedUser(userID, whitelistedUserID)
		return false
	}
	AddWhitelistedUser(userID, whitelistedUserID)
	return true
}

func AddWhitelistedUser(userID, whitelistedUserID UserID) {
	ignore := PmWhitelistedUsers{UserID: userID, WhitelistedUserID: whitelistedUserID}
	if err := DB.Create(&ignore).Error; err != nil {
		logrus.Error(err)
	}
}

func RmWhitelistedUser(userID, whitelistedUserID UserID) {
	if err := DB.Delete(PmWhitelistedUsers{}, "user_id = ? AND whitelisted_user_id = ?", userID, whitelistedUserID).Error; err != nil {
		logrus.Error(err)
	}
}
