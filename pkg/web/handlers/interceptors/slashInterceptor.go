package interceptors

import (
	"dkforest/pkg/clockwork"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/levenshtein"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
	"dkforest/pkg/web/handlers/poker"
	"dkforest/pkg/web/handlers/streamModals"
	"errors"
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/asaskevich/govalidator"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SlashInterceptor handle all forward slash commands.
//
// If by the end of this function, the c.err is set, it will trigger
// different behavior according to the type of error it holds.
// if c.err is set to ErrRedirect, the chat-bar iframe will refresh completely.
// if c.err is set to ErrStop, no further processing of the user input will be done,
//
//	and the chat iframe will be rendered instead of redirected.
//	This is useful to keep a prefix in the text box (eg: /pm user )
//
// if c.err is set to an instance of ErrSuccess,
//
//	a green message will appear beside the text box.
//
// otherwise if c.err is set to a different error,
//
//	text box is retested to original message,
//	and a red message will appear beside the text box.
type SlashInterceptor struct{}

type CmdHandler func(c *command.Command) (handled bool)

var userCmdsMap = map[string]CmdHandler{
	"/i":                   handleIgnoreCmd,
	"/ignore":              handleIgnoreCmd,
	"/ui":                  handleUnIgnoreCmd,
	"/unignore":            handleUnIgnoreCmd,
	"/toggle-autocomplete": handleToggleAutocomplete,
	"/tuto":                handleTutorialCmd,
	"/d":                   handleDeleteMsgCmd,
	"/hide":                handleHideMsgCmd,
	"/unhide":              handleUnHideMsgCmd,
	"/pmwhitelist":         handleListPmWhitelistCmd,
	"/setpmmode":           handleSetPmModeCmd,
	"/pmb":                 handleTogglePmBlacklistedUser,
	"/pmw":                 handleTogglePmWhitelistedUser,
	"/g":                   handleGroupChatCmd,
	"/me":                  handleMeCmd,
	"/e":                   handleEditCmd,
	"/pm":                  handlePMCmd,
	"/subscribe":           handleSubscribeCmd,
	"/unsubscribe":         handleUnsubscribeCmd,
	"/p":                   handleProfileCmd,
	"/inbox":               handleInboxCmd,
	"/chess":               handleChessCmd,
	"/hbm":                 handleHbmCmd,
	"/hbmt":                handleHbmtCmd,
	"/token":               handleTokenCmd,
	"/md5":                 handleMd5Cmd,
	"/sha1":                handleSha1Cmd,
	"/sha256":              handleSha256Cmd,
	"/sha512":              handleSha512Cmd,
	"/dice":                handleDiceCmd,
	"/rand":                handleRandCmd,
	"/choice":              handleChoiceCmd,
	"/memes":               handleListMemes,
	"/success":             handleSuccessCmd,
	"/afk":                 handleAfkCmd,
	"/date":                handleDateCmd,
	"/r":                   handleUpdateReadMarkerCmd,
	"/code":                handleCodeCmd,
	"/locate":              handleLocateCmd,
	"/error":               handleErrorCmd,
	"/chips":               handleChipsBalanceCmd,
	"/chips-reset":         handleChipsResetCmd,
	"/wizz":                handleWizzCmd,
	"/itr":                 handleInThisRoomCmd,
	"/check":               handleCheckCmd,
	"/call":                handleCallCmd,
	"/fold":                handleFoldCmd,
	"/raise":               handleRaiseCmd,
	"/allin":               handleAllInCmd,
	"/bet":                 handleBetCmd,
	"/deal":                handleDealCmd,
	"/dist":                handleDistCmd,
	//"/chips-send":          handleChipsSendCmd,
}

var privateRoomCmdsMap = map[string]CmdHandler{
	"/mode":      handleGetModeCmd,
	"/wl":        handleWhitelistCmd,
	"/whitelist": handleWhitelistCmd,
}

var privateRoomOwnerCmdsMap = map[string]CmdHandler{
	"/addgroup":  handleAddGroupCmd,
	"/rmgroup":   handleRmGroupCmd,
	"/glock":     handleLockGroupCmd,
	"/gunlock":   handleUnlockGroupCmd,
	"/gusers":    handleGroupUsersCmd,
	"/groups":    handleListGroupsCmd,
	"/gadduser":  handleGroupAddUserCmd,
	"/grmuser":   handleGroupRmUserCmd,
	"/mode":      handleSetModeCmd,
	"/ro":        handleToggleReadOnlyCmd,
	"/wl":        handleGetRoomWhitelistCmd,
	"/whitelist": handleGetRoomWhitelistCmd,
}

var moderatorCmdsMap = map[string]CmdHandler{
	"/m":          handleModeratorGroupCmd,
	"/n":          handleModeratorGroupCmd,
	"/moderators": handleListModeratorsCmd,
	"/mods":       handleListModeratorsCmd,
	"/k":          handleKickCmd,
	"/kick":       handleKickCmd,
	"/kk":         handleKickKeepCmd,
	"/ks":         handleKickSilentCmd,
	"/kks":        handleKickKeepSilentCmd,
	"/uk":         handleUnkickCmd,
	"/unkick":     handleUnkickCmd,
	"/logout":     handleLogoutCmd,
	"/captcha":    handleForceCaptchaCmd,
	"/rtuto":      handleResetTutorialCmd,
	"/hb":         handleHellbanCmd,
	"/hellban":    handleHellbanCmd,
	"/unhellban":  handleUnhellbanCmd,
	"/uhb":        handleUnhellbanCmd,
}

var adminCmdsMap = map[string]CmdHandler{
	"/sys":     handleSystemCmd,
	"/system":  handleSystemCmd,
	"/seturl":  handleSetChatRoomExternalLink,
	"/purge":   handlePurge,
	"/rename":  handleRename,
	"/meme":    handleNewMeme,
	"/memerm":  handleRemoveMeme,
	"/refresh": handleRefreshCmd,
	"/chips":   handleChipsCmd,
	"/close":   handleCloseCmd,
	"/closem":  handleCloseMenuCmd,
}

func (i SlashInterceptor) InterceptMsg(c *command.Command) {
	if !strings.HasPrefix(c.Message, "/") {
		return
	}
	handled := handleUserCmd(c) ||
		handlePrivateRoomCmd(c) ||
		handlePrivateRoomOwnerCmd(c) ||
		handleModeratorCmd(c) ||
		handleAdminCmd(c)
	if !handled {
		c.Err = errors.New("invalid slash command")
	}
}

func handleUserCmd(c *command.Command) (handled bool) {
	cmd := strings.Fields(c.Message)[0]
	if cmdFn, found := userCmdsMap[cmd]; found {
		return cmdFn(c)
	}
	return
}

func handlePrivateRoomCmd(c *command.Command) (handled bool) {
	cmd := strings.Fields(c.Message)[0]
	if cmdFn, found := privateRoomCmdsMap[cmd]; found {
		return cmdFn(c)
	}
	return
}

func handlePrivateRoomOwnerCmd(c *command.Command) (handled bool) {
	if c.Room.IsRoomOwner(c.AuthUser.ID) || c.AuthUser.IsAdmin {
		cmd := strings.Fields(c.Message)[0]
		if cmdFn, found := privateRoomOwnerCmdsMap[cmd]; found {
			return cmdFn(c)
		}
	}
	return false
}

func handleModeratorCmd(c *command.Command) (handled bool) {
	if c.AuthUser.IsModerator() {
		cmd := strings.Fields(c.Message)[0]
		if cmdFn, found := moderatorCmdsMap[cmd]; found {
			return cmdFn(c)
		}
	}
	return false
}

func handleAdminCmd(c *command.Command) (handled bool) {
	if c.AuthUser.IsAdmin {
		cmd := strings.Fields(c.Message)[0]
		if cmdFn, found := adminCmdsMap[cmd]; found {
			return cmdFn(c)
		}
	}
	return false
}

func handleModeratorGroupCmd(c *command.Command) (handled bool) {
	if strings.HasPrefix(c.Message, "/m ") || strings.HasPrefix(c.Message, "/n ") {
		if strings.HasPrefix(c.Message, "/n ") {
			c.Message = strings.Replace(c.Message, "/n ", "/m ", 1)
		}
		c.Message = strings.TrimPrefix(c.Message, "/m ")
		c.RedirectQP.Set(command.RedirectModQP, "1")
		c.ModMsg = true
		if handleMeCmd(c) {
			return true
		} else if handleCodeCmd(c) {
			return true
		}
		return true
	}
	return false
}

func handleListModeratorsCmd(c *command.Command) (handled bool) {
	if c.Message == "/moderators" || c.Message == "/mods" {
		mods, err := c.DB.GetModeratorsUsers()
		if err != nil {
			c.Err = err
			return true
		}
		msg := "Moderators:\n"
		if len(mods) > 0 {
			msg += "\n"
			for _, mod := range mods {
				msg += mod.Username.AtStr() + "\n"
			}
		} else {
			msg += "no moderators"
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleInThisRoomCmd(c *command.Command) (handled bool) {
	if c.Message == "/itr" {
		membersInRoom, _ := managers.ActiveUsers.GetRoomUsers(c.Room, managers.GetUserIgnoreSet(c.DB, c.AuthUser))

		msg := "In this room:"
		if len(membersInRoom) > 0 {
			msg += " "
			for _, mod := range membersInRoom {
				msg += mod.Username.AtStr() + " "
			}
		} else {
			msg += "no one"
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleKickCmd(c *command.Command) (handled bool) {
	if m := kickRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		if err := kickCmd(c, username, true, false); err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	}
	return
}

// Kick a user but keep the messages
func handleKickKeepCmd(c *command.Command) (handled bool) {
	if m := kickKeepRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		if err := kickCmd(c, username, false, false); err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	}
	return
}

// Kick a user, no system message in chat
func handleKickSilentCmd(c *command.Command) (handled bool) {
	if m := kickSilentRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		if err := kickCmd(c, username, true, true); err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	}
	return
}

// Kick a user, keep the messages, no system message in chat
func handleKickKeepSilentCmd(c *command.Command) (handled bool) {
	if m := kickKeepSilentRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		if err := kickCmd(c, username, false, true); err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func kickCmd(c *command.Command, username database.Username, purge, silent bool) error {
	user, err := c.DB.GetUserByUsername(username)
	if err != nil {
		return ErrUsernameNotFound
	}
	return dutils.Kick(c.DB, user, *c.AuthUser, purge, silent)
}

var ErrUsernameNotFound = errors.New("username not found")
var ErrUnauthorized = errors.New("unauthorized")

func handleUnkickCmd(c *command.Command) (handled bool) {
	if m := unkickRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = ErrUsernameNotFound
			return true
		}
		if user.Verified {
			c.Err = errors.New("user already not kicked")
			return true
		}
		c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("unkick %s #%d", user.Username, user.ID))
		user.SetVerified(c.DB, true)

		// Display unkick message
		c.DB.CreateUnkickMsg(user, *c.AuthUser)

		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleForceCaptchaCmd(c *command.Command) (handled bool) {
	if m := forceCaptchaRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = ErrUsernameNotFound
			return true
		}
		if c.AuthUser.IsAdmin || !user.IsModerator() || c.AuthUser.Username == username {
			c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("force captcha %s #%d", user.Username, user.ID))
			user.SetCaptchaRequired(c.DB, true)
			database.MsgPubSub.Pub("refresh_"+string(user.Username), database.ChatMessageType{Typ: database.ForceRefresh})
		}
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleLogoutCmd(c *command.Command) (handled bool) {
	if m := logoutRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = ErrUsernameNotFound
			return true
		}
		if !c.AuthUser.IsAdmin && user.Vetted {
			c.Err = ErrUnauthorized
			return true
		}
		if c.AuthUser.IsAdmin || !user.IsModerator() {
			c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("logout %s #%d", user.Username, user.ID))

			_ = c.DB.DeleteUserSessions(user.ID)

			// Remove user from the user cache
			managers.ActiveUsers.RemoveUser(user.ID)
			database.MsgPubSub.Pub("refresh_"+string(user.Username), database.ChatMessageType{Typ: database.ForceRefresh})
		}

		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleResetTutorialCmd(c *command.Command) (handled bool) {
	if m := rtutoRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = ErrUsernameNotFound
			return true
		}
		if !c.AuthUser.IsAdmin && user.Vetted {
			c.Err = ErrUnauthorized
			return true
		}
		if c.AuthUser.IsAdmin || !user.IsModerator() {
			c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("rtuto %s #%d", user.Username, user.ID))
			user.ResetTutorial(c.DB)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleHellbanCmd(c *command.Command) (handled bool) {
	if m := hellbanRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = ErrUsernameNotFound
			return true
		}
		if !c.AuthUser.IsAdmin && (user.Vetted || user.IsModerator()) {
			c.Err = ErrUnauthorized
			return true
		}
		c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("hellban %s #%d", user.Username, user.ID))
		user.HellBan(c.DB)
		managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(&user))

		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleUnhellbanCmd(c *command.Command) (handled bool) {
	if m := unhellbanRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = ErrUsernameNotFound
			return true
		}
		if !c.AuthUser.IsAdmin && (user.Vetted || user.IsModerator()) {
			c.Err = ErrUnauthorized
			return true
		}
		c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("unhellban %s #%d", user.Username, user.ID))
		user.UnHellBan(c.DB)
		managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(&user))

		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleHbmCmd(c *command.Command) (handled bool) {
	if !c.AuthUser.CanSeeHB() {
		return
	}
	if strings.HasPrefix(c.Message, "/hbm ") {
		c.Message = strings.TrimPrefix(c.Message, "/hbm ")
		c.HellbanMsg = true
		c.RedirectQP.Set(command.RedirectHbmQP, "1")
		return true
	}
	return
}

