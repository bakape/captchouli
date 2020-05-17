package db

import (
	"database/sql"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/bakape/boorufetch"
	"github.com/bakape/captchouli/v2/common"
)

type Image struct {
	Rating boorufetch.Rating
	Source common.DataSource
	MD5    [16]byte
	Tags   []string
}

// Return, if file is not already registered in the DB as valid thumbnail or in
// a blacklist
func IsInDatabase(md5 [16]byte) (bool, error) {
	return imageExists("images", md5)
}

// Write image to database
func InsertImage(img Image) (err error) {
	if len(img.Tags) == 0 {
		return BlacklistImage(img.MD5)
	}

	lowercaseTags(img.Tags)

	dbMu.Lock()
	defer dbMu.Unlock()

	return InTransaction(func(tx *sql.Tx) (err error) {
		r, err := sq.
			Insert("images").
			Columns("hash", "rating").
			Values(img.MD5[:], img.Rating).
			RunWith(tx).
			Exec()
		if err != nil {
			return
		}
		id, err := r.LastInsertId()
		if err != nil {
			return
		}

		q, err := tx.Prepare(
			`insert into image_tags (image_id, tag, source)
			values(?, ?, ?)`)
		if err != nil {
			return
		}
		for _, t := range img.Tags {
			_, err = q.Exec(id, t, img.Source)
			if err != nil {
				return
			}
		}
		return
	})
}

// Add image to blacklist so that it is not fetched again
func BlacklistImage(hash [16]byte) (err error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	_, err = sq.
		Insert("images").
		Columns("hash", "blacklist").
		Values(hash[:], true).
		Exec()
	return
}

// Return count of images matching selectors
func ImageCount(f Filters) (n int, err error) {
	f.Tag = strings.ToLower(f.Tag)

	dbMu.RLock()
	defer dbMu.RUnlock()

	err = sq.Select("count(*)").
		From("image_tags").
		Join("images on image_id = images.id").
		Where(squirrel.Eq{
			"tag":       f.Tag,
			"blacklist": false,
			"rating":    f.Explicitness,
		}).
		Scan(&n)
	return
}
