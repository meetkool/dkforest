package v1

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"fmt"
	html2 "html"
	"strings"
	"time"
)

type MsgInterceptor struct{}

func (i MsgInterceptor) InterceptMsg(cmd *Command) {
	// Only check length of message if we're not uploading a file
	// Trim whitespaces and ensure minimum length
	if cmd.upload == nil && !utils.ValidateRuneLength(strings.TrimSpace(cmd.message), minMsgLen, maxMsgLen) {
		cmd.dataMessage = cmd.origMessage
		cmd.err = fmt.Errorf("%d - %d characters", minMsgLen, maxMsgLen)
		return
	}

	html, taggedUsersIDsMap := ProcessRawMessage(cmd.message, cmd.roomKey, cmd.authUser.ID, cmd.room.ID, cmd.upload)

	toUserID := database.UserPtrID(cmd.toUser)

	msgID, _ := database.CreateOrEditMessage(cmd.editMsg, html, cmd.origMessage, cmd.roomKey, cmd.room.ID, cmd.fromUserID, toUserID, cmd.upload, cmd.groupID, cmd.hellbanMsg, cmd.modMsg, cmd.systemMsg)

	if !cmd.skipInboxes {
		sendInboxes(cmd.room, cmd.authUser, cmd.toUser, msgID, cmd.groupID, html, cmd.modMsg, taggedUsersIDsMap)
	}

	// Count public messages in #general room
	if cmd.room.ID == config.GeneralRoomID && cmd.toUser == nil {
		cmd.authUser.GeneralMessagesCount++
		generalRoomKarma(cmd.authUser)
		cmd.authUser.DoSave()
	}

	// Update chat read marker
	database.UpdateChatReadMarker(cmd.authUser.ID, cmd.room.ID)

	// Update user activity
	isPM := cmd.toUser != nil
	updateUserActivity(isPM, cmd.room, cmd.authUser)
}

func generalRoomKarma(authUser *database.User) {
	// Hellban users ain't getting karma
	if authUser.IsHellbanned {
		return
	}
	messagesCount := authUser.GeneralMessagesCount
	if messagesCount%100 == 0 {
		description := fmt.Sprintf("sent %d messages", messagesCount)
		authUser.IncrKarma(1, description)
	} else if messagesCount == 20 {
		authUser.IncrKarma(1, "first 20 messages sent")
	}
}

// ProcessRawMessage return the new html, and a map of tagged users used for notifications
// This function takes an "unsafe" user input "in", and return html which will be safe to render.
func ProcessRawMessage(in, roomKey string, authUserID database.UserID, roomID database.RoomID, upload *database.Upload) (string, map[database.UserID]database.User) {
	html, quoted := convertQuote(in, roomKey, roomID) // Get raw quote text which is not safe to render
	html = html2.EscapeString(html)                   // Makes user input safe to render
	// All html generated from this point on shall be safe to render.
	html = convertPGPMessageToFile(html, authUserID)
	html = convertPGPPublicKeyToFile(html, authUserID)
	html = convertAgeMessageToFile(html, authUserID)
	html = convertLinksWithoutScheme(html)
	html = convertMarkdown(html)
	html = convertBangShortcuts(html)
	html = convertArchiveLinks(html, roomID, authUserID)
	html = convertLinks(html)
	html = linkDefaultRooms(html)
	html, taggedUsersIDsMap := colorifyTaggedUsers(html, database.GetUsersByUsername)
	html = linkRoomTags(html)
	html = emojiReplacer.Replace(html)
	html = styleQuote(html, quoted)
	html = appendUploadLink(html, upload)
	return html, taggedUsersIDsMap
}

func sendInboxes(room database.ChatRoom, authUser, toUser *database.User, msgID int64, groupID *database.GroupID, html string, modMsg bool,
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

	blacklistedBy, _ := database.GetPmBlacklistedByUsers(authUser.ID)
	blacklistedByMap := make(map[database.UserID]struct{})
	for _, b := range blacklistedBy {
		blacklistedByMap[b.UserID] = struct{}{}
	}

	ignoredBy, _ := database.GetIgnoredByUsers(authUser.ID)
	ignoredByMap := make(map[database.UserID]struct{})
	for _, b := range ignoredBy {
		ignoredByMap[b.UserID] = struct{}{}
	}

	sendInbox := func(user database.User, isPM, modCh bool) {
		if !managers.ActiveUsers.IsUserActiveInRoom(user.ID, room) || user.AFK {
			// Do not send notification if receiver is blacklisting you
			if _, ok := blacklistedByMap[user.ID]; ok {
				return
			}
			// Do not send notification if receiver is ignoring you
			if _, ok := ignoredByMap[user.ID]; ok {
				return
			}
			database.CreateInboxMessage(html, room.ID, authUser.ID, user.ID, isPM, modCh, &msgID)
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
			if database.IsUserInGroupByID(user.ID, *groupID) {
				sendInbox(user, false, false)
			}
		}
	} else { // Otherwise, notify tagged people
		for _, user := range taggedUsersIDsMap {
			sendInbox(user, false, false)
		}
	}
}

func updateUserActivity(isPM bool, room database.ChatRoom, authUser *database.User) {
	// We do not update user presence when they send private messages
	if isPM {
		return
	}
	now := time.Now()
	managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfo(*authUser, &now))
}
