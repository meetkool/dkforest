package command

import (
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"net/url"
)

var ErrRedirect = errors.New("redirect")
var ErrStop = errors.New("stop")

type ErrSuccess struct {
	msg string
}

func NewErrSuccess(msg string) *ErrSuccess {
	return &ErrSuccess{msg: msg}
}

func (e ErrSuccess) Error() string {
	return e.msg
}

const (
	RedirectPmQP         = "pm"
	RedirectEditQP       = "e"
	RedirectGroupQP      = "g"
	RedirectModQP        = "m"
	RedirectHbmQP        = "hbm"
	RedirectTagQP        = "tag"
	RedirectHTagQP       = "htag"
	RedirectMTagQP       = "mtag"
	RedirectQuoteQP      = "quote"
	RedirectMultilineQP  = "ml"
	RedirectPmUsernameQP = "pmusername"
)

type Command struct {
	Err error

	// Data that can be mutated
	RedirectQP  url.Values            // RedirectURL Query Parameters
	OrigMessage string                // This is the original text that the user input (can be changed by /e)
	DataMessage string                // This is what the user will have in his input box
	Message     string                // Un-sanitized message received from the user
	Room        database.ChatRoom     // Room the user is in
	RoomKey     string                // Room password (if any)
	AuthUser    *database.User        // Authenticated user (sender of the message)
	DB          *database.DkfDB       // Database instance
	ToUser      *database.User        // If not nil, will be a PM
	Upload      *database.Upload      // If the message contains an uploaded file
	EditMsg     *database.ChatMessage // If we're editing a message
	GroupID     *database.GroupID     // If the message is for a subgroup
	HellbanMsg  bool                  // Is the message will be marked HB
	SystemMsg   bool                  // Is the message system
	ModMsg      bool                  // Is the message part of the "moderators" group
	C           echo.Context
	SkipInboxes bool

	zeroUser *database.User // Cache the zero (@0) user
}

func NewCommand(c echo.Context, origMessage string, room database.ChatRoom, roomKey string) *Command {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	return &Command{
		C:           c,
		AuthUser:    authUser,
		DB:          db,
		HellbanMsg:  authUser.IsHellbanned,
		RedirectQP:  url.Values{},
		OrigMessage: origMessage,
		Message:     origMessage,
		Room:        room,
		RoomKey:     roomKey,
	}
}

func (c *Command) RedirectURL() string {
	return fmt.Sprintf("/api/v1/chat/top-bar/%s?%s", c.Room.Name, c.RedirectQP.Encode())
}

func (c *Command) SetToUser(username database.Username) (err error) {
	user, err := c.DB.GetUserByUsername(username)
	if err != nil {
		c.Err = errors.New("invalid username")
		return c.Err
	}

	c.SkipInboxes, c.Err = dutils.CanUserPmOther(c.DB, *c.AuthUser, user, c.Room.IsOwned())
	if c.Err != nil {
		return c.Err
	}
	c.ToUser = &user
	return nil
}

// GetZeroUser lazy loading and cache of the zero user
func (c *Command) GetZeroUser() database.User {
	if c.zeroUser == nil {
		zeroUser := dutils.GetZeroUser(c.DB)
		c.zeroUser = &zeroUser
	}
	return *c.zeroUser
}

// ZeroProcMsg have the "zero user" send a processed message to the authUser
func (c *Command) ZeroProcMsg(rawMsg string) {
	c.zeroProcMsgRoom(rawMsg, c.RoomKey, c.Room.ID)
}

// ZeroPublicProcMsgRoom have the "zero user" send a processed message in the specified room
func (c *Command) ZeroPublicProcMsgRoom(rawMsg, roomKey string, roomID database.RoomID) {
	c.zeroProcMsgRoomToUser(rawMsg, roomKey, roomID, nil)
}

// Have the "zero user" send a processed message to the authUser in the specified room
func (c *Command) zeroProcMsgRoom(rawMsg, roomKey string, roomID database.RoomID) {
	c.zeroProcMsgRoomToUser(rawMsg, roomKey, roomID, c.AuthUser)
}

// Have the "zero user" send a "processed message" PM to a user in a specific room.
func (c *Command) zeroProcMsgRoomToUser(rawMsg, roomKey string, roomID database.RoomID, toUser *database.User) {
	procMsg, _, _ := dutils.ProcessRawMessage(c.DB, rawMsg, roomKey, c.AuthUser.ID, roomID, nil, c.AuthUser.IsModerator(), true, false)
	c.zeroRawMsg(toUser, rawMsg, procMsg)
}

// ZeroMsg have the "zero usser" send an unprocessed private message to the authUser
func (c *Command) ZeroMsg(msg string) {
	c.zeroRawMsg(c.AuthUser, msg, msg)
}

func (c *Command) ZeroSysMsgTo(user2 *database.User, msg string) {
	c.zeroSysRawMsg(user2, msg, msg, false)
}

func (c *Command) ZeroSysMsgToSkipNotify(user2 *database.User, msg string) {
	c.zeroSysRawMsg(user2, msg, msg, true)
}

// ZeroPublicMsg have the "zero usser" send an unprocessed message in the current room
func (c *Command) ZeroPublicMsg(raw, msg string) {
	c.zeroRawMsg(nil, raw, msg)
}

func (c *Command) zeroRawMsg(user2 *database.User, raw, msg string) {
	zeroUser := c.GetZeroUser()
	c.rawMsg(zeroUser, user2, raw, msg)
}

func (c *Command) zeroSysRawMsg(user2 *database.User, raw, msg string, skipNotify bool) {
	zeroUser := c.GetZeroUser()
	c.rawSysMsg(zeroUser, user2, raw, msg, skipNotify)
}

func (c *Command) rawMsg(user1 database.User, user2 *database.User, raw, msg string) {
	if c.Room.ReadOnly {
		return
	}
	rawMsgRoom(c.DB, user1, user2, raw, msg, c.RoomKey, c.Room.ID)
}

func (c *Command) rawSysMsg(user1 database.User, user2 *database.User, raw, msg string, skipNotify bool) {
	if c.Room.ReadOnly {
		return
	}
	rawSysMsgRoom(c.DB, user1, user2, raw, msg, c.RoomKey, c.Room.ID, skipNotify)
}

func rawMsgRoom(db *database.DkfDB, user1 database.User, user2 *database.User, raw, msg, roomKey string, roomID database.RoomID) {
	var toUserID *database.UserID
	if user2 != nil {
		toUserID = &user2.ID
	}
	_, _ = db.CreateMsg(raw, msg, roomKey, roomID, user1.ID, toUserID)
}

func rawSysMsgRoom(db *database.DkfDB, user1 database.User, user2 *database.User, raw, msg, roomKey string, roomID database.RoomID, skipNotify bool) {
	var toUserID *database.UserID
	if user2 != nil {
		toUserID = &user2.ID
	}
	_ = db.CreateSysMsgPM(raw, msg, roomKey, roomID, user1.ID, toUserID, skipNotify)
}
