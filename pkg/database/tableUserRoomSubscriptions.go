package database

import (
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type UserRoomSubscription struct {
	gorm.Model
	UserID UserID `gorm:"index;not null"`
	RoomID RoomID `gorm:"index;not null"`
	Room   ChatRoom `gorm:"foreignkey:RoomID;association_foreignkey:ID"`
}

func (s *UserRoomSubscription) Save(db *DkfDB) {
	if err := db.db.Save(s).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) SubscribeToRoom(userID UserID, roomID RoomID) (err error) {
	subscription := UserRoomSubscription{UserID: userID, RoomID: roomID}
	err = d.db.Create(&subscription).Error
	return
}

func (d *DkfDB) UnsubscribeFromRoom(userID UserID, roomID RoomID) (err error) {
	return d.db.Delete(&UserRoomSubscription{}, "user_id = ? AND room_id = ?", userID, roomID).Error
}

func (d *DkfDB) IsUserSubscribedToRoom(userID UserID, roomID RoomID) bool {
	var count int
	d.db.Model(UserRoomSubscription{}).Where("user_id = ? AND room_id = ?", userID, roomID).Count(&count)
	return count == 1
}
