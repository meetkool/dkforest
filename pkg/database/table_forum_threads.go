package database

import (
	"dkforest/pkg/utils"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/google/uuid"
	html2 "html"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	bf "dkforest/pkg/blackfriday/v2"
)

type ForumCategoryID int64

type ForumCategory struct {
	ID   ForumCategoryID
	Idx  int64
	Name string
	Slug string
}

type ForumThreadID int64
type ForumThreadUUID string

type ForumThread struct {
	ID         ForumThreadID
	UUID       ForumThreadUUID
	Name       string
	UserID     UserID
	CategoryID ForumCategoryID
	CreatedAt  time.Time
	User       User
	Category   ForumCategory
}

func MakeForumThread(threadName string, userID UserID, categoryID ForumCategoryID) ForumThread {
	return ForumThread{UUID: ForumThreadUUID(uuid.New().String()), Name: threadName, UserID: userID, CategoryID: categoryID}
}

func (u *ForumThread) DoSave(db *DkfDB) {
	if err := db.db.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetForumCategories() (out []ForumCategory, err error) {
	err = d.db.Find(&out).Order("idx ASC, name ASC").Error
	return
}

func (d *DkfDB) GetForumCategoryBySlug(slug string) (out ForumCategory, err error) {
	err = d.db.First(&out, "slug = ?", slug).Error
	return
}

type ForumMessageID int64

type ForumMessageUUID string

type ForumMessage struct {
	ID        ForumMessageID
	UUID      ForumMessageUUID
	Message   string
	UserID    UserID
	ThreadID  ForumThreadID
	IsSigned  bool
	CreatedAt time.Time
	User      User
}

func MakeForumMessage(message string, userID UserID, threadID ForumThreadID) ForumMessage {
	return ForumMessage{UUID: ForumMessageUUID(uuid.New().String()), Message: message, UserID: userID, ThreadID: threadID}
}

type ForumReadRecord struct {
	UserID   UserID
	ThreadID ForumThreadID
	ReadAt   time.Time
}

func (d *DkfDB) UpdateForumReadRecord(userID UserID, threadID ForumThreadID) {
	now := time.Now()
	res := d.db.Table("forum_read_records").Where("user_id = ? AND thread_id = ?", userID, threadID).Update("read_at", now)
	if res.RowsAffected == 0 {
		d.db.Create(ForumReadRecord{UserID: userID, ThreadID: threadID, ReadAt: now})
	}
}

// DoSave user in the database, ignore error
func (u *ForumReadRecord) DoSave(db *DkfDB) {
	if err := db.db.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

// DoSave user in the database, ignore error
func (u *ForumMessage) DoSave(db *DkfDB) {
	if err := db.db.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

func (m *ForumMessage) Escape(db *DkfDB) string {
	msg := m.Message
	if m.IsSigned {
		if b, _ := clearsign.Decode([]byte(msg)); b != nil {
			msg = string(b.Plaintext)
		}
	}
	res := strings.Replace(msg, "\r", "", -1)
	res = html2.EscapeString(res)
	resBytes := bf.Run([]byte(res), bf.WithRenderer(MyRendererForum(db, true, true)), bf.WithExtensions(bf.CommonExtensions|bf.HardLineBreak))
	res = string(resBytes)

	// Tags
	var tagRgx = regexp.MustCompile(`@(\w{3,20})`)
	if tagRgx.MatchString(res) {
		res = tagRgx.ReplaceAllStringFunc(res, func(s string) string {
			if user, err := db.GetUserByUsername(Username(strings.TrimPrefix(s, "@"))); err == nil {
				return `<span style="color: ` + user.ChatColor + `;">` + s + `</span>`
			}
			return s
		})
	}
	return res
}

func (d *DkfDB) GetForumMessage(messageID ForumMessageID) (out ForumMessage, err error) {
	err = d.db.First(&out, "id = ?", messageID).Error
	return
}

func (d *DkfDB) GetForumMessageByUUID(messageUUID ForumMessageUUID) (out ForumMessage, err error) {
	err = d.db.First(&out, "uuid = ?", messageUUID).Error
	return
}

func (d *DkfDB) DeleteForumMessageByID(messageID ForumMessageID) error {
	return d.db.Where("id = ?", messageID).Delete(&ForumMessage{}).Error
}

func (d *DkfDB) DeleteForumThreadByID(threadID ForumThreadID) error {
	return d.db.Where("id = ?", threadID).Delete(&ForumThread{}).Error
}

func (m *ForumMessage) CanEdit() bool {
	//return time.Since(m.CreatedAt) < time.Hour
	return true
}

func (m *ForumMessage) ValidateSignature(pkey string) bool {
	if pkey == "" {
		return false
	}
	return utils.PgpCheckClearSignMessage(pkey, m.Message)
}

func (d *DkfDB) GetForumThread(threadID ForumThreadID) (out ForumThread, err error) {
	err = d.db.First(&out, "id = ? AND is_club = 1", threadID).Error
	return
}

func (d *DkfDB) GetForumThreadByID(threadID ForumThreadID) (out ForumThread, err error) {
	err = d.db.First(&out, "id = ? AND is_club = 0", threadID).Error
	return
}

func (d *DkfDB) GetForumThreadByUUID(threadUUID ForumThreadUUID) (out ForumThread, err error) {
	err = d.db.First(&out, "uuid = ? AND is_club = 0", threadUUID).Error
	return
}

func (d *DkfDB) GetForumThreads() (out []ForumThread, err error) {
	err = d.db.Order("id DESC").Find(&out).Error
	return
}

type ForumNews struct {
	ForumThread
	ForumMessage
	User
}

type ForumThreadAug struct {
	ForumThread
	Author           string
	AuthorChatColor  string
	LastMsgAuthor    string
	LastMsgChatColor string
	LastMsgChatFont  string
	LastMsgCreatedAt time.Time
	IsUnread         bool
	RepliesCount     int64
}

func (d *DkfDB) GetClubForumThreads(userID UserID) (out []ForumThreadAug, err error) {
	err = d.db.Raw(`SELECT t.*,
u.username as author,
u.chat_color as author_chat_color,
lu.username as last_msg_author,
lu.chat_color as last_msg_chat_color,
lu.chat_font as last_msg_chat_font,
m.created_at as last_msg_created_at,
COALESCE((r.read_at < m.created_at), 1) as is_unread
FROM forum_threads t
INNER JOIN users u ON u.id = t.user_id
LEFT JOIN forum_messages m ON m.thread_id = t.id AND m.id = (SELECT max(id) FROM forum_messages WHERE thread_id = t.id)
INNER JOIN users lu ON lu.id = m.user_id
LEFT JOIN forum_read_records r ON r.user_id = ? AND r.thread_id = t.id
WHERE t.is_club = 1
ORDER BY t.id DESC`, userID).Scan(&out).Error
	return
}

func (d *DkfDB) GetPublicForumCategoryThreads(userID UserID, categoryID ForumCategoryID) (out []ForumThreadAug, err error) {
	err = d.db.Raw(`SELECT t.*,
u.username as author,
u.chat_color as author_chat_color,
lu.username as last_msg_author,
lu.chat_color as last_msg_chat_color,
lu.chat_font as last_msg_chat_font,
m.created_at as last_msg_created_at,
COALESCE((r.read_at < m.created_at), 1) as is_unread,
mmm.replies_count
FROM forum_threads t
-- Count replies
LEFT JOIN (SELECT mm.thread_id, COUNT(mm.id) as replies_count FROM forum_messages mm GROUP BY mm.thread_id) as mmm ON mmm.thread_id = t.id
-- Join author user
INNER JOIN users u ON u.id = t.user_id
-- Find last message for thread
LEFT JOIN forum_messages m ON m.thread_id = t.id AND m.id = (SELECT max(id) FROM forum_messages WHERE thread_id = t.id)
-- Join last message user
INNER JOIN users lu ON lu.id = m.user_id
-- Get read record for the authUser & thread
LEFT JOIN forum_read_records r ON r.user_id = ? AND r.thread_id = t.id
WHERE t.is_club = 0 AND t.category_id = ?
ORDER BY m.created_at DESC, t.id DESC`, userID, categoryID).Scan(&out).Error
	return
}

func (d *DkfDB) GetForumNews(categoryID ForumCategoryID) (out []ForumNews, err error) {
	err = d.db.Raw(`SELECT t.*,
       m.*,
       mu.*
FROM forum_threads t
-- Find first message for thread
INNER JOIN forum_messages m ON m.thread_id = t.id AND m.id = (SELECT min(id) FROM forum_messages WHERE thread_id = t.id)
-- Join last message user
INNER JOIN users mu ON mu.id = m.user_id
WHERE t.category_id = ?
ORDER BY m.created_at DESC, t.id DESC`, categoryID).Scan(&out).Error
	return
}

func (d *DkfDB) GetPublicForumThreadsSearch(userID UserID) (out []ForumThreadAug, err error) {
	err = d.db.Raw(`SELECT t.*,
u.username as author,
u.chat_color as author_chat_color,
lu.username as last_msg_author,
lu.chat_color as last_msg_chat_color,
lu.chat_font as last_msg_chat_font,
m.created_at as last_msg_created_at,
COALESCE((r.read_at < m.created_at), 1) as is_unread,
mmm.replies_count
FROM forum_threads t
-- Count replies
LEFT JOIN (SELECT mm.thread_id, COUNT(mm.id) as replies_count FROM forum_messages mm GROUP BY mm.thread_id) as mmm ON mmm.thread_id = t.id
-- Join author user
INNER JOIN users u ON u.id = t.user_id
-- Find last message for thread
LEFT JOIN forum_messages m ON m.thread_id = t.id AND m.id = (SELECT max(id) FROM forum_messages WHERE thread_id = t.id)
-- Join last message user
INNER JOIN users lu ON lu.id = m.user_id
-- Get read record for the authUser & thread
LEFT JOIN forum_read_records r ON r.user_id = ? AND r.thread_id = t.id
WHERE t.is_club = 0
ORDER BY m.created_at DESC, t.id DESC`, userID).Scan(&out).Error
	return
}

func (d *DkfDB) GetThreadMessages(threadID ForumThreadID) (out []ForumMessage, err error) {
	err = d.db.Preload("User").Find(&out, "thread_id = ?", threadID).Error
	return
}
