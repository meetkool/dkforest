package interceptors

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const minMsgLen = 1
const maxMsgLen = 10000

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
var pmRgx = regexp.MustCompile(`^/pm ` + optAtGUserOr0 + `(?:\s(?s:(.*)))?`)
var editRgx = regexp.MustCompile(`^/e (` + chatTs + `)\s(?s:(.*))`)
var hbmtRgx = regexp.MustCompile(`^/hbmt (` + chatTs + `)$`)
var chessRgx = regexp.MustCompile(`^/chess ` + optAtGUser + `(?:\s(w|b|r))?`)
var inboxRgx = regexp.MustCompile(`^/inbox ` + optAtGUser + `(\s-e)?\s(?s:(.*))`)
var purgeRgx = regexp.MustCompile(`^/purge(\s-hb)? ` + optAtGUserOr0)
var renameRgx = regexp.MustCompile(`^/rename ` + optAtGUser + ` ` + optAtGUser)
var profileRgx = regexp.MustCompile(`^/p ` + optAtGUserOr0)
var kickRgx = regexp.MustCompile(`^/(?:kick|k) ` + optAtGUser)
var setUrlRgx = regexp.MustCompile(`^/seturl (.+)`)
var kickKeepRgx = regexp.MustCompile(`^/(?:kk) ` + optAtGUser)
var kickSilentRgx = regexp.MustCompile(`^/(?:ks) ` + optAtGUser)
var kickKeepSilentRgx = regexp.MustCompile(`^/(?:kks) ` + optAtGUser)
var rtutoRgx = regexp.MustCompile(`^/(?:rtuto) ` + optAtGUser)
var logoutRgx = regexp.MustCompile(`^/(?:logout) ` + optAtGUser)
var wizzRgx = regexp.MustCompile(`^/(?:wizz) ` + optAtGUser)
var forceCaptchaRgx = regexp.MustCompile(`^/(?:captcha) ` + optAtGUser)
var unkickRgx = regexp.MustCompile(`^/(?:unkick|uk) ` + optAtGUser)
var hellbanRgx = regexp.MustCompile(`^/(?:hellban|hb) ` + optAtGUser)
var unhellbanRgx = regexp.MustCompile(`^/(?:unhellban|uhb) ` + optAtGUser)
var randRgx = regexp.MustCompile(`^/rand (-?\d+) (-?\d+)$`)
var tokenRgx = regexp.MustCompile(`^/token (\d{1,2})$`)
var snippetRgx = regexp.MustCompile(`!\w{1,20}`)
var tagRgx = regexp.MustCompile(`@(` + userOr0 + `)`)
var autoTagRgx = regexp.MustCompile(`(?:\\?)@(\w+)\*`)
var roomTagRgx = regexp.MustCompile(`#(` + roomNameF + `)`)
var tzRgx = regexp.MustCompile(`(\d{4}-\d{1,2}-\d{1,2} at \d{1,2}\.\d{1,2}\.\d{1,2} (?i)[A|P]M)`) // Screen Shot 2022-02-04 at 11.58.58 PM
var tz1Rgx = regexp.MustCompile(`(\d{4}-\d{1,2}-\d{1,2} \d{1,2}-\d{1,2}-\d{1,2})`)                // Screenshot from 2022-02-04 11-58-58.png
var tz3Rgx = regexp.MustCompile(`(\d{4}-\d{1,2}-\d{1,2} \d{1,6})`)                                // Screenshot 2023-05-20 202351.png
var tz4Rgx = regexp.MustCompile(`(\d{4}-\d{1,2}-\d{1,2}_\d{1,2}_\d{1,2}_\d{1,2})`)                // Screenshot_2023-05-20_11_13_14.png
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
var hideRgx = regexp.MustCompile(`^/hide (?:“\[)?(\d{2}:\d{2}:\d{2})`)
var unhideRgx = regexp.MustCompile(`^/unhide (\d{2}:\d{2}:\d{2})$`)
var memeRgx = regexp.MustCompile(`^/meme ([a-zA-Z0-9_-]{3,50})$`)
var memeRenameRgx = regexp.MustCompile(`^/meme ([a-zA-Z0-9_-]{3,50}) ([a-zA-Z0-9_-]{3,50})$`)
var memeRemoveRgx = regexp.MustCompile(`^/memerm ([a-zA-Z0-9_-]{3,50})$`)
var memesRgx = regexp.MustCompile(`^/memes$`)
var locateRgx = regexp.MustCompile(`^/locate ` + optAtGUser)
var chipsRgx = regexp.MustCompile(`^/chips ` + optAtGUser + ` (\d+)`)
var chipsSendRgx = regexp.MustCompile(`^/chips-send ` + optAtGUser + ` (\d+)`)
var betRgx = regexp.MustCompile(`^/bet (\d+)$`)
var distRgx = regexp.MustCompile(`^/dist ` + optAtGUser + ` ` + optAtGUser + `$`)

