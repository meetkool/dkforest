package handlers

import (
	"dkforest/pkg/managers"
	v1 "dkforest/pkg/web/handlers/api/v1"
	"time"

	"dkforest/pkg/database"
	"dkforest/pkg/utils"
)

type firstUseData struct {
	Username   string
	Email      string
	Password   string
	RePassword string
	Errors     database.UserErrors
}

type homeData struct {
}

type homeAttackData struct {
}

type loginData struct {
	Autofocus       int64
	Username        string
	Password        string
	Error           string
	HomeUsersList   bool
	CaptchaRequired bool
	ErrCaptcha      string
	CaptchaID       string
	CaptchaImg      string
	Online          []managers.UserInfo
}

type sessionsTwoFactorData struct {
	Token string
	Error string
}

type sessionsGpgTwoFactorData struct {
	Token            string
	EncryptedMessage string
	Code             string
	Error            string
	ErrorCode        string
}

type sessionsGpgSignTwoFactorData struct {
	Token              string
	ToBeSignedMessage  string
	SignedMessage      string
	Error              string
	ErrorSignedMessage string
}

type sessionsTwoFactorRecoveryData struct {
	Token string
	Error string
}

type waitData struct {
	WaitTime  int64
	Frames    []string
	WaitToken string
}

type signupData struct {
	Username         string
	Password         string
	RePassword       string
	CaptchaImg       string
	CaptchaID        string
	Captcha          string
	CaptchaSec       int64
	Frames           []string
	HasSolvedCaptcha bool
	ErrCaptcha       string
	Errors           database.UserErrors
}

type byteRoadChallengeData struct {
	ActiveTab            string
	CaptchaImg           string
	CaptchaID            string
	Captcha              string
	Username             string
	Password             string
	ErrCaptcha           string
	ErrRegistration      string
	CaptchaSolved        bool
	Registered           bool
	FlagFound            bool
	NbAccountsRegistered int64
	SessionExp           time.Duration
}

type forgotPasswordData struct {
	Error              string
	Username           string
	UsernameError      string
	Frames             []string
	CaptchaSec         int64
	Captcha            string
	CaptchaID          string
	CaptchaImg         string
	ErrCaptcha         string
	GpgMode            bool
	ToBeSignedMessage  string
	SignedMessage      string
	ErrorSignedMessage string
	EncryptedMessage   string
	Code               string
	ErrorCode          string
	Token              string
	Step               int64
	NewPassword        string
	ErrorNewPassword   string
	RePassword         string
	ErrorRePassword    string
}

type forgotPasswordResetData struct {
	Error      string
	Username   string
	Password   string
	RePassword string
}

type newsData struct {
}

type gistData struct {
	Gist        database.Gist
	Highlighted string
	Error       string
}

type ForumThreadSearch struct {
	UUID             string
	Name             string
	Author           string
	AuthorChatColor  string
	LastMsgAuthor    string
	LastMsgChatColor string
	LastMsgChatFont  string
	LastMsgCreatedAt time.Time
	RepliesCount     int64
	CreatedAt        time.Time
}

type ForumMessageSearch struct {
	UUID            string
	ThreadUUID      string
	ThreadName      string
	Message         string
	Snippet         string
	Author          string
	AuthorChatFont  string
	AuthorChatColor string
	CreatedAt       time.Time
}

type forumSearchData struct {
	Search        string
	ForumThreads  []ForumThreadSearch
	ForumMessages []ForumMessageSearch
}

type linksUploadData struct {
	CsvStr string
	Error  string
}

type newLinkData struct {
	IsEdit              bool
	Link                string
	Title               string
	Description         string
	Shorthand           string
	Categories          string
	Tags                string
	ErrorLink           string
	ErrorTitle          string
	ErrorDescription    string
	ErrorShorthand      string
	ErrorCategories     string
	ErrorTags           string
	PGPTitle            string
	PGPDescription      string
	PGPPublicKey        string
	ErrorPGPTitle       string
	ErrorPGPDescription string
	ErrorPGPPublicKey   string
	MirrorLink          string
	ErrorMirrorLink     string
	LinkPgps            []database.LinksPgp
	Mirrors             []database.LinksMirror
}

type editLinkData struct {
	IsEdit              bool
	Link                string
	Title               string
	Description         string
	Shorthand           string
	Categories          string
	Tags                string
	ErrorLink           string
	ErrorTitle          string
	ErrorDescription    string
	ErrorShorthand      string
	ErrorCategories     string
	ErrorTags           string
	PGPTitle            string
	PGPDescription      string
	PGPPublicKey        string
	ErrorPGPTitle       string
	ErrorPGPDescription string
	ErrorPGPPublicKey   string
	MirrorLink          string
	ErrorMirrorLink     string
	LinkPgps            []database.LinksPgp
	Mirrors             []database.LinksMirror
}

