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
}

func (d *DkfDB) DeleteOldSessionNotifications() {
	if err := d.db.Delete(SessionNotification{}, "created_at < date('now', '-90 Day')").Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetUserNotifications(userID UserID) (msgs []Notification, err error) {
	err = d.db.Order("id DESC").
		Limit(50).
		Preload("User").
		Find(&msgs, "user_id = ?", userID).Error
	var ids []int64
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	now := time.Now()
	if err := d.db.Model(&Notification{}).Where("id IN (?)", ids).
		Updates(map[string]any{"is_read": true, "read_at": &now}).Error; err != nil {
		logrus.Error(err)
	}
	return
}

func (d *DkfDB) GetUserSessionNotifications(sessionToken string) (msgs []SessionNotification, err error) {
	err = d.db.Order("session_notifications.id DESC").
		Limit(50).
		Joins("INNER JOIN sessions s ON s.token = session_token").
		Joins("INNER JOIN users u ON u.id = s.user_id").
		Find(&msgs, "session_token = ?", sessionToken).Error
	var ids []int64
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	now := time.Now()
	if err := d.db.Table("session_notifications").Where("id IN (?)", ids).
		Updates(map[string]any{"is_read": true, "read_at": &now}).Error; err != nil {
		logrus.Error(err)
	}
	return
}

func (d *DkfDB) DeleteNotificationByID(notificationID int64) error {
	return d.db.Where("id = ?", notificationID).Delete(&Notification{}).Error
}

func (d *DkfDB) DeleteSessionNotificationByID(sessionNotificationID int64) error {
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
