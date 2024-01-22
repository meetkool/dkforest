package v1

import (
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/labstack/echo"
	"net/http"
)

// Handle the forms/actions in the chat controls, bottom of the page, iframe.
// Such as toggle "@"/"PM"/"Ignored"/"afk"...

func ChatControlsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	var data chatControlsData
	data.RoomName = c.Param("roomName")
	data.IsStream = utils.DoParseBool(c.Param("isStream"))
	data.TogglePms = utils.DoParseInt64(c.QueryParam("pmonly"))
	data.ToggleMentions = utils.DoParseBool(c.QueryParam("mentionsOnly"))
	v := c.QueryParams()
	if data.TogglePms != 0 {
		v.Set("pmonly", utils.FormatInt64(data.TogglePms))
	}
	if data.ToggleMentions {
		v.Set("mentionsOnly", "1")
	}
	data.ChatQueryParams = "?" + v.Encode()

	if c.Request().Method == http.MethodPost {
		return handlePost(db, c, data, authUser)
	}

	return c.Render(http.StatusOK, "chat-controls", data)
}

func handlePost(db *database.DkfDB, c echo.Context, data chatControlsData, authUser *database.User) error {
	formName := c.Request().PostFormValue("formName")
	switch formName {
	case "toggle-hb":
		return handleToggleHBPost(db, c, authUser)
	case "toggle-m":
		return handleToggleMPost(db, c, authUser)
	case "toggle-ignored":
		return handleToggleIgnoredPost(db, c, authUser)
	case "afk":
		return handleAfkPost(db, c, authUser)
	case "update-read-marker":
		if room, err := db.GetChatRoomByName(data.RoomName); err == nil {
			return handleUpdateReadMarkerPost(db, c, room, authUser)
		}
	}
	return hutils.RedirectReferer(c)
}

func handleToggleHBPost(db *database.DkfDB, c echo.Context, authUser *database.User) error {
	if authUser.CanSeeHB() {
		authUser.ToggleDisplayHellbanned(db)
		database.MsgPubSub.Pub("refresh_"+string(authUser.Username), database.ChatMessageType{})
	}
	return hutils.RedirectReferer(c)
}

func handleToggleMPost(db *database.DkfDB, c echo.Context, authUser *database.User) error {
	if authUser.IsModerator() {
		authUser.ToggleDisplayModerators(db)
		database.MsgPubSub.Pub("refresh_"+string(authUser.Username), database.ChatMessageType{})
	}
	return hutils.RedirectReferer(c)
}

func handleToggleIgnoredPost(db *database.DkfDB, c echo.Context, authUser *database.User) error {
	authUser.ToggleDisplayIgnored(db)
	database.MsgPubSub.Pub("refresh_"+string(authUser.Username), database.ChatMessageType{})
	return hutils.RedirectReferer(c)
}

func handleAfkPost(db *database.DkfDB, c echo.Context, authUser *database.User) error {
	authUser.ToggleAFK(db)
	return hutils.RedirectReferer(c)
}

func handleUpdateReadMarkerPost(db *database.DkfDB, c echo.Context, room database.ChatRoom, authUser *database.User) error {
	db.UpdateChatReadMarker(authUser.ID, room.ID)
	return hutils.RedirectReferer(c)
}
