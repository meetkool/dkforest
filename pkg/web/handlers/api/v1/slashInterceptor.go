package v1

import (
	"dkforest/pkg/clockwork"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
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
//   and the chat iframe will be rendered instead of redirected.
//   This is useful to keep a prefix in the text box (eg: /pm user )
// if c.err is set to an instance of ErrSuccess,
//   a green message will appear beside the text box.
// otherwise if c.err is set to a different error,
//   text box is retested to original message,
//   and a red message will appear beside the text box.
type SlashInterceptor struct{}

func (i SlashInterceptor) InterceptMsg(c *Command) {
	if !strings.HasPrefix(c.message, "/") {
		return
	}
	handled := handleUserCmd(c) ||
		handlePrivateRoomCmd(c) ||
		handlePrivateRoomOwnerCmd(c) ||
		handleModeratorCmd(c) ||
		handleAdminCmd(c)
	if !handled {
		c.err = errors.New("invalid slash command")
	}
}

func handleUserCmd(c *Command) (handled bool) {
	return handleIgnoreCmd(c) ||
		handleUnIgnoreCmd(c) ||
		handleToggleAutocomplete(c) ||
		handleTutorialCmd(c) ||
		handleDeleteMsgCmd(c) ||
		handleListIgnoredCmd(c) ||
		handleListPmWhitelistCmd(c) ||
		handleSetPmModeWhitelistCmd(c) ||
		handleSetPmModeStandardCmd(c) ||
		handleTogglePmBlacklistedUser(c) ||
		handleTogglePmWhitelistedUser(c) ||
		handleGroupChatCmd(c) ||
		handleMeCmd(c) ||
		handleEditCmd(c) ||
		handlePMCmd(c) ||
		handleEditLastCmd(c) ||
		handleSubscribeCmd(c) ||
		handleUnsubscribeCmd(c) ||
		handleProfileCmd(c) ||
		handleInboxCmd(c) ||
		handleChessCmd(c) ||
		handleHbmCmd(c) ||
		handleHbmtCmd(c) ||
		handleTokenCmd(c) ||
		handleMd5Cmd(c) ||
		handleSha1Cmd(c) ||
		handleSha256Cmd(c) ||
		handleSha512Cmd(c) ||
		handleDiceCmd(c) ||
		handleChoiceCmd(c) ||
		handleSuccessCmd(c) ||
		handleErrorCmd(c)
}

func handlePrivateRoomCmd(c *Command) (handled bool) {
	return handleGetModeCmd(c) ||
		handleWhitelistCmd(c)
}

func handlePrivateRoomOwnerCmd(c *Command) (handled bool) {
	if (c.room.OwnerUserID != nil && *c.room.OwnerUserID == c.authUser.ID) || c.authUser.IsAdmin {
		return handleAddGroupCmd(c) ||
			handleRmGroupCmd(c) ||
			handleLockGroupCmd(c) ||
			handleUnlockGroupCmd(c) ||
			handleGroupUsersCmd(c) ||
			handleListGroupsCmd(c) ||
			handleGroupAddUserCmd(c) ||
			handleGroupRmUserCmd(c) ||
			handleSetModeWhitelistCmd(c) ||
			handleSetModeStandardCmd(c) ||
			handleGetRoomWhitelistCmd(c)
	}
	return false
}

func handleModeratorCmd(c *Command) (handled bool) {
	if c.authUser.IsModerator() {
		return handleModeratorGroupCmd(c) ||
			handleListModeratorsCmd(c) ||
			handleKickCmd(c) ||
			handleKickKeepCmd(c) ||
			handleUnkickCmd(c) ||
			handleLogoutCmd(c) ||
			handleForceCaptchaCmd(c) ||
			handleResetTutorialCmd(c) ||
			handleHellbanCmd(c) ||
			handleUnhellbanCmd(c)
	}
	return false
}

func handleAdminCmd(c *Command) (handled bool) {
	if c.authUser.IsAdmin {
		return handleSystemCmd(c)
	}
	return false
}

func handleModeratorGroupCmd(c *Command) (handled bool) {
	if strings.HasPrefix(c.message, "/m ") || strings.HasPrefix(c.message, "/n ") {
		if strings.HasPrefix(c.message, "/n ") {
			c.message = strings.Replace(c.message, "/n ", "/m ", 1)
		}
		c.message = strings.TrimPrefix(c.message, "/m ")
		c.redirectQP.Set(redirectModQP, "1")
		c.modMsg = true
		if handleMeCmd(c) {
			return true
		}
		return true
	}
	return false
}

func handleListModeratorsCmd(c *Command) (handled bool) {
	if c.message == "/moderators" || c.message == "/mods" {
		mods, err := database.GetModeratorsUsers()
		if err != nil {
			c.err = err
			return true
		}
		sort.Slice(mods, func(i, j int) bool { return mods[i].Username < mods[j].Username })
		c.message = "Moderators:\n"
		if len(mods) > 0 {
			c.message += "\n"
			for _, mod := range mods {
				c.message += "@" + mod.Username + "\n"
			}
		} else {
			c.message += "no moderators"
		}
		c.receivePM()
		return true
	}
	return false
}

func handleKickCmd(c *Command) (handled bool) {
	if m := kickRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		kickCmd(c, username, true)
		c.err = ErrRedirect
		return true
	}
	return
}

