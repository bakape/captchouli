package captchouli

import (
	"errors"
	"log"

	"github.com/bakape/captchouli/db"

	"github.com/bakape/captchouli/common"
)

// Source of image database to use for captcha image generation
type DataSource = common.DataSource

// Image rating to use for source dataset
type Rating = common.Rating

const Gelbooru = common.Gelbooru
const (
	Safe         = common.Safe
	Questionable = common.Questionable
	Explicit     = common.Explicit
)

const (
	// minimum size of image pool for a tag
	poolMinSize = 6
)

// Options passed on Service creation
type Options struct {
	// Source of image database to use for captcha image generation
	Source DataSource

	// Explicitness ratings to include. Defaults to {Safe}.
	Ratings []Rating

	// Tags to source for captcha solutions. One tag is randomly chosen for each
	// generated captcha. Required to contain at least 1 tag but the database
	// must include at least 3 different tags for correct operation.
	//
	// Note that you can only include tags that are discernable from the
	// character's face, such as who the character is (example: "cirno") or a
	// facial feature of the character (example: "smug").
	Tags []string
}

// Encapsulates a configured captcha-generation and verification service
type Service struct {
	source  DataSource
	ratings []Rating
	tags    []string
}

// Create new captcha-generation and verification service
func NewService(opts Options) (s *Service, err error) {
	if len(opts.Tags) == 0 {
		err = Error{errors.New("no tags provided")}
		return
	}

	s = &Service{
		source:  opts.Source,
		tags:    opts.Tags,
		ratings: opts.Ratings,
	}
	if len(s.ratings) == 0 {
		s.ratings = []Rating{Safe}
	}

	err = initClassifier(opts.Source)
	if err != nil {
		return
	}
	err = s.initPool()
	return
}

// Initialize pool with enough images, if lacking
func (s *Service) initPool() (err error) {
	var count int
	for _, t := range s.tags {
		first := true
	check:
		count, err = db.ImageCount(t, s.source, s.ratings)
		if err != nil {
			return
		}
		if count >= poolMinSize {
			continue
		} else if first {
			first = false
			log.Printf("initializing image pool for tag `%s`\n", t)
		}

		err = fetch(common.FetchRequest{
			Tag:    t,
			Rating: s.randomRating(),
		})
		if err != nil {
			return
		}
		goto check
	}
	return
}

func (s *Service) randomRating() common.Rating {
	if len(s.ratings) == 1 {
		return s.ratings[0]
	}
	return s.ratings[common.RandomInt(len(s.ratings))]
}
