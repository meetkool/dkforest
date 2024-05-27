package database

import (
	"dkforest/pkg/config"
	"errors"
	"fmt"
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"image"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dkforest/pkg/utils"
	"github.com/asaskevich/govalidator"
	"github.com/sirupsen/logrus"
)

type UserID int64

func (u UserID) String() string {
	return utils.FormatInt64(int64(u))
}

type Username string

func (u Username) String() string {
	return string(u)
}

func (u Username) AtStr() string {
	return "@" + string(u)
}

// IUserRenderMessage is the smallest interface needed to render the chat messages
type IUserRenderMessage interface {
	GetID() UserID
	GetUsername() Username
	GetRefreshRate() int64
	GetChatColor() string
	GetIsIncognito() bool
	GetIsHellbanned() bool
	GetAFK() bool
	GetAfkIndicatorEnabled() bool
	GetDisplayIgnored() bool
	GetDisplayAliveIndicator() bool
	GetDisplayModerators() bool
	GetNotifyNewMessage() bool
	GetNotifyTagged() bool
	GetNotifyPmmed() bool
	GetChatReadMarkerEnabled() bool
	GetHighlightOwnMessages() bool
	GetDisplayDeleteButton() bool
	GetIsAdmin() bool
	GetDisplayHellbanButton() bool
	GetDisplayKickButton() bool
	GetDisplayHellbanned() bool
	GetSyntaxHighlightCode() string
	GetDateFormat() string
	CanSeeHB() bool
	GetConfirmExternalLinks() bool
	GetCanSeeHellbanned() bool
	IsModerator() bool
	CountUIButtons() int64
}

// User struct an internal representation of a user for our app
type User struct {
	ID                           UserID
	Avatar                       []byte
	Username                     Username
	GPGPublicKey                 string
	AgePublicKey                 string
	Password                     string          `json:"-"`
	DuressPassword               string          `json:"-"`
	TwoFactorSecret              EncryptedString `json:"-"`
	TwoFactorRecovery            string          `json:"-"`
	SecretPhrase                 EncryptedString `json:"-"`
	GpgTwoFactorEnabled          bool
	GpgTwoFactorMode             bool // false -> decrypt; true -> sign
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
	DeletedAt                    *time.Time
	LastSeenAt                   time.Time
	IsUnderDuress                bool
	IsAdmin                      bool
	CanSeeHellbanned             bool
	IsHellbanned                 bool
	IsIncognito                  bool
	IsClubMember                 bool
	DisplayHellbanned            bool
	DisplayModerators            bool
	DisplayKickButton            bool
	DisplayHellbanButton         bool
	DisplayDeleteButton          bool
	PmMode                       int64 // Normal: 0, Whitelist 1
	DisplayPms                   int64 // deprecated
	HellbanOpacity               int64
	CodeBlockHeight              int64
	DisplayIgnored               bool
	DisplayAliveIndicator        bool
	HideIgnoredUsersFromList     bool
	Verified                     bool
	Temp                         bool // Temporary account
	Token                        *string
	Role                         string
	ApiKey                       string
	Lang                         string
	ChatColor                    string
	ChatBackgroundColor          string
	ChatFont                     int64
	ChatBold                     bool
	ChatItalic                   bool
	RefreshRate                  int64
	LoginAttempts                int64
	Karma                        int64
	NotifyChessGames             bool
	NotifyChessMove              bool
	NotifyNewMessage             bool
	NotifyTagged                 bool
	NotifyPmmed                  bool
	NotifyNewMessageSound        int64
	NotifyTaggedSound            int64
	NotifyPmmedSound             int64
	Email                        string
	Website                      string
	ChatTutorial                 int64
	ChatTutorialTime             time.Time
	ChatReadMarkerEnabled        bool
	ChatReadMarkerColor          string
	ChatReadMarkerSize           int64
	CanUploadFile                bool
	CanUseForum                  bool
	CanChangeUsername            bool
	CanUseUppercase              bool
	CanChangeColor               bool
	CanUseMultiline              bool
	ManualMultiline              bool
	CanUseChessAnalyze           bool
	Vetted                       bool
	RegistrationDuration         int64
	LastSeenPublic               bool
	TerminateAllSessionsOnLogout bool
	DateFormat                   int64
	BlockNewUsersPm              bool
	HideRightColumn              bool
	ChatBarAtBottom              bool
	AutocompleteCommandsEnabled  bool
	SpellcheckEnabled            bool
	AfkIndicatorEnabled          bool
	SignupMetadata               string
	CollectMetadata              bool
	CaptchaRequired              bool
	Theme                        int64
	GeneralMessagesCount         int64
	ChipsTest                    PokerChip
	XmrBalance                   Piconero
	AFK                          bool
	UseStream                    bool
	UseStreamMenu                bool
	SyntaxHighlightCode          string
	ConfirmExternalLinks         bool
	ChessSoundsEnabled           bool
	PokerSoundsEnabled           bool
	PokerXmrSubAddress           string
	PokerReferredBy              *UserID
	PokerReferralToken           *string
	PokerRakeBack                PokerChip
	HighlightOwnMessages         bool `gorm:"-"`
}

