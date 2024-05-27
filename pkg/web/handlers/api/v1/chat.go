package v1

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/hashset"
	"dkforest/pkg/managers"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
	"dkforest/pkg/web/handlers/poker"
	"dkforest/pkg/web/handlers/streamModals"
	hutils "dkforest/pkg/web/handlers/utils"
	"dkforest/pkg/web/handlers/utils/stream"
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"net/http"
	"strings"
	"time"
)

func manualPreload(db *database.DkfDB, msg *database.ChatMessage, room database.ChatRoom) {
	if msg.GroupID != nil {
		if msg.Group == nil {
			group, _ := db.GetRoomGroupByID(msg.RoomID, *msg.GroupID)
			msg.Group = &group
		}
	}
	if msg.ToUserID != nil {
		if msg.ToUser == nil {
			toUser, _ := db.GetUserByID(*msg.ToUserID)
			msg.ToUser = &toUser
		}
	}
	if msg.User.ID == 0 {
		msg.User, _ = db.GetUserByID(msg.UserID)
	}
	msg.Room = room
}

// Return true if the message passes all the user's filter.
// false if the message does not and should be discarded.
func applyUserFilters(db *database.DkfDB, authUser database.IUserRenderMessage, msg *database.ChatMessage,
	pmUserID *database.UserID, pmOnlyQuery database.PmDisplayMode, displayHellbanned, mentionsOnlyQuery bool) bool {
	if pmUserID != nil {
		if msg.ToUserID == nil {
			return false
		}
		if *msg.ToUserID == *pmUserID || msg.UserID == *pmUserID {
			return true
		}
	}
	if (pmOnlyQuery == database.PmOnly && msg.ToUser == nil) ||
		(pmOnlyQuery == database.PmNone && msg.ToUser != nil) ||
		!authUser.GetDisplayModerators() && msg.Moderators ||
		!displayHellbanned && msg.IsHellbanned {
		return false
	}

	if !authUser.GetDisplayIgnored() {
		ignoredUsersIDs, _ := db.GetIgnoredUsersIDs(authUser.GetID())
		if utils.InArr(msg.UserID, ignoredUsersIDs) {
			return false
		}
	}

	if mentionsOnlyQuery && !strings.Contains(msg.Message, authUser.GetUsername().AtStr()) {
		return false
	}
	return true
}

func soundNotifications(msg *database.ChatMessage, authUser database.IUserRenderMessage, renderedMsg *string) (out string) {
	var newMessageSound, taggedSound, pmSound bool
	if msg.User.ID != authUser.GetID() && !msg.SkipNotify {
		newMessageSound = true
		if strings.Contains(*renderedMsg, authUser.GetUsername().AtStr()) {
			taggedSound = true
		}
		if msg.IsPmRecipient(authUser.GetID()) {
			pmSound = true
		}
	}
	if (authUser.GetNotifyTagged() && taggedSound) || (authUser.GetNotifyPmmed() && pmSound) {
		out = `<audio src="/public/mp3/sound5.mp3" autoplay></audio>`
	} else if authUser.GetNotifyNewMessage() && newMessageSound {
		out = `<audio src="/public/mp3/sound6.mp3" autoplay></audio>`
	}
	return
}

type Alternator struct {
	state          bool
	fmt, animation string
}

func newAlternator(fmt, animation string) *Alternator {
	return &Alternator{fmt: fmt, animation: animation}
}

func (a *Alternator) alternate() string {
	a.state = !a.state
	return fmt.Sprintf(a.fmt, a.animation+utils.Ternary(a.state, "1", "2"))
}

func ChatStreamMessagesHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)
	csrf, _ := c.Get("csrf").(string)

	queryParams := c.QueryParams()
	_, mlFound := queryParams["ml"]
	_, hrmFound := queryParams["hrm"]
	_, hideTsFound := queryParams["hide_ts"]

	roomName := c.Param("roomName")
	room, roomKey, err := dutils.GetRoomAndKey(db, c, roomName)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	streamItem, err := stream.SetStreaming(c, authUser.ID, "")
	if err != nil {
		return nil
	}
	defer streamItem.Cleanup()

	// Keep track of how many bytes we sent on the http request, so we can auto-refresh when passing a threshold
	bytesSent := 0
	send := func(s string) {
		n, _ := c.Response().Write([]byte(s))
		bytesSent += n
	}

	data := ChatMessagesData{}
	data.TopBarQueryParams = utils.TernaryOrZero(mlFound, "&ml=1")
	data.HideRightColumn = authUser.HideRightColumn || hrmFound
	data.HideTimestamps = authUser.GetDateFormat() == "" || hideTsFound

	// Register modals and send the css for them
	modalsManager := streamModals.NewModalsManager()
	modalsManager.Register(streamModals.NewCodeModal(authUser.ID, room))
	if authUser.IsAdmin {
		modalsManager.Register(streamModals.NewPurgeModal(authUser.ID, room))
	}
	send(modalsManager.Css())

	data.ReadMarker, _ = db.GetUserReadMarker(authUser.ID, room.ID)
	data.ChatMenuData.RoomName = room.Name
	data.ManualRefreshTimeout = 0
	send(GenerateStyle(authUser, data))
	if authUser.DisplayAliveIndicator {
		send(`<div id="i"></div>`) // http alive indicator; green/red dot
	}
	send(fmt.Sprintf(`<div style="display:flex;flex-direction:column-reverse;" id="msgs">`))

	// Get initial messages for the user
	pmOnlyQuery := dutils.DoParsePmDisplayMode(c.QueryParam("pmonly"))
	mentionsOnlyQuery := utils.DoParseBool(c.QueryParam("mentionsOnly"))
	pmUserID := dutils.GetUserIDFromUsername(db, c.QueryParam(command.RedirectPmUsernameQP))
	displayHellbanned := authUser.DisplayHellbanned || authUser.IsHellbanned
	displayIgnoredMessages := utils.False()
	msgs, err := db.GetChatMessages(room.ID, roomKey, authUser.Username, authUser.ID, pmUserID, pmOnlyQuery, mentionsOnlyQuery,
		displayHellbanned, authUser.DisplayIgnored, authUser.DisplayModerators, displayIgnoredMessages, 150, 0)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	// Render the messages as html
	data.Messages = msgs
	data.NbButtons = authUser.CountUIButtons()
	nullUsername := config.NullUsername
	hasNoMsgs := len(data.Messages) == 0
	send("<div>" + RenderMessages(authUser, data, csrf, nullUsername, nil, false) + "</div>")
	c.Response().Flush()

	// Create a subscriber and topics to listen to
	selfRefreshLoadingIconTopic := "refresh_loading_icon_" + string(authUser.Username)
	selfRefreshTopic := "refresh_" + string(authUser.Username)
	selfWizzTopic := "wizz_" + string(authUser.Username)
	readMarkerTopic := "readmarker_" + authUser.ID.String()
	authorizedTopics := []string{
		database.RefreshTopic,
		selfRefreshTopic,
		selfRefreshLoadingIconTopic,
		selfWizzTopic,
		readMarkerTopic,
		"room_" + room.ID.String()}
	authorizedTopics = append(authorizedTopics, modalsManager.Topics()...)
	sub := database.MsgPubSub.Subscribe(authorizedTopics)
	defer sub.Close()

	// Keep track of messages that are after the read-marker (unread).
	// When we receive a "delete msg", and this map is empty, we should hide the read-marker
	// as it means the read marker is now at the very top.
	msgsMap := hashset.New[int64]()
	for _, msg := range msgs {
		if msg.CreatedAt.After(data.ReadMarker.ReadAt) {
			msgsMap.Set(msg.ID)
		}
	}

	// If the read-marker is at the very top, it will be hidden and need to be displayed when we receive a new message.
	// If it is not at the top, it will already be visible and does not need to be displayed again.
	var displayReadMarker bool
	if len(msgs) > 0 {
		fstMsgTsRound := msgs[0].CreatedAt.Round(time.Second)
		readMarkerTsRound := data.ReadMarker.ReadAt.Round(time.Second)
		displayReadMarker = !fstMsgTsRound.After(readMarkerTsRound)
	}

	// Keep track of current read-marker revision
	readMarkerRev := 0
	// Function to hide current rev of read marker and insert an invisible one at the top.
	updateReadMarker := func() {
		if authUser.ChatReadMarkerEnabled {
			send(fmt.Sprintf(`<style>.read-marker-%d{display:none !important;}</style>`, readMarkerRev))
			send(fmt.Sprintf(`<div class="read-marker read-marker-%d" style="display:none;"></div>`, readMarkerRev+1))
		}
		readMarkerRev++
		displayReadMarker = true
	}
	// Function to show the invisible read-marker which used to be at the top.
	showReadMarker := func() {
		if displayReadMarker {
			if authUser.ChatReadMarkerEnabled {
				send(fmt.Sprintf(`<style>.read-marker-%d{display:block !important;}</style>`, readMarkerRev))
			}
			displayReadMarker = false
		}
	}

	// Toggle between true/false every 5sec. This bool keep track of which class to send for our "online indicator"
	// We need to change the css class in order for the css to never actually complete the animation and stay "green".
	indicatorAlt := newAlternator(`<style>#i{animation: %s 30s forwards}</style>`, "i")
	wizzAlt := newAlternator(`<style>#msgs{animation: %s 0.25s linear 7;}</style>`, "horizontal-shaking")

