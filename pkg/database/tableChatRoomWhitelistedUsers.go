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

func (r *ChatRoomWhitelistedUser) DoSave(db *DkfDB) {
	if err := db.db.Save(r).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) IsUserWhitelistedInRoom(userID UserID, roomID RoomID) bool {
	var count int64
	d.db.Table("chat_room_whitelisted_users").Where("user_id = ? and room_id = ?", userID, roomID).Count(&count)
	return count == 1
}

func (d *DkfDB) GetWhitelistedUsers(roomID RoomID) (out []ChatRoomWhitelistedUser, err error) {
	err = d.db.Preload("User").Find(&out, "room_id = ?", roomID).Error
	return
}

func (d *DkfDB) WhitelistUser(roomID RoomID, userID UserID) (out ChatRoomWhitelistedUser, err error) {
	out = ChatRoomWhitelistedUser{UserID: userID, RoomID: roomID}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) DeWhitelistUser(roomID RoomID, userID UserID) (err error) {
	err = d.db.Delete(ChatRoomWhitelistedUser{}, "user_id = ? and room_id = ?", userID, roomID).Error
	return
}
