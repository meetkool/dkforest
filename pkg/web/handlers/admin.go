package handlers

import (
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/managers"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"github.com/asaskevich/govalidator"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
)

func AdminNewGistHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	var data adminCreateGistData
	data.ActiveTab = "gists"
	if c.Request().Method == http.MethodPost {
		data.Name = c.Request().PostFormValue("name")
		data.Password = c.Request().PostFormValue("password")
		data.Content = c.Request().PostFormValue("content")
		if !govalidator.Matches(data.Name, "^[a-zA-Z0-9_.]{3,50}$") {
			data.ErrorName = "invalid name"
			return c.Render(http.StatusOK, "admin.gist-create", data)
		}
		passwordHash := ""
		if data.Password != "" {
			passwordHash = utils.Sha512([]byte(config.GistPasswordSalt + data.Password))
		}
		gist := database.Gist{Name: data.Name, Password: passwordHash, UserID: authUser.ID, Content: data.Content}
		gist.UUID = uuid.New().String()
		if err := database.DB.Create(&gist).Error; err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "admin.gist-create", data)
		}
		return c.Redirect(http.StatusFound, "/gists/"+gist.UUID)
	}
	return c.Render(http.StatusOK, "admin.gist-create", data)
}

func AdminEditGistHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	gistUUID := c.Param("gistUUID")
	gist, err := database.GetGistByUUID(gistUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if gist.UserID != authUser.ID && !authUser.IsAdmin {
		return c.Redirect(http.StatusFound, "/")
	}
	var data adminCreateGistData
	data.ActiveTab = "gists"
	data.IsEdit = true
	data.Name = gist.Name
	data.Content = gist.Content
	if gist.Password != "" {
		data.Password = "*****"
	}

	if c.Request().Method == http.MethodPost {
		data.Name = c.Request().PostFormValue("name")
		data.Password = c.Request().PostFormValue("password")
		data.Content = c.Request().PostFormValue("content")
		if !govalidator.Matches(data.Name, "^[a-zA-Z0-9_.]{3,50}$") {
			data.ErrorName = "invalid name"
			return c.Render(http.StatusOK, "admin.gist-create", data)
		}
		passwordHash := ""
		if data.Password != "" && data.Password != "*****" {
			passwordHash = utils.Sha512([]byte(config.GistPasswordSalt + data.Password))
			gist.Password = passwordHash
		}
		gist.Name = data.Name
		gist.Content = data.Content
		gist.DoSave()
		return c.Redirect(http.StatusFound, "/gists/"+gist.UUID)
	}
	return c.Render(http.StatusOK, "admin.gist-create", data)
}

func AdminGistsHandler(c echo.Context) error {
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))

	var data adminGistsData
	data.ActiveTab = "gists"

	userQuery := c.QueryParam("u")

	query := database.DB.Table("gists")
	if userQuery != "" {
		query = query.Where("user_id = ?", userQuery)
	}
	query.Count(&data.GistsCount)

	data.CurrentPage, data.MaxPage = Paginate(ResultsPerPage, wantedPage, data.GistsCount)

	resultsPerPage := int64(100)
	query = database.DB.
		Unscoped().
		Preload("User").
		Offset((data.CurrentPage - 1) * resultsPerPage).
		Limit(resultsPerPage).
		Order("id DESC")
	if userQuery != "" {
		query = query.Where("user_id = ?", userQuery)
	}
	if err := query.Find(&data.Gists).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "admin.gists", data)
}

func AdminUploadsHandler(c echo.Context) error {
	if c.Request().Method == http.MethodPost {
		formName := c.FormValue("formName")
		if formName == "deleteUpload" {
			fileName := c.Request().PostFormValue("file_name")
			file, err := database.GetUploadByFileName(fileName)
			if err != nil {
				return c.Redirect(http.StatusFound, "/")
			}
			if err := os.Remove(filepath.Join("uploads", file.FileName)); err != nil {
				logrus.Error(err.Error())
			}
			if err := database.DB.Delete(&file).Error; err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}
	}

	var data adminUploadsData
	data.ActiveTab = "uploads"
	data.Uploads, _ = database.GetUploads()
	for _, f := range data.Uploads {
		data.TotalSize += f.FileSize
	}
	return c.Render(http.StatusOK, "admin.uploads", data)
}

