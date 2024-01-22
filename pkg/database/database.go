package database

import (
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type DkfDB struct {
	db *gorm.DB
}

func (d *DkfDB) DB() *gorm.DB {
	return d.db
}

// Compile time checks to ensure type satisfies IDkfDB interface
var _ IDkfDB = (*DkfDB)(nil)

type IDkfDB interface {
	AddBlacklistedUser(userID, blacklistedUserID UserID)
	AddLinkCategory(linkID, categoryID int64) (err error)
	AddLinkTag(linkID, tagID int64) (err error)
	AddUserToRoomGroup(roomID RoomID, groupID GroupID, userID UserID) (out ChatRoomUserGroup, err error)
	AddWhitelistedUser(userID, whitelistedUserID UserID)
	CanRenameTo(oldUsername, newUsername Username) error
	CanUseUsername(username Username, isFirstUser bool) error
	ClearRoomGroup(roomID RoomID, groupID GroupID) (err error)
	CreateChatReaction(userID UserID, messageID, reaction int64) error
	CreateChatRoomGroup(roomID RoomID, name, color string) (out ChatRoomGroup, err error)
	CreateDownload(userID UserID, filename string) (out Download, err error)
	CreateEncryptedUploadWithSize(fileName string, content []byte, userID UserID, size int64) (*Upload, error)
	CreateFiledrop() (out Filedrop, err error)
	CreateInboxMessage(msg string, roomID RoomID, fromUserID, toUserID UserID, isPm, moderators bool, msgID *int64)
	CreateInvitation(userID UserID) (out Invitation, err error)
	CreateKarmaHistory(karma int64, description string, userID UserID, fromUserID *int64) (out KarmaHistory, err error)
	CreateKickMsg(kickedUser, kickedByUser User)
	CreateLink(url, title, description, shorthand string) (out Link, err error)
	CreateLinkMirror(linkID int64, link string) (out LinksMirror, err error)
	CreateLinkPgp(linkID int64, title, description, publicKey string) (out LinksPgp, err error)
	CreateLinksCategory(category string) (out LinksCategory, err error)
	CreateLinksTag(tag string) (out LinksTag, err error)
	CreateMsg(raw, txt, roomKey string, roomID RoomID, userID UserID, toUserID *UserID) (out ChatMessage, err error)
	CreateNotification(msg string, userID UserID)
	CreateOrEditMessage(editMsg *ChatMessage, message, raw, roomKey string, roomID RoomID, fromUserID UserID, toUserID *UserID, upload *Upload, groupID *GroupID, hellbanMsg, modMsg, systemMsg bool) (int64, error)
	CreateRoom(name string, passwordHash string, ownerID UserID, isListed bool) (out ChatRoom, err error)
	CreateSecurityLog(userID UserID, typ int64)
	CreateSession(userID UserID, userAgent string, sessionDuration time.Duration) (Session, error)
	CreateSessionNotification(msg string, sessionToken string)
	CreateSnippet(userID UserID, name, text string) (out Snippet, err error)
	CreateSysMsg(raw, txt, roomKey string, roomID RoomID, userID UserID) error
	CreateUnkickMsg(kickedUser, kickedByUser User)
	CreateUpload(fileName string, content []byte, userID UserID) (*Upload, error)
	CreateUser(username, password, repassword string, registrationDuration int64, signupInfoEnc string) (User, UserErrors)
	CreateUserBadge(userID UserID, badgeID int64) error
	CreateXmrInvoice(userID UserID, productID int64) (out XmrInvoice, err error)
	DeWhitelistUser(roomID RoomID, userID UserID) (err error)
	DeleteAllChatInbox(userID UserID) error
	DeleteAllNotifications(userID UserID) error
	DeleteAllSessionNotifications(sessionToken string) error
	DeleteChatInboxMessageByChatMessageID(chatMessageID int64) error
	DeleteChatInboxMessageByID(messageID int64) error
	DeleteChatMessageByUUID(messageUUID string) error
	DeleteChatRoomByID(id RoomID)
	DeleteChatRoomGroup(roomID RoomID, name string) (err error)
	DeleteChatRoomGroups(roomID RoomID) (err error)
	DeleteChatRoomMessages(roomID RoomID) error
	DeleteDownloadByID(downloadID int64) (err error)
	DeleteForumMessageByID(messageID ForumMessageID) error
	DeleteForumThreadByID(threadID ForumThreadID) error
	DeleteLinkByID(id int64) error
	DeleteLinkCategories(linkID int64) error
	DeleteLinkMirrorByID(id int64) error
	DeleteLinkPgpByID(id int64) error
	DeleteLinkTags(linkID int64) error
	DeleteNotificationByID(notificationID int64) error
	DeleteOldAuditLogs()
	DeleteOldCaptchaRequests()
	DeleteOldChatMessages()
	DeleteOldPrivateChatRooms()
	DeleteOldSecurityLogs()
	DeleteOldSessions()
	DeleteOldUploads()
	DeleteReaction(userID UserID, messageID, reaction int64) error
	DeleteSessionByToken(token string) error
	DeleteSessionNotificationByID(sessionNotificationID int64) error
	DeleteSnippet(userID UserID, name string)
	DeleteUserByID(userID UserID) (err error)
	DeleteUserChatInboxMessages(userID UserID) error
	DeleteUserChatMessages(userID UserID) error
	DeleteUserOtherSessions(userID UserID, currentToken string) error
	DeleteUserSessionByToken(userID UserID, token string) error
	DeleteUserSessions(userID UserID) error
	DoCreateSession(userID UserID, userAgent string, sessionDuration time.Duration) Session
	GetActiveUserSessions(userID UserID) (out []Session)
	GetCategories() (out []CategoriesResult, err error)
	GetChatMessages(roomID RoomID, roomKey string, username Username, userID UserID, pmUserID *UserID, displayPms PmDisplayMode, mentionsOnly, displayHellbanned, displayIgnored, displayModerators, displayIgnoredMessages bool, msgsLimit, minID1 int64) (out ChatMessages, err error)
	GetChatRoomByID(roomID RoomID) (out ChatRoom, err error)
	GetChatRoomByName(roomName string) (out ChatRoom, err error)
	GetChessSubscribers() (out []User, err error)
	GetClubForumThreads(userID UserID) (out []ForumThreadAug, err error)
	GetClubMembers() (out []User, err error)
	GetFiledropByFileName(fileName string) (out Filedrop, err error)
	GetFiledropByUUID(uuid string) (out Filedrop, err error)
	GetFiledrops() (out []Filedrop, err error)
	GetForumCategories() (out []ForumCategory, err error)
	GetForumCategoryBySlug(slug string) (out ForumCategory, err error)
	GetForumMessage(messageID ForumMessageID) (out ForumMessage, err error)
	GetForumMessageByUUID(messageUUID ForumMessageUUID) (out ForumMessage, err error)
	GetForumThread(threadID ForumThreadID) (out ForumThread, err error)
	GetForumThreadByID(threadID ForumThreadID) (out ForumThread, err error)
	GetForumThreadByUUID(threadUUID ForumThreadUUID) (out ForumThread, err error)
	GetForumThreads() (out []ForumThread, err error)
	GetGistByUUID(uuid string) (out Gist, err error)
	GetIgnoredByUsers(userID UserID) (out []IgnoredUser, err error)
	GetIgnoredUsers(userID UserID) (out []IgnoredUser, err error)
	GetLinkByID(linkID int64) (out Link, err error)
	GetLinkByShorthand(shorthand string) (out Link, err error)
	GetLinkByUUID(linkUUID string) (out Link, err error)
	GetLinkCategories(linkID int64) (out []LinksCategory, err error)
	GetLinkMirrorByID(id int64) (out LinksMirror, err error)
	GetLinkMirrors(linkID int64) (out []LinksMirror, err error)
	GetLinkPgpByID(id int64) (out LinksPgp, err error)
	GetLinkPgps(linkID int64) (out []LinksPgp, err error)
	GetLinkTags(linkID int64) (out []LinksTag, err error)
	GetLinks() (out []Link, err error)
	GetListedChatRooms(userID UserID) (out []ChatRoomAug, err error)
	GetMemeByFileName(filename string) (out Meme, err error)
	GetMemeByID(memeID MemeID) (out Meme, err error)
	GetMemeBySlug(slug string) (out Meme, err error)
	GetMemes() (out []Meme, err error)
	GetModeratorsUsers() (out []User, err error)
	GetOfficialChatRooms() (out []ChatRoom, err error)
	GetOfficialChatRooms1(userID UserID) (out []ChatRoomAug1, err error)
	GetOnionBlacklist(hash string) (out OnionBlacklist, err error)
	GetPmBlacklistedByUsers(userID UserID) (out []PmBlacklistedUsers, err error)
	GetPmBlacklistedUsers(userID UserID) (out []PmBlacklistedUsers, err error)
	GetPmWhitelistedUsers(userID UserID) (out []PmWhitelistedUsers, err error)
	GetPublicForumCategoryThreads(userID UserID, categoryID ForumCategoryID) (out []ForumThreadAug, err error)
	GetPublicForumThreadsSearch(userID UserID) (out []ForumThreadAug, err error)
	GetRecentLinks() (out []Link, err error)
	GetRecentUsersCount() int64
	GetRoomChatMessageByDate(roomID RoomID, userID UserID, dt time.Time) (out ChatMessage, err error)
	GetRoomChatMessageByUUID(roomID RoomID, msgUUID string) (out ChatMessage, err error)
	GetRoomChatMessages(roomID RoomID) (out ChatMessages, err error)
	GetRoomChatMessagesByDate(roomID RoomID, dt time.Time) (out []ChatMessage, err error)
	GetRoomGroupByName(roomID RoomID, groupName string) (out ChatRoomGroup, err error)
	GetRoomGroupUsers(roomID RoomID, groupID GroupID) (out []ChatRoomUserGroup, err error)
	GetRoomGroups(roomID RoomID) (out []ChatRoomGroup, err error)
	GetSecurityLogs(userID UserID) (out []SecurityLog, err error)
	GetSettings() (out Settings)
	GetThreadMessages(threadID ForumThreadID) (out []ForumMessage, err error)
	GetUnusedInvitationByToken(token string) (out Invitation, err error)
	GetUploadByFileName(filename string) (out Upload, err error)
	GetUploadByID(uploadID UploadID) (out Upload, err error)
	GetUploads() (out []Upload, err error)
	GetUserByApiKey(user *User, apiKey string) error
	GetUserByID(userID UserID) (out User, err error)
	GetUserBySessionKey(user *User, sessionKey string) error
	GetUserByUsername(username Username) (out User, err error)
	GetUserChatInboxMessages(userID UserID) (msgs []ChatInboxMessage, err error)
	GetUserChatInboxMessagesSent(userID UserID) (msgs []ChatInboxMessage, err error)
	GetUserInboxMessagesCount(userID UserID) (count int64)
	GetUserInvitations(userID UserID) (out []Invitation, err error)
	GetUserLastChatMessageInRoom(userID UserID, roomID RoomID) (out ChatMessage, err error)
	GetUserNotifications(userID UserID) (msgs []Notification, err error)
	GetUserNotificationsCount(userID UserID) (count int64)
	GetUserPrivateNotes(userID UserID) (out UserPrivateNote, err error)
	GetUserPublicNotes(userID UserID) (out UserPublicNote, err error)
	GetUserReadMarker(userID UserID, roomID RoomID) (out ChatReadMarker, err error)
	GetUserRoomGroups(userID UserID, roomID RoomID) (out []ChatRoomUserGroup, err error)
	GetUserRoomSubscriptions(userID UserID) (out []ChatRoomAug1, err error)
	GetUserSessionNotifications(sessionToken string) (msgs []SessionNotification, err error)
	GetUserSessionNotificationsCount(sessionToken string) (count int64)
	GetUserSnippets(userID UserID) (out []Snippet, err error)
	GetUserTotalUploadSize(userID UserID) int64
	GetUserUnusedInvitations(userID UserID) (out []Invitation, err error)
	GetUserUploads(userID UserID) (out []Upload, err error)
	GetUsersBadges() (out []UserBadge, err error)
	GetUsersByID(ids []UserID) (out []User, err error)
	GetUsersByUsername(usernames []string) (out []User, err error)
	GetUsersSubscribedToForumThread(threadID ForumThreadID) (out []UserForumThreadSubscription, err error)
	GetVerifiedUserBySessionID(token string) (out User, err error)
	GetVerifiedUserByUsername(username Username) (out User, err error)
	GetWhitelistedUsers(roomID RoomID) (out []ChatRoomWhitelistedUser, err error)
	GetXmrInvoiceByAddress(address string) (out XmrInvoice, err error)
	IgnoreMessage(userID UserID, messageID int64)
	IgnoreUser(userID, ignoredUserID UserID)
	IsPasswordProhibited(password string) bool
	IsUserInGroupByID(userID UserID, groupID GroupID) bool
	IsUserPmBlacklisted(fromUserID, toUserID UserID) bool
	IsUserPmWhitelisted(fromUserID, toUserID UserID) bool
	IsUserSubscribedToForumThread(userID UserID, threadID ForumThreadID) bool
	IsUserSubscribedToRoom(userID UserID, roomID RoomID) bool
	IsUserWhitelistedInRoom(userID UserID, roomID RoomID) bool
	IsUsernameAlreadyTaken(username Username) bool
	NewAudit(authUser User, log string)
	RmBlacklistedUser(userID, blacklistedUserID UserID)
	RmUserFromRoomGroup(roomID RoomID, groupID GroupID, userID UserID) (err error)
	RmWhitelistedUser(userID, whitelistedUserID UserID)
	SetUserPrivateNotes(userID UserID, notes string) error
	SetUserPublicNotes(userID UserID, notes string) error
	SubscribeToForumThread(userID UserID, threadID ForumThreadID) (err error)
	SubscribeToRoom(userID UserID, roomID RoomID) (err error)
	ToggleBlacklistedUser(userID, blacklistedUserID UserID) bool
	ToggleWhitelistedUser(userID, whitelistedUserID UserID) bool
	UnIgnoreMessage(userID UserID, messageID int64)
	UnIgnoreUser(userID, ignoredUserID UserID)
	UnsubscribeFromForumThread(userID UserID, threadID ForumThreadID) (err error)
	UnsubscribeFromRoom(userID UserID, roomID RoomID) (err error)
	UpdateChatReadMarker(userID UserID, roomID RoomID)
	UpdateChatReadRecord(userID UserID, roomID RoomID)
	UpdateForumReadRecord(userID UserID, threadID ForumThreadID)
	UserNbDownloaded(userID UserID, filename string) (out int64)
	WhitelistUser(roomID RoomID, userID UserID) (out ChatRoomWhitelistedUser, err error)
}

func NewDkfDB(dbPath string) *DkfDB {
	conf := &gorm.Config{}
	if config.Development.IsTrue() {
		conf.Logger = logger.Default.LogMode(logger.Silent)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), conf)
	if err != nil {
		logrus.Fatal("Failed to open sqlite3 db : " + err.Error())
	}
	utils.Must(db.DB()).SetMaxIdleConns(1) // 10
	utils.Must(db.DB()).SetMaxOpenConns(1) // 25
	//db.LogMode(false)
	db.Exec("PRAGMA foreign_keys=ON")
	return &DkfDB{db: db}
}

