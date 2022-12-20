package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type CaptchaRequest struct {
	ID         int64
	UserID     UserID
	CaptchaImg string
	Answer     string
	CreatedAt  time.Time
	User       User // User object for association queries
}

//func (r CaptchaRequest) CaptchaImgB64() string {
//	return base64.StdEncoding.EncodeToString(r.CaptchaImg)
//}

func DeleteOldCaptchaRequests() {
	if err := DB.Delete(CaptchaRequest{}, "created_at < date('now', '-90 Day')").Error; err != nil {
		logrus.Error(err)
	}
}
