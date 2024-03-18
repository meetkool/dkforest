package database

import (
	"time"
)

type ChatReaction struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	UserID    UserID    `gorm:"not null" json:"user_id"`
	MessageID int64     `gorm:"not null" json:"message_id"`
	Reaction  int64     `gorm:"not null" json:"reaction"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (d *DkfDB) CreateChatReaction(userID UserID, messageID, reaction int64) error {
	chatReaction := ChatReaction{
		UserID:    userID,
		MessageID: messageID,
		Reaction:  reaction,
	}
	return d.db.Create(&chatReaction).Error
}

func (d *DkfDB) DeleteReaction(userID UserID, messageID, reaction int64) error {
	result := d.db.Delete(&ChatReaction{}, "user_id = ? AND message_id = ? AND reaction = ?", userID, messageID, reaction)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNoReactionFound
	}
	return nil
}
