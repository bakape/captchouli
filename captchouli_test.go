package captchouli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

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
			writeSample(t, fmt.Sprintf("sample_%s_thumb.jpg", c.ext), thumb)
		})
	}
}

func writeSample(t *testing.T, name string, buf []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)

	// Remove previous file, if any
	_, err := os.Stat(path)
	switch {
	case os.IsExist(err):
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	case os.IsNotExist(err):
	case err == nil:
	default:
		t.Fatal(err)
	}

	err = ioutil.WriteFile(path, buf, 0600)
	if err != nil {
		t.Fatal(err)
	}
}
