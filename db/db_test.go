package db

import (
	"github.com/bakape/captchouli/common"
)

func init() {
	common.IsTest = true
	err := Open()
	if err != nil {
		panic(err)
	}
}