Loop:
	for {
		select {
		case <-streamItem.Quit:
			break Loop
		default:
		}

		// Refresh the page to prevent having it growing infinitely bigger
		if bytesSent > 10<<20 { // 10 MB
			send(hutils.MetaRefreshNow())
			return nil
		}

		authUser1, err := db.GetUserRenderMessageByID(authUser.ID)
		if err != nil {
			break Loop
		}

		managers.ActiveUsers.UpdateUserInRoom(room, managers.NewUserInfo(authUser1))

		// Update read record
		db.UpdateChatReadRecord(authUser1.GetID(), room.ID)

		// Toggle the "http alive indicator" class to keep the dot green
		if authUser1.GetDisplayAliveIndicator() {
			send(indicatorAlt.alternate())
		}

		topic, msgTyp, err := sub.ReceiveTimeout2(5*time.Second, streamItem.Quit)
		if err != nil {
			if errors.Is(err, pubsub.ErrCancelled) {
				break Loop
			}
			c.Response().Flush()
			continue
		}

		// We receive this event when the "update read-marker" button is clicked.
		// This means the user is saying that all messages are read, and read-marker should be at the very top.
		if topic == readMarkerTopic {
			msgsMap.Clear() // read-marker at the top, so no unread message
			updateReadMarker()
			c.Response().Flush()
			continue
		}

		if topic == selfRefreshLoadingIconTopic {
			send(hutils.MetaRefresh(1))
			return nil
		}

		if topic == selfRefreshTopic && msgTyp.Typ == database.Close {
			return nil
		}

		if topic == selfRefreshTopic && msgTyp.Typ == database.Redirect {
			send(hutils.MetaRedirectNow(msgTyp.NewURL))
			return nil
		}

		if topic == selfRefreshTopic || msgTyp.Typ == database.ForceRefresh {
			send(hutils.MetaRefreshNow())
			return nil
		}

		if topic == selfWizzTopic || msgTyp.Typ == database.Wizz {
			send(wizzAlt.alternate())
			c.Response().Flush()
			continue
		}

		if modalsManager.Handle(db, authUser1, topic, csrf, msgTyp, send) {
			c.Response().Flush()
			continue
		}

		if msgTyp.Typ == database.DeleteMsg {
			// Delete msg from the map that keep track of unread messages.
			// If the map is now empty, we hide the read-marker.
			msgsMap.Delete(msgTyp.Msg.ID)
			if msgsMap.Empty() {
				updateReadMarker()
			}

			send(fmt.Sprintf(`<style>.msgidc-%s-%d{display:none;}</style>`, msgTyp.Msg.UUID, msgTyp.Msg.Rev))
			c.Response().Flush()
			continue
		}

		if msgTyp.Typ == database.EditMsg {
			// Get all messages for the user that were created after the edited one (included)
			msgs, err := db.GetChatMessages(room.ID, roomKey, authUser1.GetUsername(), authUser1.GetID(), pmUserID, pmOnlyQuery,
				mentionsOnlyQuery, displayHellbanned, authUser1.GetDisplayIgnored(), authUser1.GetDisplayModerators(),
				displayIgnoredMessages, 150, msgTyp.Msg.ID)
			if err != nil {
				return c.Redirect(http.StatusFound, "/")
			}

			// If no messages, continue. This might happen if the user has ignored the user making the edit.
			if len(msgs) == 0 {
				c.Response().Flush()
				continue
			}

			// Generate css to hide the previous revision of these messages
			toHide := make([]string, len(msgs))
			for i, msg := range msgs {
				toHide[i] = fmt.Sprintf(".msgidc-%s-%d", msg.UUID, msg.Rev-1)
			}
			send(fmt.Sprintf(`<style>%s{display:none;}</style>`, strings.Join(toHide, ",")))

			// Render the new revision of the messages in html
			data.Messages = msgs
			data.NbButtons = authUser1.CountUIButtons()
			data.ReadMarker, _ = db.GetUserReadMarker(authUser1.GetID(), room.ID)

			// Only try to redraw the read-marker if the first message
			// that we redraw is older than our read-marker position.
			var readMarkerRevRef *int
			fstMsgTsRound := msgs[0].CreatedAt.Round(time.Second)
			readMarkerTsRound := data.ReadMarker.ReadAt.Round(time.Second)
			if !fstMsgTsRound.After(readMarkerTsRound) {
				readMarkerRevRef = &readMarkerRev
			}

			send(RenderMessages(authUser1, data, csrf, nullUsername, readMarkerRevRef, true))

			c.Response().Flush()
			continue
		}

		msg := &msgTyp.Msg
		if room.IsProtected() {
			if err := msg.Decrypt(roomKey); err != nil {
				return c.Redirect(http.StatusFound, "/")
			}
		}

		if !dutils.VerifyMsgAuth(db, msg, authUser1.GetID(), authUser1.IsModerator()) ||
			!applyUserFilters(db, authUser1, msg, pmUserID, pmOnlyQuery, displayHellbanned, mentionsOnlyQuery) {
			continue
		}

		manualPreload(db, msg, room)

		baseTopBarURL := "/api/v1/chat/top-bar/" + room.Name
		if hasNoMsgs {
			send(`<style>#no-msg{display:none}</style>`)
			hasNoMsgs = false
		}
		readMarkerRendered := true
		isFirstMsg := false
		isEdit := utils.False()
		renderedMsg := RenderMessage(1, *msg, authUser1, data, baseTopBarURL, &readMarkerRendered, &isFirstMsg, csrf, nullUsername, &readMarkerRev, isEdit)

		// Keep track of unread messages
		msgsMap.Set(msg.ID)

		send(renderedMsg)
		showReadMarker()

		// Sound notifications
		send(soundNotifications(msg, authUser1, &renderedMsg))

		c.Response().Flush()
	} // end of infinite loop (LOOP)

	// Display a big banner stating the connection is closed.
	send(`<div class="connection-closed">Connection closed</div>`)
	// Auto refresh the page after 5sec so that the client reconnect after the app has restarted
	send(hutils.MetaRefresh(5))
	c.Response().Flush()
	return nil
}

