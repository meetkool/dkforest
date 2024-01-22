package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type UserRoomSubscription struct {
	UserID    UserID
	RoomID    RoomID
	CreatedAt time.Time
	Room      ChatRoom
}

func (s *UserRoomSubscription) DoSave(db *DkfDB) {
	if err := db.db.Save(s).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) SubscribeToRoom(userID UserID, roomID RoomID) (err error) {
	return d.db.Create(&UserRoomSubscription{UserID: userID, RoomID: roomID}).Error
}

func (d *DkfDB) UnsubscribeFromRoom(userID UserID, roomID RoomID) (err error) {
	return d.db.Delete(&UserRoomSubscription{}, "user_id = ? AND room_id = ?", userID, roomID).Error
}

func (d *DkfDB) IsUserSubscribedToRoom(userID UserID, roomID RoomID) bool {
	var count int64
	d.db.Model(UserRoomSubscription{}).Where("user_id = ? AND room_id = ?", userID, roomID).Count(&count)
	return count == 1
}
