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
	"io"
	"io/ioutil"
	"os"
	"sync"
	"unsafe"
)

var (
	classifier   unsafe.Pointer
	classifierMu sync.Mutex
)

func initClassifier() (err error) {
	classifierMu.Lock()
	defer classifierMu.Unlock()

	if classifier != nil {
		return
	}

	// XXX: Not having the cascade file embedded into the binary would prevent
	// go-getablity but the OpenCV CascadeClassifier requires a file path.
	r, err := gzip.NewReader(bytes.NewReader(cascade_animeface))
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
	c := C.cpli_load_classifier(name)
	if c == nil {
		return Error{errors.New("unable to load classifier")}
	}
	classifier = c
	return
}

// Generate a thumbnail of passed image.
// NOTE: the generated thumbnail is not deterministic.
func thumbnail(path string) (thumb []byte, err error) {
	classifierMu.Lock()
	defer classifierMu.Unlock()

	var out C.Buffer
	pathC := C.CString(path)
	defer C.free(unsafe.Pointer(pathC))

	errC := C.cpli_thumbnail(classifier, pathC, &out)
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
