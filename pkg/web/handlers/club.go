package handlers

import (
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"net/http"
)

type clubData struct {
	ActiveTab   string
	ForumThreads []database.ForumThread
	ErrorThreadName string
	ErrorMessage string
}

type clubNewThread struct {
	ActiveTab   string
	ThreadName   string
	Message      string
	ErrorThreadName string
	ErrorMessage string
}

type clubNewThreadReply struct {
	ActiveTab   string
	Thread       database.ForumThread
	Message      string
	ErrorMessage string
	IsEdit       bool
}

func ClubHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	data := clubData{
		ActiveTab: "home",
	}
	forumThreads, err := db.GetClubForumThreads(authUser.ID)
	if err != nil {
		return err
	}
	data.ForumThreads = forumThreads
	return c.Render(http.StatusOK, "club.home", data)
}

func ClubNewThreadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	data := clubNewThread{
		ActiveTab: "home",
	}
	if c.Request().Method == http.MethodPost {
		data.ThreadName = c.Request().PostFormValue("thread_name")
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.ThreadName, "3", "255") {
			data.ErrorThreadName = "Thread name must have 3-255 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Thread name must have at least 3 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		thread := database.MakeForumThread(data.ThreadName, authUser.ID, 0)
		if err := db.DB().Create(&thread).Error; err != nil {
			return err
		}
		message := database.MakeForumMessage(data.Message, authUser.ID, thread.ID)
		if err := db.DB().Create(&message).Error; err != nil {
			return err
		}
		return c.Redirect(http.StatusFound, "/club/threads/"+utils.FormatInt64(int64(thread.ID)))
	}

	return c.Render(http.StatusOK, "club.new-thread", data)
}

func ClubThreadReplyHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadID := database.ForumThreadID(utils.DoParseInt64(c.Param("threadID")))
	thread, err := db.GetForumThread(threadID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	data := clubNewThreadReply{
		ActiveTab:   "home",
		Thread:       thread,
	}
	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Message must have at least 3 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		message := database.MakeForumMessage(data.Message, authUser.ID, thread.ID)
		if err := db.DB().Create(&message).Error; err != nil {
			return err
		}
		return c.Redirect(http.StatusFound, "/club/threads/"+utils.FormatInt64(int64(thread.ID)))
	}

	return c.Render(http.StatusOK, "club.thread-reply", data)
}

func ClubThreadEditMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadID := database.ForumThreadID(utils.DoParseInt64(c.Param("threadID")))
	messageID := database.ForumMessageID(utils.DoParseInt64(c.Param("messageID")))
	thread, err := db.GetForumThread(threadID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	msg, err := db.GetForumMessage(messageID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if msg.UserID != authUser.ID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/")
	}
	data := clubNewThreadReply{
		ActiveTab:   "home",
		Thread:       thread,
		Message:      msg.Message,
		IsEdit:       true,
	}
	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.
