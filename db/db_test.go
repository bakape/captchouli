package db

import (
	"os"
	"testing"
)

func TestMain(t *testing.M) {
	OpenForTests()
	os.Exit(t.Run())
}