// ChatStreamMenuHandler return the html for the "stream" chat right-manu.
func ChatStreamMenuHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)
	roomName := c.Param("roomName")

	room, _, err := dutils.GetRoomAndKey(db, c, roomName)
	if err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	if !authUser.UseStreamMenu {
		data := GetChatMenuData(c, room)
		s := utils.TernaryOrZero(!data.PreventRefresh, hutils.MetaRefresh(5))
		s += GenerateStyle(authUser, ChatMessagesData{})
		s += RenderRightColumn(authUser, data)
		return c.HTML(http.StatusOK, s)
	}

	streamItem, err := stream.SetStreaming(c, authUser.ID, "")
	if err != nil {
		return nil
	}
	defer streamItem.Cleanup()

	send := func(s string) { _, _ = c.Response().Write([]byte(s)) }
	var prevHash uint32
	var menuID int
	var once utils.Once

	selfRefreshLoadingIconTopic := "refresh_loading_icon_" + string(authUser.Username)
	selfRefreshTopic := "refresh_" + string(authUser.Username)
	sub := database.MsgPubSub.Subscribe([]string{
		database.RefreshTopic,
		selfRefreshTopic,
		selfRefreshLoadingIconTopic})
	defer sub.Close()

	send(GenerateStyle(authUser, ChatMessagesData{}))

Loop:
	for {
		select {
		case <-once.Now():
		case <-time.After(5 * time.Second):
		case p := <-sub.ReceiveCh():
			if p.Topic == selfRefreshLoadingIconTopic {
				send(hutils.MetaRefresh(1))
				return nil
			}
			if p.Msg.Typ == database.ForceRefresh || p.Topic == selfRefreshTopic {
				send(hutils.MetaRefreshNow())
				return nil
			}
			if p.Msg.Typ == database.CloseMenu {
				return nil
			}
			send(hutils.MetaRefresh(1))
			return nil
		case <-streamItem.Quit:
			break Loop
		}

		data := GetChatMenuData(c, room)
		rightColumn := RenderRightColumn(authUser, data)
		newHash := utils.Crc32([]byte(rightColumn))
		if newHash != prevHash {
			send(fmt.Sprintf(`<style>#menu_%d{display:none}</style><div id="menu_%d">%s</div>`, menuID, menuID+1, rightColumn))
			c.Response().Flush()
			prevHash = newHash
			menuID++
		}
	}
	send(hutils.MetaRefresh(5))
	c.Response().Flush()
	return nil
}

func ChatStreamMessagesRefreshHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	database.MsgPubSub.Pub("refresh_loading_icon_"+string(authUser.Username), database.ChatMessageType{Typ: database.ForceRefresh})
	poker.PubSub.Pub("refresh_loading_icon_"+string(authUser.Username), poker.RefreshLoadingIconEvent{})
	return hutils.RedirectReferer(c)
}
