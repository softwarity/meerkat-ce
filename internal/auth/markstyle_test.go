package auth

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// The mark's style belongs to the SHARED chrome, with the markup that needs
// it. It spent a version inside one page's style block: every other page
// rendered it as the browser's default blue link over a background image,
// which is as good as not rendering at all - and it was reported, correctly,
// as "the mark is gone".
func TestTheMarkIsStyledOnEveryPage(t *testing.T) {
	mux, _ := setup(t)
	for _, path := range []string{"/login", "/register", "/forgot-password"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		body, _ := io.ReadAll(rec.Result().Body)
		page := string(body)
		if rec.Code == 404 || !strings.Contains(page, `class="powered"`) {
			continue // this page is closed on a bare installation
		}
		if !strings.Contains(page, ".powered a {") {
			t.Errorf("%s carries the mark with no style for it", path)
		}
	}
	// The sign-in page is always there, so it is the one that must hold.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	if page := string(body); !strings.Contains(page, `class="powered"`) ||
		!strings.Contains(page, ".powered a {") {
		t.Fatal("the sign-in page lost the mark or its style")
	}
}

// The mark's words and address live in ONE place, so changing them is one
// line. A template spelling either of them out would be a second place, found
// only by whoever forgot it.
func TestTheMarkComesFromTheConstants(t *testing.T) {
	mux, _ := setup(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	page := string(body)
	if !strings.Contains(page, MarkText) {
		t.Errorf("the page does not carry MarkText (%q)", MarkText)
	}
	if !strings.Contains(page, `href="`+MarkURL+`"`) {
		t.Errorf("the page does not point at MarkURL (%q)", MarkURL)
	}
}