// DB2 is the SQL database.
type DB2 struct {
	path     string // Path to database file.
	dsnQuery string // DSN query params, if any.
	memory   bool   // In-memory only.
	fqdsn    string // Fully-qualified DSN for opening SQLite.
}

// Conn represents a connection to a database. Two Connection objects
// to the same database are READ_COMMITTED isolated.
type Conn struct {
	sqlite *sqlite3.SQLiteConn
}

// Connect returns a connection to the database.
func (d *DB2) Connect() (*Conn, error) {
	drv := sqlite3.SQLiteDriver{}
	c, err := drv.Open(d.fqdsn)
	if err != nil {
		return nil, err
	}

	return &Conn{
		sqlite: c.(*sqlite3.SQLiteConn),
	}, nil
}

// New returns an instance of the database at path. If the database
// has already been created and opened, this database will share
// the data of that database when connected.
func New(path, dsnQuery string, memory bool) (*DB2, error) {
	q, err := url.ParseQuery(dsnQuery)
	if err != nil {
		return nil, err
	}
	if memory {
		q.Set("mode", "memory")
		q.Set("cache", "shared")
	}

	if !strings.HasPrefix(path, "file:") {
		path = fmt.Sprintf("file:%s", path)
	}

	var fqdsn string
	if len(q) > 0 {
		fqdsn = fmt.Sprintf("%s?%s", path, q.Encode())
	} else {
		fqdsn = path
	}

	return &DB2{
		path:     path,
		dsnQuery: dsnQuery,
		memory:   memory,
		fqdsn:    fqdsn,
	}, nil
}

