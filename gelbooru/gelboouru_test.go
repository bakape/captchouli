package gelbooru

import (
	"os"
	"testing"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
)

func TestMain(t *testing.M) {
	db.OpenForTests()
	os.Exit(t.Run())
}

func TestFetch(t *testing.T) {
	f, _, err := Fetch(common.FetchRequest{
		Tag:    "sakura_kyouko",
		Source: common.Gelbooru,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		defer os.Remove(f.Name())
		defer f.Close()
	}
}

func TestNoMatch(t *testing.T) {
	_, _, err := Fetch(common.FetchRequest{
		Tag:    "sakura_kyouko_dsadsdadsadsad",
		Source: common.Gelbooru,
	})
	if err != common.ErrNoMatch {
		t.Fatal(err)
	}
}

func TestOnlyOnePage(t *testing.T) {
	f, _, err := Fetch(common.FetchRequest{
		Tag:    "=>",
		Source: common.Gelbooru,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		defer os.Remove(f.Name())
		defer f.Close()
	}
}
