package captchouli

import (
	"compress/gzip"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bakape/captchouli/db"
	"golang.org/x/net/html"
)

func TestCaptcha(t *testing.T) {
	router := newService(t).Router()

	// Create
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	assertCode(t, w, 200)

	// Generate solution
	gzr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer gzr.Close()
	doc, err := html.Parse(gzr)
	if err != nil {
		t.Fatal(err)
	}
	idStr := findID(doc)
	id, err := DecodeID(idStr)
	if err != nil {
		t.Fatal(err)
	}
	solution, err := db.GetSolution(id)
	if err != nil {
		t.Fatal(err)
	}
	data := url.Values{
		"captchouli-id": {idStr},
	}
	for _, i := range solution {
		data.Set(solutionIDs[i], "on")
	}
	dataR := strings.NewReader(data.Encode())

	testRequest := func(url string, code int) {
		dataR.Seek(0, 0)
		r = httptest.NewRequest("POST", url, dataR)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, r)
		assertCode(t, w, code)
	}

	// Solve
	testRequest("/", 200)

	// Check status
	testRequest("/status", 200)
	if s := w.Body.String(); s != "true" {
		t.Fatal(s)
	}

	// Failure redirect
	testRequest("/", 302)
}

func assertCode(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatal(w.Code)
	}
}

func findID(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "input" {
		if getAttr(n, "name") == "captchouli-id" {
			return getAttr(n, "value")
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

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
