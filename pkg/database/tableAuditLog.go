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

func (d *DkfDB) NewAudit(authUser User, log string) {
	if err := d.db.Create(&AuditLog{UserID: authUser.ID, Log: log}).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) DeleteOldAuditLogs() {
	if err := d.db.Delete(AuditLog{}, "created_at < date('now', '-90 Day')").Error; err != nil {
		logrus.Error(err)
	}
}
