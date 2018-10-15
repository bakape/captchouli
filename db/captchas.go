package db

import (
	crypto "crypto/rand"
	"database/sql"
	"encoding/binary"
	"math/rand"

	"github.com/Masterminds/squirrel"
	"github.com/bakape/captchouli/common"
)

var _cryptoSource = cryptoSource{}

type cryptoSource struct{}

func (cryptoSource) Int63() int64 {
	var b [8]byte
	crypto.Read(b[:])
	// mask off sign bit to ensure positive number
	return int64(binary.LittleEndian.Uint64(b[:]) & (1<<63 - 1))
}

func (cryptoSource) Seed(_ int64) {}

// Generate a new captcha and return it ID and image list in order
func GenerateCaptcha(tag string, src common.DataSource,
) (id [64]byte, images [9][16]byte, err error) {
	buf := make([]byte, 16)
	err = getMatchingImages(tag, src, &images, &buf)
	if err != nil {
		return
	}
	var matched [3][16]byte
	copy(matched[:], images[:])

	err = getNonMatchingImages(tag, &images, &buf)
	if err != nil {
		return
	}

	rand.New(&_cryptoSource).Shuffle(9, func(i, j int) {
		images[i], images[j] = images[j], images[i]
	})

	// This produces a sorted array of the correct answer indices.
	// There might be a better way to do this.
	j := 0
	var solution [3]byte
	for i := 0; i < 9 && j < 3; i++ {
		for k := 0; k < 3; k++ {
			if matched[k] == images[i] {
				solution[j] = byte(i)
				j++
			}
		}
	}

	_, err = crypto.Read(id[:])
	if err != nil {
		return
	}

	dbMu.Lock()
	defer dbMu.Unlock()

	_, err = sq.Insert("captchas").
		Columns("id", "solution").
		Values(id[:], solution[:]).
		Exec()
	return
}

func getMatchingImages(tag string, source common.DataSource,
	images *[9][16]byte, buf *[]byte,
) (err error) {
	q := sq.Select("hash").
		From("image_tags").
		Join("images on images.id = image_id").
		Where("tag = ? and source = ?", tag, source).
		OrderBy("random()").
		Limit(3)
	return scanHashes(q, 0, images, buf)
}

func getNonMatchingImages(tag string, images *[9][16]byte, buf *[]byte,
) (err error) {
	q := sq.Select("hash").
		From("images").
		Join("image_tags on images.id = image_id").
		Where(
			`not exists (
				select 1
				from image_tags
				where image_id = images.id and tag = ?)`, tag).
		OrderBy("random()").
		Limit(6)
	return scanHashes(q, 3, images, buf)
}

func scanHashes(q squirrel.SelectBuilder, i int, images *[9][16]byte,
	buf *[]byte,
) (err error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	r, err := q.Query()
	if err != nil {
		return
	}
	defer r.Close()

	for r.Next() {
		err = r.Scan(buf)
		if err != nil {
			return
		}
		copy(images[i][:], *buf)
		i++
	}
	err = r.Err()
	return
}

// Check if a solution to a captcha is valid
func CheckCaptcha(id [64]byte, solution [3]byte) (ok bool, err error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	err = InTransaction(func(tx *sql.Tx) (err error) {
		var res [3]byte
		b := res[:]
		err = withTransaction(tx, sq.
			Select("solution").
			From("captchas").
			Where("id = ?", id[:])).
			QueryRow().
			Scan(&b)
		switch err {
		case nil:
			ok = res == solution
		case sql.ErrNoRows:
			return nil
		default:
			return
		}

		_, err = withTransaction(tx, sq.
			Delete("captcha").
			Where("id = ?", id[:])).
			Exec()
		return
	})
	return
}
