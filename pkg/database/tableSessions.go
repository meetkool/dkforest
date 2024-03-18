package database

import (
	"github.com/sirupsen/logrus"
	"time"

	"dkforest/pkg/utils"
)

// Session table
type Session struct {
	Token         string    `gorm:"size:32;not null" json:"token"`
	ExpiresAt     time.Time `gorm:"not null" json:"expires_at"`
	DeletedAt     *time.Time
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UserID        UserID     `gorm:"not null" json:"user_id"`
	ClientIP      string    `gorm:"size:45" json:"client_ip"`
	UserAgent     string    `gorm:"size:255" json:"user_agent"`
	User          User      `gorm:"foreignkey:UserID;association_autoupdate:false;association_autocreate:false" json:"user,omitempty"`
	AssociatedUser User      `gorm:"-" json:"associated_user,omitempty"`
}

// GetActiveUserSessions gets all user sessions
func (d *DkfDB) GetActiveUserSessions(userID UserID) (out []Session) {
	db := d.db.Model(&Session{}).Where("user_id = ? AND expires_at > ? AND deleted_at IS NULL", userID, time.Now())
	db.Order("created_at DESC").Find(&out)
	return
}

// CreateSession creates a session for a user
func (d *DkfDB) CreateSession(userID UserID, userAgent string, sessionDuration time.Duration) (Session, error) {
	// Delete all sessions except the last 4
	if err := d.deleteOldSessions(userID, 4); err != nil {
		logrus.Error(err)
	}
	session := Session{
		Token:         utils.GenerateToken32(),
		UserID:        userID,
		ClientIP:      "",
		UserAgent:     userAgent,
		ExpiresAt:     time.Now().Add(sessionDuration),
		AssociatedUser: User{ID: userID}, // Preload associated user
	}
	err := d.db.Create(&session).Error
	return session, err
}

// DeleteUserSessions all sessions of the user.
func (d *DkfDB) DeleteUserSessions(userID UserID) error {
	return d.db.Unscoped().Where("user_id = ?", userID).Delete(&Session{}).Error
}

// DeleteSessionByToken a session by its token
func (d *DkfDB) DeleteSessionByToken(token string) error {
	return d.db.Unscoped().Where("token = ?", token).Delete(&Session{}).Error
}

func (d *DkfDB) deleteOldSessions(userID UserID, limit int) error {
	return d.db.Exec(`DELETE FROM sessions WHERE user_id = ? AND token NOT IN (SELECT s2.token FROM sessions s2 WHERE s2.user_id = ? ORDER BY s2.created_at DESC LIMIT ?)`, userID, userID, limit).Error
}

func (d *DkfDB) DeleteUserOtherSessions(userID UserID, currentToken string) error {
	return d.db.Unscoped().Where("user_id = ? AND token != ?", userID, currentToken).Delete(&Session{}).Error
}

func (d *DkfDB) DeleteOldSessions() {
	cutoff := time.Now().AddDate(0, 0, -32)
	if err := d.db.Unscoped().Where("expires_at < ? OR (expires_at < ? AND deleted_at IS NOT NULL)", cutoff, cutoff).Delete(&Session{}).Error; err != nil {
		logrus.Error(err)
	}
}

