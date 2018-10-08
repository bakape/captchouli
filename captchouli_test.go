package captchouli

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/bakape/captchouli/common"
)

func init() {
	common.IsTest = true
}

func TestThumbnailing(t *testing.T) {
	s, err := NewService(Options{
		Tags: []string{"patchouli_knowledge", "cirno", "hakurei_reimu"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := [...]struct {
		ext string
	}{
		{"jpg"},
		{"png"},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.ext, func(t *testing.T) {
			t.Parallel()

			p, err := filepath.Abs(filepath.Join("testdata", "sample."+c.ext))
			if err != nil {
				t.Fatal(err)
			}
			thumb, err := s.Thumbnail(p)
			if err != nil {
				t.Fatal(err)
			}
			common.WriteSample(t, fmt.Sprintf("sample_%s_thumb.jpg", c.ext),
				thumb)
		})
	}
}
