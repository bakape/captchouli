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
	"sync"
	"unsafe"

	"github.com/bakape/captchouli/common"
)

var (
	classifiers   = make(map[common.DataSource]unsafe.Pointer)
	classifiersMu sync.Mutex
)

func initClassifier(src common.DataSource) (err error) {
	classifiersMu.Lock()
	defer classifiersMu.Unlock()

	c, ok := classifiers[src]
	if ok {
		return
	}

	// XXX: Not having the cascade file embedded into the binary would prevent
	// go-getablity but the OpenCV CascadeClassifier requires a file path.
	var zipped []byte
	switch src {
	case Gelbooru:
		zipped = cascade_animeface
	}
	r, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return
	}
	defer r.Close()

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
	c = C.load_classifier(name)
	if c == nil {
		return Error{fmt.Errorf("unable to load classifier: %s", src)}
	}
	classifiers[src] = c
	return
}

// Generate a thumbnail of passed image.
// NOTE: the generated thumbnail is not deterministic.
func thumbnail(path string, src common.DataSource) (thumb []byte, err error) {
	classifiersMu.Lock()
	defer classifiersMu.Unlock()
	classifier := classifiers[src]

	var out C.Buffer
	pathC := C.CString(path)
	defer C.free(unsafe.Pointer(pathC))

	errC := C.thumbnail(classifier, pathC, &out)
	defer func() {
		if errC != nil {
			C.free(unsafe.Pointer(errC))
		}
		if out.data != nil {
			C.free(out.data)
		}
	}()
	if errC != nil {
		s := C.GoString(errC)
		if s == "no faces detected" {
			err = ErrNoFace
		} else {
			err = Error{errors.New(s)}
		}
		return
	}

	thumb = C.GoBytes(out.data, C.int(out.size))
	return
}
