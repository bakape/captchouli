package captchouli

import (
	"encoding/base64"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bakape/captchouli/common"
)

func TestCaptcha(t *testing.T) {
	router := newService(t).Router()

	// Create
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	assertCode(t, w, 200)

	// Generate solution
	id, solution, err := ExtractCaptcha(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	data := url.Values{
		common.IDKey: {base64.StdEncoding.EncodeToString(id[:])},
	}
	for _, i := range solution {
		data.Set(solutionIDs[i], "on")
	}
	dataR := strings.NewReader(data.Encode())

	testRequest := func(url string, code int) {
		t.Helper()

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

	// Second captcha should be deleted now
	testRequest("/status", 200)
	if s := w.Body.String(); s != "false" {
		t.Fatal(s)
	}

	// Failure redirect
	testRequest("/", 200)
}

func assertCode(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatal(w.Code)
	}
}
