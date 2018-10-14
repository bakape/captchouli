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
			`insert into main (id, val)
				values('version', '1')`,
			`create table images (
				id integer primary key,
				hash blob not null
			)`,
			createIndex("images", "hash", true),
			`create table image_tags (
				image_id int not null,
				tag text not null,
				source int not null,
				rating int not null,
				primary key (image_id, tag, source)
			)`,
			createIndex("image_tags", "image_id", false),
			`create index image_tags_search_idx
				on image_tags (tag, source, rating)`,
		)
	},
	func(tx *sql.Tx) (err error) {
		return execAll(tx,
			createIndex("image_tags", "tag", false),
			createIndex("image_tags", "source", false),
			createIndex("image_tags", "rating", false),
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
		log.Printf("upgrading database to version %d\n", i+1)
		tx, err = db.Begin()
		if err != nil {
			return
		}

		err = migrations[i](tx)
		if err != nil {
			return rollBack()
		}

		// Write new version number
		_, err = withTransaction(tx, sq.Update("main").
			Set("val", i+1).
			Where("id = 'version'")).
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
