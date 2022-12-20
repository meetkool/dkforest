package database

import (
	"github.com/sirupsen/logrus"
	"time"
)

type SecurityLog struct {
	ID        int64
	Message   string
	UserID    UserID
	Typ       int64
	CreatedAt time.Time
	User      User
}

const (
	LoginSecurityLog = iota + 1
	LogoutSecurityLog
	ChangePasswordSecurityLog
	TotpEnabledSecurityLog
	TotpDisabledSecurityLog
	Gpg2faEnabledSecurityLog
	Gpg2faDisabledSecurityLog
	UsernameChangedSecurityLog
	ChangeDuressPasswordSecurityLog
	ChangeSecretPhraseSecurityLog
	PasswordRecoverySecurityLog
)

func getMessageForType(typ int64) string {
	switch typ {
	case LoginSecurityLog:
		return "Successful login"
	case LogoutSecurityLog:
		return "Logout"
	case ChangePasswordSecurityLog:
		return "Password changed"
	case ChangeDuressPasswordSecurityLog:
		return "Duress password changed"
	case TotpEnabledSecurityLog:
		return "TOTP enabled"
	case TotpDisabledSecurityLog:
		return "TOTP disabled"
	case Gpg2faEnabledSecurityLog:
		return "GPG 2FA enabled"
	case Gpg2faDisabledSecurityLog:
		return "GPG 2FA disabled"
	case UsernameChangedSecurityLog:
		return "Username changed"
	case ChangeSecretPhraseSecurityLog:
		return "Secret phrase changed"
	case PasswordRecoverySecurityLog:
		return "Password recovery"
	}
	return ""
}

func CreateSecurityLog(userID UserID, typ int64) {
	log := SecurityLog{
		Message: getMessageForType(typ),
		UserID:  userID,
		Typ:     typ,
	}
	if err := DB.Create(&log).Error; err != nil {
		logrus.Error(err)
	}
}

func GetSecurityLogs(userID UserID) (out []SecurityLog, err error) {
	err = DB.Order("id DESC").Find(&out, "user_id  = ?", userID).Error
	return
}

func DeleteOldSecurityLogs() {
	if err := DB.Delete(SecurityLog{}, "created_at < date('now', '-7 Day')").Error; err != nil {
		logrus.Error(err)
	}
}
