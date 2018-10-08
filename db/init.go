package db

import (
	"sync"
)

var (
	// To avoid locking "database locked" errors. Hard limitation of SQLite,
	// when used from multiple threads. Lock appropriately for read and write
	// queries.
	dbMu sync.RWMutex
)

func Open() (err error) {
	return nil
}

func Close() error {
	return nil
}