func (u *User) GetID() UserID                  { return u.ID }
func (u *User) GetUsername() Username          { return u.Username }
func (u *User) GetRefreshRate() int64          { return u.RefreshRate }
func (u *User) GetChatColor() string           { return u.ChatColor }
func (u *User) GetIsIncognito() bool           { return u.IsIncognito }
func (u *User) GetIsHellbanned() bool          { return u.IsHellbanned }
func (u *User) GetAFK() bool                   { return u.AFK }
func (u *User) GetAfkIndicatorEnabled() bool   { return u.AfkIndicatorEnabled }
func (u *User) GetDisplayIgnored() bool        { return u.DisplayIgnored }
func (u *User) GetDisplayAliveIndicator() bool { return u.DisplayAliveIndicator }
func (u *User) GetDisplayModerators() bool     { return u.DisplayModerators }
func (u *User) GetNotifyNewMessage() bool      { return u.NotifyNewMessage }
func (u *User) GetNotifyTagged() bool          { return u.NotifyTagged }
func (u *User) GetNotifyPmmed() bool           { return u.NotifyPmmed }
func (u *User) GetChatReadMarkerEnabled() bool { return u.ChatReadMarkerEnabled }
func (u *User) GetHighlightOwnMessages() bool  { return u.HighlightOwnMessages }
func (u *User) GetDisplayDeleteButton() bool   { return u.DisplayDeleteButton }
func (u *User) GetIsAdmin() bool               { return u.IsAdmin }
func (u *User) GetDisplayHellbanButton() bool  { return u.DisplayHellbanButton }
func (u *User) GetDisplayKickButton() bool     { return u.DisplayKickButton }
func (u *User) GetDisplayHellbanned() bool     { return u.DisplayHellbanned }
func (u *User) GetSyntaxHighlightCode() string { return u.SyntaxHighlightCode }
func (u *User) GetConfirmExternalLinks() bool  { return u.ConfirmExternalLinks }
func (u *User) GetCanSeeHellbanned() bool      { return u.CanSeeHellbanned }

const (
	ThemeDefault   = 0
	ThemeChristmas = 1
)

const (
	PmModeStandard  = 0
	PmModeWhitelist = 1
)

// UserPtrID given a User pointer, return the ID or nil
func UserPtrID(user *User) *UserID {
	if user != nil {
		return &user.ID
	}
	return nil
}

func (d *DkfDB) GetOnlineChessSubscribers(userIDs []UserID) (out []User, err error) {
	err = d.db.Find(&out, "notify_chess_games == 1 AND id IN (?)", userIDs).Error
	return
}

func (d *DkfDB) GetChessSubscribers() (out []User, err error) {
	err = d.db.Find(&out, "notify_chess_games == 1").Error
	return
}

func (u *User) TutorialCompleted() bool {
	return u.ChatTutorial == 3
}

func (u *User) GetFont() string {
	switch u.ChatFont {
	case 1:
		return `'Courier New', Courier, monospace`
	case 2:
		return `Arial,Helvetica,sans-serif`
	case 3:
		return `Georgia,'Times New Roman',Times,serif`
	case 4:
		return `'Book Antiqua','MS Gothic',serif`
	case 5:
		return `'Comic Sans MS',Papyrus,sans-serif`
	case 6:
		return `Cursive,Papyrus,sans-serif`
	case 7:
		return `Fantasy,Futura,Papyrus,sans`
	case 8:
		return `Garamond,Palatino,serif`
	case 9:
		return `'MS Serif','New York',serif`
	case 10:
		return `System,Chicago,sans-serif`
	case 11:
		return `'Times New Roman',Times,serif`
	case 12:
		return `Verdana,Geneva,Arial,Helvetica,sans-serif`
	default:
		return ""
	}
}

