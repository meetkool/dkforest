package database

import (
	"time"
)

// ChatReadMarker the "read marker" is a line displayed in the chat that indicate the last time you sent a message.
// Or if you clicked "update read marker" button, indicate the position in the messages when that was done.
// This is useful to quickly visually find the last message you actually read.
type ChatReadMarker struct {
	UserID UserID
	RoomID RoomID
	ReadAt time.Time
}

func (d *DkfDB) GetUserReadMarker(userID UserID, roomID RoomID) (out ChatReadMarker, err error) {
	err = d.db.First(&out, "user_id = ? AND room_id = ?", userID, roomID).Error
	return
}

func (d *DkfDB) UpdateChatReadMarker(userID UserID, roomID RoomID) {
	now := time.Now()
	res := d.db.Table("chat_read_markers").Where("user_id = ? AND room_id = ?", userID, roomID).Update("read_at", now)
	if res.RowsAffected == 0 {
		d.db.Create(ChatReadMarker{UserID: userID, RoomID: roomID, ReadAt: now})
	}
	MsgPubSub.Pub("readmarker_"+userID.String(), ChatMessageType{})
}
