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

func CreateChatReaction(userID UserID, messageID, reaction int64) error {
	out := ChatReaction{
		UserID:    userID,
		MessageID: messageID,
		Reaction:  reaction,
	}
	return DB.Create(&out).Error
}

func DeleteReaction(userID UserID, messageID, reaction int64) error {
	return DB.Delete(ChatReaction{}, "user_id = ? AND message_id = ? AND reaction = ?", userID, messageID, reaction).Error
}
