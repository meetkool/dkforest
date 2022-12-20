package database

import (
	"github.com/sirupsen/logrus"
	"time"
)

type GroupID int64

type ChatRoomGroup struct {
	ID        GroupID
	RoomID    RoomID
	Name      string
	Color     string
	Locked    bool
	CreatedAt time.Time
}

func (g *ChatRoomGroup) DoSave() {
	if err := DB.Save(g).Error; err != nil {
		logrus.Error(err)
	}
}

type ChatRoomUserGroup struct {
	GroupID GroupID
	RoomID  RoomID
	UserID  UserID
	User    User
}

func GetUserRoomGroups(userID UserID, roomID RoomID) (out []ChatRoomUserGroup, err error) {
	err = DB.Find(&out, "user_id = ? AND room_id = ?", userID, roomID).Error
	return
}

func GetRoomGroupByName(roomID RoomID, groupName string) (out ChatRoomGroup, err error) {
	err = DB.First(&out, "room_id = ? AND name = ?", roomID, groupName).Error
	return
}

func IsUserInGroupByID(userID UserID, groupID GroupID) bool {
	var count int64
	DB.Model(ChatRoomUserGroup{}).Where("group_id = ? AND user_id = ?", groupID, userID).Count(&count)
	return count == 1
}

func DeleteChatRoomGroup(roomID RoomID, name string) (err error) {
	err = DB.Delete(&ChatRoomGroup{}, "room_id = ? AND name = ?", roomID, name).Error
	return
}

func DeleteChatRoomGroups(roomID RoomID) (err error) {
	err = DB.Delete(&ChatRoomGroup{}, "room_id = ?", roomID).Error
	return
}

func CreateChatRoomGroup(roomID RoomID, name, color string) (out ChatRoomGroup, err error) {
	out = ChatRoomGroup{Name: name, Color: color, RoomID: roomID}
	err = DB.Create(&out).Error
	return
}

func AddUserToRoomGroup(roomID RoomID, groupID GroupID, userID UserID) (out ChatRoomUserGroup, err error) {
	out = ChatRoomUserGroup{GroupID: groupID, RoomID: roomID, UserID: userID}
	err = DB.Create(&out).Error
	return
}

func RmUserFromRoomGroup(roomID RoomID, groupID GroupID, userID UserID) (err error) {
	err = DB.Delete(&ChatRoomUserGroup{}, "user_id = ? AND group_id = ? AND room_id = ?", userID, groupID, roomID).Error
	return
}

func GetRoomGroups(roomID RoomID) (out []ChatRoomGroup, err error) {
	err = DB.Find(&out, "room_id = ?", roomID).Error
	return
}

func GetRoomGroupUsers(roomID RoomID, groupID GroupID) (out []ChatRoomUserGroup, err error) {
	err = DB.Where("room_id = ? AND group_id = ?", roomID, groupID).Preload("User").Find(&out).Error
	return
}
