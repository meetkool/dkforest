package web

import (
	"context"
	"dkforest/bindata"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/staticbin"
	tmp "dkforest/pkg/template"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/clientFrontends"
	"dkforest/pkg/web/handlers"
	v1 "dkforest/pkg/web/handlers/api/v1"
	"dkforest/pkg/web/middlewares"
	"fmt"
	"github.com/labstack/echo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/store/memory"
	"golang.org/x/text/language"
	yaml "gopkg.in/yaml.v1"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func getMainServer(db *database.DkfDB, i18nBundle *i18n.Bundle, renderer *tmp.Templates, clientFE clientFrontends.ClientFrontend) echo.HandlerFunc {
	e := newEcho()

	e.Server.ReadHeaderTimeout = 10 * time.Second
	e.Server.ReadTimeout = 10 * time.Second
	e.Server.WriteTimeout = 10 * time.Second

	e.Use(staticbin.Static(bindata.Asset, staticbin.Options{Dir: "/public", SkipLogging: true}))
	e.Renderer = renderer
	e.Use(middlewares.SetDatabaseMiddleware(db))
	e.Use(middlewares.SetClientFEMiddleware(clientFE))
	e.Use(middlewares.FirstUseMiddleware)
	e.Use(middlewares.DdosMiddleware)
	e.Use(middlewares.MaintenanceMiddleware)
	e.Use(middlewares.SecureMiddleware)
	e.Use(middlewares.GzipMiddleware)
	e.Use(middlewares.CSRFMiddleware())
	e.Use(middlewares.SetUserMiddleware)
	e.Use(middlewares.I18nMiddleware(i18nBundle, "en"))
	e.Use(middlewares.BodyLimit)
	e.Use(middlewares.HellbannedCookieMiddleware)
	e.Use(middlewares.AprilFoolMiddleware())
	e.GET("/", handlers.HomeHandler, middlewares.CircuitRateLimitMiddleware(15*time.Second, 4, true))
	e.POST("/", handlers.HomeHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 4, false))
	e.GET("/bhcli", handlers.BhcliHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	e.GET("/torchess", handlers.TorchessHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	e.GET("/captcha-help", handlers.CaptchaHelpHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	e.GET("/pow-help", handlers.PowHelpHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	e.GET("/werewolf", handlers.WerewolfHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	e.GET("/gists/:gistUUID", handlers.GistHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	e.POST("/gists/:gistUUID", handlers.GistHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 3, false))
	e.GET("/chat/:roomName", handlers.ChatHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 4, false))
	e.POST("/chat/:roomName", handlers.ChatHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 2, false))
	e.GET("/bhc", handlers.BHCHandler, middlewares.CircuitRateLimitMiddleware(5*time.Second, 4, true))
	e.POST("/bhc", handlers.BHCHandler, middlewares.CircuitRateLimitMiddleware(5*time.Second, 4, true))
	e.GET("/public/css/:signupToken/signup.css", handlers.SignupCss, middlewares.CircuitRateLimitMiddleware(15*time.Second, 4, false))
	e.GET("/public/img/:signupToken/:signal/:data", handlers.SignalCss, middlewares.CircuitRateLimitMiddleware(15*time.Second, 4, false))
	noAuthGroup := e.Group("", middlewares.NoAuthMiddleware)
	noAuthGroup.GET("/login", handlers.LoginHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 4, false))
	noAuthGroup.POST("/login", handlers.LoginHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 4, false))
	noAuthGroup.GET("/login/:loginToken", handlers.LoginAttackHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 4, false))
	noAuthGroup.POST("/login/:loginToken", handlers.LoginAttackHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 2, false))
	noAuthGroup.GET("/signup", handlers.SignupHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	noAuthGroup.POST("/signup", handlers.SignupHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 4, false))
	noAuthGroup.GET("/signup/invitation", handlers.SignupInvitationHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	noAuthGroup.GET("/signup/invitation/:invitationToken", handlers.SignupInvitationHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	noAuthGroup.POST("/signup/invitation/:invitationToken", handlers.SignupInvitationHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	noAuthGroup.GET("/signup/:signupToken", handlers.SignupAttackHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 5, false))
	noAuthGroup.POST("/signup/:signupToken", handlers.SignupAttackHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 2, false))
	noAuthGroup.GET("/forgot-password", handlers.ForgotPasswordHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 4, false))
	noAuthGroup.POST("/forgot-password", handlers.ForgotPasswordHandler, middlewares.CircuitRateLimitMiddleware(1*time.Second, 2, false))
	maybeAuthGroup := e.Group("", middlewares.MaybeAuthMiddleware)
	maybeAuthGroup.GET("/u/:username", handlers.PublicUserProfileHandler, middlewares.GenericRateLimitMiddleware(time.Second, 2))
	maybeAuthGroup.GET("/u/:username/pgp", handlers.PublicUserProfilePGPHandler, middlewares.GenericRateLimitMiddleware(time.Second, 2))
	maybeAuthGroup.GET("/t/:threadUUID", handlers.ThreadHandler, middlewares.GenericRateLimitMiddleware(time.Second, 2))
	authGroup := e.Group("", middlewares.IsAuthMiddleware, middlewares.ForceCaptchaMiddleware)
	authGroup.GET("/public/css/meta.css", handlers.MetaCss)
	authGroup.GET("/public/img/signal/:signal/:data", handlers.SignalCss1)
	authGroup.GET("/captcha-required", handlers.CaptchaRequiredHandler, middlewares.AuthRateLimitMiddleware(time.Second, 1))
	authGroup.POST("/captcha-required", handlers.CaptchaRequiredHandler, middlewares.AuthRateLimitMiddleware(time.Second, 1))
	authGroup.GET("/odometer", handlers.OdometerHandler, middlewares.AuthRateLimitMiddleware(time.Second, 1))
	authGroup.GET("/captcha", handlers.CaptchaHandler, middlewares.AuthRateLimitMiddleware(time.Second, 1))
	authGroup.POST("/captcha", handlers.CaptchaHandler, middlewares.AuthRateLimitMiddleware(time.Second, 1))
	authGroup.GET("/donate", handlers.DonateHandler)
	authGroup.GET("/shop", handlers.ShopHandler)
	authGroup.GET("/poker", handlers.PokerHomeHandler)
	authGroup.POST("/poker", handlers.PokerHomeHandler, middlewares.AuthRateLimitMiddleware(time.Second, 1))
	authGroup.GET("/poker/rake-back", handlers.PokerRakeBackHandler)
	authGroup.POST("/poker/rake-back", handlers.PokerRakeBackHandler, middlewares.AuthRateLimitMiddleware(time.Second, 1))
	authGroup.GET("/poker/:roomID", handlers.PokerTableHandler)
	authGroup.GET("/poker/:roomID/stream", handlers.PokerStreamHandler)
	authGroup.GET("/poker/:roomID/logs", handlers.PokerLogsHandler)
	authGroup.GET("/poker/:roomID/bet", handlers.PokerBetHandler)
	authGroup.POST("/poker/:roomID/bet", handlers.PokerBetHandler)
	authGroup.GET("/poker/:roomID/deal", handlers.PokerDealHandler)
	authGroup.POST("/poker/:roomID/deal", handlers.PokerDealHandler)
	authGroup.GET("/poker/:roomID/unsit", handlers.PokerUnSitHandler)
	authGroup.POST("/poker/:roomID/unsit", handlers.PokerUnSitHandler)
	authGroup.GET("/poker/:roomID/sit/:pos", handlers.PokerSitHandler)
	authGroup.POST("/poker/:roomID/sit/:pos", handlers.PokerSitHandler)
	authGroup.GET("/chess", handlers.ChessHandler)
	authGroup.POST("/chess", handlers.ChessHandler)
	authGroup.GET("/chess/analyze", handlers.ChessAnalyzeHandler)
	authGroup.POST("/chess/analyze", handlers.ChessAnalyzeHandler)
	authGroup.GET("/chess/:key", handlers.ChessGameHandler)
	authGroup.POST("/chess/:key", handlers.ChessGameHandler)
	authGroup.GET("/chess/:key/analyze", handlers.ChessGameAnalyzeHandler)
	authGroup.POST("/chess/:key/analyze", handlers.ChessGameAnalyzeHandler)
	authGroup.GET("/chess/:key/form", handlers.ChessGameFormHandler)
	authGroup.POST("/chess/:key/form", handlers.ChessGameFormHandler)
	authGroup.GET("/chess/:key/stats", handlers.ChessGameStatsHandler)
	authGroup.POST("/chess/:key/stats", handlers.ChessGameStatsHandler)
	authGroup.GET("/settings/chat", handlers.SettingsChatHandler)
	authGroup.POST("/settings/chat", handlers.SettingsChatHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 1))
	authGroup.GET("/settings/chat/pm", handlers.SettingsChatPMHandler)
	authGroup.POST("/settings/chat/pm", handlers.SettingsChatPMHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 5))
	authGroup.GET("/settings/chat/ignore", handlers.SettingsChatIgnoreHandler)
	authGroup.POST("/settings/chat/ignore", handlers.SettingsChatIgnoreHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 5))
	authGroup.GET("/settings/chat/snippets", handlers.SettingsChatSnippetsHandler)
	authGroup.POST("/settings/chat/snippets", handlers.SettingsChatSnippetsHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 5))
	authGroup.GET("/settings/public-notes", handlers.SettingsPublicNotesHandler)
	authGroup.POST("/settings/public-notes", handlers.SettingsPublicNotesHandler)
	authGroup.GET("/settings/private-notes", handlers.SettingsPrivateNotesHandler)
	authGroup.POST("/settings/private-notes", handlers.SettingsPrivateNotesHandler)
	authGroup.GET("/settings/sessions", handlers.SettingsSessionsHandler)
	authGroup.POST("/settings/sessions", handlers.SettingsSessionsHandler)
	authGroup.GET("/settings/api", handlers.SettingsAPIHandler)
	authGroup.POST("/settings/api", handlers.SettingsAPIHandler)
	authGroup.GET("/settings/security", handlers.SettingsSecurityHandler)
	authGroup.GET("/settings/account", handlers.SettingsAccountHandler)
	authGroup.POST("/settings/account", handlers.SettingsAccountHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 1))
	authGroup.GET("/settings/password", handlers.SettingsPasswordHandler)
	authGroup.POST("/settings/password", handlers.SettingsPasswordHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 1))
	authGroup.GET("/settings/uploads", handlers.SettingsUploadsHandler)
	authGroup.POST("/settings/uploads", handlers.SettingsUploadsHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 5))
	authGroup.GET("/settings/inbox", handlers.SettingsInboxHandler)
	authGroup.POST("/settings/inbox", handlers.SettingsInboxHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 1))
	authGroup.GET("/settings/inbox/sent", handlers.SettingsInboxSentHandler)
	authGroup.POST("/settings/inbox/sent", handlers.SettingsInboxSentHandler, middlewares.AuthRateLimitMiddleware(2*time.Second, 1))
	authGroup.GET("/settings/pgp/add", handlers.AddPGPHandler)
	authGroup.POST("/settings/pgp/add", handlers.AddPGPHandler)
	authGroup.GET("/settings/pgp", handlers.SettingsPGPHandler)
	authGroup.GET("/settings/age", handlers.SettingsAgeHandler)
	authGroup.GET("/settings/age/add", handlers.AddAgeHandler)
	authGroup.POST("/settings/age/add", handlers.AddAgeHandler)
	authGroup.GET("/gpg-two-factor-authentication/toggle", handlers.GpgTwoFactorAuthenticationToggleHandler)
	authGroup.POST("/gpg-two-factor-authentication/toggle", handlers.GpgTwoFactorAuthenticationToggleHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 4))
	authGroup.GET("/two-factor-authentication/verify", handlers.TwoFactorAuthenticationVerifyHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 4))
	authGroup.POST("/two-factor-authentication/verify", handlers.TwoFactorAuthenticationVerifyHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/two-factor-authentication/disable", handlers.TwoFactorAuthenticationDisableHandler)
	authGroup.POST("/two-factor-authentication/disable", handlers.TwoFactorAuthenticationDisableHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/api/v1/captcha-svc", v1.GetCaptchaHandler)
	authGroup.POST("/api/v1/chat/:roomName/notifier", v1.RoomNotifierHandler)
	authGroup.POST("/api/v1/battleship", v1.BattleshipHandler)
	authGroup.POST("/api/v1/werewolf", v1.WerewolfHandler)
	authGroup.POST("/api/v1/captcha/solver", v1.CaptchaSolverHandler)
	authGroup.GET("/api/v1/chat/controls/:roomName/:isStream", v1.ChatControlsHandler)
	authGroup.POST("/api/v1/chat/controls/:roomName/:isStream", v1.ChatControlsHandler)
	authGroup.GET("/api/v1/chat/top-bar/:roomName", v1.ChatTopBarHandler)
	authGroup.POST("/api/v1/chat/top-bar/:roomName", v1.ChatTopBarHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 3))
	authGroup.GET("/api/v1/chat/messages/:roomName", v1.ChatMessagesHandler)
	authGroup.GET("/api/v1/chat/messages/:roomName/refresh", v1.ChatStreamMessagesRefreshHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 4))
	authGroup.GET("/api/v1/chat/messages/:roomName/stream", v1.ChatStreamMessagesHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 4))
	authGroup.GET("/api/v1/chat/messages/:roomName/stream/menu", v1.ChatStreamMenuHandler)
	authGroup.POST("/api/v1/notifications/delete/:notificationID", v1.DeleteNotificationHandler)
	authGroup.POST("/api/v1/session-notifications/delete/:sessionNotificationID", v1.DeleteSessionNotificationHandler)
	authGroup.POST("/api/v1/inbox/delete/:messageID", v1.ChatInboxDeleteMessageHandler)
	authGroup.POST("/api/v1/inbox/delete-all", v1.ChatInboxDeleteAllMessageHandler)
	authGroup.GET("/api/v1/chat/messages/delete/:messageUUID", v1.ChatDeleteMessageHandler)
	authGroup.POST("/api/v1/chat/messages/delete/:messageUUID", v1.ChatDeleteMessageHandler)
	authGroup.POST("/api/v1/chat/messages/reactions", v1.ChatMessageReactionHandler)
	authGroup.POST("/api/v1/rooms/:roomName/subscribe", v1.SubscribeHandler)
	authGroup.POST("/api/v1/rooms/:roomName/unsubscribe", v1.UnsubscribeHandler)
	authGroup.POST("/api/v1/threads/:threadUUID/subscribe", v1.ThreadSubscribeHandler)
	authGroup.POST("/api/v1/threads/:threadUUID/unsubscribe", v1.ThreadUnsubscribeHandler)
	authGroup.POST("/logout", handlers.LogoutHandler)
	authGroup.GET("/uploads/:filename", handlers.UploadsDownloadHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.POST("/uploads/:filename", handlers.UploadsDownloadHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/torchess/downloads", handlers.TorchessDownloadsHandler)
	authGroup.GET("/torchess/downloads/:filename", handlers.TorChessDownloadFileHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2), middlewares.CaptchaMiddleware())
	authGroup.POST("/torchess/downloads/:filename", handlers.TorChessDownloadFileHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2), middlewares.CaptchaMiddleware())
	authGroup.GET("/bhcli/downloads", handlers.BhcliDownloadsHandler)
	authGroup.GET("/bhcli/downloads/:filename", handlers.BhcliDownloadFileHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2), middlewares.CaptchaMiddleware())
	authGroup.POST("/bhcli/downloads/:filename", handlers.BhcliDownloadFileHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2), middlewares.CaptchaMiddleware())
	authGroup.GET("/memes/:slug", handlers.MemeHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/news", handlers.NewsHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links", handlers.LinksHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links/download", handlers.LinksDownloadHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.POST("/links/download", handlers.LinksDownloadHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/l/:shorthand", handlers.LinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links/claim-instructions", handlers.LinksClaimInstructionsHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links/:linkUUID", handlers.LinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.POST("/links/:linkUUID/restore", handlers.RestoreLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links/:linkUUID/claim", handlers.ClaimLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.POST("/links/:linkUUID/claim", handlers.ClaimLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.POST("/links/:linkUUID/claim/download-certificate", handlers.ClaimDownloadCertificateLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links/:linkUUID/claim-certificate", handlers.ClaimCertificateLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links/:linkUUID/edit", handlers.EditLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.POST("/links/:linkUUID/edit", handlers.EditLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/links/:linkUUID/delete", handlers.LinkDeleteHandler)
	authGroup.POST("/links/:linkUUID/delete", handlers.LinkDeleteHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.POST("/api/v1/pgp/:linkPgpID/download", handlers.LinkPgpDownloadHandler)
	authGroup.GET("/links/pgp/:linkPgpID/delete", handlers.LinkPgpDeleteHandler)
	authGroup.POST("/links/pgp/:linkPgpID/delete", handlers.LinkPgpDeleteHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/links/mirrors/:linkMirrorID/delete", handlers.LinkMirrorDeleteHandler)
	authGroup.POST("/links/mirrors/:linkMirrorID/delete", handlers.LinkMirrorDeleteHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/links/upload", handlers.LinksUploadHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.POST("/links/upload", handlers.LinksUploadHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/new-link", handlers.NewLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.POST("/new-link", handlers.NewLinkHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/forum", handlers.ForumHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/forum/c/:categorySlug", handlers.ForumCategoryHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/forum/search", handlers.ForumSearchHandler, middlewares.AuthRateLimitMiddleware(time.Second, 2))
	authGroup.GET("/t/:threadUUID/edit", handlers.ThreadEditHandler)
	authGroup.POST("/t/:threadUUID/edit", handlers.ThreadEditHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/t/:threadUUID/delete", handlers.ThreadDeleteHandler)
	authGroup.POST("/t/:threadUUID/delete", handlers.ThreadDeleteHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/t/:threadUUID/reply", handlers.ThreadReplyHandler)
	authGroup.POST("/t/:threadUUID/reply", handlers.ThreadReplyHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/t/:threadUUID/messages/:messageUUID/raw", handlers.ThreadRawMessageHandler)
	authGroup.GET("/t/:threadUUID/messages/:messageUUID/edit", handlers.ThreadEditMessageHandler)
	authGroup.POST("/t/:threadUUID/messages/:messageUUID/edit", handlers.ThreadEditMessageHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/t/:threadUUID/messages/:messageUUID/delete", handlers.ThreadDeleteMessageHandler)
	authGroup.POST("/t/:threadUUID/messages/:messageUUID/delete", handlers.ThreadDeleteMessageHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/new-thread", handlers.NewThreadHandler)
	authGroup.POST("/new-thread", handlers.NewThreadHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/red-room", handlers.RedRoomHandler)
	authGroup.GET("/rooms", handlers.RoomsHandler)
	authGroup.GET("/chat", handlers.ChatHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 4))
	authGroup.POST("/chat", handlers.ChatHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/chat/help", handlers.ChatHelpHandler)
	authGroup.GET("/chat-code/:messageUUID/:idx", handlers.ChatCodeHandler)
	authGroup.GET("/chat/create-room", handlers.ChatCreateRoomHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.POST("/chat/create-room", handlers.ChatCreateRoomHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	authGroup.GET("/chat/:roomName/stream", handlers.ChatStreamHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 4))
	authGroup.GET("/chat/:roomName/archive", handlers.ChatArchiveHandler)
	authGroup.GET("/chat/:roomName/delete", handlers.ChatDeleteHandler)
	authGroup.POST("/chat/:roomName/delete", handlers.ChatDeleteHandler)
	authGroup.GET("/chat/:roomName/settings", handlers.RoomChatSettingsHandler)
	authGroup.POST("/chat/:roomName/settings", handlers.RoomChatSettingsHandler)
	authGroup.GET("/external-link/:original", handlers.ExternalLink1Handler)
	authGroup.GET("/external-link/:service/:original", handlers.ExternalLinkHandler)
	moderatorGroup := e.Group("", middlewares.IsModeratorMiddleware)
	moderatorGroup.POST("/api/v1/users/:userID/hellban", v1.UserHellbanHandler)
	moderatorGroup.POST("/api/v1/users/:userID/unhellban", v1.UserUnHellbanHandler)
	moderatorGroup.POST("/api/v1/users/:userID/kick", v1.KickHandler)
	moderatorGroup.POST("/links/reindex", handlers.LinksReindexHandler)
	moderatorGroup.GET("/forum/reindex", handlers.ForumReindexHandler)
	moderatorGroup.GET("/settings/website", handlers.SettingsWebsiteHandler)
	moderatorGroup.POST("/settings/website", handlers.SettingsWebsiteHandler)
	moderatorGroup.GET("/settings/invitations", handlers.SettingsInvitationsHandler)
	moderatorGroup.POST("/settings/invitations", handlers.SettingsInvitationsHandler)
	adminGroup := e.Group("", middlewares.IsAdminMiddleware)
	adminGroup.GET("/debug/*", echo.WrapHandler(http.DefaultServeMux))
	adminGroup.GET("/admin", handlers.AdminHandler)
	adminGroup.POST("/admin", handlers.AdminHandler)
	adminGroup.GET("/admin/ignored", handlers.IgnoredHandler)
	adminGroup.POST("/admin/ignored/delete", handlers.IgnoredDeleteHandler)
	adminGroup.GET("/admin/sessions", handlers.SessionsHandler)
	adminGroup.GET("/admin/backup", handlers.BackupHandler)
	adminGroup.POST("/admin/backup", handlers.BackupHandler)
	adminGroup.GET("/admin/poker-transactions", handlers.AdminPokerTransactionsHandler)
	adminGroup.GET("/admin/spam-filters", handlers.AdminSpamFiltersHandler)
	adminGroup.POST("/admin/spam-filters", handlers.AdminSpamFiltersHandler)
	adminGroup.GET("/admin/ddos", handlers.DdosHandler)
	adminGroup.POST("/admin/ddos", handlers.DdosHandler)
	adminGroup.GET("/admin/audits", handlers.AdminAuditsHandler)
	adminGroup.POST("/admin/users/:userID/delete", handlers.AdminDeleteUserHandler)
	adminGroup.GET("/admin/users/:userID/security-logs", handlers.AdminUserSecurityLogsHandler)
	adminGroup.GET("/admin/users/:userID/edit", handlers.AdminEditUserHandler)
	adminGroup.POST("/admin/users/:userID/edit", handlers.AdminEditUserHandler)
	adminGroup.GET("/admin/captcha", handlers.AdminCaptchaHandler)
	adminGroup.GET("/admin/rooms", handlers.AdminRoomsHandler)
	adminGroup.GET("/admin/rooms/:roomID/edit", handlers.AdminEditRoomHandler)
	adminGroup.POST("/admin/rooms/:roomID/edit", handlers.AdminEditRoomHandler)
	adminGroup.POST("/admin/rooms/:roomID/delete", handlers.AdminDeleteRoomHandler)
	adminGroup.GET("/admin/settings", handlers.AdminSettingsHandler)
	adminGroup.POST("/admin/settings", handlers.AdminSettingsHandler)
	adminGroup.GET("/admin/uploads", handlers.AdminUploadsHandler)
	adminGroup.POST("/admin/uploads", handlers.AdminUploadsHandler)
	adminGroup.GET("/admin/filedrops", handlers.AdminFiledropsHandler)
	adminGroup.POST("/admin/filedrops", handlers.AdminFiledropsHandler)
	adminGroup.GET("/admin/file-drop/:filename", handlers.FiledropDownloadHandler)
	adminGroup.GET("/admin/downloads", handlers.AdminDownloadsHandler)
	adminGroup.POST("/admin/downloads/:downloadID/delete", handlers.AdminDeleteDownloadHandler)
	adminGroup.GET("/admin/gists", handlers.AdminGistsHandler)
	adminGroup.GET("/admin/gists/new", handlers.AdminNewGistHandler)
	adminGroup.POST("/admin/gists/new", handlers.AdminNewGistHandler)
	adminGroup.GET("/admin/gists/:gistUUID/edit", handlers.AdminEditGistHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	adminGroup.POST("/admin/gists/:gistUUID/edit", handlers.AdminEditGistHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	adminGroup.GET("/admin/stream-users", handlers.StreamUsersHandler)
	clubGroup := authGroup.Group("", middlewares.ClubMiddleware)
	clubGroup.GET("/club", handlers.ClubHandler)
	clubGroup.GET("/club/threads/:threadID", handlers.ClubThreadHandler)
	clubGroup.GET("/club/threads/:threadID/reply", handlers.ClubThreadReplyHandler)
	clubGroup.POST("/club/threads/:threadID/reply", handlers.ClubThreadReplyHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	clubGroup.GET("/club/threads/:threadID/messages/:messageID/edit", handlers.ClubThreadEditMessageHandler)
	clubGroup.POST("/club/threads/:threadID/messages/:messageID/edit", handlers.ClubThreadEditMessageHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	clubGroup.POST("/api/v1/club/messages/:messageID/delete", v1.ClubDeleteMessageHandler)
	clubGroup.GET("/club/new-thread", handlers.ClubNewThreadHandler)
	clubGroup.POST("/club/new-thread", handlers.ClubNewThreadHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2))
	clubGroup.GET("/club/members", handlers.ClubMembersHandler)
	vipGroup := authGroup.Group("", middlewares.VipMiddleware)
	vipGroup.GET("/vip", handlers.VipHandler)
	vipGroup.GET("/vip/challenges/stego1", handlers.Stego1ChallengeHandler)
	vipGroup.POST("/vip/challenges/stego1", handlers.Stego1ChallengeHandler)
	vipGroup.GET("/vip/challenges/forgot-password-bypass", handlers.ForgotPasswordBypassChallengeHandler)
	vipGroup.GET("/vip/challenges/byte-road", handlers.ByteRoadChallengeHandler, middlewares.AuthRateLimitMiddleware(1*time.Minute, 500))
	vipGroup.POST("/vip/challenges/byte-road", handlers.ByteRoadChallengeHandler, middlewares.AuthRateLimitMiddleware(1*time.Minute, 500))
	vipGroup.GET("/vip/challenges/re-1", handlers.VipDownloadsHandler)
	vipGroup.POST("/vip/challenges/re-1", handlers.VipDownloadsHandler)
	vipGroup.GET("/vip/challenges/re-1/:filename", handlers.VipDownloadFileHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2), middlewares.CaptchaMiddleware())
	vipGroup.POST("/vip/challenges/re-1/:filename", handlers.VipDownloadFileHandler, middlewares.AuthRateLimitMiddleware(1*time.Second, 2), middlewares.CaptchaMiddleware())
	vipGroup.GET("/vip/projects", handlers.VipProjectsHandler)
	vipGroup.GET("/vip/projects/ip-grabber", handlers.VipProjectsIPGrabberHandler)
	vipGroup.GET("/vip/projects/malware-dropper", handlers.VipProjectsMalwareDropperHandler)
	vipGroup.GET("/vip/projects/rust-ransomware", handlers.VipProjectsRustRansomwareHandler)

	return func(c echo.Context) error {
		e.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

func getBaseServer(db *database.DkfDB, clientFE clientFrontends.ClientFrontend) *echo.Echo {
	e := newEcho()
	renderer := tmp.GetRenderer(e)
	i18nBundle := getI18nBundle()
	e.Renderer = renderer
	e.Use(middlewares.SetUselessHeadersMiddleware)
	e.GET("/file-drop/:uuid", handlers.FileDropHandler, middlewares.SetDatabaseMiddleware(db), middlewares.I18nMiddleware(i18nBundle, "en"))
	e.POST("/file-drop/:uuid", handlers.FileDropHandler, middlewares.SetDatabaseMiddleware(db), middlewares.I18nMiddleware(i18nBundle, "en"))
	e.POST("/file-drop/:uuid/dkfupload", handlers.FileDropDkfUploadHandler, middlewares.SetDatabaseMiddleware(db), middlewares.I18nMiddleware(i18nBundle, "en"))
	e.POST("/api/v1/file-drop/:uuid/dkfdownload", handlers.FileDropDkfDownloadHandler, middlewares.SetDatabaseMiddleware(db), middlewares.I18nMiddleware(i18nBundle, "en"), middlewares.SetUserMiddleware, middlewares.IsAuthMiddleware)
	e.GET("/downloads/:fileName", handlers.FileDropDownloadHandler, middlewares.SetDatabaseMiddleware(db), middlewares.I18nMiddleware(i18nBundle, "en"), middlewares.SetUserMiddleware)
	e.POST("/downloads/:fileName", handlers.FileDropDownloadHandler, middlewares.SetDatabaseMiddleware(db), middlewares.I18nMiddleware(i18nBundle, "en"), middlewares.SetUserMiddleware)
	e.Any("*", getMainServer(db, i18nBundle, renderer, clientFE))
	return e
}

func getSubdomainServer(db *database.DkfDB, clientFE clientFrontends.ClientFrontend) *echo.Echo {
	rp := getReverseProxy(config.GogsURL)
	be := getBaseServer(db, clientFE)
	e := newEcho()
	e.Any("*", func(c echo.Context) error {
		res := c.Response()
		req := c.Request()
		host := req.Host
		hostParts := strings.SplitN(host, ".", 2)
		if hostParts[0] == "git" {
			rp.ServeHTTP(res, req)
			return nil
		}
		be.ServeHTTP(res, req)
		return nil
	})
	return e
}

func getI2pServer(db *database.DkfDB) *echo.Echo {
	if config.Development.IsTrue() {
		return nil
	}
	return getSubdomainServer(db, clientFrontends.I2PClientFE)
}

func getTorServer(db *database.DkfDB) *echo.Echo {
	e := getSubdomainServer(db, clientFrontends.TorClientFE)
	configTorProdServer(e)
	return e
}

// Start ...
func Start(db *database.DkfDB, host string, port int) {
	// Server for Tor/dev
	e1 := getTorServer(db)
	// Start server for I2P
	e2 := getI2pServer(db)

	serverError1 := make(chan struct{})
	serverError2 := make(chan struct{})

	utils.SGo(func() {
		address := host + ":" + strconv.Itoa(port)
		logrus.Info("start tor server on " + address)
		if err := e1.Start(address); err != nil {
			if err != http.ErrServerClosed {
				logrus.Error(err)
			}
			close(serverError1)
		}
	})

	utils.SGo(func() {
		if e2 != nil {
			address := host + ":" + strconv.Itoa(port+1)
			logrus.Info("start i2p server on " + address)
			if err := e2.Start(address); err != nil {
				if err != http.ErrServerClosed {
					logrus.Error(err)
				}
				close(serverError2)
			}
		}
	})

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 10 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	select {
	case <-quit:
	case <-serverError1:
	case <-serverError2:
	}

	logrus.Info("graceful shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e1.Shutdown(ctx); err != nil {
		logrus.Errorf("tor graceful shutdown failed: %s", err.Error())
	}

	if e2 != nil {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e2.Shutdown(ctx); err != nil {
			logrus.Errorf("i2p graceful shutdown failed: %s", err.Error())
		}
	}

	logrus.Info("Bye!")
}

func extractGlobalCircuitIdentifier(m string) int64 {
	// You can compute the global circuit identifier using the following formula given the IPv6 address "fc00:dead:beef:4dad::AABB:CCDD":
	// global_circuit_id = (0xAA << 24) + (0xBB << 16) + (0xCC << 8) + 0xDD;
	s1 := strings.Split(m, "::")[1]
	s2 := strings.Split(s1, ":")
	aabb := fmt.Sprintf("%04s", s2[0])
	ccdd := fmt.Sprintf("%04s", s2[1])
	aa, _ := strconv.ParseInt(aabb[0:2], 16, 64)
	bb, _ := strconv.ParseInt(aabb[2:4], 16, 64)
	cc, _ := strconv.ParseInt(ccdd[0:2], 16, 64)
	dd, _ := strconv.ParseInt(ccdd[2:4], 16, 64)
	globalCircuitID := (aa << 24) + (bb << 16) + (cc << 8) + dd
	return globalCircuitID
}

func getReverseProxy(u string) *httputil.ReverseProxy {
	remote, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(remote)
	reverseProxy.FlushInterval = 1000 * time.Millisecond
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, e error) {
		if e.Error() != "context canceled" {
			logrus.Error(e.Error())
		}
		w.WriteHeader(http.StatusBadGateway)
	}
	return reverseProxy
}

func newEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Debug = true
	return e
}

func configTorProdServer(e *echo.Echo) {
	if config.Development.IsTrue() {
		return
	}
	rate := limiter.Rate{Period: 5 * time.Second, Limit: 25}
	store := memory.NewStore()
	limiterInstance := limiter.New(store, rate)

	var haproxyRgx = regexp.MustCompile(`PROXY TCP6 (\S+)`)
	e.Server.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateNew {
			buf := make([]byte, 1024)
			_, err := conn.Read(buf)
			if err != nil {
				return
			}
			m := haproxyRgx.FindStringSubmatch(string(buf))
			if len(m) == 2 {
				globalCircuitID := extractGlobalCircuitIdentifier(m[1])
				config.ConnMap.Set(conn, globalCircuitID)

				limiterCtx, _ := limiterInstance.Get(context.Background(), utils.FormatInt64(globalCircuitID))
				if limiterCtx.Reached {
					config.ConnMap.CloseCircuit(globalCircuitID)
				}
			}
		} else if state == http.StateClosed {
			config.ConnMap.Delete(conn)
		}
	}
	e.Server.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
		return context.WithValue(ctx, "conn", c)
	}

	// Open a tcp connection to each of tor process & authenticate
	servers := []string{"127.0.0.1:6668"}
	conns := make([]net.Conn, 0)
	for _, server := range servers {
		conn1, err := net.Dial("tcp", server)
		if err != nil {
			logrus.Errorf("failed to connect to tor port %s : %v", server, err)
		}
		_, _ = conn1.Write([]byte("AUTHENTICATE \"\"\n"))
		buf := make([]byte, 1024)
		n, _ := conn1.Read(buf)
		fmt.Println("AUTHENTICATE", strings.TrimSpace(string(buf[0:n])))
		conns = append(conns, conn1)
	}
	// Listen for circuit to close
	go func() {
		for circuitID := range config.ConnMap.CircuitIDCh {
			res := ""
			for _, conn := range conns {
				_, _ = fmt.Fprintf(conn, "CLOSECIRCUIT %d\n", circuitID)
				buf1 := make([]byte, 1024)
				n1, _ := conn.Read(buf1)
				res += " : " + strings.TrimSpace(string(buf1[0:n1]))
			}
			logrus.Warnf("CLOSECIRCUIT %d -> %s", circuitID, res)
		}
	}()
}

func getI18nBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	dir, _ := config.LocalsFs.ReadDir(".")
	fileNames := make([]string, 0)
	for _, d := range dir {
		fileNames = append(fileNames, d.Name())
	}
	for _, fileName := range fileNames {
		if strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, "sample.yaml") {
			if _, err := bundle.ParseMessageFileBytes(utils.Must(config.LocalsFs.ReadFile(fileName)), fileName); err != nil {
				logrus.Errorf("failed to parse %s : %s", fileName, err.Error())
			}
		}
	}

	if err := utils.LoadLocals(bundle); err != nil {
		logrus.Fatal(err)
	}
	return bundle
}
