package interceptors

import (
	"dkforest/pkg/config"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
)

type BangInterceptor struct{}

func (i BangInterceptor) InterceptMsg(cmd *command.Command) {
	switch cmd.Message {
	case "!links":
		handleLinksBangCmd(cmd)
	case "!rtuto":
		handleRtutoBangCmd(cmd)
	}
	return
}

func handleLinksBangCmd(cmd *command.Command) {
	message := `
Chats:
Black Hat Chat: ` + config.BhcOnion + `
Forums:
CryptBB: ` + config.CryptbbOnion
	msg, _, _ := dutils.ProcessRawMessage(cmd.DB, message, "", cmd.AuthUser.ID, cmd.Room.ID, nil, cmd.AuthUser.IsModerator(), true, false)
	cmd.ZeroMsg(msg)
	cmd.Err = command.ErrRedirect
}

func handleRtutoBangCmd(cmd *command.Command) {
	cmd.AuthUser.ResetTutorial(cmd.DB)
	cmd.Err = command.ErrRedirect
}