type linkData struct {
	Link    database.Link
	PgpKeys []database.LinksPgp
	Mirrors []database.LinksMirror
}

type linksData struct {
	Categories  []database.CategoriesResult
	Links       []database.Link
	LinksCount  int64
	CurrentPage int64
	MaxPage     int64
	Search      string
}

type forumData struct {
	ForumCategories []database.ForumCategory
	ForumThreads    []database.ForumThreadAug
}

type forumCategoryData struct {
	ForumCategories []database.ForumCategory
	ForumThreads    []database.ForumThreadAug
}

type threadData struct {
	Thread        database.ForumThread
	Messages      []database.ForumMessage
	MessagesCount int64
	CurrentPage   int64
	MaxPage       int64
	Search        string
	IsSubscribed  bool
}

type deleteForumMessageData struct {
	Thread  database.ForumThread
	Message database.ForumMessage
}

type deleteLinkData struct {
	Link database.Link
}

type deleteLinkPgpData struct {
	Link    database.Link
	LinkPgp database.LinksPgp
}

type deleteLinkMirrorData struct {
	Link       database.Link
	LinkMirror database.LinksMirror
}

type editForumThreadData struct {
	Thread database.ForumThread
}

type deleteForumThreadData struct {
	Thread database.ForumThread
}

type clubData struct {
	ActiveTab    string
	ForumThreads []database.ForumThreadAug
}

type threadReplyData struct {
	IsEdit       bool
	Thread       database.ForumThread
	Message      string
	ErrorMessage string
}

type clubNewThreadReplyData struct {
	ActiveTab    string
	IsEdit       bool
	Thread       database.ForumThread
	Message      string
	ErrorMessage string
}

type newThreadData struct {
	ThreadName      string
	Message         string
	ErrorMessage    string
	ErrorThreadName string
}

type clubNewThreadData struct {
	ActiveTab       string
	ThreadName      string
	Message         string
	ErrorMessage    string
	ErrorThreadName string
}

type clubMembersData struct {
	ActiveTab string
	Members   []database.User
}

type clubThreadData struct {
	ActiveTab string
	Thread    database.ForumThread
	Messages  []database.ForumMessage
}

type vipData struct {
	ActiveTab   string
	UsersBadges []database.UserBadge
}

type roomsData struct {
	Rooms []database.ChatRoomAug
}

type chatData struct {
	Error            string
	RoomPassword     string
	GuestUsername    string
	Room             database.ChatRoom
	IsOfficialRoom   bool
	DisplayTutorial  bool
	Multiline        bool
	ChatQueryParams  string
	ToggleMentions   bool
	TogglePms        int64
	RedRoom          bool
	IsSubscribed     bool
	CaptchaID        string
	CaptchaImg       string
	ErrGuestUsername string
	ErrCaptcha       string
}

type chatHelpData struct {
}

type chatDeleteData struct {
	Room database.ChatRoom
}

type chatArchiveData struct {
	Room          database.ChatRoom
	Messages      database.ChatMessages
	DateFormat    string
	MessagesCount int64
	CurrentPage   int64
	MaxPage       int64
}

type roomChatSettingsData struct {
	Room database.ChatRoom
}

type chatCreateRoomData struct {
	RoomName      string
	Password      string
	IsListed      bool
	IsEphemeral   bool
	Error         string
	ErrorRoomName string
	CaptchaImg    string
	CaptchaID     string
	Captcha       string
	ErrCaptcha    string
}

type captchaData struct {
	Ts         int64
	Seed       int64
	CaptchaSec int64
	Frames     []string
	CaptchaImg string
	CaptchaID  string
	Captcha    string
	Success    string
	Error      string
}

type captchaRequiredData struct {
	CaptchaID  string
	CaptchaImg string
	ErrCaptcha string
}

type bhcData struct {
	CaptchaImg string
	CaptchaID  string
	Captcha    string
	Success    string
	Error      string
}

type adminData struct {
	ActiveTab   string
	Query       string
	Users       []database.User
	UsersCount  int64
	CurrentPage int64
	MaxPage     int64
	Error       string
}

type adminSessionsData struct {
	ActiveTab     string
	Query         string
	Sessions      []database.Session
	SessionsCount int64
	CurrentPage   int64
	MaxPage       int64
	Error         string
}

type adminIgnoredData struct {
	ActiveTab    string
	Query        string
	Ignored      []database.IgnoredUser
	IgnoredCount int64
	CurrentPage  int64
	MaxPage      int64
	Error        string
}

