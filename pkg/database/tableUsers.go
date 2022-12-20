package database

import (
	"dkforest/pkg/config"
	"errors"
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

// User struct an internal representation of a user for our app
type User struct {
	ID                           UserID
	Avatar                       []byte
	Username                     string
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
	DisplayIgnored               bool
	HideIgnoredUsersFromList     bool
	Verified                     bool
	Temp                         bool // Temporary account
	Token                        *string
	Role                         string
	ApiKey                       string
	Lang                         string
	ChatColor                    string
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
	CanChangeColor               bool
	CanUseMultiline              bool
	Vetted                       bool
	RegistrationDuration         int64
	LastSeenPublic               bool
	TerminateAllSessionsOnLogout bool
	DateFormat                   int64
	BlockNewUsersPm              bool
	HideRightColumn              bool
	ChatBarAtBottom              bool
	AutocompleteCommandsEnabled  bool
	AfkIndicatorEnabled          bool
	SignupMetadata               string
	CollectMetadata              bool
	CaptchaRequired              bool
	Theme                        int64
	GeneralMessagesCount         int64
	AFK                          bool
	HighlightOwnMessages         bool `gorm:"-"`
}

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

func GetChessSubscribers() (out []User, err error) {
	err = DB.Find(&out, "notify_chess_games == 1").Error
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
	if u.DateFormat == 1 {
		return "15:04:05"
	} else if u.DateFormat == 2 {
		return "01-02 03:04:05"
	} else if u.DateFormat == 3 {
		return "03:04:05"
	} else if u.DateFormat == 4 {
		return ""
	}
	return "01-02 15:04:05"
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

func (u *User) IsModerator() bool {
	return u.IsAdmin || u.Role == "moderator"
}

func (u *User) CanSeeHB() bool {
	return u.CanSeeHellbanned || u.IsModerator()
}

// Save user in the database
func (u *User) Save() error {
	return DB.Save(u).Error
}

// DoSave user in the database, ignore error
func (u *User) DoSave() {
	if err := DB.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

func (u *User) HellBan() {
	u.IsHellbanned = true
	u.DoSave()
	if err := DB.Model(&ChatMessage{}).Where("user_id = ?", u.ID).Update("is_hellbanned", true).Error; err != nil {
		logrus.Error(err)
	}
}

func (u *User) UnHellBan() {
	u.IsHellbanned = false
	u.DoSave()
	if err := DB.Model(&ChatMessage{}).Where("user_id = ?", u.ID).Update("is_hellbanned", false).Error; err != nil {
		logrus.Error(err)
	}
}

// GetUserBySessionKey ...
func GetUserBySessionKey(user *User, sessionKey string) error {
	return DB.Joins("INNER JOIN sessions s ON s.token = ? AND s.expires_at > DATETIME('now') and s.deleted_at IS NULL AND s.user_id = users.id").
		Where("users.verified = 1", sessionKey).
		First(user).Error
}

// GetUserByApiKey ...
func GetUserByApiKey(user *User, apiKey string) error {
	return DB.First(user, "api_key = ?", apiKey).Error
}

// GetUserByID ...
func GetUserByID(userID UserID) (out User, err error) {
	err = DB.First(&out, "id = ?", userID).Error
	return
}

// GetUserByUsername ...
func GetUserByUsername(username string) (out User, err error) {
	err = DB.First(&out, "username = ? COLLATE NOCASE", username).Error
	return
}

func GetVerifiedUserByUsername(username string) (out User, err error) {
	err = DB.First(&out, "username = ? COLLATE NOCASE AND verified = 1", username).Error
	return
}

func GetUsersByUsername(usernames []string) (out []User, err error) {
	err = DB.Find(&out, "username IN (?)", usernames).Error
	return
}

func GetModeratorsUsers() (out []User, err error) {
	err = DB.Find(&out, "role = ? OR is_admin = 1", "moderator").Error
	return
}

func GetClubMembers() (out []User, err error) {
	err = DB.Find(&out, "is_club_member = ?", true).Error
	return
}

// ChangePassword change user's password. Save the user, and delete all active sessions.
// NOTE: When changing the password, it is important to delete any active sessions.
// Assume I realize I left myself logged into a shared computer.
// I change my password to protect myself.
// The session on the public computer needs to be invalidated.
func (u *User) ChangePassword(hashedPassword string) error {
	u.Password = hashedPassword
	if err := DB.Save(u).Error; err != nil {
		return err
	}
	// Delete active user sessions
	if err := DeleteUserSessions(u.ID); err != nil {
		return err
	}
	return nil
}

func (u *User) ChangeDuressPassword(hashedDuressPassword string) error {
	u.DuressPassword = hashedDuressPassword
	if err := DB.Save(u).Error; err != nil {
		return err
	}
	// Delete active user sessions
	if err := DeleteUserSessions(u.ID); err != nil {
		return err
	}
	return nil
}

func (u *User) CheckPassword(password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		if err := bcrypt.CompareHashAndPassword([]byte(u.DuressPassword), []byte(password)); err != nil {
			return false
		}
		u.IsUnderDuress = true
		u.DoSave()
	} else {
		u.IsUnderDuress = false
		u.DoSave()
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

// ValidateUsername ...
func ValidateUsername(username string, isFirstUser bool) (bool, error) {
	if !govalidator.IsPrintableASCII(username) {
		return false, errors.New("username must be ascii printable only")
	}
	lowerUsername := strings.ToLower(username)
	if !isFirstUser {
		if govalidator.Matches(lowerUsername, "n[o|0]tr[1|i|l][v|y]") ||
			strings.Contains(lowerUsername, "admin") {
			return false, errors.New("forbidden username")
		}
	}
	if strings.Contains(lowerUsername, "pedo") ||
		strings.Contains(lowerUsername, "fuck") ||
		strings.Contains(lowerUsername, "nigger") ||
		strings.Contains(lowerUsername, "nigga") {
		return false, errors.New("forbidden username")
	}
	if !govalidator.Matches(username, "^[a-zA-Z0-9_]+$") {
		return false, errors.New("username must match [a-zA-Z0-9_]+")
	}
	if !govalidator.StringLength(username, "3", "20") {
		return false, errors.New("username must have between 3 and 20 characters")
	}
	return true, nil
}

func isUsernameReserved(username string) bool {
	return false
}

// GetVerifiedUserBySessionID ...
func GetVerifiedUserBySessionID(token string) (out User, err error) {
	err = DB.First(&out, "token = ? and verified = 1", token).Error
	return
}

// GetRecentUsersCount ...
func GetRecentUsersCount() int64 {
	var count int64
	DB.Table("users").Where("created_at > datetime('now', '-1 Minute')").Count(&count)
	return count
}

// IsUsernameAlreadyTaken ...
func IsUsernameAlreadyTaken(username string) bool {
	var count int64
	DB.Table("users").Where("username = ? COLLATE NOCASE", username).Count(&count)
	return count > 0 || isUsernameReserved(username)
}

// PasswordValidator ...
type PasswordValidator struct {
	password string
	error    error
}

// NewPasswordValidator ...
func NewPasswordValidator(password string) *PasswordValidator {
	p := new(PasswordValidator)
	p.password = password
	if len(password) < 8 {
		p.error = errors.New("password must be at least 8 characters")
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

func CanUseUsername(username string, isFirstUser bool) error {
	if _, err := ValidateUsername(username, isFirstUser); err != nil {
		return err
	} else if IsUsernameAlreadyTaken(username) {
		return errors.New("username already taken")
	}
	return nil
}

// CreateUser ...
func CreateUser(username, password, repassword string, registrationDuration int64, signupInfoEnc string) (User, UserErrors) {
	return createUser(username, password, repassword, "", false, true, false, false, false, registrationDuration, signupInfoEnc)
}

func CreateGuestUser(username, password string) (User, UserErrors) {
	return createUser(username, password, password, "", false, true, true, false, false, 0, "signupInfoEnc")
}

func CreateFirstUser(username, password, repassword string) (User, UserErrors) {
	return createUser(username, password, repassword, "", true, true, false, true, false, 12000, "")
}

func CreateZeroUser() (User, UserErrors) {
	password := utils.GenerateToken10()
	return createUser("0", password, password, config.NullUserPublicKey, false, true, false, false, true, 12000, "")
}

// skipUsernameValidation: entirely skip username validation (for "0" user)
// isFirstUser: less strict username validation; can use "admin"/"n0tr1v" usernames
func createUser(username, password, repassword, gpgPublicKey string, isAdmin, verified, temp, isFirstUser, skipUsernameValidation bool, registrationDuration int64, signupInfoEnc string) (User, UserErrors) {
	username = strings.TrimSpace(username)
	var errs UserErrors
	if !skipUsernameValidation {
		if err := CanUseUsername(username, isFirstUser); err != nil {
			errs.Username = err.Error()
		}
	}
	hashedPassword, err := NewPasswordValidator(password).CompareWith(repassword).Hash()
	if err != nil {
		errs.Password = err.Error()
	}
	var newUser User
	if !errs.HasError() {
		newUser.Temp = temp
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
		newUser.ChatReadMarkerSize = 1
		newUser.DisplayIgnored = false
		newUser.DisplayPms = 0
		newUser.CanUseForum = true
		newUser.CanUseMultiline = false
		newUser.CanChangeUsername = true
		newUser.CanUploadFile = true
		newUser.CanChangeColor = true
		newUser.DisplayDeleteButton = true
		newUser.DisplayHellbanButton = true
		newUser.DisplayModerators = true
		newUser.DisplayHellbanned = false
		newUser.LastSeenPublic = true
		newUser.CollectMetadata = false
		newUser.RegistrationDuration = registrationDuration
		newUser.SignupMetadata = signupInfoEnc
		if !verified {
			token := utils.GenerateToken32()
			newUser.Token = &token
		}
		if err := DB.Create(&newUser).Error; err != nil {
			logrus.Error(err)
		}

		return newUser, errs
	}
	return newUser, errs
}

func (u *User) SetAvatar(b []byte) {
	u.Avatar = b
}

func (u *User) IncrKarma(karma int64, description string) {
	if _, err := CreateKarmaHistory(karma, description, u.ID, nil); err != nil {
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
