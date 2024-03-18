package interceptors

import (
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dkforest/pkg/clockwork"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/levenshtein"
	"dkforest/pkg/managers"
	"dkforest/pkg/poker"
	"dkforest/pkg/web/handlers/interceptors/command"
	"dkforest/pkg/web/handlers/poker"
	"dkforest/pkg/web/handlers/streamModals"
)

type SlashInterceptor struct{}

type CommandHandler func(c *command.Command) (handled bool)

var userCommands = map[string]CommandHandler{
	"/i":                   handleIgnore,
	"/ignore":              handleIgnore,
	"/ui":                  handleUnIgnore,
	"/unignore":            handleUnIgnore,
	"/toggle-autocomplete": handleToggleAutocomplete,
	"/tuto":                handleTutorial,
	"/d":                   handleDeleteMessage,
	"/hide":                handleHideMessage,
	"/unhide":              handleUnHideMessage,
	"/pmwhitelist":         handleListPmWhitelist,
	"/setpmmode":           handleSetPmMode,
	"/pmb":                 handleTogglePmBlacklistedUser,
	"/pmw":                 handleTogglePmWhitelistedUser,
	"/g":                   handleGroupChat,
	"/me":                  handleMe,
	"/e":                   handleEdit,
	"/pm":                  handlePm,
	"/subscribe":           handleSubscribe,
	"/unsubscribe":         handleUnsubscribe,
	"/p":                   handleProfile,
	"/inbox":               handleInbox,
	"/chess":               handleChess,
	"/hbm":                 handleHbm,
	"/hbmt":                handleHbmt,
	"/token":               handleToken,
	"/md5":                 handleMd5,
	"/sha1":                handleSha1,
	"/sha256":              handleSha256,
	"/sha512":              handleSha512,
	"/dice":                handleDice,
	"/rand":                handleRand,
	"/choice":              handleChoice,
	"/memes":               handleListMemes,
	"/success":             handleSuccess,
	"/afk":                 handleAfk,
	"/date":                handleDate,
	"/r":                   handleUpdateReadMarker,
	"/code":                handleCode,
	"/locate":              handleLocate,
	"/error":               handleError,
	"/chips":               handleChipsBalance,
	"/chips-reset":         handleChipsReset,
	"/wizz":                handleWizz,
	"/itr":                 handleInThisRoom,
	"/check":               handleCheck,
	"/call":                handleCall,
	"/fold":                handleFold,
	"/raise":               handleRaise,
	"/allin":               handleAllIn,
	"/bet":                 handleBet,
	"/deal":                handleDeal,
	"/dist":                handleDist,
	//"/chips-send":          handleChipsSend,
}

var privateRoomCommands = map[string]CommandHandler{
	"/mode":      handleGetMode,
	"/wl":        handleWhitelist,
	"/whitelist": handleWhitelist,
}

var privateRoomOwnerCommands = map[string]CommandHandler{
	"/addgroup":  handleAddGroup,
	"/rmgroup":   handleRemoveGroup,
	"/glock":     handleLockGroup,
	"/gunlock":   handleUnlockGroup,
	"/gusers":    handleGroupUsers,
	"/groups":    handleListGroups,
	"/gadduser":  handleAddUserToGroup,
	"/grmuser":   handleRemoveUserFromGroup,
	"/mode":      handleSetMode,
	"/ro":        handleToggleReadOnly,
	"/wl":        handleGetRoomWhitelist,
	"/whitelist": handleGetRoomWhitelist,
}

var moderatorCommands = map[string]CommandHandler{
	"/m":          handleModeratorGroup,
	"/n":          handleModeratorGroup,
	"/moderators": handleListModerators,
	"/mods":       handleListModerators,
	"/k":          handleKick,
	"/kick":       handleKick,
	"/kk":         handleKickKeep,
	"/ks":         handleKickSilent,
	"/kks":        handleKickKeepSilent,
	"/uk":         handleUnkick,
	"/unkick":     handleUnkick,
	"/logout":     handleLogout,
	"/captcha":    handleForceCaptcha,
	"/rtuto":      handleResetTutorial,
	"/hb":         handleHellban,
	"/hell
