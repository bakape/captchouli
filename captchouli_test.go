package captchouli

import (
	"os"
	"testing"

	"github.com/bakape/captchouli/v2/db"
)

func TestMain(t *testing.M) {
	db.OpenForTests()
	os.Exit(t.Run())
}

func newService(t *testing.T) *Service {
	s, err := NewService(Options{
		Tags: []string{"patchouli_knowledge", "cirno", "hakurei_reimu"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
