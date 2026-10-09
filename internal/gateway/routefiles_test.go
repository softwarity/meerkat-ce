package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// A files route (ROUTE-22) answers from what was uploaded on it: each file
// under the route's path and its name, the index at the path itself, with the
// type its name says, an ETag a browser revalidates against, and the header
// that lets another origin read a font.
func TestAFilesRouteServesWhatWasUploaded(t *testing.T) {
	rt := newRouter(t, store.Route{
		ID: "fonts", Name: "fonts", Enabled: true, Order: 1,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []string{"/fonts/**"}}}},
		Filters:    []routing.Spec{{Type: "files", Args: map[string]any{"index": "inter.css"}}},
	})
	ctx := context.Background()
	if _, err := rt.st.SetRouteFile(ctx, "fonts", "inter.css", []byte("@font-face{}")); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.st.SetRouteFile(ctx, "fonts", "files/inter.woff2", []byte("wOF2 fake")); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	serve := func(method, path string, h map[string]string) *http.Response {
		req := httptest.NewRequest(method, path, nil)
		for k, v := range h {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		return rec.Result()
	}
	res := serve(http.MethodGet, "/fonts/files/inter.woff2", nil)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || string(body) != "wOF2 fake" || res.Header.Get("Content-Type") != "font/woff2" ||
		res.Header.Get("Access-Control-Allow-Origin") != "*" || res.Header.Get("ETag") == "" {
		t.Fatalf("woff2: %d %q %v", res.StatusCode, body, res.Header)
	}
	if again := serve(http.MethodGet, "/fonts/files/inter.woff2", map[string]string{"If-None-Match": res.Header.Get("ETag")}); again.StatusCode != http.StatusNotModified {
		t.Errorf("a revalidation answered %d, want 304", again.StatusCode)
	}
	if idx := serve(http.MethodGet, "/fonts", nil); idx.StatusCode != 200 || idx.Header.Get("Content-Type") != "text/css; charset=utf-8" {
		t.Errorf("the index: %d %s", idx.StatusCode, idx.Header.Get("Content-Type"))
	}
	if miss := serve(http.MethodGet, "/fonts/nothing.woff2", nil); miss.StatusCode != http.StatusNotFound {
		t.Errorf("an unknown file answered %d", miss.StatusCode)
	}
	if post := serve(http.MethodPost, "/fonts/inter.css", nil); post.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("a POST answered %d", post.StatusCode)
	}
}
