package database

import (
	"github.com/sirupsen/logrus"
	"time"

	"dkforest/pkg/utils"
)

// Session table
type Session struct {
	Token     string     // 32 bytes random token
	ExpiresAt time.Time  // Time at which the session expires
	DeletedAt *time.Time // Time at which a session was soft deleted
	CreatedAt time.Time  // Time at which a session was created
	UserID    UserID     // User that owns the session
	ClientIP  string     // IP address used to create the session
	UserAgent string     // Browser UserAgent that was used to create the session
	User      User       // User object for association queries
}

// GetActiveUserSessions gets all user sessions
func (d *DkfDB) GetActiveUserSessions(userID UserID) (out []Session) {
	d.db.Order("created_at DESC").Find(&out, "user_id = ? AND expires_at > DATETIME('now', 'localtime') AND deleted_at IS NULL", userID)
	return
}

// CreateSession creates a session for a user
func (d *DkfDB) CreateSession(userID UserID, userAgent string, sessionDuration time.Duration) (Session, error) {
	// Delete all sessions except the last 4
	if err := d.db.Exec(`DELETE FROM sessions WHERE user_id = ? AND token NOT IN (SELECT s2.token FROM sessions s2 WHERE s2.user_id = ? ORDER BY s2.created_at DESC LIMIT 4)`, userID, userID).Error; err != nil {
		logrus.Error(err)
	}
	session := Session{
		Token:     utils.GenerateToken32(),
		UserID:    userID,
		ClientIP:  "",
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	err := d.db.Create(&session).Error
	return session, err
}

// DoCreateSession same as CreateSession but log the error instead of returning it
func (d *DkfDB) DoCreateSession(userID UserID, userAgent string, sessionDuration time.Duration) Session {
	session, err := d.CreateSession(userID, userAgent, sessionDuration)
	if err != nil {
		logrus.Error("Failed to create session : ", err)
	}
	return session
}

// DeleteUserSessions all sessions of the user.
func (d *DkfDB) DeleteUserSessions(userID UserID) error {
	return d.db.Unscoped().Where("user_id = ?", userID).Delete(&Session{}).Error
}

// DeleteSessionByToken a session by its token
func (d *DkfDB) DeleteSessionByToken(token string) error {
	return d.db.Unscoped().Where("token = ?", token).Delete(&Session{}).Error
}

func (d *DkfDB) DeleteUserSessionByToken(userID UserID, token string) error {
	return d.db.Unscoped().Where("user_id = ? AND token = ?", userID, token).Delete(&Session{}).Error
}

func (d *DkfDB) DeleteUserOtherSessions(userID UserID, currentToken string) error {
	return d.db.Unscoped().Where("user_id = ? AND token != ?", userID, currentToken).Delete(&Session{}).Error
}

func (d *DkfDB) DeleteOldSessions() {
	if err := d.db.Unscoped().Delete(Session{}, "expires_at < date('now', '-32 Day') OR (expires_at < date('now', '-32 Day') AND deleted_at IS NOT NULL)").Error; err != nil {
		logrus.Error(err)
	}
}