type MsgInterceptor struct{}

func (i MsgInterceptor) InterceptMsg(cmd *command.Command) {
	if cmd.Room.ReadOnly {
		if !cmd.Room.IsRoomOwner(cmd.AuthUser.ID) {
			cmd.Err = fmt.Errorf("room is read-only")
			return
		}
	}

	// Only check maximum length of message if we are uploading a file
	// Trim whitespaces and ensure minimum length
	minLen := utils.Ternary(cmd.Upload != nil, 0, minMsgLen)
	if !utils.ValidateRuneLength(strings.TrimSpace(cmd.Message), minLen, maxMsgLen) {
		cmd.DataMessage = cmd.OrigMessage
		cmd.Err = fmt.Errorf("%d - %d characters", minLen, maxMsgLen)
		return
	}

	html, taggedUsersIDsMap, err := dutils.ProcessRawMessage(cmd.DB, cmd.Message, cmd.RoomKey, cmd.AuthUser.ID, cmd.Room.ID, cmd.Upload, cmd.AuthUser.IsModerator(), cmd.AuthUser.CanUseMultiline, cmd.AuthUser.ManualMultiline)
	if err != nil {
		cmd.DataMessage = cmd.OrigMessage
		cmd.Err = err
		return
	}

	if len(strings.TrimSpace(html)) <= len("<p></p>") {
		cmd.DataMessage = cmd.OrigMessage
		cmd.Err = errors.New("empty message")
		return
	}

	pmUsername := dutils.DoParseUsernamePtr(cmd.C.QueryParam(command.RedirectPmUsernameQP))
	if pmUsername != nil {
		if err := cmd.SetToUser(*pmUsername); err != nil {
			cmd.Err = command.ErrRedirect
			return
		}
		cmd.HellbanMsg = false
		cmd.ModMsg = false
		cmd.SystemMsg = false
		cmd.GroupID = nil
	}

	toUserID := database.UserPtrID(cmd.ToUser)

	msgID, _ := cmd.DB.CreateOrEditMessage(cmd.EditMsg, html, cmd.OrigMessage, cmd.RoomKey, cmd.Room.ID, cmd.AuthUser.ID, toUserID, cmd.Upload, cmd.GroupID, cmd.HellbanMsg, cmd.ModMsg, cmd.SystemMsg)

	if !cmd.SkipInboxes {
		sendInboxes(cmd.DB, cmd.Room, cmd.AuthUser, cmd.ToUser, msgID, cmd.GroupID, html, cmd.ModMsg, taggedUsersIDsMap)
	}

	// Count public messages in #general room
	if cmd.Room.ID == config.GeneralRoomID && cmd.ToUser == nil {
		cmd.AuthUser.GeneralMessagesCount++
		generalRoomKarma(cmd.DB, cmd.AuthUser)
		cmd.AuthUser.DoSave(cmd.DB)
	}

	// Update chat read marker
	if cmd.EditMsg == nil {
		cmd.DB.UpdateChatReadMarker(cmd.AuthUser.ID, cmd.Room.ID)
	}

	// Update user activity
	isPM := cmd.ToUser != nil
	updateUserActivity(isPM, cmd.ModMsg, cmd.Room, cmd.AuthUser)
}

