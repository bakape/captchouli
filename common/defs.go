package common

import (
	"errors"
)

// Source of image database to use for captcha image generation
type DataSource uint8

const (
	Gelbooru DataSource = iota
)

const (
	// Keys used as names for input elements in captcha form HTML
	IDKey         = "captchouli-id"
	ColourKey     = "captchouli-color"
	BackgroundKey = "captchouli-background"
)

var (
	ErrNoMatch = Error{errors.New("no images match tag")}
)

func (d DataSource) String() string {
	return "gelbooru"
}

type FetchRequest struct {
	Source DataSource
	Tag    string
}

// Generic error with prefix string
type Error struct {
	Err error
}

func (e Error) Error() string {
	return "captchouli: " + e.Err.Error()
}