func AdminFiledropsHandler(c echo.Context) error {
	if c.Request().Method == http.MethodPost {
		formName := c.FormValue("formName")
		if formName == "createFiledrop" {
			if _, err := database.CreateFiledrop(); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())

		} else if formName == "deleteFiledrop" {
			fileName := c.Request().PostFormValue("file_name")
			file, err := database.GetFiledropByFileName(fileName)
			if err != nil {
				return c.Redirect(http.StatusFound, "/")
			}
			if err := os.Remove(filepath.Join("filedrop", file.FileName)); err != nil {
				logrus.Error(err.Error())
			}
			if err := database.DB.Delete(&file).Error; err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}
	}

	var data adminFiledropsData
	data.ActiveTab = "filedrops"
	data.Filedrops, _ = database.GetFiledrops()
	for _, f := range data.Filedrops {
		data.TotalSize += f.FileSize
	}
	return c.Render(http.StatusOK, "admin.filedrops", data)
}

func AdminDownloadsHandler(c echo.Context) error {
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))

	var data adminDownloadsData
	data.ActiveTab = "downloads"

	userQuery := c.QueryParam("u")

	query := database.DB.Table("downloads")
	if userQuery != "" {
		query = query.Where("user_id = ?", userQuery)
	}
	query.Count(&data.DownloadsCount)

	data.CurrentPage, data.MaxPage = Paginate(ResultsPerPage, wantedPage, data.DownloadsCount)

	resultsPerPage := int64(100)
	query = database.DB.
		Unscoped().
		Preload("User").
		Offset((data.CurrentPage - 1) * resultsPerPage).
		Limit(resultsPerPage).
		Order("id DESC")
	if userQuery != "" {
		query = query.Where("user_id = ?", userQuery)
	}
	if err := query.Find(&data.Downloads).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "admin.downloads", data)
}

// AdminSettingsHandler ...
func AdminSettingsHandler(c echo.Context) error {
	var data adminSettingsData
	data.ActiveTab = "settings"
	settings := database.GetSettings()
	data.ProtectHome = settings.ProtectHome
	data.HomeUsersList = settings.HomeUsersList
	data.ForceLoginCaptcha = settings.ForceLoginCaptcha
	data.SignupEnabled = settings.SignupEnabled
	data.SignupFakeEnabled = settings.SignupFakeEnabled
	data.DownloadsEnabled = settings.DownloadsEnabled
	data.ForumEnabled = settings.ForumEnabled
	data.MaybeAuthEnabled = settings.MaybeAuthEnabled
	data.CaptchaDifficulty = settings.CaptchaDifficulty

	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "openProjectFolder" {
			if err := open.Run(config.Global.ProjectPath()); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())

		} else if formName == "saveSettings" {
			settings := database.GetSettings()
			settings.ProtectHome = utils.DoParseBool(c.Request().PostFormValue("protectHome"))
			settings.HomeUsersList = utils.DoParseBool(c.Request().PostFormValue("homeUsersList"))
			settings.ForceLoginCaptcha = utils.DoParseBool(c.Request().PostFormValue("forceLoginCaptcha"))
			settings.SignupEnabled = utils.DoParseBool(c.Request().PostFormValue("signupEnabled"))
			settings.SignupFakeEnabled = utils.DoParseBool(c.Request().PostFormValue("signupFakeEnabled"))
			settings.DownloadsEnabled = utils.DoParseBool(c.Request().PostFormValue("downloadsEnabled"))
			settings.ForumEnabled = utils.DoParseBool(c.Request().PostFormValue("forumEnabled"))
			settings.MaybeAuthEnabled = utils.DoParseBool(c.Request().PostFormValue("maybeAuthEnabled"))
			settings.CaptchaDifficulty = utils.DoParseInt64(c.Request().PostFormValue("captchaDifficulty"))
			_ = settings.Save()
			config.ProtectHome.Store(settings.ProtectHome)
			config.HomeUsersList.Store(settings.HomeUsersList)
			config.ForceLoginCaptcha.Store(settings.ForceLoginCaptcha)
			config.SignupEnabled.Store(settings.SignupEnabled)
			config.CaptchaDifficulty.Store(settings.CaptchaDifficulty)
			config.SignupFakeEnabled.Store(settings.SignupFakeEnabled)
			config.DownloadsEnabled.Store(settings.DownloadsEnabled)
			config.ForumEnabled.Store(settings.ForumEnabled)
			config.MaybeAuthEnabled.Store(settings.MaybeAuthEnabled)
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}

	}

	return c.Render(http.StatusOK, "admin.settings", data)
}

