package utils

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
)

func GetZeroUser(db *database.DkfDB) database.User {
	zeroUser, err := db.GetUserByUsername(config.NullUsername)
	if err != nil {
		logrus.Fatal(err)
	}
	return zeroUser
}

func ZeroSendMsg(db *database.DkfDB, recipientID database.UserID, msg string) {
	zeroUser := GetZeroUser(db)
	_, _ = db.CreateMsg(msg, msg, "", config.GeneralRoomID, zeroUser.ID, &recipientID)
}

func RootAdminNotify(db *database.DkfDB, msg string) {
	rootAdminID := database.UserID(config.RootAdminID)
	ZeroSendMsg(db, rootAdminID, msg)
}

func SendNewChessGameMessages(db *database.DkfDB, key, roomKey string, roomID database.RoomID, zeroUser, player1, player2 database.User) {
	// Send game link to players
	getPlayerMsg := func(opponent database.User) (raw string, msg string) {
		raw = `Chess game against ` + string(opponent.Username)
		msg = `<a href="/chess/` + key + `" rel="noopener noreferrer" target="_blank">Chess game against ` + string(opponent.Username) + `</a>`
		return
	}
	raw, msg := getPlayerMsg(player2)
	_, _ = db.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &player1.ID)
	raw, msg = getPlayerMsg(player1)
	_, _ = db.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &player2.ID)

	// Send notifications to chess games subscribers
	raw = `Chess game: ` + string(player1.Username) + ` VS ` + string(player2.Username)
	msg = `<a href="/chess/` + key + `" rel="noopener noreferrer" target="_blank">Chess game: ` + string(player1.Username) + ` VS ` + string(player2.Username) + `</a>`

	activeUsers := managers.ActiveUsers.GetActiveUsers()
	activeUsersIDs := make([]database.UserID, len(activeUsers))
	for idx, activeUser := range activeUsers {
		activeUsersIDs[idx] = activeUser.UserID
	}

	users, _ := db.GetOnlineChessSubscribers(activeUsersIDs)
	for _, user := range users {
		if user.ID == player1.ID || user.ID == player2.ID {
			continue
		}
		// Make a copy of user ID, otherwise next iteration will overwrite the pointer
		// and change data that was sent previously in the pubsub later on
		userID := user.ID
		_, _ = db.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &userID)
	}
}

func DoParseUsernamePtr(v string) *database.Username {
	if v == "" {
		return nil
	}
	username := database.Username(v)
	return &username
}

func GetUserIDFromUsername(db *database.DkfDB, u string) *database.UserID {
	username := DoParseUsernamePtr(u)
	if username == nil {
		return nil
	}
	userID, err := db.GetUserIDByUsername(*username)
	if err != nil {
		return nil
	}
	return &userID
}

func DoParsePmDisplayMode(v string) database.PmDisplayMode {
	p, err := utils.ParseInt64(v)
	if err != nil {
		return database.PmNoFilter
	}
	switch p {
	case 1:
		return database.PmOnly
	case 2:
		return database.PmNone
	default:
		return database.PmNoFilter
	}
}

func Parse[T ~int64](v string) (out T, err error) {
	p, err := utils.ParseInt64(v)
	if err != nil {
		return out, err
	}
	return T(p), nil
}

func DoParse[T ~int64](v string) (out T) {
	out, _ = Parse[T](v)
	return
}

func ParseUserID(v string) (database.UserID, error) {
	return Parse[database.UserID](v)
}

func DoParseUserID(v string) (out database.UserID) {
	return DoParse[database.UserID](v)
}

func ParseRoomID(v string) (database.RoomID, error) {
	return Parse[database.RoomID](v)
}

func DoParseRoomID(v string) (out database.RoomID) {
	return DoParse[database.RoomID](v)
}

func SelfHellBan(db *database.DkfDB, user *database.User) {
	db.NewAudit(*user, fmt.Sprintf("hellban %s #%d", user.Username, user.ID))
	user.HellBan(db)
	managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(user))
}

func Kick(db *database.DkfDB, kicked, kickedBy database.User, purge, silent bool) error {
	if kicked.IsHellbanned {
		silent = true
	}
	return kick(db, kicked, kickedBy, silent, purge)
}

func SilentKick(db *database.DkfDB, kicked, kickedBy database.User) error {
	return kick(db, kicked, kickedBy, true, true)
}

func SelfKick(db *database.DkfDB, kicked database.User, silent bool) error {
	return kick(db, kicked, kicked, silent, true)
}

