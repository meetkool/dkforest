package database

import (
	"time"
)

type ChatReadMarker struct {
	UserID UserID
	RoomID RoomID
	ReadAt time.Time
}

func GetUserReadMarker(userID UserID, roomID RoomID) (out ChatReadMarker, err error) {
	err = DB.First(&out, "user_id = ? AND room_id = ?", userID, roomID).Error
	return
}

func UpdateChatReadMarker(userID UserID, roomID RoomID) {
	now := time.Now()
	res := DB.Table("chat_read_markers").Where("user_id = ? AND room_id = ?", userID, roomID).Update("read_at", now)
	if res.RowsAffected == 0 {
		DB.Create(ChatReadMarker{UserID: userID, RoomID: roomID, ReadAt: now})
	}
}
