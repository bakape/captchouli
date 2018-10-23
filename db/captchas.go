package db

import (
	crypto "crypto/rand"
	"database/sql"
	"encoding/binary"
	"math/rand"

	"github.com/Masterminds/squirrel"
	"github.com/bakape/captchouli/common"
)

// Number of matching images in each captcha
const MatchingCount = 4

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
	var matched [MatchingCount][16]byte
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
	var solution [MatchingCount]byte
	for i := 0; i < 9 && j < MatchingCount; i++ {
		for k := 0; k < MatchingCount; k++ {
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
		Where("tag = ? and source = ? and blacklist = false", tag, source).
		OrderBy("random()").
		Limit(MatchingCount)
	return scanHashes(q, 0, images, buf)
}

func getNonMatchingImages(tag string, images *[9][16]byte, buf *[]byte,
) (err error) {
	q := sq.Select("hash").
		From("images").
		Where(
			`not exists (
				select 1
				from image_tags
				where image_id = images.id and tag = ?)
			and blacklist = false`,
			tag).
		OrderBy("random()").
		Limit(9 - MatchingCount)
	return scanHashes(q, MatchingCount, images, buf)
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

// Check, if a solution to a captcha is valid
func CheckSolution(id [64]byte, solution []byte) (solved bool, err error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	err = InTransaction(func(tx *sql.Tx) (err error) {
		var (
			correct [MatchingCount]byte
			buf     []byte
		)
		err = withTransaction(tx, sq.
			Select("solution").
			From("captchas").
			Where("id = ? and status = 0", id[:])).
			QueryRow().
			Scan(&buf)
		switch err {
		case nil:
			copy(correct[:], buf)
		case sql.ErrNoRows:
			err = nil
			return
		default:
			return
		}

		solved = isSolved(correct, solution)
		var status int
		if solved {
			status = 1
		} else {
			status = 2
		}
		_, err = withTransaction(tx, sq.
			Update("captchas").
			Set("status", status).
			Where("id = ?", id[:])).
			Exec()
		return
	})
	return
}

func isSolved(correct [MatchingCount]byte, proposed []byte) bool {
	solved := 0
	for _, id := range proposed {
		for _, c := range correct {
			if id == c {
				solved++
				goto next
			}
		}
		return false
	next:
	}
	return solved >= MatchingCount-1
}

// Get solution for captcha by ID
func GetSolution(id [64]byte) (solution []byte, err error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	err = sq.
		Select("solution").
		From("captchas").
		Where("id = ?", id[:]).
		QueryRow().
		Scan(&solution)
	return
}

// Return, if captcha exists and is solved
func IsSolved(id [64]byte) (is bool, err error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	err = sq.Select("status = 1").
		From("captchas").
		Where("id = ?", id[:]).
		Scan(&is)
	if err != sql.ErrNoRows {
		err = nil
	}
	return
}