// Kick a user but keep the messages
func handleKickKeepCmd(c *Command) (handled bool) {
	if m := kickKeepRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		kickCmd(c, username, false)
		c.err = ErrRedirect
		return true
	}
	return
}

func kickCmd(c *Command, username string, purge bool) {
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return
	}
	// Can't kick a vetted user (unless admin)
	if !c.authUser.IsAdmin && user.Vetted {
		return
	}
	// Can't kick another moderator (unless admin)
	if !c.authUser.IsAdmin && user.IsModerator() {
		return
	}
	dutils.Kick(user, *c.authUser, purge)
}

func handleUnkickCmd(c *Command) (handled bool) {
	if m := unkickRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		database.NewAudit(*c.authUser, fmt.Sprintf("unkick %s #%d", user.Username, user.ID))
		user.Verified = true
		_ = user.Save()

		// Display unkick message
		styledUsername := fmt.Sprintf(`<span %s>%s</span>`, user.GenerateChatStyle(), user.Username)
		rawTxt := fmt.Sprintf("%s has been unkicked. (%s)", user.Username, c.authUser.Username)
		txt := fmt.Sprintf("%s has been unkicked. (%s)", styledUsername, c.authUser.Username)
		if err := database.CreateSysMsg(rawTxt, txt, "", config.GeneralRoomID, c.authUser.ID); err != nil {
			logrus.Error(err)
		}

		c.err = ErrRedirect
		return true
	}
	return
}

func handleForceCaptchaCmd(c *Command) (handled bool) {
	if m := forceCaptchaRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		if c.authUser.IsAdmin || !user.IsModerator() || c.authUser.Username == username {
			database.NewAudit(*c.authUser, fmt.Sprintf("force captcha %s #%d", user.Username, user.ID))
			user.CaptchaRequired = true
			user.DoSave()
		}
		c.err = ErrRedirect
		return true
	}
	return
}

func handleLogoutCmd(c *Command) (handled bool) {
	if m := logoutRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		if !c.authUser.IsAdmin && user.Vetted {
			c.err = ErrRedirect
			return true
		}
		if c.authUser.IsAdmin || !user.IsModerator() {
			database.NewAudit(*c.authUser, fmt.Sprintf("logout %s #%d", user.Username, user.ID))

			_ = database.DeleteUserSessions(user.ID)

			// Remove user from the user cache
			managers.ActiveUsers.RemoveUser(user.ID)
		}

		c.err = ErrRedirect
		return true
	}
	return
}

func handleResetTutorialCmd(c *Command) (handled bool) {
	if m := rtutoRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		if !c.authUser.IsAdmin && user.Vetted {
			c.err = ErrRedirect
			return true
		}
		if c.authUser.IsAdmin || !user.IsModerator() {
			database.NewAudit(*c.authUser, fmt.Sprintf("rtuto %s #%d", user.Username, user.ID))
			user.ChatTutorial = 0
			user.DoSave()
		}
		c.err = ErrRedirect
		return true
	}
	return
}

func handleHellbanCmd(c *Command) (handled bool) {
	if m := hellbanRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		if !c.authUser.IsAdmin && user.Vetted {
			c.err = ErrRedirect
			return true
		}
		if !c.authUser.IsAdmin && user.IsModerator() {
			c.err = ErrRedirect
			return true
		}
		database.NewAudit(*c.authUser, fmt.Sprintf("hellban %s #%d", user.Username, user.ID))
		user.HellBan()
		managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(user, nil))

		c.err = ErrRedirect
		return true
	}
	return
}

func handleUnhellbanCmd(c *Command) (handled bool) {
	if m := unhellbanRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		if !c.authUser.IsAdmin && user.IsModerator() {
			c.err = ErrRedirect
			return true
		}
		database.NewAudit(*c.authUser, fmt.Sprintf("unhellban %s #%d", user.Username, user.ID))
		user.UnHellBan()
		managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(user, nil))

		c.err = ErrRedirect
		return true
	}
	return false
}

func handleHbmCmd(c *Command) (handled bool) {
	if !c.authUser.CanSeeHB() {
		return
	}
	if strings.HasPrefix(c.message, "/hbm ") {
		c.message = strings.TrimPrefix(c.message, "/hbm ")
		c.hellbanMsg = true
		c.redirectQP.Set(redirectHbmQP, "1")
		return true
	}
	return
}

