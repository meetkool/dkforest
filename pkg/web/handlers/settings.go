package handlers

import (
	"bytes"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/global"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"errors"
	"filippo.io/age"
	armor1 "filippo.io/age/armor"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"image"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SettingsChatHandler ...
func SettingsChatHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsChatData
	data.ActiveTab = "chat"
	data.AllFonts = utils.GetFonts()
	data.ChatColor = authUser.ChatColor
	data.ChatBackgroundColor = authUser.ChatBackgroundColor
	data.ChatFont = authUser.ChatFont
	data.ChatItalic = authUser.ChatItalic
	data.ChatBold = authUser.ChatBold
	data.DateFormat = authUser.DateFormat
	data.ChatReadMarkerEnabled = authUser.ChatReadMarkerEnabled
	data.ChatReadMarkerColor = authUser.ChatReadMarkerColor
	data.ChatReadMarkerSize = authUser.ChatReadMarkerSize
	data.DisplayHellbanned = authUser.DisplayHellbanned
	data.DisplayModerators = authUser.DisplayModerators
	data.DisplayDeleteButton = authUser.DisplayDeleteButton
	data.DisplayKickButton = authUser.DisplayKickButton
	data.DisplayHellbanButton = authUser.DisplayHellbanButton
	data.HideRightColumn = authUser.HideRightColumn
	data.ChatBarAtBottom = authUser.ChatBarAtBottom
	data.AutocompleteCommandsEnabled = authUser.AutocompleteCommandsEnabled
	data.SpellcheckEnabled = authUser.SpellcheckEnabled
	data.AfkIndicatorEnabled = authUser.AfkIndicatorEnabled
	data.HideIgnoredUsersFromList = authUser.HideIgnoredUsersFromList
	data.HellbanOpacity = float64(authUser.HellbanOpacity) / 100
	data.CodeBlockHeight = authUser.CodeBlockHeight
	data.RefreshRate = authUser.RefreshRate
	data.NotifyNewMessage = authUser.NotifyNewMessage
	data.NotifyTagged = authUser.NotifyTagged
	data.NotifyPmmed = authUser.NotifyPmmed
	data.NotifyNewMessageSound = authUser.NotifyNewMessageSound
	data.NotifyTaggedSound = authUser.NotifyTaggedSound
	data.NotifyPmmedSound = authUser.NotifyPmmedSound
	data.Theme = authUser.Theme
	data.NotifyChessGames = authUser.NotifyChessGames
	data.NotifyChessMove = authUser.NotifyChessMove
	data.UseStream = authUser.UseStream
	data.UseStreamMenu = authUser.UseStreamMenu
	data.DisplayAliveIndicator = authUser.DisplayAliveIndicator
	data.ConfirmExternalLinks = authUser.ConfirmExternalLinks
	data.ChessSoundsEnabled = authUser.ChessSoundsEnabled
	data.PokerSoundsEnabled = authUser.PokerSoundsEnabled
	data.ManualMultiline = authUser.ManualMultiline

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat", data)
	}

	// POST
	formName := c.FormValue("formName")
	if formName == "changeSettings" {
		return changeSettingsForm(c, data)
	}
	return c.Render(http.StatusOK, "settings.chat", data)
}

