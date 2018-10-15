package captchouli

import (
	"net/http/httptest"
	"testing"
)

func TestCaptchaHTML(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	err := newService(t).ServeHTTPError(w, r)
	if err != nil {
		t.Fatal(err)
	}
}