// ResultsPerPage ...
var ResultsPerPage = int64(50)

// Paginate ...
func Paginate(resultsPerPage, wantedPage, size int64) (page int64, maxPage int64) {
	page = wantedPage
	if page <= 1 {
		page = 1
	}
	maxPage = int64(math.Ceil(float64(size) / float64(resultsPerPage)))
	if maxPage <= 1 {
		maxPage = 1
	}
	if page > maxPage {
		page = maxPage
	}
	return
}

func AdminHandler(c echo.Context) error {
	var data adminData
	data.ActiveTab = "users"
	data.Query = strings.TrimSpace(c.QueryParam("q"))
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))
	likeQuery := "%" + data.Query + "%"

	query := database.DB.Table("users")
	if data.Query != "" {
		query = query.Where("username LIKE ?", likeQuery, likeQuery, data.Query)
	}
	query.Count(&data.UsersCount)

	page, maxPage := Paginate(ResultsPerPage, wantedPage, data.UsersCount)

	query = database.DB.
		Order("id DESC").
		Offset((page - 1) * ResultsPerPage).
		Limit(ResultsPerPage)
	if data.Query != "" {
		query = query.Where("username LIKE ?", likeQuery, likeQuery, data.Query)
	}
	if err := query.Find(&data.Users).Error; err != nil {
		logrus.Error(err)
	}
	data.CurrentPage = page
	data.MaxPage = maxPage
	return c.Render(http.StatusOK, "admin.users", data)
}

func SessionsHandler(c echo.Context) error {
	var data adminSessionsData
	data.ActiveTab = "sessions"
	data.Query = c.QueryParam("q")
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))
	likeQuery := "%" + data.Query + "%"

	query := database.DB.Table("sessions").Where("deleted_at IS NULL")
	if data.Query != "" {
		query = query.Where("token LIKE ?", likeQuery, likeQuery, data.Query)
	}
	query.Count(&data.SessionsCount)

	page, maxPage := Paginate(ResultsPerPage, wantedPage, data.SessionsCount)

	query = database.DB.
		Order("created_at DESC").
		Offset((page - 1) * ResultsPerPage).
		Limit(ResultsPerPage)
	if data.Query != "" {
		query = query.Where("token LIKE ?", likeQuery, likeQuery, data.Query)
	}
	if err := query.Preload("User").Find(&data.Sessions).Error; err != nil {
		logrus.Error(err)
	}
	data.CurrentPage = page
	data.MaxPage = maxPage
	return c.Render(http.StatusOK, "admin.sessions", data)
}

func IgnoredHandler(c echo.Context) error {
	var data adminIgnoredData
	data.ActiveTab = "ignored"
	data.Query = c.QueryParam("q")
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))
	likeQuery := "%" + data.Query + "%"

	query := database.DB.Table("ignored_users")
	if data.Query != "" {
		query = query.Where("token LIKE ?", likeQuery, likeQuery, data.Query)
	}
	query.Count(&data.IgnoredCount)

	page, maxPage := Paginate(ResultsPerPage, wantedPage, data.IgnoredCount)

	query = database.DB.
		Order("created_at DESC").
		Offset((page - 1) * ResultsPerPage).
		Limit(ResultsPerPage)
	if data.Query != "" {
		query = query.Where("token LIKE ?", likeQuery, likeQuery, data.Query)
	}
	if err := query.Preload("User").Preload("IgnoredUser").Find(&data.Ignored).Error; err != nil {
		logrus.Error(err)
	}
	data.CurrentPage = page
	data.MaxPage = maxPage
	return c.Render(http.StatusOK, "admin.ignored", data)
}

