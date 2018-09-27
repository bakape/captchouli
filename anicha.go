package anicha

// Image rating to use for source dataset
type Rating uint8

const (
	Safe Rating = 1 << iota
	Questionable
	Explicit
)

type Options struct {
	// Bit flags of ratings to include.
	// Defaults to Safe.
	IncludeRatings Rating

	// Tags to source for captcha solutions. One tag is randomly chosen for each
	// generated captcha.
	// Required.
	Tags []string
}
