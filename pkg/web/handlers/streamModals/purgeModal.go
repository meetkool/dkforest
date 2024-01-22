package streamModals

import (
	"bytes"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
	"fmt"
	"html/template"
	"strconv"
	"strings"
)

const purgeModalName = "purge"

type PurgeModal struct {
	StreamModal
}

func (m PurgeModal) Show(userID database.UserID, roomID database.RoomID, payload database.ChatMessageType) {
	database.MsgPubSub.Pub(m.showTopic(purgeModalName, userID, roomID), payload)
}

func (m PurgeModal) Hide(userID database.UserID, roomID database.RoomID) {
	database.MsgPubSub.Pub(m.hideTopic(purgeModalName, userID, roomID), database.ChatMessageType{})
}

func NewPurgeModal(userID database.UserID, room database.ChatRoom) *PurgeModal {
	m := &PurgeModal{StreamModal{name: purgeModalName, userID: userID, room: room}}
	m.topics = append(m.topics, m.showTopic(purgeModalName, userID, room.ID), m.hideTopic(purgeModalName, userID, room.ID))
	return m
}

func (m *PurgeModal) Css() string {
	return getPurgeModalCss()
}

func (m *PurgeModal) Handle(db *database.DkfDB, authUser database.IUserRenderMessage, topic, csrf string, msgTyp database.ChatMessageType, send func(string)) bool {
	if topic == m.topics[0] {
		send(getPurgeModalHTML(db, m.idx, m.room.Name, csrf, msgTyp))
		return true

	} else if topic == m.topics[1] {
		send(`<style>.purge-modal-` + strconv.Itoa(m.idx) + `{display:none;}</style>`)
		m.idx++
		return true
	}

	return false
}

func getPurgeModalCss() string {
	return strings.Join(strings.Split(`
.purge-modal {
	display: block;
	width: 400px;
	left: calc(50% - 200px - (185px/2));
	height: 100px;
	position: fixed; top: 0;
	background-color: gray; z-index: 999; border-radius: 5px;
}
    .purge-modal .header { position: absolute; top: 0; right: 0; }
    .purge-modal .header .cancel {
		border: 1px solid gray;
		background-color: #ff7070;
		color: #850000;
		font-size: 18px;
		height: 23px;
		border: 1px solid #850000;
		border-radius: 0 5px 0 5px;
		cursor: pointer;
	}
	.purge-modal .header .cancel:hover {
		background-color: #ff6767;
	}
    .purge-modal .wrapper { position: absolute; top: 25px; left: 10px; right: 10px; bottom: 30px; }
    .purge-modal .wrapper textarea { width: 100%; height: 100%; color: #fff; background-color: rgba(79,79,79,1); border: 1px solid rgba(90,90,90,1); }
    .purge-modal .controls { position: absolute; left: 10px; right: 10px; bottom: 5px; }`, "\n"), " ")
}

func getPurgeModalHTML(db *database.DkfDB, purgeModalIdx int, roomName, csrf string, msgTyp database.ChatMessageType) string {
	htmlTmpl := `<div class="purge-modal purge-modal-{{ .PurgeModalIdx }}">
<form method="post" target="iframe1" action="/api/v1/chat/top-bar/{{ .RoomName }}">
	<input type="hidden" name="csrf" value="{{ .CSRF }}" />
	<input type="hidden" name="sender" value="purgeModal" />
	<div class=wrapper>
		<input type="text" name="username" placeholder="username" autocomplete="off" autocapitalize="none" />
		<select name="typ">
			<option value="all">All messages</option>
			<option value="hb">HB messages</option>
		</select>
		<select name="delta">
			<option value="300">5 minutes</option>
			<option value="3600" selected>1 hour</option>
			<option value="21600">6 hour</option>
			<option value="43200">12 hour</option>
			<option value="86400">24 hour</option>
		</select>
	</div>
	<div class="controls">
		<button type="submit">purge</button>
	</div>
	<div class="header">
		<button class="cancel" type="submit" name="btn_cancel" value="1">×</button>
	</div>
</form>
</div>`
	data := map[string]any{
		"CSRF":          csrf,
		"RoomName":      roomName,
		"PurgeModalIdx": purgeModalIdx,
	}
	var buf bytes.Buffer
	_ = utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data)
	return buf.String()
}

func (_ PurgeModal) InterceptMsg(cmd *command.Command) {
	sender := cmd.C.Request().PostFormValue("sender")
	btnCancel := cmd.C.Request().PostFormValue("btn_cancel")
	delta := utils.DoParseInt64(cmd.C.Request().PostFormValue("delta"))
	username := database.Username(cmd.C.Request().PostFormValue("username"))
	typ := cmd.C.Request().PostFormValue("typ")

	if !cmd.AuthUser.IsAdmin || sender != "purgeModal" {
		return
	}

	PurgeModal{}.Hide(cmd.AuthUser.ID, cmd.Room.ID)

	if btnCancel == "1" {
		cmd.Err = command.ErrRedirect
		return
	}

	user, err := cmd.DB.GetUserByUsername(username)
	if err != nil {
		cmd.Err = err
		return
	}
	cmd.DB.NewAudit(*cmd.AuthUser, fmt.Sprintf("purge %s #%d", user.Username, user.ID))
	_ = cmd.DB.DeleteUserChatMessagesOpt(user.ID, typ == "hb", delta)

	database.MsgPubSub.Pub(database.RefreshTopic, database.ChatMessageType{Typ: database.ForceRefresh})

	cmd.Err = command.NewErrSuccess(string(user.Username) + " purged")
	return
}
