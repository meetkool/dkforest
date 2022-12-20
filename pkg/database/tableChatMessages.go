package database

import (
	"crypto/cipher"
	"crypto/rand"
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ChatMessages []ChatMessage

func (m ChatMessages) DecryptAll(key string) error {
	aesgcm, _, err := utils.GetGCM(key)
	if err != nil {
		return err
	}
	for i := 0; i < len(m); i++ {
		m[i].Message = decrypt(m[i].Message, aesgcm)
	}
	return nil
}

func (m ChatMessages) DecryptAllRaw(key string) error {
	aesgcm, _, err := utils.GetGCM(key)
	if err != nil {
		return err
	}
	for i := 0; i < len(m); i++ {
		m[i].RawMessage = decrypt(m[i].RawMessage, aesgcm)
	}
	return nil
}

func decrypt(msg string, aesgcm cipher.AEAD) string {
	nonceSize := aesgcm.NonceSize()
	msgBytes := []byte(msg)
	if len(msgBytes) < nonceSize {
		msg = "<Failed to decrypt message>"
		return msg
	}
	nonce, ciphertext := msgBytes[:nonceSize], msgBytes[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		msg = "<Failed to decrypt message>"
	} else {
		msg = string(plaintext)
	}
	return msg
}

type ChatMessage struct {
	ID           int64
	UUID         string
	Message      string
	RawMessage   string
	RoomID       RoomID
	UserID       UserID
	ToUserID     *UserID
	GroupID      *GroupID
	UploadID     *UploadID
	CreatedAt    time.Time
	User         User
	Room         ChatRoom
	ToUser       *User
	Group        *ChatRoomGroup
	System       bool
	Moderators   bool
	IsHellbanned bool
}

// GetRawMessage get RawMessage value, decrypt it if needed
func (m *ChatMessage) GetRawMessage(key string) (string, error) {
	if !m.Room.IsProtected() {
		return m.RawMessage, nil
	}
	if key == "" {
		return "", errors.New("room key not provided")
	}
	decrypted, err := decryptMessageWithKey(key, m.RawMessage)
	if err != nil {
		return "", err
	}
	return decrypted, nil
}

func decryptMessageWithKey(key, msg string) (string, error) {
	aesgcm, nonceSize, err := utils.GetGCM(key)
	if err != nil {
		return "", err
	}

	msgBytes := []byte(msg)
	nonce, ciphertext := msgBytes[:nonceSize], msgBytes[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	var out string
	if err != nil {
		out = "<Failed to decrypt message>"
	} else {
		out = string(plaintext)
	}
	return out, nil
}

func (m *ChatMessage) MarshalJSON() ([]byte, error) {
	var tmp struct {
		UUID       string
		Message    string
		RawMessage string
		Username   string
		ToUsername *string `json:"ToUsername,omitempty"`
		CreatedAt  string
	}
	tmp.UUID = m.UUID
	tmp.Message = m.Message
	tmp.RawMessage = m.RawMessage
	tmp.Username = m.User.Username
	if m.ToUser != nil {
		tmp.ToUsername = &m.ToUser.Username
	}
	tmp.CreatedAt = m.CreatedAt.Format("2006-01-02T15:04:05")
	return json.Marshal(tmp)
}

func (m *ChatMessage) UserCanSee(user User) bool {
	// If user is not moderator, cannot see "moderators only" messages
	if m.Moderators && !user.IsModerator() {
		return false
	}
	// msg is HB and user is not hb
	if m.IsHellbanned && !user.IsHellbanned {
		// Cannot see hb if you're not a mod or CanSeeHellbanned is disabled
		cannotSeeHB := !(user.IsModerator() || user.CanSeeHellbanned)
		// user cannot see hb OR user disabled hb display
		if cannotSeeHB || !user.DisplayHellbanned {
			return false
		}
	}
	// msg user is not hb || own msg || msg user is hb & user is also hb || user can see and wish to see hb
	return !m.User.IsHellbanned || user.ID == m.UserID || (m.User.IsHellbanned && user.IsHellbanned) || (user.CanSeeHB() && user.DisplayHellbanned)
}

func (m *ChatMessage) CanBeEdited() bool {
	return time.Since(m.CreatedAt) <= config.EditMessageTimeLimit
}

// UserCanDelete returns either or not "user" can delete the messages "m"
func (m *ChatMessage) UserCanDelete(user User) bool {
	// Admin can delete everything
	if user.IsAdmin {
		return true
	}
	// Moderators cannot delete vetted user messages
	if m.UserID != user.ID && m.User.Vetted {
		return false
	}
	// Mod cannot delete admin
	if user.IsModerator() && m.User.IsAdmin {
		return false
	}
	// if room owner, you can delete messages
	if m.Room.OwnerUserID != nil && user.ID == *m.Room.OwnerUserID {
		return true
	}
	// Mod can delete own messages
	if user.IsModerator() && m.User.IsModerator() && user.ID == m.UserID {
		return true
	}
	// Mod cannot delete mod
	if user.IsModerator() && m.User.IsModerator() {
		return false
	}
	// User can delete PMs from user 0
	if m.ToUserID != nil && *m.ToUserID == user.ID && m.User.Username == config.NullUsername {
		return true
	}
	// If not a mod, you can only delete your own message
	if !user.IsModerator() && user.ID != m.UserID {
		return false
	}
	return true
}

func (m *ChatMessage) TooOldToDelete() bool {
	// PM sent by "0" can always be deleted
	if m.ToUserID != nil && m.User.Username == config.NullUsername {
		return false
	}
	return time.Since(m.CreatedAt) > config.EditMessageTimeLimit
}

func (m *ChatMessage) IsMe() bool {
	return strings.HasPrefix(m.Message, "<p>/me ")
}

func (m *ChatMessage) TrimMe() string {
	return "<p>" + strings.TrimPrefix(m.Message, "<p>/me ")
}

func (m *ChatMessage) DoSave() {
	if err := DB.Save(m).Error; err != nil {
		logrus.Error(err)
	}
}

func GetUserLastChatMessageInRoom(userID UserID, roomID RoomID) (out ChatMessage, err error) {
	err = DB.
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Order("id DESC").
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		First(&out).Error
	return
}

func GetRoomChatMessages(roomID RoomID) (out ChatMessages, err error) {
	err = DB.
		Where("room_id = ?", roomID).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		Find(&out).Error
	return
}

func GetRoomChatMessageByUUID(roomID RoomID, msgUUID string) (out ChatMessage, err error) {
	err = DB.
		Where("room_id = ? AND uuid = ?", roomID, msgUUID).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		First(&out).Error
	return
}

func GetRoomChatMessageByDate(roomID RoomID, userID UserID, dt time.Time) (out ChatMessage, err error) {
	err = DB.
		Select("*, strftime('%Y-%m-%d %H:%M:%S', created_at) as created_at1").
		Where("room_id = ? AND user_id = ? AND created_at1 = ?", roomID, userID, dt.Format("2006-01-02 15:04:05")).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		First(&out).Error
	return
}

func GetRoomChatMessagesByDate(roomID RoomID, dt time.Time) (out []ChatMessage, err error) {
	err = DB.
		Select("*, strftime('%m-%d %H:%M:%S', created_at) as created_at1").
		Where("room_id = ? AND created_at1 = ?", roomID, dt.Format("01-02 15:04:05")).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Order("id DESC").
		Find(&out).Error
	return
}

func GetChatMessages(roomID RoomID, username string, userID UserID, displayPms int64, mentionsOnly, DisplayHellbanned, displayIgnored, displayModerators bool) (out ChatMessages, err error) {
	q := DB.
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		Limit(150).
		Where(`room_id = ? AND group_id IS NULL`, roomID).
		Order("id DESC")
	if !displayIgnored {
		q = q.Where(`user_id NOT IN (SELECT ignored_user_id FROM ignored_users WHERE user_id = ?)`, userID)
	}
	if !displayModerators {
		q = q.Where(`moderators = 0`)
	}
	if mentionsOnly {
		q = q.Where(`raw_message LIKE ?`, "%@"+username+"%")
	}
	if displayPms == 0 { // Display all messages
		q = q.Where(`to_user_id is null OR to_user_id = ? OR user_id = ?`, userID, userID)
	} else if displayPms == 1 { // Display PMs only
		q = q.Where(`to_user_id = ? OR (user_id = ? AND to_user_id IS NOT NULL)`, userID, userID)
	} else { // No PMs displayed
		q = q.Where(`to_user_id is null`)
	}

	//-----------

	q1 := q.Where("is_hellbanned = 0")
	var out1 []ChatMessage
	err = q1.Find(&out1).Error

	var minID int64
	if len(out1) > 0 {
		minID = out1[len(out1)-1].ID
	}

	//-----------

	var out2 []ChatMessage
	if DisplayHellbanned {
		q2 := q.Where("is_hellbanned = 1 AND id > ?", minID)
		err = q2.Find(&out2).Error
	}

	mergedTmp := sortedMerge(out1, out2)

	//-----------

	qg := DB.
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		Limit(150).
		Where(`room_id = ? AND group_id IN (SELECT group_id FROM chat_room_user_groups g WHERE g.room_id = ? AND g.user_id = ?)`, roomID, roomID, userID).
		Order("id DESC")
	var out3 []ChatMessage
	err = qg.Find(&out3).Error

	out = sortedMerge(mergedTmp, out3)

	return
}

// merge two sorted slices. The output will also be sorted.
func sortedMerge(a, b []ChatMessage) []ChatMessage {
	out := make([]ChatMessage, len(a)+len(b))
	// "i" is a pointer for slice "a"
	// "j" is a pointer for slice "b"
	// "k" is a pointer for the output slice
	var i, j, k int
	// Loop until we reach the end of either "a" or "b"
	for i < len(a) && j < len(b) {
		if a[i].ID > b[j].ID {
			out[k] = a[i]
			i++
		} else {
			out[k] = b[j]
			j++
		}
		k++
	}
	// At this point only "a" or "b" will have remaining items.
	// If "a" still have items, finish it.
	for i < len(a) {
		out[k] = a[i]
		k++
		i++
	}
	// Otherwise, if "b" still have items, finish it.
	for j < len(b) {
		out[k] = b[j]
		k++
		j++
	}
	return out
}

func DeleteChatRoomMessages(roomID RoomID) error {
	return DB.Delete(&ChatMessage{}, "room_id = ?", roomID).Error
}

func DeleteChatMessageByUUID(messageUUID string) error {
	return DB.Where("uuid = ?", messageUUID).Delete(&ChatMessage{}).Error
}

func DeleteUserChatMessages(userID UserID) error {
	return DB.Where("user_id = ?", userID).Delete(&ChatMessage{}).Error
}

func DeleteOldChatMessages() {
	rooms, _ := GetOfficialChatRooms()
	for _, room := range rooms {
		DB.Exec(`
DELETE FROM chat_messages
-- Don't delete the last 500 "non PM" and "not hellbanned" messages
WHERE id NOT IN (
	SELECT id FROM chat_messages
	WHERE room_id = ? AND is_hellbanned = 0
	ORDER BY id DESC
	LIMIT 500
)
-- Don't delete messages that were created in the past 24h
AND id NOT IN (
	SELECT id FROM chat_messages
	WHERE room_id = ?
		AND created_at >= date('now', '-1 Day')
 		AND is_hellbanned = 0
)
-- Don't delete the last 500 hellbanned messages
AND id NOT IN (
	SELECT id FROM chat_messages
	WHERE room_id = ? AND is_hellbanned = 1
	ORDER BY id DESC
	LIMIT 500
)
AND room_id = ?
`, room.ID, room.ID, room.ID, room.ID)
	}
}

func makeMsg(raw, txt string, roomID RoomID, userID UserID) ChatMessage {
	msg := ChatMessage{
		UUID:       uuid.New().String(),
		Message:    txt,
		RawMessage: raw,
		RoomID:     roomID,
		UserID:     userID,
	}
	return msg
}

func CreateMsg(raw, txt, roomKey string, roomID RoomID, userID UserID, toUserID *UserID) (out ChatMessage, err error) {
	if roomKey != "" {
		var err error
		txt, raw, err = encryptMessages(txt, raw, roomKey)
		if err != nil {
			return out, err
		}
	}

	out = makeMsg(raw, txt, roomID, userID)
	if toUserID != nil {
		out.ToUserID = toUserID
	}
	err = DB.Create(&out).Error
	return
}

func CreateSysMsg(raw, txt, roomKey string, roomID RoomID, userID UserID) error {
	if roomKey != "" {
		var err error
		txt, raw, err = encryptMessages(txt, raw, roomKey)
		if err != nil {
			return err
		}
	}
	msg := makeMsg(raw, txt, roomID, userID)
	msg.System = true
	return DB.Create(&msg).Error
}

func CreateKickMsg(kickedUser, kickedByUser User) {
	// Display kick message
	styledUsername := fmt.Sprintf(`<span %s>%s</span>`, kickedUser.GenerateChatStyle(), kickedUser.Username)
	rawTxt := fmt.Sprintf("%s has been kicked. (%s)", kickedUser.Username, kickedByUser.Username)
	txt := fmt.Sprintf("%s has been kicked. (%s)", styledUsername, kickedByUser.Username)
	if err := CreateSysMsg(rawTxt, txt, "", config.GeneralRoomID, kickedByUser.ID); err != nil {
		logrus.Error(err)
	}
}

func CreateOrEditMessage(
	editMsg *ChatMessage,
	message, raw, roomKey string,
	roomID RoomID,
	fromUserID UserID,
	toUserID *UserID,
	upload *Upload,
	groupID *GroupID,
	hellbanMsg, modMsg, systemMsg bool) (int64, error) {

	if roomKey != "" {
		var err error
		message, raw, err = encryptMessages(message, raw, roomKey)
		if err != nil {
			return 0, err
		}
	}

	if editMsg != nil {
		editMsg.Message = message
		editMsg.RawMessage = raw
		// Delete inboxes, we'll create new ones bellow
		_ = DeleteChatInboxMessageByChatMessageID(editMsg.ID)
	} else {
		msg := makeMsg(raw, message, roomID, fromUserID)
		editMsg = &msg
		editMsg.IsHellbanned = hellbanMsg
		editMsg.System = systemMsg
		editMsg.Moderators = modMsg
		editMsg.GroupID = groupID
		editMsg.ToUserID = toUserID
		if upload != nil {
			editMsg.UploadID = &upload.ID
		}
	}
	editMsg.DoSave()
	return editMsg.ID, nil
}

func encryptMessages(html, origMessage, roomKey string) (string, string, error) {
	var err error
	// Encrypt html message (for displaying)
	html, err = encryptMessage(roomKey, html)
	if err != nil {
		return "", "", err
	}
	// Encrypt original message (for /e command)
	origMessage, err = encryptMessage(roomKey, origMessage)
	if err != nil {
		return "", "", err
	}
	return html, origMessage, err
}

func encryptMessage(roomKey, msg string) (string, error) {
	aesgcm, nonceSize, err := utils.GetGCM(roomKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return string(aesgcm.Seal(nonce, nonce, []byte(msg), nil)), nil
}