func handleHbmtCmd(c *command.Command) (handled bool) {
	if !c.AuthUser.CanSeeHB() {
		return
	}
	if m := hbmtRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		date := m[1]
		if dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock()); err == nil {
			if msg, err := c.DB.GetRoomChatMessageByDate(c.Room.ID, c.AuthUser.ID, dt.UTC()); err == nil {
				msg.IsHellbanned = !msg.IsHellbanned
				msg.DoSave(c.DB)
			} else {
				c.Err = errors.New("no message found at this timestamp")
				return true
			}
		}
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleDiceCmd(c *command.Command) (handled bool) {
	if strings.HasPrefix(c.Message, "/dice") {
		dice := utils.RandInt(1, 6)
		raw := fmt.Sprintf(`rolling dice for @%s ... "%d"`, c.AuthUser.Username, dice)
		msg := fmt.Sprintf(`rolling dice for @%s ... "<span style="color: white;">%d</span>"`, c.AuthUser.Username, dice)
		msg, _ = dutils.ColorifyTaggedUsers(msg, c.DB.GetUsersByUsername)
		go func() {
			time.Sleep(time.Second)
			c.ZeroPublicMsg(raw, msg)
		}()
		return true
	}
	return
}

func handleRandCmd(c *command.Command) (handled bool) {
	if strings.HasPrefix(c.Message, "/rand") {
		minV := 1
		maxV := 6
		var dice int
		if m := randRgx.FindStringSubmatch(c.Message); len(m) == 3 {
			var err error
			minV, err = strconv.Atoi(m[1])
			if err != nil {
				c.Err = err
				return true
			}
			maxV, err = strconv.Atoi(m[2])
			if err != nil {
				c.Err = err
				return true
			}
			if maxV <= minV {
				c.Err = errors.New("max must be greater than min")
				return true
			}
		} else if c.Message != "/rand" {
			c.Err = errors.New("invalid /rand command")
			return true
		}
		dice = utils.RandInt(minV, maxV)
		raw := fmt.Sprintf(`rolling dice for @%s ... "%d"`, c.AuthUser.Username, dice)
		msg := fmt.Sprintf(`rolling dice for @%s ... "<span style="color: white;">%d</span>"`, c.AuthUser.Username, dice)
		msg, _ = dutils.ColorifyTaggedUsers(msg, c.DB.GetUsersByUsername)
		go func() {
			time.Sleep(time.Second)
			c.ZeroPublicMsg(raw, msg)
		}()
		return true
	}
	return
}

