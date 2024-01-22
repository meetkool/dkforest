package handlers

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	html2 "html"
	"net/http"
	"strings"
)

func ForumHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data forumData
	data.ForumCategories, _ = db.GetForumCategories()
	data.ForumThreads, _ = db.GetPublicForumCategoryThreads(authUser.ID, 1)
	return c.Render(http.StatusOK, "forum", data)
}

func ForumCategoryHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	categorySlug := c.Param("categorySlug")
	var data forumCategoryData
	category, err := db.GetForumCategoryBySlug(categorySlug)
	if err != nil {
		return c.Redirect(http.StatusFound, "/forum")
	}
	data.ForumThreads, _ = db.GetPublicForumCategoryThreads(authUser.ID, category.ID)
	return c.Render(http.StatusOK, "forum", data)
}

func ForumSearchHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data forumSearchData
	data.Search = c.QueryParam("search")
	data.AuthorFilter = c.QueryParam("author")

	if data.AuthorFilter != "" {
		if err := db.DB().Raw(`select
t.*,
u.username as author,
u.chat_color as author_chat_color,
lu.username as last_msg_author,
lu.chat_color as last_msg_chat_color,
lu.chat_font as last_msg_chat_font,
m.created_at as last_msg_created_at,
mmm.replies_count
from fts5_forum_threads ft
inner join forum_threads t on t.id = ft.id 
-- Count replies
LEFT JOIN (SELECT mm.thread_id, COUNT(mm.id) as replies_count FROM forum_messages mm GROUP BY mm.thread_id) as mmm ON mmm.thread_id = t.id
-- Join author user
INNER JOIN users u ON u.id = t.user_id
-- Find last message for thread
LEFT JOIN forum_messages m ON m.thread_id = t.id AND m.id = (SELECT max(id) FROM forum_messages WHERE thread_id = t.id)
-- Join last message user
INNER JOIN users lu ON lu.id = m.user_id
where u.username = ? and t.is_club = 0 order by id desc limit 100`, data.AuthorFilter).Scan(&data.ForumThreads).Error; err != nil {
			logrus.Error(err)
		}
		return c.Render(http.StatusOK, "forum-search", data)
	}

	if err := db.DB().Raw(`select m.uuid, snippet(fts5_forum_messages,-1, '[', ']', '...', 10) as snippet, t.uuid as thread_uuid, t.name as thread_name,
u.username as author,
u.chat_color as author_chat_color,
u.chat_font as author_chat_font,
mm.created_at as created_at
from fts5_forum_messages m
inner join forum_threads t on t.id = m.thread_id
-- Find message
LEFT JOIN forum_messages mm ON mm.uuid = m.uuid
-- Join author user
INNER JOIN users u ON u.id = mm.user_id
where fts5_forum_messages match ? and t.is_club = 0 order by rank limit 100`, data.Search).Scan(&data.ForumMessages).Error; err != nil {
		logrus.Error(err)
	}

	if err := db.DB().Raw(`select
t.*,
u.username as author,
u.chat_color as author_chat_color,
lu.username as last_msg_author,
lu.chat_color as last_msg_chat_color,
lu.chat_font as last_msg_chat_font,
m.created_at as last_msg_created_at,
mmm.replies_count
from fts5_forum_threads ft
inner join forum_threads t on t.id = ft.id 
-- Count replies
LEFT JOIN (SELECT mm.thread_id, COUNT(mm.id) as replies_count FROM forum_messages mm GROUP BY mm.thread_id) as mmm ON mmm.thread_id = t.id
-- Join author user
INNER JOIN users u ON u.id = t.user_id
-- Find last message for thread
LEFT JOIN forum_messages m ON m.thread_id = t.id AND m.id = (SELECT max(id) FROM forum_messages WHERE thread_id = t.id)
-- Join last message user
INNER JOIN users lu ON lu.id = m.user_id
where fts5_forum_threads match ? and t.is_club = 0 order by rank limit 100`, data.Search).Scan(&data.ForumThreads).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "forum-search", data)
}

func ThreadEditHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.ForumDisabledErr.Error(), Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/t/" + string(threadUUID), Type: "alert-danger"})
	}
	if !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/t/"+string(threadUUID))
	}
	thread, err := db.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/t/"+string(threadUUID))
	}
	var data editForumThreadData
	data.Thread = thread
	data.Categories, _ = db.GetForumCategories()

	if c.Request().Method == http.MethodPost {
		thread.CategoryID = database.ForumCategoryID(utils.DoParseInt64(c.Request().PostFormValue("category_id")))
		thread.DoSave(db)
		return c.Redirect(http.StatusFound, "/t/"+string(threadUUID))
	}

	return c.Render(http.StatusOK, "thread-edit", data)
}

func ThreadDeleteHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.ForumDisabledErr.Error(), Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/t/" + string(threadUUID), Type: "alert-danger"})
	}
	thread, err := db.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/forum")
	}

	if !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/forum")
	}

	var data deleteForumThreadData
	data.Thread = thread

	if c.Request().Method == http.MethodPost {
		if err := db.DeleteForumThreadByID(thread.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/forum")
	}

	return c.Render(http.StatusOK, "thread-delete", data)
}

func ThreadReplyHandler(c echo.Context) error {
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))

	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.ForumDisabledErr.Error(), Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/t/" + string(threadUUID), Type: "alert-danger"})
	}

	thread, err := db.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data threadReplyData
	data.Thread = thread

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Message must have at least 3 characters"
			return c.Render(http.StatusOK, "thread-reply", data)
		}
		if isForumSpam(data.Message) {
			db.NewAudit(*authUser, fmt.Sprintf("spam forum thread reply %s (#%d)", authUser.Username, authUser.ID))
			authUser.SetCanUseForum(db, false)
			return c.Redirect(http.StatusFound, "/")
		}
		message := database.MakeForumMessage(data.Message, authUser.ID, thread.ID)
		message.IsSigned = message.ValidateSignature(authUser.GPGPublicKey)
		if err := db.DB().Create(&message).Error; err != nil {
			logrus.Error(err)
		}
		// Send notifications
		subs, _ := db.GetUsersSubscribedToForumThread(thread.ID)
		for _, sub := range subs {
			if sub.UserID != authUser.ID {
				threadName := html2.EscapeString(thread.Name)
				msg := fmt.Sprintf(`New reply in thread &quot;<a href="/t/%s#%s">%s</a>&quot;`, thread.UUID, message.UUID, threadName)
				db.CreateNotification(msg, sub.UserID)
			}
		}
		return c.Redirect(http.StatusFound, "/t/"+string(thread.UUID))
	}

	return c.Render(http.StatusOK, "thread-reply", data)
}

func ThreadRawMessageHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.ForumDisabledErr.Error(), Redirect: "/", Type: "alert-danger"})
	}
	db := c.Get("database").(*database.DkfDB)
	messageUUID := database.ForumMessageUUID(c.Param("messageUUID"))
	msg, err := db.GetForumMessageByUUID(messageUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	return c.String(http.StatusOK, msg.Message)
}

func ThreadEditMessageHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.ForumDisabledErr.Error(), Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/t/" + string(threadUUID), Type: "alert-danger"})
	}
	messageUUID := database.ForumMessageUUID(c.Param("messageUUID"))
	thread, err := db.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	msg, err := db.GetForumMessageByUUID(messageUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if msg.UserID != authUser.ID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/")
	}
	var data threadReplyData
	data.IsEdit = true
	data.Thread = thread
	data.Message = msg.Message

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "20000") {
			data.ErrorMessage = "Message must have 3 to 20k characters"
			return c.Render(http.StatusOK, "thread-reply", data)
		}
		if isForumSpam(data.Message) {
			db.NewAudit(*authUser, fmt.Sprintf("spam forum edit msg %s (#%d)", authUser.Username, authUser.ID))
			authUser.SetCanUseForum(db, false)
			return c.Redirect(http.StatusFound, "/")
		}
		msg.Message = data.Message
		msg.IsSigned = msg.ValidateSignature(authUser.GPGPublicKey)
		msg.DoSave(db)
		return c.Redirect(http.StatusFound, "/t/"+string(thread.UUID))
	}

	return c.Render(http.StatusOK, "thread-reply", data)
}

