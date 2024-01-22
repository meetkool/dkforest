package actions

import (
	"bytes"
	captcha "dkforest/pkg/captcha"
	"dkforest/pkg/color"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	"dkforest/pkg/web"
	"dkforest/pkg/web/handlers/interceptors"
	"dkforest/pkg/web/handlers/poker"
	"fmt"
	"github.com/mattn/go-colorable"
	wallet1 "github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
	cli "github.com/urfave/cli/v2"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func Start(c *cli.Context) error {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Ltime | log.Lshortfile)
	logrus.SetFormatter(LogFormatter{})
	logrus.SetOutput(colorable.NewColorableStderr())

	logrus.Info("DarkForest v" + config.Global.AppVersion.Get().String())

	port := c.Int("port")
	host := c.String("host")
	noBrowser := c.Bool("no-browser")

	config.Global.CookieSecure.Set(c.Bool("cookie-secure"))
	config.Global.CookieDomain.Set(c.String("cookie-domain"))

	ensureProjectHome()

	dbPath := filepath.Join(config.Global.ProjectPath.Get(), config.DbFileName)
	db := database.NewDkfDB(dbPath)

	runMigrations(db)

	config.IsFirstUse.Store(isFirstUse(db))

	captcha.SetCustomStore(captcha.NewMemoryStore(captcha.CollectNum, 120*time.Second))

	if false {
		err := db.DB().Debug().Exec(`
	`).Error
		logrus.Error(err)
	}

	db.GetPokerCasino()
	settings := db.GetSettings()
	config.ProtectHome.Store(settings.ProtectHome)
	config.HomeUsersList.Store(settings.HomeUsersList)
	config.ForceLoginCaptcha.Store(settings.ForceLoginCaptcha)
	config.SignupEnabled.Store(settings.SignupEnabled)
	config.SignupFakeEnabled.Store(settings.SignupFakeEnabled)
	config.DownloadsEnabled.Store(settings.DownloadsEnabled)
	config.ForumEnabled.Store(settings.ForumEnabled)
	config.SilentSelfKick.Store(settings.SilentSelfKick)
	config.MaybeAuthEnabled.Store(settings.MaybeAuthEnabled)
	config.PowEnabled.Store(settings.PowEnabled)
	config.PokerWithdrawEnabled.Store(settings.PokerWithdrawEnabled)
	config.CaptchaDifficulty.Store(settings.CaptchaDifficulty)
	config.MoneroPrice.Store(settings.MoneroPrice)

	walletClient := config.Xmr()

	utils.SGo(func() { cleanupDatabase(db) })
	utils.SGo(func() { managers.ActiveUsers.CleanupUsersCache() })
	utils.SGo(func() { xmrWatch(db) })
	utils.SGo(func() { openBrowser(noBrowser, int64(port)) })

	poker.Refund(db)

	if !walletIsBalanced(db, walletClient) {
		// TODO: automatically send transactions that are not processed
		config.PokerWithdrawEnabled.Store(false)
		logrus.Error("wallet is not balanced")
		dutils.RootAdminNotify(db, "wallet is not balanced; poker withdraw disabled")
	}

	interceptors.LoadFilters(db)
	interceptors.ChessInstance = interceptors.NewChess(db)
	interceptors.BattleshipInstance = interceptors.NewBattleship(db)
	interceptors.WWInstance = interceptors.NewWerewolf(db)

	web.Start(db, host, port)
	return nil
}

// Returns either or not the monero wallet balance matches the database balance
func walletIsBalanced(db *database.DkfDB, client wallet1.Client) (balanced bool) {
	resBalance, err := client.GetBalance(&wallet1.RequestGetBalance{})
	if err != nil {
		logrus.Error(err)
		return false
	}
	walletBalance := database.Piconero(resBalance.Balance)

	var diffInOut database.Piconero
	if err := db.WithE(func(tx *database.DkfDB) error {
		sumIn, err := tx.GetPokerXmrTransactionsSumIn()
		if err != nil {
			return err
		}
		sumOut, err := tx.GetPokerXmrTransactionsSumOut()
		if err != nil {
			return err
		}
		diffInOut = sumIn - sumOut
		return nil
	}); err != nil {
		logrus.Error(err)
		return false
	}

	return walletBalance == diffInOut
}

