package actions

import (
	"bytes"
	"dkforest/pkg/web/handlers/api/v1"
	"fmt"
	wallet1 "github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	captcha "dkforest/pkg/captcha"
	"dkforest/pkg/color"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"dkforest/pkg/web"
	"github.com/mattn/go-colorable"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
	cli "github.com/urfave/cli/v2"
)

func Start(c *cli.Context) error {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Ltime | log.Lshortfile)
	logrus.SetFormatter(LogFormatter{})
	logrus.SetOutput(colorable.NewColorableStderr())

	logrus.Info("DarkForest v" + config.Global.GetVersion().String())

	port := c.Int("port")
	host := c.String("host")
	noBrowser := c.Bool("no-browser")

	config.Global.SetCookieSecure(c.Bool("cookie-secure"))
	config.Global.SetCookieDomain(c.String("cookie-domain"))

	ensureProjectHome()

	initDB()
	defer database.DB.Close()

	runMigrations()

	config.IsFirstUse.Store(isFirstUse())

	captcha.SetCustomStore(captcha.NewMemoryStore(captcha.CollectNum, 120*time.Second))

	if false {
		err := database.DB.Debug().Exec(`
	`).Error
		logrus.Error(err)
	}

	settings := database.GetSettings()
	config.ProtectHome.Store(settings.ProtectHome)
	config.HomeUsersList.Store(settings.HomeUsersList)
	config.ForceLoginCaptcha.Store(settings.ForceLoginCaptcha)
	config.SignupEnabled.Store(settings.SignupEnabled)
	config.SignupFakeEnabled.Store(settings.SignupFakeEnabled)
	config.DownloadsEnabled.Store(settings.DownloadsEnabled)
	config.ForumEnabled.Store(settings.ForumEnabled)
	config.SilentSelfKick.Store(settings.SilentSelfKick)
	config.MaybeAuthEnabled.Store(settings.MaybeAuthEnabled)
	config.CaptchaDifficulty.Store(settings.CaptchaDifficulty)

	config.Xmr()

	utils.SGo(func() { cleanupDatabase() })
	utils.SGo(func() { managers.ActiveUsers.CleanupUsersCache() })
	utils.SGo(func() { xmrWatch() })
	utils.SGo(func() { openBrowser(noBrowser, int64(port)) })

	v1.ChessInstance = v1.NewChess()
	v1.BattleshipInstance = v1.NewBattleship()
	v1.WWInstance = v1.NewWerewolf()

	web.Start(host, port)
	return nil
}

func isFirstUse() bool {
	var count int64
	database.DB.Model(database.User{}).Count(&count)
	return count <= 0
}

func openBrowser(noBrowser bool, port int64) {
	if noBrowser {
		return
	}
	time.Sleep(1000 * time.Millisecond)
	if err := open.Run(fmt.Sprintf("http://127.0.0.1:%d", port)); err != nil {
		logrus.Error("failed to open browser : " + err.Error())
	}
}

func runMigrations() {
	logrus.Info("running migrations")
	migrations := &migrate.AssetMigrationSource{
		Asset: config.MigrationsFs.ReadFile,
		AssetDir: func(path string) ([]string, error) {
			dir, err := config.MigrationsFs.ReadDir(path)
			out := make([]string, 0)
			for _, d := range dir {
				out = append(out, d.Name())
			}
			return out, err
		},
		Dir: "migrations",
	}
	database.DB.Exec("PRAGMA foreign_keys=OFF")
	n, err := migrate.Exec(database.DB.DB(), "sqlite3", migrations, migrate.Up)
	if err != nil {
		panic(err)
	}
	database.DB.Exec("PRAGMA foreign_keys=ON")
	logrus.Infof("applied %d migrations", n)
}

