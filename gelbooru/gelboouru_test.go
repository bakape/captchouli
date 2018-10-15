package gelbooru

import (
	"io/ioutil"
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
	defer os.Remove(f.Name())
	defer f.Close()

	_, err = f.Seek(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	common.WriteSample(t, "fetched", buf)
}
