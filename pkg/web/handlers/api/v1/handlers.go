package v1

import (
	"dkforest/pkg/LeChatPHP/captcha"
	mycaptcha "dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/global"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors"
	"dkforest/pkg/web/handlers/interceptors/command"
	hutils "dkforest/pkg/web/handlers/utils"
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	qt "github.com/valyala/quicktemplate"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetChatMenuData gets the data needed to render the "right-menu" in a chat room.
// We have it separate because we have one endpoint that only render the right menu (for "stream" chat).
// and one endpoint to render both messages and menu ("non-stream" chat).
func GetChatMenuData(c echo.Context, room database.ChatRoom) ChatMenuData {
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)

	data := ChatMenuData{}
	data.PreventRefresh = utils.DoParseBool(c.QueryParam("r"))
	sessionToken := ""
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	if authCookie != nil {
		sessionToken = authCookie.Value
	}
	data.InboxCount = global.GetUserNotificationCount(db, authUser.ID, sessionToken)
	data.OfficialRooms, _ = db.GetOfficialChatRooms1(authUser.ID)
	data.SubscribedRooms, _ = db.GetUserRoomSubscriptions(authUser.ID)

	membersInRoom, membersInChat := managers.ActiveUsers.GetRoomUsers(room, managers.GetUserIgnoreSet(db, authUser))
	data.Members = membersInRoom
	data.MembersInChat = membersInChat
	for _, user := range membersInChat {
		if !user.IsHellbanned {
			data.VisibleMemberInChat = true
			break
		}
	}
	data.RoomName = room.Name

	if _, found := c.QueryParams()["ml"]; found {
		data.TopBarQueryParams = "&ml=1"
	}

	return data
}

func chatMessages(c echo.Context) (status int, data ChatMessagesData) {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	roomName := c.Param("roomName")

	pmOnlyQuery := dutils.DoParsePmDisplayMode(c.QueryParam("pmonly"))
	mentionsOnlyQuery := utils.DoParseBool(c.QueryParam("mentionsOnly"))
	pmUserID := dutils.GetUserIDFromUsername(db, c.QueryParam(command.RedirectPmUsernameQP))

	room, roomKey, err := dutils.GetRoomAndKey(db, c, roomName)
	if err != nil {
		return http.StatusForbidden, data
	}

	managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfo(authUser))

	displayHellbanned := authUser.DisplayHellbanned || authUser.IsHellbanned
	displayIgnoredMessages := utils.False()
	msgs, err := db.GetChatMessages(room.ID, roomKey, authUser.Username, authUser.ID, pmUserID, pmOnlyQuery, mentionsOnlyQuery,
		displayHellbanned, authUser.DisplayIgnored, authUser.DisplayModerators, displayIgnoredMessages, 150, 0)
	if err != nil {
		return http.StatusInternalServerError, data
	}

	// Update read record
	db.UpdateChatReadRecord(authUser.ID, room.ID)

	data.Error = c.QueryParam("error")
	if data.Error != "" {
		errorDisplayTime := int64(4) // Time in seconds
		nowUnix := time.Now().Unix()
		data.ErrorTs = utils.DoParseInt64(c.QueryParam("errorTs"))
		if nowUnix > data.ErrorTs+errorDisplayTime {
			data.Error = ""
		}
	}

	// If your tutorial was reset (you are not a new user), force display manual refresh popup
	if ((room.IsOfficialRoom() || (room.IsListed && !room.IsProtected())) && !authUser.TutorialCompleted()) &&
		authUser.GeneralMessagesCount > 0 {
		data.ForceManualRefresh = true
	}

	data.ManualRefreshTimeout = authUser.RefreshRate + 25
	data.Messages = msgs
	data.ReadMarker, _ = db.GetUserReadMarker(authUser.ID, room.ID)
	data.NbButtons = authUser.CountUIButtons()

	if authUser.NotifyNewMessage || authUser.NotifyPmmed || authUser.NotifyTagged {
		lastKnownDate := ""
		if lastKnownDateCookie, err := hutils.GetLastMsgCookie(c, roomName); err == nil {
			lastKnownDate = lastKnownDateCookie.Value
		}
		newMessageSound, pmSound, taggedSound, lastMessageCreatedAt := shouldPlaySound(authUser, lastKnownDate, msgs)
		hutils.CreateLastMsgCookie(c, roomName, lastMessageCreatedAt)
		data.NewMessageSound = utils.TernaryOrZero(authUser.NotifyNewMessage, newMessageSound)
		data.PmSound = utils.TernaryOrZero(authUser.NotifyPmmed, pmSound)
		data.TaggedSound = utils.TernaryOrZero(authUser.NotifyTagged, taggedSound)
	}

	data.ChatMenuData = GetChatMenuData(c, room)

	return http.StatusOK, data
}

