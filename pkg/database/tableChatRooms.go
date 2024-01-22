package database

import (
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"time"

	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

type RoomID int64

func (r RoomID) String() string {
	return utils.FormatInt64(int64(r))
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

func (d *DkfDB) CreateRoom(name string, passwordHash string, ownerID UserID, isListed bool) (out ChatRoom, err error) {
	out = ChatRoom{
		Name:        name,
		Password:    passwordHash,
		OwnerUserID: &ownerID,
		IsListed:    isListed,
		IsEphemeral: true,
	}
	err = d.db.Create(&out).Error
	return
}

func GetRoomPasswordHash(password string) string {
	return utils.Sha512(getRoomSaltedPasswordBytes(password))
}

func GetRoomDecryptionKey(password string) string {
	return utils.Sha256(getRoomSaltedPasswordBytes(password))[:32]
}

func getRoomSaltedPasswordBytes(password string) []byte {
	return getSaltedPasswordBytes(config.RoomPasswordSalt, password)
}

func getSaltedPasswordBytes(salt, password string) []byte {
	return []byte(salt + password)
}

// IsOwned returns either or not a user created the room
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

func (r *ChatRoom) HasAccess(c echo.Context) (bool, string) {
	authUser := c.Get("authUser").(*User)
	db := c.Get("database").(*DkfDB)
	if authUser == nil {
		return false, ""
	}
	if r.Name == "club" && !authUser.IsClubMember {
		return false, ""
	}
	if r.Name == "moderators" && !authUser.IsModerator() {
		return false, ""
	}
	if r.Mode == UserWhitelistRoomMode {
		if !r.IsRoomOwner(authUser.ID) {
			if !db.IsUserWhitelistedInRoom(authUser.ID, r.ID) {
				return false, ""
			}
		}
	}
	if !r.IsProtected() {
		return true, ""
	}
	cookie, err := hutils.GetRoomCookie(c, int64(r.ID))
	if err != nil {
		return false, ""
	}
	if !r.VerifyPasswordHash(cookie.Value) {
		hutils.DeleteRoomCookie(c, int64(r.ID))
		return false, ""
	}
	cookie, err = hutils.GetRoomKeyCookie(c, int64(r.ID))
	if err != nil {
		return false, ""
	}
	return true, cookie.Value
}

func (d *DkfDB) GetChatRoomsByID(roomIDs []RoomID) (out []ChatRoom, err error) {
	err = d.db.Where("id IN (?)", roomIDs).Find(&out).Error
	return
}

func (d *DkfDB) GetChatRoomByID(roomID RoomID) (out ChatRoom, err error) {
	err = d.db.Where("id = ?", roomID).First(&out).Error
	return
}

func (d *DkfDB) GetChatRoomByName(roomName string) (out ChatRoom, err error) {
	err = d.db.Where("name = ?", roomName).First(&out).Error
	return
}

func (d *DkfDB) DeleteChatRoomByID(id RoomID) {
	if err := d.db.Delete(ChatRoom{}, "id = ?", id).Error; err != nil {
		logrus.Error(err)
	}
}

type ChatRoomAug struct {
	ChatRoom
	OwnerUser *User `gorm:"embedded"` // https://gorm.io/docs/models.html#Embedded-Struct
	IsUnread  bool
}

type ChatRoomAug1 struct {
	Name     string
	IsUnread bool
}

// GetOfficialChatRooms1 returns official chat rooms with additional information such as "IsUnread"
func (d *DkfDB) GetOfficialChatRooms1(userID UserID) (out []ChatRoomAug1, err error) {
	err = d.db.Raw(`SELECT r.name,
COALESCE((rr.read_at < m.created_at), 1) as is_unread
FROM chat_rooms r
-- Find last message for room
LEFT JOIN chat_messages m ON m.id = (SELECT max(id) FROM chat_messages WHERE room_id = r.id AND (to_user_id IS NULL OR to_user_id = ?))
-- Get read record for the authUser & room
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
-- Find last message for room
LEFT JOIN chat_messages m ON m.id = (SELECT max(id) FROM chat_messages WHERE room_id = r.id AND (to_user_id IS NULL OR to_user_id = ?))
-- Get read record for the authUser & room
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

func (d *DkfDB) GetOfficialChatRooms() (out []ChatRoom, err error) {
	rooms := []string{"general", "club", "moderators"}
	err = d.db.Where("name IN (?)", rooms).Find(&out).Error
	return
}

func (d *DkfDB) DeleteOldPrivateChatRooms() {
	d.db.Exec(`DELETE FROM chat_rooms
WHERE owner_user_id IS NOT NULL
	AND is_ephemeral = 1
	AND ((SELECT chat_messages.created_at FROM chat_messages WHERE chat_messages.room_id = chat_rooms.id ORDER BY chat_messages.ID DESC) < date('now', '-1 Day')
		OR (SELECT COUNT(*) FROM chat_messages WHERE chat_messages.room_id = chat_rooms.id) == 0)
	AND chat_rooms.created_at < date('now', '-1 Day');`)
}

// ChatReadRecord use to keep track of last message read (loaded) in a room, for rooms new message indicator.
// ie: a room you're not currently in, changes color if new messages are posted in it.
type ChatReadRecord struct {
	UserID UserID
	RoomID RoomID
	ReadAt time.Time
}

func (r *ChatReadRecord) DoSave(db *DkfDB) {
	if err := db.db.Save(r).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) UpdateChatReadRecord(userID UserID, roomID RoomID) {
	now := time.Now()
	res := d.db.Exec(`UPDATE chat_read_records SET read_at = ? WHERE user_id = ? AND room_id = ?`, now, userID, roomID)
	if res.RowsAffected == 0 {
		d.db.Create(ChatReadRecord{UserID: userID, RoomID: roomID, ReadAt: now})
	}
}
