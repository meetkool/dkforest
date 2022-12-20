package utils

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"fmt"
	"github.com/sirupsen/logrus"
)

func GetZeroUser() database.User {
	zeroUser, err := database.GetUserByUsername(config.NullUsername)
	if err != nil {
		logrus.Fatal(err)
	}
	return zeroUser
}

func SendNewChessGameMessages(key, roomKey string, roomID database.RoomID, zeroUser, player1, player2 database.User) {
	// Send game link to players
	getPlayerMsg := func(opponent database.User) (raw string, msg string) {
		raw = `Chess game against ` + opponent.Username
		msg = `<a href="/chess/` + key + `" rel="noopener noreferrer" target="_blank">Chess game against ` + opponent.Username + `</a>`
		return
	}
	raw, msg := getPlayerMsg(player2)
	_, _ = database.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &player1.ID)
	raw, msg = getPlayerMsg(player1)
	_, _ = database.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &player2.ID)

	// Send notifications to chess games subscribers
	raw = `Chess game: ` + player1.Username + ` VS ` + player2.Username
	msg = `<a href="/chess/` + key + `" rel="noopener noreferrer" target="_blank">Chess game: ` + player1.Username + ` VS ` + player2.Username + `</a>`
	users, _ := database.GetChessSubscribers()
	for _, user := range users {
		if user.ID == player1.ID || user.ID == player2.ID {
			continue
		}
		_, _ = database.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &user.ID)
	}
}

func ParseUserID(v string) (database.UserID, error) {
	p, err := utils.ParseInt64(v)
	if err != nil {
		return 0, err
	}
	return database.UserID(p), nil
}

func DoParseUserID(v string) (out database.UserID) {
	out, _ = ParseUserID(v)
	return
}

func Kick(kicked, kickedBy database.User, purge bool) {
	silent := kicked.IsHellbanned
	kick(kicked, kickedBy, silent, purge)
}

func SilentKick(kicked, kickedBy database.User) {
	kick(kicked, kickedBy, true, true)
}

func SelfKick(kicked database.User, silent bool) {
	kick(kicked, kicked, silent, true)
}

func kick(kicked, kickedBy database.User, silent, purge bool) {
	database.NewAudit(kickedBy, fmt.Sprintf("kick %s #%d", kicked.Username, kicked.ID))
	kicked.Verified = false
	kicked.DoSave()

	// Remove user from the user cache
	managers.ActiveUsers.RemoveUser(kicked.ID)

	if purge {
		// Purge user messages
		if err := database.DeleteUserChatMessages(kicked.ID); err != nil {
			logrus.Error(err)
		}
	}

	// If user is HB, do not display system message
	if !silent {
		// Display kick message
		database.CreateKickMsg(kicked, kickedBy)
	}
}
