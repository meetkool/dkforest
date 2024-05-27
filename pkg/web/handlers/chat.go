package handlers

import (
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/hashset"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/PuerkitoBio/goquery"
	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func RedRoomHandler(c echo.Context) error {
	return chatHandler(c, true, false)
}

func ChatHandler(c echo.Context) error {
	return chatHandler(c, false, false)
}

func ChatStreamHandler(c echo.Context) error {
	return chatHandler(c, false, true)
}

func chatHandler(c echo.Context, redRoom, stream bool) error {
	const chatPasswordTmplName = "standalone.chat-password"

	// WARNING: in this handler, "authUser" can be null.
	authUser := c.Get("authUser").(*database.User)

	db := c.Get("database").(*database.DkfDB)

	if !stream && authUser != nil {
		stream = authUser.UseStream
	}

	var data chatData
	data.PowEnabled = config.PowEnabled.Load()
	data.RedRoom = redRoom
	preventRefresh := utils.DoParseBool(c.QueryParam("r"))

	v := c.QueryParams()
	if preventRefresh {
		v.Set("r", "1")
	}
	if _, found := c.QueryParams()["ml"]; found {
		v.Set("ml", "1")
		data.Multiline = true
	}
	data.ChatQueryParams = "?" + v.Encode()

	if authUser == nil {
		if config.SignupEnabled.IsFalse() {
			return c.Render(http.StatusOK, "flash", FlashResponse{Message: "New signup are temporarily disabled", Redirect: "/", Type: "alert-danger"})
		}

		data.CaptchaID, data.CaptchaImg = captcha.New()
	}

	room, err := db.GetChatRoomByName(getRoomName(c))
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	data.Room = room

	if authUser != nil {
		managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfo(authUser))

		// We display tutorial on official or public rooms
		data.DisplayTutorial = (room.IsOfficialRoom() || (room.IsListed && !room.IsProtected())) && !authUser.TutorialCompleted()

		if data.DisplayTutorial {
			data.TutoSecs = getTutorialStepDuration()
			data.TutoFrames = generateCssFrames(data.TutoSecs, nil, true)
			if c.Request().Method == http.MethodGet {
				authUser.SetChatTutorialTime(db, time.Now())
			}
		}
	}

	if c.Request().Method == http.MethodPost {
		return handlePost(db, c, data, authUser)
	}

	// If you don't have access to the room (room is protected and user is nil or no cookie with the password)
	// We display the page to enter room password.
	if hasAccess, _ := room.HasAccess(c); !hasAccess {
		if !room.IsProtected() && room.Mode == database.UserWhitelistRoomMode {
			return c.Render(http.StatusOK, "standalone.chat-whitelist", data)
		}
		return c.Render(http.StatusOK, chatPasswordTmplName, data)
	}

	data.IsSubscribed = db.IsUserSubscribedToRoom(authUser.ID, room.ID)
	data.IsOfficialRoom = room.IsOfficialRoom()
	data.IsStream = stream
	return c.Render(http.StatusOK, "chat", data)
}

func getRoomName(c echo.Context) string {
	roomName := c.Param("roomName")
	if roomName == "" {
		roomName = "general"
	}
	return roomName
}

func handlePost(db *database.DkfDB, c echo.Context, data chatData, authUser *database.User) error {
	formName := c.Request().PostFormValue("formName")
	switch formName {
	case "logout":
		return handleLogoutPost(c, data.Room)
	case "tutorialP1", "tutorialP2", "tutorialP3":
		return handleTutorialPost(db, c, data, authUser)
	case "chat-password":
		return handleChatPasswordPost(db, c, data, authUser)
	}
	return hutils.RedirectReferer(c)
}

// Logout of a protected room (delete room cookies)
func handleLogoutPost(c echo.Context, room database.ChatRoom) error {
	hutils.DeleteRoomCookie(c, int64(room.ID))
	return c.Redirect(http.StatusFound, "/chat")
}

func handleTutorialPost(db *database.DkfDB, c echo.Context, data chatData, authUser *database.User) error {
	if authUser.ChatTutorial < 3 && time.Since(authUser.ChatTutorialTime) >= time.Duration(data.TutoSecs)*time.Second {
		authUser.IncrChatTutorial(db)
	}
	return hutils.RedirectReferer(c)
}