func DdosHandler(c echo.Context) error {
	var data adminDdosData
	data.ActiveTab = "ddos"
	data.RPS = config.RpsCounter.Rate()
	data.RejectedReq = config.RejectedReqCounter.Rate()
	data.SignupPageLoad = config.SignupPageLoad.Load()
	data.SignupFailed = config.SignupFailed.Load()
	data.SignupSucceed = config.SignupSucceed.Load()
	data.BHCCaptchaGenerated = config.BHCCaptchaGenerated.Load()
	data.BHCCaptchaSuccess = config.BHCCaptchaSuccess.Load()
	data.BHCCaptchaFailed = config.BHCCaptchaFailed.Load()
	data.CaptchaRequiredGenerated = config.CaptchaRequiredGenerated.Load()
	data.CaptchaRequiredSuccess = config.CaptchaRequiredSuccess.Load()
	data.CaptchaRequiredFailed = config.CaptchaRequiredFailed.Load()
	return c.Render(http.StatusOK, "admin.ddos", data)
}

func BackupHandler(c echo.Context) error {
	var data backupData
	data.ActiveTab = "backup"
	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "backup" {
			if config.MaintenanceAtom.CAS(false, true) {
				utils.SGo(func() {
					defer config.MaintenanceAtom.SetFalse()
					if err := database.Backup(); err != nil {
						logrus.Error("Failed to backup database: ", err)
					}
				})
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		} else if formName == "toggleMaintenance" {
			if config.MaintenanceAtom.CAS(false, true) {
				logrus.Info("maintenance mode turned on")
			} else if config.MaintenanceAtom.CAS(true, false) {
				logrus.Info("maintenance mode turned off")
			}
			return c.Redirect(http.StatusFound, c.Request().Referer())
		}
	}

	return c.Render(http.StatusOK, "admin.backup", data)
}

func AdminAuditsHandler(c echo.Context) error {
	var data adminAuditsData
	data.ActiveTab = "audits"
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))

	query := database.DB.Table("audit_logs")
	query.Count(&data.AuditLogsCount)

	page, maxPage := Paginate(ResultsPerPage, wantedPage, data.AuditLogsCount)

	query = database.DB.
		Preload("User").
		Order("id DESC").
		Offset((page - 1) * ResultsPerPage).
		Limit(ResultsPerPage)
	if err := query.Find(&data.AuditLogs).Error; err != nil {
		logrus.Error(err)
	}
	data.CurrentPage = page
	data.MaxPage = maxPage
	return c.Render(http.StatusOK, "admin.audits", data)
}

func AdminRoomsHandler(c echo.Context) error {
	var data adminRoomsData
	data.ActiveTab = "rooms"
	data.Query = c.QueryParam("q")
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))
	likeQuery := "%" + data.Query + "%"

	query := database.DB.Table("chat_rooms")
	if data.Query != "" {
		query = query.Where("username LIKE ?", likeQuery, likeQuery, data.Query)
	}
	query.Count(&data.RoomsCount)

	page, maxPage := Paginate(ResultsPerPage, wantedPage, data.RoomsCount)

	query = database.DB.
		Order("id DESC").
		Preload("OwnerUser").
		Offset((page - 1) * ResultsPerPage).
		Limit(ResultsPerPage)
	if data.Query != "" {
		query = query.Where("username LIKE ?", likeQuery, likeQuery, data.Query)
	}
	if err := query.Unscoped().Find(&data.Rooms).Error; err != nil {
		logrus.Error(err)
	}
	data.CurrentPage = page
	data.MaxPage = maxPage
	return c.Render(http.StatusOK, "admin.rooms", data)
}

func AdminCaptchaHandler(c echo.Context) error {
	var data adminCaptchaData
	data.ActiveTab = "captcha"
	wantedPage := utils.DoParseInt64(c.QueryParam("p"))

	database.DB.Table("captcha_requests").Count(&data.CaptchasCount)

	page, maxPage := Paginate(ResultsPerPage, wantedPage, data.CaptchasCount)

	if err := database.DB.
		Preload("User").
		Order("id DESC").
		Offset((page - 1) * ResultsPerPage).
		Limit(ResultsPerPage).
		Find(&data.Captchas).Error; err != nil {
		logrus.Error(err)
	}
	data.CurrentPage = page
	data.MaxPage = maxPage
	return c.Render(http.StatusOK, "admin.captcha", data)
}

