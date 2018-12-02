package db

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
)

var version = len(migrations)

var migrations = []func(*sql.Tx) error{
	func(tx *sql.Tx) (err error) {
		// Initialize DB
		return execAll(tx,
			`create table main (
				id text primary key,
				val text not null
			)`,
			`insert into main (id, val) values('version', '1')`,
			`create table images (
				id integer primary key,
				hash blob not null,
				blacklist bool not null default false
			)`,
			createIndex("images", "hash", true),
			createIndex("images", "blacklist", false),
			`create table image_tags (
				image_id integer not null references images on delete cascade,
				tag text not null,
				source int not null,
				primary key (image_id, tag, source)
			)`,
			createIndex("image_tags", "image_id", false),
			createIndex("image_tags", "tag", false),
			createIndex("image_tags", "source", false),
		)
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			`create table captchas (
				id blob primary key,
				solution blob not null,
				status int not null default 0,
				created datetime not null default current_timestamp
			)`,
			createIndex("captchas", "created", false),
			createIndex("captchas", "status", false),
		)
	},
}

// Run migrations from version `from`to version `to`
func runMigrations(from, to int) (err error) {
	var tx *sql.Tx

	rollBack := func() error {
		if rbErr := tx.Rollback(); rbErr != nil {
			return rbErr
		}
		return err
	}

	for i := from; i < to; i++ {
		log.Printf("captchouli: upgrading database to version %d\n", i+1)
		tx, err = db.Begin()
		if err != nil {
			return
		}

		err = migrations[i](tx)
		if err != nil {
			return rollBack()
		}

		// Write new version number
		_, err = sq.Update("main").
			Set("val", i+1).
			Where("id = 'version'").
			RunWith(tx).
			Exec()
		if err != nil {
			return rollBack()
		}

		err = tx.Commit()
		if err != nil {
			return
		}
	}
	return
}

func createIndex(table, column string, unique bool) string {
	var w bytes.Buffer
	w.WriteString("create ")
	if unique {
		w.WriteString("unique ")
	}
	fmt.Fprintf(&w, `index %s_%s on %s (%s)`, table, column, table, column)
	return w.String()
}
