package utils

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

// GetZeroUser returns the zero user from the database.
func GetZeroUser(db *database.DkfDB) database.User {
	zeroUser, err := db.GetUserByUsername(config.NullUsername)
	if err != nil {
		logrus.Fatal(err)
	}
	return zeroUser
}

// RootAdminNotify sends a notification message to the root admin.
func RootAdminNotify(db *database.DkfDB, msg string) {
	zeroUser := GetZeroUser(db)
	rootAdminID := database.UserID(config.RootAdminID)
	_, err := db.CreateMsg(msg, msg, "", config.GeneralRoomID, zeroUser.ID, &rootAdminID)
	if err != nil {
		logrus.Error(err)
	}
}

// SendNewChessGameMessages sends messages to the players and subscribers of a new chess game.
func SendNewChessGameMessages(db *database.DkfDB, key, roomKey string, roomID database.RoomID, zeroUser, player1, player2 database.User) {
	// Send game link to players
	getPlayerMsg := func(opponent database.User) (raw string, msg string) {
		raw = `Chess game against ` + opponent.Username
		msg = fmt.Sprintf("<a href='/chess/%s' rel='noopener noreferrer' target='_blank'>Chess game against %s</a>", key, opponent.Username)
		return
	}
	raw, msg := getPlayerMsg(player2)
	_, err := db.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &player1.ID)
	if err != nil {
		logrus.Error(err)
	}
	raw, msg = getPlayerMsg(player1)
	_, err = db.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &player2.ID)
	if err != nil {
		logrus.Error(err)
	}

	// Send notifications to chess games subscribers
	raw = fmt.Sprintf(`Chess game: %s VS %s`, player1.Username, player2.Username)
	msg = fmt.Sprintf("<a href='/chess/%s' rel='noopener noreferrer' target='_blank'>Chess game: %s VS %s</a>", key, player1.Username, player2.Username)

	activeUsers := managers.ActiveUsers.GetActiveUsers()
	activeUsersIDs := make([]database.UserID, len(activeUsers))
	for i, activeUser := range activeUsers {
		activeUsersIDs[i] = activeUser.UserID
	}

	users, err := db.GetOnlineChessSubscribers(activeUsersIDs)
	if err != nil {
		logrus.Error(err)
		return
	}

	for _, user := range users {
		if user.ID == player1.ID || user.ID == player2.ID {
			continue
		}
		_, err = db.CreateMsg(raw, msg, roomKey, roomID, zeroUser.ID, &user.ID)
		if err != nil {
			logrus.Error(err)
		}
	}
}

// DoParseUsernamePtr returns a pointer to a parsed username, or nil if the input is empty.
func DoParseUsernamePtr(v string) *database.Username {
	if v == "" {
		return nil
	}
	username := database.Username(v)
	return &username
}

// GetUserIDFromUsername returns a pointer to a parsed user ID, or nil if the input is invalid.
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

// DoParsePmDisplayMode returns a parsed PM display mode.
func DoParsePmDisplayMode(v string) database.PmDisplayMode {
	p, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return database.PmNoFilter
	}
	switch p {
	case 1:
		return database.PmOnly
	case 2:
		return database.PmNone
	default:
		return database.P