func handleHbmtCmd(c *Command) (handled bool) {
	if !c.authUser.CanSeeHB() {
		return
	}
	if m := hbmtRgx.FindStringSubmatch(c.message); len(m) == 2 {
		date := m[1]
		if dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock()); err == nil {
			if time.Since(dt) <= config.EditMessageTimeLimit {
				if msg, err := database.GetRoomChatMessageByDate(c.room.ID, c.authUser.ID, dt.UTC()); err == nil {
					msg.IsHellbanned = !msg.IsHellbanned
					msg.DoSave()
				} else {
					c.err = errors.New("no message found at this timestamp")
					return true
				}
			} else {
				c.err = errors.New("message is too old to be edited")
				return true
			}
		}
		c.err = ErrRedirect
		return true
	}
	return
}

func handleDiceCmd(c *Command) (handled bool) {
	if strings.HasPrefix(c.message, "/dice") {
		dice := utils.RandInt(1, 6)
		raw := fmt.Sprintf(`rolling dice for @%s ... "%d"`, c.authUser.Username, dice)
		msg := fmt.Sprintf(`rolling dice for @%s ... "<span style="color: white;">%d</span>"`, c.authUser.Username, dice)
		msg, _ = colorifyTaggedUsers(msg, database.GetUsersByUsername)
		go func() {
			time.Sleep(time.Second)
			c.zeroPublicMsg(raw, msg)
		}()
		return true
	}
	return
}

func handleChoiceCmd(c *Command) (handled bool) {
	if strings.HasPrefix(c.message, "/choice ") {
		tmp := strings.TrimPrefix(c.message, "/choice ")
		words := strings.Fields(tmp)
		answer := utils.RandChoice(words)
		raw := fmt.Sprintf(`@%s choice %s ... "%s"`, c.authUser.Username, words, answer)
		msg := fmt.Sprintf(`@%s choice %s ... "<span style="color: white;">%s</span>"`, c.authUser.Username, words, answer)
		msg, _ = colorifyTaggedUsers(msg, database.GetUsersByUsername)
		go func() {
			time.Sleep(time.Second)
			c.zeroPublicMsg(raw, msg)
		}()
		c.skipInboxes = true
		return true
	}
	return
}

func handleTokenCmd(c *Command) (handled bool) {
	if c.message == "/token" {
		c.zeroMsg(utils.GenerateToken10())
		c.err = ErrRedirect
		return true
	} else if m := tokenRgx.FindStringSubmatch(c.message); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > 32 {
			c.err = errors.New("value must be [1;32]")
			return true
		}
		n = utils.Clamp(n, 1, 32)
		c.zeroMsg(utils.GenerateTokenN(n))
		c.err = ErrRedirect
		return true
	}
	return
}

func handleMd5Cmd(c *Command) (handled bool) {
	return handleHasherCmd(c, "/md5 ", utils.MD5)
}

func handleSha1Cmd(c *Command) (handled bool) {
	return handleHasherCmd(c, "/sha1 ", utils.Sha1)
}

func handleSha256Cmd(c *Command) (handled bool) {
	return handleHasherCmd(c, "/sha256 ", utils.Sha256)
}

func handleSha512Cmd(c *Command) (handled bool) {
	return handleHasherCmd(c, "/sha512 ", utils.Sha512)
}

func handleHasherCmd(c *Command, prefix string, fn func([]byte) string) (handled bool) {
	if strings.HasPrefix(c.message, prefix) {
		c.message = strings.TrimPrefix(c.message, prefix)
		c.dataMessage = prefix
		c.zeroMsg(fn([]byte(c.message)))
		c.err = ErrStop
		return true
	}
	return
}

func handleRmGroupCmd(c *Command) (handled bool) {
	if m := rmGroupRgx.FindStringSubmatch(c.message); len(m) == 2 {
		groupName := m[1]
		if err := database.DeleteChatRoomGroup(c.room.ID, groupName); err != nil {
			c.err = err
			return true
		}
		c.err = ErrRedirect
		return true
	}
	return false
}

func handleLockGroupCmd(c *Command) (handled bool) {
	if m := lockGroupRgx.FindStringSubmatch(c.message); len(m) == 2 {
		groupName := m[1]
		group, err := database.GetRoomGroupByName(c.room.ID, groupName)
		if err != nil {
			c.err = err
			return true
		}
		group.Locked = true
		group.DoSave()
		c.err = ErrRedirect
		return true
	}
	return false
}

func handleUnlockGroupCmd(c *Command) (handled bool) {
	if m := unlockGroupRgx.FindStringSubmatch(c.message); len(m) == 2 {
		groupName := m[1]
		group, err := database.GetRoomGroupByName(c.room.ID, groupName)
		if err != nil {
			c.err = err
			return true
		}
		group.Locked = false
		group.DoSave()
		c.err = ErrRedirect
		return true
	}
	return false
}

