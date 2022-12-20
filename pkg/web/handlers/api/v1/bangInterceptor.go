package v1

import "dkforest/pkg/config"

type BangInterceptor struct{}

func (i BangInterceptor) InterceptMsg(cmd *Command) {
	switch cmd.message {
	case "!links":
		handleLinksBangCmd(cmd)
	case "!rtuto":
		handleRtutoBangCmd(cmd)
	}
	return
}

func handleLinksBangCmd(cmd *Command) {
	message := `
Chats:
Black Hat Chat: ` + config.BhcOnion + `
Forums:
CryptBB: ` + config.CryptbbOnion
	msg, _ := ProcessRawMessage(message, "", cmd.authUser.ID, cmd.room.ID, nil)
	cmd.zeroMsg(msg)
	cmd.err = ErrRedirect
}

func handleRtutoBangCmd(cmd *Command) {
	cmd.authUser.ChatTutorial = 0
	cmd.authUser.DoSave()
	cmd.err = ErrRedirect
}
