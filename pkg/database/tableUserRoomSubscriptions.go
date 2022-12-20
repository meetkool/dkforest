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

func (s *UserRoomSubscription) DoSave() {
	if err := DB.Save(s).Error; err != nil {
		logrus.Error(err)
	}
}

func GetUserRoomSubscriptions(userID UserID) (out []ChatRoomAug, err error) {
	err = DB.Raw(`SELECT r.*,
COALESCE((rr.read_at < m.created_at), 1) as is_unread
FROM user_room_subscriptions s
INNER JOIN chat_rooms r ON r.id = s.room_id
-- Find last message for room
LEFT JOIN chat_messages m ON m.id = (SELECT max(id) FROM chat_messages WHERE room_id = r.id AND (to_user_id IS NULL OR to_user_id = ?))
-- Get read record for the authUser & room
LEFT JOIN chat_read_records rr ON rr.user_id = ? AND rr.room_id = r.id
WHERE s.user_id = ?
ORDER BY r.id ASC`, userID, userID, userID).Scan(&out).Error
	return
}

func SubscribeToRoom(userID UserID, roomID RoomID) (err error) {
	return DB.Create(&UserRoomSubscription{UserID: userID, RoomID: roomID}).Error
}

func UnsubscribeFromRoom(userID UserID, roomID RoomID) (err error) {
	return DB.Delete(&UserRoomSubscription{}, "user_id = ? AND room_id = ?", userID, roomID).Error
}

func IsUserSubscribedToRoom(userID UserID, roomID RoomID) bool {
	var count int64
	DB.Model(UserRoomSubscription{}).Where("user_id = ? AND room_id = ?", userID, roomID).Count(&count)
	return count == 1
}
