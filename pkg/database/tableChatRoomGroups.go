package database

import (
	"github.com/sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
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
	err = d.db.Model(&ChatRoomUserGroup{}).Where("user_id = ? AND room_id = ?", userID, roomID).Find(&out).Error
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
	if err != nil && err != gorm.ErrRecordNotFound {
		return out, err
	}
	return
}

func (d *DkfDB) GetRoomGroupByID(roomID RoomID, groupID GroupID) (out ChatRoomGroup, err error) {
	err = d.db.First(&out, "room_id = ? AND id = ?", roomID, groupID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
	