func (u *User) GetDateFormat() string {
	switch u.DateFormat {
	case 1:
		return "15:04:05"
	case 2:
		return "01-02 03:04:05"
	case 3:
		return "03:04:05"
	case 4:
		return ""
	default:
		return "01-02 15:04:05"
	}
}

func (u *User) AccountOldEnough() bool {
	return time.Since(u.CreatedAt) > 3*24*time.Hour
}

func (u *User) CanUseForumFn() bool {
	return u.CanUseForum && (u.AccountOldEnough() || u.Vetted)
}

func (u *User) CanUpload() bool {
	return u.CanUploadFile && (u.AccountOldEnough() || u.Vetted)
}

func (u *User) CountUIButtons() int64 {
	bools := []bool{u.DisplayDeleteButton}
	if u.IsModerator() {
		bools = append(bools, u.DisplayHellbanButton, u.DisplayKickButton)
	}
	return utils.CountBools(bools...)
}

func (u *User) generateBaseStyle() string {
	sb := strings.Builder{}
	sb.WriteString(`color: `)
	sb.WriteString(u.ChatColor)
	sb.WriteString(`; font-weight: `)
	if u.ChatBold {
		sb.WriteString(`bold`)
	} else {
		sb.WriteString(`normal`)
	}
	sb.WriteString(`; font-style: `)
	if u.ChatItalic {
		sb.WriteString(`italic`)
	} else {
		sb.WriteString(`normal`)
	}
	sb.WriteString(`;`)
	font := u.GetFont()
	if font != "" {
		sb.WriteString(` font-family: `)
		sb.WriteString(font)
		sb.WriteString(`;`)
	} else {
		sb.WriteString(` font-family: Arial,Helvetica,sans-serif;`)
	}
	return sb.String()
}

func (u *User) GenerateChatStyle() string {
	sb := strings.Builder{}
	sb.WriteString(`style="`)
	sb.WriteString(u.generateBaseStyle())
	sb.WriteString(` font-size: 14px;`)
	sb.WriteString(`"`)
	return sb.String()
}

func (u *User) GenerateChatStyle1() string {
	sb := strings.Builder{}
	sb.WriteString(`style="`)
	sb.WriteString(u.generateBaseStyle())
	sb.WriteString(`"`)
	return sb.String()
}

func (u *User) HasTotpEnabled() bool {
	return string(u.TwoFactorSecret) != ""
}

func (u *User) IsModerator() bool {
	return u.IsAdmin || u.Role == "moderator"
}

func (u *User) CanSeeHB() bool {
	return u.CanSeeHellbanned || u.IsModerator()
}

func (u *User) GetHellbanOpacityF64() float64 {
	return float64(u.HellbanOpacity) / 100
}

// Save user in the database
func (u *User) Save(db *DkfDB) error {
	return db.db.Save(u).Error
}

// DoSave user in the database, ignore error
func (u *User) DoSave(db *DkfDB) {
	if err := u.Save(db); err != nil {
		logrus.Error(err)
	}
}

func (u *User) DisableTotp2FA(db *DkfDB) {
	db.db.Model(u).Select("TwoFactorSecret", "TwoFactorRecovery").Updates(User{TwoFactorSecret: "", TwoFactorRecovery: ""})
}

func (u *User) DisableGpg2FA(db *DkfDB) {
	db.db.Model(u).Select("GpgTwoFactorEnabled").Updates(User{GpgTwoFactorEnabled: false})
}

func (u *User) SetAgePublicKey(db *DkfDB, agePublicKey string) {
	db.db.Model(u).Select("AgePublicKey").Updates(User{AgePublicKey: agePublicKey})
}

func (u *User) SetApiKey(db *DkfDB, apiKey string) {
	db.db.Model(u).Select("ApiKey").Updates(User{ApiKey: apiKey})
}

func (u *User) SetPokerReferralToken(db *DkfDB, pokerReferralToken *string) {
	db.db.Model(u).Select("PokerReferralToken").Updates(User{PokerReferralToken: pokerReferralToken})
}

func (u *User) SetPokerReferredBy(db *DkfDB, pokerReferredBy *UserID) {
	db.db.Model(u).Select("PokerReferredBy").Updates(User{PokerReferredBy: pokerReferredBy})
}

