package database

import (
	"crypto/cipher"
	"crypto/rand"
	"dkforest/pkg/config"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"io"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ChatMessages []ChatMessage

func (m *ChatMessage) Decrypt(key string) error {
	aesgcm, _, err := utils.GetGCM(key)
	if err != nil {
		return err
	}
	m.Message = decrypt(m.Message, aesgcm)
	return nil
}

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
	Rev          int64 // Revision, is incr every time a message is edited
	SkipNotify   bool  `gorm:"-"`
}

func (m *ChatMessage) GetProfile(authUserID UserID) Username {
	if m.ToUserID != nil && *m.ToUserID != authUserID {
		return m.ToUser.Username
	}
	return m.User.Username
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
	var out struct {
		UUID         string
		Message      string
		RawMessage   string
		Username     Username
		ToUsername   *Username `json:"ToUsername,omitempty"`
		CreatedAt    string
		IsHellbanned bool
	}
	out.UUID = m.UUID
	out.Message = m.Message
	out.RawMessage = m.RawMessage
	out.Username = m.User.Username
	out.IsHellbanned = m.IsHellbanned
	if m.ToUser != nil {
		out.ToUsername = &m.ToUser.Username
	}
	out.CreatedAt = m.CreatedAt.Format("2006-01-02T15:04:05")
	return json.Marshal(out)
}

func (m *ChatMessage) UserCanSee(user IUserRenderMessage) bool {
	// If user is not moderator, cannot see "moderators only" messages
	if m.Moderators && !user.IsModerator() {
		return false
	}
	// msg is HB and user is not hb
	if m.IsHellbanned && !user.GetIsHellbanned() {
		// Cannot see hb if you're not a mod or CanSeeHellbanned is disabled
		cannotSeeHB := !(user.IsModerator() || user.GetCanSeeHellbanned())
		// user cannot see hb OR user disabled hb display
		if cannotSeeHB || !user.GetDisplayHellbanned() {
			return false
		}
	}
	// msg user is not hb || own msg || msg user is hb & user is also hb || user can see and wish to see hb
	return !m.User.IsHellbanned || m.OwnMessage(user.GetID()) || (m.User.IsHellbanned && user.GetIsHellbanned()) || (user.CanSeeHB() && user.GetDisplayHellbanned())
}

func (m *ChatMessage) DeleteSecondsRemaining() int64 {
	return int64(math.Max((config.EditMessageTimeLimit - time.Since(m.CreatedAt)).Seconds(), 0))
}

func (m *ChatMessage) CanBeEdited() bool {
	return time.Since(m.CreatedAt) <= config.EditMessageTimeLimit
}

func (m *ChatMessage) UserCanDelete(user IUserRenderMessage) bool {
	return m.UserCanDeleteErr(user) == nil
}

// UserCanDeleteErr returns either or not "user" can delete the messages "m"
func (m *ChatMessage) UserCanDeleteErr(user IUserRenderMessage) error {
	// Admin can delete everything
	if user.GetIsAdmin() {
		return nil
	}
	// room owner can delete any messages in their room
	if m.IsRoomOwner(user.GetID()) {
		return nil
	}
	// User can delete PMs from user 0
	if m.IsPmRecipient(user.GetID()) && m.User.Username == config.NullUsername {
		return nil
	}
	// Own messages can be deleted if not too old
	if m.UserID == user.GetID() {
		if m.TooOldToDelete() {
			return errors.New("message is too old to be deleted")
		}
		return nil
	}
	// Moderators cannot delete vetted user messages
	if user.IsModerator() && m.User.Vetted {
		return errors.New("cannot delete message of vetted user")
	}
	// Mod cannot delete admin
	if user.IsModerator() && m.User.IsAdmin {
		return errors.New("cannot delete message of admin user")
	}
	// Mod cannot delete mod
	if user.IsModerator() && m.User.IsModerator() {
		return errors.New("cannot delete message of moderator user")
	}
	// Mod can delete messages they don't own
	if user.IsModerator() {
		return nil
	}
	// Cannot delete message you don't own
	return errors.New("cannot delete this message")
}

func (m *ChatMessage) TooOldToDelete() bool {
	// PM sent by "0" can always be deleted
	if m.ToUserID != nil && m.User.Username == config.NullUsername {
		return false
	}
	return time.Since(m.CreatedAt) > config.EditMessageTimeLimit
}

func (m *ChatMessage) OwnMessage(userID UserID) bool {
	return m.UserID == userID
}

func (m *ChatMessage) IsPm() bool {
	return m.ToUserID != nil
}

