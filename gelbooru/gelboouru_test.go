package gelbooru

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/bakape/captchouli/common"
)

func init() {
	common.IsTest = true
}

func TestFetch(t *testing.T) {
	f, _, err := Fetch(common.FetchRequest{
		Tag:    "sakura_kyouko",
		Rating: common.Safe,
		Source: common.Gelbooru,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	f.Seek(0, 0)
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	common.WriteSample(t, "fetched", buf)
}
