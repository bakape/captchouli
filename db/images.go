package db

import (
	"database/sql"

	"github.com/bakape/captchouli/common"
)

type Image struct {
	Source common.DataSource
	Rating common.Rating
	MD5    [16]byte
	Tags   []string
}

// Return, if file is not already registered in the DB as valid thumbnail or in
// a blacklist
func IsInDatabase(md5 [16]byte) (is bool, err error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	err = sq.Select("1").
		From("images").
		Where("hash = ?", md5[:]).
		Scan(&is)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}

// Write image to database. An image without tags will count as a blacklisted
// image.
func InsertImage(img Image) (err error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	return InTransaction(func(tx *sql.Tx) (err error) {
		r, err := withTransaction(tx, sq.
			Insert("images").
			Columns("hash").
			Values(img.MD5[:]).
			Suffix("on conflict do nothing")).
			Exec()
		if err != nil {
			return
		}
		id, err := r.LastInsertId()
		if err != nil {
			return
		}

		q, err := tx.Prepare(
			`insert into image_tags (image_id, tag, source, rating)
			values(?, ?, ?, ?)`)
		if err != nil {
			return
		}
		for _, t := range img.Tags {
			_, err = q.Exec(id, t, int(img.Source), int(img.Rating))
			if err != nil {
				return
			}
		}
		return
	})
}
