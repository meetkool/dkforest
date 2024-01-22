package global

import (
	"dkforest/pkg/cache"
	"dkforest/pkg/database"
	"time"
)

var notifCountCache = cache.NewWithKey[string, int64](30*time.Second, time.Minute)

// Notifications count cache has to be set per user session.
// Each session can have a different notif count.
// This is due to the fact that we notify other sessions of new successful
// logins on the user account.
func cacheKey(userID database.UserID, sessionToken string) string {
	return userID.String() + "_" + sessionToken
}

func DeleteUserNotificationCount(userID database.UserID, sessionToken string) {
	notifCountCache.Delete(cacheKey(userID, sessionToken))
}

func GetUserNotificationCount(db *database.DkfDB, userID database.UserID, sessionToken string) int64 {
	count, found := notifCountCache.Get(cacheKey(userID, sessionToken))
	if found {
		return count
	}
	count = db.GetUserInboxMessagesCount(userID)
	count += db.GetUserNotificationsCount(userID)
	// sessionToken can be empty when using the API
	if sessionToken != "" {
		count += db.GetUserSessionNotificationsCount(sessionToken)
	}
	notifCountCache.SetD(cacheKey(userID, sessionToken), count)
	return count
}
