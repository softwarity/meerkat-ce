package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// An application written for the root, behind /app/** with strip-prefix: its
// page and its stylesheet name /static/..., which the gateway turns into
// /app/static/... once the route says so - and only then.
func TestARootAppIsServedUnderItsPrefix(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, `<!doctype html><html><head><link rel="stylesheet" href="/static/app.css"></head>`+
				`<body><script src="/static/app.js"></script><a href="docs">docs</a></body></html>`)
		case "/static/app.css":
			w.Header().Set("Content-Type", "text/css")
			_, _ = io.WriteString(w, `body { background: url(/static/bg.png) }`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	serve := func(rootPaths bool) func(path string) string {
		st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })
		ctx := context.Background()
		route := store.Route{ID: "app", Name: "app", Enabled: true, IsUI: true, Upstream: upstream.URL,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/app/**"}}}},
			Filters:    []routing.Spec{{Type: "strip-prefix"}},
			UI:         &store.RouteUI{RootPaths: rootPaths}}
		if err := st.SaveRoute(ctx, route); err != nil {
			t.Fatal(err)
		}
		rt := New(st, session.NewManager(st))
		if err := rt.Reload(ctx); err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(rt)
		t.Cleanup(srv.Close)
		return func(path string) string {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			return string(body)
		}
	}

	get := serve(true)
	page := get("/app/")
	for _, want := range []string{`href="/app/static/app.css"`, `src="/app/static/app.js"`, `href="docs"`, `var P = "/app"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s:\n%s", want, page)
		}
	}
	if css := get("/app/static/app.css"); css != `body { background: url(/app/static/bg.png) }` {
		t.Errorf("stylesheet: %s", css)
	}

	get = serve(false)
	if page := get("/app/"); strings.Contains(page, "/app/static") || strings.Contains(page, "var P =") {
		t.Errorf("switch off, the page was rewritten anyway:\n%s", page)
	}
	if css := get("/app/static/app.css"); css != `body { background: url(/static/bg.png) }` {
		t.Errorf("switch off, the stylesheet was rewritten anyway: %s", css)
	}
}

// The pages of /app asking for /static/... land on the catch-all: noted
// against /app, folded to /static. A call from the same page to another
// application's own route, or to /app itself, is not a stray; and a new
// revision of the route starts clean.
func TestAPageAskingOutsideItsRouteIsNoted(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(upstream.Close)
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	path := func(p string) []routing.Spec {
		return []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{p}}}}
	}
	app := store.Route{ID: "app", Name: "app", Enabled: true, IsUI: true, Upstream: upstream.URL,
		Predicates: path("/app/**"), Filters: []routing.Spec{{Type: "strip-prefix"}}}
	for _, r := range []store.Route{app,
		{ID: "api", Name: "api", Enabled: true, Upstream: upstream.URL, Predicates: path("/api/**")},
		{ID: "trap", Name: "trap", Enabled: true, Upstream: upstream.URL, Predicates: path("/**")},
	} {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	get := func(p, referer string) {
		req, _ := http.NewRequest("GET", srv.URL+p, nil)
		if referer != "" {
			req.Header.Set("Referer", srv.URL+referer)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
	}
	get("/static/app.js", "/app/")
	get("/static/app.css", "/app/")
	get("/spec.json", "/app/page")
	get("/api/data", "/app/")
	get("/app/js/main.js", "/app/")
	get("/other", "")
	got := rt.Strays("app")
	if len(got) != 2 || got[0].Path != "/static" || got[0].Count != 2 || got[0].LandedOn != "trap" ||
		got[0].Example != "/static/app.css" || got[1].Path != "/spec.json" {
		t.Fatalf("strays = %+v", got)
	}
	if s := rt.Strays("trap"); len(s) != 0 {
		t.Fatalf("a route that strips nothing is watched: %+v", s)
	}

	saved, err := st.GetRoute(ctx, "app")
	if err != nil {
		t.Fatal(err)
	}
	saved.UI = &store.RouteUI{RootPaths: true}
	if err := st.SaveRoute(ctx, saved); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if s := rt.Strays("app"); len(s) != 0 {
		t.Fatalf("the strays of the previous revision are still shown: %+v", s)
	}
}
