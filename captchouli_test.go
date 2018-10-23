package captchouli

import (
	"os"
	"testing"

	"github.com/bakape/captchouli/db"
)

func TestMain(t *testing.M) {
	db.OpenForTests()
	os.Exit(t.Run())
}

func newService(t *testing.T) *Service {
	s, err := NewService(Options{
		Tags:  []string{"patchouli_knowledge", "cirno", "hakurei_reimu"},
		Quiet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