// Handle POST requests for chat-password, when someone tries to authenticate in a protected room providing a password.
func handleChatPasswordPost(db *database.DkfDB, c echo.Context, data chatData, authUser *database.User) error {
	const chatPasswordTmplName = "standalone.chat-password"
	data.RoomPassword = c.Request().PostFormValue("password")

	// If no user set, we verify the captcha and username for the guest account
	if authUser == nil {
		data.GuestUsername = c.Request().PostFormValue("guest_username")
		data.Pow = c.Request().PostFormValue("pow")
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, chatPasswordTmplName, data)
		}

		if err := db.CanUseUsername(database.Username(data.GuestUsername), false); err != nil {
			data.ErrGuestUsername = err.Error()
			return c.Render(http.StatusOK, chatPasswordTmplName, data)
		}

		// verify POW
		if config.PowEnabled.IsTrue() {
			if !hutils.VerifyPow(data.GuestUsername, data.Pow, config.PowDifficulty) {
				data.ErrPow = "invalid proof of work"
				return c.Render(http.StatusOK, chatPasswordTmplName, data)
			}
		}
	}

	// Verify room password is correct
	key := database.GetRoomDecryptionKey(data.RoomPassword)
	hashedPassword := database.GetRoomPasswordHash(data.RoomPassword)
	if !data.Room.VerifyPasswordHash(hashedPassword) {
		data.Error = "Invalid room password"
		return c.Render(http.StatusOK, chatPasswordTmplName, data)
	}

	// If no user set, create the guest account + session
	// TODO: maybe add "_guest" suffix to guest accounts?
	if authUser == nil {
		password := utils.GenerateToken32()
		newUser, errs := db.CreateGuestUser(data.GuestUsername, password)
		if errs.HasError() {
			data.ErrGuestUsername = errs.Username
			return c.Render(http.StatusOK, chatPasswordTmplName, data)
		}

		session := db.DoCreateSession(newUser.ID, c.Request().UserAgent(), time.Hour*24)
		c.SetCookie(createSessionCookie(session.Token, time.Hour*24))
	}

	hutils.CreateRoomCookie(c, int64(data.Room.ID), hashedPassword, key)
	return c.Redirect(http.StatusFound, "/chat/"+data.Room.Name)
}

func ChatArchiveHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data chatArchiveData
	data.DateFormat = authUser.GetDateFormat()
	roomName := c.Param("roomName")

	room, roomKey, err := dutils.GetRoomAndKey(db, c, roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/chat")
	}

	data.UUID = c.QueryParam("uuid")
	data.Room = room

	if data.UUID != "" {
		msg, err := db.GetRoomChatMessageByUUID(room.ID, data.UUID)
		if err != nil {
			return c.Redirect(http.StatusFound, "/")
		}
		nbMsg := 150
		args := []any{room.ID, authUser.ID, authUser.ID}
		whereClause := `room_id = ? AND group_id IS NULL AND (to_user_id is null OR to_user_id = ? OR user_id = ?)`
		if !authUser.DisplayIgnored {
			args = append(args, authUser.ID)
			whereClause += ` AND user_id NOT IN (SELECT ignored_user_id FROM ignored_users WHERE user_id = ?)`
		}
		raw := `
	SELECT * FROM (
		SELECT *
		FROM chat_messages
		WHERE ` + whereClause + `
		  AND id >= ?
		ORDER BY id ASC
		LIMIT ?
	)
	UNION
	SELECT * FROM (
		SELECT *
		FROM chat_messages
		WHERE ` + whereClause + `
		  AND id < ?
		ORDER BY id DESC
		LIMIT ?
	) ORDER BY id DESC`
		args = append(args, msg.ID, nbMsg)
		args = append(args, args...)
		db.DB().Raw(raw, args...).Scan(&data.Messages)

		// Manually do Preload("Room")
		for _, m := range data.Messages {
			m.Room = data.Room
		}

		//--- < Manually do a Preload("User") Preload("ToUser") > ---
		usersIDs := hashset.New[database.UserID]()
		for _, m := range data.Messages {
			usersIDs.Insert(m.UserID)
			if m.ToUserID != nil {
				usersIDs.Insert(*m.ToUserID)
			}
		}
		users, _ := db.GetUsersByID(usersIDs.ToArray())
		usersMap := make(map[database.UserID]database.User)
		for _, u := range users {
			usersMap[u.ID] = u
		}
		for i, m := range data.Messages {
			if u, ok := usersMap[m.UserID]; ok {
				data.Messages[i].User = u
			}
			if m.ToUserID != nil {
				if u, ok := usersMap[*m.ToUserID]; ok {
					data.Messages[i].ToUser = &u
				}
			}
		}
		//--- </ Manually do a Preload("User") Preload("ToUser") > ---

	} else {
		if err := db.DB().Table("chat_messages").
			Where("room_id = ? AND group_id IS NULL AND (to_user_id is null OR to_user_id = ? OR user_id = ?)", room.ID, authUser.ID, authUser.ID).
			Scopes(func(query *gorm.DB) *gorm.DB {
				if !authUser.DisplayIgnored {
					query = query.Where(`user_id NOT IN (SELECT ignored_user_id FROM ignored_users WHERE user_id = ?)`, authUser.ID)
				}
				data.CurrentPage, data.MaxPage, data.MessagesCount, query = NewPaginator().SetResultPerPage(300).Paginate(c, query)
				return query
			}).
			Order("id DESC").
			Preload("Room").
			Preload("User").
			Preload("ToUser").
			Find(&data.Messages).Error; err != nil {
			logrus.Error(err)
		}
	}

	if roomKey != "" {
		if err := data.Messages.DecryptAll(roomKey); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.Render(http.StatusOK, "chat-archive", data)
}

func ChatDeleteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data chatDeleteData
	roomName := c.Param("roomName")
	room, err := db.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if !room.IsRoomOwner(authUser.ID) {
		return c.Redirect(http.StatusFound, "/")
	}
	data.Room = room

	if c.Request().Method == http.MethodPost {
		if room.IsProtected() {
			hutils.DeleteRoomCookie(c, int64(room.ID))
		}
		db.DeleteChatRoomByID(room.ID)
		return c.Redirect(http.StatusFound, "/chat")
	}

	return c.Render(http.StatusOK, "chat-delete", data)
}

func RoomChatSettingsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data roomChatSettingsData
	roomName := c.Param("roomName")
	room, err := db.GetChatRoomByName(roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if !room.IsRoomOwner(authUser.ID) {
		return c.Redirect(http.StatusFound, "/")
	}
	data.Room = room

	if c.Request().Method == http.MethodPost {
		return c.Redirect(http.StatusFound, "/chat")
	}

	return c.Render(http.StatusOK, "chat-room-settings", data)
}

func ChatCreateRoomHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data chatCreateRoomData
	data.CaptchaID, data.CaptchaImg = captcha.New()
	data.IsEphemeral = true
	if c.Request().Method == http.MethodPost {
		data.RoomName = c.Request().PostFormValue("room_name")
		data.Password = c.Request().PostFormValue("password")
		data.IsListed = utils.DoParseBool(c.Request().PostFormValue("is_listed"))
		data.IsEphemeral = utils.DoParseBool(c.Request().PostFormValue("is_ephemeral"))
		if !govalidator.Matches(data.RoomName, "^[a-zA-Z0-9_]{3,50}$") {
			data.ErrorRoomName = "invalid room name"
			return c.Render(http.StatusOK, "chat-create-room", data)
		}
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, "chat-create-room", data)
		}
		passwordHash := ""
		if data.Password != "" {
			passwordHash = database.GetRoomPasswordHash(data.Password)
		}
		if _, err := db.CreateRoom(data.RoomName, passwordHash, authUser.ID, data.IsListed); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "chat-create-room", data)
		}
		return c.Redirect(http.StatusFound, "/chat/"+data.RoomName)
	}
	return c.Render(http.StatusOK, "chat-create-room", data)
}

func ChatCodeHandler(c echo.Context) error {
	messageUUID := c.Param("messageUUID")
	idx, err := strconv.Atoi(c.Param("idx"))
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	msg, err := db.GetChatMessageByUUID(messageUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	if !dutils.VerifyMsgAuth(db, &msg, authUser.ID, authUser.IsModerator()) {
		return c.Redirect(http.StatusFound, "/")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(msg.Message))
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	n := doc.Find("pre").Eq(idx)
	if n == nil {
		return c.Redirect(http.StatusFound, "/")
	}

	var data chatCodepData
	data.Code, err = n.Html()
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	return c.Render(http.StatusOK, "chat-code", data)
}

func ChatHelpHandler(c echo.Context) error {
	var data chatHelpData
	return c.Render(http.StatusOK, "chat-help", data)
}
