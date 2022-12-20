package v1

import (
	"dkforest/pkg/clockwork"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/hashset"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"errors"
	"fmt"
	"github.com/Depado/bfchroma"
	chtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/labstack/echo"
	"github.com/microcosm-cc/bluemonday"
	bf "github.com/russross/blackfriday/v2"
	html2 "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	agePrefix     = "-----BEGIN AGE ENCRYPTED FILE-----"
	ageSuffix     = "-----END AGE ENCRYPTED FILE-----"
	pgpPrefix     = "-----BEGIN PGP MESSAGE-----"
	pgpSuffix     = "-----END PGP MESSAGE-----"
	pgpPKeyPrefix = "-----BEGIN PGP PUBLIC KEY BLOCK-----"
	pgpPKeySuffix = "-----END PGP PUBLIC KEY BLOCK-----"
)

var linkRgx = regexp.MustCompile(`(http|ftp|https):\/\/([\w\-_]+(?:(?:\.[\w\-_]+)+))([\w\-\.,@?^=%&amp;:/~\+#]*[\w\-\@?^=%&amp;/~\+#])?`)

var emojiReplacer = strings.NewReplacer(
	":):", `<span class="emoji" style="background-position: -54px -0px;" title=":):"></span>`,
	":smile:", `<span class="emoji" style="background-position: -54px -0px;" title=":smile:"></span>`,
	":happy:", `<span class="emoji" style="background-position: -18px -0px;" title=":happy:"></span>`,
	":see-no-evil:", `<span class="emoji" style="background-position: -54px -54px;" title=":see-no-evil:"></span>`,
	":hear-no-evil:", `<span class="emoji" style="background-position: -72px -54px;" title=":hear-no-evil:"></span>`,
	":speak-no-evil:", `<span class="emoji" style="background-position: -90px -54px;" title=":speak-no-evil:"></span>`,
	":poop:", `<span class="emoji" style="background-position: -144px -54px;" title=":poop:"></span>`,
	":+1:", `<span class="emoji" style="background-position: -432px -54px;" title=":+1:"></span>`,
	":evil:", `<span class="emoji" style="background-position: -360px -18px;" title=":evil:"></span>`,
	":cat-happy:", `<span class="emoji" style="background-position: -360px -36px;" title=":cat-happy:"></span>`,
	":eyes:", `<span class="emoji" style="background-position: -360px -54px;" title=":eyes:"></span>`,
	":wave:", `<span class="emoji" style="background-position: -54px -72px;" title=":wave:"></span>`,
	":clap:", `<span class="emoji" style="background-position: -234px -72px;" title=":clap:"></span>`,
	":fire:", `<span class="emoji" style="background-position: -162px -54px;" title=":fire:"></span>`,
	":sparkles:", `<span class="emoji" style="background-position: -180px -54px;" title=":sparkles:"></span>`,
	":sweat:", `<span class="emoji" style="background-position: -270px -54px;" title=":sweat:"></span>`,
	":heart:", `<span class="emoji" style="background-position: -180px -108px;" title=":heart:"></span>`,
	":broken-heart:", `<span class="emoji" style="background-position: -198px -108px;" title=":broken-heart:"></span>`,
	":zzz:", `<span class="emoji" style="background-position: -306px -54px;" title=":zzz:"></span>`,
	":praise:", `<span class="emoji" style="background-position: -180px -72px;" title=":praise:"></span>`,
	":joy:", `<span class="emoji" style="background-position: -396px -0px;" title=":joy:"></span>`,
	":sob:", `<span class="emoji" style="background-position: -414px -0px;" title=":joy:"></span>`,
	":scream:", `<span class="emoji" style="background-position: -90px -18px;" title=":scream:"></span>`,
	":heart-eyes:", `<span class="emoji" style="background-position: -108px -0px;" title=":heart-eyes:"></span>`,
	":blush:", `<span class="emoji" style="background-position: -72px -0px;" title=":blush:"></span>`,
	":crazy:", `<span class="emoji" style="background-position: -198px -0px;" title=":crazy:"></span>`,
	":angry:", `<span class="emoji" style="background-position: -126px -18px;" title=":angry:"></span>`,
	":triumph:", `<span class="emoji" style="background-position: -144px -18px;" title=":triumph:"></span>`,
	":skull:", `<span class="emoji" style="background-position: -108px -54px;" title=":skull:"></span>`,
	":alien:", `<span class="emoji" style="background-position: -126px -54px;" title=":alien:"></span>`,
	":sleeping:", `<span class="emoji" style="background-position: -252px -18px;" title=":sleeping:"></span>`,
	":tongue:", `<span class="emoji" style="background-position: -234px -0px;" title=":tongue:"></span>`,
	":cool:", `<span class="emoji" style="background-position: -234px -18px;" title=":cool:"></span>`,
	":wink:", `<span class="emoji" style="background-position: -90px -0px;" title=":wink:"></span>`,
	":happy-sweat:", `<span class="emoji" style="background-position: -0px -18px;" title=":happy-sweat:"></span>`,
	":shrug:", `¯\_(ツ)_/¯`,
	":flip:", `(╯°□°)╯︵ ┻━┻`,
	":flip-all:", `┻━┻︵ \(°□°)/ ︵ ┻━┻`,
	":fix-table:", `(ヘ･_･)ヘ┳━┳`,
	":disap:", `ಠ_ಠ`,
	":fox:", `🦊`,
	":popcorn:", `🍿`,
)

var ErrRedirect = errors.New("redirect")
var ErrStop = errors.New("stop")

const minMsgLen = 1
const maxMsgLen = 10000

