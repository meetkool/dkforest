package handlers

import (
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/managers"
	"dkforest/pkg/web/handlers/interceptors"
	"dkforest/pkg/web/handlers/usersStreamsManager"
	hutils "dkforest/pkg/web/handlers/utils"
	"fmt"
	wallet1 "github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"gorm.io/gorm"
	"io"
	"math"
	"net/http"
	"regexp"
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

func AdminSpamFiltersHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminSpamFiltersData
	data.ActiveTab = "spamfilters"
	data.SpamFilters, _ = db.GetSpamFilters()
	data.SpamFiltersCount = int64(len(data.SpamFilters))

	if c.Request().Method == http.MethodPost {
		btnSubmit := c.Request().PostFormValue("btn_submit")
		data.ID = utils.DoParseInt64(c.Request().PostFormValue("id"))
		data.Filter = c.Request().PostFormValue("filter")
		data.IsRegex = utils.DoParseBool(c.Request().PostFormValue("is_regex"))
		data.Action = utils.Clamp(utils.DoParseInt64(c.Request().PostFormValue("action")), 0, 2)
		if !utils.ValidateRuneLength(data.Filter, 1, 255) {
			data.Error = "filter must be within 1-255 characters"
			return c.Render(http.StatusOK, "admin.spam-filter", data)
		}
		if data.ID == 0 || btnSubmit == "edit" {
			if _, err := db.CreateOrEditSpamFilter(data.ID, data.Filter, data.IsRegex, data.Action); err != nil {
				logrus.Error(err)
			}
			interceptors.LoadFilters(db)
		} else if btnSubmit == "delete" {
			if err := db.DeleteSpamFilterByID(data.ID); err != nil {
				logrus.Error(err)
			}
			interceptors.LoadFilters(db)
		}
		return c.Redirect(http.StatusFound, "/admin/spam-filters")
	}

	return c.Render(http.StatusOK, "admin.spam-filters", data)
}

func AdminNewGistHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
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
			passwordHash = database.GetGistPasswordHash(data.Password)
		}
		gist := database.Gist{UUID: uuid.New().String(), Name: data.Name, Password: passwordHash, UserID: authUser.ID, Content: data.Content}
		if err := db.DB().Create(&gist).Error; err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "admin.gist-create", data)
		}
		return c.Redirect(http.StatusFound, "/gists/"+gist.UUID)
	}
	return c.Render(http.StatusOK, "admin.gist-create", data)
}

func AdminEditGistHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	gistUUID := c.Param("gistUUID")
	gist, err := db.GetGistByUUID(gistUUID)
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
			passwordHash = database.GetGistPasswordHash(data.Password)
			gist.Password = passwordHash
		}
		gist.Name = data.Name
		gist.Content = data.Content
		gist.DoSave(db)
		return c.Redirect(http.StatusFound, "/gists/"+gist.UUID)
	}
	return c.Render(http.StatusOK, "admin.gist-create", data)
}

func AdminGistsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminGistsData
	data.ActiveTab = "gists"

	userQuery := c.QueryParam("u")

	if err := db.DB().Table("gists").
		Scopes(func(query *gorm.DB) *gorm.DB {
			if userQuery != "" {
				query = query.Where("user_id = ?", userQuery)
			}
			data.CurrentPage, data.MaxPage, data.GistsCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Preload("User").
		Order("id DESC").
		Find(&data.Gists).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "admin.gists", data)
}

func AdminUploadsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	if c.Request().Method == http.MethodPost {
		formName := c.FormValue("formName")
		if formName == "deleteUpload" {
			fileName := c.Request().PostFormValue("file_name")
			file, err := db.GetUploadByFileName(fileName)
			if err != nil {
				return c.Redirect(http.StatusFound, "/")
			}
			if err := file.Delete(db); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, "/admin/uploads")
		}
	}

	var data adminUploadsData
	data.ActiveTab = "uploads"
	data.Uploads, _ = db.GetUploads()
	for _, f := range data.Uploads {
		data.TotalSize += f.FileSize
	}
	return c.Render(http.StatusOK, "admin.uploads", data)
}

func AdminFiledropsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	if c.Request().Method == http.MethodPost {
		formName := c.FormValue("formName")
		if formName == "createFiledrop" {
			if _, err := db.CreateFiledrop(); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, "/admin/filedrops")

		} else if formName == "deleteFiledrop" {
			fileName := c.Request().PostFormValue("file_name")
			file, err := db.GetFiledropByFileName(fileName)
			if err != nil {
				return c.Redirect(http.StatusFound, "/admin/filedrops")
			}
			if err := file.Delete(db); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, "/admin/filedrops")
		}
	}

	var data adminFiledropsData
	data.ActiveTab = "filedrops"
	data.Filedrops, _ = db.GetFiledrops()
	for _, f := range data.Filedrops {
		data.TotalSize += f.FileSize
	}
	return c.Render(http.StatusOK, "admin.filedrops", data)
}

func AdminDownloadsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminDownloadsData
	data.ActiveTab = "downloads"

	userQuery := c.QueryParam("u")

	db.DB().Model(&database.Download{}).
		Scopes(func(query *gorm.DB) *gorm.DB {
			if userQuery != "" {
				query = query.Where("user_id = ?", userQuery)
			}
			data.CurrentPage, data.MaxPage, data.DownloadsCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Preload("User").
		Order("id DESC").
		Find(&data.Downloads)

	return c.Render(http.StatusOK, "admin.downloads", data)
}

func AdminDeleteDownloadHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	downloadID, err := utils.ParseInt64(c.Param("downloadID"))
	if err != nil {
		return c.Render(http.StatusOK, "flash",
			FlashResponse{"download id not found", "/admin/downloads", "alert-danger"})
	}

	if err := db.DeleteDownloadByID(downloadID); err != nil {
		logrus.Error(err)
	}
	return c.Redirect(http.StatusFound, "/admin/downloads")
}

// AdminSettingsHandler ...
func AdminSettingsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminSettingsData
	data.ActiveTab = "settings"
	settings := db.GetSettings()
	data.ProtectHome = settings.ProtectHome
	data.HomeUsersList = settings.HomeUsersList
	data.ForceLoginCaptcha = settings.ForceLoginCaptcha
	data.SignupEnabled = settings.SignupEnabled
	data.SignupFakeEnabled = settings.SignupFakeEnabled
	data.DownloadsEnabled = settings.DownloadsEnabled
	data.ForumEnabled = settings.ForumEnabled
	data.MaybeAuthEnabled = settings.MaybeAuthEnabled
	data.CaptchaDifficulty = settings.CaptchaDifficulty
	data.PowEnabled = settings.PowEnabled
	data.PokerWithdrawEnabled = settings.PokerWithdrawEnabled
	data.MoneroPrice = settings.MoneroPrice

	if c.Request().Method == http.MethodPost {
		formName := c.Request().PostFormValue("formName")
		if formName == "openProjectFolder" {
			if err := open.Run(config.Global.ProjectPath.Get()); err != nil {
				logrus.Error(err)
			}
			return c.Redirect(http.StatusFound, "/admin/settings")

		} else if formName == "saveSettings" {
			settings := db.GetSettings()
			settings.ProtectHome = utils.DoParseBool(c.Request().PostFormValue("protectHome"))
			settings.HomeUsersList = utils.DoParseBool(c.Request().PostFormValue("homeUsersList"))
			settings.ForceLoginCaptcha = utils.DoParseBool(c.Request().PostFormValue("forceLoginCaptcha"))
			settings.SignupEnabled = utils.DoParseBool(c.Request().PostFormValue("signupEnabled"))
			settings.SignupFakeEnabled = utils.DoParseBool(c.Request().PostFormValue("signupFakeEnabled"))
			settings.DownloadsEnabled = utils.DoParseBool(c.Request().PostFormValue("downloadsEnabled"))
			settings.ForumEnabled = utils.DoParseBool(c.Request().PostFormValue("forumEnabled"))
			settings.MaybeAuthEnabled = utils.DoParseBool(c.Request().PostFormValue("maybeAuthEnabled"))
			settings.CaptchaDifficulty = utils.DoParseInt64(c.Request().PostFormValue("captchaDifficulty"))
			settings.PowEnabled = utils.DoParseBool(c.Request().PostFormValue("powEnabled"))
			settings.PokerWithdrawEnabled = utils.DoParseBool(c.Request().PostFormValue("pokerWithdrawEnabled"))
			settings.MoneroPrice = math.Max(utils.DoParseF64(c.Request().PostFormValue("moneroPrice")), 1)
			settings.DoSave(db)
			config.ProtectHome.Store(settings.ProtectHome)
			config.HomeUsersList.Store(settings.HomeUsersList)
			config.ForceLoginCaptcha.Store(settings.ForceLoginCaptcha)
			config.SignupEnabled.Store(settings.SignupEnabled)
			config.CaptchaDifficulty.Store(settings.CaptchaDifficulty)
			config.PowEnabled.Store(settings.PowEnabled)
			config.PokerWithdrawEnabled.Store(settings.PokerWithdrawEnabled)
			config.SignupFakeEnabled.Store(settings.SignupFakeEnabled)
			config.DownloadsEnabled.Store(settings.DownloadsEnabled)
			config.ForumEnabled.Store(settings.ForumEnabled)
			config.MaybeAuthEnabled.Store(settings.MaybeAuthEnabled)
			config.MoneroPrice.Store(settings.MoneroPrice)
			return c.Redirect(http.StatusFound, "/admin/settings")
		}

	}

	return c.Render(http.StatusOK, "admin.settings", data)
}

func AdminHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminData
	data.ActiveTab = "users"
	data.Query = strings.TrimSpace(c.QueryParam("q"))
	likeQuery := "%" + data.Query + "%"

	if err := db.DB().
		Table("users").
		Scopes(func(query *gorm.DB) *gorm.DB {
			if data.Query != "" {
				query = query.Where("username LIKE ?", likeQuery, likeQuery, data.Query)
			}
			data.CurrentPage, data.MaxPage, data.UsersCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Order("id DESC").
		Find(&data.Users).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "admin.users", data)
}

func SessionsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminSessionsData
	data.ActiveTab = "sessions"
	data.Query = c.QueryParam("q")
	likeQuery := "%" + data.Query + "%"

	if err := db.DB().
		Table("sessions").
		Where("deleted_at IS NULL").
		Scopes(func(query *gorm.DB) *gorm.DB {
			if data.Query != "" {
				query = query.Where("token LIKE ?", likeQuery, likeQuery, data.Query)
			}
			data.CurrentPage, data.MaxPage, data.SessionsCount, query = NewPaginator().Paginate(c, query)
			return query
		}).Preload("User").
		Order("created_at DESC").
		Find(&data.Sessions).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "admin.sessions", data)
}

func IgnoredHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminIgnoredData
	data.ActiveTab = "ignored"
	data.Query = c.QueryParam("q")
	likeQuery := "%" + data.Query + "%"

	if err := db.DB().
		Table("ignored_users").
		Scopes(func(query *gorm.DB) *gorm.DB {
			if data.Query != "" {
				query = query.Where("token LIKE ?", likeQuery, likeQuery, data.Query)
			}
			data.CurrentPage, data.MaxPage, data.IgnoredCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Preload("User").
		Preload("IgnoredUser").
		Order("created_at DESC").
		Find(&data.Ignored).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "admin.ignored", data)
}

func DdosHandler(c echo.Context) error {
	if c.Request().Method == http.MethodPost {
		config.SignupPageLoad.Store(0)
		config.SignupFailed.Store(0)
		config.SignupSucceed.Store(0)
		config.BHCCaptchaGenerated.Store(0)
		config.BHCCaptchaSuccess.Store(0)
		config.BHCCaptchaFailed.Store(0)
		config.CaptchaRequiredGenerated.Store(0)
		config.CaptchaRequiredSuccess.Store(0)
		config.CaptchaRequiredFailed.Store(0)
		return c.Redirect(http.StatusFound, "/admin/ddos")
	}
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
			return c.Redirect(http.StatusFound, "/admin/backup")
		} else if formName == "toggleMaintenance" {
			if config.MaintenanceAtom.CAS(false, true) {
				logrus.Info("maintenance mode turned on")
			} else if config.MaintenanceAtom.CAS(true, false) {
				logrus.Info("maintenance mode turned off")
			}
			return c.Redirect(http.StatusFound, "/admin/backup")
		}
	}

	return c.Render(http.StatusOK, "admin.backup", data)
}

func AdminAuditsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminAuditsData
	data.ActiveTab = "audits"

	if err := db.DB().
		Table("audit_logs").
		Scopes(func(query *gorm.DB) *gorm.DB {
			data.CurrentPage, data.MaxPage, data.AuditLogsCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Preload("User").
		Order("id DESC").
		Find(&data.AuditLogs).Error; err != nil {
		logrus.Error(err)
	}
	return c.Render(http.StatusOK, "admin.audits", data)
}

func AdminRoomsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminRoomsData
	data.ActiveTab = "rooms"
	data.Query = c.QueryParam("q")
	likeQuery := "%" + data.Query + "%"

	if err := db.DB().
		Table("chat_rooms").
		Scopes(func(query *gorm.DB) *gorm.DB {
			if data.Query != "" {
				query = query.Where("username LIKE ?", likeQuery, likeQuery, data.Query)
			}
			data.CurrentPage, data.MaxPage, data.RoomsCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Order("id DESC").
		Preload("OwnerUser").
		Find(&data.Rooms).Error; err != nil {
		logrus.Error(err)
	}
	return c.Render(http.StatusOK, "admin.rooms", data)
}

func AdminPokerTransactionsHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminPokerTransactionsData
	data.ActiveTab = "pokerTransactions"
	data.PokerCasino = db.GetPokerCasino()
	res, err := config.Xmr().GetBalance(&wallet1.RequestGetBalance{})
	if err != nil {
		logrus.Error(err)
		return hutils.RedirectReferer(c)
	}
	data.Balance = database.Piconero(res.Balance)
	data.UnlockedBalance = database.Piconero(res.UnlockedBalance)
	data.SumIn, _ = db.GetPokerXmrTransactionsSumIn()
	data.SumOut, _ = db.GetPokerXmrTransactionsSumOut()
	data.DiffInOut = data.SumIn - data.SumOut
	sumXmrBalance, _ := db.GetUsersXmrBalance()
	data.UsersRakeBack, _ = db.GetUsersRakeBack()
	sumTableAccounts, sumTableBets, _ := db.GetPokerTableAccountSums()
	data.Discrepancy = (int64(data.SumIn) - int64(data.SumOut)) -
		int64(sumXmrBalance) -
		int64(data.UsersRakeBack.ToPiconero()) -
		int64(data.PokerCasino.Rake.ToPiconero()) -
		int64(sumTableAccounts.ToPiconero()) -
		int64(sumTableBets.ToPiconero())
	data.DiscrepancyPiconero = database.Piconero(uint64(math.Abs(float64(data.Discrepancy))))

	if err := db.DB().
		Table("poker_xmr_transactions").
		Scopes(func(query *gorm.DB) *gorm.DB {
			data.CurrentPage, data.MaxPage, data.TransactionsCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Order("id DESC").
		Preload("User").
		Find(&data.Transactions).Error; err != nil {
		logrus.Error(err)
	}

	return c.Render(http.StatusOK, "admin.poker-transactions", data)
}

func AdminCaptchaHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	var data adminCaptchaData
	data.ActiveTab = "captcha"

	if err := db.DB().Table("captcha_requests").
		Scopes(func(query *gorm.DB) *gorm.DB {
			data.CurrentPage, data.MaxPage, data.CaptchasCount, query = NewPaginator().Paginate(c, query)
			return query
		}).
		Preload("User").
		Order("id DESC").
		Find(&data.Captchas).Error; err != nil {
		logrus.Error(err)
	}
	return c.Render(http.StatusOK, "admin.captcha", data)
}

// AdminDeleteUserHandler ...
func AdminDeleteUserHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	userID, err := dutils.ParseUserID(c.Param("userID"))
	if err != nil {
		return c.Render(http.StatusOK, "flash",
			FlashResponse{"user id not found", "/admin/users", "alert-danger"})
	}
	if userID == config.RootAdminID {
		return c.Render(http.StatusOK, "flash",
			FlashResponse{"Root admin cannot be deleted", "/admin/users", "alert-danger"})
	}

	if err := db.DeleteUserByID(userID); err != nil {
		logrus.Error(err)
	}
	return hutils.RedirectReferer(c)
}

func IgnoredDeleteHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	userID := dutils.DoParseUserID(c.Request().PostFormValue("user_id"))
	ignoredUserID := dutils.DoParseUserID(c.Request().PostFormValue("ignored_user_id"))
	db.UnIgnoreUser(userID, ignoredUserID)
	return c.Redirect(http.StatusFound, "/admin/ignored")
}

