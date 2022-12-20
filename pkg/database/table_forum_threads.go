package database

import (
	html2 "html"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Depado/bfchroma"
	"github.com/alecthomas/chroma/formatters/html"

	bf "github.com/russross/blackfriday/v2"
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

func (u *ForumThread) DoSave() {
	if err := DB.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

func GetForumCategories() (out []ForumCategory, err error) {
	err = DB.Find(&out).Order("idx ASC, name ASC").Error
	return
}

func GetForumCategoryBySlug(slug string) (out ForumCategory, err error) {
	err = DB.First(&out, "slug = ?", slug).Error
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
	CreatedAt time.Time
	User      User
}

type ForumReadRecord struct {
	UserID   UserID
	ThreadID ForumThreadID
	ReadAt   time.Time
}

// DoSave user in the database, ignore error
func (u *ForumReadRecord) DoSave() {
	if err := DB.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

// DoSave user in the database, ignore error
func (u *ForumMessage) DoSave() {
	if err := DB.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

func MyRenderer() *Renderer {
	// Defines the HTML rendering flags that are used
	var flags = bf.UseXHTML

	r := &Renderer{
		Base: bfchroma.NewRenderer(
			bfchroma.WithoutAutodetect(),
			bfchroma.ChromaOptions(
				html.WithLineNumbers(true),
				html.LineNumbersInTable(true),
			),
			bfchroma.Extend(
				bf.NewHTMLRenderer(bf.HTMLRendererParameters{
					Flags: flags,
				}),
			),
		),
	}
	return r
}

type Renderer struct {
	Base *bfchroma.Renderer
}

func (r Renderer) RenderNode(w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
	switch node.Type {
	case bf.Text:
		if node.Parent.Type != bf.Link {
			node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
		}
	case bf.Code:
		node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
	case bf.CodeBlock:
		node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
	}
	return r.Base.RenderNode(w, node, entering)
}

func (r Renderer) RenderHeader(w io.Writer, ast *bf.Node) {
	r.Base.RenderHeader(w, ast)
}

func (r Renderer) RenderFooter(w io.Writer, ast *bf.Node) {
	r.Base.RenderFooter(w, ast)
}

func (m *ForumMessage) Escape() string {
	res := strings.Replace(m.Message, "\r", "", -1)
	res = html2.EscapeString(res)
	resBytes := bf.Run([]byte(res), bf.WithRenderer(MyRenderer()), bf.WithExtensions(bf.CommonExtensions|bf.HardLineBreak))
	res = string(resBytes)

	// Tags
	var tagRgx = regexp.MustCompile(`@(\w{3,20})`)
	if tagRgx.MatchString(res) {
		res = tagRgx.ReplaceAllStringFunc(res, func(s string) string {
			if user, err := GetUserByUsername(strings.TrimPrefix(s, "@")); err == nil {
				return `<span style="color: ` + user.ChatColor + `;">` + s + `</span>`
			}
			return s
		})
	}
	return res
}

func GetForumMessage(messageID ForumMessageID) (out ForumMessage, err error) {
	err = DB.First(&out, "id = ?", messageID).Error
	return
}

func GetForumMessageByUUID(messageUUID ForumMessageUUID) (out ForumMessage, err error) {
	err = DB.First(&out, "uuid = ?", messageUUID).Error
	return
}

func DeleteForumMessageByID(messageID ForumMessageID) error {
	return DB.Where("id = ?", messageID).Delete(&ForumMessage{}).Error
}

func DeleteForumThreadByID(threadID ForumThreadID) error {
	return DB.Where("id = ?", threadID).Delete(&ForumThread{}).Error
}

func (m *ForumMessage) CanEdit() bool {
	//return time.Since(m.CreatedAt) < time.Hour
	return true
}

func GetForumThread(threadID ForumThreadID) (out ForumThread, err error) {
	err = DB.First(&out, "id = ? AND is_club = 1", threadID).Error
	return
}

func GetForumThreadByID(threadID ForumThreadID) (out ForumThread, err error) {
	err = DB.First(&out, "id = ? AND is_club = 0", threadID).Error
	return
}

func GetForumThreadByUUID(threadUUID ForumThreadUUID) (out ForumThread, err error) {
	err = DB.First(&out, "uuid = ? AND is_club = 0", threadUUID).Error
	return
}

func GetForumThreads() (out []ForumThread, err error) {
	err = DB.Order("id DESC").Find(&out).Error
	return
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

func GetClubForumThreads(userID UserID) (out []ForumThreadAug, err error) {
	err = DB.Raw(`SELECT t.*,
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

func GetPublicForumCategoryThreads(userID UserID, categoryID ForumCategoryID) (out []ForumThreadAug, err error) {
	err = DB.Raw(`SELECT t.*,
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

func GetPublicForumThreadsSearch(userID UserID) (out []ForumThreadAug, err error) {
	err = DB.Raw(`SELECT t.*,
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

func GetThreadMessages(threadID ForumThreadID) (out []ForumMessage, err error) {
	err = DB.Preload("User").Find(&out, "thread_id = ?", threadID).Error
	return
}
