package captchouli

// #include "thumbnail.h"
import "C"
import (
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
	"golang.org/x/net/html"
)

const (
	// Keys used as names for input elements in captcha form HTML
	IDKey         = common.IDKey
	ColourKey     = common.ColourKey
	BackgroundKey = common.BackgroundKey
)

// Generic error with prefix string
type Error = common.Error

var (
	// No faces detected in downloaded image
	ErrNoFace = Error{fmt.Errorf("no faces detected")}
)

// Init storage and start the runtime
func Open() error {
	return db.Open()
}

// Close open resources
func Close() error {
	classifierMu.Lock()
	C.cpli_unload_classifier(classifier)
	classifier = nil
	classifierMu.Unlock()

	return db.Close()
}

// Extact captcha ID and solution from request
func ExtractSolution(r *http.Request) (solution []byte, err error) {
	err = r.ParseForm()
	if err != nil {
		return
	}

	solution = make([]byte, 0, 4)
	for i := 0; i < 9; i++ {
		s := r.Form.Get(solutionIDs[i])
		if s == "on" {
			solution = append(solution, byte(i))
		}
	}

	return
}

// Decode captcha ID from POST request
func ExtractID(r *http.Request) (id [64]byte, err error) {
	err = r.ParseForm()
	if err != nil {
		return
	}
	return DecodeID(r.Form.Get(common.IDKey))
}

// Decode captcha ID from base64 string
func DecodeID(s string) (id [64]byte, err error) {
	if s == "" {
		err = ErrInvalidID
		return
	}
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return
	}
	if len(buf) != 64 {
		err = ErrInvalidID
		return
	}
	copy(id[:], buf)
	return
}

// Extract captcha from GZipped HTML body and return together with its solution
func ExtractCaptcha(r io.Reader) (id [64]byte, solution []byte, err error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return
	}
	defer gzr.Close()
	doc, err := html.Parse(gzr)
	if err != nil {
		return
	}
	id, err = DecodeID(findID(doc))
	if err != nil {
		return
	}
	solution, err = db.GetSolution(id)
	return
}

func findID(n *html.Node) string {
	getAttr := func(key string) string {
		for _, attr := range n.Attr {
			if attr.Key == key {
				return attr.Val
			}
		}
		return ""
	}

	if n.Type == html.ElementNode && n.Data == "input" {
		if getAttr("name") == IDKey {
			return getAttr("value")
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		found := findID(c)
		if found != "" {
			return found
		}
	}

	return ""
}
