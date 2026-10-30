package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/store"
)

// withNotFoundPage wires the page the way main does.
func withNotFoundPage(rt *Router) *Router {
	rt.NotFound = auth.New(rt.st, rt.sm).ServeNotFound
	return rt
}

func ask(t *testing.T, rt *Router, method, path, accept string) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return res, string(body)
}

const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

// Where no route answers, a browser gets a page and everything else the plain
// 404 - and both are a 404.
func TestWhereNoRouteAnswers(t *testing.T) {
	rt := withNotFoundPage(newRouter(t))

	t.Run("a browser gets the page", func(t *testing.T) {
		res, body := ask(t, rt, http.MethodGet, "/nowhere", browserAccept)
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, want 404", res.StatusCode)
		}
		if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("Content-Type %q, want a page", ct)
		}
		if !strings.Contains(body, "--mk-primary") || !strings.Contains(body, `<p class="nf-path">/nowhere</p>`) {
			t.Errorf("not the themed page naming the path:\n%.800s", body)
		}
		if strings.Contains(body, "meerkat-portal-nav") {
			t.Error("no portal, and the page carries a bar")
		}
	})

	t.Run("a HEAD from a browser gets its headers", func(t *testing.T) {
		res, body := ask(t, rt, http.MethodHead, "/nowhere", browserAccept)
		if res.StatusCode != http.StatusNotFound || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") || body != "" {
			t.Errorf("HEAD: %d %q, %d bytes", res.StatusCode, res.Header.Get("Content-Type"), len(body))
		}
	})

	for _, c := range []struct{ name, method, accept string }{
		{"an API client", http.MethodGet, "application/json"},
		{"a client that says nothing", http.MethodGet, ""},
		{"anything accepted, not a page asked for", http.MethodGet, "*/*"},
		{"a form posted from a browser", http.MethodPost, browserAccept},
	} {
		t.Run(c.name+" gets the plain 404", func(t *testing.T) {
			res, body := ask(t, rt, c.method, "/nowhere", c.accept)
			if res.StatusCode != http.StatusNotFound {
				t.Fatalf("status %d, want 404", res.StatusCode)
			}
			if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") || body != "404 page not found\n" {
				t.Errorf("not the plain 404: %q %q", ct, body)
			}
		})
	}
}

// A route still wins: the page only answers what no route serves - and a
// route turned off serves nothing.
func TestARouteStillWinsOverThePage(t *testing.T) {
	upstream := htmlUpstream(t, "/anything")
	catchAll := pathRoute("all", "catch-all", 100, "/**", upstream.URL)
	off := pathRoute("off", "off", 1, "/off/**", upstream.URL)
	off.Enabled = false
	rt := withNotFoundPage(newRouter(t, catchAll, off))

	res, body := ask(t, rt, http.MethodGet, "/anything", browserAccept)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "ok") || strings.Contains(body, "nf-path") {
		t.Errorf("the catch-all did not answer: %d\n%.400s", res.StatusCode, body)
	}

	ctx := context.Background()
	if _, err := rt.st.DeleteRoute(ctx, "all"); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	res, body = ask(t, rt, http.MethodGet, "/off/page", browserAccept)
	if res.StatusCode != http.StatusNotFound || !strings.Contains(body, `<p class="nf-path">/off/page</p>`) {
		t.Errorf("a disabled route's path is not the page: %d\n%.400s", res.StatusCode, body)
	}
}

// Behind the portal, the page carries the bar a proxied page of this
// installation carries; on an image without the bar, a portal catalogue
// draws none.
func TestThePageStandsBehindThePortal(t *testing.T) {
	shop := pathRoute("shop", "Shop", 1, "/shop/**", "http://127.0.0.1:1")
	shop.IsUI = true
	// As the console stores it: a list of patterns, which is what the bar
	// reads an entry's address from.
	shop.Predicates[0].Args["patterns"] = []any{"/shop/**"}
	portal := store.PortalConfig{Mode: store.PortalModePortal, Layout: store.PortalHeader, Side: "left",
		Entries: []store.PortalEntry{{RouteID: "shop"}}}

	t.Run("with the bar", func(t *testing.T) {
		standInBar(t)
		rt := withNotFoundPage(routerWithPortal(t, portal, shop))
		res, body := ask(t, rt, http.MethodGet, "/nowhere", browserAccept)
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, want 404", res.StatusCode)
		}
		if !strings.Contains(body, "<meerkat-portal-nav") || !strings.Contains(body, "/meerkat/page.js") {
			t.Errorf("the page does not carry the bar and its agent:\n%.1200s", body)
		}
	})

	t.Run("without it", func(t *testing.T) {
		rt := withNotFoundPage(routerWithPortal(t, portal, shop))
		_, body := ask(t, rt, http.MethodGet, "/nowhere", browserAccept)
		if strings.Contains(body, "meerkat-portal-nav") || !strings.Contains(body, `href="/login"`) {
			t.Errorf("without the bar: no bar, and the page's own way on:\n%.1200s", body)
		}
	})
}