func (u *User) SetSignupMetadata(db *DkfDB, signupMetadata string) {
	db.db.Model(u).Select("SignupMetadata").Updates(User{SignupMetadata: signupMetadata})
}

func (u *User) ToggleAutocompleteCommandsEnabled(db *DkfDB) {
	db.db.Model(u).Select("AutocompleteCommandsEnabled").Updates(User{AutocompleteCommandsEnabled: !u.AutocompleteCommandsEnabled})
}

func (u *User) SetIsUnderDuress(db *DkfDB, isUnderDuress bool) {
	db.db.Model(u).Select("IsUnderDuress").Updates(User{IsUnderDuress: isUnderDuress})
}

func (u *User) SetCaptchaRequired(db *DkfDB, captchaRequired bool) {
	db.db.Model(u).Select("CaptchaRequired").Updates(User{CaptchaRequired: captchaRequired})
}

func (u *User) SetSyntaxHighlightCode(db *DkfDB, syntaxHighlightCode string) {
	db.db.Model(u).Select("SyntaxHighlightCode").Updates(User{SyntaxHighlightCode: syntaxHighlightCode})
}

func (u *User) DecrGeneralMessagesCount(db *DkfDB) {
	db.db.Model(u).Select("GeneralMessagesCount").Updates(User{GeneralMessagesCount: u.GeneralMessagesCount - 1})
}

func (u *User) SetVerified(db *DkfDB, verified bool) {
	db.db.Model(u).Select("Verified").Updates(User{Verified: verified})
}

func (u *User) IncrChatTutorial(db *DkfDB) {
	db.db.Model(u).Select("ChatTutorial").Updates(User{ChatTutorial: u.ChatTutorial + 1})
}

func (u *User) SetChatTutorialTime(db *DkfDB, chatTutorialTime time.Time) {
	db.db.Model(u).Select("ChatTutorialTime").Updates(User{ChatTutorialTime: chatTutorialTime})
}

func (u *User) SetCanUseForum(db *DkfDB, canUseForum bool) {
	db.db.Model(u).Select("CanUseForum").Updates(User{CanUseForum: canUseForum})
}

func (u *User) ResetLoginAttempts(db *DkfDB) {
	db.db.Model(u).Select("LoginAttempts").Updates(User{LoginAttempts: 0})
}

func (u *User) IncrLoginAttempts(db *DkfDB) {
	db.db.Model(u).Select("LoginAttempts").Updates(User{LoginAttempts: u.LoginAttempts + 1})
}

func (u *User) ResetChipsTest(db *DkfDB) {
	db.db.Model(u).Select("ChipsTest").Updates(User{ChipsTest: 1000})
}

func (u *User) SetPokerXmrSubAddress(db *DkfDB, newPokerXmrSubAddress string) {
	db.db.Model(u).Select("PokerXmrSubAddress").Updates(User{PokerXmrSubAddress: newPokerXmrSubAddress})
}

func (u *User) SetPmMode(db *DkfDB, pmMode int64) {
	db.db.Model(u).Select("PmMode").Updates(User{PmMode: pmMode})
}

func (u *User) ResetTutorial(db *DkfDB) {
	db.db.Model(u).Select("ChatTutorial").Updates(User{ChatTutorial: 0})
}

func (u *User) ToggleDisplayHellbanned(db *DkfDB) {
	db.db.Model(u).Update("DisplayHellbanned", !u.DisplayHellbanned)
}

func (u *User) ToggleDisplayModerators(db *DkfDB) {
	db.db.Model(u).Update("DisplayModerators", !u.DisplayModerators)
}

func (u *User) ToggleDisplayIgnored(db *DkfDB) {
	db.db.Model(u).Update("DisplayIgnored", !u.DisplayIgnored)
}

func (u *User) ToggleAFK(db *DkfDB) {
	db.db.Model(u).Update("AFK", !u.AFK)
}

func (u *User) HellBan(db *DkfDB) {
	u.setHellBan(db, true)
}

func (u *User) UnHellBan(db *DkfDB) {
	u.setHellBan(db, false)
}

func (u *User) setHellBan(db *DkfDB, hb bool) {
	db.db.Model(u).Select("IsHellbanned", "DisplayHellbanned").Updates(User{IsHellbanned: hb, DisplayHellbanned: false})
	if err := db.db.Model(&ChatMessage{}).Where("user_id = ?", u.ID).Update("is_hellbanned", hb).Error; err != nil {
		logrus.Error(err)
	}
	MsgPubSub.Pub(RefreshTopic, ChatMessageType{Typ: ForceRefresh})
}

