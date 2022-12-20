package global

import (
	"dkforest/pkg/cache"
	"dkforest/pkg/database"
	"time"
)

var notifCountCache = cache.NewWithKey[database.UserID, int64](30*time.Second, time.Minute)

func DeleteUserNotificationCount(userID database.UserID) {
	notifCountCache.Delete(userID)
}

func GetUserNotificationCount(userID database.UserID, sessionToken string) int64 {
	count, found := notifCountCache.Get(userID)
	if found {
		return count
	}
	count = database.GetUserInboxMessagesCount(userID)
	count += database.GetUserNotificationsCount(userID)
	if sessionToken != "" {
		count += database.GetUserSessionNotificationsCount(sessionToken)
	}
	notifCountCache.SetD(userID, count)
	return count
}
