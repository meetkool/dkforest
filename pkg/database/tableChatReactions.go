package database

import (
	"time"
)

type ChatReaction struct {
	ID        int64
	UserID    UserID
	MessageID int64
	Reaction  int64
	CreatedAt time.Time
}

func (d *DkfDB) CreateChatReaction(userID UserID, messageID, reaction int64) error {
	out := ChatReaction{
		UserID:    userID,
		MessageID: messageID,
		Reaction:  reaction,
	}
	return d.db.Create(&out).Error
}

func (d *DkfDB) DeleteReaction(userID UserID, messageID, reaction int64) error {
	return d.db.Delete(ChatReaction{}, "user_id = ? AND message_id = ? AND reaction = ?", userID, messageID, reaction).Error
}