// GetUserBySessionKey ...
func (d *DkfDB) GetUserBySessionKey(user *User, sessionKey string) error {
	return d.db.
		Joins("INNER JOIN sessions s ON s.user_id = users.id").
		Where("s.token = ? AND users.verified = 1 AND s.deleted_at IS NULL AND s.expires_at > DATETIME('now', 'localtime')", sessionKey).
		First(user).Error
}

// GetUserByApiKey ...
func (d *DkfDB) GetUserByApiKey(user *User, apiKey string) error {
	return d.db.First(user, "api_key = ?", apiKey).Error
}

// GetUserByID ...
func (d *DkfDB) GetUserByID(userID UserID) (out User, err error) {
	err = d.db.First(&out, "id = ?", userID).Error
	return
}

func (d *DkfDB) GetUserRenderMessageByID(userID UserID) (out IUserRenderMessage, err error) {
	var out1 User
	err = d.db.Raw(`
SELECT
id,
username,
refresh_rate,
chat_color,
is_incognito,
is_hellbanned,
afk,
afk_indicator_enabled,
display_ignored,
display_moderators,
notify_new_message,
notify_tagged,
notify_pmmed,
chat_read_marker_enabled,
display_delete_button,
is_admin,
display_hellban_button,
display_kick_button,
display_hellbanned,
display_alive_indicator,
syntax_highlight_code,
date_format,
confirm_external_links,
can_see_hellbanned,
role
FROM users WHERE id = ? LIMIT 1
`, userID).Scan(&out1).Error
	return &out1, err
}

func (d *DkfDB) GetUserByPokerReferralToken(token string) (out User, err error) {
	err = d.db.First(&out, "poker_referral_token = ?", token).Error
	return
}

func (d *DkfDB) GetUserByPokerXmrSubAddress(pokerXmrSubAddress string) (out User, err error) {
	err = d.db.First(&out, "poker_xmr_sub_address = ?", pokerXmrSubAddress).Error
	return
}

// GetUserByUsername ...
func (d *DkfDB) GetUserByUsername(username Username) (out User, err error) {
	err = d.db.First(&out, "username = ? COLLATE NOCASE", username).Error
	return
}

func (d *DkfDB) GetUserIDByUsername(username Username) (out UserID, err error) {
	var tmp struct{ ID UserID }
	err = d.db.Table("users").Select("id").First(&tmp, "username = ? COLLATE NOCASE", username).Error
	return tmp.ID, err
}

func (d *DkfDB) GetVerifiedUserByUsername(username Username) (out User, err error) {
	err = d.db.First(&out, "username = ? COLLATE NOCASE AND verified = 1", username).Error
	return
}

func (d *DkfDB) GetUsersByID(ids []UserID) (out []User, err error) {
	err = d.db.Find(&out, "id IN (?)", ids).Error
	return
}

func (d *DkfDB) GetUsersByUsername(usernames []string) (out []User, err error) {
	err = d.db.Find(&out, "username IN (?)", usernames).Error
	return
}

func (d *DkfDB) DeleteUserByID(userID UserID) (err error) {
	err = d.db.Unscoped().Delete(User{}, "id = ?", userID).Error
	return
}

func (d *DkfDB) GetModeratorsUsers() (out []User, err error) {
	err = d.db.Order("username ASC").Find(&out, "role = ? OR is_admin = 1", "moderator").Error
	return
}

func (d *DkfDB) GetClubMembers() (out []User, err error) {
	err = d.db.Find(&out, "is_club_member = ?", true).Error
	return
}

// ChangePassword change user's password. Save the user, and delete all active sessions.
// NOTE: When changing the password, it is important to delete any active sessions.
// Assume I realize I left myself logged into a shared computer.
// I change my password to protect myself.
// The session on the public computer needs to be invalidated.
func (u *User) ChangePassword(db *DkfDB, hashedPassword string) error {
	u.Password = hashedPassword
	if err := u.Save(db); err != nil {
		return err
	}
	// Delete active user sessions
	if err := db.DeleteUserSessions(u.ID); err != nil {
		return err
	}
	return nil
}