const bkDelay = 250

// Backup the database
func Backup() error {
	projectPath := config.Global.ProjectPath.Get()
	dbPath := filepath.Join(projectPath, config.DbFileName)
	bckPath := filepath.Join(projectPath, "backup.db")
	srcDB, err := New(dbPath, "", false)
	if err != nil {
		return err
	}
	srcConn, err := srcDB.Connect()
	if err != nil {
		return err
	}

	dstDB, err := New(bckPath, "", false)
	if err != nil {
		return err
	}
	dstConn, err := dstDB.Connect()
	if err != nil {
		return err
	}

	bk, err := dstConn.sqlite.Backup("main", srcConn.sqlite, "main")
	if err != nil {
		return err
	}

	for {
		done, err := bk.Step(-1)
		if err != nil {
			_ = bk.Finish()
			return err
		}
		if done {
			break
		}
		time.Sleep(bkDelay * time.Millisecond)
	}

	return bk.Finish()
}

func (d *DkfDB) With(clb func(tx *DkfDB)) {
	_ = d.WithE(func(tx *DkfDB) error {
		clb(tx)
		return nil
	})
}

func (d *DkfDB) WithE(clb func(tx *DkfDB) error) error {
	tx := d.Begin()
	err := clb(tx)
	if err != nil {
		tx.Rollback()
	} else {
		tx.Commit()
	}
	return err
}

func (d *DkfDB) Begin() *DkfDB {
	return &DkfDB{db: d.db.Begin()}
}

func (d *DkfDB) Commit() *DkfDB {
	return &DkfDB{db: d.db.Commit()}
}

func (d *DkfDB) Rollback() *DkfDB {
	return &DkfDB{db: d.db.Rollback()}
}
