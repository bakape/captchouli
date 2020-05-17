package db

import (
	"database/sql"
	"strings"
)

// Execute all SQL statement strings and return on first error, if any
func execAll(tx *sql.Tx, q ...string) error {
	for _, q := range q {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Runs function inside a transaction and handles comminting and rollback on
// error
func InTransaction(fn func(*sql.Tx) error) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return
	}

	err = fn(tx)
	if err != nil {
		tx.Rollback()
		return
	}
	return tx.Commit()
}

// Check, if image exists in table
func imageExists(table string, md5 [16]byte) (exists bool, err error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	err = sq.Select("1").
		From(table).
		Where("hash = ?", md5[:]).
		Scan(&exists)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}

func lowercaseTags(tags []string) {
	for i := range tags {
		tags[i] = strings.ToLower(tags[i])
	}
}
