package database

import (
	"time"

	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

type RoomID int64

type ChatRoom struct {
	ID          RoomID
	Name        string
	OwnerUserID *UserID
	Password    string // Hashed password (sha512)
	IsListed    bool
	IsEphemeral bool
	CreatedAt   time.Time
	OwnerUser   *User
	ReadRecord  *ChatReadRecord
	Mode        int64
}

const (
	NormalRoomMode        = 0
	UserWhitelistRoomMode = 1
)

func CreateRoom(name string, passwordHash string, ownerID UserID, isListed bool) (out ChatRoom, err error) {
	out = ChatRoom{
		Name:        name,
		Password:    passwordHash,
		OwnerUserID: &ownerID,
		IsListed:    isListed,
		IsEphemeral: true,
	}
	err = DB.Create(&out).Error
	return
}

// IsOwned returns either or not a user created the room
func (r *ChatRoom) IsOwned() bool {
	return r.OwnerUserID != nil
}

func (r *ChatRoom) IsProtected() bool {
	return r.Password != ""
}

func (r *ChatRoom) DoSave() {
	if err := DB.Save(r).Error; err != nil {
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

func (r *ChatRoom) HasAccess(c echo.Context) bool {
	authUser := c.Get("authUser").(*User)
	if authUser == nil {
		return false
	}
	if r.Name == "club" && !authUser.IsClubMember {
		return false
	}
	if r.Name == "moderators" && !authUser.IsModerator() {
		return false
	}
	if r.Mode == UserWhitelistRoomMode {
		if r.OwnerUserID != nil && *r.OwnerUserID != authUser.ID {
			if !IsUserWhitelistedInRoom(authUser.ID, r.ID) {
				return false
			}
		}
	}
	if !r.IsProtected() {
		return true
	}
	cookie, err := hutils.GetRoomCookie(c, int64(r.ID))
	if err != nil {
		return false
	}
	if cookie.Value != r.Password {
		hutils.DeleteRoomCookie(c, int64(r.ID))
		return false
	}
	return true
}

func GetChatRoomByID(roomID RoomID) (out ChatRoom, err error) {
	err = DB.Where("id = ?", roomID).First(&out).Error
	return
}

func GetChatRoomByName(roomName string) (out ChatRoom, err error) {
	err = DB.Where("name = ?", roomName).First(&out).Error
	return
}

type ChatRoomAug struct {
	ChatRoom
	OwnerUser *User `gorm:"embedded"`
	IsUnread  bool
}

func GetOfficialChatRooms1(userID UserID) (out []ChatRoomAug, err error) {
	err = DB.Raw(`SELECT r.*,
COALESCE((rr.read_at < m.created_at), 1) as is_unread
FROM chat_rooms r
-- Find last message for room
LEFT JOIN chat_messages m ON m.id = (SELECT max(id) FROM chat_messages WHERE room_id = r.id AND (to_user_id IS NULL OR to_user_id = ?))
-- Get read record for the authUser & room
LEFT JOIN chat_read_records rr ON rr.user_id = ? AND rr.room_id = r.id
WHERE r.name in ('general', 'programming', 'hacking', 'suggestions', 'club', 'moderators', 'announcements')
ORDER BY r.id ASC`, userID, userID).Scan(&out).Error
	return
}

func GetOfficialChatRooms() (out []ChatRoom, err error) {
	err = DB.Where("id IN (1, 2, 3, 4, 14)").Preload("ReadRecord").Find(&out).Error
	return
}

func GetListedChatRooms(userID UserID) (out []ChatRoomAug, err error) {
	err = DB.Raw(`SELECT r.*,
u.*,
COALESCE((rr.read_at < m.created_at), 1) as is_unread
FROM chat_rooms r
-- Join OwnerUser
INNER JOIN users u ON r.owner_user_id = u.id
-- Find last message for room
LEFT JOIN chat_messages m ON m.id = (SELECT max(id) FROM chat_messages WHERE room_id = r.id AND (to_user_id IS NULL OR to_user_id = ?))
-- Get read record for the authUser & room
LEFT JOIN chat_read_records rr ON rr.user_id = ? AND rr.room_id = r.id
WHERE r.is_listed = 1
ORDER BY r.id ASC`, userID, userID).Scan(&out).Error
	return
}

func DeleteOldPrivateChatRooms() {
	DB.Exec(`DELETE FROM chat_rooms
WHERE owner_user_id IS NOT NULL
	AND is_ephemeral = 1
	AND ((SELECT chat_messages.created_at FROM chat_messages WHERE chat_messages.room_id = chat_rooms.id ORDER BY chat_messages.ID DESC) < date('now', '-1 Day')
		OR (SELECT COUNT(*) FROM chat_messages WHERE chat_messages.room_id = chat_rooms.id) == 0)
	AND chat_rooms.created_at < date('now', '-1 Day');`)
}

type ChatReadRecord struct {
	UserID UserID
	RoomID RoomID
	ReadAt time.Time
}

func (r *ChatReadRecord) DoSave() {
	if err := DB.Save(r).Error; err != nil {
		logrus.Error(err)
	}
}
