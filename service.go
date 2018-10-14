package captchouli

import (
	"errors"

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

type Options struct {
	// Source of image database to use for captcha image generation
	Source DataSource

	// Explicitness ratings to include. Defaults to Safe.
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
// Caller must call .Close() to deallocate the service after use.
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
	return
}