func (u *User) ChangeDuressPassword(db *DkfDB, hashedDuressPassword string) error {
	u.DuressPassword = hashedDuressPassword
	if err := u.Save(db); err != nil {
		return err
	}
	// Delete active user sessions
	if err := db.DeleteUserSessions(u.ID); err != nil {
		return err
	}
	return nil
}

func (u *User) CheckPassword(db *DkfDB, password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		if err := bcrypt.CompareHashAndPassword([]byte(u.DuressPassword), []byte(password)); err != nil {
			return false
		}
		u.SetIsUnderDuress(db, true)
	} else {
		u.SetIsUnderDuress(db, false)
	}
	return true
}

// UserErrors ...
type UserErrors struct {
	Username     string
	Password     string
	GPGPublicKey string
}

// HasError ...
func (e UserErrors) HasError() bool {
	return e.Username != "" || e.Password != "" || e.GPGPublicKey != ""
}

var ErrForbiddenUsername = errors.New("forbidden username")

// ValidateUsername ...
func ValidateUsername(username string, isFirstUser bool) (bool, error) {
	if !govalidator.IsPrintableASCII(username) {
		return false, errors.New("username must be ascii printable only")
	}
	lowerUsername := strings.ToLower(username)
	if !isFirstUser {
		if govalidator.Matches(lowerUsername, "n[o|0]tr[1|i|l][v|y]") ||
			strings.Contains(lowerUsername, "admin") {
			return false, ErrForbiddenUsername
		}
	}
	if strings.Contains(lowerUsername, "pedo") ||
		strings.Contains(lowerUsername, "fuck") ||
		strings.Contains(lowerUsername, "nigger") ||
		strings.Contains(lowerUsername, "nigga") {
		return false, ErrForbiddenUsername
	}
	if !govalidator.Matches(username, "^[a-zA-Z0-9_]+$") {
		return false, errors.New("username must match [a-zA-Z0-9_]+")
	}
	if !govalidator.StringLength(username, "3", "20") {
		return false, errors.New("username must have between 3 and 20 characters")
	}
	return true, nil
}

func isUsernameReserved(username Username) bool {
	return false
}

// GetVerifiedUserBySessionID ...
func (d *DkfDB) GetVerifiedUserBySessionID(token string) (out User, err error) {
	err = d.db.First(&out, "token = ? and verified = 1", token).Error
	return
}

// GetRecentUsersCount ...
func (d *DkfDB) GetRecentUsersCount() int64 {
	var count int64
	d.db.Table("users").Where("created_at > datetime('now', '-1 Minute', 'localtime')").Count(&count)
	return count
}

// IsUsernameAlreadyTaken ...
func (d *DkfDB) IsUsernameAlreadyTaken(username Username) bool {
	var count int64
	d.db.Table("users").Where("username = ? COLLATE NOCASE", username).Count(&count)
	return count > 0 || isUsernameReserved(username)
}

// PasswordValidator ...
type PasswordValidator struct {
	password string
	error    error
}

// NewPasswordValidator ...
func NewPasswordValidator(db *DkfDB, password string) *PasswordValidator {
	p := new(PasswordValidator)
	p.password = password
	if len(password) < 8 {
		p.error = errors.New("password must be at least 8 characters")
	}
	if len(password) > 128 {
		p.error = errors.New("password must be at most 128 characters")
	}
	if db.IsPasswordProhibited(password) {
		p.error = errors.New("this password is too weak")
	}
	return p
}

// CompareWith ...
func (p *PasswordValidator) CompareWith(repassword string) *PasswordValidator {
	if p.password != repassword {
		p.error = errors.New("passwords are not equal")
	}
	return p
}

// Hash ...
func (p *PasswordValidator) Hash() (string, error) {
	h := []byte("")
	var err error
	if p.error == nil {
		h, err = bcrypt.GenerateFromPassword([]byte(p.password), 12)
		if err != nil {
			p.error = errors.New("unable to hash password: " + err.Error())
		}
	}
	return string(h), p.error
}

func (d *DkfDB) CanUseUsername(username Username, isFirstUser bool) error {
	if _, err := ValidateUsername(string(username), isFirstUser); err != nil {
		return err
	} else if d.IsUsernameAlreadyTaken(username) {
		return errors.New("username already taken")
	}
	return nil
}

func (d *DkfDB) CanRenameTo(oldUsername, newUsername Username) error {
	if _, err := ValidateUsername(string(newUsername), false); err != nil {
		return err
	}
	if strings.ToLower(string(oldUsername)) != strings.ToLower(string(newUsername)) {
		if d.IsUsernameAlreadyTaken(newUsername) {
			return errors.New("username already taken")
		}
	}
	return nil
}

