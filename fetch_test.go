package captchouli

import (
	"testing"

	"github.com/bakape/captchouli/common"
)

func TestFetch(t *testing.T) {
	err := fetch(common.FetchRequest{
		Tag: "patchouli_knowledge",
	})
	if err != nil {
		t.Fatal(err)
	}
}
