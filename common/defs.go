package common

import (
	"errors"
)

// Source of image database to use for captcha image generation
type DataSource uint8

const (
	Gelbooru DataSource = iota
	Danbooru
)

const (
	// Keys used as names for input elements in captcha form HTML
	IDKey         = "captchouli-id"
	ColourKey     = "captchouli-color"
	BackgroundKey = "captchouli-background"
)

var (
	ErrNoMatch = Error{errors.New("not enough images match tag")}
)

func (d DataSource) String() string {
	switch d {
	case Gelbooru:
		return "gelbooru"
	case Danbooru:
		return "danBooru"
	default:
		return "unknown_source"
	}
}

type FetchRequest struct {
	Tag string
}

// Generic error with prefix string
type Error struct {
	Err error
}

func (e Error) Error() string {
	return "captchouli: " + e.Err.Error()
}
