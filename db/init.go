package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Masterminds/squirrel"
	"github.com/bakape/captchouli/common"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db *sql.DB
	sq squirrel.StatementBuilderType

	// To avoid locking "database locked" errors. Hard limitation of SQLite,
	// when used from multiple threads. Lock appropriately for read and write
	// queries.
	dbMu sync.RWMutex
)

// Open a database connection
func Open() (err error) {
	// Create root dir, id it does not exist
	_, err = os.Stat(common.RootDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Join(common.RootDir, "images"),
				os.ModeDir|0700)
			if err != nil {
				return
			}
		} else {
			return
		}
	}

	db, err = sql.Open("sqlite3",
		fmt.Sprintf("file:%s?cache=shared&mode=rwc",
			filepath.Join(common.RootDir, "db.db")))
	if err != nil {
		return
	}
	sq = squirrel.StatementBuilder.RunWith(squirrel.NewStmtCacheProxy(db))

	var currentVersion int
	err = sq.Select("val").
		From("main").
		Where("id = 'version'").
		QueryRow().
		Scan(&currentVersion)
	if err != nil {
		if s := err.Error(); strings.HasPrefix(s, "no such table") {
			err = nil
		} else {
			return
		}
	}
	return runMigrations(currentVersion, version)
}

// Close database connection
func Close() error {
	return db.Close()
}

// Open database for testing purposes
func OpenForTests() {
	common.IsTest = true
	err := Open()
	if err != nil {
		panic(err)
	}
}
