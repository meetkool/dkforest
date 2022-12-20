package v1

import (
	"dkforest/pkg/config"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/global"
	"dkforest/pkg/hashset"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dkforest/pkg/LeChatPHP/captcha"
	mycaptcha "dkforest/pkg/captcha"
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

var usernameF = `\w{3,20}` // username (regex Fragment)
var userOr0 = usernameF + `|0`
var groupName = `\w{3,20}`
var roomNameF = `\w{3,50}`
var chatTs = `\d{2}:\d{2}:\d{2}`
var optAtGUser = `@?(` + usernameF + `)`  // Optional @, Grouped, Username
var optAtGUserOr0 = `@?(` + userOr0 + `)` // Optional @, Grouped, Username or 0
var onionV2Rgx = regexp.MustCompile(`[a-z2-7]{16}\.onion`)
var onionV3Rgx = regexp.MustCompile(`[a-z2-7]{56}\.onion`)
var deleteMsgRgx = regexp.MustCompile(`^/d (\d{2}:\d{2}:\d{2})(?:\s` + optAtGUserOr0 + `)?$`)
var ignoreRgx = regexp.MustCompile(`^/(?:ignore|i) ` + optAtGUser)
var pmToggleWhitelistUserRgx = regexp.MustCompile(`^/pmw ` + optAtGUser)
var pmToggleBlacklistUserRgx = regexp.MustCompile(`^/pmb ` + optAtGUser)
var whitelistUserRgx = regexp.MustCompile(`^/(?:whitelist|wl) ` + optAtGUser)
var unIgnoreRgx = regexp.MustCompile(`^/(?:unignore|ui) ` + optAtGUser)
var groupRgx = regexp.MustCompile(`^/g (` + groupName + `)\s(?s:(.*))`)
var pmRgx = regexp.MustCompile(`^/pm ` + optAtGUserOr0 + `\s(?s:(.*))`)
var editRgx = regexp.MustCompile(`^/e (` + chatTs + `)\s(?s:(.*))`)
var hbmtRgx = regexp.MustCompile(`^/hbmt (` + chatTs + `)$`)
var chessRgx = regexp.MustCompile(`^/chess ` + optAtGUser)
var inboxRgx = regexp.MustCompile(`^/inbox ` + optAtGUser + `(\s-e)?\s(?s:(.*))`)
var profileRgx = regexp.MustCompile(`^/p ` + optAtGUserOr0)
var kickRgx = regexp.MustCompile(`^/(?:kick|k) ` + optAtGUser)
var kickKeepRgx = regexp.MustCompile(`^/(?:kk) ` + optAtGUser)
var rtutoRgx = regexp.MustCompile(`^/(?:rtuto) ` + optAtGUser)
var logoutRgx = regexp.MustCompile(`^/(?:logout) ` + optAtGUser)
var forceCaptchaRgx = regexp.MustCompile(`^/(?:captcha) ` + optAtGUser)
var unkickRgx = regexp.MustCompile(`^/(?:unkick|uk) ` + optAtGUser)
var hellbanRgx = regexp.MustCompile(`^/(?:hellban|hb) ` + optAtGUser)
var unhellbanRgx = regexp.MustCompile(`^/(?:unhellban|uhb) ` + optAtGUser)
var tokenRgx = regexp.MustCompile(`^/token (\d{1,2})$`)
var tagRgx = regexp.MustCompile(`@(` + userOr0 + `)`)
var autoTagRgx = regexp.MustCompile(`@(\w+)\*`)
var roomTagRgx = regexp.MustCompile(`#(` + roomNameF + `)`)
var tzRgx = regexp.MustCompile(`(\d{4}-\d{1,2}-\d{1,2} at \d{1,2}\.\d{1,2}\.\d{1,2} [A|P]M)`) // Screen Shot 2022-02-04 at 11.58.58 PM
var addGroupRgx = regexp.MustCompile(`^/addgroup (` + groupName + `)$`)
var rmGroupRgx = regexp.MustCompile(`^/rmgroup (` + groupName + `)$`)
var lockGroupRgx = regexp.MustCompile(`^/glock (` + groupName + `)$`)
var unlockGroupRgx = regexp.MustCompile(`^/gunlock (` + groupName + `)$`)
var groupUsersRgx = regexp.MustCompile(`^/gusers (` + groupName + `)$`)
var groupAddUserRgx = regexp.MustCompile(`^/gadduser (` + groupName + `) ` + optAtGUser + `$`)
var groupRmUserRgx = regexp.MustCompile(`^/grmuser (` + groupName + `) ` + optAtGUser + `$`)
var unsubscribeRgx = regexp.MustCompile(`^/unsubscribe (` + roomNameF + `)$`)
var bsRgx = regexp.MustCompile(`^/pm ` + optAtGUser + ` /bs\s?([A-J]\d)?$`)
var cRgx = regexp.MustCompile(`^/pm ` + optAtGUser + ` /c\s?(move)?$`)

// ChatMessagesHandler room messages iframe handler
// The chat messages iframe use this endpoint to get the messages for a room.
func ChatMessagesHandler(c echo.Context) error {
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	authUser := c.Get("authUser").(*database.User)
	roomName := c.Param("roomName")

	pmOnlyQuery := utils.DoParseInt64(c.QueryParam("pmonly"))
	mentionsOnlyQuery := utils.DoParseBool(c.QueryParam("mentionsOnly"))

	room, err := database.GetChatRoomByName(roomName)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	if !room.HasAccess(c) {
		return c.NoContent(http.StatusForbidden)
	}

	managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfo(*authUser, nil))

	ignoredSet := hashset.New[string]()
	// Only fill the ignored set if the user does not display the ignored users ("Toggle ignored" chat setting)
	// and if the user has "Hide ignored users from users lists" enabled (user setting)
	if !authUser.DisplayIgnored && authUser.HideIgnoredUsersFromList {
		ignoredUsers, _ := database.GetIgnoredUsers(authUser.ID)
		for _, ignoredUser := range ignoredUsers {
			ignoredSet.Insert(ignoredUser.IgnoredUser.Username)
		}
	}

	membersInRoom, membersInChat := managers.ActiveUsers.GetRoomUsers(room, ignoredSet)
	msgs, _ := database.GetChatMessages(room.ID, authUser.Username, authUser.ID, pmOnlyQuery, mentionsOnlyQuery, authUser.DisplayHellbanned || authUser.IsHellbanned, authUser.DisplayIgnored, authUser.DisplayModerators)
	if room.IsProtected() {
		key, err := hutils.GetRoomKeyCookie(c, int64(room.ID))
		if err != nil {
			return c.NoContent(http.StatusForbidden)
		}
		if err := msgs.DecryptAll(key.Value); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// Update read record
	database.DB.Create(database.ChatReadRecord{UserID: authUser.ID, RoomID: room.ID})
	database.DB.Table("chat_read_records").Where("user_id = ? AND room_id = ?", authUser.ID, room.ID).Update("read_at", time.Now())

	var data chatMessagesData
	data.ManualRefreshTimeout = authUser.RefreshRate + 25
	data.DateFormat = authUser.GetDateFormat()
	data.IsModerator = authUser.IsModerator()
	data.Messages = msgs
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
		topBarQueryParams := url.Values{}
		topBarQueryParams.Set("ml", "1")
		topBarQueryParamsStr := topBarQueryParams.Encode()
		if topBarQueryParamsStr != "" {
			topBarQueryParamsStr = "&" + topBarQueryParamsStr
		}
		data.TopBarQueryParams = topBarQueryParamsStr
	}
	data.PreventRefresh = utils.DoParseBool(c.QueryParam("r"))

	sessionToken := ""
	if authCookie != nil {
		sessionToken = authCookie.Value
	}
	data.InboxCount = global.GetUserNotificationCount(authUser.ID, sessionToken)

	data.ReadMarker, _ = database.GetUserReadMarker(authUser.ID, room.ID)
	data.OfficialRooms, _ = database.GetOfficialChatRooms1(authUser.ID)
	data.SubscribedRooms, _ = database.GetUserRoomSubscriptions(authUser.ID)

	if authUser.DisplayHellbanButton {
		data.NbButtons += 1
	}
	if authUser.DisplayKickButton {
		data.NbButtons += 1
	}
	if authUser.DisplayDeleteButton {
		data.NbButtons += 1
	}

	if c.QueryParams().Has("json") {
		return c.JSON(http.StatusOK, data)
	}

	return c.Render(http.StatusOK, "chat-messages", data)
}

func UserHellbanHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if authUser.IsModerator() {
		userID := dutils.DoParseUserID(c.Param("userID"))
		user, err := database.GetUserByID(userID)
		if err != nil {
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}
		if !user.IsHellbanned {
			if authUser.IsAdmin || !user.IsModerator() {
				database.NewAudit(*authUser, fmt.Sprintf("hellban %s #%d", user.Username, user.ID))
				user.HellBan()
				managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(user, nil))
			}
		} else {
			database.NewAudit(*authUser, fmt.Sprintf("unhellban %s #%d", user.Username, user.ID))
			user.UnHellBan()
			managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(user, nil))
		}
	}
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func KickHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if authUser.IsModerator() {
		userID := dutils.DoParseUserID(c.Param("userID"))
		user, err := database.GetUserByID(userID)
		if err != nil {
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}
		if user.IsModerator() {
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}
		dutils.SilentKick(user, *authUser)
	}
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func SubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	roomName := c.Param("roomName")
	room, err := database.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	_ = database.SubscribeToRoom(authUser.ID, room.ID)
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func UnsubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	roomName := c.Param("roomName")
	room, err := database.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	_ = database.UnsubscribeFromRoom(authUser.ID, room.ID)
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func ThreadSubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := database.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	_ = database.SubscribeToForumThread(authUser.ID, thread.ID)
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func ThreadUnsubscribeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	threadUUID := database.ForumThreadUUID(c.Param("threadUUID"))
	thread, err := database.GetForumThreadByUUID(threadUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	_ = database.UnsubscribeFromForumThread(authUser.ID, thread.ID)
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func ChatMessageReactionHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	messageUUID := c.Request().PostFormValue("message_uuid")
	var msg database.ChatMessage
	if err := database.DB.Where("uuid = ?", messageUUID).Preload("User").Preload("Room").First(&msg).Error; err != nil {
		return err
	}
	reaction := utils.DoParseInt64(c.Request().PostFormValue("reaction_id"))
	if reaction < 0 || reaction > 2 {
		return errors.New("invalid reaction")
	}

	if err := database.CreateChatReaction(authUser.ID, msg.ID, reaction); err != nil {
		_ = database.DeleteReaction(authUser.ID, msg.ID, reaction)
	}

	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func ChatDeleteMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)

	messageUUID := c.Param("messageUUID")
	var msg database.ChatMessage
	if err := database.DB.Where("uuid = ?", messageUUID).
		Preload("User").
		Preload("Room").
		First(&msg).Error; err != nil {
		return err
	}

	if !msg.UserCanDelete(*authUser) {
		return errors.New("cannot delete this message")
	}

	if authUser.IsAdmin {
	} else if authUser.IsModerator() {
		if msg.User.Username != config.NullUsername {
			if msg.TooOldToDelete() && msg.UserID == authUser.ID {
				return c.Redirect(http.StatusFound, c.Request().Referer())
			}
			if msg.UserID != authUser.ID {
				auditMsg := fmt.Sprintf(`deleted msg #%d from user "%s" #%d -> %s`,
					msg.ID,
					msg.User.Username,
					msg.User.ID,
					utils.TruncStr(msg.RawMessage, 75, "…"))
				database.NewAudit(*authUser, auditMsg)
			}
		}
	} else if msg.Room.OwnerUserID != nil && authUser.ID == *msg.Room.OwnerUserID { // Room owner can delete messages in its room
	} else if msg.TooOldToDelete() {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if msg.RoomID == config.GeneralRoomID && msg.ToUserID == nil {
		authUser.GeneralMessagesCount--
		authUser.DoSave()
	}

	// If we delete message manually, also delete linked inbox if any
	_ = database.DeleteChatInboxMessageByChatMessageID(msg.ID)
	if err := database.DeleteChatMessageByUUID(messageUUID); err != nil {
		logrus.Error(err)
	}

	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func ClubDeleteMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	messageID := database.ForumMessageID(utils.DoParseInt64(c.Param("messageID")))
	msg, err := database.GetForumMessage(messageID)
	if err != nil {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if authUser.ID != msg.UserID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if !msg.CanEdit() && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	if err := database.DeleteForumMessageByID(messageID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func DeleteNotificationHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	notificationID := utils.DoParseInt64(c.Param("notificationID"))
	var msg database.Notification
	if err := database.DB.Where("ID = ? AND user_id = ?", notificationID, authUser.ID).First(&msg).Error; err != nil {
		logrus.Error(err)
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	if err := database.DeleteNotificationByID(notificationID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/settings/inbox")
}

func DeleteSessionNotificationHandler(c echo.Context) error {
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	sessionNotificationID := utils.DoParseInt64(c.Param("sessionNotificationID"))
	var msg database.SessionNotification
	if err := database.DB.Where("ID = ? AND session_token = ?", sessionNotificationID, authCookie.Value).First(&msg).Error; err != nil {
		logrus.Error(err)
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}
	if err := database.DeleteSessionNotificationByID(sessionNotificationID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/settings/inbox")
}

func ChatInboxDeleteMessageHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	messageID := utils.DoParseInt64(c.Param("messageID"))
	var msg database.ChatInboxMessage
	if err := database.DB.Where("ID = ? AND to_user_id = ?", messageID, authUser.ID).First(&msg).Error; err != nil {
		logrus.Error(err)
		return c.Redirect(http.StatusFound, "/settings/inbox")
	}
	if err := database.DeleteChatInboxMessageByID(messageID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/settings/inbox")
}

func ChatInboxDeleteAllMessageHandler(c echo.Context) error {
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	authUser := c.Get("authUser").(*database.User)
	if err := database.DeleteAllChatInbox(authUser.ID); err != nil {
		logrus.Error(err)
	}
	if err := database.DeleteAllNotifications(authUser.ID); err != nil {
		logrus.Error(err)
	}
	if err := database.DeleteAllSessionNotifications(authCookie.Value); err != nil {
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
	if err := database.DB.Create(&captchaReq).Error; err != nil {
		logrus.Error(err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"answer": answer})
}

func RoomNotifierHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	roomID := database.RoomID(utils.DoParseInt64(c.Param("roomID")))
	lastKnownDate := c.Request().PostFormValue("last_known_date")

	room, err := database.GetChatRoomByID(roomID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	if !room.HasAccess(c) {
		return c.NoContent(http.StatusForbidden)
	}

	managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfo(*authUser, nil))

	msgs, _ := database.GetChatMessages(roomID, authUser.Username, authUser.ID, 0, false, authUser.DisplayHellbanned || authUser.IsHellbanned, authUser.DisplayIgnored, authUser.DisplayModerators)
	if room.IsProtected() {
		key, err := hutils.GetRoomKeyCookie(c, int64(room.ID))
		if err != nil {
			return c.NoContent(http.StatusForbidden)
		}
		if err := msgs.DecryptAll(key.Value); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	var data testData

	newMessageSound := false
	pmSound := false
	taggedSound := false
	if len(msgs) > 0 {
		if lastKnownMsgDate, err := time.Parse(time.RFC3339Nano, lastKnownDate); err == nil {
			for _, msg := range msgs {
				if msg.CreatedAt.Truncate(time.Second).After(lastKnownMsgDate.Truncate(time.Second)) {
					if msg.User.ID != authUser.ID {
						newMessageSound = true
						if strings.Contains(msg.Message, "@"+authUser.Username) {
							taggedSound = true
						}
						if msg.ToUserID != nil && *msg.ToUserID == authUser.ID {
							pmSound = true
						}
						break
					}
				} else if msg.CreatedAt.Truncate(time.Second).Before(lastKnownMsgDate.Truncate(time.Second)) {
					break
				}
			}
		}
		lastMsg := msgs[0]
		data.LastMessageCreatedAt = lastMsg.CreatedAt.Format(time.RFC3339)
	}

	data.NewMessageSound = newMessageSound
	data.PmSound = pmSound
	data.TaggedSound = taggedSound
	data.InboxCount = database.GetUserInboxMessagesCount(authUser.ID)

	return c.JSON(http.StatusOK, data)
}