func handleGroupUsersCmd(c *Command) (handled bool) {
	if m := groupUsersRgx.FindStringSubmatch(c.message); len(m) == 2 {
		groupName := m[1]
		group, err := database.GetRoomGroupByName(c.room.ID, groupName)
		if err != nil {
			c.err = err
			return true
		}
		users, err := database.GetRoomGroupUsers(c.room.ID, group.ID)
		sort.Slice(users, func(i, j int) bool {
			return users[i].User.Username < users[j].User.Username
		})
		c.message = ""
		if len(users) > 0 {
			c.message += "\n"
			for _, user := range users {
				c.message += "@" + user.User.Username + "\n"
			}
		} else {
			c.message += "no user in th group: " + groupName
		}
		c.receivePM()
		return true
	}
	return false
}

func handleListGroupsCmd(c *Command) (handled bool) {
	if c.message == "/groups" {
		groups, err := database.GetRoomGroups(c.room.ID)
		if err != nil {
			c.err = err
			return true
		}
		c.message = ""
		if len(groups) > 0 {
			c.message += "\n"
			for _, group := range groups {
				c.message += group.Name + " (" + group.Color + ")\n"
			}
		} else {
			c.message += "no groups"
		}
		c.receivePM()
		return true
	}
	return false
}

func handleGroupAddUserCmd(c *Command) (handled bool) {
	if m := groupAddUserRgx.FindStringSubmatch(c.message); len(m) == 3 {
		groupName := m[1]
		username := m[2]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = err
			return true
		}
		group, err := database.GetRoomGroupByName(c.room.ID, groupName)
		if err != nil {
			c.err = err
			return true
		}
		_, err = database.AddUserToRoomGroup(c.room.ID, group.ID, user.ID)
		if err != nil {
			c.err = err
			return true
		}
		c.err = ErrRedirect
		return true
	} else if strings.HasPrefix(c.message, "/gadduser ") {
		c.err = errors.New("invalid /gadduser command")
	}
	return false
}

func handleGroupRmUserCmd(c *Command) (handled bool) {
	if m := groupRmUserRgx.FindStringSubmatch(c.message); len(m) == 3 {
		groupName := m[1]
		username := m[2]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = err
			return true
		}
		group, err := database.GetRoomGroupByName(c.room.ID, groupName)
		if err != nil {
			c.err = err
			return true
		}
		err = database.RmUserFromRoomGroup(c.room.ID, group.ID, user.ID)
		if err != nil {
			c.err = err
			return true
		}
		c.err = ErrRedirect
		return true
	} else if strings.HasPrefix(c.message, "/grmuser ") {
		c.err = errors.New("invalid /grmuser command")
	}
	return false
}

func handleSetModeWhitelistCmd(c *Command) (handled bool) {
	if c.message == "/mode user-whitelist" {
		c.room.Mode = database.UserWhitelistRoomMode
		c.room.DoSave()
		c.message = `room mode set to "user whitelist"`
		c.receivePM()
		return true
	}
	return false
}

func handleSetModeStandardCmd(c *Command) (handled bool) {
	if c.message == "/mode standard" {
		c.room.Mode = database.NormalRoomMode
		c.room.DoSave()
		c.message = `room mode set to "standard"`
		c.receivePM()
		return true
	}
	return false
}

func handleGetRoomWhitelistCmd(c *Command) (handled bool) {
	if m := whitelistUserRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.message = fmt.Sprintf(`username "%s" not found`, username)
		} else {
			if _, err := database.WhitelistUser(c.room.ID, user.ID); err != nil {
				if err := database.DeWhitelistUser(c.room.ID, user.ID); err != nil {
					c.message = fmt.Sprintf("failed to toggle @%s in whitelist", user.Username)
				} else {
					c.message = fmt.Sprintf("@%s removed from whitelist", user.Username)
				}
			} else {
				c.message = fmt.Sprintf("@%s added to whitelist", user.Username)
			}
		}
		c.receivePM()
		return true
	}
	return false
}

func handleAddGroupCmd(c *Command) (handled bool) {
	if m := addGroupRgx.FindStringSubmatch(c.message); len(m) == 2 {
		name := m[1]
		_, err := database.CreateChatRoomGroup(c.room.ID, name, "#fff")
		if err != nil {
			c.err = err
			return true
		}
		c.err = ErrRedirect
		return true
	}
	return false
}

func handleWhitelistCmd(c *Command) (handled bool) {
	if c.message == "/whitelist" || c.message == "/wl" {
		whitelistedUsers, _ := database.GetWhitelistedUsers(c.room.ID)
		if len(whitelistedUsers) > 0 {
			usernames := make([]string, 0)
			for _, whitelistedUser := range whitelistedUsers {
				usernames = append(usernames, "@"+whitelistedUser.User.Username)
			}
			sort.Slice(usernames, func(i, j int) bool { return usernames[i] < usernames[j] })
			c.message = "whitelisted users: " + strings.Join(usernames, ", ")
		} else {
			c.message = "no whitelisted user"
		}
		c.receivePM()
		return true
	}
	return false
}

