package db

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
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

type rowScanner interface {
	Scan(dest ...interface{}) error
}

type tableScanner interface {
	rowScanner
	Next() bool
	Err() error
	Close() error
}

// Allows easily running squirrel queries with transactions
type transactionalQuery struct {
	tx  *sql.Tx
	sq  squirrel.Sqlizer
	err error
	row *sql.Row
}

func withTransaction(tx *sql.Tx, q squirrel.Sqlizer) transactionalQuery {
	return transactionalQuery{
		tx: tx,
		sq: q,
	}
}

func (t transactionalQuery) Exec() (res sql.Result, err error) {
	sql, args, err := t.sq.ToSql()
	if err != nil {
		return
	}
	return t.tx.Exec(sql, args...)
}

func (t transactionalQuery) Query() (ts tableScanner, err error) {
	sql, args, err := t.sq.ToSql()
	if err != nil {
		return
	}
	return t.tx.Query(sql, args...)
}

func (t transactionalQuery) QueryRow() rowScanner {
	var (
		sql  string
		args []interface{}
	)
	sql, args, t.err = t.sq.ToSql()
	if t.err == nil {
		t.row = t.tx.QueryRow(sql, args...)
	}
	return &t
}

func (t transactionalQuery) Scan(dst ...interface{}) error {
	if t.err != nil {
		return t.err
	}
	return t.row.Scan(dst...)
}
