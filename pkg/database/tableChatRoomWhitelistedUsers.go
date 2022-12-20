package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type ChatRoomWhitelistedUser struct {
	UserID    UserID
	RoomID    RoomID
	CreatedAt time.Time
	User      User
}

func (r *ChatRoomWhitelistedUser) DoSave() {
	if err := DB.Save(r).Error; err != nil {
		logrus.Error(err)
	}
}

func IsUserWhitelistedInRoom(userID UserID, roomID RoomID) bool {
	var count int64
	DB.Table("chat_room_whitelisted_users").Where("user_id = ? and room_id = ?", userID, roomID).Count(&count)
	return count == 1
}

func GetWhitelistedUsers(roomID RoomID) (out []ChatRoomWhitelistedUser, err error) {
	err = DB.Preload("User").Find(&out, "room_id = ?", roomID).Error
	return
}

func WhitelistUser(roomID RoomID, userID UserID) (out ChatRoomWhitelistedUser, err error) {
	out = ChatRoomWhitelistedUser{UserID: userID, RoomID: roomID}
	err = DB.Create(&out).Error
	return
}

func DeWhitelistUser(roomID RoomID, userID UserID) (err error) {
	err = DB.Delete(ChatRoomWhitelistedUser{}, "user_id = ? and room_id = ?", userID, roomID).Error
	return
}
