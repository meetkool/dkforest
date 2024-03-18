package command

import (
	"dkforest/pkg/database"
	"errors"
	"net/url"
)

var (
	ErrRedirect      = errors.New("redirect")
	ErrStop          = errors.New("stop")
	RedirectPmQP     = "pm"
	RedirectEditQP   = "e"
	RedirectGroupQP  = "g"
	RedirectModQP    = "m"
	RedirectHbmQP    = "hbm"
	RedirectTagQP    = "tag"
	RedirectHTagQP   = "htag"
	RedirectMTagQP   = "mtag"
	RedirectQuoteQP  = "quote"
	RedirectMultilineQP = "ml"
	RedirectPmUsernameQP = "pmusername"
)

type ErrSuccess struct {
	msg string
}

func NewErrSuccess(msg string) *ErrSuccess {
	return &ErrSuccess{msg: msg}
}

func (e ErrSuccess) Error() string {
	return e.msg
}

type Command struct {
	RedirectQP          url.Values            // RedirectURL Query Parameters
	OrigMessage         string                // This is the original text that the user input (can be changed by /e)
	DataMessage         string                // This is what the user will have in his input box
	Message             string                // Un-sanitized message received from the user
	Room                database.ChatRoom     // Room the user is in
	RoomKey             string                // Room password (if any)
	AuthUser            *database.User        // Authenticated user (sender of the message)
	DB                  *database.DkfDB       // Database instance
	ToUser              *database.User        // If not nil, will be a PM
	Upload              *database.Upload      // If the message contains an uploaded file
	EditMsg             *database.ChatMessage // If we're editing a message
	GroupID             *database.GroupID     // If the message is for a subgroup
	HellbanMsg          bool                  // Is the message will be marked HB
	SystemMsg           bool                  // Is the message system
	ModMsg              bool                  // Is the message part of the "moderators" group
	C                   echo.Context
	SkipInboxes         bool

	zeroUser *database.User // Cache the zero (@0) user
}

func NewCommand(c echo.Context, origMessage string, room database.ChatRoom, roomKey string) *Command {
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)
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
func (c *Command) ZeroProcMsgRoomToUser(rawMsg, roomKey string, roomID database.RoomID, toUser *database
