package database

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

type RoomID int64

func (r RoomID) String() string {
	return fmt.Sprintf("%d", r)
}

type ChatRoom struct {
	ID           RoomID
	Name         string
	ExternalLink string
	OwnerUserID  *UserID
	Password     string // Hashed password (sha512)
	IsListed     bool
	IsEphemeral  bool
	ReadOnly     bool
	CreatedAt    time.Time
	OwnerUser    *User
	Mode         int64
}

const (
	NormalRoomMode        = 0
	UserWhitelistRoomMode = 1
)

func (d *DkfDB) CreateRoom(name string, passwordHash string, ownerID UserID, isListed bool) (out *ChatRoom, err error) {
	out = &ChatRoom{
		Name:        name,
		Password:    passwordHash,
		OwnerUserID: &ownerID,
		IsListed:    isListed,
		IsEphemeral: true,
	}
	err = d.db.Create(out).Error
	return
}

func (r *ChatRoom) IsOwned() bool {
	return r.OwnerUserID != nil
}

func (r *ChatRoom) IsRoomOwner(userID UserID) bool {
	return r.OwnerUserID != nil && *r.OwnerUserID == userID
}

func (r *ChatRoom) VerifyPasswordHash(passwordHash string) bool {
	return r.Password == passwordHash
}

func (r *ChatRoom) IsProtected() bool {
	return r.Password != ""
}

func (r *ChatRoom) DoSave(db *DkfDB) {
	if err := db.db.Save(r).Error; err != nil {
		logrus.Error(err)
	}
}

func (r *ChatRoom) IsOfficialRoom() bool {
	return r.Name == "general" ||
		r.Name == "announcements" ||
		r.Name == "suggestions" ||
		r.Name == "moderators" ||
		r.Name == "club"
}

func (d *DkfDB) GetChatRoomsByID(roomIDs []RoomID) (out []ChatRoom, err error) {
	err = d.db.Where("id IN (?)", roomIDs).Find(&out).Error
	return
}

func (d *DkfDB) GetChatRoomByID(roomID RoomID) (out *ChatRoom, err error) {
	err = d.db.Where("id = ?", roomID).First(&out).Error
	return
}

func (d *DkfDB) GetChatRoomByName(name string) (out *ChatRoom, err error) {
	err = d.db.Where("name = ?", name).First(&out).Error
	return
}

func (d *DkfDB) DeleteChatRoomByID(id RoomID) {
	if err := d.db.Delete(ChatRoom{}, "id = ?", id).Error; err != nil {
		logrus.Error(err)
	}
}

type ChatRoomAug struct {
	ChatRoom
	OwnerUser *User `gorm:"embedded"`
	IsUnread  bool
}

type ChatRoomAug1 struct {
	Name     string
	IsUnread bool
}

func (d *DkfDB) GetOfficialChatRooms1(userID UserID) (out []ChatRoomAug1, err error) {
	err = d.db.Raw(`SELECT r.name,
COALESCE((rr.read_at < m.created_at), 1) as is_unread
FROM chat_rooms r
LEFT JOIN chat_messages m ON m.id = (SELECT max(id) FROM chat_messages WHERE room_id = r.id AND (to_user_id IS NULL OR to_user_id = ?))
LEFT JOIN chat_read_records rr ON rr.user_id = ? AND rr.room_id = r.id
WHERE r.name IN ('general', 'programming', 'hacking', 'suggestions', 'club', 'moderators', 'announcements')
ORDER BY r.id ASC`, userID, userID).Scan(&out).Error
	return
}

func (d *DkfDB) GetUserRoomSubscriptions(userID UserID) (out []ChatRoomAug1, err error) {
	err = d.db.Raw(`SELECT r.name,
COALESCE((rr.read_at < m.created_at), 1) as is_unread
FROM user_room_subscriptions s
INNER JOIN chat_rooms r ON r.id = s.room_id
LEFT JOIN chat_messages m ON m.id = (SELECT max(id) FROM chat_messages WHERE room_id = r.id AND (to_user_id IS NULL OR to_user_id = ?))
LEFT JOIN chat_read_records rr ON rr.user_id = ? AND rr.room_id = r.id
WHERE s.user_id = ?
ORDER BY r.id ASC`, userID, userID, userID).Scan(&out).Error
	return
}

func (d *DkfDB) GetListedChatRooms(userID UserID) (out []ChatRoomAug, err error) {
	err = d.db.Raw(`SELECT r.*,
u.*,
COALESCE((rr.read_at < m.created_at), 1) as is_unread
FROM chat_rooms r
INNER JOIN