// AdminDeleteUserHandler ...
func AdminDeleteUserHandler(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("userID"))
	if err != nil {
		return c.Render(http.StatusOK, "flash",
			FlashResponse{"user id not found", c.Request().Referer(), "alert-danger"})
	}
	if id == 1 {
		return c.Render(http.StatusOK, "flash",
			FlashResponse{"Root admin cannot be deleted", c.Request().Referer(), "alert-danger"})
	}
	if err := database.DB.Unscoped().Delete(database.User{}, "id = ?", id).Error; err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func IgnoredDeleteHandler(c echo.Context) error {
	userID := utils.DoParseInt64(c.Request().PostFormValue("user_id"))
	ignoredUserID := utils.DoParseInt64(c.Request().PostFormValue("ignored_user_id"))
	if err := database.DB.Delete(database.IgnoredUser{}, "user_id = ? AND ignored_user_id = ?", userID, ignoredUserID).Error; err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

// AdminDeleteRoomHandler ...
func AdminDeleteRoomHandler(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("roomID"))
	if err != nil {
		return c.Render(http.StatusOK, "flash",
			FlashResponse{"room id not found", c.Request().Referer(), "alert-danger"})
	}
	if err := database.DB.Unscoped().Delete(database.ChatRoom{}, "id = ?", id).Error; err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, c.Request().Referer())
}

func AdminUserSecurityLogsHandler(c echo.Context) error {
	//authUser := c.Get("authUser").(*database.User)
	userID, err := dutils.ParseUserID(c.Param("userID"))
	if err != nil {
		return c.Redirect(http.StatusFound, "/admin/users")
	}
	var data settingsSecurityData
	data.ActiveTab = "security"
	data.Logs, _ = database.GetSecurityLogs(userID)
	return c.Render(http.StatusOK, "admin.user-security-logs", data)
}

