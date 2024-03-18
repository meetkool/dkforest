package config

import (
	"fmt"
	"net"

	"dkforest/pkg/utils/rwmtx"
	"embed"
	"github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/hashicorp/go-version"
	_ "github.com/lib/pq"
	"sync"
	"time"

	"dkforest/pkg/atom"
	"dkforest/pkg/ratecounter"
)

// DefaultMasterKey Should be overwritten using ldflags
var DefaultMasterKey = "Ucn%1fw%bPz3<Ir}lJD6H!X+fP47j]c2"
var GistPasswordSalt = "gist_pa$$word_$alt_tdjfPAgjyNdor"
var RoomPasswordSalt = "room_pa$$word_$alt_OYvUwmNPVTdsw"

const (
	DkfOnion     = "http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion"
	DkfGitOnion  = "http://git.dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion"
	I2pGitOnion  = "http://git.dkforest4gwaceahf4te3vs7ycddtbpf2lucocxdzhphezikdgnq.b32.i2p"
	DkfGit1Onion = "http://yylovpz7taca7jfrub3wltxabzzjp34fngj5lpwl6eo47ekt5cxs6mid.onion"
	DreadOnion   = "http://dreadytofatroptsdj6io7l3xptbet6onoyno2yv7jicoxknyazubrad.onion"
	BhcOnion     = "http://blkhatjxlrvc5aevqzz5t6kxldayog6jlx5h7glnu44euzongl4fh5ad.onion"
	CryptbbOnion = "http://cryptbbtg65gibadeeo2awe3j7s6evg7eklserehqr4w4e2bis5tebid.onion"
	DnmxOnion    = "http://hxuzjtocnzvv5g2rtg2bhwkcbupmk7rclb6lly3fo4tvqkk5oyrv3nid.onion"
	WhonixOnion  = "http://dds6qkxpwdeubwucdiaord2xgbbeyds25rbsgr73tbfpqpt4a6vjwsyd.onion"
	AgeUrl       = "https://github.com/FiloSottile/age"
)

const (
	RootAdminID           = 1
	GeneralRoomID         = 1
	AnnouncementsRoomName = "announcements"
)

const PowDifficulty = 7

const EditMessageTimeLimit = 2 * time.Minute

const NullUsername = "0"

var NullUserPrivateKey string
var NullUserPublicKey string

// Global ...
var Global = NewGlobalConf()
var (
	Development = atom.NewBool(true) // either or not the instance is running in development mode

	IsFirstUse               = atom.NewBool(true)  // either or not we need to set up root account
	MaybeAuthEnabled         = atom.NewBool(true)  // either or not unauthenticated users can access the "maybe auth" pages
	ForumEnabled             = atom.NewBool(true)  // either or not people can use the forum features
	SilentSelfKick           = atom.NewBool(true)  // either or not self kick are silent
	SignupEnabled            = atom.NewBool(true)  // either or not people can sign up
	PowEnabled               = atom.NewBool(false) // either or not pow is enabled to signup
	PokerWithdrawEnabled     = atom.NewBool(true)  // either or not poker withdraw is enabled
	SignupFakeEnabled        = atom.NewBool(true)  // either or not signup is faked to be enabled
	ProtectHome              = atom.NewBool(true)  // enable "dynamic login url" to prevent ddos on the login page
	HomeUsersList            = atom.NewBool(true)  // either or not to display the online users on the login page
	ForceLoginCaptcha        = atom.NewBool(true)  // either or not people are forced to complete captcha at login
	DownloadsEnabled         = atom.NewBool(true)  // either or not people can download files
	MaintenanceAtom          = atom.NewBool(false) // people are redirected to a maintenance page
	CaptchaDifficulty        = atom.NewInt64(1)    // captcha difficulty
	SignupPageLoad           = atom.NewInt64(0)
	SignupFailed             = atom.NewInt64(0)
	SignupSucceed            = atom.NewInt64(0)
	BHCCaptchaFailed         = atom.NewInt64(0)
	BHCCaptchaSuccess        = atom.NewInt64(0)
	BHCCaptchaGenerated      = atom.NewInt64(0)
	CaptchaRequiredGenerated = atom.NewInt64(0)
	CaptchaRequiredSuccess   = atom.NewInt64(0)
	CaptchaRequiredFailed    = atom.NewInt64(0)