func (m *ChatMessage) IsPmRecipient(userID UserID) bool {
	return m.ToUserID != nil && *m.ToUserID == userID
}

func (m *ChatMessage) IsRoomOwner(userID UserID) bool {
	return m.Room.IsRoomOwner(userID)
}

func (m *ChatMessage) IsMe() bool {
	return strings.HasPrefix(m.Message, "<p>/me ")
}

func (m *ChatMessage) TrimMe() string {
	return "<p>" + strings.TrimPrefix(m.Message, "<p>/me ")
}

var externalLinkRgx = regexp.MustCompile(`<a href="([^"]+)" rel="noopener noreferrer" target="_blank">`)

func (m *ChatMessage) MsgToDisplay(authUser IUserRenderMessage) string {
	var msg string
	if m.IsMe() {
		msg = m.TrimMe()
	} else {
		msg = m.Message
	}
	if authUser.GetConfirmExternalLinks() {
		msg = externalLinkRgx.ReplaceAllStringFunc(msg, func(s string) string {
			original := externalLinkRgx.FindStringSubmatch(s)[1]
			if strings.HasPrefix(original, "/") || strings.HasPrefix(original, "?") {
				return s
			}
			return `<a href="/external-link/` + url.PathEscape(original) + `" rel="noopener noreferrer" target="_blank">`
		})
	}
	return msg
}

func (m *ChatMessage) Delete(db *DkfDB) error {
	// If we delete message manually, also delete linked inbox if any
	_ = db.DeleteChatInboxMessageByChatMessageID(m.ID)
	err := db.DeleteChatMessageByUUID(m.UUID)
	MsgPubSub.Pub("room_"+m.RoomID.String(), ChatMessageType{Typ: DeleteMsg, Msg: *m})
	return err
}

