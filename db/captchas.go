package db

import (
	crypto "crypto/rand"
	"database/sql"
	"math/rand"

	"github.com/bakape/boorufetch"

	"github.com/Masterminds/squirrel"
	"github.com/bakape/captchouli/common"
)

// Filters for querying an image for a captcha
type Filters struct {
	common.FetchRequest
	Explicitness []boorufetch.Rating
}

// Generate a new captcha and return its ID and image list in order
func GenerateCaptcha(f Filters) (id [64]byte, images [9][16]byte, err error) {
	buf := make([]byte, 16)
	matchedCount, err := getMatchingImages(f, &images, &buf)
	if err != nil {
		return
	}
	matched := make([][16]byte, matchedCount)
	copy(matched, images[:])

	err = getNonMatchingImages(f, 9-matchedCount, &images, &buf)
	if err != nil {
		return
	}

	rand.New(common.CryptoSource).Shuffle(9, func(i, j int) {
		images[i], images[j] = images[j], images[i]
	})

	// This produces a sorted array of the correct answer indices.
	// There might be a better way to do this.
	j := 0
	solution := make([]byte, matchedCount)
	for i := 0; i < 9 && j < matchedCount; i++ {
		for k := 0; k < matchedCount; k++ {
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

func getMatchingImages(f Filters, images *[9][16]byte, buf *[]byte,
) (n int, err error) {
	n = common.RandomInt(2) + 2
	q := sq.Select("hash").
		From("image_tags").
		Join("images on images.id = image_id").
		Where(squirrel.Eq{
			"tag":       f.Tag,
			"source":    f.Source,
			"blacklist": false,
			"rating":    f.Explicitness,
		}).
		OrderBy("random()").
		Limit(uint64(n))
	err = scanHashes(q, 0, images, buf)
	return
}

func getNonMatchingImages(f Filters, n int, images *[9][16]byte, buf *[]byte,
) (err error) {
	q := sq.Select("hash").
		From("images").
		Where(
			`not exists (
				select 1
				from image_tags
				where image_id = images.id and tag = ?)`,
			f.Tag).
		Where(squirrel.Eq{
			"blacklist": false,
			"rating":    f.Explicitness,
		}).
		OrderBy("random()").
		Limit(uint64(n))
	return scanHashes(q, 9-n, images, buf)
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
			correct []byte
		)
		err = sq.
			Select("solution").
			From("captchas").
			Where("id = ? and status = 0", id[:]).
			RunWith(tx).
			QueryRow().
			Scan(&correct)
		switch err {
		case nil:
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
		_, err = sq.
			Update("captchas").
			Set("status", status).
			Where("id = ?", id[:]).
			RunWith(tx).
			Exec()
		return
	})
	return
}

func isSolved(correct []byte, proposed []byte) bool {
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
	return solved >= len(correct)-1
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

// Return, if captcha exists and is solved. The captcha is deleted on a
// successful check to prevent replayagain attacks.
func IsSolved(id [64]byte) (is bool, err error) {
	dbMu.Lock()
	defer dbMu.Unlock()

	res, err := sq.Delete("captchas").
		Where("id = ? and status = 1", id[:]).
		Exec()
	if err != nil {
		return
	}
	n, err := res.RowsAffected()
	if err != nil {
		return
	}
	is = n != 0
	return
}
