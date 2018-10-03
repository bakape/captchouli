package captchouli

// #cgo pkg-config: opencv
// #cgo CFLAGS: -std=c11
// #cgo CXXFLAGS: -std=c++17
// #include "thumbnail.h"
// #include <stdlib.h>
import "C"
import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"unsafe"
)

// Image rating to use for source dataset
type Rating uint8

const (
	UndefinedRating Rating = 0
	Safe                   = 1 << iota
	Questionable
	Explicit
)

// Source of image database to use for captcha image generation
type DataSource uint8

const (
	Gelbooru DataSource = iota
)

func (d DataSource) String() string {
	switch d {
	case Gelbooru:
		return "gelbooru"
	default:
		return ""
	}
}

type Options struct {
	// Source of image database to use for captcha image generation
	Source DataSource

	// Bit flags of ratings to include.
	// Defaults to Safe.
	IncludeRatings Rating

	// Tags to source for captcha solutions. One tag is randomly chosen for each
	// generated captcha.
	// Required to contain at least 1 tag.
	Tags []string
}

// Generic error with prefix string
type Error string

func (e Error) Error() string {
	return "captchouli: " + string(e)
}

// Encapsulates a configured captcha-generation and verification service
type Service struct {
	opts       Options
	classifier unsafe.Pointer
}

// Create new captcha-generation and verification service
// Caller must call .Close() to deallocate the service after use.
func NewService(opts Options) (s *Service, err error) {
	if len(opts.Tags) == 0 {
		err = Error("no tags provided")
		return
	}

	// XXX: Not having the cascade file embedded into the binary would prevent
	// go-gettablity but the OpenCV CascadeClassifier requires a file path.
	var zipped []byte
	switch opts.Source {
	case Gelbooru:
		zipped = cascade_animeface
	}
	r, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return
	}

	tmp, err := ioutil.TempFile("", "*.xml")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	_, err = io.Copy(tmp, r)
	if err != nil {
		return
	}

	name := C.CString(tmp.Name())
	defer C.free(unsafe.Pointer(name))
	c := C.load_classifier(name)
	if c == nil {
		err = Error(fmt.Sprintf("unable to load classifier: %s", opts.Source))
		return
	}

	return &Service{opts, c}, nil
}

func (s *Service) Close() error {
	C.unload_classifier(s.classifier)
	return nil
}

// Generate a thumbnail of passed image.
// NOTE: the generated thumbnail is not deterministic.
func (s *Service) Thumbnail(src []byte) (thumb []byte, err error) {
	var out C.Buffer
	errC := C.thumbnail(s.classifier,
		C.Buffer{
			data: unsafe.Pointer(&src[0]),
			size: C.size_t(len(src)),
		},
		&out)
	if errC != nil {
		err = Error(C.GoString(errC))
		C.free(unsafe.Pointer(errC))
		return
	}
	thumb = C.GoBytes(out.data, C.int(out.size))
	C.free(out.data)
	return
}