// AdminEditUserHandler ...
func AdminEditUserHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	userID, err := strconv.Atoi(c.Param("userID"))
	if err != nil {
		return c.Redirect(http.StatusFound, "/admin/users")
	}
	// Only root admin can edit the root admin
	if userID == config.RootAdminID && authUser.ID != config.RootAdminID {
		return c.Redirect(http.StatusFound, "/admin/users")
	}
	var user database.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/users")
	}
	var data adminEditUsereData
	data.ActiveTab = "users"
	data.User = user
	data.ChatTutorial = user.ChatTutorial
	data.AllFonts = utils.GetFonts()
	data.IsEdit = true
	data.Username = user.Username
	data.ApiKey = user.ApiKey
	data.IsAdmin = user.IsAdmin
	data.IsHellbanned = user.IsHellbanned
	data.Verified = user.Verified
	data.IsClubMember = user.IsClubMember
	data.CanUploadFile = user.CanUploadFile
	data.CanUseForum = user.CanUseForum
	data.CanUseMultiline = user.CanUseMultiline
	data.CanSeeHellbanned = user.CanSeeHellbanned
	data.IsIncognito = user.IsIncognito
	data.CanChangeUsername = user.CanChangeUsername
	data.CanChangeColor = user.CanChangeColor
	data.Vetted = user.Vetted
	data.Role = user.Role
	data.ChatColor = user.ChatColor
	data.ChatFont = user.ChatFont
	data.SignupMetadata = user.SignupMetadata
	data.CollectMetadata = user.CollectMetadata
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "admin.user-edit", data)
	}

	formName := c.Request().PostFormValue("formName")
	if formName == "reset_tutorial" {
		user.ChatTutorial = 0
		user.DoSave()
		return c.Redirect(http.StatusFound, c.Request().Referer())
	}

	data.Username = c.FormValue("username")
	data.Role = c.Request().PostFormValue("role")
	data.IsAdmin = utils.DoParseBool(c.FormValue("isAdmin"))
	data.IsHellbanned = utils.DoParseBool(c.FormValue("isHellbanned"))
	data.Verified = utils.DoParseBool(c.FormValue("verified"))
	data.IsClubMember = utils.DoParseBool(c.FormValue("is_club_member"))
	data.CanUploadFile = utils.DoParseBool(c.FormValue("can_upload_file"))
	data.CanUseForum = utils.DoParseBool(c.FormValue("can_use_forum"))
	data.CanUseMultiline = utils.DoParseBool(c.FormValue("can_use_multiline"))
	data.CanSeeHellbanned = utils.DoParseBool(c.FormValue("can_see_hellbanned"))
	data.IsIncognito = utils.DoParseBool(c.FormValue("is_incognito"))
	data.CanChangeUsername = utils.DoParseBool(c.FormValue("can_change_username"))
	data.CanChangeColor = utils.DoParseBool(c.FormValue("can_change_color"))
	data.Vetted = utils.DoParseBool(c.FormValue("vetted"))
	data.CollectMetadata = utils.DoParseBool(c.FormValue("collect_metadata"))
	data.ChatColor = c.FormValue("chat_color")
	data.ChatFont = utils.DoParseInt64(c.FormValue("chat_font"))
	if data.Username != user.Username {
		if _, err := database.ValidateUsername(data.Username, false); err != nil {
			data.Errors.Username = err.Error()
		}
		var existingUser database.User
		database.DB.Select("username").Where("(username = ? COLLATE NOCASE) and id != ?", data.Username, user.ID).First(&existingUser)
		if existingUser.Username != "" && existingUser.Username == data.Username {
			data.Errors.Username = "Username already exists"
		}
	}
	// Edit password
	var hashedPassword string
	data.Password = c.Request().PostFormValue("password")
	data.RePassword = c.Request().PostFormValue("repassword")
	data.ApiKey = c.Request().PostFormValue("api_key")
	if data.Password != "" || data.RePassword != "" {
		hashedPassword, err = database.NewPasswordValidator(data.Password).CompareWith(data.RePassword).Hash()
		if err != nil {
			data.Errors.Password = err.Error()
		}
	}
	if data.Errors.HasError() {
		return c.Render(http.StatusOK, "admin.user-edit", data)
	}

	if hashedPassword != "" {
		if err := user.ChangePassword(hashedPassword); err != nil {
			data.Errors.Password = err.Error()
			return c.Render(http.StatusOK, "admin.user-edit", data)
		}
	}

	user.Username = data.Username
	user.IsAdmin = data.IsAdmin
	if data.IsHellbanned {
		user.HellBan()
		managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(user, nil))
	}
	user.ApiKey = data.ApiKey
	user.Verified = data.Verified
	user.IsHellbanned = data.IsHellbanned
	user.IsClubMember = data.IsClubMember
	user.CanUploadFile = data.CanUploadFile
	user.CanUseForum = data.CanUseForum
	user.CanUseMultiline = data.CanUseMultiline
	user.CanSeeHellbanned = data.CanSeeHellbanned
	user.IsIncognito = data.IsIncognito
	user.CanChangeUsername = data.CanChangeUsername
	user.CanChangeColor = data.CanChangeColor
	user.Vetted = data.Vetted
	user.CollectMetadata = data.CollectMetadata
	user.Role = data.Role
	user.ChatColor = data.ChatColor
	user.ChatFont = data.ChatFont
	user.DoSave()

	return c.Redirect(http.StatusFound, c.Request().Referer())
}

// AdminEditRoomHandler ...
func AdminEditRoomHandler(c echo.Context) error {
	roomID, err := strconv.Atoi(c.Param("roomID"))
	if err != nil {
		return c.Redirect(http.StatusFound, "/admin/rooms")
	}
	var room database.ChatRoom
	if err := database.DB.First(&room, roomID).Error; err != nil {
		return c.Redirect(http.StatusFound, "/admin/rooms")
	}
	var data adminEditRoomData
	data.ActiveTab = "rooms"
	data.IsEdit = true
	data.IsEphemeral = room.IsEphemeral
	data.IsListed = room.IsListed
	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, "admin.room-edit", data)
	}

	data.IsEphemeral = utils.DoParseBool(c.Request().PostFormValue("is_ephemeral"))
	data.IsListed = utils.DoParseBool(c.Request().PostFormValue("is_listed"))

	room.IsEphemeral = data.IsEphemeral
	room.IsListed = data.IsListed
	room.DoSave()

	return c.Redirect(http.StatusFound, "/admin/rooms")
}
