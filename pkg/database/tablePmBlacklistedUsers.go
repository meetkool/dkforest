package database

import (
	"github.com/sirupsen/logrus"
)

type PmBlacklistedUsers struct {
	UserID            UserID
	BlacklistedUserID UserID
	BlacklistedUser   User
}

// IsUserPmBlacklisted returns either or not toUserID blacklisted fromUserID
func IsUserPmBlacklisted(fromUserID, toUserID UserID) bool {
	var count int64
	DB.Model(&PmBlacklistedUsers{}).Where("blacklisted_user_id = ? AND user_id = ?", fromUserID, toUserID).Count(&count)
	return count == 1
}

// GetPmBlacklistedUsers returns a list of userID blacklisted users
func GetPmBlacklistedUsers(userID UserID) (out []PmBlacklistedUsers, err error) {
	err = DB.Where("user_id = ?", userID).Preload("BlacklistedUser").Find(&out).Error
	return
}

// GetPmBlacklistedByUsers returns a list of users that are blacklisting userID
func GetPmBlacklistedByUsers(userID UserID) (out []PmBlacklistedUsers, err error) {
	err = DB.Where("blacklisted_user_id = ?", userID).Find(&out).Error
	return
}

// ToggleBlacklistedUser returns true if the user was added to the blacklist
func ToggleBlacklistedUser(userID, blacklistedUserID UserID) bool {
	if IsUserPmBlacklisted(blacklistedUserID, userID) {
		RmBlacklistedUser(userID, blacklistedUserID)
		return false
	}
	AddBlacklistedUser(userID, blacklistedUserID)
	return true
}

func AddBlacklistedUser(userID, blacklistedUserID UserID) {
	ignore := PmBlacklistedUsers{UserID: userID, BlacklistedUserID: blacklistedUserID}
	if err := DB.Create(&ignore).Error; err != nil {
		logrus.Error(err)
	}
}

func RmBlacklistedUser(userID, blacklistedUserID UserID) {
	if err := DB.Delete(PmBlacklistedUsers{}, "user_id = ? AND blacklisted_user_id = ?", userID, blacklistedUserID).Error; err != nil {
		logrus.Error(err)
	}
}