func handleGetModeCmd(c *Command) (handled bool) {
	if c.message == "/mode" {
		if c.room.Mode == database.NormalRoomMode {
			c.message = `room is in "standard" mode`
		} else if c.room.Mode == database.UserWhitelistRoomMode {
			c.message = `room is in "user whitelist" mode`
		}
		c.receivePM()
		return true
	}
	return false
}

func handleMeCmd(c *Command) (handled bool) {
	if c.message == "/me " {
		c.err = errors.New("invalid /me command")
		return true
	}
	if strings.HasPrefix(c.message, "/me ") {
		return true
	}
	return
}

func handleEditCmd(c *Command) (handled bool) {
	if m := editRgx.FindStringSubmatch(c.message); len(m) == 3 {
		date := m[1]
		newMsg := m[2]
		if dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock()); err == nil {
			if time.Since(dt) <= config.EditMessageTimeLimit {
				if msg, err := database.GetRoomChatMessageByDate(c.room.ID, c.authUser.ID, dt.UTC()); err == nil {
					c.editMsg = &msg
					c.origMessage = newMsg
					c.message = newMsg

					// If we're editing a message which contains a link to an uploaded file,
					// we need to re-add the link to the html.
					if msg.UploadID != nil {
						if newUpload, err := database.GetUploadByID(*msg.UploadID); err == nil {
							c.upload = &newUpload
						}
					}

					if pmRgx.MatchString(c.message) {
						handlePMCmd(c)
					} else if c.authUser.IsModerator() && strings.HasPrefix(c.message, "/m ") {
						handleModeratorGroupCmd(c)
					} else if strings.HasPrefix(c.message, "/hbm ") {
						handleHbmCmd(c)
					} else if strings.HasPrefix(c.message, "/g ") {
						handleGroupChatCmd(c)
					} else if strings.HasPrefix(c.message, "/system ") || strings.HasPrefix(c.message, "/sys ") {
						handleSystemCmd(c)
					}
				}
			}
		}
		return true
	}
	return
}

func handleEditLastCmd(c *Command) (handled bool) {
	if c.message == "/e" {
		msg, err := database.GetUserLastChatMessageInRoom(c.authUser.ID, c.room.ID)
		if err != nil {
			return true
		}
		c.redirectQP.Set(redirectEditQP, msg.CreatedAt.Format("15:04:05"))
		c.err = ErrRedirect
		return true
	}
	return
}

var ErrPMDenied = errors.New("you cannot /pm this user")

func handlePMCmd(c *Command) (handled bool) {
	if m := pmRgx.FindStringSubmatch(c.message); len(m) == 3 {
		username := m[1]
		newMsg := m[2]

		// Chat helpers
		if username == config.NullUsername {
			return handlePm0(c, newMsg)
		}

		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = errors.New("invalid username")
			return true
		}
		if user.PmMode == database.PmModeWhitelist && !database.IsUserPmWhitelisted(c.authUser.ID, user.ID) {
			c.err = ErrPMDenied
			return true
		} else if user.PmMode == database.PmModeStandard {

			if !c.authUser.CanSendPM() {
				if c.room.IsOwned() {
					// In private rooms, can send PM but inboxes will be skipped if not enough karma
					c.skipInboxes = true
				} else {
					// Need at least 1 karma to send PM from a public room
					c.err = errors.New(`you need 20 public messages to unlock PMs; or be whitelisted`)
					return true
				}
			}

			if database.IsUserPmBlacklisted(c.authUser.ID, user.ID) {
				c.err = ErrPMDenied
				return true
			} else if user.BlockNewUsersPm && !c.authUser.AccountOldEnough() && !database.IsUserPmWhitelisted(c.authUser.ID, user.ID) {
				c.err = ErrPMDenied
				return true
			}
		}
		if user.ID == c.authUser.ID {
			c.err = errors.New("cannot /pm yourself")
			return true
		}
		c.toUser = &user
		c.message = newMsg
		c.redirectQP.Set(redirectPmQP, user.Username)

		if newMsg == "/d" || strings.HasPrefix(newMsg, "/d ") {
			handled = handleDeleteMsgCmd(c)
			if c.err != nil && c.err != ErrRedirect {
				return handled
			}
			c.err = ErrRedirect
			return handled
		}

		return true
	} else if strings.HasPrefix(c.message, "/pm ") {
		c.err = errors.New("invalid /pm command")
		return true
	}
	return false
}