func isForumSpam(msg string) bool {
	if strings.Contains(strings.ToLower(msg), "profjerry") ||
		strings.Contains(strings.ToLower(msg), "autorization.online") {
		return true
	}
	return false
}

func ThreadDeleteMessageHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.ForumDisabledErr.Error(), Redirect: "/", Type: "alert-danger"})
	}
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/t/" + string(threadUUID), Type: "alert-danger"})
	}
	messageUUID := database.ForumMessageUUID(c.Param("messageUUID"))
	msg, err := db.GetForumMessageByUUID(messageUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/t/"+string(threadUUID))
	}

	if authUser.ID != msg.UserID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/t/"+string(threadUUID))
	}

	if !msg.CanEdit() && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/t/"+string(threadUUID))
	}

	var data deleteForumMessageData
	data.Thread, err = db.GetForumThreadByID(msg.ThreadID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/t/"+string(threadUUID))
	}
	data.Message = msg

	if c.Request().Method == http.MethodPost {
		if err := db.DeleteForumMessageByID(msg.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/t/"+string(data.Thread.UUID))
	}

	return c.Render(http.StatusOK, "thread-message-delete", data)
}

func NewThreadHandler(c echo.Context) error {
	if config.ForumEnabled.IsFalse() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.ForumDisabledErr.Error(), Redirect: "/", Type: "alert-danger"})
	}
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/forum", Type: "alert-danger"})
	}
	var data newThreadData

	if c.Request().Method == http.MethodPost {
		data.ThreadName = c.Request().PostFormValue("thread_name")
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.ThreadName, "3", "255") {
			data.ErrorThreadName = "Thread name must have 3-255 characters"
			return c.Render(http.StatusOK, "new-thread", data)
		}
		if !govalidator.RuneLength(data.Message, "3", "20000") {
			data.ErrorMessage = "Thread message must have at least 3-20000 characters"
			return c.Render(http.StatusOK, "new-thread", data)
		}
		if isForumSpam(data.Message) {
			db.NewAudit(*authUser, fmt.Sprintf("spam forum new thread %s (#%d)", authUser.Username, authUser.ID))
			authUser.SetCanUseForum(db, false)
			return c.Redirect(http.StatusFound, "/")
		}
		thread := database.MakeForumThread(data.ThreadName, authUser.ID, 1)
		db.DB().Create(&thread)
		message := database.MakeForumMessage(data.Message, authUser.ID, thread.ID)
		message.IsSigned = message.ValidateSignature(authUser.GPGPublicKey)
		db.DB().Create(&message)
		_ = db.SubscribeToForumThread(authUser.ID, thread.ID)
		return c.Redirect(http.StatusFound, "/t/"+string(thread.UUID))
	}

	return c.Render(http.StatusOK, "new-thread", data)
}

func ThreadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := db.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data threadData
	data.Thread = thread

	if err := db.DB().
		Table("forum_messages").
		Scopes(func(query *gorm.DB) *gorm.DB {
			query.Where("thread_id = ?", thread.ID)
			data.CurrentPage, data.MaxPage, data.MessagesCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Order("id ASC").
		Preload("User").
		Find(&data.Messages).Error; err != nil {
		logrus.Error(err)
	}

	if authUser != nil {
		data.IsSubscribed = db.IsUserSubscribedToForumThread(authUser.ID, thread.ID)
		// Update read record
		db.UpdateForumReadRecord(authUser.ID, thread.ID)
	}

	return c.Render(http.StatusOK, "thread", data)
}

func ForumReindexHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	if err := db.DB().Exec(`INSERT INTO fts5_forum_threads(fts5_forum_threads) VALUES('rebuild')`).Error; err != nil {
		logrus.Error(err)
	}
	if err := db.DB().Exec(`INSERT INTO fts5_forum_messages(fts5_forum_messages) VALUES('rebuild')`).Error; err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/forum")
}