func kick(db *database.DkfDB, kicked, kickedBy database.User, silent, purge bool) error {
	if !kicked.Verified {
		return errors.New("user already kicked")
	}
	// Can't kick a vetted user (unless admin)
	if !kickedBy.IsAdmin && kicked.Vetted {
		return errors.New("cannot kick a vetted user")
	}
	// Can't kick another moderator (unless admin)
	if !kickedBy.IsAdmin && kicked.IsModerator() {
		return errors.New("cannot kick another moderator")
	}
	// Can't kick yourself as a moderator/admin
	if (kicked.IsAdmin || kicked.IsModerator()) && kickedBy.ID == kicked.ID {
		return errors.New("cannot kick yourself")
	}

	db.NewAudit(kickedBy, fmt.Sprintf("kick %s #%d", kicked.Username, kicked.ID))
	kicked.SetVerified(db, false)

	// Remove user from the user cache
	managers.ActiveUsers.RemoveUser(kicked.ID)

	if purge {
		// Purge user messages
		if err := db.DeleteUserChatMessages(kicked.ID); err != nil {
			logrus.Error(err)
		}
		database.MsgPubSub.Pub(database.RefreshTopic, database.ChatMessageType{Typ: database.ForceRefresh})
	} else {
		database.MsgPubSub.Pub("refresh_"+string(kicked.Username), database.ChatMessageType{Typ: database.ForceRefresh})
	}

	// If user is HB, do not display system message
	if !silent {
		// Display kick message
		db.CreateKickMsg(kicked, kickedBy)
	}

	return nil
}

func GetRoomAndKey(db *database.DkfDB, c echo.Context, roomName string) (database.ChatRoom, string, error) {
	roomKey := ""
	room, err := db.GetChatRoomByName(roomName)
	if err != nil {
		return room, roomKey, errors.New("room not found")
	}
	hasAccess, roomKey := room.HasAccess(c)
	if !hasAccess {
		return room, roomKey, errors.New("forbidden")
	}
	return room, roomKey, nil
}

var ErrPMDenied = errors.New("you cannot pm/inbox this user")
var Err20Msgs = errors.New("you need 20 public messages to unlock PMs/Inbox; or be whitelisted")
var ErrOther20Msgs = errors.New("dest user must be whitelisted or have 20 public messages")

func CanUserPmOther(db *database.DkfDB, user, other database.User, roomIsPrivate bool) (skipInbox bool, err error) {
	errPMDenied := ErrPMDenied

	if user.ID == other.ID {
		return false, errors.New("cannot /pm yourself")
	}

	if db.IsUserPmWhitelisted(user.ID, other.ID) {
		return false, nil
	}

	switch other.PmMode {
	case database.PmModeWhitelist:
		// We are in whitelist mode, and user is not whitelisted
		return false, errPMDenied

	case database.PmModeStandard:
		if !user.CanSendPM() {
			// In private rooms, can send PM but inboxes will be skipped if not enough public messages
			if roomIsPrivate {
				return true, nil
			}
			// Need at least 20 public messages to send PM in a public room
			return false, Err20Msgs
		}

		// User on blacklist cannot PM/Inbox
		if db.IsUserPmBlacklisted(user.ID, other.ID) {
			return false, errPMDenied
		}
		// Other doesn't want PM from new users
		if !user.AccountOldEnough() && other.BlockNewUsersPm {
			return false, errPMDenied
		}

		if !other.CanSendPM() {
			if db.IsUserPmWhitelisted(other.ID, user.ID) {
				return true, nil
			}
			// In private rooms, can send PM but inboxes will be skipped if not enough public messages
			if roomIsPrivate {
				return true, nil
			}
			return false, ErrOther20Msgs
		}

		return false, nil
	}

	// Should never go here
	return false, nil
}

// VerifyMsgAuth returns either or not authUser is allowed to see msg
func VerifyMsgAuth(db *database.DkfDB, msg *database.ChatMessage, authUserID database.UserID, isModerator bool) bool {
	// Verify moderators channel authorization
	if msg.Moderators && !isModerator {
		return false
	}
	// Verify group authorization
	if msg.GroupID != nil {
		userGroupsIDs, _ := db.GetUserRoomGroupsIDs(authUserID, msg.RoomID)
		if !utils.InArr(*msg.GroupID, userGroupsIDs) {
			return false
		}
	}
	// verify PM authorization
	if msg.IsPm() {
		if msg.UserID != authUserID && *msg.ToUserID != authUserID {
			return false
		}
	}
	return true
}