// AdminDeleteRoomHandler ...
func AdminDeleteRoomHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	id := dutils.DoParseRoomID(c.Param("roomID"))
	db.DeleteChatRoomByID(id)
	return c.Redirect(http.StatusFound, "/admin/rooms")
}

func AdminUserSecurityLogsHandler(c echo.Context) error {
	//authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	userID, err := dutils.ParseUserID(c.Param("userID"))
	if err != nil {
		return c.Redirect(http.StatusFound, "/admin/users")
	}
	var data settingsSecurityData
	data.ActiveTab = "security"
	data.Logs, _ = db.GetSecurityLogs(userID)
	return c.Render(http.StatusOK, "admin.user-security-logs", data)
}

// AdminEditUserHandler ...
func AdminEditUserHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	userID, err := dutils.ParseUserID(c.Param("userID"))
	if err != nil {
		return c.Redirect(http.StatusFound, "/admin/users")
	}
	// Only root admin can edit the root admin
	if userID == config.RootAdminID && authUser.ID != config.RootAdminID {
		return c.Redirect(http.StatusFound, "/admin/users")
	}
	user, err := db.GetUserByID(userID)
	if err != nil {
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
	data.CanUseChessAnalyze = user.CanUseChessAnalyze
	data.CanSeeHellbanned = user.CanSeeHellbanned
	data.IsIncognito = user.IsIncognito
	data.CanChangeUsername = user.CanChangeUsername
	data.CanUseUppercase = user.CanUseUppercase
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
	if formName == "reset_login_attempts" {
		user.ResetLoginAttempts(db)
		return c.Redirect(http.StatusFound, "/admin/users/"+userID.String()+"/edit")
	} else if formName == "disable_2fa" {
		user.DisableTotp2FA(db)
		user.DisableGpg2FA(db)
		return c.Redirect(http.StatusFound, "/admin/users/"+userID.String()+"/edit")
	} else if formName == "reset_tutorial" {
		user.ResetTutorial(db)
		return c.Redirect(http.StatusFound, "/admin/users/"+userID.String()+"/edit")
	}

	data.Username = database.Username(c.FormValue("username"))
	data.Role = c.Request().PostFormValue("role")
	data.IsAdmin = utils.DoParseBool(c.FormValue("isAdmin"))
	data.IsHellbanned = utils.DoParseBool(c.FormValue("isHellbanned"))
	data.Verified = utils.DoParseBool(c.FormValue("verified"))
	data.IsClubMember = utils.DoParseBool(c.FormValue("is_club_member"))
	data.CanUploadFile = utils.DoParseBool(c.FormValue("can_upload_file"))
	data.CanUseForum = utils.DoParseBool(c.FormValue("can_use_forum"))
	data.CanUseMultiline = utils.DoParseBool(c.FormValue("can_use_multiline"))
	data.CanUseChessAnalyze = utils.DoParseBool(c.FormValue("can_use_chess_analyze"))
	data.CanSeeHellbanned = utils.DoParseBool(c.FormValue("can_see_hellbanned"))
	data.IsIncognito = utils.DoParseBool(c.FormValue("is_incognito"))
	data.CanChangeUsername = utils.DoParseBool(c.FormValue("can_change_username"))
	data.CanUseUppercase = utils.DoParseBool(c.FormValue("can_use_uppercase"))
	data.CanChangeColor = utils.DoParseBool(c.FormValue("can_change_color"))
	data.Vetted = utils.DoParseBool(c.FormValue("vetted"))
	data.CollectMetadata = utils.DoParseBool(c.FormValue("collect_metadata"))
	data.ChatColor = c.FormValue("chat_color")
	colorRgx := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	if !colorRgx.MatchString(data.ChatColor) {
		data.Errors.Username = "Invalid color format"
	}
	data.ChatFont = utils.DoParseInt64(c.FormValue("chat_font"))
	if data.Username != user.Username {
		if err := db.CanRenameTo(user.Username, data.Username); err != nil {
			data.Errors.Username = err.Error()
		}
	}
	// Edit password
	var hashedPassword string
	data.Password = c.Request().PostFormValue("password")
	data.RePassword = c.Request().PostFormValue("repassword")
	data.ApiKey = c.Request().PostFormValue("api_key")
	if data.Password != "" || data.RePassword != "" {
		hashedPassword, err = database.NewPasswordValidator(db, data.Password).CompareWith(data.RePassword).Hash()
		if err != nil {
			data.Errors.Password = err.Error()
		}
	}
	if data.Errors.HasError() {
		return c.Render(http.StatusOK, "admin.user-edit", data)
	}

	if hashedPassword != "" {
		user.LoginAttempts = 0
		if err := user.ChangePassword(db, hashedPassword); err != nil {
			data.Errors.Password = err.Error()
			return c.Render(http.StatusOK, "admin.user-edit", data)
		}
	}

	user.Username = data.Username
	user.IsAdmin = data.IsAdmin
	if data.IsHellbanned {
		user.HellBan(db)
		managers.ActiveUsers.UpdateUserHBInRooms(managers.NewUserInfo(&user))
	}
	user.ApiKey = data.ApiKey
	user.Verified = data.Verified
	user.IsHellbanned = data.IsHellbanned
	user.IsClubMember = data.IsClubMember
	user.CanUploadFile = data.CanUploadFile
	user.CanUseForum = data.CanUseForum
	user.CanUseMultiline = data.CanUseMultiline
	user.CanUseChessAnalyze = data.CanUseChessAnalyze
	user.CanSeeHellbanned = data.CanSeeHellbanned
	user.IsIncognito = data.IsIncognito
	user.CanChangeUsername = data.CanChangeUsername
	user.CanUseUppercase = data.CanUseUppercase
	user.CanChangeColor = data.CanChangeColor
	user.Vetted = data.Vetted
	user.CollectMetadata = data.CollectMetadata
	user.Role = data.Role
	user.ChatColor = data.ChatColor
	user.ChatFont = data.ChatFont
	user.DoSave(db)

	return c.Redirect(http.StatusFound, "/admin/users/"+userID.String()+"/edit")
}

