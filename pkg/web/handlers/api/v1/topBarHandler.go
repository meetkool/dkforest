package v1

import (
	"dkforest/pkg/clockwork"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors"
	"dkforest/pkg/web/handlers/interceptors/command"
	"dkforest/pkg/web/handlers/streamModals"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func getDataMessagePrefix(db *database.DkfDB, c echo.Context, roomKey string, room database.ChatRoom, authUser *database.User) (out string, err error) {
	pm := c.QueryParam(command.RedirectPmQP)
	edit := c.QueryParam(command.RedirectEditQP)
	group := c.QueryParam(command.RedirectGroupQP)
	mod := c.QueryParam(command.RedirectModQP)
	hbm := c.QueryParam(command.RedirectHbmQP)
	tag := c.QueryParam(command.RedirectTagQP)
	htag := c.QueryParam(command.RedirectHTagQP)
	mtag := c.QueryParam(command.RedirectMTagQP)
	quote := c.QueryParam(command.RedirectQuoteQP)

	if pm != "" {
		out = "/pm " + pm + " "
	} else if hbm != "" {
		out = "/hbm "
	} else if mod != "" {
		out = "/m "
	} else if group != "" {
		out = "/g " + group + " "
	} else if tag != "" {
		out = "@" + tag + " "
	} else if htag != "" {
		out = "/hbm @" + htag + " "
	} else if mtag != "" {
		out = "/m @" + mtag + " "
	} else if edit != "" {
		out, err = handleGetEdit(db, edit, roomKey, room, authUser)
		if err != nil {
			return
		}
	} else if quote != "" {
		out, err = handleGetQuote(db, quote, roomKey, room, authUser)
		if err != nil {
			return
		}
	}
	return
}

func buildCommandsList(authUser *database.User, room database.ChatRoom) (commandsList []string) {
	if !authUser.AutocompleteCommandsEnabled {
		return
	}
	commandsList = append(commandsList, "/pm ")
	commandsList = append(commandsList, "/pmw ")
	commandsList = append(commandsList, "/pmb ")
	if authUser.IsModerator() {
		commandsList = append(commandsList, "/m ")
	}
	commandsList = append(commandsList, "/me ")
	commandsList = append(commandsList, "/e ")
	commandsList = append(commandsList, "/chess ")
	commandsList = append(commandsList, "/ignore ")
	commandsList = append(commandsList, "/unignore ")
	commandsList = append(commandsList, "/inbox ")
	commandsList = append(commandsList, "/toggle-autocomplete")
	commandsList = append(commandsList, "/d")
	commandsList = append(commandsList, "/hide")
	commandsList = append(commandsList, "/unhide")
	commandsList = append(commandsList, "/pmwhitelist")
	commandsList = append(commandsList, "/setpmmode whitelist")
	commandsList = append(commandsList, "/setpmmode standard")
	commandsList = append(commandsList, "/g ")
	commandsList = append(commandsList, "/subscribe")
	commandsList = append(commandsList, "/unsubscribe")
	commandsList = append(commandsList, "/p ")
	commandsList = append(commandsList, "/token")
	commandsList = append(commandsList, "/md5 ")
	commandsList = append(commandsList, "/sha1 ")
	commandsList = append(commandsList, "/sha256 ")
	commandsList = append(commandsList, "/sha512 ")
	commandsList = append(commandsList, "/dice")
	commandsList = append(commandsList, "/choice ")
	if authUser.CanSeeHB() {
		commandsList = append(commandsList, "/hbm") // CanSeeHB
	}
	// Private room
	if room.IsOwned() {
		commandsList = append(commandsList, "/mode")
		commandsList = append(commandsList, "/wl")
	}
	// Private room owner
	if room.IsRoomOwner(authUser.ID) {
		commandsList = append(commandsList, "/addgroup")
		commandsList = append(commandsList, "/rmgroup")
		commandsList = append(commandsList, "/glock")
		commandsList = append(commandsList, "/gunlock")
		commandsList = append(commandsList, "/gusers")
		commandsList = append(commandsList, "/groups")
		commandsList = append(commandsList, "/gadduser")
		commandsList = append(commandsList, "/grmuser")
		commandsList = append(commandsList, "/mode user-whitelist")
		commandsList = append(commandsList, "/mode standard")
		commandsList = append(commandsList, "/wl groupName")
	}
	// Moderators
	if authUser.IsModerator() {
		commandsList = append(commandsList, "/moderators")
		commandsList = append(commandsList, "/kick ")
		commandsList = append(commandsList, "/unkick ")
		commandsList = append(commandsList, "/logout ")
		commandsList = append(commandsList, "/captcha ")
		commandsList = append(commandsList, "/rtuto ")
		commandsList = append(commandsList, "/hellban ")
		commandsList = append(commandsList, "/unhellban ")
	}
	return commandsList
}

func ChatTopBarHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data chatTopBarData
	data.RoomName = c.Param("roomName")

	redirectPmUsernameQP := command.RedirectPmUsernameQP
	redirectMultilineQP := command.RedirectMultilineQP
	queryParams := c.QueryParams()
	pmUsername := c.QueryParam(redirectPmUsernameQP)
	origMl := utils.DoParseBool(c.QueryParam(redirectMultilineQP))
	data.QueryParams = queryParams.Encode()
	queryParams.Set(redirectMultilineQP, "1")
	data.QueryParamsMl = queryParams.Encode()
	queryParams.Del(redirectMultilineQP)
	data.QueryParamsNml = queryParams.Encode()

	redirectQP := url.Values{}
	if authUser.CanUseMultiline {
		data.Multiline = origMl
		if data.Multiline {
			redirectQP.Set(redirectMultilineQP, "1")
		}
	}
	if pmUsername != "" {
		redirectQP.Set(redirectPmUsernameQP, pmUsername)
	}

	room, roomKey, err := dutils.GetRoomAndKey(db, c, data.RoomName)
	if err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	// If the tutorial is not completed, just render the chat top-bar, no matter what.
	if (room.IsOfficialRoom() || (room.IsListed && !room.IsProtected())) && !authUser.TutorialCompleted() {
		return c.Render(http.StatusOK, "chat-top-bar", data)
	}

	data.Message, err = getDataMessagePrefix(db, c, roomKey, room, authUser)
	if err != nil {
		return c.Redirect(http.StatusFound, "/api/v1/chat/top-bar/"+room.Name)
	}

	data.CommandsList = buildCommandsList(authUser, room)

	// GET requests stops here
	if c.Request().Method != http.MethodPost {
		return c.Render(http.StatusOK, "chat-top-bar", data)
	}

	// ------------------------------------------------------------------------

	if room.Name == config.AnnouncementsRoomName && authUser.ID != config.RootAdminID {
		data.Error = "read only room"
		return c.Render(http.StatusOK, "chat-top-bar", data)
	}

	if c.Request().ContentLength > config.MaxUserFileUploadSize {
		data.Error = fmt.Sprintf("The maximum file size is %s", humanize.Bytes(config.MaxUserFileUploadSize))
		return c.Render(http.StatusOK, "chat-top-bar", data)
	}

	origMessage := strings.TrimSpace(c.Request().PostFormValue("message"))

	cmd := command.NewCommand(c, origMessage, room, roomKey)
	cmd.RedirectQP = redirectQP

	interceptorsArr := []interceptors.Interceptor{
		interceptors.SnippetInterceptor{},
		interceptors.SpamInterceptor{},
		interceptors.BattleshipInstance,
		interceptors.WWInstance,
		interceptors.BangInterceptor{},
		interceptors.UploadInterceptor{},
		interceptors.SlashInterceptor{},
		streamModals.CodeModal{},
		streamModals.PurgeModal{},
		interceptors.MsgInterceptor{},
	}
	for _, interceptor := range interceptorsArr {
		interceptor.InterceptMsg(cmd)
		data.Message = cmd.DataMessage
		if cmd.Err != nil {
			return handleCmdError(cmd.Err, c, data, cmd.RedirectURL(), cmd.OrigMessage)
		}
	}

	return c.Redirect(http.StatusFound, cmd.RedirectURL())
}