func initDB() {
	dbPath := filepath.Join(config.Global.ProjectPath(), config.DbFileName)
	db, err := database.OpenSqlite3DB(dbPath)
	if err != nil {
		logrus.Fatal("Failed to open sqlite3 db : " + err.Error())
		return
	}
	database.DB = db
}

//  Ensure the project folder is created properly
func ensureProjectHome() {
	config.Global.SetProjectPath(utils.MustGetDefaultProjectPath())
	projectPath := config.Global.ProjectPath()
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		logrus.Fatal("Failed to create project folder", err)
	}

	config.Global.SetProjectLocalsPath(filepath.Join(projectPath, "locals"))
	if err := os.MkdirAll(config.Global.ProjectLocalsPath(), 0755); err != nil {
		logrus.Fatal("Failed to create dkforest locals folder", err)
	}

	config.Global.SetProjectHTMLPath(filepath.Join(projectPath, "html"))
	if err := os.MkdirAll(config.Global.ProjectHTMLPath(), 0755); err != nil {
		logrus.Fatal("Failed to create dkforest html folder", err)
	}
}

// LogFormatter ...
type LogFormatter struct{}

// Format ...
func (f LogFormatter) Format(e *logrus.Entry) ([]byte, error) {
	skip := 6
	var fn string
	var line int
	for {
		_, fn, line, _ = runtime.Caller(skip)
		if !strings.Contains(fn, "/logrus/") || skip >= 10 {
			break
		}
		skip++
	}
	var buffer bytes.Buffer
	var level string
	switch e.Level {
	case logrus.DebugLevel:
		level = color.Magenta("DEBU")
	case logrus.InfoLevel:
		level = color.Cyan("INFO")
	case logrus.WarnLevel:
		level = color.Yellow("WARN")
	case logrus.ErrorLevel:
		level = color.Red("ERRO")
	case logrus.FatalLevel:
		level = color.Red("FATA")
	case logrus.PanicLevel:
		level = color.Red("PANI")
	}
	buffer.WriteString(e.Time.Format("06/01/02 15:04:05"))
	buffer.WriteString(" ")
	buffer.WriteString(level)
	buffer.WriteString(" ")
	buffer.WriteString(color.Magenta("[" + strings.TrimSuffix(filepath.Base(fn), ".go") + ":" + strconv.Itoa(line) + "]"))
	buffer.WriteString(" ")
	buffer.WriteString(e.Message)
	buffer.WriteString("\n")
	return buffer.Bytes(), nil
}

func xmrWatch() {
	var once utils.Once
	for {
		select {
		case <-once.After(5 * time.Second):
		case <-time.After(1 * time.Minute):
		}
		transfers, err := config.Xmr().GetTransfers(&wallet1.RequestGetTransfers{In: true})
		if err != nil {
			continue
		}
		for _, transfer := range transfers.In {
			invoice, err := database.GetXmrInvoiceByAddress(transfer.Address)
			if err != nil {
				logrus.Error(err, transfer.TxID)
				continue
			}
			origConfirmations := invoice.Confirmations
			invoice.Confirmations = int64(transfer.Confirmations)
			amount := int64(transfer.Amount)
			invoice.AmountReceived = &amount
			invoice.DoSave()
			if origConfirmations >= 10 {
				continue
			} else if transfer.Confirmations < 10 {
				logrus.Error("payment processing")
				continue
			}
			logrus.Error("payment done")
			// TODO: execute something
		}
	}
}

func cleanupDatabase() {
	var once utils.Once
	for {
		select {
		case <-once.After(5 * time.Second):
		case <-time.After(1 * time.Hour):
		}
		start := time.Now()
		database.DeleteOldSessions()
		database.DeleteOldUploads()
		database.DeleteOldChatMessages()
		database.DeleteOldPrivateChatRooms()
		database.DeleteOldCaptchaRequests()
		database.DeleteOldAuditLogs()
		database.DeleteOldSecurityLogs()
		logrus.Debugf("done cleaning database, took %s", time.Since(start))
	}
}
