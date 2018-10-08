package common

import (
	"os"
	"path/filepath"
	"runtime"
)

var (
	RootDir string
	IsTest  = false
)

func init() {
	envKey := "HOME"
	dir := "captchouli"
	if runtime.GOOS == "windows" {
		envKey = "APPDATA"
	} else {
		dir = "." + dir
	}
	RootDir = filepath.Join(os.Getenv(envKey), dir)
}