// Handle PMs sent to user 0 (/pm 0 msg)
func handlePm0(c *Command, msg string) (handled bool) {
	c.redirectQP.Set(redirectPmQP, "0")
	if msg == "ping" {
		c.zeroMsg("pong")
		c.err = ErrRedirect
		return true

	} else if msg == "talk" {
		c.zeroMsg("talking")
		c.err = ErrRedirect
		return true

	} else if msg == "pgp" || msg == "gpg" {
		pkey := c.authUser.GPGPublicKey
		if pkey == "" {
			c.message = "I could not find a public pgp key in your profile."
		} else {
			msg := "This is a sample text"
			if encrypted, err := utils.GeneratePgpEncryptedMessage(pkey, msg); err != nil {
				c.message = err.Error()
			} else {
				c.message = strings.Join(strings.Split(encrypted, "\n"), " ")
			}
		}
		c.zeroProcMsg(c.message)
		c.err = ErrRedirect
		return true

	} else if pgpMsg := extractPGPMessage(msg); pgpMsg != "" {
		decrypted, err := utils.PgpDecryptMessage(config.NullUserPrivateKey, pgpMsg)
		if err != nil {
			c.message = err.Error()
		} else {
			c.message = "Decrypted message: " + decrypted
		}
		c.zeroProcMsg(c.message)
		c.err = ErrRedirect
		return true
	}

	zeroUser := c.getZeroUser()
	c.toUser = &zeroUser
	c.message = msg

	return true
}

func handleSubscribeCmd(c *Command) (handled bool) {
	if c.message == "/subscribe" {
		_ = database.SubscribeToRoom(c.authUser.ID, c.room.ID)
		c.err = ErrRedirect
		return true
	}
	return
}

func handleUnsubscribeCmd(c *Command) (handled bool) {
	if m := unsubscribeRgx.FindStringSubmatch(c.message); len(m) == 2 {
		room, err := database.GetChatRoomByName(m[1])
		if err != nil {
			c.err = err
			return true
		}
		_ = database.UnsubscribeFromRoom(c.authUser.ID, room.ID)
		c.err = ErrRedirect
		return true

	} else if c.message == "/unsubscribe" {
		_ = database.UnsubscribeFromRoom(c.authUser.ID, c.room.ID)
		c.err = ErrRedirect
		return true
	}
	return
}

func handleGroupChatCmd(c *Command) (handled bool) {
	if m := groupRgx.FindStringSubmatch(c.message); len(m) == 3 {
		groupName := m[1]
		c.message = m[2]
		group, err := database.GetRoomGroupByName(c.room.ID, groupName)
		if err != nil {
			c.err = err
			return true
		}
		if group.Locked {
			c.err = errors.New("group is locked")
			return true
		}
		c.redirectQP.Set(redirectGroupQP, group.Name)
		c.groupID = &group.ID
		return true
	} else if strings.HasPrefix(c.message, "/g ") {
		c.err = errors.New("invalid /g command")
		return true
	}
	return false
}

func handleListIgnoredCmd(c *Command) (handled bool) {
	if c.message == "/i" || c.message == "/ignore" {
		ignoredUsers, _ := database.GetIgnoredUsers(c.authUser.ID)
		sort.Slice(ignoredUsers, func(i, j int) bool {
			return ignoredUsers[i].IgnoredUser.Username < ignoredUsers[j].IgnoredUser.Username
		})
		c.message = ""
		if len(ignoredUsers) > 0 {
			c.message += "\n"
			for _, ignoredUser := range ignoredUsers {
				c.message += "@" + ignoredUser.IgnoredUser.Username + "\n"
			}
		} else {
			c.message += "no ignored users"
		}
		c.receivePM()
		return true
	}
	return false
}

func handleListPmWhitelistCmd(c *Command) (handled bool) {
	if c.message == "/pmwhitelist" {
		pmWhitelistUsers, _ := database.GetPmWhitelistedUsers(c.authUser.ID)
		sort.Slice(pmWhitelistUsers, func(i, j int) bool {
			return pmWhitelistUsers[i].WhitelistedUser.Username < pmWhitelistUsers[j].WhitelistedUser.Username
		})
		c.message = ""
		if len(pmWhitelistUsers) > 0 {
			c.message += "\n"
			for _, ignoredUser := range pmWhitelistUsers {
				c.message += "@" + ignoredUser.WhitelistedUser.Username + "\n"
			}
		} else {
			c.message += "no PM whitelisted users"
		}
		c.receivePM()
		return true
	}
	return false
}

func handleSetPmModeWhitelistCmd(c *Command) (handled bool) {
	if c.message == "/setpmmode whitelist" {
		c.authUser.PmMode = database.PmModeWhitelist
		c.authUser.DoSave()
		c.message = `pm mode set to "whitelist"`
		c.receivePM()
		return true
	}
	return false
}

func handleSetPmModeStandardCmd(c *Command) (handled bool) {
	if c.message == "/setpmmode standard" {
		c.authUser.PmMode = database.PmModeStandard
		c.authUser.DoSave()
		c.message = `pm mode set to "standard"`
		c.receivePM()
		return true
	}
	return false
}

