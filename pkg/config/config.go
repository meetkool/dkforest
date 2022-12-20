package config

import (
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
	DbFileName = "dkf.db"
	AppDirName = ".dkf"
)

// DefaultMasterKey Should be overwritten using ldflags
var DefaultMasterKey = "Ucn%1fw%bPz3<Ir}lJD6H!X+fP47j]c2"
var GistPasswordSalt = "gist_pa$$word_$alt_tdjfPAgjyNdor"
var RoomPasswordSalt = "room_pa$$word_$alt_OYvUwmNPVTdsw"

const (
	DkfOnion     = "http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion"
	DkfGitOnion  = "http://yylovpz7taca7jfrub3wltxabzzjp34fngj5lpwl6eo47ekt5cxs6mid.onion"
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
	sync.RWMutex
	appVersion        *version.Version
	projectPath       string // project path
	projectLocalsPath string // directory where we keep custom translation files
	projectHtmlPath   string
	sha               string
	masterKey         string
	cookieSecure      bool
	cookieDomain      string
	baseURL           string // (http://127.0.0.1:8080)
}

// NewGlobalConf ...
func NewGlobalConf() *GlobalConf {
	c := new(GlobalConf)
	c.masterKey = DefaultMasterKey
	return c
}

// ProjectPath ...
func (c *GlobalConf) ProjectPath() string {
	c.RLock()
	defer c.RUnlock()
	return c.projectPath
}

// SetProjectPath ...
func (c *GlobalConf) SetProjectPath(v string) {
	c.Lock()
	defer c.Unlock()
	c.projectPath = v
}

// MasterKey ...
func (c *GlobalConf) MasterKey() string {
	c.RLock()
	defer c.RUnlock()
	return c.masterKey
}

// SetMasterKey ...
func (c *GlobalConf) SetMasterKey(v string) {
	c.Lock()
	defer c.Unlock()
	c.masterKey = v
}

// GetVersion ...
func (c *GlobalConf) GetVersion() *version.Version {
	c.RLock()
	defer c.RUnlock()
	return c.appVersion
}

// SetVersion ...
func (c *GlobalConf) SetVersion(newVersion string) {
	c.Lock()
	defer c.Unlock()
	c.appVersion = version.Must(version.NewVersion(newVersion))
}

// Sha ...
func (c *GlobalConf) Sha() string {
	c.RLock()
	defer c.RUnlock()
	return c.sha
}

// SetSha ...
func (c *GlobalConf) SetSha(sha string) {
	c.Lock()
	defer c.Unlock()
	c.sha = sha
}

// CookieSecure ...
func (c *GlobalConf) CookieSecure() bool {
	c.RLock()
	defer c.RUnlock()
	return c.cookieSecure
}

// SetCookieSecure ...
func (c *GlobalConf) SetCookieSecure(cookieSecure bool) {
	c.Lock()
	defer c.Unlock()
	c.cookieSecure = cookieSecure
}

// CookieDomain ...
func (c *GlobalConf) CookieDomain() string {
	c.RLock()
	defer c.RUnlock()
	return c.cookieDomain
}

// SetCookieDomain ...
func (c *GlobalConf) SetCookieDomain(cookieDomain string) {
	c.Lock()
	defer c.Unlock()
	c.cookieDomain = cookieDomain
}

// ProjectLocalsPath ...
func (c *GlobalConf) ProjectLocalsPath() string {
	c.RLock()
	defer c.RUnlock()
	return c.projectLocalsPath
}

// SetProjectLocalsPath ...
func (c *GlobalConf) SetProjectLocalsPath(projectLocalsPath string) {
	c.Lock()
	defer c.Unlock()
	c.projectLocalsPath = projectLocalsPath
}

// ProjectHTMLPath ...
func (c *GlobalConf) ProjectHTMLPath() string {
	c.RLock()
	defer c.RUnlock()
	return c.projectHtmlPath
}

// SetProjectHTMLPath ...
func (c *GlobalConf) SetProjectHTMLPath(projectHtmlPath string) {
	c.Lock()
	defer c.Unlock()
	c.projectHtmlPath = projectHtmlPath
}

// BaseURL ...
func (c *GlobalConf) BaseURL() string {
	c.RLock()
	defer c.RUnlock()
	return c.baseURL
}

// SetBaseURL ...
func (c *GlobalConf) SetBaseURL(v string) {
	c.Lock()
	defer c.Unlock()
	c.baseURL = v
}
