package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

const pngPixel = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// The background (THEME-06) is the one asset of the flow that can weigh a
// megabyte. It is fetched from an endpoint and never inlined: what rides in the
// page is the rule pointing at it, so a sign-in stays the light page it was and
// a browser pays for the picture once.
func TestBackgroundIsFetchedNotInlined(t *testing.T) {
	// The skin is cached for a few seconds, so the branding is in place before
	// the first page: a second handler would be measuring the cache, not the
	// feature.
	mux, _, st := setupFlow(t)
	b := store.DefaultBranding()
	b.Background = store.Background{Image: pngPixel, Fit: "cover", Dim: 45}
	if err := st.SetSetting(context.Background(), store.SettingBranding, b); err != nil {
		t.Fatal(err)
	}

	body := get(t, mux, "/login", nil).Body.String()
	if !strings.Contains(body, `url("`+backgroundPath+`")`) {
		t.Errorf("the page should point at the background endpoint:\n%.600s", body)
	}
	if strings.Contains(body, "iVBORw0KGgo") {
		t.Error("the image itself must not ride in the page")
	}

	img := get(t, mux, backgroundPath, nil)
	if img.Code != http.StatusOK {
		t.Fatalf("the background answered %d", img.Code)
	}
	if ct := img.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content type is %q", ct)
	}
	etag := img.Header().Get("ETag")
	if etag == "" {
		t.Fatal("a background without an ETag makes every page view re-download it")
	}
	again := get(t, mux, backgroundPath, http.Header{"If-None-Match": {etag}})
	if again.Code != http.StatusNotModified {
		t.Errorf("a known ETag should answer 304, got %d", again.Code)
	}
}

// Nothing set: the page is the page it always was, and the endpoint has
// nothing to serve.
func TestNoBackgroundNoRule(t *testing.T) {
	mux, _, _ := setupFlow(t)
	if body := get(t, mux, "/login", nil).Body.String(); strings.Contains(body, "body::before") {
		t.Error("a page without a background must carry no background rule")
	}
	if code := get(t, mux, backgroundPath, nil).Code; code != http.StatusNotFound {
		t.Errorf("no background should answer 404, got %d", code)
	}
}

func get(t *testing.T, mux *http.ServeMux, path string, h http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range h {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// The dashed placeholder is a note to the integrator - "a logo goes here". It
// earns its place on a page with nothing else to look at; over a picture it is
// a dashed square telling a visitor the page is unfinished, while the picture
// already says whose application this is.
func TestBackgroundReplacesTheLogoPlaceholder(t *testing.T) {
	mux, _, st := setupFlow(t)
	b := store.DefaultBranding()
	b.Background = store.Background{Image: pngPixel, Fit: "cover", Dim: 35}
	if err := st.SetSetting(context.Background(), store.SettingBranding, b); err != nil {
		t.Fatal(err)
	}
	body := get(t, mux, "/login", nil).Body.String()
	if !strings.Contains(body, `class="generic" style="display:none"`) {
		t.Errorf("the placeholder should stand down for a background:\n%.900s", body)
	}
}

// Without one, it stays: a page with no logo and no picture would otherwise
// open on a bare word.
func TestPlaceholderStaysWithoutABackground(t *testing.T) {
	mux, _, _ := setupFlow(t)
	body := get(t, mux, "/login", nil).Body.String()
	if !strings.Contains(body, `class="generic"`) || strings.Contains(body, `class="generic" style="display:none"`) {
		t.Errorf("the placeholder should be visible when nothing else is:\n%.900s", body)
	}
}