// AdminEditRoomHandler ...
func AdminEditRoomHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	roomID, err := dutils.ParseRoomID(c.Param("roomID"))
	if err != nil {
		return c.Redirect(http.StatusFound, "/admin/rooms")
	}
	room, err := db.GetChatRoomByID(roomID)
	if err != nil {
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
	room.DoSave(db)

	return c.Redirect(http.StatusFound, "/admin/rooms")
}

func StreamUsersHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	usersIDs := usersStreamsManager.Inst.GetUsers()
	users, _ := db.GetUsersByID(usersIDs)
	out := ""
	for _, user := range users {
		out += string(user.Username) + ", "
	}
	return c.String(http.StatusOK, out)
}

func FiledropDownloadHandler(c echo.Context) error {
	filename := c.Param("filename")
	db := c.Get("database").(*database.DkfDB)
	filedrop, err := db.GetFiledropByFileName(filename)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if !filedrop.Exists() {
		logrus.Error(filename + " does not exists")
		return c.Redirect(http.StatusFound, "/")
	}

	osFile, decrypter, err := filedrop.GetContent()
	if err != nil {
		return echo.NotFoundHandler(c)
	}
	defer osFile.Close()

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", "attachment", filedrop.OrigFileName))

	if _, err := io.Copy(c.Response().Writer, decrypter); err != nil {
		logrus.Error(err)
	}
	c.Response().Flush()
	return nil
}