func handleTogglePmBlacklistedUser(c *Command) (handled bool) {
	if m := pmToggleBlacklistUserRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		if database.ToggleBlacklistedUser(c.authUser.ID, user.ID) {
			c.err = NewErrSuccess("added to blacklist")
		} else {
			c.err = NewErrSuccess("removed from blacklist")
		}
		return true
	}
	return false
}

func handleTogglePmWhitelistedUser(c *Command) (handled bool) {
	if m := pmToggleWhitelistUserRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		if database.ToggleWhitelistedUser(c.authUser.ID, user.ID) {
			c.err = NewErrSuccess("added to whitelist")
		} else {
			c.err = NewErrSuccess("removed from whitelist")
		}
		return true
	}
	return false
}

func handleChessCmd(c *Command) (handled bool) {
	if m := chessRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		player1 := *c.authUser
		player2, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = errors.New("invalid username")
			return true
		}
		if _, err := ChessInstance.NewGame1(c.roomKey, c.room.ID, player1, player2); err != nil {
			c.err = err
			return true
		}
		c.err = NewErrSuccess("chess game created")
		return true
	}
	return
}

func handleInboxCmd(c *Command) (handled bool) {
	if m := inboxRgx.FindStringSubmatch(c.message); len(m) == 4 {
		username := m[1]
		encryptRaw := m[2]
		message := m[3]
		tryEncrypt := false
		if encryptRaw == " -e" {
			tryEncrypt = true
		}
		toUser, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = errors.New("invalid username")
			return true
		}
		if toUser.PmMode == database.PmModeWhitelist && !database.IsUserPmWhitelisted(c.authUser.ID, toUser.ID) {
			c.err = errors.New("you cannot inbox this user")
			return true
		} else if toUser.PmMode == database.PmModeStandard {
			if !c.authUser.CanSendPM() {
				c.err = errors.New("you need 20 public messages to unlock inboxes; or be whitelisted")
				return true
			}
			if database.IsUserPmBlacklisted(c.authUser.ID, toUser.ID) {
				c.err = errors.New("you cannot inbox this user")
				return true
			} else if !c.authUser.AccountOldEnough() && toUser.BlockNewUsersPm && !database.IsUserPmWhitelisted(c.authUser.ID, toUser.ID) {
				c.err = errors.New("you cannot inbox this user")
				return true
			}
		}
		html := message
		if tryEncrypt {
			if toUser.GPGPublicKey == "" {
				c.err = errors.New("user has no pgp public key")
				return true
			}
			html, err = utils.GeneratePgpEncryptedMessage(toUser.GPGPublicKey, message)
			if err != nil {
				c.err = errors.New("failed to encrypt")
				return true
			}
			html = strings.Join(strings.Split(html, "\n"), " ")
		}

		html, _ = ProcessRawMessage(html, c.roomKey, c.authUser.ID, c.room.ID, nil)
		database.CreateInboxMessage(html, c.room.ID, c.authUser.ID, toUser.ID, true, false, nil)

		c.dataMessage = "/inbox " + username + " "
		c.err = NewErrSuccess("inbox sent")
		return true

	} else if strings.HasPrefix(c.message, "/inbox ") {
		c.err = errors.New("invalid /inbox command")
		return true
	}
	return
}

func handleProfileCmd(c *Command) (handled bool) {
	if m := profileRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = errors.New("username not found")
			return true
		}
		profile := `/u/` + user.Username
		c.zeroMsg(fmt.Sprintf(`[<a href="%s" rel="noopener noreferrer" target="_blank">profile of %s</a>]`, profile, username))
		c.err = ErrRedirect
		return true
	} else if strings.HasPrefix(c.message, "/p ") {
		c.err = errors.New("invalid profile command")
		return true
	}
	return
}

type tutorialSteps struct {
	SendMessage  bool
	EditMessage  bool
	SendPM       bool
	EditPM       bool
	SendQuote    bool
	TagSomeone   bool
	VisitProfile bool
}

func handleTutorialCmd(c *Command) (handled bool) {
	if c.message == "/tuto" && false {
		name := "tuto_" + utils.GenerateToken10()
		room, _ := database.CreateRoom(name, "", c.authUser.ID, false)
		c.err = ErrRedirect
		c.zeroProcMsg("Tutorial here -> #" + room.Name)
		c.zeroPublicProcMsgRoom("Welcome to the tutorial", "", room.ID)
		return true
	}
	return
}

