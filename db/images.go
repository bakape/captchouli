package db

import (
	"github.com/bakape/captchouli/common"
)

type Image struct {
	MD5 [16]byte
}

// Return, if file is not already registered in the DB as valid thumbnail or in
// a blacklist
func IsInDatabase(md5 [16]byte) (is bool, err error) {
	if common.IsTest {
		return
	}
	dbMu.RLock()
	defer dbMu.RUnlock()

	panic("TODO")
}