type backupData struct {
	ActiveTab string
	Error     string
}

type adminDdosData struct {
	ActiveTab                string
	RPS                      int64
	RejectedReq              int64
	SignupPageLoad           int64
	SignupFailed             int64
	SignupSucceed            int64
	BHCCaptchaGenerated      int64
	BHCCaptchaSuccess        int64
	BHCCaptchaFailed         int64
	CaptchaRequiredGenerated int64
	CaptchaRequiredSuccess   int64
	CaptchaRequiredFailed    int64
}

type adminAuditsData struct {
	ActiveTab      string
	AuditLogs      []database.AuditLog
	AuditLogsCount int64
	CurrentPage    int64
	MaxPage        int64
	Error          string
}

type adminRoomsData struct {
	ActiveTab   string
	Query       string
	Rooms       []database.ChatRoom
	RoomsCount  int64
	CurrentPage int64
	MaxPage     int64
	Error       string
}

type adminCaptchaData struct {
	ActiveTab     string
	Query         string
	Captchas      []database.CaptchaRequest
	CaptchasCount int64
	CurrentPage   int64
	MaxPage       int64
	Error         string
}

type adminSettingsData struct {
	ActiveTab         string
	ProtectHome       bool
	HomeUsersList     bool
	ForceLoginCaptcha bool
	SignupEnabled     bool
	SignupFakeEnabled bool
	DownloadsEnabled  bool
	ForumEnabled      bool
	MaybeAuthEnabled  bool
	CaptchaDifficulty int64
}

type settingsPGPData struct {
	ActiveTab      string
	PGPPublicKeyID string
}

type settingsAgeData struct {
	ActiveTab    string
	AgePublicKey string
}

type addPGPData struct {
	GpgMode            bool
	SignedMessage      string
	ToBeSignedMessage  string
	ErrorSignedMessage string
	PGPPublicKey       string
	ErrorPGPPublicKey  string
	Error              string
	Code               string
	EncryptedMessage   string
	ErrorCode          string
}

type addAgeData struct {
	AgePublicKey      string
	ErrorAgePublicKey string
	Error             string
	Code              string
	EncryptedMessage  string
	ErrorCode         string
}

type diableTotpData struct {
	IsEnabled     bool
	Password      string
	ErrorPassword string
}

type gpgTwoFactorAuthenticationVerifyData struct {
	IsEnabled        bool
	GpgTwoFactorMode bool
	Password         string
	ErrorPassword    string
}

type twoFactorAuthenticationVerifyData struct {
	QRCode        string
	Secret        string
	RecoveryCode  string
	Password      string
	Error         string
	ErrorPassword string
}

type settingsChatPMData struct {
	ActiveTab        string
	PmMode           int64
	BlockNewUsersPm  bool
	AddWhitelist     string
	AddBlacklist     string
	WhitelistedUsers []database.PmWhitelistedUsers
	BlacklistedUsers []database.PmBlacklistedUsers
	Error            string
}

type settingsChatIgnoreData struct {
	ActiveTab    string
	PmMode       int64
	AddIgnored   string
	IgnoredUsers []database.IgnoredUser
	Error        string
}

type settingsChatSnippetsData struct {
	ActiveTab string
	Snippets  []database.Snippet
	Name      string
	Text      string
	Error     string
}

type shopData struct {
	Img     string
	Invoice database.XmrInvoice
}

type settingsChatData struct {
	ActiveTab                   string
	ChatColor                   string
	ChatFont                    int64
	RefreshRate                 int64
	ChatBold                    bool
	DateFormat                  int64
	ChatItalic                  bool
	AllFonts                    []utils.Font
	ChatReadMarkerEnabled       bool
	ChatReadMarkerColor         string
	ChatReadMarkerSize          int64
	DisplayHellbanned           bool
	DisplayModerators           bool
	HideIgnoredUsersFromList    bool
	HideRightColumn             bool
	ChatBarAtBottom             bool
	AutocompleteCommandsEnabled bool
	AfkIndicatorEnabled         bool
	DisplayDeleteButton         bool
	DisplayKickButton           bool
	DisplayHellbanButton        bool
	NotifyChessGames            bool
	NotifyChessMove             bool
	NotifyNewMessage            bool
	NotifyTagged                bool
	NotifyPmmed                 bool
	NotifyNewMessageSound       int64
	NotifyTaggedSound           int64
	NotifyPmmedSound            int64
	Theme                       int64
	Error                       string
}

type settingsUploadsData struct {
	ActiveTab string
	TotalSize int64
	Files     []database.Upload
}

type uploadsDownloadData struct {
	CaptchaID  string
	CaptchaImg string
	ErrCaptcha string
}