func isFirstUse(db *database.DkfDB) bool {
	var count int64
	db.DB().Model(database.User{}).Count(&count)
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

func runMigrations(db *database.DkfDB) {
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
	db.DB().Exec("PRAGMA foreign_keys=OFF")
	n, err := migrate.Exec(utils.Must(db.DB().DB()), "sqlite3", migrations, migrate.Up)
	if err != nil {
		panic(err)
	}
	db.DB().Exec("PRAGMA foreign_keys=ON")
	logrus.Infof("applied %d migrations", n)
}

// Ensure the project folder is created properly
func ensureProjectHome() {
	config.Global.ProjectPath.Set(utils.MustGetDefaultProjectPath())
	projectPath := config.Global.ProjectPath.Get()
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		logrus.Fatal("Failed to create project folder", err)
	}

	config.Global.ProjectLocalsPath.Set(filepath.Join(projectPath, "locals"))
	if err := os.MkdirAll(config.Global.ProjectLocalsPath.Get(), 0755); err != nil {
		logrus.Fatal("Failed to create dkforest locals folder", err)
	}

	config.Global.ProjectHTMLPath.Set(filepath.Join(projectPath, "html"))
	if err := os.MkdirAll(config.Global.ProjectHTMLPath.Get(), 0755); err != nil {
		logrus.Fatal("Failed to create dkforest html folder", err)
	}

	config.Global.ProjectMemesPath.Set(filepath.Join(projectPath, "memes"))
	if err := os.MkdirAll(config.Global.ProjectMemesPath.Get(), 0755); err != nil {
		logrus.Fatal("Failed to create memes uploads folder", err)
	}

	config.Global.ProjectUploadsPath.Set(filepath.Join(projectPath, "uploads"))
	if err := os.MkdirAll(config.Global.ProjectUploadsPath.Get(), 0755); err != nil {
		logrus.Fatal("Failed to create dkforest uploads folder", err)
	}

	config.Global.ProjectFiledropPath.Set(filepath.Join(projectPath, "filedrop"))
	if err := os.MkdirAll(config.Global.ProjectFiledropPath.Get(), 0755); err != nil {
		logrus.Fatal("Failed to create dkforest filedrop folder", err)
	}

	// Contains files that we offer for download directly
	config.Global.ProjectDownloadsPath.Set(filepath.Join(projectPath, "downloads"))
	if err := os.MkdirAll(config.Global.ProjectDownloadsPath.Get(), 0755); err != nil {
		logrus.Fatal("Failed to create dkforest downloads folder", err)
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

func xmrWatch(db *database.DkfDB) {
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
			if processPokerTransfer(db, transfer) {
				continue
			}
		}
	}
}

func processPokerTransfer(db *database.DkfDB, transfer *wallet1.Transfer) (found bool) {
	var user database.User
	pokerTransfer, err := db.GetPokerXmrTransaction(transfer.TxID)
	if err != nil {
		user, err = db.GetUserByPokerXmrSubAddress(transfer.Address)
		if err != nil {
			return false // Not a poker deposit address
		}

		// Create a new transaction and rotate user deposit address
		if txErr := db.WithE(func(tx *database.DkfDB) error {
			userID := user.ID
			pokerTransfer, err = tx.CreatePokerXmrInTransaction(userID, transfer)
			if err != nil {
				return err
			}
			// Update user's xmr deposit address
			res, err := config.Xmr().CreateAddress(&wallet1.RequestCreateAddress{})
			if err != nil {
				return err
			}
			if err := tx.SetPokerSubAddress(userID, res.Address); err != nil {
				return err
			}
			return nil
		}); txErr != nil {
			logrus.Error(txErr)
			return true
		}
	} else {
		if pokerTransfer.Confirmations < 10 {
			pokerTransfer.Confirmations = utils.MinInt(transfer.Confirmations, 10)
			pokerTransfer.DoSave(db)
		}
		if pokerTransfer.Processed {
			return true
		}
		user, _ = db.GetUserByID(pokerTransfer.UserID)
	}

	if !pokerTransfer.HasEnoughConfirmations() {
		return true
	}

	// Increment user's xmr balance, and update transfer status
	if txErr := db.WithE(func(tx *database.DkfDB) error {
		if err := user.IncrXmrBalance(tx, pokerTransfer.Amount); err != nil {
			return err
		}
		pokerTransfer.Processed = true
		pokerTransfer.DoSave(tx)
		return nil
	}); txErr != nil {
		logrus.Error(err)
		return true
	}

	dutils.RootAdminNotify(db, fmt.Sprintf("new deposit %s xmr by %s", pokerTransfer.Amount.XmrStr(), user.Username))
	return true
}

func cleanupDatabase(db *database.DkfDB) {
	var once utils.Once
	for {
		select {
		case <-once.After(5 * time.Second):
		case <-time.After(1 * time.Hour):
		}
		start := time.Now()
		db.DeleteOldSessions()
		db.DeleteOldUploads()
		db.DeleteOldChatMessages()
		db.DeleteOldPrivateChatRooms()
		db.DeleteOldCaptchaRequests()
		db.DeleteOldAuditLogs()
		db.DeleteOldSecurityLogs()
		db.DeleteOldIgnoredUsers()
		db.DeleteOldPmBlacklistedUsers()
		db.DeleteOldPmWhitelistedUsers()
		db.DeleteOldChatInboxMessages()
		db.DeleteOldDownloads()
		db.DeleteOldSessionNotifications()
		logrus.Debugf("done cleaning database, took %s", time.Since(start))
	}
}

func BuildProhibitedPasswords(c *cli.Context) error {
	// TODO: Fix gormbulk to use new gorm lib
	//fmt.Println("start")
	//if !utils.FileExists("rockyou.txt") {
	//	return errors.New("rockyou.txt not found")
	//}
	//
	//ensureProjectHome()
	//
	//dbPath := filepath.Join(config.Global.ProjectPath.Get(), config.DbFileName)
	//db := database.NewDkfDB(dbPath).DB()
	//
	//readFile, _ := os.Open("rockyou.txt")
	//fileScanner := bufio.NewScanner(readFile)
	//fileScanner.Split(bufio.ScanLines)
	//var rows []interface{}
	//for fileScanner.Scan() {
	//	rows = append(rows, database.ProhibitedPassword{Password: fileScanner.Text()})
	//}
	//readFile.Close()
	//if err := gormbulk.BulkInsert(db, rows, 10000); err != nil {
	//	logrus.Error(err)
	//}
	return nil
}