const (
	redirectPmQP        = "pm"
	redirectEditQP      = "e"
	redirectGroupQP     = "g"
	redirectModQP       = "m"
	redirectHbmQP       = "hbm"
	redirectTagQP       = "tag"
	redirectHTagQP      = "htag"
	redirectMTagQP      = "mtag"
	redirectQuoteQP     = "quote"
	redirectMultilineQP = "ml"
)

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
	if room.OwnerUserID != nil {
		commandsList = append(commandsList, "/mode")
		commandsList = append(commandsList, "/wl")
	}
	// Private room owner
	if room.OwnerUserID != nil && *room.OwnerUserID == authUser.ID {
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
	var data chatTopBarData
	data.RoomName = c.Param("roomName")
	if data.RoomName == "battleship" {
		data.RoomName = "general"
	} else if data.RoomName == "chess" {
		data.RoomName = "general"
	}
	pm := c.QueryParam(redirectPmQP)
	edit := c.QueryParam(redirectEditQP)
	group := c.QueryParam(redirectGroupQP)
	mod := c.QueryParam(redirectModQP)
	hbm := c.QueryParam(redirectHbmQP)
	tag := c.QueryParam(redirectTagQP)
	htag := c.QueryParam(redirectHTagQP)
	mtag := c.QueryParam(redirectMTagQP)
	quote := c.QueryParam(redirectQuoteQP)

	queryParams := c.QueryParams()
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

	room, err := database.GetChatRoomByName(data.RoomName)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	if !room.HasAccess(c) {
		return c.NoContent(http.StatusForbidden)
	}

	// If the tutorial is not completed, just render the chat top-bar, no matter what.
	if (room.ID < 5 || (room.IsListed && !room.IsProtected())) && !authUser.TutorialCompleted() {
		return c.Render(http.StatusOK, "chat-top-bar", data)
	}

	roomKey := ""
	if room.IsProtected() {
		key, err := hutils.GetRoomKeyCookie(c, int64(room.ID))
		if err != nil {
			return c.NoContent(http.StatusForbidden)
		}
		roomKey = key.Value
	}

	baseRedirectURL := "/api/v1/chat/top-bar/" + room.Name

	if pm != "" {
		data.Message = "/pm " + pm + " "
	} else if hbm != "" {
		data.Message = "/hbm "
	} else if mod != "" {
		data.Message = "/m "
	} else if group != "" {
		data.Message = "/g " + group + " "
	} else if tag != "" {
		data.Message = "@" + tag + " "
	} else if htag != "" {
		data.Message = "/hbm @" + htag + " "
	} else if mtag != "" {
		data.Message = "/m @" + mtag + " "
	} else if edit != "" {
		data.Message, err = handleGetEdit(edit, roomKey, room, authUser)
		if err != nil {
			return c.Redirect(http.StatusFound, baseRedirectURL)
		}
	} else if quote != "" {
		data.Message, err = handleGetQuote(quote, roomKey, room, authUser)
		if err != nil {
			return c.Redirect(http.StatusFound, baseRedirectURL)
		}
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

	if c.Request().ContentLength > 30<<20 {
		data.Error = "The maximum file size is 30 MB"
		return c.Render(http.StatusOK, "chat-top-bar", data)
	}

	data.Message = ""
	origMessage := c.Request().PostFormValue("message")

	cmd := &Command{
		redirectQP:  redirectQP,
		dataMessage: data.Message,
		origMessage: origMessage,
		message:     origMessage,
		room:        room,
		roomKey:     roomKey,
		authUser:    authUser,
		fromUserID:  authUser.ID,
		hellbanMsg:  authUser.IsHellbanned,
		c:           c,
	}

	type Interceptor interface {
		InterceptMsg(*Command)
	}

	interceptors := []Interceptor{
		SnippetInterceptor{},
		SpamInterceptor{},
		ChessInstance,
		BattleshipInstance,
		WWInstance,
		BangInterceptor{},
		SlashInterceptor{},
		UploadInterceptor{},
		MsgInterceptor{},
	}
	for _, interceptor := range interceptors {
		interceptor.InterceptMsg(cmd)
		data.Message = cmd.dataMessage
		if cmd.err != nil {
			return handleCmdError(cmd.err, c, data, cmd.redirectURL(), cmd.origMessage)
		}
	}

	return c.Redirect(http.StatusFound, cmd.redirectURL())
}

func handleCmdError(err error, ctx echo.Context, data chatTopBarData, redirectURL, origMessage string) error {
	if err == ErrRedirect {
		return ctx.Redirect(http.StatusFound, redirectURL)
	} else if err == ErrStop {
		return ctx.Render(http.StatusOK, "chat-top-bar", data)
	} else if serr, ok := err.(*ErrSuccess); ok {
		data.Success = serr.Error()
		return ctx.Render(http.StatusOK, "chat-top-bar", data)
	}
	data.Message = origMessage
	data.Error = err.Error()
	return ctx.Render(http.StatusOK, "chat-top-bar", data)
}

func MyRenderer() *Renderer {
	// Defines the HTML rendering flags that are used
	var flags = bf.UseXHTML

	r := &Renderer{
		Base: bfchroma.NewRenderer(
			bfchroma.WithoutAutodetect(),
			bfchroma.ChromaOptions(
				chtml.WithLineNumbers(false),
				chtml.LineNumbersInTable(false),
			),
			bfchroma.Extend(
				bf.NewHTMLRenderer(bf.HTMLRendererParameters{
					Flags: flags,
				}),
			),
		),
	}
	return r
}

type Renderer struct {
	Base *bfchroma.Renderer
}

func (r Renderer) RenderNode(w io.Writer, node *bf.Node, entering bool) bf.WalkStatus {
	switch node.Type {
	case bf.Text:
		if node.Parent.Type != bf.Link {
			node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
		}
	case bf.Code:
		node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
	case bf.CodeBlock:
		node.Literal = []byte(html2.UnescapeString(string(node.Literal)))
	}
	return r.Base.RenderNode(w, node, entering)
}

func (r Renderer) RenderHeader(w io.Writer, ast *bf.Node) {
	r.Base.RenderHeader(w, ast)
}

func (r Renderer) RenderFooter(w io.Writer, ast *bf.Node) {
	r.Base.RenderFooter(w, ast)
}

func convertMarkdown(in string) string {
	out := strings.Replace(in, "\r", "", -1)
	resBytes := bf.Run([]byte(out), bf.WithRenderer(MyRenderer()), bf.WithExtensions(
		bf.NoIntraEmphasis|bf.Tables|bf.FencedCode|
			bf.Strikethrough|bf.SpaceHeadings|
			bf.DefinitionLists|bf.HardLineBreak|bf.NoLink))
	out = string(resBytes)
	return out
}

func replTextPrefixSuffix(msg, prefix, suffix, repl string) (out string) {
	out = msg
	pgpPIdx := strings.Index(msg, prefix)
	pgpSIdx := strings.Index(msg, suffix)
	if pgpPIdx != -1 && pgpSIdx != -1 {
		newMsg := msg[:pgpPIdx]
		newMsg += repl
		newMsg += msg[pgpSIdx+len(suffix):]
		out = newMsg
	}
	return
}

func handleGetQuote(msgUUID, roomKey string, room database.ChatRoom, authUser *database.User) (dataMessage string, err error) {
	quoted, err := database.GetRoomChatMessageByUUID(room.ID, msgUUID)
	if err != nil {
		return
	}

	// Build prefix for /m | /pm | /g | /hbm
	prefix := ""
	if quoted.ToUserID != nil {
		toUsername := quoted.User.Username
		if quoted.UserID == authUser.ID {
			toUsername = quoted.ToUser.Username
		}
		prefix = fmt.Sprintf(`/pm %s `, toUsername)
	} else if quoted.GroupID != nil {
		prefix = fmt.Sprintf(`/g %s `, quoted.Group.Name)
	} else if quoted.Moderators {
		prefix = fmt.Sprintf(`/m `)
	} else if (quoted.IsHellbanned || quoted.User.IsHellbanned) && authUser.IsModerator() {
		prefix = fmt.Sprintf(`/hbm `)
	}

	// Append the actual quoted text
	dataMessage = prefix + getQuoteTxt(roomKey, quoted) + " "
	return
}

func handleGetEdit(hourMinSec, roomKey string, room database.ChatRoom, authUser *database.User) (dataMessage string, err error) {
	if dt, err := utils.ParsePrevDatetimeAt(hourMinSec, clockwork.NewRealClock()); err == nil {
		if time.Since(dt) <= config.EditMessageTimeLimit {
			if msg, err := database.GetRoomChatMessageByDate(room.ID, authUser.ID, dt.UTC()); err == nil {
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

type Command struct {
	err error

	// Data that can be mutated
	redirectQP  url.Values            // RedirectURL Query Parameters
	origMessage string                // This is the original text that the user input (can be changed by /e)
	dataMessage string                // This is what the user will have in his input box
	message     string                // Un-sanitized message received from the user
	room        database.ChatRoom     // Room the user is in
	roomKey     string                // Room password (if any)
	authUser    *database.User        // Authenticated user
	fromUserID  database.UserID       // Sender of message
	toUser      *database.User        // If not nil, will be a PM
	upload      *database.Upload      // If the message contains an uploaded file
	editMsg     *database.ChatMessage // If we're editing a message
	groupID     *database.GroupID     // If the message is for a subgroup
	hellbanMsg  bool                  // Is the message will be marked HB
	systemMsg   bool                  // Is the message system
	modMsg      bool                  // Is the message part of the "moderators" group
	c           echo.Context
	zeroUser    *database.User // Cache the zero (@0) user
	skipInboxes bool
}

func (c *Command) redirectURL() string {
	return fmt.Sprintf("/api/v1/chat/top-bar/%s?%s", c.room.Name, c.redirectQP.Encode())
}

func (c *Command) receivePM() {
	zeroUser := c.getZeroUser()
	c.toUser = c.authUser
	c.fromUserID = zeroUser.ID
}

// Lazy loading and cache of the zero user
func (c *Command) getZeroUser() database.User {
	if c.zeroUser == nil {
		zeroUser := dutils.GetZeroUser()
		c.zeroUser = &zeroUser
	}
	return *c.zeroUser
}

// Have the "zero" bot account send a message to the authUser
func (c *Command) zeroMsg(msg string) {
	zeroUser := c.getZeroUser()
	c.rawMsg(zeroUser, c.authUser, msg, msg)
}

// Have the "zero" bot account send a processed message to the user
func (c *Command) zeroProcMsg(rawMsg string) {
	c.zeroProcMsgRoom(rawMsg, c.roomKey, c.room.ID)
}

func (c *Command) zeroProcMsgRoom(rawMsg, roomKey string, roomID database.RoomID) {
	zeroUser := c.getZeroUser()
	procMsg, _ := ProcessRawMessage(rawMsg, roomKey, c.authUser.ID, roomID, nil)
	rawMsgRoom(zeroUser, c.authUser, rawMsg, procMsg, roomKey, roomID)
}

func (c *Command) zeroPublicProcMsgRoom(rawMsg, roomKey string, roomID database.RoomID) {
	zeroUser := c.getZeroUser()
	procMsg, _ := ProcessRawMessage(rawMsg, roomKey, c.authUser.ID, roomID, nil)
	rawMsgRoom(zeroUser, nil, rawMsg, procMsg, roomKey, roomID)
}

func (c *Command) zeroPublicMsg(raw, msg string) {
	zeroUser := c.getZeroUser()
	c.rawMsg(zeroUser, nil, raw, msg)
}

func (c *Command) rawMsg(user1 database.User, user2 *database.User, raw, msg string) {
	rawMsgRoom(user1, user2, raw, msg, c.roomKey, c.room.ID)
}

func rawMsgRoom(user1 database.User, user2 *database.User, raw, msg, roomKey string, roomID database.RoomID) {
	var toUserID *database.UserID
	if user2 != nil {
		toUserID = &user2.ID
	}
	_, _ = database.CreateMsg(raw, msg, roomKey, roomID, user1.ID, toUserID)
}

type ErrSuccess struct {
	msg string
}

func NewErrSuccess(msg string) *ErrSuccess {
	return &ErrSuccess{msg: msg}
}

func (e ErrSuccess) Error() string {
	return e.msg
}

func appendUploadLink(html string, upload *database.Upload) string {
	if upload != nil {
		escapedOrigFileName := html2.EscapeString(upload.OrigFileName)
		if html != "" {
			html += " "
		}
		html += `[<a href="/uploads/` + upload.FileName + `" rel="noopener noreferrer" target="_blank">` + escapedOrigFileName + `</a>]`
	}
	return html
}

func checkCPLinks(html string) bool {
	m1 := onionV3Rgx.FindAllStringSubmatch(html, -1)
	m2 := onionV2Rgx.FindAllStringSubmatch(html, -1)
	for _, m := range append(m1, m2...) {
		hash := utils.MD5([]byte(m[0]))
		if _, err := database.GetOnionBlacklist(hash); err == nil {
			return true
		}
	}
	return false
}

func linkDefaultRooms(html string) string {
	r := strings.NewReplacer(
		"#general", `<a href="/chat/general" target="_top">#general</a>`,
		"#programming", `<a href="/chat/programming" target="_top">#programming</a>`,
		"#hacking", `<a href="/chat/hacking" target="_top">#hacking</a>`,
		"#suggestions", `<a href="/chat/suggestions" target="_top">#suggestions</a>`,
		"#announcements", `<a href="/chat/announcements" target="_top">#announcements</a>`,
	)
	return r.Replace(html)
}

func sanitizeUserInput(html string) string {
	p := bluemonday.NewPolicy()
	p.AllowAttrs("href", "target", "rel").OnElements("a")
	p.AllowElements("br")
	return p.Sanitize(html)
}

// Convert timestamps such as 01:23:45 to an archive link if a message with that timestamp exists.
// eg: "Some text 14:31:46 some more text"
func convertArchiveLinks(html string, roomID database.RoomID, authUserID database.UserID) string {
	start, rest := "", html

	// Do not replace timestamps that are inside a quote text
	const quoteSuffix = `”`
	endOfQuoteIdx := strings.LastIndex(html, quoteSuffix)
	if endOfQuoteIdx != -1 {
		start, rest = html[:endOfQuoteIdx], html[endOfQuoteIdx:]
	}

	archiveRgx := regexp.MustCompile(`(\d{2}-\d{2} )?\d{2}:\d{2}:\d{2}`)
	if archiveRgx.MatchString(rest) {
		rest = archiveRgx.ReplaceAllStringFunc(rest, func(s string) string {
			var dt time.Time
			var err error
			if len(s) == 8 { // HH:MM:SS
				dt, err = utils.ParsePrevDatetimeAt(s, clockwork.NewRealClock())
			} else if len(s) == 14 { // mm-dd HH:MM:SS
				dt, err = utils.ParsePrevDatetimeAt2(s, clockwork.NewRealClock())
			}
			if err != nil {
				return s
			}
			if msgs, err := database.GetRoomChatMessagesByDate(roomID, dt.UTC()); err == nil && len(msgs) > 0 {
				msg := msgs[0]
				if len(msgs) > 1 {
					for _, msgTmp := range msgs {
						if msgTmp.User.ID == authUserID || (msgTmp.ToUserID != nil && *msgTmp.ToUserID == authUserID) {
							msg = msgTmp
							break
						}
					}
				}
				return fmt.Sprintf(`<a href="/chat/%s/archive#%s" target="_blank" rel="noopener noreferrer">%s</a>`, msg.Room.Name, msg.UUID, s)
			}
			return s
		})
	}
	return start + rest
}

func convertBangShortcuts(html string) string {
	r := strings.NewReplacer(
		"!bhc", config.BhcOnion,
		"!cryptbb", config.CryptbbOnion,
		"!dread", config.DreadOnion,
		"!dkf", config.DkfOnion,
		"!rroom", config.DkfOnion+`/red-room`,
		"!dnmx", config.DnmxOnion,
		"!whonix", config.WhonixOnion,
		"!age", config.AgeUrl,
	)
	return r.Replace(html)
}

type getUsersByUsernameFn func(usernames []string) ([]database.User, error)

// Update the given html to add user style for tags.
// Return the new html, and a map[userID]User of tagged users.
func colorifyTaggedUsers(html string, getUsersByUsername getUsersByUsernameFn) (string, map[database.UserID]database.User) {
	usernameMatches := tagRgx.FindAllStringSubmatch(html, -1)
	usernames := hashset.New[string]()
	for _, usernameMatch := range usernameMatches {
		usernames.Insert(usernameMatch[1])
	}
	taggedUsers, _ := getUsersByUsername(usernames.ToArray())

	taggedUsersMap := make(map[string]database.User)
	taggedUsersIDsMap := make(map[database.UserID]database.User)
	for _, taggedUser := range taggedUsers {
		taggedUsersMap["@"+taggedUser.Username] = taggedUser
		if taggedUser.Username != "0" {
			taggedUsersIDsMap[taggedUser.ID] = taggedUser
		}
	}

	if tagRgx.MatchString(html) {
		html = tagRgx.ReplaceAllStringFunc(html, func(s string) string {
			if user, ok := taggedUsersMap[s]; ok {
				return fmt.Sprintf("<span %s>%s</span>", user.GenerateChatStyle1(), s)
			}
			return s
		})
	}
	return html, taggedUsersIDsMap
}

func linkRoomTags(html string) string {
	if roomTagRgx.MatchString(html) {
		html = roomTagRgx.ReplaceAllStringFunc(html, func(s string) string {
			if room, err := database.GetChatRoomByName(strings.TrimPrefix(s, "#")); err == nil {
				return `<a href="/chat/` + room.Name + `" target="_top">` + s + `</a>`
			}
			return s
		})
	}
	return html
}

// Given a roomID and hourMinSec (01:23:45) and a username, retrieve the message from database that fits the predicates.
func getQuotedChatMessage(hourMinSec, username string, roomID database.RoomID) (quoted *database.ChatMessage) {
	if dt, err := utils.ParsePrevDatetimeAt(hourMinSec, clockwork.NewRealClock()); err == nil {
		if msgs, err := database.GetRoomChatMessagesByDate(roomID, dt.UTC()); err == nil && len(msgs) > 0 {
			msg := msgs[0]
			if len(msgs) > 1 {
				for _, msgTmp := range msgs {
					if msgTmp.User.Username == username {
						msg = msgTmp
						break
					}
				}
			}
			quoted = &msg
		}
	}
	return
}

// Given a chat message, return the text to be used as a quote.
func getQuoteTxt(roomKey string, quoted database.ChatMessage) (out string) {
	var err error
	decrypted, err := quoted.GetRawMessage(roomKey)
	if err != nil {
		return
	}
	if quoted.ToUserID != nil {
		if m := pmRgx.FindStringSubmatch(decrypted); len(m) == 3 {
			decrypted = m[2]
		}
	} else if quoted.Moderators {
		decrypted = strings.TrimPrefix(decrypted, "/m ")
	} else if quoted.IsHellbanned {
		decrypted = strings.TrimPrefix(decrypted, "/hbm ")
	}
	isMe := false
	if strings.HasPrefix(decrypted, "/me") {
		isMe = true
		decrypted = strings.TrimPrefix(decrypted, "/me ")
	}
	startIdx := strings.LastIndex(decrypted, `” `)
	if startIdx == -1 {
		startIdx = 0
	} else {
		startIdx += len(`” `)
	}

	decrypted = replTextPrefixSuffix(decrypted, agePrefix, ageSuffix, "[age.txt]")
	decrypted = replTextPrefixSuffix(decrypted, pgpPrefix, pgpSuffix, "[pgp.txt]")
	decrypted = replTextPrefixSuffix(decrypted, pgpPKeyPrefix, pgpPKeySuffix, "[pgp_pkey.txt]")

	remaining := " "
	if !quoted.System {
		remaining += fmt.Sprintf(`%s `, quoted.User.Username)
	}
	if quoted.UploadID != nil {
		if upload, err := database.GetUploadByID(*quoted.UploadID); err == nil {
			if decrypted != "" {
				decrypted += " "
			}
			decrypted += `[` + upload.OrigFileName + `]`
		}
	}
	if !isMe {
		remaining += "- "
	}
	remaining += utils.TruncStr2(decrypted[startIdx:], 70, "…")
	return `“[` + quoted.CreatedAt.Format("15:04:05") + "]" + remaining + `”`
}

// This function will get the raw user input message which is not safe to directly render.
//
// To prevent people from altering the text of the quote,
// we retrieve the original quoted message using the timestamp and username,
// and we use the original message text.
//
// eg: we received altered quote, and return original quote ->
// “[01:23:45] username - Some maliciously altered quote” Some text
// “[01:23:45] username - The original text” Some text
func convertQuote(origHtml string, roomKey string, roomID database.RoomID) (html string, quoted *database.ChatMessage) {
	const quotePrefix = `“[`
	const quoteSuffix = `”`
	html = origHtml
	idx := strings.LastIndex(origHtml, quoteSuffix)
	if strings.HasPrefix(origHtml, quotePrefix) && idx > -1 {
		prefixLen := len(quotePrefix)
		suffixLen := len(quoteSuffix)
		if len(origHtml) > prefixLen+9 {
			hourMinSec := origHtml[prefixLen : prefixLen+8]
			username := origHtml[prefixLen+10 : strings.Index(origHtml[prefixLen+10:], " ")+prefixLen+10]
			if quoted = getQuotedChatMessage(hourMinSec, username, roomID); quoted != nil {
				html = getQuoteTxt(roomKey, *quoted)
				html += origHtml[idx+suffixLen:]
			}
		}
	}
	return html, quoted
}

func styleQuote(origHtml string, quoted *database.ChatMessage) (html string) {
	const quoteSuffix = `”`
	html = origHtml
	if quoted != nil {
		idx := strings.LastIndex(origHtml, quoteSuffix)
		prefixLen := len(`<p>“[`)
		suffixLen := len(quoteSuffix)
		dateLen := 8 // 01:23:45 --> 8

		// <p>“[01:23:45] username - quoted text” user text</p>
		date := origHtml[prefixLen : prefixLen+dateLen] // `01:23:45`
		quoteTxt := origHtml[prefixLen+dateLen+1 : idx] // ` username - quoted text`
		userTxt := origHtml[idx+suffixLen:]             // ` user text</p>`

		sb := strings.Builder{}
		sb.WriteString(`<p>“<small style="opacity: 0.8;"><i>[`)

		// Date link
		sb.WriteString(`<a href="/chat/`)
		sb.WriteString(quoted.Room.Name)
		sb.WriteString(`/archive#`)
		sb.WriteString(quoted.UUID)
		sb.WriteString(`" target="_blank" rel="noopener noreferrer">`)
		sb.WriteString(date)
		sb.WriteString(`</a>`)

		sb.WriteString(`]<span `)
		sb.WriteString(quoted.User.GenerateChatStyle1())
		sb.WriteString(`>`)
		sb.WriteString(quoteTxt)
		sb.WriteString(`</span></i></small>”`)
		sb.WriteString(userTxt)
		html = sb.String()
	}
	return html
}

var noSchemeOnionLinkRgx = regexp.MustCompile(`\s[a-z2-7]{56}\.onion`)

// Fix up onion links that are missing the http scheme. This often happen when copy/pasting a link.
func convertLinksWithoutScheme(in string) string {
	html := noSchemeOnionLinkRgx.ReplaceAllStringFunc(in, func(s string) string {
		return " http://" + strings.TrimSpace(s)
	})
	return html
}

var youtubeComIDRgx = regexp.MustCompile(`watch\?v=([\w-]+)`)
var youtubeComShosrtsIDRgx = regexp.MustCompile(`/shorts/([\w-]+)`)
var youtuBeIDRgx = regexp.MustCompile(`https://youtu\.be/([\w-]+)`)
var yewtubeBeIDRgx = youtubeComIDRgx
var invidiousIDRgx = youtubeComIDRgx

func makeHtmlLink(label, link string) string {
	return fmt.Sprintf(`<a href="%s" rel="noopener noreferrer" target="_blank">%s</a>`, link, label)
}

func convertLinks(in string) string {
	libredditURLs := []string{
		"http://spjmllawtheisznfs7uryhxumin26ssv2draj7oope3ok3wuhy43eoyd.onion",
		"http://fwhhsbrbltmrct5hshrnqlqygqvcgmnek3cnka55zj4y7nuus5muwyyd.onion",
		"http://kphht2jcflojtqte4b4kyx7p2ahagv4debjj32nre67dxz7y57seqwyd.onion",
		"http://inytumdgnri7xsqtvpntjevaelxtgbjqkuqhtf6txxhwbll2fwqtakqd.onion",
		"http://liredejj74h5xjqr2dylnl5howb2bpikfowqoveub55ru27x43357iid.onion",
		"http://kzhfp3nvb4qp575vy23ccbrgfocezjtl5dx66uthgrhu7nscu6rcwjyd.onion",
		"http://ecue64ybzvn6vjzl37kcsnwt4ycmbsyf74nbttyg7rkc3t3qwnj7mcyd.onion",
		"http://ledditqo2mxfvlgobxnlhrkq4dh34jss6evfkdkb2thlvy6dn4f4gpyd.onion",
		"http://ol5begilptoou34emq2sshf3may3hlblvipdjtybbovpb7c7zodxmtqd.onion",
		"http://lbrdtjaj7567ptdd4rv74lv27qhxfkraabnyphgcvptl64ijx2tijwid.onion",
		//"http://libredoxhxwnmsb6dvzzd35hmgzmawsq5i764es7witwhddvpc2razid.onion", // Broken
	}

	invidiousURLs := []string{
		"http://c7hqkpkpemu6e7emz5b4vyz7idjgdvgaaa3dyimmeojqbgpea3xqjoid.onion",
		"http://w6ijuptxiku4xpnnaetxvnkc5vqcdu7mgns2u77qefoixi63vbvnpnqd.onion",
		"http://kbjggqkzv65ivcqj6bumvp337z6264huv5kpkwuv6gu5yjiskvan7fad.onion",
		"http://grwp24hodrefzvjjuccrkw3mjq4tzhaaq32amf33dzpmuxe7ilepcmad.onion",
		"http://u2cvlit75owumwpy4dj2hsmvkq7nvrclkpht7xgyye2pyoxhpmclkrad.onion",
		"http://2rorw2w54tr7jkasn53l5swbjnbvz3ubebhswscnc54yac6gmkxaeeqd.onion"}

	wikilessURLs := []string{
		"http://c2pesewpalbi6lbfc5hf53q4g3ovnxe4s7tfa6k2aqkf7jd7a7dlz5ad.onion",
		"http://dj2tbh2nqfxyfmvq33cjmhuw7nb6am7thzd3zsjvizeqf374fixbrxyd.onion"}

	rimgoURLs := []string{
		"http://be7udfhmnzqyt7cxysg6c4pbawarvaofjjywp35nhd5qamewdfxl6sid.onion"}

	knownOnions := [][]string{
		{"http://dkf.onion", config.DkfOnion},
		{"http://dkfgit.onion", config.DkfGitOnion},
		{"http://dread.onion", config.DreadOnion},
		{"http://cryptbb.onion", config.CryptbbOnion},
		{"http://blkhat.onion", config.BhcOnion},
		{"http://dnmx.onion", config.DnmxOnion},
		{"http://whonix.onion", config.WhonixOnion},
	}

	return linkRgx.ReplaceAllStringFunc(in, func(link string) string {
		// Handle reddit links
		if strings.HasPrefix(link, "https://www.reddit.com/") {
			old := strings.Replace(link, "https://www.reddit.com/", "https://old.reddit.com/", 1)
			libredditLink := utils.RandChoice(libredditURLs)
			libredditLink = strings.Replace(link, "https://www.reddit.com", libredditLink, 1)
			oldHtmlLink := makeHtmlLink("old", old)
			libredditHtmlLink := makeHtmlLink("libredditLink", libredditLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + oldHtmlLink + ` | ` + libredditHtmlLink + `)`
		} else if strings.HasPrefix(link, "https://old.reddit.com/") {
			libredditLink := utils.RandChoice(libredditURLs)
			libredditLink = strings.Replace(link, "https://old.reddit.com", libredditLink, 1)
			libredditHtmlLink := makeHtmlLink("libredditLink", libredditLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + libredditHtmlLink + `)`
		}
		for _, libredditURL := range libredditURLs {
			if strings.HasPrefix(link, libredditURL) {
				newPrefix := strings.Replace(link, libredditURL, "http://reddit.onion", 1)
				old := strings.Replace(link, libredditURL, "https://old.reddit.com", 1)
				oldHtmlLink := makeHtmlLink("old", old)
				htmlLink := makeHtmlLink(newPrefix, link)
				return htmlLink + ` (` + oldHtmlLink + `)`
			}
		}

		// Append YouTube link to invidious link
		for _, invidiousURL := range invidiousURLs {
			if strings.HasPrefix(link, invidiousURL) {
				if strings.Contains(link, ".onion/watch?v=") {
					newPrefix := strings.Replace(link, invidiousURL, "http://invidious.onion", 1)
					m := invidiousIDRgx.FindStringSubmatch(link)
					if len(m) == 2 {
						videoID := m[1]
						youtubeLink := "https://www.youtube.com/watch?v=" + videoID
						youtubeHtmlLink := makeHtmlLink("Youtube", youtubeLink)
						htmlLink := makeHtmlLink(newPrefix, link)
						return htmlLink + ` (` + youtubeHtmlLink + `)`
					}
				}
			}
		}
		// Unknown invidious links
		if strings.Contains(link, ".onion/watch?v=") {
			m := invidiousIDRgx.FindStringSubmatch(link)
			if len(m) == 2 {
				videoID := m[1]
				youtubeLink := "https://www.youtube.com/watch?v=" + videoID
				youtubeHtmlLink := makeHtmlLink("Youtube", youtubeLink)
				htmlLink := makeHtmlLink(link, link)
				return htmlLink + ` (` + youtubeHtmlLink + `)`
			}
		}

		// Append wikiless link to wikipedia link
		if strings.HasPrefix(link, "https://en.wikipedia.org/") {
			wikilessLink := utils.RandChoice(wikilessURLs)
			wikilessLink = strings.Replace(link, "https://en.wikipedia.org", wikilessLink, 1)
			wikilessHtmlLink := makeHtmlLink("Wikiless", wikilessLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + wikilessHtmlLink + `)`
		}
		for _, wikilessURL := range wikilessURLs {
			if strings.HasPrefix(link, wikilessURL) {
				newPrefix := strings.Replace(link, wikilessURL, "http://wikiless.onion", 1)
				wikipediaPrefix := strings.Replace(link, wikilessURL, "https://en.wikipedia.org", 1)
				wikipediaHtmlLink := makeHtmlLink("Wikipedia", wikipediaPrefix)
				htmlLink := makeHtmlLink(newPrefix, link)
				return htmlLink + ` (` + wikipediaHtmlLink + `)`
			}
		}

		// Append rimgo link to imgur link
		if strings.HasPrefix(link, "https://imgur.com/") {
			rimgoLink := utils.RandChoice(rimgoURLs)
			rimgoLink = strings.Replace(link, "https://imgur.com", rimgoLink, 1)
			rimgoHtmlLink := makeHtmlLink("Rimgo", rimgoLink)
			htmlLink := makeHtmlLink(link, link)
			return htmlLink + ` (` + rimgoHtmlLink + `)`
		}
		for _, rimgoURL := range rimgoURLs {
			if strings.HasPrefix(link, rimgoURL) {
				newPrefix := strings.Replace(link, rimgoURL, "http://rimgo.onion", 1)
				imgurPrefix := strings.Replace(link, rimgoURL, "https://imgur.com", 1)
				imgurHtmlLink := makeHtmlLink("Imgur", imgurPrefix)
				htmlLink := makeHtmlLink(newPrefix, link)
				return htmlLink + ` (` + imgurHtmlLink + `)`
			}
		}

		// Append invidious link to YouTube/yewtube link
		var videoID string
		var m []string
		var isShortUrl, isYewtube bool
		if strings.HasPrefix(link, "https://youtu.be/") {
			m = youtuBeIDRgx.FindStringSubmatch(link)
		} else if strings.HasPrefix(link, "https://www.youtube.com/watch?v=") {
			m = youtubeComIDRgx.FindStringSubmatch(link)
		} else if strings.HasPrefix(link, "https://yewtu.be/") || strings.HasPrefix(link, "https://www.yewtu.be/") {
			m = yewtubeBeIDRgx.FindStringSubmatch(link)
			isYewtube = true
		} else if strings.HasPrefix(link, "https://www.youtube.com/shorts/") {
			m = youtubeComShosrtsIDRgx.FindStringSubmatch(link)
			isShortUrl = true
		}
		if len(m) == 2 {
			videoID = m[1]
		}
		if videoID != "" {
			invidiousLink := utils.RandChoice(invidiousURLs) + "/watch?v=" + videoID + "&local=true"
			invidiousHtmlLink := makeHtmlLink("Invidious", invidiousLink)
			htmlLink := makeHtmlLink(link, link)
			youtubeLink := "https://www.youtube.com/watch?v=" + videoID
			youtubeHtmlLink := makeHtmlLink("YT", youtubeLink)
			out := htmlLink + ` (` + invidiousHtmlLink + `)`
			if isShortUrl || isYewtube {
				out = htmlLink + ` (` + youtubeHtmlLink + ` | ` + invidiousHtmlLink + `)`
			}
			return out
		}

		for _, el := range knownOnions {
			shortPrefix := el[0]
			longPrefix := el[1]
			if strings.HasPrefix(link, longPrefix) {
				return makeHtmlLink(shortPrefix+strings.TrimPrefix(link, longPrefix), link)
			} else if strings.HasPrefix(link, shortPrefix) {
				return makeHtmlLink(link, longPrefix+strings.TrimPrefix(link, shortPrefix))
			}
		}
		return makeHtmlLink(link, link)
	})
}

func extractPGPMessage(html string) (out string) {
	startIdx := strings.Index(html, pgpPrefix)
	endIdx := strings.Index(html, pgpSuffix)
	if startIdx != -1 && endIdx != -1 {
		out = html[startIdx : endIdx+len(pgpSuffix)]
		out = strings.TrimSpace(out)
		out = strings.TrimPrefix(out, pgpPrefix)
		out = strings.TrimSuffix(out, pgpSuffix)
		out = strings.Join(strings.Split(out, " "), "\n")
		out = pgpPrefix + out
		out += pgpSuffix
	}
	return out
}

// Auto convert pasted pgp message into uploaded file
func convertPGPMessageToFile(html string, authUserID database.UserID) string {
	startIdx := strings.Index(html, pgpPrefix)
	endIdx := strings.Index(html, pgpSuffix)
	if startIdx != -1 && endIdx != -1 {
		tmp := html[startIdx : endIdx+len(pgpSuffix)]
		tmp = strings.TrimSpace(tmp)
		tmp = strings.TrimPrefix(tmp, pgpPrefix)
		tmp = strings.TrimSuffix(tmp, pgpSuffix)
		tmp = strings.Join(strings.Split(tmp, " "), "\n")
		tmp = pgpPrefix + tmp
		tmp += pgpSuffix
		upload, _ := database.CreateUpload("pgp.txt", []byte(tmp), authUserID)
		msgBefore := html[0:startIdx]
		msgAfter := html[endIdx+len(pgpSuffix):]
		html = msgBefore + ` [<a href="/uploads/` + upload.FileName + `" rel="noopener noreferrer" target="_blank">` + upload.OrigFileName + `</a>] ` + msgAfter
		html = strings.TrimSpace(html)
	}
	return html
}

// Auto convert pasted pgp public key into uploaded file
func convertPGPPublicKeyToFile(html string, authUserID database.UserID) string {
	startIdx := strings.Index(html, pgpPKeyPrefix)
	endIdx := strings.Index(html, pgpPKeySuffix)
	if startIdx != -1 && endIdx != -1 {
		pkeySubSlice := html[startIdx : endIdx+len(pgpPKeySuffix)]
		unescapedPkey := html2.UnescapeString(pkeySubSlice)
		tmp := convertInlinePGPPublicKey(unescapedPkey)

		upload, _ := database.CreateUpload("pgp_pkey.txt", []byte(tmp), authUserID)

		msgBefore := html[0:startIdx]
		msgAfter := html[endIdx+len(pgpPKeySuffix):]
		html = msgBefore + ` [<a href="/uploads/` + upload.FileName + `" rel="noopener noreferrer" target="_blank">` + upload.OrigFileName + `</a>] ` + msgAfter
		html = strings.TrimSpace(html)
	}
	return html
}

func convertInlinePGPPublicKey(inlinePKey string) string {
	// If it contains new lines, it was probably pasted using multi-line text box
	if strings.Contains(inlinePKey, "\n") {
		return inlinePKey
	}
	inlinePKey = strings.TrimSpace(inlinePKey)
	inlinePKey = strings.TrimPrefix(inlinePKey, pgpPKeyPrefix)
	inlinePKey = strings.TrimSuffix(inlinePKey, pgpPKeySuffix)
	inlinePKey = strings.TrimSpace(inlinePKey)
	commentsParts := strings.Split(inlinePKey, "Comment: ")
	commentsParts, lastCommentPart := commentsParts[:len(commentsParts)-1], commentsParts[len(commentsParts)-1]
	newCommentsParts := make([]string, 0)
	for idx := range commentsParts {
		if commentsParts[idx] != "" {
			commentsParts[idx] = "Comment: " + commentsParts[idx]
			commentsParts[idx] = strings.TrimSpace(commentsParts[idx])
			newCommentsParts = append(newCommentsParts, commentsParts[idx])
		}
	}

	rgx := regexp.MustCompile(`\s\s(\w|\+|/){64}`)
	m := rgx.FindStringIndex(lastCommentPart)
	commentsStr := ""
	key := ""
	if len(m) == 2 {
		idx := m[0]
		lastCommentP1 := lastCommentPart[:idx]
		lastCommentP2 := lastCommentPart[idx+2:]
		key = strings.Join(strings.Split(lastCommentP2, " "), "\n")
		commentsStr = strings.Join(newCommentsParts, "\n")
		commentsStr += "\nComment: " + lastCommentP1 + "\n\n"
	} else {
		key = "\n" + strings.Join(strings.Split(lastCommentPart, " "), "\n")
	}
	inlinePKey = pgpPKeyPrefix + "\n" + commentsStr + key + "\n" + pgpPKeySuffix
	return inlinePKey
}

// Auto convert pasted age message into uploaded file
func convertAgeMessageToFile(html string, authUserID database.UserID) string {
	startIdx := strings.Index(html, agePrefix)
	endIdx := strings.Index(html, ageSuffix)
	if startIdx != -1 && endIdx != -1 {
		tmp := html[startIdx : endIdx+len(ageSuffix)]
		tmp = strings.TrimSpace(tmp)
		tmp = strings.TrimPrefix(tmp, agePrefix)
		tmp = strings.TrimSuffix(tmp, ageSuffix)
		tmp = strings.Join(strings.Split(tmp, " "), "\n")
		tmp = agePrefix + tmp
		tmp += ageSuffix
		upload, _ := database.CreateUpload("age.txt", []byte(tmp), authUserID)
		msgBefore := html[0:startIdx]
		msgAfter := html[endIdx+len(ageSuffix):]
		html = msgBefore + ` [<a href="/uploads/` + upload.FileName + `" rel="noopener noreferrer" target="_blank">` + upload.OrigFileName + `</a>] ` + msgAfter
		html = strings.TrimSpace(html)
	}
	return html
}
