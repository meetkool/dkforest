package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type Notification struct {
	ID        int64
	Message   string
	UserID    UserID
	IsRead    bool
	ReadAt    *time.Time
	CreatedAt time.Time
	User      User
}

type SessionNotification struct {
	ID           int64
	Message      string
	SessionToken string
	IsRead       bool
	ReadAt       *time.Time
	CreatedAt    time.Time
	User         User
}

func GetUserNotifications(userID UserID) (msgs []Notification, err error) {
	err = DB.Order("id DESC").
		Limit(50).
		Preload("User").
		Find(&msgs, "user_id = ?", userID).Error
	var ids []int64
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	now := time.Now()
	if err := DB.Model(&Notification{}).Where("id IN (?)", ids).
		UpdateColumn("is_read", true, "read_at", &now).Error; err != nil {
		logrus.Error(err)
	}
	return
}

func GetUserSessionNotifications(sessionToken string) (msgs []SessionNotification, err error) {
	err = DB.Order("id DESC").
		Limit(50).
		Joins("INNER JOIN sessions s ON s.token = session_token").
		Joins("INNER JOIN users u ON u.id = s.user_id").
		Find(&msgs, "session_token = ?", sessionToken).Error
	var ids []int64
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	now := time.Now()
	if err := DB.Model(&SessionNotification{}).Where("id IN (?)", ids).
		UpdateColumn("is_read", true, "read_at", &now).Error; err != nil {
		logrus.Error(err)
	}
	return
}

func DeleteNotificationByID(notificationID int64) error {
	return DB.Where("id = ?", notificationID).Delete(&Notification{}).Error
}

func DeleteSessionNotificationByID(sessionNotificationID int64) error {
	return DB.Where("id = ?", sessionNotificationID).Delete(&SessionNotification{}).Error
}

func DeleteAllNotifications(userID UserID) error {
	return DB.Where("user_id = ?", userID).Delete(&Notification{}).Error
}

func CreateNotification(msg string, userID UserID) {
	inbox := Notification{Message: msg, UserID: userID, IsRead: false}
	if err := DB.Create(&inbox).Error; err != nil {
		logrus.Error(err)
	}
}

func GetUserNotificationsCount(userID UserID) (count int64) {
	DB.Table("notifications").Where("user_id = ? AND is_read = ?", userID, false).Count(&count)
	return
}

func GetUserSessionNotificationsCount(sessionToken string) (count int64) {
	DB.Table("session_notifications").Where("session_token = ? AND is_read = ?", sessionToken, false).Count(&count)
	return
}

func CreateSessionNotification(msg string, sessionToken string) {
	inbox := SessionNotification{Message: msg, SessionToken: sessionToken, IsRead: false}
	if err := DB.Create(&inbox).Error; err != nil {
		logrus.Error(err)
	}
}

func DeleteAllSessionNotifications(sessionToken string) error {
	return DB.Where("session_token = ?", sessionToken).Delete(&SessionNotification{}).Error
}