func handleCmdError(err error, ctx echo.Context, data chatTopBarData, redirectURL, origMessage string) error {
	if err == command.ErrRedirect {
		return ctx.Redirect(http.StatusFound, redirectURL)
	} else if err == command.ErrStop {
		return ctx.Render(http.StatusOK, "chat-top-bar", data)
	} else if serr, ok := err.(*command.ErrSuccess); ok {
		data.Success = serr.Error()
		return ctx.Render(http.StatusOK, "chat-top-bar", data)
	}
	data.Message = origMessage
	data.Error = err.Error()
	return ctx.Render(http.StatusOK, "chat-top-bar", data)
}

func handleGetQuote(db *database.DkfDB, msgUUID, roomKey string, room database.ChatRoom, authUser *database.User) (dataMessage string, err error) {
	quoted, err := db.GetRoomChatMessageByUUID(room.ID, msgUUID)
	if err != nil {
		return
	}

	// Build prefix for /m | /pm | /g | /hbm
	prefix := ""
	if quoted.IsPm() {
		toUsername := utils.Ternary(quoted.OwnMessage(authUser.ID), quoted.ToUser.Username, quoted.User.Username)
		prefix = fmt.Sprintf(`/pm %s `, toUsername)
	} else if quoted.GroupID != nil {
		prefix = fmt.Sprintf(`/g %s `, quoted.Group.Name)
	} else if quoted.Moderators {
		prefix = fmt.Sprintf(`/m `)
	} else if (quoted.IsHellbanned || quoted.User.IsHellbanned) && authUser.IsModerator() {
		prefix = fmt.Sprintf(`/hbm `)
	}

	// Append the actual quoted text
	dataMessage = prefix + dutils.GetQuoteTxt(db, roomKey, quoted) + " "
	return
}

func handleGetEdit(db *database.DkfDB, hourMinSec, roomKey string, room database.ChatRoom, authUser *database.User) (dataMessage string, err error) {
	if dt, err := utils.ParsePrevDatetimeAt(hourMinSec, clockwork.NewRealClock()); err == nil {
		if time.Since(dt) <= config.EditMessageTimeLimit {
			if msg, err := db.GetRoomChatMessageByDate(room.ID, authUser.ID, dt.UTC()); err == nil {
				decrypted, err := msg.GetRawMessage(roomKey)
				if err != nil {
					return "", err
				}
				dataMessage = "/e " + hourMinSec + " " + decrypted
			}
		}
	}
	return dataMessage, nil
}