func handleDeleteMsgCmd(c *Command) (handled bool) {
	delMsgFn := func(msg database.ChatMessage) {
		if msg.RoomID == config.GeneralRoomID && msg.ToUserID == nil {
			msg.User.GeneralMessagesCount--
			msg.User.DoSave()
		}
		_ = database.DeleteChatMessageByUUID(msg.UUID)
	}
	if c.message == "/d" {
		if msg, err := database.GetUserLastChatMessageInRoom(c.authUser.ID, c.room.ID); err != nil {
			c.err = errors.New("unable to find last message")
			return true
		} else if msg.TooOldToDelete() {
			c.err = errors.New("message is to old to be deleted")
			return true
		} else {
			delMsgFn(msg)
		}
		c.err = ErrRedirect
		return true

	} else if m := deleteMsgRgx.FindStringSubmatch(c.message); len(m) >= 3 {
		if len(m) == 3 {
			date := m[1]
			matchUsername := m[2]
			dt, err := utils.ParsePrevDatetimeAt(date, clockwork.NewRealClock())
			if err != nil {
				logrus.Error(err)
				c.err = err
				return true
			}
			msgs, err := database.GetRoomChatMessagesByDate(c.room.ID, dt.UTC())
			if err != nil {
				c.err = err
				return true
			}
			if len(msgs) == 0 {
				c.err = errors.New("failed to find msg")
				return true

			} else if len(msgs) == 1 {
				msg := msgs[0]
				if !c.authUser.IsModerator() {
					if msg.User.Username != c.authUser.Username {
						c.err = errors.New("failed to find msg")
						return true
					}
					if msg.TooOldToDelete() {
						c.err = errors.New("message is to old to be deleted")
						return true
					}
					delMsgFn(msg)
					c.err = ErrRedirect
					return true
				}
				// Moderator
				_ = database.DeleteChatMessageByUUID(msg.UUID)
				c.err = ErrRedirect
				return true

			} else if len(msgs) > 1 {
				if !c.authUser.IsModerator() {
					var msg database.ChatMessage
					for _, msgTmp := range msgs {
						if msgTmp.User.Username == c.authUser.Username {
							msg = msgTmp
							break
						}
					}
					if msg.UUID == "" {
						c.err = errors.New("failed to find msg")
						return true
					}
					if msg.TooOldToDelete() {
						c.err = errors.New("message is to old to be deleted")
						return true
					}
					delMsgFn(msg)
					c.err = ErrRedirect
					return true

				}

				// Moderator
				if matchUsername == "" {
					c.err = errors.New("more the 1 msg with this timestamp")
					return true
				}
				var msg database.ChatMessage
				for _, msgTmp := range msgs {
					if msgTmp.User.Username == matchUsername {
						msg = msgTmp
						break
					}
				}
				if msg.UUID == "" {
					c.err = errors.New("failed to find msg")
					return true
				}
				_ = database.DeleteChatMessageByUUID(msg.UUID)
				c.err = ErrRedirect
				return true
			}
		}
		return true

	} else if strings.HasPrefix(c.message, "/d ") {
		c.err = errors.New("invalid /d command")
		return true
	}
	return
}

func handleIgnoreCmd(c *Command) (handled bool) {
	if m := ignoreRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		database.IgnoreUser(c.authUser.ID, user.ID)
		c.err = ErrRedirect
		return true
	} else if strings.HasPrefix(c.message, "/ignore ") || strings.HasPrefix(c.message, "/i ") {
		c.err = errors.New("invalid ignore command")
		return true
	}
	return
}

func handleUnIgnoreCmd(c *Command) (handled bool) {
	if m := unIgnoreRgx.FindStringSubmatch(c.message); len(m) == 2 {
		username := m[1]
		user, err := database.GetUserByUsername(username)
		if err != nil {
			c.err = ErrRedirect
			return true
		}
		database.UnIgnoreUser(c.authUser.ID, user.ID)
		c.err = ErrRedirect
		return true
	} else if strings.HasPrefix(c.message, "/unignore ") || strings.HasPrefix(c.message, "/ui ") {
		c.err = errors.New("invalid unignore command")
		return true
	}
	return
}

func handleToggleAutocomplete(c *Command) (handled bool) {
	if c.message == "/toggle-autocomplete" {
		c.authUser.AutocompleteCommandsEnabled = !c.authUser.AutocompleteCommandsEnabled
		c.authUser.DoSave()
		c.err = ErrRedirect
		return true
	}
	return
}

func handleSuccessCmd(c *Command) (handled bool) {
	if c.message == "/success" {
		c.err = NewErrSuccess("success message")
		return true
	}
	return
}

func handleErrorCmd(c *Command) (handled bool) {
	if c.message == "/error" {
		c.err = errors.New("error message")
		return true
	}
	return
}

func handleSystemCmd(c *Command) (handled bool) {
	if strings.HasPrefix(c.message, "/sys ") {
		c.message = strings.Replace(c.message, "/sys ", "/system ", 1)
	}
	if strings.HasPrefix(c.message, "/system ") {
		c.message = strings.TrimPrefix(c.message, "/system ")
		c.systemMsg = true
		return true
	}
	return false
}