func changeSettingsForm(c echo.Context, data settingsChatData) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	data.RefreshRate = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("refresh_rate")), 5, 60)
	data.ChatColor = c.Request().PostFormValue("chat_color")
	data.ChatBackgroundColor = c.Request().PostFormValue("chat_background_color")
	data.ChatFont = utils.DoParseInt64(c.Request().PostFormValue("chat_font"))
	data.ChatBold = utils.DoParseBool(c.Request().PostFormValue("chat_bold"))
	data.DateFormat = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("date_format")), 0, 4)
	data.ChatReadMarkerEnabled = utils.DoParseBool(c.Request().PostFormValue("chat_read_marker_enabled"))
	data.ChatReadMarkerColor = c.Request().PostFormValue("chat_read_marker_color")
	data.ChatReadMarkerSize = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("chat_read_marker_size")), 1, 5)
	data.DisplayHellbanned = utils.DoParseBool(c.Request().PostFormValue("display_hellbanned"))
	data.DisplayModerators = utils.DoParseBool(c.Request().PostFormValue("display_moderators"))
	data.DisplayKickButton = utils.DoParseBool(c.Request().PostFormValue("display_kick_button"))
	data.DisplayDeleteButton = utils.DoParseBool(c.Request().PostFormValue("display_delete_button"))
	data.DisplayHellbanButton = utils.DoParseBool(c.Request().PostFormValue("display_hellban_button"))
	data.HideIgnoredUsersFromList = utils.DoParseBool(c.Request().PostFormValue("hide_ignored_users_from_list"))
	data.HideRightColumn = utils.DoParseBool(c.Request().PostFormValue("hide_right_column"))
	data.ChatBarAtBottom = utils.DoParseBool(c.Request().PostFormValue("chat_bar_at_bottom"))
	data.AutocompleteCommandsEnabled = utils.DoParseBool(c.Request().PostFormValue("autocomplete_commands_enabled"))
	data.SpellcheckEnabled = utils.DoParseBool(c.Request().PostFormValue("spellcheck_enabled"))
	data.AfkIndicatorEnabled = utils.DoParseBool(c.Request().PostFormValue("afk_indicator_enabled"))
	data.ChatItalic = utils.DoParseBool(c.Request().PostFormValue("chat_italic"))
	data.NotifyNewMessage = utils.DoParseBool(c.Request().PostFormValue("notify_new_message"))
	data.NotifyTagged = utils.DoParseBool(c.Request().PostFormValue("notify_tagged"))
	data.NotifyPmmed = utils.DoParseBool(c.Request().PostFormValue("notify_pmmed"))
	data.Theme = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("theme")), 0, 2)
	data.NotifyChessGames = utils.DoParseBool(c.Request().PostFormValue("notify_chess_games"))
	data.NotifyChessMove = utils.DoParseBool(c.Request().PostFormValue("notify_chess_move"))
	data.UseStream = utils.DoParseBool(c.Request().PostFormValue("use_stream"))
	data.UseStreamMenu = utils.DoParseBool(c.Request().PostFormValue("use_stream_menu"))
	data.DisplayAliveIndicator = utils.DoParseBool(c.Request().PostFormValue("display_alive_indicator"))
	data.ConfirmExternalLinks = utils.DoParseBool(c.Request().PostFormValue("confirm_external_links"))
	data.ChessSoundsEnabled = utils.DoParseBool(c.Request().PostFormValue("chess_sounds_enabled"))
	data.PokerSoundsEnabled = utils.DoParseBool(c.Request().PostFormValue("poker_sounds_enabled"))
	data.HellbanOpacity = utils.DoParseF64(c.Request().PostFormValue("hellban_opacity"))
	data.CodeBlockHeight = utils.DoParseInt64(c.Request().PostFormValue("code_block_height"))
	data.ManualMultiline = utils.DoParseBool(c.Request().PostFormValue("manual_multiline"))
	//data.NotifyNewMessageSound = utils.DoParseInt64(c.Request().PostFormValue("notify_new_message_sound"))
	//data.NotifyTaggedSound = utils.DoParseInt64(c.Request().PostFormValue("notify_tagged_sound"))
	//data.NotifyPmmedSound = utils.DoParseInt64(c.Request().PostFormValue("notify_pmmed_sound"))
	colorRgx := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	if !colorRgx.MatchString(data.ChatColor) {
		data.Error = "Invalid color format (text)"
		return c.Render(http.StatusOK, "settings.chat", data)
	}
	if !colorRgx.MatchString(data.ChatBackgroundColor) {
		data.Error = "Invalid color format (background)"
		return c.Render(http.StatusOK, "settings.chat", data)
	}
	if !colorRgx.MatchString(data.ChatReadMarkerColor) {
		data.Error = "Invalid marker color format"
		return c.Render(http.StatusOK, "settings.chat", data)
	}
	authUser.RefreshRate = data.RefreshRate
	if authUser.CanChangeColor {
		authUser.ChatColor = data.ChatColor
	}
	authUser.ChatBackgroundColor = data.ChatBackgroundColor
	authUser.ChatFont = data.ChatFont
	authUser.ChatItalic = data.ChatItalic
	authUser.ChatBold = data.ChatBold
	authUser.DateFormat = data.DateFormat
	authUser.ChatReadMarkerEnabled = data.ChatReadMarkerEnabled
	authUser.ChatReadMarkerColor = data.ChatReadMarkerColor
	authUser.ChatReadMarkerSize = data.ChatReadMarkerSize
	authUser.DisplayDeleteButton = data.DisplayDeleteButton
	authUser.HideIgnoredUsersFromList = data.HideIgnoredUsersFromList
	authUser.HideRightColumn = data.HideRightColumn
	authUser.ChatBarAtBottom = data.ChatBarAtBottom
	authUser.AutocompleteCommandsEnabled = data.AutocompleteCommandsEnabled
	authUser.SpellcheckEnabled = data.SpellcheckEnabled
	authUser.AfkIndicatorEnabled = data.AfkIndicatorEnabled
	authUser.NotifyNewMessage = data.NotifyNewMessage
	authUser.NotifyTagged = data.NotifyTagged
	authUser.NotifyPmmed = data.NotifyPmmed
	authUser.NotifyChessGames = data.NotifyChessGames
	authUser.NotifyChessMove = data.NotifyChessMove
	authUser.UseStream = data.UseStream
	authUser.UseStreamMenu = data.UseStreamMenu
	authUser.DisplayAliveIndicator = data.DisplayAliveIndicator
	authUser.ConfirmExternalLinks = data.ConfirmExternalLinks
	authUser.ChessSoundsEnabled = data.ChessSoundsEnabled
	authUser.PokerSoundsEnabled = data.PokerSoundsEnabled
	authUser.Theme = data.Theme
	//authUser.NotifyNewMessageSound = data.NotifyNewMessageSound
	//authUser.NotifyTaggedSound = data.NotifyTaggedSound
	//authUser.NotifyPmmedSound = data.NotifyPmmedSound

	authUser.CodeBlockHeight = utils.Clamp(data.CodeBlockHeight, 15, 300)
	if authUser.CanSeeHB() {
		authUser.HellbanOpacity = utils.Clamp(int64(data.HellbanOpacity*100), 0, 100)
	}
	if authUser.IsModerator() {
		authUser.DisplayHellbanned = data.DisplayHellbanned
		authUser.DisplayModerators = data.DisplayModerators
		authUser.DisplayKickButton = data.DisplayKickButton
		authUser.DisplayHellbanButton = data.DisplayHellbanButton
	}
	if authUser.CanUseMultiline {
		authUser.ManualMultiline = data.ManualMultiline
	}

	if err := authUser.Save(db); err != nil {
		logrus.Error(err)
		data.Error = err.Error()
		return c.Render(http.StatusOK, "settings.chat", data)
	}

	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Settings changed successfully", Redirect: "/settings/chat"})
}

func SettingsSecurityHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsSecurityData
	data.ActiveTab = "security"
	data.Logs, _ = db.GetSecurityLogs(authUser.ID)
	return c.Render(http.StatusOK, "settings.security", data)
}

// SettingsAccountHandler ...
func SettingsAccountHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsAccountData
	data.AccountTooYoungErrorString = hutils.AccountTooYoungErr.Error()
	data.ActiveTab = "account"
	data.Username = authUser.Username
	data.Email = authUser.Email
	data.LastSeenPublic = authUser.LastSeenPublic
	data.TerminateAllSessionsOnLogout = authUser.TerminateAllSessionsOnLogout
	data.Website = authUser.Website

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.account", data)
	}

	// POST
	formName := c.FormValue("formName")
	switch formName {
	case "changeUsername":
		return changeUsernameForm(c, data)
	case "editProfile":
		return editProfileForm(c, data)
	case "changeAvatar":
		return changeAvatarForm(c, data)
	default:
		return c.Render(http.StatusOK, "settings.account", data)
	}
}

func changeUsernameForm(c echo.Context, data settingsAccountData) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.CanChangeUsername {
		data.ErrorUsername = "Not allowed to change your username"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	username := database.Username(c.Request().PostFormValue("username"))
	data.Username = username

	if username == authUser.Username {
		data.ErrorUsername = "username did not change"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	if err := db.CanRenameTo(authUser.Username, username); err != nil {
		data.ErrorUsername = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	managers.ActiveUsers.RemoveUser(authUser.ID)
	authUser.Username = username
	if err := db.DB().Save(authUser).Error; err != nil {
		logrus.Error(err)
		data.ErrorUsername = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	db.CreateSecurityLog(authUser.ID, database.UsernameChangedSecurityLog)
	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Username changed successfully", Redirect: "/settings/account"})
}

func editProfileForm(c echo.Context, data settingsAccountData) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	email := c.Request().PostFormValue("email")
	website := c.Request().PostFormValue("website")
	lastSeenPublic := utils.DoParseBool(c.Request().PostFormValue("last_seen_public"))
	terminateAllSessionsOnLogout := utils.DoParseBool(c.Request().PostFormValue("terminate_all_sessions_on_logout"))
	data.Email = email
	data.Website = website
	data.LastSeenPublic = lastSeenPublic
	data.TerminateAllSessionsOnLogout = terminateAllSessionsOnLogout

	if data.Email != "" && !govalidator.IsEmail(data.Email) {
		data.ErrorEmail = "invalid email"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	if data.Website != "" && !govalidator.IsURL(data.Website) {
		data.ErrorWebsite = "invalid website"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	authUser.Website = data.Website
	authUser.Email = data.Email
	authUser.LastSeenPublic = data.LastSeenPublic
	authUser.TerminateAllSessionsOnLogout = data.TerminateAllSessionsOnLogout
	authUser.DoSave(db)

	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Profile changed successfully", Redirect: "/settings/account"})
}

func changeAvatarForm(c echo.Context, data settingsAccountData) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.CanUpload() {
		data.ErrorAvatar = hutils.AccountTooYoungErr.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	if err := c.Request().ParseMultipartForm(config.MaxAvatarFormSize); err != nil {
		data.ErrorAvatar = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	file, handler, err := c.Request().FormFile("avatar")
	if err != nil {
		data.ErrorAvatar = "Failed to get avatar: " + err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	defer file.Close()
	if handler.Size > config.MaxAvatarSize {
		data.ErrorAvatar = fmt.Sprintf("The maximum file size for avatars is %s", humanize.Bytes(config.MaxAvatarSize))
		return c.Render(http.StatusOK, "settings.account", data)
	}
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		data.ErrorAvatar = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	filetype := http.DetectContentType(fileBytes)
	if !utils.InArr(filetype, []string{"image/jpeg", "image/png", "image/gif", "image/bmp", "image/webp"}) {
		data.ErrorAvatar = "The provided file format is not allowed. Please upload a JPEG, PNG, WEBP, BMP or GIF image"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	// Validate image type and determine extension
	if handler.Header.Get("Content-Type") != filetype {
		data.ErrorAvatar = "Content-Type does not match mimetype"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	im, _, err := image.DecodeConfig(bytes.NewReader(fileBytes))
	if err != nil {
		data.ErrorAvatar = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}
	if im.Width > 120 || im.Height > 120 {
		data.ErrorAvatar = "The maximum dimensions for avatars are: 120x120 pixels"
		return c.Render(http.StatusOK, "settings.account", data)
	}

	if filetype == "image/jpeg" {
		fileBytes, err = utils.ReencodeJpg(fileBytes)
	} else if filetype == "image/png" {
		fileBytes, err = utils.ReencodePng(fileBytes)
	}
	if err != nil {
		data.Error = err.Error()
		return c.Render(http.StatusOK, "settings.account", data)
	}

	authUser.SetAvatar(fileBytes)
	authUser.DoSave(db)
	return c.Render(http.StatusOK, "flash", FlashResponse{Message: "Avatar changed successfully", Redirect: "/settings/account"})
}

func SettingsChatPMHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsChatPMData
	data.ActiveTab = "chat"
	data.PmMode = authUser.PmMode
	data.BlockNewUsersPm = authUser.BlockNewUsersPm
	data.WhitelistedUsers, _ = db.GetPmWhitelistedUsers(authUser.ID)
	data.BlacklistedUsers, _ = db.GetPmBlacklistedUsers(authUser.ID)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat-pm", data)
	}

	// POST
	formName := c.Request().PostFormValue("formName")

	if formName == "addWhitelist" {
		data.AddWhitelist = database.Username(strings.TrimSpace(c.Request().PostFormValue("username")))
		user, err := db.GetUserByUsername(data.AddWhitelist)
		if err != nil {
			data.Error = "username not found"
			return c.Render(http.StatusOK, "settings.chat-pm", data)
		}
		db.AddWhitelistedUser(authUser.ID, user.ID)
		return c.Redirect(http.StatusFound, "/settings/chat/pm")

	} else if formName == "rmWhitelist" {
		userID := dutils.DoParseUserID(c.Request().PostFormValue("userID"))
		db.RmWhitelistedUser(authUser.ID, userID)
		return c.Redirect(http.StatusFound, "/settings/chat/pm")

	} else if formName == "addBlacklist" {
		data.AddBlacklist = database.Username(strings.TrimSpace(c.Request().PostFormValue("username")))
		user, err := db.GetUserByUsername(data.AddBlacklist)
		if err != nil {
			data.Error = "username not found"
			return c.Render(http.StatusOK, "settings.chat-pm", data)
		}
		db.AddBlacklistedUser(authUser.ID, user.ID)
		return c.Redirect(http.StatusFound, "/settings/chat/pm")

	} else if formName == "rmBlacklist" {
		userID := dutils.DoParseUserID(c.Request().PostFormValue("userID"))
		db.RmBlacklistedUser(authUser.ID, userID)
		return c.Redirect(http.StatusFound, "/settings/chat/pm")
	}

	data.PmMode = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("pm_mode")), 0, 1)
	authUser.BlockNewUsersPm = utils.DoParseBool(c.Request().PostFormValue("block_new_users_pm"))
	authUser.PmMode = data.PmMode
	authUser.DoSave(db)
	return c.Redirect(http.StatusFound, "/settings/chat/pm")
}

func SettingsChatIgnoreHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsChatIgnoreData
	data.ActiveTab = "chat"
	data.PmMode = authUser.PmMode
	data.IgnoredUsers, _ = db.GetIgnoredUsers(authUser.ID)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat-ignore", data)
	}

	// POST
	formName := c.Request().PostFormValue("formName")

	if formName == "addIgnored" {
		data.AddIgnored = database.Username(strings.TrimSpace(c.Request().PostFormValue("username")))
		user, err := db.GetUserByUsername(data.AddIgnored)
		if err != nil {
			data.Error = "username not found"
			return c.Render(http.StatusOK, "settings.chat-ignore", data)
		}
		db.IgnoreUser(authUser.ID, user.ID)
		return c.Redirect(http.StatusFound, "/settings/chat/ignore")

	} else if formName == "rmIgnored" {
		userID := dutils.DoParseUserID(c.Request().PostFormValue("userID"))
		db.UnIgnoreUser(authUser.ID, userID)
		return c.Redirect(http.StatusFound, "/settings/chat/ignore")
	}

	data.PmMode = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("pm_mode")), 0, 1)
	authUser.PmMode = data.PmMode
	authUser.DoSave(db)
	return c.Redirect(http.StatusFound, "/settings/chat/ignore")
}

func SettingsChatSnippetsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsChatSnippetsData
	data.ActiveTab = "snippets"
	data.Snippets, _ = db.GetUserSnippets(authUser.ID)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.chat-snippets", data)
	}

	// POST
	formName := c.Request().PostFormValue("formName")

	if formName == "addSnippet" {
		data.Name = strings.TrimSpace(c.Request().PostFormValue("name"))
		data.Text = strings.TrimSpace(c.Request().PostFormValue("text"))
		if len(data.Snippets) >= 20 {
			data.Error = "snippets limit reached"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if !govalidator.Matches(data.Name, `^\w{1,20}$`) {
			data.Error = "name must match : ^\\w{1,20}$"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if !govalidator.StringLength(data.Name, "1", "20") {
			data.Error = "name must be 1-20 characters"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if !govalidator.StringLength(data.Text, "1", "1000") {
			data.Error = "text must be 1-1000 characters"
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		if _, err := db.CreateSnippet(authUser.ID, data.Name, data.Text); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "settings.chat-snippets", data)
		}
		return c.Redirect(http.StatusFound, "/settings/chat/snippets")

	} else if formName == "rmSnippet" {
		snippetName := c.Request().PostFormValue("snippetName")
		db.DeleteSnippet(authUser.ID, snippetName)
		return c.Redirect(http.StatusFound, "/settings/chat/snippets")
	}

	return c.Redirect(http.StatusFound, "/settings/chat/snippets")
}

func SettingsPublicNotesHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/settings/account", Type: "alert-danger"})
	}
	var data settingsPublicNotesData
	data.ActiveTab = "notes"
	data.Notes, _ = db.GetUserPublicNotes(authUser.ID)

	if c.Request().Method == http.MethodPost {
		notes := c.Request().PostFormValue("public_notes")
		if err := db.SetUserPublicNotes(authUser.ID, notes); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "settings.public-notes", data)
		}
		return c.Redirect(http.StatusFound, "/settings/public-notes")
	}

	return c.Render(http.StatusOK, "settings.public-notes", data)
}

func SettingsPrivateNotesHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !authUser.CanUseForumFn() {
		return c.Render(http.StatusOK, "flash", FlashResponse{Message: hutils.AccountTooYoungErr.Error(), Redirect: "/settings/account", Type: "alert-danger"})
	}
	var data settingsPrivateNotesData
	data.ActiveTab = "notes"
	if !authUser.IsUnderDuress {
		data.Notes, _ = db.GetUserPrivateNotes(authUser.ID)
	}

	if c.Request().Method == http.MethodPost {
		notes := c.Request().PostFormValue("private_notes")
		if err := db.SetUserPrivateNotes(authUser.ID, notes); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "settings.private-notes", data)
		}
		return c.Redirect(http.StatusFound, "/settings/private-notes")
	}

	return c.Render(http.StatusOK, "settings.private-notes", data)
}

func SettingsSessionsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsSessionsData
	data.ActiveTab = "sessions"
	sessions := db.GetActiveUserSessions(authUser.ID)
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	for _, session := range sessions {
		s := WrapperSession{Session: session}
		if authCookie.Value == s.Token {
			s.CurrentSession = true
		}
		data.Sessions = append(data.Sessions, s)
	}

	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "revoke_all_other_sessions" {
			_ = db.DeleteUserOtherSessions(authUser.ID, authCookie.Value)
		} else {
			sessionToken := c.Request().PostFormValue("sessionToken")
			_ = db.DeleteUserSessionByToken(authUser.ID, sessionToken)
		}
		return c.Redirect(http.StatusFound, "/settings/sessions")
	}

	return c.Render(http.StatusOK, "settings.sessions", data)
}

func SettingsAPIHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsAPIData
	data.ActiveTab = "api"
	data.APIKey = authUser.ApiKey
	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		btnSubmit := c.Request().PostFormValue("btn_submit")
		if btnSubmit == "Cancel" {
			return c.Redirect(http.StatusFound, "/settings/api")
		}
		if formName == "confirm" {
			token := utils.GenerateToken16()
			authUser.SetApiKey(db, token)
			return c.Redirect(http.StatusFound, "/settings/api")
		}
		data.NeedConfirm = true
	}
	return c.Render(http.StatusOK, "settings.api", data)
}