func handleChoiceCmd(c *command.Command) (handled bool) {
	if strings.HasPrefix(c.Message, "/choice ") {
		tmp := html.EscapeString(strings.TrimPrefix(c.Message, "/choice "))
		words := strings.Fields(tmp)
		answer := utils.RandChoice(words)
		raw := fmt.Sprintf(`@%s choice %s ... "%s"`, c.AuthUser.Username, words, answer)
		msg := fmt.Sprintf(`@%s choice %s ... "<span style="color: white;">%s</span>"`, c.AuthUser.Username, words, answer)
		msg, _ = dutils.ColorifyTaggedUsers(msg, c.DB.GetUsersByUsername)
		go func() {
			time.Sleep(time.Second)
			c.ZeroPublicMsg(raw, msg)
		}()
		c.SkipInboxes = true
		return true
	}
	return
}

func handleTokenCmd(c *command.Command) (handled bool) {
	if c.Message == "/token" {
		c.ZeroMsg(utils.GenerateToken10())
		c.Err = command.ErrRedirect
		return true
	} else if m := tokenRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 32 {
			c.Err = errors.New("value must be [1;32]")
			return true
		}
		n = utils.Clamp(n, 1, 32)
		c.ZeroMsg(utils.GenerateTokenN(n))
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleMd5Cmd(c *command.Command) (handled bool) {
	return handleHasherCmd(c, "/md5 ", utils.MD5)
}

func handleSha1Cmd(c *command.Command) (handled bool) {
	return handleHasherCmd(c, "/sha1 ", utils.Sha1)
}

func handleSha256Cmd(c *command.Command) (handled bool) {
	return handleHasherCmd(c, "/sha256 ", utils.Sha256)
}

func handleSha512Cmd(c *command.Command) (handled bool) {
	return handleHasherCmd(c, "/sha512 ", utils.Sha512)
}

func handleHasherCmd(c *command.Command, prefix string, fn func([]byte) string) (handled bool) {
	if strings.HasPrefix(c.Message, prefix) {
		c.Message = strings.TrimPrefix(c.Message, prefix)
		c.DataMessage = prefix
		c.ZeroMsg(fn([]byte(c.Message)))
		c.Err = command.ErrStop
		return true
	}
	return
}

func handleRmGroupCmd(c *command.Command) (handled bool) {
	if m := rmGroupRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		groupName := m[1]
		if err := c.DB.DeleteChatRoomGroup(c.Room.ID, groupName); err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleLockGroupCmd(c *command.Command) (handled bool) {
	if m := lockGroupRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		groupName := m[1]
		group, err := c.DB.GetRoomGroupByName(c.Room.ID, groupName)
		if err != nil {
			c.Err = err
			return true
		}
		group.Locked = true
		group.DoSave(c.DB)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleUnlockGroupCmd(c *command.Command) (handled bool) {
	if m := unlockGroupRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		groupName := m[1]
		group, err := c.DB.GetRoomGroupByName(c.Room.ID, groupName)
		if err != nil {
			c.Err = err
			return true
		}
		group.Locked = false
		group.DoSave(c.DB)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleGroupUsersCmd(c *command.Command) (handled bool) {
	if m := groupUsersRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		groupName := m[1]
		group, err := c.DB.GetRoomGroupByName(c.Room.ID, groupName)
		if err != nil {
			c.Err = err
			return true
		}
		users, err := c.DB.GetRoomGroupUsers(c.Room.ID, group.ID)
		sort.Slice(users, func(i, j int) bool {
			return users[i].User.Username < users[j].User.Username
		})
		msg := ""
		if len(users) > 0 {
			msg += "\n"
			for _, user := range users {
				msg += user.User.Username.AtStr() + "\n"
			}
		} else {
			msg += "no user in th group: " + groupName
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleListGroupsCmd(c *command.Command) (handled bool) {
	if c.Message == "/groups" {
		groups, err := c.DB.GetRoomGroups(c.Room.ID)
		if err != nil {
			c.Err = err
			return true
		}
		msg := ""
		if len(groups) > 0 {
			msg += "\n"
			for _, group := range groups {
				msg += group.Name + " (" + group.Color + ")\n"
			}
		} else {
			msg += "no groups"
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleGroupAddUserCmd(c *command.Command) (handled bool) {
	if m := groupAddUserRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		groupName := m[1]
		username := database.Username(m[2])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = err
			return true
		}
		group, err := c.DB.GetRoomGroupByName(c.Room.ID, groupName)
		if err != nil {
			c.Err = err
			return true
		}
		_, err = c.DB.AddUserToRoomGroup(c.Room.ID, group.ID, user.ID)
		if err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	} else if strings.HasPrefix(c.Message, "/gadduser ") {
		c.Err = errors.New("invalid /gadduser command")
	}
	return false
}

func handleGroupRmUserCmd(c *command.Command) (handled bool) {
	if m := groupRmUserRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		groupName := m[1]
		username := database.Username(m[2])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = err
			return true
		}
		group, err := c.DB.GetRoomGroupByName(c.Room.ID, groupName)
		if err != nil {
			c.Err = err
			return true
		}
		err = c.DB.RmUserFromRoomGroup(c.Room.ID, group.ID, user.ID)
		if err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	} else if strings.HasPrefix(c.Message, "/grmuser ") {
		c.Err = errors.New("invalid /grmuser command")
	}
	return false
}

func handleSetModeCmd(c *command.Command) (handled bool) {
	if c.Message == "/mode user-whitelist" {
		c.Room.Mode = database.UserWhitelistRoomMode
		c.Room.DoSave(c.DB)
		msg := `room mode set to "user whitelist"`
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true

	} else if c.Message == "/mode standard" {
		c.Room.Mode = database.NormalRoomMode
		c.Room.DoSave(c.DB)
		msg := `room mode set to "standard"`
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleGetRoomWhitelistCmd(c *command.Command) (handled bool) {
	if m := whitelistUserRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		var msg string
		if err != nil {
			msg = fmt.Sprintf(`username "%s" not found`, username)
		} else {
			if _, err := c.DB.WhitelistUser(c.Room.ID, user.ID); err != nil {
				if err := c.DB.DeWhitelistUser(c.Room.ID, user.ID); err != nil {
					msg = fmt.Sprintf("failed to toggle @%s in whitelist", user.Username)
				} else {
					msg = fmt.Sprintf("@%s removed from whitelist", user.Username)
				}
			} else {
				msg = fmt.Sprintf("@%s added to whitelist", user.Username)
			}
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleToggleReadOnlyCmd(c *command.Command) (handled bool) {
	if c.Message == "/ro" {
		c.Room.ReadOnly = !c.Room.ReadOnly
		c.Room.DoSave(c.DB)
		if c.Room.ReadOnly {
			c.Err = command.NewErrSuccess("room is now read-only")
		} else {
			c.Err = command.NewErrSuccess("room is no longer read-only")
		}
		return true
	}
	return
}

func handleAddGroupCmd(c *command.Command) (handled bool) {
	if m := addGroupRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		name := m[1]
		_, err := c.DB.CreateChatRoomGroup(c.Room.ID, name, "#fff")
		if err != nil {
			c.Err = err
			return true
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleWhitelistCmd(c *command.Command) (handled bool) {
	if c.Message == "/whitelist" || c.Message == "/wl" {
		usernames := make([]string, 0)
		whitelistedUsers, _ := c.DB.GetWhitelistedUsers(c.Room.ID)
		if c.Room.OwnerUserID != nil {
			owner, _ := c.DB.GetUserByID(*c.Room.OwnerUserID)
			usernames = append(usernames, owner.Username.AtStr())
		}
		for _, whitelistedUser := range whitelistedUsers {
			usernames = append(usernames, whitelistedUser.User.Username.AtStr())
		}
		sort.Slice(usernames, func(i, j int) bool { return usernames[i] < usernames[j] })
		var msg string
		if len(whitelistedUsers) > 0 {
			msg = "whitelisted users: " + strings.Join(usernames, ", ")
		} else {
			msg = "no whitelisted user"
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleGetModeCmd(c *command.Command) (handled bool) {
	if c.Message == "/mode" {
		var msg string
		if c.Room.Mode == database.NormalRoomMode {
			msg = `room is in "standard" mode`
		} else if c.Room.Mode == database.UserWhitelistRoomMode {
			msg = `room is in "user whitelist" mode`
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleMeCmd(c *command.Command) (handled bool) {
	if c.Message == "/me " {
		c.Err = errors.New("invalid /me command")
		return true
	}
	if strings.HasPrefix(c.Message, "/me ") {
		return true
	}
	return
}

func handleEditCmd(c *command.Command) (handled bool) {
	if m := editRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		date := m[1]
		newMsg := m[2]
		dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock())
		if err != nil {
			c.Err = errors.New("failed to parse timestamp")
			return true
		}
		if time.Since(dt) > config.EditMessageTimeLimit {
			c.Err = errors.New("message too old to be edited")
			return true
		}
		msg, err := c.DB.GetRoomChatMessageByDate(c.Room.ID, c.AuthUser.ID, dt.UTC())
		if err != nil {
			c.Err = fmt.Errorf("failed to get message at timestamp %s", date)
			return true
		}
		c.EditMsg = &msg
		c.OrigMessage = newMsg
		c.Message = newMsg

		// If we're editing a message which contains a link to an uploaded file,
		// we need to re-add the link to the html.
		if msg.UploadID != nil {
			if newUpload, err := c.DB.GetUploadByID(*msg.UploadID); err == nil {
				c.Upload = &newUpload
			}
		}

		if pmRgx.MatchString(c.Message) {
			handlePMCmd(c)
		} else if c.AuthUser.IsModerator() && strings.HasPrefix(c.Message, "/m ") {
			handleModeratorGroupCmd(c)
		} else if strings.HasPrefix(c.Message, "/hbm ") {
			handleHbmCmd(c)
		} else if strings.HasPrefix(c.Message, "/g ") {
			handleGroupChatCmd(c)
		} else if strings.HasPrefix(c.Message, "/system ") || strings.HasPrefix(c.Message, "/sys ") {
			handleSystemCmd(c)
		}
		return true

	} else if c.Message == "/e" {
		msg, err := c.DB.GetUserLastChatMessageInRoom(c.AuthUser.ID, c.Room.ID)
		if err != nil {
			return true
		}
		c.RedirectQP.Set(command.RedirectEditQP, msg.CreatedAt.Format("15:04:05"))
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func canUserInboxOther(db *database.DkfDB, user, other database.User) error {
	doesNotMatter := utils.False()
	_, err := dutils.CanUserPmOther(db, user, other, doesNotMatter)
	return err
}

func handlePMCmd(c *command.Command) (handled bool) {
	if m := pmRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		username := database.Username(m[1])
		newMsg := m[2]
		redirectPmQP := command.RedirectPmQP

		// Chat helpers
		if username == config.NullUsername {
			c.RedirectQP.Set(redirectPmQP, config.NullUsername)
			return handlePm0(c, newMsg)
		}

		// Hack to have 1 on 1 chat with the user
		if strings.TrimSpace(newMsg) == "" && c.Upload == nil {
			redirectPmUsernameQP := command.RedirectPmUsernameQP
			newURL := fmt.Sprintf("/api/v1/chat/messages/%s/stream?%s=%s", c.Room.Name, redirectPmUsernameQP, username)
			database.MsgPubSub.Pub("refresh_"+string(c.AuthUser.Username), database.ChatMessageType{Typ: database.Redirect, NewURL: newURL})
			c.RedirectQP.Set(redirectPmUsernameQP, username.String())
			c.Err = command.ErrRedirect
			return true
		}

		if err := c.SetToUser(username); err != nil {
			return true
		}
		c.Message = newMsg
		c.RedirectQP.Set(redirectPmQP, string(c.ToUser.Username))

		if newMsg == "/d" || strings.HasPrefix(newMsg, "/d ") {
			handled = handleDeleteMsgCmd(c)
			if c.Err != nil && !errors.Is(c.Err, command.ErrRedirect) {
				return handled
			}
			c.Err = command.ErrRedirect
			return handled
		}

		if handleCodeCmd(c) {
			return true
		}

		return true
	} else if strings.HasPrefix(c.Message, "/pm ") {
		c.Err = errors.New("invalid /pm command")
		return true
	}
	return false
}

// Handle PMs sent to user 0 (/pm 0 msg)
func handlePm0(c *command.Command, msg string) (handled bool) {
	if msg == "ping" {
		c.ZeroMsg("pong")
		c.Err = command.ErrRedirect
		return true

	} else if msg == "talk" {
		c.ZeroMsg("talking")
		c.Err = command.ErrRedirect
		return true

	} else if msg == "pgp" || msg == "gpg" {
		pkey := c.AuthUser.GPGPublicKey
		if pkey == "" {
			c.Message = "I could not find a public pgp key in your profile."
		} else {
			msg := "This is a sample text"
			if encrypted, err := utils.GeneratePgpEncryptedMessage(pkey, msg); err != nil {
				c.Message = err.Error()
			} else {
				c.Message = strings.Join(strings.Split(encrypted, "\n"), " ")
			}
		}
		c.ZeroProcMsg(c.Message)
		c.Err = command.ErrRedirect
		return true

	} else if pgpMsg, _, _ := dutils.ExtractPGPMessage(msg); pgpMsg != "" {
		decrypted, err := utils.PgpDecryptMessage(config.NullUserPrivateKey, pgpMsg)
		if err != nil {
			c.Message = err.Error()
		} else {
			c.Message = "Decrypted message: " + decrypted
		}
		c.ZeroProcMsg(c.Message)
		c.Err = command.ErrRedirect
		return true

	} else if b, _ := clearsign.Decode([]byte(msg)); b != nil {
		if p, err := packet.Read(b.ArmoredSignature.Body); err == nil {
			if sig, ok := p.(*packet.Signature); ok {
				zero := c.GetZeroUser()
				msg := fmt.Sprintf("<br />"+
					"<table %s>"+
					"<tr><td align=\"right\">Signature made:&nbsp;&nbsp;</td><td><span style=\"color: #82e17f;\">%s</span></td></tr>"+
					"<tr><td align=\"right\">Fingerprint:&nbsp;&nbsp;</td><td><span style=\"color: #82e17f;\">%s</span></td></tr>"+
					"<tr><td align=\"right\">Issuer:&nbsp;&nbsp;</td><td><span style=\"color: #82e17f;\">%s</span></td></tr>"+
					"</table>",
					zero.GenerateChatStyle(),
					sig.CreationTime.Format(time.RFC1123),
					utils.FormatPgPFingerprint(sig.IssuerFingerprint),
					utils.Ternary(sig.SignerUserId != nil, *sig.SignerUserId, "n/a"))
				c.ZeroMsg(msg)
				c.Err = command.ErrRedirect
				return true
			}
		}

	} else if c.Upload != nil {

		// If we sent a clearsign file to @0, the bot will reply with information about the signature
		if c.Upload.FileSize < config.MaxFileSizeBeforeDownload {
			if file, err := c.DB.GetUploadByFileName(c.Upload.FileName); err == nil {
				if _, by, err := file.GetContent(); err == nil {
					if b, _ := clearsign.Decode(by); b != nil {
						if p, err := packet.Read(b.ArmoredSignature.Body); err == nil {
							if sig, ok := p.(*packet.Signature); ok {
								zero := c.GetZeroUser()
								msg := fmt.Sprintf("<br />"+
									"<table %s>"+
									"<tr><td align=\"right\">File:&nbsp;&nbsp;</td><td><span style=\"color: #82e17f;\">%s</span> (<span style=\"color: #82e17f;\">%s</span>)</td></tr>"+
									"<tr><td align=\"right\">Signature made:&nbsp;&nbsp;</td><td><span style=\"color: #82e17f;\">%s</span></td></tr>"+
									"<tr><td align=\"right\">Fingerprint:&nbsp;&nbsp;</td><td><span style=\"color: #82e17f;\">%s</span></td></tr>"+
									"<tr><td align=\"right\">Issuer:&nbsp;&nbsp;</td><td><span style=\"color: #82e17f;\">%s</span></td></tr>"+
									"</table>",
									zero.GenerateChatStyle(),
									c.Upload.OrigFileName,
									humanize.Bytes(uint64(c.Upload.FileSize)),
									sig.CreationTime.Format(time.RFC1123),
									utils.FormatPgPFingerprint(sig.IssuerFingerprint),
									utils.Ternary(sig.SignerUserId != nil, *sig.SignerUserId, "n/a"))
								c.ZeroMsg(msg)
								c.Err = command.ErrRedirect
								return true
							}
						}
					}
				}
			}
		}
	}

	zeroUser := c.GetZeroUser()
	c.ToUser = &zeroUser
	c.Message = msg

	return true
}

func handleSubscribeCmd(c *command.Command) (handled bool) {
	if c.Message == "/subscribe" {
		_ = c.DB.SubscribeToRoom(c.AuthUser.ID, c.Room.ID)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleUnsubscribeCmd(c *command.Command) (handled bool) {
	if m := unsubscribeRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		room, err := c.DB.GetChatRoomByName(m[1])
		if err != nil {
			c.Err = err
			return true
		}
		_ = c.DB.UnsubscribeFromRoom(c.AuthUser.ID, room.ID)
		c.Err = command.ErrRedirect
		return true

	} else if c.Message == "/unsubscribe" {
		_ = c.DB.UnsubscribeFromRoom(c.AuthUser.ID, c.Room.ID)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleGroupChatCmd(c *command.Command) (handled bool) {
	if m := groupRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		groupName := m[1]
		c.Message = m[2]
		group, err := c.DB.GetRoomGroupByName(c.Room.ID, groupName)
		if err != nil {
			c.Err = err
			return true
		}
		if group.Locked {
			c.Err = errors.New("group is locked")
			return true
		}
		c.RedirectQP.Set(command.RedirectGroupQP, group.Name)
		c.GroupID = &group.ID
		return true
	} else if strings.HasPrefix(c.Message, "/g ") {
		c.Err = errors.New("invalid /g command")
		return true
	}
	return false
}

func handleListPmWhitelistCmd(c *command.Command) (handled bool) {
	if c.Message == "/pmwhitelist" {
		pmWhitelistUsers, _ := c.DB.GetPmWhitelistedUsers(c.AuthUser.ID)
		sort.Slice(pmWhitelistUsers, func(i, j int) bool {
			return pmWhitelistUsers[i].WhitelistedUser.Username < pmWhitelistUsers[j].WhitelistedUser.Username
		})
		msg := ""
		if len(pmWhitelistUsers) > 0 {
			msg += "\n"
			for _, ignoredUser := range pmWhitelistUsers {
				msg += ignoredUser.WhitelistedUser.Username.AtStr() + "\n"
			}
		} else {
			msg += "no PM whitelisted users"
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleSetPmModeCmd(c *command.Command) (handled bool) {
	if c.Message == "/setpmmode whitelist" {
		c.AuthUser.SetPmMode(c.DB, database.PmModeWhitelist)
		msg := `pm mode set to "whitelist"`
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true

	} else if c.Message == "/setpmmode standard" {
		c.AuthUser.SetPmMode(c.DB, database.PmModeStandard)
		msg := `pm mode set to "standard"`
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleTogglePmBlacklistedUser(c *command.Command) (handled bool) {
	if m := pmToggleBlacklistUserRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = command.ErrRedirect
			return true
		}
		if c.DB.ToggleBlacklistedUser(c.AuthUser.ID, user.ID) {
			c.Err = command.NewErrSuccess("added to blacklist")
		} else {
			c.Err = command.NewErrSuccess("removed from blacklist")
		}
		return true
	}
	return false
}

func handleTogglePmWhitelistedUser(c *command.Command) (handled bool) {
	if m := pmToggleWhitelistUserRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = command.ErrRedirect
			return true
		}
		if c.DB.ToggleWhitelistedUser(c.AuthUser.ID, user.ID) {
			c.Err = command.NewErrSuccess("added to whitelist")
		} else {
			c.Err = command.NewErrSuccess("removed from whitelist")
		}
		return true
	}
	return false
}

func handleChessCmd(c *command.Command) (handled bool) {
	if m := chessRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		username := database.Username(m[1])
		color := m[2]
		player1 := *c.AuthUser
		player2, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = errors.New("invalid username")
			return true
		}
		if _, err := ChessInstance.NewGame1(c.RoomKey, c.Room.ID, player1, player2, color); err != nil {
			c.Err = err
			return true
		}
		c.Err = command.NewErrSuccess("chess game created")
		return true
	}
	return
}

func handleInboxCmd(c *command.Command) (handled bool) {
	if m := inboxRgx.FindStringSubmatch(c.Message); len(m) == 4 {
		username := database.Username(m[1])
		encryptRaw := m[2]
		message := m[3]
		tryEncrypt := false
		if encryptRaw == " -e" {
			tryEncrypt = true
		}
		toUser, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = errors.New("invalid username")
			return true
		}

		if err := canUserInboxOther(c.DB, *c.AuthUser, toUser); err != nil {
			c.Err = err
			return true
		}

		inboxHTML := message
		if tryEncrypt {
			if toUser.GPGPublicKey == "" {
				c.Err = errors.New("user has no pgp public key")
				return true
			}
			inboxHTML, err = utils.GeneratePgpEncryptedMessage(toUser.GPGPublicKey, message)
			if err != nil {
				c.Err = errors.New("failed to encrypt")
				return true
			}
			inboxHTML = strings.Join(strings.Split(inboxHTML, "\n"), " ")
		}

		inboxHTML, _, _ = dutils.ProcessRawMessage(c.DB, inboxHTML, c.RoomKey, c.AuthUser.ID, c.Room.ID, nil, c.AuthUser.IsModerator(), c.AuthUser.CanUseMultiline, c.AuthUser.ManualMultiline)
		c.DB.CreateInboxMessage(inboxHTML, c.Room.ID, c.AuthUser.ID, toUser.ID, true, false, nil)

		c.DataMessage = "/inbox " + string(username) + " "
		c.Err = command.NewErrSuccess("inbox sent")
		return true

	} else if strings.HasPrefix(c.Message, "/inbox ") {
		c.Err = errors.New("invalid /inbox command")
		return true
	}
	return
}

func handleProfileCmd(c *command.Command) (handled bool) {
	if m := profileRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = ErrUsernameNotFound
			return true
		}
		profile := `/u/` + user.Username
		c.ZeroMsg(fmt.Sprintf(`[<a href="%s" rel="noopener noreferrer" target="_blank">profile of %s</a>]`, profile, user.Username))
		c.Err = command.ErrRedirect
		return true
	} else if strings.HasPrefix(c.Message, "/p ") {
		c.Err = errors.New("invalid profile command")
		return true
	}
	return
}

func handleChipsSendCmd(c *command.Command) (handled bool) {
	if m := chipsSendRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		username := database.Username(m[1])
		chips := database.PokerChip(utils.DoParseInt(m[2]))
		if chips <= 0 {
			c.Err = errors.New("must send at least 1 chip")
			return true
		}
		if chips > 1000000 {
			c.Err = errors.New("cannot send more than 1000000 chips")
			return true
		}
		if c.AuthUser.ChipsTest < chips {
			c.Err = errors.New("you do not have enough chips")
			return true
		}
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = errors.New("username does not exists")
			return true
		}
		user.ChipsTest += chips
		user.DoSave(c.DB)
		c.DataMessage = "/chips-send " + username.String() + " "
		c.Err = command.NewErrSuccess("chips sent")
		return true
	}
	return
}

func handleChipsResetCmd(c *command.Command) (handled bool) {
	if c.Message == "/chips-reset" {
		c.AuthUser.ResetChipsTest(c.DB)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleChipsBalanceCmd(c *command.Command) (handled bool) {
	if c.Message == "/chips" {
		c.ZeroMsg(fmt.Sprintf(`Balance: %d`, c.AuthUser.ChipsTest))
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleTutorialCmd(c *command.Command) (handled bool) {
	if c.Message == "/tuto" && utils.False() {
		name := "tuto_" + utils.GenerateToken10()
		room, _ := c.DB.CreateRoom(name, "", c.AuthUser.ID, false)
		c.Err = command.ErrRedirect
		c.ZeroProcMsg("Tutorial here -> #" + room.Name)
		c.ZeroPublicProcMsgRoom("Welcome to the tutorial", "", room.ID)
		return true
	}
	return
}

func handleDeleteMsgCmd(c *command.Command) (handled bool) {
	getMsgForUsername := func(msgs []database.ChatMessage, username database.Username) (database.ChatMessage, error) {
		var msg database.ChatMessage
		for _, msgTmp := range msgs {
			if msgTmp.User.Username == username {
				msg = msgTmp
				return msg, nil
			}
		}
		return msg, errors.New("failed to find msg")
	}
	delMsgFn := func(msgs []database.ChatMessage) error {
		msg, err := getMsgForUsername(msgs, c.AuthUser.Username)
		if err != nil {
			return err
		}
		if err := msg.UserCanDeleteErr(c.AuthUser); err != nil {
			return err
		}
		if msg.RoomID == config.GeneralRoomID && !msg.IsPm() {
			msg.User.DecrGeneralMessagesCount(c.DB)
		}
		_ = msg.Delete(c.DB)
		return command.ErrRedirect
	}
	if c.Message == "/d" {
		lastMsg, err := c.DB.GetUserLastChatMessageInRoom(c.AuthUser.ID, c.Room.ID)
		if err != nil {
			c.Err = errors.New("unable to find last message")
			return true
		}
		msgs := []database.ChatMessage{lastMsg}
		c.Err = delMsgFn(msgs)
		return true

	} else if m := deleteMsgRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		date := m[1]
		matchUsername := m[2]
		dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock())
		if err != nil {
			logrus.Error(err)
			c.Err = err
			return true
		}
		msgs, err := c.DB.GetRoomChatMessagesByDate(c.Room.ID, dt.UTC())
		if err != nil {
			c.Err = err
			return true
		}
		if len(msgs) == 0 {
			c.Err = errors.New("failed to find msg")
			return true
		}

		if !c.AuthUser.IsModerator() {
			c.Err = delMsgFn(msgs)
			return true
		}

		// Moderator
		var msg database.ChatMessage
		if len(msgs) == 1 {
			msg = msgs[0]
		} else if len(msgs) > 1 {
			if matchUsername == "" {
				c.Err = errors.New("more the 1 msg with this timestamp")
				return true
			}
			msg, err = getMsgForUsername(msgs, database.Username(matchUsername))
			if err != nil {
				c.Err = err
				return true
			}
		}
		if err := msg.UserCanDeleteErr(c.AuthUser); err != nil {
			c.Err = err
			return true
		}
		_ = msg.Delete(c.DB)
		c.Err = command.ErrRedirect
		return true

	} else if strings.HasPrefix(c.Message, "/d ") {
		c.Err = errors.New("invalid /d command")
		return true
	}
	return
}

func handleHideMsgCmd(c *command.Command) (handled bool) {
	if m := hideRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		date := m[1]
		dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock())
		if err != nil {
			logrus.Error(err)
			c.Err = err
			return true
		}
		msgs, err := c.DB.GetRoomChatMessagesByDate(c.Room.ID, dt.UTC())
		if err != nil {
			c.Err = err
			return true
		}
		if len(msgs) == 1 {
			c.DB.IgnoreMessage(c.AuthUser.ID, msgs[0].ID)
			c.Err = command.ErrRedirect
		} else {
			c.Err = errors.New("more than 1 message")
		}
		return true
	}
	return
}

func handleUnHideMsgCmd(c *command.Command) (handled bool) {
	if m := unhideRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		date := m[1]
		dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock())
		if err != nil {
			logrus.Error(err)
			c.Err = err
			return true
		}
		msgs, err := c.DB.GetRoomChatMessagesByDate(c.Room.ID, dt.UTC())
		if err != nil {
			c.Err = err
			return true
		}
		if len(msgs) == 1 {
			c.DB.UnIgnoreMessage(c.AuthUser.ID, msgs[0].ID)
			c.Err = command.ErrRedirect
		} else {
			c.Err = errors.New("more than 1 message")
		}
		return true
	}
	return
}

func handleIgnoreCmd(c *command.Command) (handled bool) {
	if m := ignoreRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = command.ErrRedirect
			return true
		}
		c.DB.IgnoreUser(c.AuthUser.ID, user.ID)
		database.MsgPubSub.Pub("refresh_"+string(c.AuthUser.Username), database.ChatMessageType{Typ: database.ForceRefresh})
		c.Err = command.ErrRedirect
		return true

	} else if c.Message == "/i" || c.Message == "/ignore" {
		ignoredUsers, _ := c.DB.GetIgnoredUsers(c.AuthUser.ID)
		sort.Slice(ignoredUsers, func(i, j int) bool {
			return ignoredUsers[i].IgnoredUser.Username < ignoredUsers[j].IgnoredUser.Username
		})
		msg := ""
		if len(ignoredUsers) > 0 {
			msg += "\n"
			for _, ignoredUser := range ignoredUsers {
				msg += ignoredUser.IgnoredUser.Username.AtStr() + "\n"
			}
		} else {
			msg += "no ignored users"
		}
		c.ZeroProcMsg(msg)
		c.Err = command.ErrRedirect
		return true

	} else if strings.HasPrefix(c.Message, "/ignore ") || strings.HasPrefix(c.Message, "/i ") {
		c.Err = errors.New("invalid ignore command")
		return true
	}
	return
}

func handleUnIgnoreCmd(c *command.Command) (handled bool) {
	if m := unIgnoreRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = command.ErrRedirect
			return true
		}
		c.DB.UnIgnoreUser(c.AuthUser.ID, user.ID)
		database.MsgPubSub.Pub("refresh_"+string(c.AuthUser.Username), database.ChatMessageType{Typ: database.ForceRefresh})
		c.Err = command.ErrRedirect
		return true
	} else if strings.HasPrefix(c.Message, "/unignore ") || strings.HasPrefix(c.Message, "/ui ") {
		c.Err = errors.New("invalid unignore command")
		return true
	}
	return
}

func handleToggleAutocomplete(c *command.Command) (handled bool) {
	if c.Message == "/toggle-autocomplete" {
		c.AuthUser.ToggleAutocompleteCommandsEnabled(c.DB)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleAfkCmd(c *command.Command) (handled bool) {
	if c.Message == "/afk" {
		c.AuthUser.ToggleAFK(c.DB)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleDateCmd(c *command.Command) (handled bool) {
	if c.Message == "/date" {
		c.ZeroMsg(time.Now().Format(time.RFC1123))
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleSuccessCmd(c *command.Command) (handled bool) {
	if c.Message == "/success" {
		c.Err = command.NewErrSuccess("success message")
		return true
	}
	return
}

func handleErrorCmd(c *command.Command) (handled bool) {
	if c.Message == "/error" {
		c.Err = errors.New("error message")
		return true
	}
	return
}

func handleSystemCmd(c *command.Command) (handled bool) {
	if strings.HasPrefix(c.Message, "/sys ") {
		c.Message = strings.Replace(c.Message, "/sys ", "/system ", 1)
	}
	if strings.HasPrefix(c.Message, "/system ") {
		c.Message = strings.TrimPrefix(c.Message, "/system ")
		c.SystemMsg = true
		return true
	}
	return false
}

func handleSetChatRoomExternalLink(c *command.Command) (handled bool) {
	if m := setUrlRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		externalURL := m[1]
		if !govalidator.IsURL(externalURL) {
			externalURL = ""
		}
		room, err := c.DB.GetChatRoomByID(c.Room.ID)
		if err != nil {
			c.Err = err
			return true
		}
		room.ExternalLink = externalURL
		room.DoSave(c.DB)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handlePurge(c *command.Command) (handled bool) {
	if m := purgeRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		isHB := m[1] == " -hb"
		username := database.Username(m[2])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = err
			return true
		}
		c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("purge %s #%d", user.Username, user.ID))
		if isHB {
			_ = c.DB.DeleteUserHbChatMessages(user.ID)
		} else {
			_ = c.DB.DeleteUserChatMessages(user.ID)
		}
		database.MsgPubSub.Pub(database.RefreshTopic, database.ChatMessageType{Typ: database.ForceRefresh})
		c.Err = command.ErrRedirect
		return true

	} else if c.Message == "/purge" {
		c.Err = command.ErrRedirect
		if !c.AuthUser.UseStream {
			c.Err = errors.New("only work on stream version of this chat")
			return true
		}
		payload := database.ChatMessageType{}
		streamModals.PurgeModal{}.Show(c.AuthUser.ID, c.Room.ID, payload)
		return true
	}
	return
}

func handleRename(c *command.Command) (handled bool) {
	if m := renameRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		oldUsername := database.Username(m[1])
		newUsername := database.Username(m[2])
		user, err := c.DB.GetUserByUsername(oldUsername)
		if err != nil {
			c.Err = err
			return true
		}
		c.DB.NewAudit(*c.AuthUser, fmt.Sprintf("rename %s -> %s #%d", user.Username, newUsername, user.ID))

		if err := c.DB.CanRenameTo(oldUsername, newUsername); err != nil {
			c.Err = err
			return true
		}

		managers.ActiveUsers.RemoveUser(user.ID)
		user.Username = newUsername
		user.DoSave(c.DB)

		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleNewMeme(c *command.Command) (handled bool) {
	if m := memeRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		if c.Upload == nil {
			c.Err = errors.New("no file uploaded")
			return true
		}
		slug := m[1]
		oldPath := filepath.Join(config.Global.ProjectUploadsPath.Get(), c.Upload.FileName)
		newPath := filepath.Join(config.Global.ProjectMemesPath.Get(), c.Upload.FileName)
		_ = os.Rename(oldPath, newPath)

		if err := c.DB.DB().Delete(&c.Upload).Error; err != nil {
			logrus.Error(err)
		}

		meme := database.Meme{
			Slug:         slug,
			FileName:     c.Upload.FileName,
			OrigFileName: c.Upload.OrigFileName,
			FileSize:     c.Upload.FileSize,
		}
		if err := c.DB.DB().Create(&meme).Error; err != nil {
			_ = os.Remove(newPath)
			logrus.Error(err)
		}

		c.Err = command.ErrRedirect
		return true

	} else if m := memeRenameRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		slug := m[1]
		newSlug := m[2]
		meme, err := c.DB.GetMemeBySlug(slug)
		if err != nil {
			c.Err = errors.New("meme not found")
			return true
		}
		meme.Slug = newSlug
		meme.DoSave(c.DB)
		c.Err = command.NewErrSuccess("meme renamed")
		return true
	}
	return
}

func handleRemoveMeme(c *command.Command) (handled bool) {
	if m := memeRemoveRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		slug := m[1]
		meme, err := c.DB.GetMemeBySlug(slug)
		if err != nil {
			c.Err = errors.New("meme not found")
			return true
		}
		if err := meme.Delete(c.DB); err != nil {
			c.Err = err
			return true
		}
		c.Err = command.NewErrSuccess("meme removed")
		return true
	}
	return
}

func handleListMemes(c *command.Command) (handled bool) {
	if m := memesRgx.FindStringSubmatch(c.Message); len(m) == 1 {
		memes, _ := c.DB.GetMemes()
		msg := ""
		for _, m := range memes {
			msg += fmt.Sprintf(`<a href="/memes/%s" rel="noopener noreferrer" target="_blank">%s</a>`, m.Slug, m.Slug)
			if c.AuthUser.IsAdmin {
				msg += fmt.Sprintf(` (%s)`, humanize.Bytes(uint64(m.FileSize)))
			}
			msg += "<br />"
		}
		c.ZeroMsg(msg)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleRefreshCmd(c *command.Command) (handled bool) {
	if c.Message == "/refresh" {
		c.Err = command.ErrRedirect
		database.MsgPubSub.Pub(database.RefreshTopic, database.ChatMessageType{Typ: database.ForceRefresh})
		return true
	}
	return
}

func handleWizzCmd(c *command.Command) (handled bool) {
	m := wizzRgx.FindStringSubmatch(c.Message)
	if c.Message == "/wizz" || len(m) == 2 {
		var wizzedUser *database.User
		wizzedUser = c.AuthUser

		if len(m) == 2 {
			username := database.Username(m[1])
			if username != c.AuthUser.Username {
				user, err := c.DB.GetUserByUsername(username)
				if err != nil {
					c.Err = ErrUsernameNotFound
					return true
				}
				wizzedUser = &user
			}
			c.ZeroSysMsgToSkipNotify(c.AuthUser, "you wizzed "+wizzedUser.Username.String())
		}

		c.ZeroSysMsgTo(wizzedUser, "wizzed by "+c.AuthUser.Username.String())
		database.MsgPubSub.Pub("wizz_"+wizzedUser.Username.String(), database.ChatMessageType{Typ: database.Wizz})
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleChipsCmd(c *command.Command) (handled bool) {
	if m := chipsRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		username := database.Username(m[1])
		chips := utils.DoParseInt64(m[2])

		if c.DB.DB().Model(&database.User{}).
			Where("username = ?", username).
			Select("ChipsTest").
			Updates(database.User{ChipsTest: database.PokerChip(chips)}).RowsAffected == 0 {
			c.Err = errors.New("username does not exists")
			return true
		}
		c.Err = command.NewErrSuccess("chips set")
		return true
	}
	return
}

func handleLocateCmd(c *command.Command) (handled bool) {
	if m := locateRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		username := database.Username(m[1])
		user, err := c.DB.GetUserByUsername(username)
		if err != nil {
			c.Err = errors.New("username does not exists")
			return true
		}
		roomIDs := managers.ActiveUsers.LocateUser(user.Username)
		rooms, _ := c.DB.GetChatRoomsByID(roomIDs)
		var msg string
		if len(rooms) > 0 {
			roomLinks := make([]string, len(rooms))
			for idx, room := range rooms {
				roomLinks[idx] = "#" + room.Name
			}
			msg = username.AtStr() + " is in " + strings.Join(roomLinks, " ")
		} else {
			msg = username.AtStr() + " could not be located in a public room"
		}
		c.ZeroProcMsg(msg)
		c.DataMessage = "/locate "
		c.Err = command.ErrStop
		return true
	}
	return
}

func handleCodeCmd(c *command.Command) (handled bool) {
	if c.Message == "/code" {
		c.Err = command.ErrRedirect
		if !c.AuthUser.CanUseMultiline {
			c.Err = errors.New("multiline is disabled for your account")
			return true
		} else if !c.AuthUser.UseStream {
			c.Err = errors.New("only work on stream version of this chat")
			return true
		}
		payload := database.ChatMessageType{}
		if c.ModMsg {
			payload.IsMod = true
		}
		if c.ToUser != nil {
			toUserUsername := c.ToUser.Username
			payload.ToUserUsername = &toUserUsername
		}
		streamModals.CodeModal{}.Show(c.AuthUser.ID, c.Room.ID, payload)
		return true
	}
	return
}

func handleUpdateReadMarkerCmd(c *command.Command) (handled bool) {
	if c.Message == "/r" {
		c.DB.UpdateChatReadMarker(c.AuthUser.ID, c.Room.ID)
		c.Err = command.ErrRedirect
		return true
	}
	return
}

func handleCheckCmd(c *command.Command) (handled bool) {
	if c.Message == "/check" {
		roomID := poker.RoomID(strings.ReplaceAll(c.Room.Name, "_", "-"))
		if g := poker.PokerInstance.GetGame(roomID); g != nil {
			g.Check(c.AuthUser.ID)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleCallCmd(c *command.Command) (handled bool) {
	if c.Message == "/call" {
		roomID := poker.RoomID(strings.ReplaceAll(c.Room.Name, "_", "-"))
		if g := poker.PokerInstance.GetGame(roomID); g != nil {
			g.Call(c.AuthUser.ID)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleFoldCmd(c *command.Command) (handled bool) {
	if c.Message == "/fold" {
		roomID := poker.RoomID(strings.ReplaceAll(c.Room.Name, "_", "-"))
		if g := poker.PokerInstance.GetGame(roomID); g != nil {
			g.Fold(c.AuthUser.ID)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleRaiseCmd(c *command.Command) (handled bool) {
	if c.Message == "/raise" {
		roomID := poker.RoomID(strings.ReplaceAll(c.Room.Name, "_", "-"))
		if g := poker.PokerInstance.GetGame(roomID); g != nil {
			g.Raise(c.AuthUser.ID)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleBetCmd(c *command.Command) (handled bool) {
	if m := betRgx.FindStringSubmatch(c.Message); len(m) == 2 {
		roomID := poker.RoomID(strings.ReplaceAll(c.Room.Name, "_", "-"))
		if g := poker.PokerInstance.GetGame(roomID); g != nil {
			bet := database.PokerChip(utils.DoParseUint64(m[1]))
			g.Bet(c.AuthUser.ID, bet)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleAllInCmd(c *command.Command) (handled bool) {
	if c.Message == "/allin" {
		roomID := poker.RoomID(strings.ReplaceAll(c.Room.Name, "_", "-"))
		if g := poker.PokerInstance.GetGame(roomID); g != nil {
			g.AllIn(c.AuthUser.ID)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleDealCmd(c *command.Command) (handled bool) {
	if c.Message == "/deal" {
		roomID := poker.RoomID(strings.ReplaceAll(c.Room.Name, "_", "-"))
		if g := poker.PokerInstance.GetGame(roomID); g != nil {
			g.Deal(c.AuthUser.ID)
		}
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleDistCmd(c *command.Command) (handled bool) {
	if m := distRgx.FindStringSubmatch(c.Message); len(m) == 3 {
		u1 := strings.ToLower(m[1])
		u2 := strings.ToLower(m[2])
		dist := levenshtein.ComputeDistance(u1, u2)
		c.ZeroProcMsg(fmt.Sprintf("levenshtein distance is %d", dist))
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleCloseCmd(c *command.Command) (handled bool) {
	if c.Message == "/close" {
		database.MsgPubSub.Pub("refresh_"+string(c.AuthUser.Username), database.ChatMessageType{Typ: database.Close})
		c.Err = command.ErrRedirect
		return true
	}
	return false
}

func handleCloseMenuCmd(c *command.Command) (handled bool) {
	if c.Message == "/closem" {
		database.MsgPubSub.Pub("refresh_loading_icon_"+string(c.AuthUser.Username), database.ChatMessageType{Typ: database.CloseMenu})
		c.Err = command.ErrRedirect
		return true
	}
	return false
}
