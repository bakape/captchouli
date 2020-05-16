package db

import (
	"database/sql"
	"encoding/json"

	"github.com/bakape/boorufetch"
	"github.com/bakape/captchouli/common"
)

// Image data fetched from boorus pending processing by random selection
type PendingImage struct {
	Rating         boorufetch.Rating
	MD5            [16]byte
	TargetTag, URL string
	Tags           []string
}

// Check, if image is already on the list of pending images
func IsPendingImage(md5 [16]byte) (bool, error) {
	return imageExists("pending_images", md5)
}

// Insert a new image pending processing
func InsertPendingImage(img PendingImage) (err error) {
	tags, err := json.Marshal(img.Tags)
	if err != nil {
		return
	}

	dbMu.Lock()
	defer dbMu.Unlock()

	_, err = sq.Insert("pending_images").
		Columns("rating", "hash", "target_tag", "url", "tags").
		Values(img.Rating, img.MD5[:], img.TargetTag, img.URL, tags).
		Exec()
	return
}

// Deletes random pending pending image for tag and returns it, if any
func PopRandomPendingImage(tag string) (img PendingImage, err error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	err = InTransaction(func(tx *sql.Tx) (err error) {
		var n int
		err = sq.Select("count(*)").
			From("pending_images").
			Where("target_tag = ?", tag).
			RunWith(tx).
			QueryRow().
			Scan(&n)
		if err != nil {
			return
		}
		if n == 0 {
			return sql.ErrNoRows
		}

		var tags, md5 []byte
		err = sq.Select("rating", "hash", "url", "tags").
			From("pending_images").
			Where("target_tag = ?", tag).
			OrderBy("hash").
			Offset(uint64(common.RandomInt(n))).
			Limit(1).
			RunWith(tx).
			QueryRow().
			Scan(&img.Rating, &md5, &img.URL, &tags)
		if err != nil {
			return
		}
		img.TargetTag = tag
		copy(img.MD5[:], md5)
		err = json.Unmarshal(tags, &img.Tags)
		if err != nil {
			return
		}

		_, err = sq.Delete("pending_images").
			Where("hash = ?", img.MD5[:]).
			RunWith(tx).
			Exec()
		return
	})
	return
}

// Count pending images for tag
func CountPending(tag string) (n int, err error) {
	err = sq.Select("count(*)").
		From("pending_images").
		Where("target_tag = ?", tag).
		QueryRow().
		Scan(&n)
	return
}
