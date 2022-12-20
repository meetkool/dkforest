package database

import (
	"dkforest/pkg/config"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/mattn/go-sqlite3"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// DB ...
var DB *gorm.DB

// OpenSqlite3DB ...
func OpenSqlite3DB(path string) (*gorm.DB, error) {
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	db.DB().SetMaxIdleConns(1) // 10
	db.DB().SetMaxOpenConns(1) // 25
	db.LogMode(false)
	db.Exec("PRAGMA foreign_keys=ON")
	return db, nil
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
	dbPath := filepath.Join(config.Global.ProjectPath(), config.DbFileName)
	bckPath := filepath.Join(config.Global.ProjectPath(), "backup.db")
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