func (m *ChatMessage) DoSave(db *DkfDB) {
	if err := db.db.Save(m).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetUserLastChatMessageInRoom(userID UserID, roomID RoomID) (out ChatMessage, err error) {
	err = d.db.
		Where("user_id = ? AND room_id = ?", userID, roomID).
		Order("id DESC").
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		First(&out).Error
	return
}

// RoomChatMessagesGeIncrRev increments revision counter of all messages newer than chatMessageID
func (d *DkfDB) RoomChatMessagesGeIncrRev(roomID RoomID, chatMessageID int64) (err error) {
	err = d.db.
		Exec(`UPDATE chat_messages SET rev = rev + 1  WHERE room_id = ? AND id > ?`, roomID, chatMessageID).
		Error
	return
}

func (d *DkfDB) GetRoomChatMessages(roomID RoomID) (out ChatMessages, err error) {
	err = d.db.
		Where("room_id = ?", roomID).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		Find(&out).Error
	return
}

func (d *DkfDB) GetChatMessageByUUID(msgUUID string) (out ChatMessage, err error) {
	err = d.db.
		Where("uuid = ?", msgUUID).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		First(&out).Error
	return
}

func (d *DkfDB) GetRoomChatMessageByUUID(roomID RoomID, msgUUID string) (out ChatMessage, err error) {
	err = d.db.
		Where("room_id = ? AND uuid = ?", roomID, msgUUID).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		First(&out).Error
	return
}

func (d *DkfDB) GetRoomChatMessageByDate(roomID RoomID, userID UserID, dt time.Time) (out ChatMessage, err error) {
	err = d.db.
		Select("*, strftime('%Y-%m-%d %H:%M:%S', created_at) as created_at1").
		Where("room_id = ? AND user_id = ? AND created_at1 = ?", roomID, userID, dt.Format("2006-01-02 15:04:05")).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		First(&out).Error
	return
}

func (d *DkfDB) GetRoomChatMessagesByDate(roomID RoomID, dt time.Time) (out []ChatMessage, err error) {
	err = d.db.
		Select("*, strftime('%m-%d %H:%M:%S', created_at) as created_at1").
		Where("room_id = ? AND created_at1 = ?", roomID, dt.Format("01-02 15:04:05")).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Order("id DESC").
		Find(&out).Error
	return
}

type PmDisplayMode int64

const (
	PmNoFilter PmDisplayMode = iota
	PmOnly
	PmNone
)

func (d *DkfDB) GetChatMessages(roomID RoomID, roomKey string, username Username, userID UserID,
	pmUserID *UserID, displayPms PmDisplayMode, mentionsOnly, displayHellbanned, displayIgnored, displayModerators,
	displayIgnoredMessages bool, msgsLimit, minID1 int64) (out ChatMessages, err error) {

	cmp := func(t, t2 ChatMessage) bool { return t.ID > t2.ID }

	q := d.db.
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Preload("Group").
		Limit(int(msgsLimit)).
		Where(`room_id = ?`, roomID)
	if minID1 > 0 {
		q = q.Where("id >= ?", minID1)
		q = q.Order("id ASC")
	} else {
		q = q.Order("id DESC")
	}
	q = q.Where(`group_id IS NULL OR group_id IN (SELECT group_id FROM chat_room_user_groups g WHERE g.room_id = ? AND g.user_id = ?)`, roomID, userID)
	if !displayIgnoredMessages {
		q = q.Where(`id NOT IN (SELECT message_id FROM ignored_messages WHERE user_id = ?)`, userID)
	}
	if !displayIgnored {
		q = q.Where(`user_id NOT IN (SELECT ignored_user_id FROM ignored_users WHERE user_id = ?)`, userID)
	}
	if !displayModerators {
		q = q.Where(`moderators = 0`)
	}
	if mentionsOnly {
		q = q.Where(`raw_message LIKE ?`, "%@"+username+"%")
	}
	if pmUserID != nil {
		q = q.Where(`(to_user_id = ? AND user_id = ?) OR (user_id = ? AND to_user_id = ?)`, userID, pmUserID, userID, pmUserID)
	}
	switch displayPms {
	case PmNoFilter: // Display all messages
		q = q.Where(`to_user_id is null OR to_user_id = ? OR user_id = ?`, userID, userID)
	case PmOnly: // Display PMs only
		q = q.Where(`to_user_id = ? OR (user_id = ? AND to_user_id IS NOT NULL)`, userID, userID)
	case PmNone: // No PMs displayed
		q = q.Where(`to_user_id is null`)
	}

	//-----------

	q1 := q.Session(&gorm.Session{})
	q1 = q1.Where("is_hellbanned = 0")
	var out1 []ChatMessage
	if err = q1.Find(&out1).Error; err != nil {
		return out, err
	}

	var minID int64
	if len(out1) > 0 {
		minID = out1[len(out1)-1].ID
	}

	//-----------

	// Get all the HB messages that are more recent than the oldest non-HB message.
	// We do this in case someone in HB keep spamming the room.
	// So we still have 150 non-HB messages for normal folks and we get all the spam for the people in HB.

	var out2 []ChatMessage
	if displayHellbanned {
		q2 := q.Session(&gorm.Session{})
		q2 = q2.Where("is_hellbanned = 1 AND id > ?", minID)
		if minID1 > 0 {
			q2 = q2.Where("is_hellbanned = 1")
		}
		if err = q2.Find(&out2).Error; err != nil {
			return out, err
		}
	}

	out = sortedMerge(out1, out2, cmp)

	if roomKey != "" {
		if err := out.DecryptAll(roomKey); err != nil {
			return out, err
		}
	}

	return out, nil
}

// merge two sorted slices. The output will also be sorted.
func sortedMerge[T any](a, b []T, less func(T, T) bool) []T {
	out := make([]T, len(a)+len(b))
	// "i" is a pointer for slice "a"
	// "j" is a pointer for slice "b"
	// "k" is a pointer for the output slice
	var i, j, k int
	// Loop until we reach the end of either "a" or "b"
	for i < len(a) && j < len(b) {
		if less(a[i], b[j]) {
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

func (d *DkfDB) DeleteChatRoomMessages(roomID RoomID) error {
	return d.db.Delete(&ChatMessage{}, "room_id = ?", roomID).Error
}

func (d *DkfDB) DeleteChatMessageByUUID(messageUUID string) error {
	return d.db.Where("uuid = ?", messageUUID).Delete(&ChatMessage{}).Error
}

func (d *DkfDB) DeleteUserChatMessages(userID UserID) error {
	return d.db.Where("user_id = ?", userID).Delete(&ChatMessage{}).Error
}

func (d *DkfDB) DeleteUserHbChatMessages(userID UserID) error {
	return d.db.Where("user_id = ? AND is_hellbanned = 1", userID).Delete(&ChatMessage{}).Error
}

func (d *DkfDB) DeleteUserChatMessagesOpt(userID UserID, hbOnly bool, secs int64) error {
	q := d.db.Where("user_id = ?", userID)
	if secs > 0 {
		secsStr := "-" + utils.FormatInt64(secs) + " Second"
		q = q.Where("created_at > datetime('now', ?, 'localtime')", secsStr)
	}
	if hbOnly {
		q = q.Where("is_hellbanned = 1")
	}
	err := q.Delete(&ChatMessage{}).Error
	return err
}

func (d *DkfDB) DeleteOldChatMessages() {
	rooms, _ := d.GetOfficialChatRooms()
	for _, room := range rooms {
		d.db.Exec(`
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

func (d *DkfDB) CreateMsg(raw, txt, roomKey string, roomID RoomID, userID UserID, toUserID *UserID) (out ChatMessage, err error) {
	return d.createMsg(raw, txt, roomKey, roomID, userID, toUserID, false, false)
}

func (d *DkfDB) CreateSysMsg(raw, txt, roomKey string, roomID RoomID, userID UserID) error {
	_, err := d.createMsg(raw, txt, roomKey, roomID, userID, nil, true, false)
	return err
}

func (d *DkfDB) CreateSysMsgPM(raw, txt, roomKey string, roomID RoomID, userID UserID, toUserID *UserID, skipNotify bool) error {
	_, err := d.createMsg(raw, txt, roomKey, roomID, userID, toUserID, true, skipNotify)
	return err
}

func (d *DkfDB) CreateKickMsg(kickedUser, kickedByUser User) {
	// Display kick message
	styledUsername := fmt.Sprintf(`<span %s>%s</span>`, kickedUser.GenerateChatStyle(), kickedUser.Username)
	rawTxt := fmt.Sprintf("%s has been kicked. (%s)", kickedUser.Username, kickedByUser.Username)
	txt := fmt.Sprintf("%s has been kicked. (%s)", styledUsername, kickedByUser.Username)
	if err := d.CreateSysMsg(rawTxt, txt, "", config.GeneralRoomID, kickedByUser.ID); err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) CreateUnkickMsg(kickedUser, kickedByUser User) {
	// Display unkick message
	styledUsername := fmt.Sprintf(`<span %s>%s</span>`, kickedUser.GenerateChatStyle(), kickedUser.Username)
	rawTxt := fmt.Sprintf("%s has been unkicked. (%s)", kickedUser.Username, kickedByUser.Username)
	txt := fmt.Sprintf("%s has been unkicked. (%s)", styledUsername, kickedByUser.Username)
	if err := d.CreateSysMsg(rawTxt, txt, "", config.GeneralRoomID, kickedByUser.ID); err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) createMsg(raw, txt, roomKey string, roomID RoomID, userID UserID, toUserID *UserID, system, skipNotify bool) (out ChatMessage, err error) {
	if roomKey != "" {
		var err error
		txt, raw, err = encryptMessages(txt, raw, roomKey)
		if err != nil {
			return out, err
		}
	}

	out = makeMsg(raw, txt, roomID, userID)
	out.SkipNotify = skipNotify
	if toUserID != nil {
		out.ToUserID = toUserID
	}
	out.System = system
	err = d.db.Create(&out).Error
	MsgPubSub.Pub("room_"+roomID.String(), ChatMessageType{Typ: CreateMsg, Msg: out})
	return
}

func (d *DkfDB) CreateOrEditMessage(
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

	typ := CreateMsg
	if editMsg != nil {
		typ = EditMsg
		_ = d.RoomChatMessagesGeIncrRev(roomID, editMsg.ID)
		editMsg.Message = message
		editMsg.RawMessage = raw
		editMsg.Rev++
		// Delete inboxes, we'll create new ones bellow
		_ = d.DeleteChatInboxMessageByChatMessageID(editMsg.ID)
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
	editMsg.DoSave(d)

	i := 0
	rgx := regexp.MustCompile(`</pre>`)
	editMsg.Message = rgx.ReplaceAllStringFunc(editMsg.Message, func(s string) string {
		i++
		return fmt.Sprintf(`</pre><a href="/chat-code/%s/%d" title="Open in fullscreen" rel="noopener noreferrer" target="_blank" class=fullscreen>&#9974;</a>`,
			editMsg.UUID, i-1)
	})
	if i > 0 {
		editMsg.DoSave(d)
	}

	MsgPubSub.Pub("room_"+roomID.String(), ChatMessageType{Typ: typ, Msg: *editMsg})
	return editMsg.ID, nil
}

type PubSubMessageType int

const (
	CreateMsg PubSubMessageType = iota
	EditMsg
	ForceRefresh
	DeleteMsg
	Wizz
	Redirect
	Close
	CloseMenu

	RefreshTopic string = "refresh"
)

type ChatMessageType struct {
	Typ            PubSubMessageType
	Msg            ChatMessage
	IsMod          bool
	ToUserUsername *Username
	NewURL         string
}

var MsgPubSub = pubsub.NewPubSub[ChatMessageType]()

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
