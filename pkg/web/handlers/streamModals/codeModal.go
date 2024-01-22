package streamModals

import (
	"bytes"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
	"html/template"
	"strconv"
	"strings"
)

const name = "code"

var langs = [][]string{
	{"", "Raw text"},
	{"go", "Golang"},
	{"rs", "Rust"},
	{"cpp", "C++"},
	{"c", "C"},
	{"py", "Python"},
	{"js", "Javascript"},
	{"php", "PHP"},
	{"css", "CSS"},
	{"sql", "SQL"},
	{"c#", "C#"},
	{"rb", "Ruby"},
	{"html", "HTML"},
	{"bash", "Bash"},
}

type CodeModal struct {
	StreamModal
}

func (m CodeModal) Show(userID database.UserID, roomID database.RoomID, payload database.ChatMessageType) {
	database.MsgPubSub.Pub(m.showTopic(name, userID, roomID), payload)
}

func (m CodeModal) Hide(userID database.UserID, roomID database.RoomID) {
	database.MsgPubSub.Pub(m.hideTopic(name, userID, roomID), database.ChatMessageType{})
}

func NewCodeModal(userID database.UserID, room database.ChatRoom) *CodeModal {
	m := &CodeModal{StreamModal{name: name, userID: userID, room: room}}
	m.topics = append(m.topics, m.showTopic(name, userID, room.ID), m.hideTopic(name, userID, room.ID))
	return m
}

func (m *CodeModal) Css() string {
	return getCss()
}

func (m *CodeModal) Handle(db *database.DkfDB, authUser database.IUserRenderMessage, topic, csrf string, msgTyp database.ChatMessageType, send func(string)) bool {
	if topic == m.topics[0] {
		send(getCodeModalHTML(m.idx, m.room.Name, csrf, msgTyp, authUser.GetSyntaxHighlightCode()))
		return true

	} else if topic == m.topics[1] {
		send(`<style>.code-modal-` + strconv.Itoa(m.idx) + `{display:none;}</style>`)
		m.idx++
		return true
	}

	return false
}

func getCss() string {
	return strings.Join(strings.Split(`
.code-modal {
	display: block; width: calc(100% - 185px - 100px); height: calc(100% - 50px);
	position: fixed; top: 0; left: calc(50% - ((100% - 185px - 100px)/2) - 92px);
	background-color: gray; z-index: 999; border-radius: 5px;
}
    .code-modal .header { position: absolute; top: 0; right: 0; }
    .code-modal .header .cancel {
		border: 1px solid gray;
		background-color: #ff7070;
		color: #850000;
		font-size: 18px;
		height: 23px;
		border: 1px solid #850000;
		border-radius: 0 5px 0 5px;
		cursor: pointer;
	}
	.code-modal .header .cancel:hover {
		background-color: #ff6767;
	}
    .code-modal .wrapper { position: absolute; top: 25px; left: 10px; right: 10px; bottom: 30px; }
    .code-modal .wrapper textarea { width: 100%; height: 100%; color: #fff; background-color: rgba(79,79,79,1); border: 1px solid rgba(90,90,90,1); }
    .code-modal .controls { position: absolute; left: 10px; right: 10px; bottom: 5px; }`, "\n"), " ")
}

func getCodeModalHTML(codeModalIdx int, roomName, csrf string, msgTyp database.ChatMessageType, syntaxHighlightCode string) string {
	htmlTmpl := `<div class="code-modal code-modal-{{ .CodeModalIdx }}">
<form method="post" target="iframe1" action="/api/v1/chat/top-bar/{{ .RoomName }}">
	<input type="hidden" name="csrf" value="{{ .CSRF }}" />
	{{ if .IsMod }}
		<input type="hidden" name="isMod" value="1" />
	{{ end }}
	{{ if .ToUserUsername }}
		<input type="hidden" name="pm" value="{{ .ToUserUsername }}" />
	{{ end }}
	<input type="hidden" name="sender" value="codeModal" />
	<div class="header">
		<button class="cancel" type="submit" name="btn_cancel" value="1">×</button>
	</div>
	<div class=wrapper>
		<textarea name="message" placeholder="Paste your code here..."></textarea>
	</div>
	<div class="controls">
		<button type="submit">send</button>
		<select name="lang">
			{{ range .Langs }}
				<option value="{{ index . 0 }}"{{ if eq $.SyntaxHighlightCode (index . 0) }} selected{{ end }}>{{ index . 1 }}</option>
			{{ end }}
		</select>
	</div>
</form>
</div>`
	data := map[string]any{
		"CSRF":                csrf,
		"RoomName":            roomName,
		"CodeModalIdx":        codeModalIdx,
		"IsMod":               msgTyp.IsMod,
		"ToUserUsername":      msgTyp.ToUserUsername,
		"SyntaxHighlightCode": syntaxHighlightCode,
		"Langs":               langs,
	}
	var buf bytes.Buffer
	_ = utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data)
	return buf.String()
}

func (_ CodeModal) InterceptMsg(cmd *command.Command) {
	sender := cmd.C.Request().PostFormValue("sender")
	lang := cmd.C.Request().PostFormValue("lang")
	isMod := utils.DoParseBool(cmd.C.Request().PostFormValue("isMod"))
	pm := cmd.C.Request().PostFormValue("pm")
	btnCancel := cmd.C.Request().PostFormValue("btn_cancel")

	if !cmd.AuthUser.CanUseMultiline || sender != "codeModal" {
		return
	}

	CodeModal{}.Hide(cmd.AuthUser.ID, cmd.Room.ID)

	if !isValidLang(lang) {
		lang = ""
	}
	cmd.AuthUser.SetSyntaxHighlightCode(cmd.DB, lang)

	cmd.ModMsg = isMod
	if pm != "" {
		if err := cmd.SetToUser(database.Username(pm)); err != nil {
			cmd.Err = command.ErrRedirect
			return
		}
		cmd.RedirectQP.Set(command.RedirectPmQP, string(cmd.ToUser.Username))
	}

	if cmd.OrigMessage == "" || btnCancel == "1" {
		cmd.Err = command.ErrRedirect
		return
	}

	cmd.OrigMessage = codeFenceWrap(lang, cmd.OrigMessage)
	cmd.Message = codeFenceWrap(lang, cmd.Message)
}

func codeFenceWrap(lang, msg string) string {
	return "\n```" + lang + "\n" + msg + "\n```\n"
}

func isValidLang(lang string) (found bool) {
	for _, l := range langs {
		if lang == l[0] {
			return true
		}
	}
	return
}
