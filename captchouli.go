package captchouli

// #include "thumbnail.h"
import "C"
import (
	"fmt"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
)

// Generic error with prefix string
type Error = common.Error

var (
	ErrNoFace = Error{fmt.Errorf("no faces detected")}
)

// Init storage and start the runtime
func Open() error {
	return db.Open()
}

// Close open resources
func Close() error {
	classifiersMu.Lock()
	for s, c := range classifiers {
		C.unload_classifier(c)
		delete(classifiers, s)
	}
	classifiersMu.Unlock()

	return db.Close()
}