type settingsPublicNotesData struct {
	ActiveTab string
	Notes     database.UserPublicNote
	Error     string
}

type settingsPrivateNotesData struct {
	ActiveTab string
	Notes     database.UserPrivateNote
	Error     string
}

type InboxTmp struct {
	IsNotif bool
	database.ChatInboxMessage
	database.Notification
	database.SessionNotification
}

type settingsInboxData struct {
	ActiveTab            string
	Notifs               []InboxTmp
	ChatMessages         []database.ChatInboxMessage
	Notifications        []database.Notification
	SessionNotifications []database.SessionNotification
}

type settingsInboxSentData struct {
	ActiveTab     string
	ChatInboxSent []database.ChatInboxMessage
}

type WrapperSession struct {
	database.Session
	CurrentSession bool
}

type settingsSecurityData struct {
	ActiveTab string
	Logs      []database.SecurityLog
}

type settingsSessionsData struct {
	ActiveTab string
	Sessions  []WrapperSession
}

type loginCompletedData struct {
	SecretPhrase string
	RedirectURL  string
}

type settingsSecretPhraseData struct {
	ActiveTab            string
	CurrentPassword      string
	SecretPhrase         string
	ErrorCurrentPassword string
	ErrorSecretPhrase    string
}

type settingsPasswordData struct {
	ActiveTab              string
	OldPassword            string
	NewPassword            string
	RePassword             string
	ErrorOldPassword       string
	ErrorNewPassword       string
	ErrorRePassword        string
	OldDuressPassword      string
	NewDuressPassword      string
	ReDuressPassword       string
	ErrorOldDuressPassword string
	ErrorNewDuressPassword string
	ErrorReDuressPassword  string
}

type settingsAccountData struct {
	AccountTooYoungErrorString   string
	ActiveTab                    string
	Username                     string
	Website                      string
	Email                        string
	LastSeenPublic               bool
	TerminateAllSessionsOnLogout bool
	Error                        string
	ErrorLang                    string
	ErrorUsername                string
	ErrorAvatar                  string
	ErrorEmail                   string
	ErrorWebsite                 string
}

type settingsInvitationsData struct {
	ActiveTab   string
	Invitations []database.Invitation
	Error       string
	DkfOnion    string
}

type settingsWebsiteData struct {
	ActiveTab      string
	SignupEnabled  bool
	SilentSelfKick bool
	ForumEnabled   bool
	Error          string
}

type adminEditUsereData struct {
	IsEdit            bool
	ActiveTab         string
	User              database.User
	Username          string
	Password          string
	RePassword        string
	ApiKey            string
	Role              string
	IsAdmin           bool
	IsHellbanned      bool
	Verified          bool
	IsClubMember      bool
	CanUploadFile     bool
	CanUseForum       bool
	CanUseMultiline   bool
	CanSeeHellbanned  bool
	IsIncognito       bool
	CanChangeUsername bool
	CanChangeColor    bool
	Vetted            bool
	ChatColor         string
	ChatFont          int64
	SignupMetadata    string
	CollectMetadata   bool
	ChatTutorial      int64
	Errors            database.UserErrors
	AllFonts          []utils.Font
}

type adminEditRoomData struct {
	IsEdit      bool
	ActiveTab   string
	IsEphemeral bool
	IsListed    bool
}

type bhcliDownloadsHandlerData struct {
	Files []downloadableFileInfo
}

type vipDownloadsHandlerData struct {
	ActiveTab   string
	Files       []downloadableFileInfo
	FlagMessage string
}

type adminUploadsData struct {
	ActiveTab string
	Uploads   []database.Upload
	TotalSize int64
}

type adminFiledropsData struct {
	ActiveTab string
	Filedrops []database.Filedrop
	TotalSize int64
}

type adminDownloadsData struct {
	ActiveTab      string
	Downloads      []database.Download
	DownloadsCount int64
	CurrentPage    int64
	MaxPage        int64
}

type adminGistsData struct {
	ActiveTab   string
	Gists       []database.Gist
	GistsCount  int64
	CurrentPage int64
	MaxPage     int64
}

type adminCreateGistData struct {
	ActiveTab  string
	IsEdit     bool
	Name       string
	Content    string
	Password   string
	Error      string
	ErrorName  string
	CaptchaImg string
	CaptchaID  string
	Captcha    string
	ErrCaptcha string
}

type publicProfileData struct {
	User        database.User
	PublicNotes database.UserPublicNote
	UserStyle   string
}

type fileDropData struct {
	Error string
}

type stego1RoadChallengeData struct {
	ActiveTab   string
	FlagMessage string
}

type chessData struct {
	Games    []v1.ChessGame
	Error    string
	Username string
}
