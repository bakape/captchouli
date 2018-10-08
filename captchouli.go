package captchouli

import (
	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
)

// Generic error with prefix string
type Error = common.Error

// Init storage and start the runtime
func Open() error {
	return db.Open()
}

// Close open storage resources
func Close() error {
	return db.Close()
}
