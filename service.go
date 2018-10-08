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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"unsafe"

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
	// generated captcha.
	// Required to contain at least 1 tag.
	Tags []string
}

// Encapsulates a configured captcha-generation and verification service
type Service struct {
	source     DataSource
	ratings    []Rating
	classifier unsafe.Pointer
	tags       []string
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

	// XXX: Not having the cascade file embedded into the binary would prevent
	// go-getablity but the OpenCV CascadeClassifier requires a file path.
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
	s.classifier = C.load_classifier(name)
	if s.classifier == nil {
		err = Error{fmt.Errorf("unable to load classifier: %s", opts.Source)}
	}
	return
}

func (s *Service) Close() error {
	C.unload_classifier(s.classifier)
	s.classifier = nil
	return nil
}

// Generate a thumbnail of passed image.
// NOTE: the generated thumbnail is not deterministic.
func (s *Service) Thumbnail(path string) (thumb []byte, err error) {
	var out C.Buffer
	pathC := C.CString(path)
	defer C.free(unsafe.Pointer(pathC))

	errC := C.thumbnail(s.classifier, pathC, &out)
	defer func() {
		if errC != nil {
			C.free(unsafe.Pointer(errC))
		}
		if out.data != nil {
			C.free(out.data)
		}
	}()
	if errC != nil {
		err = Error{errors.New(C.GoString(errC))}
		return
	}

	thumb = C.GoBytes(out.data, C.int(out.size))
	return
}