// CreateUser ...
func (d *DkfDB) CreateUser(username, password, repassword string, registrationDuration int64, signupInfoEnc string) (User, UserErrors) {
	return d.createUser(username, password, repassword, "", false, true, false, false, false, registrationDuration, signupInfoEnc)
}

func (d *DkfDB) CreateGuestUser(username, password string) (User, UserErrors) {
	return d.createUser(username, password, password, "", false, true, true, false, false, 0, "signupInfoEnc")
}

func (d *DkfDB) CreateFirstUser(username, password, repassword string) (User, UserErrors) {
	return d.createUser(username, password, repassword, "", true, true, false, true, false, 12000, "")
}

func (d *DkfDB) CreateZeroUser() (User, UserErrors) {
	password := utils.GenerateToken10()
	return d.createUser(config.NullUsername, password, password, config.NullUserPublicKey, false, true, false, false, true, 12000, "")
}

// skipUsernameValidation: entirely skip username validation (for "0" user)
// isFirstUser: less strict username validation; can use "admin"/"n0tr1v" usernames
func (d *DkfDB) createUser(usernameStr, password, repassword, gpgPublicKey string, isAdmin, verified, isGuestAcc, isFirstUser, skipUsernameValidation bool, registrationDuration int64, signupInfoEnc string) (User, UserErrors) {
	username := Username(strings.TrimSpace(usernameStr))
	var errs UserErrors
	if !skipUsernameValidation {
		if err := d.CanUseUsername(username, isFirstUser); err != nil {
			errs.Username = err.Error()
		}
	}
	hashedPassword, err := NewPasswordValidator(d, password).CompareWith(repassword).Hash()
	if err != nil {
		errs.Password = err.Error()
	}
	var newUser User
	if !errs.HasError() {
		newUser.Temp = isGuestAcc
		newUser.Role = "member"
		newUser.Username = username
		newUser.Password = hashedPassword
		newUser.GPGPublicKey = gpgPublicKey
		newUser.IsAdmin = isAdmin
		newUser.Verified = verified
		newUser.ChatColor = utils.GetRandomChatColor()
		newUser.RefreshRate = 5
		newUser.ChatReadMarkerEnabled = true
		newUser.ChatReadMarkerColor = "#4e7597"
		newUser.ChatBackgroundColor = "#111111"
		newUser.ChatReadMarkerSize = 1
		newUser.DisplayIgnored = false
		newUser.DisplayPms = 0
		newUser.CanUseForum = true
		newUser.CanUseMultiline = false
		newUser.CanUseChessAnalyze = false
		newUser.CanChangeUsername = true
		newUser.CanUseUppercase = true
		newUser.CanUploadFile = true
		newUser.CanChangeColor = true
		newUser.DisplayDeleteButton = true
		newUser.DisplayHellbanButton = true
		newUser.DisplayModerators = true
		newUser.DisplayHellbanned = false
		newUser.LastSeenPublic = true
		newUser.CollectMetadata = false
		newUser.RegistrationDuration = registrationDuration
		newUser.UseStream = true
		newUser.UseStreamMenu = true
		newUser.DisplayAliveIndicator = true
		newUser.CodeBlockHeight = 300
		newUser.HellbanOpacity = 30
		newUser.SignupMetadata = signupInfoEnc
		_, month, _ := time.Now().UTC().Date()
		if month == time.December {
			newUser.Theme = ThemeChristmas
		}
		if !verified {
			token := utils.GenerateToken32()
			newUser.Token = &token
		}
		if err := d.db.Create(&newUser).Error; err != nil {
			logrus.Error(err)
		}

		return newUser, errs
	}
	return newUser, errs
}

func (u *User) SetAvatar(b []byte) {
	u.Avatar = b
}

func (u *User) IncrKarma(db *DkfDB, karma int64, description string) {
	if _, err := db.CreateKarmaHistory(karma, description, u.ID, nil); err != nil {
		logrus.Error(err)
		return
	}
	u.Karma += karma
}

// CanSendPM you get your first karma point after sending 20 public messages
func (u *User) CanSendPM() bool {
	if u.IsModerator() || u.Vetted {
		return true
	}
	return u.GeneralMessagesCount >= 20
}

func (u *User) GetURL() string {
	return fmt.Sprintf("monero:%s", u.PokerXmrSubAddress)
}

