package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type AuditLog struct {
	ID        int64
	UserID    UserID
	Log       string
	CreatedAt time.Time
	User      User
}

func NewAudit(authUser User, log string) {
	if err := DB.Create(&AuditLog{UserID: authUser.ID, Log: log}).Error; err != nil {
		logrus.Error(err)
	}
}

func DeleteOldAuditLogs() {
	if err := DB.Delete(AuditLog{}, "created_at < date('now', '-90 Day')").Error; err != nil {
		logrus.Error(err)
	}
}