func writeunesc(iow io.Writer, s string) {
	qw := qt.AcquireWriter(iow)
	streamunesc(qw, s)
	qt.ReleaseWriter(qw)
}

func unesc(s string) string {
	qb := qt.AcquireByteBuffer()
	writeunesc(qb, s)
	qs := string(qb.B)
	qt.ReleaseByteBuffer(qb)
	return qs
}

func streamunesc(qw *qt.Writer, s string) {
	qw.N().S(s)
}

// ChatMessagesHandler room messages iframe handler
// The chat messages iframe use this endpoint to get the messages for a room.
func ChatMessagesHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	status, data := chatMessages(c)
	if status != http.StatusOK {
		return c.NoContent(status)
	}
	if c.QueryParams().Has("json") {
		authUser := c.Get("authUser").(*database.User)
		if authUser.IsHellbanned || (!authUser.IsModerator() && !authUser.CanSeeHellbanned) {
			for i := range data.Messages {
				data.Messages[i].IsHellbanned = false
			}
		}
		return c.JSON(http.StatusOK, data)
	}
	version := config.Global.AppVersion.Get().Original()
	csrf, _ := c.Get("csrf").(string)
	return c.HTML(http.StatusOK, Messages(version, csrf, config.NullUsername, authUser, data))
	//return c.Render(http.StatusOK, "chat-messages", data)
}

func RoomNotifierHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	roomName := c.Param("roomName")
	lastKnownDate := c.Request().PostFormValue("last_known_date")

	room, roomKey, err := dutils.GetRoomAndKey(db, c, roomName)
	if err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfo(authUser))

	displayHellbanned := authUser.DisplayHellbanned || authUser.IsHellbanned
	mentionsOnly := utils.False()
	displayIgnoredMessages := utils.False()
	var pmUserID *database.UserID
	msgs, err := db.GetChatMessages(room.ID, roomKey, authUser.Username, authUser.ID, pmUserID, database.PmNoFilter, mentionsOnly,
		displayHellbanned, authUser.DisplayIgnored, authUser.DisplayModerators, displayIgnoredMessages, 150, 0)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	var data testData

	data.NewMessageSound, data.PmSound, data.TaggedSound, data.LastMessageCreatedAt = shouldPlaySound(authUser, lastKnownDate, msgs)
	data.InboxCount = db.GetUserInboxMessagesCount(authUser.ID)

	return c.JSON(http.StatusOK, data)
}

// Given a "lastKnownDate" and a list of messages, returns what sound notification should be played.
func shouldPlaySound(authUser *database.User, lastKnownDate string, msgs []database.ChatMessage) (newMessageSound, pmSound, taggedSound bool, lastMsgCreatedAt string) {
	if len(msgs) > 0 {
		if lastKnownMsgDate, err := time.Parse(time.RFC3339Nano, lastKnownDate); err == nil {
			for _, msg := range msgs {
				lastKnownDateTrunc := lastKnownMsgDate.Truncate(time.Second)
				createdAtTrunc := msg.CreatedAt.Truncate(time.Second)
				if createdAtTrunc.After(lastKnownDateTrunc) {
					if msg.User.ID != authUser.ID {
						newMessageSound = true
						if strings.Contains(msg.Message, authUser.Username.AtStr()) {
							taggedSound = true
						}
						if msg.IsPmRecipient(authUser.ID) {
							pmSound = true
						}
						break
					}
				} else if createdAtTrunc.Before(lastKnownMsgDate) {
					break
				}
			}
		}
		lastMsg := msgs[0]
		lastMsgCreatedAt = lastMsg.CreatedAt.Format(time.RFC3339)
	}
	return newMessageSound, pmSound, taggedSound, lastMsgCreatedAt
}

func UserHellbanHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	userID := dutils.DoParseUserID(c.Param("userID"))
	user, err := db.GetUserByID(userID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	if !user.IsHellbanned {
		if authUser.IsAdmin || !user.IsModerator() {
			db.NewAudit(*authUser, fmt.Sprintf("hellban %s #%d", user.Username, user.ID))
			user.HellBan(db)
			managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(&user))
		}
	}
	return hutils.RedirectReferer(c)
}

func UserUnHellbanHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	userID := dutils.DoParseUserID(c.Param("userID"))
	user, err := db.GetUserByID(userID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	if user.IsHellbanned {
		db.NewAudit(*authUser, fmt.Sprintf("unhellban %s #%d", user.Username, user.ID))
		user.UnHellBan(db)
		managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(&user))
	}
	return hutils.RedirectReferer(c)
}

func KickHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	userID := dutils.DoParseUserID(c.Param("userID"))
	user, err := db.GetUserByID(userID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	if user.IsModerator() {
		return hutils.RedirectReferer(c)
	}
	_ = dutils.SilentKick(db, user, *authUser)
	return hutils.RedirectReferer(c)
}

func SubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	roomName := c.Param("roomName")
	room, err := db.GetChatRoomByName(roomName)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	_ = db.SubscribeToRoom(authUser.ID, room.ID)
	return hutils.RedirectReferer(c)
}

func UnsubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	roomName := c.Param("roomName")
	room, err := db.GetChatRoomByName(roomName)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	_ = db.UnsubscribeFromRoom(authUser.ID, room.ID)
	return hutils.RedirectReferer(c)
}

func ThreadSubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := db.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	_ = db.SubscribeToForumThread(authUser.ID, thread.ID)
	return hutils.RedirectReferer(c)
}

func ThreadUnsubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := db.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}
	_ = db.UnsubscribeFromForumThread(authUser.ID, thread.ID)
	return hutils.RedirectReferer(c)
}

func ChatMessageReactionHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	messageUUID := c.Request().PostFormValue("message_uuid")
	var msg database.ChatMessage
	if err := db.DB().Where("uuid = ?", messageUUID).Preload("User").Preload("Room").First(&msg).Error; err != nil {
		return err
	}
	reaction := utils.DoParseInt64(c.Request().PostFormValue("reaction_id"))
	if reaction < 0 || reaction > 2 {
		return errors.New("invalid reaction")
	}

	if err := db.CreateChatReaction(authUser.ID, msg.ID, reaction); err != nil {
		_ = db.DeleteReaction(authUser.ID, msg.ID, reaction)
	}

	return hutils.RedirectReferer(c)
}

func ChatDeleteMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	messageUUID := c.Param("messageUUID")
	var msg database.ChatMessage
	if err := db.DB().Where("uuid = ?", messageUUID).
		Preload("User").
		Preload("Room").
		First(&msg).Error; err != nil {
		return err
	}

	if err := msg.UserCanDeleteErr(authUser); err != nil {
		logrus.Error(err)
		return hutils.RedirectReferer(c)
	}

	// Audit when moderator/admin deletes a message he doesn't own
	if authUser.IsModerator() && !msg.OwnMessage(authUser.ID) && msg.User.Username != config.NullUsername {
		auditMsg := fmt.Sprintf(`deleted msg #%d from user "%s" #%d -> %s`,
			msg.ID,
			msg.User.Username,
			msg.User.ID,
			utils.TruncStr(msg.RawMessage, 75, "…"))
		db.NewAudit(*authUser, auditMsg)
	}

	if msg.OwnMessage(authUser.ID) && msg.RoomID == config.GeneralRoomID && !msg.IsPm() {
		authUser.DecrGeneralMessagesCount(db)
	}

	if err := msg.Delete(db); err != nil {
		logrus.Error(err)
	}

	if c.Request().Method == http.MethodGet {
		return c.NoContent(http.StatusOK)
	}
	return hutils.RedirectReferer(c)
}

func ClubDeleteMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	messageID := database.ForumMessageID(utils.DoParseInt64(c.Param("messageID")))
	msg, err := db.GetForumMessage(messageID)
	if err != nil {
		return hutils.RedirectReferer(c)
	}

	if authUser.ID != msg.UserID && !authUser.IsAdmin {
		return hutils.RedirectReferer(c)
	}

	if !msg.CanEdit() && !authUser.IsAdmin {
		return hutils.RedirectReferer(c)
	}

	if err := db.DeleteForumMessageByID(messageID); err != nil {
		logrus.Error(err)
	}
	return hutils.RedirectReferer(c)
}

func DeleteNotificationHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	notificationID := utils.DoParseInt64(c.Param("notificationID"))
	var msg database.Notification
	if err := db.DB().Where("ID = ? AND user_id = ?", notificationID, authUser.ID).First(&msg).Error; err != nil {
		logrus.Error(err)
		return hutils.RedirectReferer(c)
	}
	if err := db.DeleteNotificationByID(notificationID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/settings/inbox")
}

func DeleteSessionNotificationHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	sessionNotificationID := utils.DoParseInt64(c.Param("sessionNotificationID"))
	var msg database.SessionNotification
	if err := db.DB().Where("ID = ? AND session_token = ?", sessionNotificationID, authCookie.Value).First(&msg).Error; err != nil {
		logrus.Error(err)
		return hutils.RedirectReferer(c)
	}
	if err := db.DeleteSessionNotificationByID(sessionNotificationID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/settings/inbox")
}

func ChatInboxDeleteMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	messageID := utils.DoParseInt64(c.Param("messageID"))
	var msg database.ChatInboxMessage
	if err := db.DB().Where("ID = ? AND to_user_id = ?", messageID, authUser.ID).First(&msg).Error; err != nil {
		logrus.Error(err)
		return c.Redirect(http.StatusFound, "/settings/inbox")
	}
	if err := db.DeleteChatInboxMessageByID(messageID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/settings/inbox")
}

func ChatInboxDeleteAllMessageHandler(c echo.Context) error {
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if err := db.DeleteAllChatInbox(authUser.ID); err != nil {
		logrus.Error(err)
	}
	if err := db.DeleteAllNotifications(authUser.ID); err != nil {
		logrus.Error(err)
	}
	if err := db.DeleteAllSessionNotifications(authCookie.Value); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/settings/inbox")
}

func GetCaptchaHandler(c echo.Context) error {
	//authUser := c.Get("authUser").(*database.User)
	captchaID, captchaImg := mycaptcha.New()
	return c.JSON(http.StatusOK, map[string]any{"ID": captchaID, "img": captchaImg})
}

func CaptchaSolverHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	captchaB64 := c.Request().PostFormValue("captcha")
	answer, err := captcha.SolveBase64(captchaB64)
	if err != nil {
		logrus.Error(err.Error())
		return c.NoContent(http.StatusInternalServerError)
	}
	captchaReq := database.CaptchaRequest{
		UserID:     authUser.ID,
		CaptchaImg: captchaB64,
		Answer:     answer,
	}
	if err := db.DB().Create(&captchaReq).Error; err != nil {
		logrus.Error(err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"answer": answer})
}

func WerewolfHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	roomName := "werewolf"
	origMessage := c.Request().PostFormValue("message")
	redirectURL := "/api/v1/chat/messages/" + roomName
	room, roomKey, err := dutils.GetRoomAndKey(db, c, roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, redirectURL+"?error="+err.Error()+"&errorTs="+utils.FormatInt64(time.Now().Unix()))
	}
	cmd := command.NewCommand(c, origMessage, room, roomKey)
	interceptors.WWInstance.InterceptMsg(cmd)
	if cmd.Err != nil {
		return c.Redirect(http.StatusFound, redirectURL+"?error="+cmd.Err.Error()+"&errorTs="+utils.FormatInt64(time.Now().Unix()))
	}
	return c.Redirect(http.StatusFound, redirectURL)
}

func BattleshipHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	roomName := c.Request().PostFormValue("room")
	enemyUsername := database.Username(c.Request().PostFormValue("enemyUsername"))
	pos := c.Request().PostFormValue("move")
	redirectURL := "/api/v1/chat/messages/" + roomName
	room, roomKey, err := dutils.GetRoomAndKey(db, c, roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, redirectURL+"?error="+err.Error()+"&errorTs="+utils.FormatInt64(time.Now().Unix()))
	}
	if err = interceptors.BattleshipInstance.PlayMove(roomName, room.ID, roomKey, *authUser, enemyUsername, pos); err != nil {
		return c.Redirect(http.StatusFound, redirectURL+"?error="+err.Error()+"&errorTs="+utils.FormatInt64(time.Now().Unix()))
	}
	return c.Redirect(http.StatusFound, redirectURL)
}