func (u *User) GetImage() (image.Image, error) {
	b, err := qr.Encode(u.GetURL(), qr.L, qr.Auto)
	if err != nil {
		return nil, err
	}
	b, err = barcode.Scale(b, 150, 150)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (d *DkfDB) GetUserBalances(userID UserID) (xmrBalance Piconero, chipsTest PokerChip, err error) {
	var tmp struct {
		XmrBalance Piconero
		ChipsTest  PokerChip
	}
	err = d.db.Table("users").Select("xmr_balance, chips_test").First(&tmp, "id = ?", userID).Error
	return tmp.XmrBalance, tmp.ChipsTest, err
}

func (d *DkfDB) DecrUserBalance(userID UserID, isTest bool, amount PokerChip) (err error) {
	if isTest {
		err = d.db.Exec(`UPDATE users SET chips_test = chips_test - ? WHERE id = ?`, amount, userID).Error
	} else {
		err = d.db.Exec(`UPDATE users SET xmr_balance = xmr_balance - ? WHERE id = ?`, amount.ToPiconero(), userID).Error
	}
	return
}

func (d *DkfDB) IncrUserBalance(userID UserID, isTest bool, amount PokerChip) (err error) {
	if isTest {
		err = d.db.Exec(`UPDATE users SET chips_test = chips_test + ? WHERE id = ?`, amount, userID).Error
	} else {
		err = d.db.Exec(`UPDATE users SET xmr_balance = xmr_balance + ? WHERE id = ?`, amount.ToPiconero(), userID).Error
	}
	return
}

func (u *User) GetXmrBalance(db *DkfDB) (amount Piconero, err error) {
	var tmp struct{ XmrBalance Piconero }
	err = db.db.Table("users").Select("xmr_balance").First(&tmp, "id = ?", u.ID).Error
	return tmp.XmrBalance, err
}

func (u *User) IncrXmrBalance(db *DkfDB, amount Piconero) (err error) {
	err = db.db.Exec(`UPDATE users SET xmr_balance = xmr_balance + ? WHERE id = ?`, amount, u.ID).Error
	return
}

func (u *User) SubXmrBalance(db *DkfDB, amount Piconero) (err error) {
	err = db.db.Exec(`UPDATE users SET xmr_balance = xmr_balance - ? WHERE id = ?`, amount, u.ID).Error
	return
}

func (db *DkfDB) GetUsersXmrBalance() (out Piconero, err error) {
	var tmp struct{ SumXmrBalance Piconero }
	err = db.db.Raw(`SELECT SUM(xmr_balance) as sum_xmr_balance FROM users`).Scan(&tmp).Error
	return tmp.SumXmrBalance, err
}

func (d *DkfDB) SetPokerSubAddress(userID UserID, subAddress string) (err error) {
	err = d.db.Exec(`UPDATE users SET poker_xmr_sub_address = ? WHERE id = ?`, subAddress, userID).Error
	return
}

func (u *User) GetUserChips(isTest bool) PokerChip {
	return utils.Ternary(isTest, u.ChipsTest, u.XmrBalance.ToPokerChip())
}

func (d *DkfDB) GetUsersRakeBack() (out PokerChip, err error) {
	var tmp struct{ PokerRakeBack PokerChip }
	err = d.db.Raw(`SELECT SUM(poker_rake_back) as poker_rake_back FROM users`).Scan(&tmp).Error
	return tmp.PokerRakeBack, err
}

func (d *DkfDB) ClaimRakeBack(userID UserID) (err error) {
	err = d.db.Exec(`UPDATE users SET xmr_balance = xmr_balance + (poker_rake_back * 10000000), poker_rake_back = 0 WHERE id = ?`, int64(userID)).Error
	return
}

func (d *DkfDB) IncrUserRakeBack(referredBy UserID, rakeBack PokerChip) (err error) {
	err = d.db.Exec(`UPDATE users SET poker_rake_back = poker_rake_back + ? WHERE id = ?`, uint64(rakeBack), int64(referredBy)).Error
	return
}

func (d *DkfDB) GetRakeBackReferredCount(userID UserID) (count int64, err error) {
	var tmp struct{ Count int64 }
	err = d.db.Raw(`SELECT COUNT(id) AS count FROM users WHERE poker_referred_by = ?`, int64(userID)).Scan(&tmp).Error
	return tmp.Count, err
}
