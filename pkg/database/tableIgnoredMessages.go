package database

import (
	"github.com/sirupsen/logrus"
)

type IgnoredMessage struct {
	UserID    UserID
	MessageID int64
}

func (d *DkfDB) IgnoreMessage(userID UserID, messageID int64) {
	ignore := IgnoredMessage{UserID: userID, MessageID: messageID}
	if err := d.db.Create(&ignore).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) UnIgnoreMessage(userID UserID, messageID int64) {
	if err := d.db.Delete(&IgnoredMessage{}, "user_id = ? AND message_id = ?", userID, messageID).Error; err != nil {
		logrus.Error(err)
	}
}
