package config

import (
	"dkforest/pkg/utils/rwmtx"
	"embed"
	wallet1 "github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"net"
	"sync"
	"time"

	"dkforest/pkg/atom"
	"dkforest/pkg/ratecounter"
	version "github.com/hashicorp/go-version"
)

const (
	GogsURL                = "http://127.0.0.1:3000"
	DbFileName             = "dkf.db"
	AppDirName             = ".dkf"
	MaxUserFileUploadSize  = 30 << 20  // 30 MB
	MaxUserTotalUploadSize = 100 << 20 // 100 MB
	MaxAvatarFormSize      = 1 << 20   // 1 MB
	MaxAvatarSize          = 300 << 10 // 300 KB

	// MaxFileSizeBeforeDownload files that are bigger than this limit will trigger
	// a file download instead of simple in-browser rendering
	MaxFileSizeBeforeDownload = 1 << 20 // 1 MB
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
	MoneroPrice              = atom.NewFloat64(0)

	RpsCounter         = ratecounter.NewRateCounter()
	RejectedReqCounter = ratecounter.NewRateCounter()
)

var MigrationsFs embed.FS
var LocalsFs embed.FS

var once sync.Once
var instance wallet1.Client

func Xmr() wallet1.Client {
	once.Do(func() {
		instance = wallet1.New(wallet1.Config{
			Address: "http://127.0.0.1:6061/json_rpc",
		})
	})
	return instance
}

type ConnManager struct {
	sync.RWMutex
	m           map[net.Conn]int64
	CircuitIDCh chan int64
}

func NewConnManager() *ConnManager {
	m := new(ConnManager)
	m.m = make(map[net.Conn]int64)
	m.CircuitIDCh = make(chan int64, 1000)
	return m
}

func (m *ConnManager) Set(key net.Conn, val int64) {
	m.Lock()
	m.m[key] = val
	m.Unlock()
}

func (m *ConnManager) Get(key net.Conn) int64 {
	m.RLock()
	val, found := m.m[key]
	if !found {
		m.RUnlock()
		return 0
	}
	m.RUnlock()
	return val
}

func (m *ConnManager) Delete(key net.Conn) {
	m.Lock()
	delete(m.m, key)
	m.Unlock()
}

func (m *ConnManager) Close(key net.Conn) {
	circuitID := m.Get(key)
	m.CloseCircuit(circuitID)
}

func (m *ConnManager) CloseCircuit(circuitID int64) {
	select {
	case m.CircuitIDCh <- circuitID:
	default:
	}
}

var ConnMap = NewConnManager()

// GlobalConf ...
type GlobalConf struct {
	AppVersion           rwmtx.RWMtx[*version.Version]
	ProjectPath          rwmtx.RWMtx[string] // project path
	ProjectLocalsPath    rwmtx.RWMtx[string] // directory where we keep custom translation files
	ProjectHTMLPath      rwmtx.RWMtx[string]
	ProjectMemesPath     rwmtx.RWMtx[string]
	ProjectUploadsPath   rwmtx.RWMtx[string]
	ProjectFiledropPath  rwmtx.RWMtx[string]
	ProjectDownloadsPath rwmtx.RWMtx[string]
	Sha                  rwmtx.RWMtx[string]
	MasterKey            rwmtx.RWMtx[string]
	CookieSecure         rwmtx.RWMtx[bool]
	CookieDomain         rwmtx.RWMtx[string]
	BaseURL              rwmtx.RWMtx[string] // (http://127.0.0.1:8080)
}

// NewGlobalConf ...
func NewGlobalConf() *GlobalConf {
	c := new(GlobalConf)
	c.MasterKey.Set(DefaultMasterKey)
	return c
}
