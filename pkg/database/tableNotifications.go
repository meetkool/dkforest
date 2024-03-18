package database

import (
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type UserID = int64
type SessionNotificationID = int64

type Notification struct {
	ID        UserID
	Message   string
	UserID    UserID
	IsRead    bool
	ReadAt    *time.Time
	CreatedAt time.Time
	User      User
}

type SessionNotification struct {
	ID           SessionNotificationID
	Message      string
	SessionToken string
	IsRead       bool
	ReadAt       *time.Time
	CreatedAt    time.Time
}

func (d *DkfDB) DeleteOldSessionNotifications() error {
	return d.db.Delete(&SessionNotification{}, "created_at < date('now', '-90 Day')").Error
}

func (d *DkfDB) GetUserNotifications(userID UserID) ([]Notification, error) {
	var msgs []Notification
	err := d.db.Table("notifications").
		Order("id DESC").
		Limit(50).
		Preload("User").
		Find(&msgs, "user_id = ?", userID).Error
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

func (d *DkfDB) markNotificationsRead(ids []int64, now *time.Time) error {
	return d.db.Model(&Notification{}).
		Where("id IN (?)", ids).
		Updates(map[string]any{"is_read": true, "read_at": now}).Error
}

func (d *DkfDB) GetUserSessionNotifications(sessionToken string) ([]SessionNotification, error) {
	var msgs []SessionNotification
	err := d.db.Table("session_notifications").
		Order("session_notifications.id DESC").
		Limit(50).
		Joins("INNER JOIN sessions ON sessions.token = session_notifications.session_token").
		Joins("INNER JOIN users ON users.id = sessions.user_id").
		Find(&msgs, "session_notifications.session_token = ?", sessionToken).Error
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

func (d *DkfDB) DeleteNotificationByID(notificationID UserID) error {
	return d.db.Where("id = ?", notificationID).Delete(&Notification{}).Error
}

func (d *DkfDB) DeleteSessionNotificationByID(sessionNotificationID SessionNotificationID) error {
	return d.db.Where("id = ?", sessionNotificationID).Delete(&SessionNotification{}).Error
}

func (d *DkfDB) DeleteAllNotifications(userID UserID) error {
	return d.db.Where("user_id = ?", userID).Delete(&Notification{}).Error
}

func (d *DkfDB) CreateNotification(msg string, userID UserID) {
	inbox := Notification{Message: msg, UserID: userID, IsRead: false}
	if err := d.db.Create(&inbox).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetUserNotificationsCount(userID UserID) (count int64) {
	d.db.Table("notifications").Where("user_id = ? AND is_read = ?", userID, false).Count(&count)
	return
}

func (d *DkfDB) GetUserSessionNotificationsCount(sessionToken string) (count int64) {
	d.db.Table("session_notifications").Where("session_token = ? AND is_read = ?", sessionToken, false).Count(&count)
	return
}

func (d *DkfDB) CreateSessionNotification(msg string, sessionToken string) {
	inbox := SessionNotification{Message: msg, SessionToken: sessionToken, IsRead: false}
	if err := d.db.Create(&inbox).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) DeleteAllSessionNotifications(sessionToken string) error {
	return d.db.Where("session_token = ?", sessionToken).Delete(&SessionNotification{}).Error
}