func generalRoomKarma(db *database.DkfDB, authUser *database.User) {
	// Hellban users ain't getting karma
	if authUser.IsHellbanned {
		return
	}
	messagesCount := authUser.GeneralMessagesCount
	if messagesCount%100 == 0 {
		description := fmt.Sprintf("sent %d messages", messagesCount)
		authUser.IncrKarma(db, 1, description)
	} else if messagesCount == 20 {
		authUser.IncrKarma(db, 1, "first 20 messages sent")
	}
}

func sendInboxes(db *database.DkfDB, room database.ChatRoom, authUser, toUser *database.User, msgID int64, groupID *database.GroupID, html string, modMsg bool,
	taggedUsersIDsMap map[database.UserID]database.User) {
	// Only have chat inbox for unencrypted messages
	if room.IsProtected() {
		return
	}
	// If user is hellbanned, do not send inboxes
	if authUser.IsHellbanned {
		return
	}
	// Early return if we don't need to send inboxes
	if toUser == nil && len(taggedUsersIDsMap) == 0 {
		return
	}

	blacklistedBy, _ := db.GetPmBlacklistedByUsers(authUser.ID)
	blacklistedBySet := utils.Slice2Set(blacklistedBy, func(el database.PmBlacklistedUsers) database.UserID { return el.UserID })

	ignoredBy, _ := db.GetIgnoredByUsers(authUser.ID)
	ignoredBySet := utils.Slice2Set(ignoredBy, func(el database.IgnoredUser) database.UserID { return el.UserID })

	sendInbox := func(user database.User, isPM, modCh bool) {
		if !managers.ActiveUsers.IsUserActiveInRoom(user.ID, room) || user.AFK {
			// Do not send notification if receiver is blacklisting you
			if blacklistedBySet.Contains(user.ID) {
				return
			}
			// Do not send notification if receiver is ignoring you
			if ignoredBySet.Contains(user.ID) {
				return
			}
			db.CreateInboxMessage(html, room.ID, authUser.ID, user.ID, isPM, modCh, &msgID)
		}
	}

	// If the message is a PM, only notify the receiver, not the tagged people in it.
	if toUser != nil {
		sendInbox(*toUser, true, false)
	} else if room.Name == "moderators" { // Only tags other moderators on "moderators" room
		for _, user := range taggedUsersIDsMap {
			if user.IsModerator() {
				sendInbox(user, false, false)
			}
		}
	} else if modMsg { // Only tags other moderators on /m messages
		for _, user := range taggedUsersIDsMap {
			if user.IsModerator() {
				sendInbox(user, false, true)
			}
		}
	} else if groupID != nil { // Only tags other people in the group
		for _, user := range taggedUsersIDsMap {
			if db.IsUserInGroupByID(user.ID, *groupID) {
				sendInbox(user, false, false)
			}
		}
	} else { // Otherwise, notify tagged people
		for _, user := range taggedUsersIDsMap {
			sendInbox(user, false, false)
		}
	}
}

func updateUserActivity(isPM, modMsg bool, room database.ChatRoom, authUser *database.User) {
	// We do not update user presence when they send private messages or moderators group message
	if isPM || modMsg {
		return
	}
	managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfoUpdateActivity(authUser))
}

func checkCPLinks(db *database.DkfDB, html string) bool {
	m1 := onionV3Rgx.FindAllStringSubmatch(html, -1)
	m2 := onionV2Rgx.FindAllStringSubmatch(html, -1)
	for _, m := range append(m1, m2...) {
		hash := utils.MD5([]byte(m[0]))
		if _, err := db.GetOnionBlacklist(hash); err == nil {
			return true
		}
	}
	return false
}
