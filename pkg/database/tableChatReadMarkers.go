package database

import (
	"time"

	"github.com/jinzhu/gorm"
)

// ChatReadMarker represents the "read marker" in the chat, indicating the last time a user sent a message
// or clicked the "update read marker" button in a specific room.
type ChatReadMarker struct {
	gorm.Model
	UserID UserID `gorm:"index;not null"`
	RoomID RoomID `gorm:"index;not null"`
}

func (d *DkfDB) GetUserReadMarker(userID UserID, roomID RoomID) (out *ChatReadMarker, err error) {
	out = &ChatReadMarker{}
	err = d.db.Where("user_id = ? AND room_id = ?", userID, roomID).First(out).Error
	return
}

func (d *DkfDB) UpdateChatReadMarker(userID UserID, roomID RoomID) {
	now := time.Now()
	res := d.db.Model(&ChatReadMarker{}).Where("user_id = ? AND room_id = ?", userID, roomID).Update("read_at", now)

	if res.RowsAffected == 0 {
		d.db.Create(&ChatReadMarker{UserID: userID, RoomID: roomID, ReadAt: now})
	}

	MsgPubSub.Pub("readmarker_"+userID.String(), ChatMessageType{})
}
