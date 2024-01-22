package handlers

import (
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"net/http"
)

func ClubHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data clubData
	data.ActiveTab = "home"
	data.ForumThreads, _ = db.GetClubForumThreads(authUser.ID)
	return c.Render(http.StatusOK, "club.home", data)
}

func ClubNewThreadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data clubNewThreadData
	data.ActiveTab = "home"

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
		db.DB().Create(&thread)
		message := database.MakeForumMessage(data.Message, authUser.ID, thread.ID)
		db.DB().Create(&message)
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
	var data clubNewThreadReplyData
	data.ActiveTab = "home"
	data.Thread = thread

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Message must have at least 3 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		message := database.MakeForumMessage(data.Message, authUser.ID, thread.ID)
		db.DB().Create(&message)
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
	var data clubNewThreadReplyData
	data.ActiveTab = "home"
	data.IsEdit = true
	data.Thread = thread
	data.Message = msg.Message

	if c.Request().Method == http.MethodPost {
		data.Message = c.Request().PostFormValue("message")
		if !govalidator.RuneLength(data.Message, "3", "10000") {
			data.ErrorMessage = "Message must have at least 3 characters"
			return c.Render(http.StatusOK, "club.new-thread", data)
		}
		msg.Message = data.Message
		msg.DoSave(db)
		return c.Redirect(http.StatusFound, "/club/threads/"+utils.FormatInt64(int64(thread.ID)))
	}

	return c.Render(http.StatusOK, "club.thread-reply", data)
}

func ClubMembersHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data clubMembersData
	data.ActiveTab = "members"
	data.Members, _ = db.GetClubMembers()
	return c.Render(http.StatusOK, "club.members", data)
}

func ClubThreadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadID := database.ForumThreadID(utils.DoParseInt64(c.Param("threadID")))
	thread, err := db.GetForumThread(threadID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	var data clubThreadData
	data.ActiveTab = "home"
	data.Thread = thread
	data.Messages, _ = db.GetThreadMessages(threadID)

	// Update read record
	db.UpdateForumReadRecord(authUser.ID, threadID)

	return c.Render(http.StatusOK, "club.thread", data)
}