func SettingsPasswordHandler(c echo.Context) error {
	var data settingsPasswordData
	data.ActiveTab = "password"

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.password", data)
	}

	// POST
	formName := c.FormValue("formName")
	switch formName {
	case "changePassword":
		return changePasswordForm(c, data)
	case "changeDuressPassword":
		return changeDuressPasswordForm(c, data)
	default:
		return c.Render(http.StatusOK, "settings.password", data)
	}
}

func changePasswordForm(c echo.Context, data settingsPasswordData) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	oldPassword := c.Request().PostFormValue("oldPassword")
	newPassword := c.Request().PostFormValue("newPassword")
	rePassword := c.Request().PostFormValue("rePassword")
	data.OldPassword = oldPassword
	data.NewPassword = newPassword
	data.RePassword = rePassword

	if len(oldPassword) == 0 {
		data.ErrorOldPassword = "This field is required"
		return c.Render(http.StatusOK, "settings.password", data)
	}

	if len(newPassword) > 0 || len(rePassword) > 0 {
		hashedPassword, err := database.NewPasswordValidator(db, newPassword).CompareWith(rePassword).Hash()
		if err != nil {
			data.ErrorNewPassword = err.Error()
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if !authUser.CheckPassword(db, oldPassword) {
			data.ErrorOldPassword = "Invalid password"
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if err := authUser.ChangePassword(db, hashedPassword); err != nil {
			logrus.Error(err)
		}
		c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
		db.CreateSecurityLog(authUser.ID, database.ChangePasswordSecurityLog)
		return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Password changed successfully", Redirect: "/login"})
	}

	return c.Redirect(http.StatusFound, "/settings/password")
}

func changeDuressPasswordForm(c echo.Context, data settingsPasswordData) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	oldDuressPassword := c.Request().PostFormValue("oldDuressPassword")
	newDuressPassword := c.Request().PostFormValue("newDuressPassword")
	reDuressPassword := c.Request().PostFormValue("reDuressPassword")
	data.OldDuressPassword = oldDuressPassword
	data.NewDuressPassword = newDuressPassword
	data.ReDuressPassword = reDuressPassword

	if len(oldDuressPassword) == 0 {
		data.ErrorOldDuressPassword = "This field is required"
		return c.Render(http.StatusOK, "settings.password", data)
	}

	if len(newDuressPassword) > 0 || len(reDuressPassword) > 0 {
		hashedPassword, err := database.NewPasswordValidator(db, newDuressPassword).CompareWith(reDuressPassword).Hash()
		if err != nil {
			data.ErrorNewDuressPassword = err.Error()
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if !authUser.CheckPassword(db, oldDuressPassword) {
			data.ErrorOldDuressPassword = "Invalid password"
			return c.Render(http.StatusOK, "settings.password", data)
		}

		if err := authUser.ChangeDuressPassword(db, hashedPassword); err != nil {
			logrus.Error(err)
		}
		c.SetCookie(hutils.DeleteCookie(hutils.AuthCookieName))
		db.CreateSecurityLog(authUser.ID, database.ChangeDuressPasswordSecurityLog)
		return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Password changed successfully", Redirect: "/login"})
	}

	return c.Redirect(http.StatusFound, "/settings/password")
}

func SettingsUploadsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsUploadsData
	data.ActiveTab = "uploads"
	data.Files, _ = db.GetUserUploads(authUser.ID)
	for _, f := range data.Files {
		data.TotalSize += f.FileSize
	}

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.uploads", data)
	}

	// POST
	formName := c.FormValue("formName")
	if formName == "deleteUpload" {
		fileName := c.Request().PostFormValue("file_name")
		file, err := db.GetUploadByFileName(fileName)
		if err != nil {
			return c.Redirect(http.StatusFound, "/")
		}
		if authUser.ID != file.UserID {
			return c.Redirect(http.StatusFound, "/")
		}
		if err := file.Delete(db); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/settings/uploads")
	}
	return c.Render(http.StatusOK, "settings.uploads", data)
}

