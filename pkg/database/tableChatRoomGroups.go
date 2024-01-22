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

func (g *ChatRoomGroup) DoSave(db *DkfDB) {
	if err := db.db.Save(g).Error; err != nil {
		logrus.Error(err)
	}
}

type ChatRoomUserGroup struct {
	GroupID GroupID
	RoomID  RoomID
	UserID  UserID
	User    User
}

func (d *DkfDB) GetUserRoomGroups(userID UserID, roomID RoomID) (out []ChatRoomUserGroup, err error) {
	err = d.db.Find(&out, "user_id = ? AND room_id = ?", userID, roomID).Error
	return
}

func (d *DkfDB) GetUserRoomGroupsIDs(userID UserID, roomID RoomID) (out []GroupID, err error) {
	err = d.db.Model(&ChatRoomUserGroup{}).
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Pluck("group_id", &out).
		Error
	return
}

func (d *DkfDB) GetRoomGroupByName(roomID RoomID, groupName string) (out ChatRoomGroup, err error) {
	err = d.db.First(&out, "room_id = ? AND name = ?", roomID, groupName).Error
	return
}

func (d *DkfDB) GetRoomGroupByID(roomID RoomID, groupID GroupID) (out ChatRoomGroup, err error) {
	err = d.db.First(&out, "room_id = ? AND id = ?", roomID, groupID).Error
	return
}

func (d *DkfDB) IsUserInGroupByID(userID UserID, groupID GroupID) bool {
	var count int64
	d.db.Model(ChatRoomUserGroup{}).Where("group_id = ? AND user_id = ?", groupID, userID).Count(&count)
	return count == 1
}

func (d *DkfDB) DeleteChatRoomGroup(roomID RoomID, name string) (err error) {
	err = d.db.Delete(&ChatRoomGroup{}, "room_id = ? AND name = ?", roomID, name).Error
	return
}

func (d *DkfDB) DeleteChatRoomGroups(roomID RoomID) (err error) {
	err = d.db.Delete(&ChatRoomGroup{}, "room_id = ?", roomID).Error
	return
}

func (d *DkfDB) CreateChatRoomGroup(roomID RoomID, name, color string) (out ChatRoomGroup, err error) {
	out = ChatRoomGroup{Name: name, Color: color, RoomID: roomID}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) AddUserToRoomGroup(roomID RoomID, groupID GroupID, userID UserID) (out ChatRoomUserGroup, err error) {
	out = ChatRoomUserGroup{GroupID: groupID, RoomID: roomID, UserID: userID}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) RmUserFromRoomGroup(roomID RoomID, groupID GroupID, userID UserID) (err error) {
	err = d.db.Delete(&ChatRoomUserGroup{}, "user_id = ? AND group_id = ? AND room_id = ?", userID, groupID, roomID).Error
	return
}

func (d *DkfDB) ClearRoomGroup(roomID RoomID, groupID GroupID) (err error) {
	err = d.db.Delete(&ChatRoomUserGroup{}, "group_id = ? AND room_id = ?", groupID, roomID).Error
	return
}

func (d *DkfDB) GetRoomGroups(roomID RoomID) (out []ChatRoomGroup, err error) {
	err = d.db.Find(&out, "room_id = ?", roomID).Error
	return
}

func (d *DkfDB) GetRoomGroupUsers(roomID RoomID, groupID GroupID) (out []ChatRoomUserGroup, err error) {
	err = d.db.Where("room_id = ? AND group_id = ?", roomID, groupID).Preload("User").Find(&out).Error
	return
}