func SettingsInboxHandler(c echo.Context) error {
	authCookie, _ := c.Cookie(hutils.AuthCookieName)
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsInboxData
	data.ActiveTab = "inbox"
	// Do not fetch inboxes & notifications if logged in under duress
	if !authUser.IsUnderDuress {
		global.DeleteUserNotificationCount(authUser.ID, authCookie.Value)
		data.ChatMessages, _ = db.GetUserChatInboxMessages(authUser.ID)
		data.Notifications, _ = db.GetUserNotifications(authUser.ID)
		data.SessionNotifications, _ = db.GetUserSessionNotifications(authCookie.Value)
	}
	for _, m := range data.ChatMessages {
		data.Notifs = append(data.Notifs, InboxTmp{IsNotif: false, ChatInboxMessage: m})
	}
	for _, m := range data.Notifications {
		data.Notifs = append(data.Notifs, InboxTmp{IsNotif: true, Notification: m})
	}
	for _, m := range data.SessionNotifications {
		data.Notifs = append(data.Notifs, InboxTmp{IsNotif: true, SessionNotification: m})
	}
	sort.Slice(data.Notifs, func(i, j int) bool {
		a := data.Notifs[i]
		b := data.Notifs[j]
		var tsa time.Time
		var tsb time.Time
		if a.Notification.ID != 0 {
			tsa = a.Notification.CreatedAt
		} else if a.SessionNotification.ID != 0 {
			tsa = a.SessionNotification.CreatedAt
		} else {
			tsa = a.ChatInboxMessage.CreatedAt
		}
		if b.Notification.ID != 0 {
			tsb = b.Notification.CreatedAt
		} else if b.SessionNotification.ID != 0 {
			tsb = b.SessionNotification.CreatedAt
		} else {
			tsb = b.ChatInboxMessage.CreatedAt
		}
		return tsa.After(tsb)
	})
	return c.Render(http.StatusOK, "settings.inbox", data)
}

func SettingsInboxSentHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsInboxSentData
	data.ActiveTab = "inbox"
	// Do not fetch inboxes & notifications if logged in under duress
	if !authUser.IsUnderDuress {
		data.ChatInboxSent, _ = db.GetUserChatInboxMessagesSent(authUser.ID)
	}
	return c.Render(http.StatusOK, "settings.inbox-sent", data)
}

func AddPGPHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data addPGPData
	data.PGPPublicKey = authUser.GPGPublicKey
	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "pgp_step1" {

			data.PGPPublicKey = c.Request().PostFormValue("pgp_public_key")
			data.GpgMode = utils.DoParseBool(c.Request().PostFormValue("gpg_mode"))

			if data.GpgMode {
				data.ToBeSignedMessage = generatePgpToBeSignedTokenMessage(authUser.ID, data.PGPPublicKey)
				return c.Render(http.StatusOK, "pgp_code", data)

			} else {
				msg, err := generatePgpEncryptedTokenMessage(authUser.ID, data.PGPPublicKey)
				if err != nil {
					data.ErrorPGPPublicKey = err.Error()
					return c.Render(http.StatusOK, "pgp", data)
				}
				data.EncryptedMessage = msg
				return c.Render(http.StatusOK, "pgp_code", data)
			}

		} else if formName == "pgp_step2" {
			token, found := pgpTokenCache.Get(authUser.ID)
			if !found {
				return c.Redirect(http.StatusFound, "/settings/pgp")
			}

			data.PGPPublicKey = c.Request().PostFormValue("pgp_public_key")
			data.GpgMode = utils.DoParseBool(c.Request().PostFormValue("gpg_mode"))
			if data.GpgMode {
				data.ToBeSignedMessage = c.Request().PostFormValue("to_be_signed_message")
				data.SignedMessage = c.Request().PostFormValue("signed_message")
				if !utils.PgpCheckSignMessage(token.PKey, token.Value, data.SignedMessage) {
					data.ErrorSignedMessage = "invalid signature"
					return c.Render(http.StatusOK, "pgp_code", data)
				}

			} else {
				data.EncryptedMessage = c.Request().PostFormValue("encrypted_message")
				data.Code = c.Request().PostFormValue("pgp_code")
				if data.Code != token.Value {
					data.ErrorCode = "invalid code"
					return c.Render(http.StatusOK, "pgp_code", data)
				}
			}

			pgpTokenCache.Delete(authUser.ID)
			authUser.GPGPublicKey = token.PKey
			authUser.DoSave(db)
			return c.Redirect(http.StatusFound, "/settings/pgp")
		}
	}
	return c.Render(http.StatusOK, "pgp", data)
}

func SettingsPGPHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsPGPData
	data.ActiveTab = "pgp"

	if authUser.GPGPublicKey != "" {
		if e := utils.GetEntityFromPKey(authUser.GPGPublicKey); e != nil {
			data.PGPPublicKeyID = e.PrimaryKey.KeyIdString()
		}
	}

	return c.Render(http.StatusOK, "settings.pgp", data)
}

func SettingsAgeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data settingsAgeData
	data.ActiveTab = "age"
	data.AgePublicKey = authUser.AgePublicKey
	return c.Render(http.StatusOK, "settings.age", data)
}

func AddAgeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data addAgeData
	data.AgePublicKey = authUser.AgePublicKey
	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "age_step1" {
			data.AgePublicKey = c.Request().PostFormValue("age_public_key")
			msg, err := generateAgeEncryptedTokenMessage(authUser.ID, data.AgePublicKey)
			if err != nil {
				data.ErrorAgePublicKey = err.Error()
				return c.Render(http.StatusOK, "age", data)
			}
			data.EncryptedMessage = msg
			return c.Render(http.StatusOK, "age_code", data)

		} else if formName == "age_step2" {
			token, found := ageTokenCache.Get(authUser.ID)
			if !found {
				return c.Redirect(http.StatusFound, "/settings/age")
			}
			data.AgePublicKey = token.PKey
			data.EncryptedMessage = c.Request().PostFormValue("encrypted_message")
			data.Code = c.Request().PostFormValue("age_code")
			if data.Code != token.Value {
				data.ErrorCode = "invalid code"
				return c.Render(http.StatusOK, "age_code", data)
			}
			ageTokenCache.Delete(authUser.ID)
			authUser.SetAgePublicKey(db, token.PKey)
			return c.Redirect(http.StatusFound, "/settings/age")
		}
	}
	return c.Render(http.StatusOK, "age", data)
}

func generateAgeEncryptedTokenMessage(userID database.UserID, pkey string) (string, error) {
	token := utils.GenerateToken10()
	ageTokenCache.SetD(userID, ValueTokenCache{Value: token, PKey: pkey})

	recipient, err := age.ParseX25519Recipient(pkey)
	if err != nil {
		logrus.Error(err)
		return "", errors.New("invalid public key")
	}
	out := &bytes.Buffer{}
	aw := armor1.NewWriter(out)
	w, err := age.Encrypt(aw, recipient)
	msg := generateTokenMsg(token)
	if _, err := io.WriteString(w, msg); err != nil {
		logrus.Error(err)
		w.Close()
		aw.Close()
		return "", err
	}
	w.Close()
	aw.Close()

	return out.String(), nil
}

// SettingsWebsiteHandler ...
func SettingsWebsiteHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsWebsiteData
	data.ActiveTab = "website"
	settings := db.GetSettings()
	data.SignupEnabled = settings.SignupEnabled
	data.ForumEnabled = settings.ForumEnabled
	data.SilentSelfKick = settings.SilentSelfKick
	if c.Request().Method == http.MethodPost {
		settings.SignupEnabled = utils.DoParseBool(c.Request().PostFormValue("signupEnabled"))
		settings.ForumEnabled = utils.DoParseBool(c.Request().PostFormValue("forumEnabled"))
		settings.SilentSelfKick = utils.DoParseBool(c.Request().PostFormValue("silentSelfKick"))
		settings.DoSave(db)
		config.SignupEnabled.Store(settings.SignupEnabled)
		config.ForumEnabled.Store(settings.ForumEnabled)
		config.SilentSelfKick.Store(settings.SilentSelfKick)
		db.NewAudit(*authUser, fmt.Sprintf("website settings, signup: %t, forum: %t, sk: %t",
			settings.SignupEnabled, settings.ForumEnabled, settings.SilentSelfKick))
		return c.Redirect(http.StatusFound, "/settings/website")
	}

	return c.Render(http.StatusOK, "settings.website", data)
}

// SettingsInvitationsHandler ...
func SettingsInvitationsHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsInvitationsData
	data.ActiveTab = "invitations"
	data.DkfOnion = config.DkfOnion

	if c.Request().Method == http.MethodPost {
		if _, err := db.CreateInvitation(authUser.ID); err != nil {
			logrus.Error(err)
		}
		return c.Redirect(http.StatusFound, "/settings/invitations")
	}

	data.Invitations, _ = db.GetUserUnusedInvitations(authUser.ID)
	return c.Render(http.StatusOK, "settings.invitations", data)
}

func SettingsSecretPhraseHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data settingsSecretPhraseData
	data.ActiveTab = "secretPhrase"
	data.SecretPhrase = string(authUser.SecretPhrase)

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	// POST
	currentPassword := c.Request().PostFormValue("currentPassword")
	secretPhrase := c.Request().PostFormValue("secretPhrase")
	data.CurrentPassword = currentPassword
	data.SecretPhrase = secretPhrase

	if len(currentPassword) == 0 {
		data.ErrorCurrentPassword = "This field is required"
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	if !govalidator.RuneLength(secretPhrase, "3", "50") {
		data.ErrorSecretPhrase = "secret phrase must have between 3 and 50 characters"
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	if !authUser.CheckPassword(db, currentPassword) {
		data.ErrorCurrentPassword = "Invalid password"
		return c.Render(http.StatusOK, "settings.secret-phrase", data)
	}

	authUser.SecretPhrase = database.EncryptedString(secretPhrase)
	authUser.DoSave(db)

	db.CreateSecurityLog(authUser.ID, database.ChangeSecretPhraseSecurityLog)
	return c.Render(http.StatusFound, "flash", FlashResponse{Message: "Secret phrase changed successfully", Redirect: "/"})
}
