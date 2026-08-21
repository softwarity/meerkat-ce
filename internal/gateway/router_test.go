package gateway

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

func pathRoute(id, name string, order int, pattern, upstream string, filters ...routing.Spec) store.Route {
	return store.Route{
		ID: id, Name: name, Order: order, Enabled: true, Upstream: upstream,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": pattern}}},
		Filters:    filters,
	}
}

func newRouter(t *testing.T, routes ...store.Route) *Router {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for _, r := range routes {
		if err := st.SaveRoute(context.Background(), r); err != nil {
			t.Fatalf("SaveRoute: %v", err)
		}
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	return rt
}

func get(t *testing.T, rt *Router, path string) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res, string(body)
}

func htmlUpstream(t *testing.T, wantPath string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			http.NotFound(w, r)
			return
		}
		if xf := r.Header.Get("X-Forwarded-For"); xf == "" {
			t.Error("missing X-Forwarded-For on upstream request")
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><head></head><body>ok</body></html>`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProxiesWithStripPrefixAndInjection(t *testing.T) {
	upstream := htmlUpstream(t, "/page")
	r := pathRoute("r1", "demo", 1, "/demo/**", upstream.URL,
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}},
	)
	// Injection rides the UI options now (the generic inject-head filter is gone).
	r.IsUI = true
	r.UI = &store.RouteUI{CustomJS: "meerkat()"}
	rt := newRouter(t, r)
	res, body := get(t, rt, "/demo/page")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	// Every UI page also carries the agent: it watches the session, so no
	// route opts out of it.
	want := "<html><head><script>\nmeerkat()\n</script></head><body>" +
		`<script defer src="/meerkat/page.js"></script>` + "ok</body></html>"
	if body != want {
		t.Fatalf("got %q, want %q", body, want)
	}
}

func TestUpstreamBasePathComposesWithStrip(t *testing.T) {
	upstream := htmlUpstream(t, "/base/page")
	// Upstream carries a base path: strip works on the REQUEST path, then the
	// base path is prepended - /demo/page -> /page -> /base/page.
	rt := newRouter(t, pathRoute("r1", "demo", 1, "/demo/**", upstream.URL+"/base",
		routing.Spec{Type: "strip-prefix", Args: map[string]any{"parts": 1}},
	))
	if res, _ := get(t, rt, "/demo/page"); res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestFirstMatchWinsInOrder(t *testing.T) {
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "A")
	}))
	t.Cleanup(a.Close)
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "B")
	}))
	t.Cleanup(b.Close)

	rt := newRouter(t,
		pathRoute("r2", "wide", 2, "/**", b.URL),
		pathRoute("r1", "narrow", 1, "/api/**", a.URL),
	)
	if _, body := get(t, rt, "/api/x"); body != "A" {
		t.Fatalf("/api/x routed to %q, want A", body)
	}
	if _, body := get(t, rt, "/other"); body != "B" {
		t.Fatalf("/other routed to %q, want B", body)
	}
}

func TestMultiplePredicatesAreANDed(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hit")
	}))
	t.Cleanup(up.Close)
	r := pathRoute("r1", "and", 1, "/api/**", up.URL)
	r.Predicates = append(r.Predicates, routing.Spec{Type: "method", Args: map[string]any{"methods": "POST"}})
	rt := newRouter(t, r)

	if res, _ := get(t, rt, "/api/x"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("GET matched a POST-only route: %d", res.StatusCode)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/api/x", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST should match: %d", res.StatusCode)
	}
}

func TestCanaryWeightsEndToEnd(t *testing.T) {
	stable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "stable")
	}))
	t.Cleanup(stable.Close)
	canary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "canary")
	}))
	t.Cleanup(canary.Close)

	rStable := pathRoute("r1", "stable", 1, "/app/**", stable.URL)
	rStable.Predicates = append(rStable.Predicates, routing.Spec{Type: "weight", Args: map[string]any{"group": "app", "weight": 9}})
	rCanary := pathRoute("r2", "canary", 2, "/app/**", canary.URL)
	rCanary.Predicates = append(rCanary.Predicates, routing.Spec{Type: "weight", Args: map[string]any{"group": "app", "weight": 1}})

	rt := newRouter(t, rStable, rCanary)
	draws := []float64{0.05, 0.5, 0.89, 0.91, 0.99}
	i := 0
	rt.lottery = func() float64 { v := draws[i%len(draws)]; i++; return v }

	got := make([]string, 0, len(draws))
	for range draws {
		_, body := get(t, rt, "/app/x")
		got = append(got, body)
	}
	want := []string{"stable", "stable", "stable", "canary", "canary"}
	for j := range want {
		if got[j] != want[j] {
			t.Fatalf("draw %v routed to %q, want %q (all: %v)", draws[j], got[j], want[j], got)
		}
	}
}

func TestRedirectRoute(t *testing.T) {
	r := store.Route{
		ID: "r1", Name: "moved", Order: 1, Enabled: true, Upstream: "http://unused.invalid",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": "/old/**"}}},
		Filters:    []routing.Spec{{Type: "redirect", Args: map[string]any{"location": "/new", "status": 301}}},
	}
	rt := newRouter(t, r)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := noRedirect.Get(srv.URL + "/old/x")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != 301 || res.Header.Get("Location") != "/new" {
		t.Fatalf("redirect: %d %q", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestResponseHeaderFilters(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "leaky")
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(up.Close)
	rt := newRouter(t, pathRoute("r1", "hardened", 1, "/**", up.URL,
		routing.Spec{Type: "remove-response-header", Args: map[string]any{"name": "Server"}},
		routing.Spec{Type: "set-response-header", Args: map[string]any{"name": "X-Frame-Options", "value": "DENY"}},
	))
	res, _ := get(t, rt, "/x")
	if res.Header.Get("Server") != "" || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("response headers wrong: %+v", res.Header)
	}
}

func TestInvalidRouteAbortsReloadKeepingOldSnapshot(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(up.Close)
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SaveRoute(ctx, pathRoute("r1", "good", 1, "/**", up.URL)); err != nil {
		t.Fatal(err)
	}
	rt := New(st, nil)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	// A broken route arrives: reload must fail loudly...
	if err := st.SaveRoute(ctx, store.Route{
		ID: "r2", Name: "broken", Order: 2, Enabled: true, Upstream: up.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": "/a/**/b"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err == nil || !strings.Contains(err.Error(), `route "broken"`) {
		t.Fatalf("reload error = %v, want route name in it", err)
	}
	// ...and the previous snapshot keeps serving.
	if res, _ := get(t, rt, "/x"); res.StatusCode != http.StatusOK {
		t.Fatalf("old snapshot gone: %d", res.StatusCode)
	}
}

func TestAuthenticatedRouteGating(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "secret")
	}))
	t.Cleanup(upstream.Close)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "admin", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	secure := pathRoute("r1", "secure", 1, "/secure/**", upstream.URL)
	secure.Access = store.Access{Level: store.AccessAuth}
	if err := st.SaveRoute(ctx, secure); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	sm := session.NewManager(st)
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest("GET", srv.URL+"/secure/x?a=1", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/login?next=%2Fsecure%2Fx%3Fa%3D1" {
		t.Fatalf("anonymous HTML: %d %q", res.StatusCode, res.Header.Get("Location"))
	}

	res, err = http.Get(srv.URL + "/secure/api")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous API: %d, want 401", res.StatusCode)
	}

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	req, _ = http.NewRequest("GET", srv.URL+"/secure/x", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "secret" {
		t.Fatalf("authenticated: %d %q", res.StatusCode, body)
	}
}

func TestUpstreamDownIs502(t *testing.T) {
	rt := newRouter(t, pathRoute("r1", "down", 1, "/down/**", "http://127.0.0.1:1"))
	res, body := get(t, rt, "/down")
	if res.StatusCode != http.StatusBadGateway || !strings.Contains(body, "upstream unavailable") {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
}

func TestReloadPicksUpChanges(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "up")
	}))
	t.Cleanup(up.Close)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rt := New(st, nil)
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/new")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("before reload: %d, want 404", res.StatusCode)
	}
	if err := st.SaveRoute(context.Background(), pathRoute("r1", "new", 1, "/new/**", up.URL)); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	if err := rt.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	res, err = http.Get(srv.URL + "/new")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "up" {
		t.Fatalf("after reload: %d %q", res.StatusCode, body)
	}
}

// TestCatchAllRouteTraps: the TRAP (ROUTE-10) is an ordinary route - a "/**"
// catch-all ordered LAST. Whatever the routes above did not match lands on it,
// "/" included, any method; without such a route, unmatched is a plain 404.
func TestCatchAllRouteTraps(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("trapped"))
	}))
	t.Cleanup(up.Close)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SaveRoute(ctx, pathRoute("r1", "demo", 1, "/demo/**", up.URL+"/unused")); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveRoute(ctx, pathRoute("trap", "trap", 2, "/**", up.URL)); err != nil {
		t.Fatal(err)
	}
	rt := New(st, nil)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	for _, req := range []*http.Request{
		httptest.NewRequest("GET", "/", nil),
		httptest.NewRequest("POST", "/nope", nil),
	} {
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "trapped" {
			t.Fatalf("%s %s: %d %q, want the catch-all to trap it", req.Method, req.URL.Path, rec.Code, rec.Body.String())
		}
	}
}

// TestUpstreamHangIs502: an upstream that never answers its headers must fail
// fast as 502 thanks to the transport's ResponseHeaderTimeout (ROUTE-07).
func TestUpstreamHangIs502(t *testing.T) {
	prev := upstreamTransport
	upstreamTransport = &http.Transport{ResponseHeaderTimeout: 100 * time.Millisecond}
	t.Cleanup(func() { upstreamTransport = prev })

	release := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release // hang until the test ends
	}))
	t.Cleanup(func() { close(release); up.Close() })

	rt := newRouter(t, pathRoute("r1", "hang", 1, "/**", up.URL))

	start := time.Now()
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("hanging upstream: %d, want 502", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("502 took %v - the timeout did not bound the wait", elapsed)
	}
}

// resolveLocale: cookie first, then Accept-Language (exact then base), then
// the route's first code.
func TestResolveLocale(t *testing.T) {
	codes := []string{"en-US", "fr-FR", "de"}
	mk := func(cookie, accept string) *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "MEERKAT_LANG", Value: cookie})
		}
		if accept != "" {
			req.Header.Set("Accept-Language", accept)
		}
		return req
	}
	cases := []struct{ cookie, accept, want string }{
		{"fr-FR", "", "fr-FR"},
		{"fr", "", "fr-FR"},      // base-language cookie picks the region variant
		{"es", "de;q=0.9", "de"}, // unknown cookie falls to Accept-Language
		{"", "fr-CA,en;q=0.8", "fr-FR"},
		{"", "es,pt", "en-US"}, // nothing matches: first code
		{"", "", "en-US"},
	}
	for _, c := range cases {
		if got := resolveLocale(mk(c.cookie, c.accept), codes); got != c.want {
			t.Errorf("resolveLocale(cookie=%q accept=%q) = %q, want %q", c.cookie, c.accept, got, c.want)
		}
	}
}

// Locale mechanisms: custom/query/path only (Accept-Language always goes),
// path demands a UI route, names are validated.
func TestValidateLocales(t *testing.T) {
	mk := func(isUI bool, mech, header, param string) store.Route {
		return store.Route{IsUI: isUI, Upstream: "http://up",
			Locales: &store.LocalesConfig{Mechanism: mech, Header: header, Param: param}}
	}
	if err := Validate(mk(true, "query", "", "lg")); err != nil {
		t.Fatalf("valid locales refused: %v", err)
	}
	if err := Validate(mk(false, "query", "", "")); err != nil {
		t.Fatalf("query mechanism on an API refused: %v", err)
	}
	if err := Validate(mk(false, "path", "", "")); err == nil {
		t.Fatal("path mechanism accepted on a non-UI route")
	}
	if err := Validate(mk(true, "header", "", "")); err == nil {
		t.Fatal("header is not a mechanism anymore (always sent)")
	}
	if err := Validate(mk(true, "custom", "X Locale", "")); err == nil {
		t.Fatal("bad custom header accepted")
	}
	// The script is a mechanism like the others - it carries the change to the
	// PAGE - so it has to pass validation, and it only makes sense where there
	// is a page.
	if err := Validate(mk(true, store.LocaleScript, "", "")); err != nil {
		t.Fatalf("the script mechanism was refused on a UI route: %v", err)
	}
	if err := Validate(mk(false, store.LocaleScript, "", "")); err == nil {
		t.Fatal("the script mechanism was accepted on a route that serves no page")
	}
}

// Accept-Language promotion: the resolved locale lands first, the caller's
// other preferences stay behind, duplicates are dropped.
func TestPromoteLocale(t *testing.T) {
	cases := []struct{ orig, loc, want string }{
		{"", "fr", "fr"},
		{"en-US,en;q=0.8", "fr", "fr, en-US, en;q=0.8"},
		{"fr;q=0.5,en;q=0.8", "fr", "fr, en;q=0.8"},
		{"de", "de", "de"},
	}
	for _, c := range cases {
		if got := promoteLocale(c.orig, c.loc); got != c.want {
			t.Errorf("promoteLocale(%q, %q) = %q, want %q", c.orig, c.loc, got, c.want)
		}
	}
}

// Identity forwarding validates its mechanism, attributes and mapped names.
func TestValidateIdentity(t *testing.T) {
	mk := func(mech string, attrs ...store.IdentityAttr) store.Route {
		return store.Route{Upstream: "http://up",
			Identity: &store.IdentityForward{Mechanism: mech, Attributes: attrs}}
	}
	if err := Validate(mk("headers", store.IdentityAttr{Field: "username", As: "X-User"})); err != nil {
		t.Fatalf("valid headers identity refused: %v", err)
	}
	if err := Validate(mk("jwt", store.IdentityAttr{Field: "roles", As: "role_list", AsJSON: true})); err != nil {
		t.Fatalf("valid jwt identity refused: %v", err)
	}
	if err := Validate(mk("signed-jwt", store.IdentityAttr{Field: "username"})); err != nil {
		t.Fatalf("valid signed-jwt refused: %v", err)
	}
	badAlg := mk("signed-jwt", store.IdentityAttr{Field: "username"})
	badAlg.Identity.Algorithm = "HS256"
	if err := Validate(badAlg); err == nil {
		t.Fatal("bad signature algorithm accepted")
	}
	if err := Validate(mk("carrier-pigeon")); err == nil {
		t.Fatal("unknown mechanism accepted")
	}
	if err := Validate(mk("headers", store.IdentityAttr{Field: "shoesize"})); err == nil {
		t.Fatal("unknown identity attribute accepted")
	}
	if err := Validate(mk("headers", store.IdentityAttr{Field: "email", As: "not a header"})); err == nil {
		t.Fatal("bad header name accepted")
	}
	if err := Validate(mk("jwt", store.IdentityAttr{Field: "email", As: "bad claim"})); err == nil {
		t.Fatal("bad claim name accepted")
	}
	if err := Validate(mk("headers", store.IdentityAttr{Field: "username"}, store.IdentityAttr{Field: "username"})); err == nil {
		t.Fatal("duplicate attribute accepted")
	}
	bad := mk("jwt", store.IdentityAttr{Field: "username"})
	bad.Identity.TTL = "banana"
	if err := Validate(bad); err == nil {
		t.Fatal("bad ttl accepted")
	}
}

// Identity forwarding rides EVERY request of the route, HTML navigations and
// API calls alike (a UI route's page and its /api calls share the pipe):
// headers reach the upstream, spoofed inbound values are purged, anonymous
// requests carry nothing.
func TestIdentityForwardHeadersReachUpstream(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "neo", Email: "neo@io",
		Timezone: "Europe/Paris", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "user="+r.Header.Get("username")+
			";remote="+r.Header.Get(RemoteUserHeader)+
			";mail="+r.Header.Get("X-Mail")+
			";tenant="+r.Header.Get("tenant"))
	}))
	t.Cleanup(upstream.Close)

	route := store.Route{ID: "r", Name: "r", Enabled: true, Upstream: upstream.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
		Identity: &store.IdentityForward{Mechanism: "headers", Attributes: []store.IdentityAttr{
			{Field: "username"}, {Field: "email", As: "X-Mail"},
		}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookie := rec.Result().Cookies()[0]

	call := func(withSession bool) string {
		req, _ := http.NewRequest("GET", srv.URL+"/get", nil)
		// Spoofing attempts: the gateway must purge these.
		req.Header.Set("username", "hacker")
		req.Header.Set("tenant", "EvilCorp")
		if withSession {
			req.AddCookie(cookie)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return string(body)
	}

	// Only the selected attributes travel: username (default name) and email
	// (mapped to X-Mail); the unselected tenant is purged (the inbound spoof is
	// dropped and nothing is written back). The username ALSO travels as
	// REMOTE_USER, which is the header applications already read.
	if got := call(true); got != "user=neo;remote=neo;mail=neo@io;tenant=" {
		t.Fatalf("signed-in forwarding wrong: %q", got)
	}
	if got := call(false); got != "user=;remote=;mail=;tenant=" {
		t.Fatalf("anonymous request must carry no identity: %q", got)
	}
}

// TestIdentityForwardJWT: the jwt mechanism carries the selected attributes as
// claims in an Authorization: Bearer token, purges an inbound Authorization
// spoof, and writes nothing for an anonymous caller.
func TestIdentityForwardJWT(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "neo", Email: "neo@io",
		Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("Authorization"))
	}))
	t.Cleanup(upstream.Close)

	route := store.Route{ID: "r", Name: "orders", Enabled: true, Upstream: upstream.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
		Identity: &store.IdentityForward{Mechanism: "jwt", Attributes: []store.IdentityAttr{
			{Field: "username", As: "preferred_username"},
		}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookie := rec.Result().Cookies()[0]

	req, _ := http.NewRequest("GET", srv.URL+"/get", nil)
	req.Header.Set("Authorization", "Bearer spoofed") // must be purged
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	got := string(body)
	if !strings.HasPrefix(got, "Bearer ") {
		t.Fatalf("no bearer token forwarded: %q", got)
	}
	parts := strings.Split(strings.TrimPrefix(got, "Bearer "), ".")
	if len(parts) != 3 || parts[2] != "" {
		t.Fatalf("not an unsigned JWT (header.payload.): %q", got)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("payload not json: %v", err)
	}
	if claims["iss"] != "meerkat" || claims["sub"] != "u1" || claims["aud"] != "orders" {
		t.Fatalf("registered claims wrong: %+v", claims)
	}
	if claims["preferred_username"] != "neo" {
		t.Fatalf("mapped attribute claim wrong: %+v", claims)
	}
	if _, ok := claims["exp"]; !ok {
		t.Fatalf("missing exp claim: %+v", claims)
	}

	// Anonymous: no token at all, and the inbound spoof is gone.
	anon, _ := http.NewRequest("GET", srv.URL+"/get", nil)
	anon.Header.Set("Authorization", "Bearer spoofed")
	res2, err := http.DefaultClient.Do(anon)
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := io.ReadAll(res2.Body)
	_ = res2.Body.Close()
	if strings.TrimSpace(string(body2)) != "" {
		t.Fatalf("anonymous request must carry no Authorization: %q", string(body2))
	}
}

// TestIdentityForwardSignedJWT proves the whole signed-jwt chain: the route
// forwards a token the gateway signed (ES256), and that token verifies against
// the very key the gateway publishes at /.well-known/jwks.json.
func TestIdentityForwardSignedJWT(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "neo", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(st)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("Authorization"))
	}))
	t.Cleanup(upstream.Close)

	route := store.Route{ID: "r", Name: "orders", Enabled: true, Upstream: upstream.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
		Identity: &store.IdentityForward{Mechanism: "signed-jwt", Algorithm: "ES256",
			Attributes: []store.IdentityAttr{{Field: "username", As: "preferred_username"}}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	// The JWKS is public and carries an ES256 EC key.
	pub, kid := fetchES256JWK(t, srv.URL+JWKSPath)

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	cookie := rec.Result().Cookies()[0]
	req, _ := http.NewRequest("GET", srv.URL+"/get", nil)
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	token := strings.TrimPrefix(string(body), "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[2] == "" {
		t.Fatalf("expected a signed JWT (three non-empty segments): %q", string(body))
	}
	var header map[string]any
	if err := json.Unmarshal(mustB64(t, parts[0]), &header); err != nil {
		t.Fatalf("header: %v", err)
	}
	if header["alg"] != "ES256" || header["kid"] != kid {
		t.Fatalf("header alg/kid wrong: %+v (jwks kid %q)", header, kid)
	}
	// The signature verifies against the JWKS-published key.
	h := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig := mustB64(t, parts[2])
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pub, h[:], r, s) {
		t.Fatal("forwarded token does not verify against the published JWKS key")
	}
	var claims map[string]any
	if err := json.Unmarshal(mustB64(t, parts[1]), &claims); err != nil {
		t.Fatalf("claims: %v", err)
	}
	if claims["sub"] != "u1" || claims["aud"] != "orders" || claims["preferred_username"] != "neo" {
		t.Fatalf("claims wrong: %+v", claims)
	}
}

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("base64url %q: %v", s, err)
	}
	return b
}

// fetchES256JWK pulls the JWKS and rebuilds the ES256 EC public key.
func fetchES256JWK(t *testing.T, url string) (*ecdsa.PublicKey, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET jwks: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	var doc struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatalf("jwks decode: %v", err)
	}
	for _, k := range doc.Keys {
		if k["alg"] != "ES256" {
			continue
		}
		// Through the parser, not by filling in X and Y: those fields are
		// deprecated as of Go 1.26, and a test that builds a key the way
		// nothing else may is a test that stops resembling the code.
		key, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(),
			uncompressed(mustB64(t, k["x"]), mustB64(t, k["y"])))
		if err != nil {
			t.Fatalf("jwks EC key: %v", err)
		}
		return key, k["kid"]
	}
	t.Fatal("no ES256 key in JWKS")
	return nil, ""
}

// uncompressed lays x and y out as an uncompressed EC point, left-padded to
// the field size - JWK strips leading zeros, an EC point is fixed-width.
func uncompressed(x, y []byte) []byte {
	const size = 32 // P-256
	point := make([]byte, 1+2*size)
	point[0] = 4
	copy(point[1+size-len(x):], x)
	copy(point[1+2*size-len(y):], y)
	return point
}

// Outgoing filters apply to what the route answers ITSELF. An identity endpoint
// built with respond is called by a browser like any other, so it needs the
// same CORS or Cache-Control headers a proxied one would - telling admins that
// outgoing filters work everywhere except here would be arbitrary.
func TestOutgoingFiltersApplyToTheRoutesOwnAnswer(t *testing.T) {
	rt := newRouter(t, store.Route{
		ID: "own", Name: "own", Enabled: true, Order: 1,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []string{"/who"}}}},
		Filters: []routing.Spec{
			{Type: "respond", Args: map[string]any{"body": `{"ok":true}`}},
			// Incoming: nothing is proxied, so it has nothing to modify.
			{Type: "strip-prefix", Args: map[string]any{"parts": 1}},
			// Outgoing: must land on the answer.
			{Type: "set-response-header", Args: map[string]any{
				"name": "Access-Control-Allow-Origin", "value": "https://app.example",
			}},
		},
	})

	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/who", nil))
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("the outgoing filter did not apply: %q", got)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != `{"ok":true}` {
		t.Fatalf("body mangled by the filter pass: %q", body)
	}
	if res.Header.Get("Content-Type") == "" {
		t.Fatal("the terminal's own headers must survive the filter pass")
	}
}

// Being added to an organisation must take effect on the session already open.
//
// Roles are held IN an organisation (RBAC-06), and a session opened before the
// membership existed carries none - so an administrator could tick every role
// into a group, put the account in it, and see the upstream receive no role at
// all, with nothing anywhere saying why. Sign-in adopts the only membership
// when there is exactly one; so does this, afterwards.
func TestJoiningTakesEffectOnAnOpenSession(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "alice", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// A session with NO organisation on it: what someone signed in before
	// anyone put them in one is holding.
	sm := session.NewManager(st)
	cookie := issueSession(t, sm, "u1")

	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("roles"))
	}))
	t.Cleanup(echo.Close)

	rt := New(st, sm)
	if err := st.SaveRoute(ctx, store.Route{
		ID: "r", Name: "r", Enabled: true, Upstream: echo.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []string{"/**"}}}},
		Identity: &store.IdentityForward{Mechanism: "headers",
			Attributes: []store.IdentityAttr{{Field: "roles", As: "roles"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	call := func() string {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		body, _ := io.ReadAll(rec.Result().Body)
		return string(body)
	}

	if got := call(); got != "" {
		t.Fatalf("no organisation, no role: got %q", got)
	}

	// The administrator now does what an administrator does.
	if err := st.SaveTenant(ctx, store.Tenant{ID: "t1", Name: "Acme", Enabled: true,
		BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMembership(ctx, store.Membership{UserID: "u1", TenantID: "t1",
		Type: store.MemberUser, Enabled: true,
		BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveRole(ctx, store.Role{ID: "ROLE_A", Name: "ROLE_A"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveGroup(ctx, store.Group{ID: "g1", TenantID: "t1", Name: "ROOT",
		RoleIDs: []string{"ROLE_A"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMemberGroups(ctx, "t1", "u1", []string{"g1"}); err != nil {
		t.Fatal(err)
	}

	// Same session, no sign-out.
	if got := call(); got != "ROLE_A" {
		t.Fatalf("the open session must pick the membership up: got %q", got)
	}
}

// One expression says what a route sends: which roles, under what names, in
// what shape. It reads as a pipeline, in the order it happens.
//
// The reason it exists is weight: 320 roles is 9 KB of header on every request
// for a service that uses three, and some upstreams refuse a header that size.
func TestRoleExpressionShapesWhatTravels(t *testing.T) {
	tags := map[string][]string{
		"ROLE_APP_BILLING":   {"BILLING"},
		"ROLE_BILLING_USER":  {"BILLING"},
		"ROLE_APP_REPORTING": {"REPORTING"},
		"ROLE_CONFIGURATOR":  {"OTHER"},
		"ROLE_PLOTTING":      {"OTHER"},
	}
	held := identityData{
		Roles:      []string{"ROLE_APP_BILLING", "ROLE_BILLING_USER", "ROLE_APP_REPORTING", "ROLE_CONFIGURATOR"},
		TagsOfRole: tags,
	}

	for _, tc := range []struct {
		name, expr, want string
	}{
		{"nothing written sends the plain list", "",
			"ROLE_APP_BILLING,ROLE_BILLING_USER,ROLE_APP_REPORTING,ROLE_CONFIGURATOR"},
		{"a JSON array", `{{json .Roles}}`,
			`["ROLE_APP_BILLING","ROLE_BILLING_USER","ROLE_APP_REPORTING","ROLE_CONFIGURATOR"]`},
		{"narrowed to a tag", `{{.Roles | .Keep "tag:BILLING" | join ","}}`,
			"ROLE_APP_BILLING,ROLE_BILLING_USER"},
		// A tag gathers roles no name pattern would - which is why the
		// selector is a tag.
		{"a tag plus one named role", `{{.Roles | .Keep "tag:OTHER" "ROLE_APP_REPORTING" | join ","}}`,
			"ROLE_APP_REPORTING,ROLE_CONFIGURATOR"},
		{"everything except a tag", `{{.Roles | .Drop "tag:BILLING" | join ","}}`,
			"ROLE_APP_REPORTING,ROLE_CONFIGURATOR"},
		{"the whole pipeline", `{{.Roles | .Keep "tag:BILLING" | cutHead "ROLE_" | join ","}}`,
			"APP_BILLING,BILLING_USER"},
		{"a head and a tail of one's own", `{{.Roles | .Keep "tag:REPORTING" | cutHead "ROLE_" | addHead "app:" | addTail ":ro" | join ";"}}`,
			"app:APP_REPORTING:ro"},
		{"lower case, as a JSON array", `{{.Roles | .Keep "tag:REPORTING" | lower | json}}`,
			`["role_app_reporting"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := identityHeaderPairs(store.IdentityForward{
				Mechanism:  "headers",
				Attributes: []store.IdentityAttr{{Field: "roles", As: "roles", Expr: tc.expr}},
			}, held)
			if len(got) != 1 || got[0].Value != tc.want {
				t.Fatalf("got %+v, want %q", got, tc.want)
			}
		})
	}
}

// An expression is proven to run when the ROUTE IS SAVED. A verb that does not
// exist parses fine and would otherwise fail on the first request that needs
// it - on the header every request carries.
func TestRoleExpressionIsCheckedAtSaveTime(t *testing.T) {
	for _, tc := range []struct{ name, expr, wants string }{
		{"unknown verb", `{{.Roles | shout}}`, "shout"},
		{"unclosed", `{{.Roles | join ","`, "unclosed action"},
		{"a line break would forge a second header", "{{join \"\\n\" .Roles}}", "line break"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := routing.CompileRoleExpr(tc.expr); err == nil {
				t.Fatal("accepted an expression that cannot work")
			} else if !strings.Contains(err.Error(), tc.wants) {
				t.Fatalf("the error must name what is wrong (%q): %v", tc.wants, err)
			}
		})
	}
}

// A route may say what happens when someone picks a language, instead of the
// reload the button does by default (I18N-04). The script lands as a FUNCTION
// on the window - code belongs in a <script>, not quoted twice over in an HTML
// attribute - and the button looks it up by the one name both ends share.
func TestLocaleHookIsInjectedForUIRoutes(t *testing.T) {
	up := htmlUpstream(t, "/app")
	r := pathRoute("ui", "ui", 1, "/app/**", up.URL)
	r.IsUI = true
	r.Locales = &store.LocalesConfig{
		Mechanism: store.LocaleScript,
		OnChange:  "myApp.setLocale(locale);",
	}
	rt := newRouter(t, r)

	_, body := get(t, rt, "/app")
	if !strings.Contains(body, "window."+store.LocaleHookName+" = function (locale)") {
		t.Fatalf("the hook did not reach the page:\n%s", body)
	}
	if !strings.Contains(body, "myApp.setLocale(locale);") {
		t.Errorf("the route's own script is missing:\n%s", body)
	}

	// Nothing declared, nothing injected: the button reloads, as it always has.
	plain := pathRoute("ui2", "ui2", 1, "/app/**", up.URL)
	plain.IsUI = true
	_, body = get(t, newRouter(t, plain), "/app")
	if strings.Contains(body, store.LocaleHookName) {
		t.Errorf("a route that said nothing got a hook:\n%s", body)
	}

	// A script kept but the mechanism unticked: nothing runs. The list of
	// mechanisms is the switch, so untelling it does not cost the script -
	// ticking it again brings back what was written.
	off := pathRoute("ui3", "ui3", 1, "/app/**", up.URL)
	off.IsUI = true
	off.Locales = &store.LocalesConfig{OnChange: "myApp.setLocale(locale);"}
	_, body = get(t, newRouter(t, off), "/app")
	if strings.Contains(body, store.LocaleHookName) {
		t.Errorf("an unticked mechanism still injected its script:\n%s", body)
	}
}

// Where the language goes in the path, which is the whole question: after what
// the route matched on. The upstream serves an application that lives under
// the route's prefix, so /app/fr/route is a page it has and /fr/app/route is
// not. With a strip-prefix the head is already gone and the same rule puts the
// locale in front of what remains.
func TestLocalePathInsertion(t *testing.T) {
	prefixes := []string{"/app"}
	codes := []string{"en", "fr", "zh-Hans"}
	cases := []struct{ path, want string }{
		{"/app", "/app/fr"},
		{"/app/", "/app/fr/"},
		{"/app/route", "/app/fr/route"},
		{"/app/route/deep", "/app/fr/route/deep"},
		// Already carrying a locale at that spot: someone else drives it.
		{"/app/en/route", "/app/en/route"},
		{"/app/zh-Hans", "/app/zh-Hans"},
		// A segment that merely starts like one is not one.
		{"/app/energy", "/app/fr/energy"},
		// Post strip-prefix: the head is gone, so the locale leads.
		{"/route", "/fr/route"},
		{"/", "/fr/"},
		// A locale deeper in the path is not the locale segment.
		{"/app/docs/en/entry", "/app/fr/docs/en/entry"},
	}
	for _, c := range cases {
		if got := insertLocale(c.path, "fr", codes, prefixes); got != c.want {
			t.Errorf("insertLocale(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// The prefix comes from the predicate, and stops at the first wildcard.
func TestPathPrefixesFromPredicates(t *testing.T) {
	specs := []routing.Spec{
		{Type: "path", Args: map[string]any{"patterns": []any{"/app/**", "/a/{id}/b", "/**"}}},
		{Type: "host", Args: map[string]any{"hosts": []any{"example.test"}}},
	}
	got := routing.PathPrefixes(specs)
	want := []string{"/app", "/a"}
	if len(got) != len(want) {
		t.Fatalf("PathPrefixes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PathPrefixes = %v, want %v", got, want)
		}
	}
}

// The path mechanism has a browser half: where the language lives in the URL,
// the page agent must know WHERE the segment sits to navigate to it. Without
// this the menu reloads the same localized URL for ever, which is what an
// application built per locale hands it back.
func TestPageAgentCarriesTheLocalePath(t *testing.T) {
	r := pathRoute("ui", "ui", 1, "/fpl-ui/**", "http://up")
	r.IsUI = true
	r.UI = &store.RouteUI{UserButton: store.UserButton{Enabled: true}}
	r.Locales = &store.LocalesConfig{Mechanism: "path"}

	got := pageAgentFragment(r, []string{"en", "fr"})
	if !strings.Contains(got, `data-locale-paths="/fpl-ui"`) {
		t.Errorf("the agent was not told where the language sits:\n%s", got)
	}

	// Without the mechanism there is nothing in the URL to move, and the
	// page keeps reloading as it always has.
	r.Locales = &store.LocalesConfig{Mechanisms: []string{"query"}}
	if got := pageAgentFragment(r, []string{"en", "fr"}); strings.Contains(got, "data-locale-paths") {
		t.Errorf("a route without the path mechanism got a locale path:\n%s", got)
	}
}

// The behaviour a route declares belongs to the PAGE, not to the chrome: a
// route that offers the light/dark switch used to get nothing at all when the
// button was turned off, because the only code applying it shipped inside the
// button and sat behind the button's own frame guard.
func TestPageAgentRunsWithoutTheButton(t *testing.T) {
	r := pathRoute("ui", "ui", 1, "/app/**", "http://up")
	r.IsUI = true
	r.UI = &store.RouteUI{
		Scheme: &store.SchemeConfig{Select: true, Mechanism: "class", Light: "lt", Dark: "dk"},
	}
	// No user button on this route.
	if got := userButtonFragment(r, []string{"en", "fr"}); got != "" {
		t.Fatalf("a route without the button got chrome:\n%s", got)
	}
	got := pageAgentFragment(r, []string{"en", "fr"})
	for _, want := range []string{
		`src="/meerkat/page.js"`, `data-scheme="select"`, `data-scheme-mechanism="class"`,
		`data-scheme-light="lt"`, `data-scheme-dark="dk"`, `data-languages="en,fr"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the agent lost %s:\n%s", want, got)
		}
	}

	// And a route that declares NOTHING still gets one: the agent carries the
	// session watch, so a page without it would go on looking signed in for
	// hours after the session ended.
	plain := pathRoute("plain", "plain", 1, "/plain/**", "http://up")
	plain.IsUI = true
	plain.UI = &store.RouteUI{}
	if got := pageAgentFragment(plain, nil); !strings.Contains(got, "/meerkat/page.js") {
		t.Errorf("a plain UI route got no agent:\n%s", got)
	}
}

// Both scripts are deferred, so they run in document order, and the button's
// element must not upgrade before window.meerkatPage exists.
func TestPageAgentComesBeforeTheButton(t *testing.T) {
	r := pathRoute("ui", "ui", 1, "/app/**", "http://up")
	r.IsUI = true
	r.UI = &store.RouteUI{UserButton: store.UserButton{Enabled: true}}

	frag := pageAgentFragment(r, []string{"en", "fr"}) + userButtonFragment(r, []string{"en", "fr"})
	agent := strings.Index(frag, "/meerkat/page.js")
	button := strings.Index(frag, "/meerkat/user-button.js")
	if agent < 0 || button < 0 || agent > button {
		t.Errorf("the button would upgrade before the agent exists:\n%s", frag)
	}
}

// The URL does not decide, the person does. A language in the path is where
// the application lives, not a choice: someone who has settled on English and
// opens a link a colleague sent in French reads it in English, and the address
// bar is corrected to say so rather than lie.
func TestLocaleInTheURLFollowsThePerson(t *testing.T) {
	up := htmlUpstream(t, "/app/en/page")
	r := pathRoute("ui", "ui", 1, "/app/**", up.URL)
	r.IsUI = true
	r.Locales = &store.LocalesConfig{Mechanism: "path"}
	// The offer is the APPLICATION's: without it a route has no language to
	// recognise in a path.
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingLanguages, []string{"en", "fr"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveRoute(ctx, r); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	ask := func(path, cookie, mode string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Sec-Fetch-Mode", mode)
		req.AddCookie(&http.Cookie{Name: langCookie, Value: cookie})
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res
	}

	// French in the URL, English chosen: corrected, query carried over.
	res := ask("/app/fr/page?q=1", "en", "navigate")
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/app/en/page?q=1" {
		t.Fatalf("got %d %q, want a redirect to the person's language", res.StatusCode, res.Header.Get("Location"))
	}
	// Already in step: served, not bounced.
	if res := ask("/app/en/page", "en", "navigate"); res.StatusCode != http.StatusOK {
		t.Errorf("a URL already in the right language answered %d", res.StatusCode)
	}
	// Not a navigation: an asset a corrected page is fetching has nothing to
	// correct, and bouncing it would only cost a round trip.
	if res := ask("/app/fr/page", "en", "no-cors"); res.StatusCode == http.StatusFound {
		t.Error("an asset request was redirected")
	}
	// No language in the path: the browser is sent to the one it should have.
	// Inserting it only on the way upstream was enough until an application
	// built per locale stamped <base href="/app/en/">: every link it draws
	// then lives under /app/en/ while the address bar says /app/, and its
	// router resolves its default route against the base and lands on
	// /app/en/app. That reads as a segment the gateway doubled; it is the
	// application reconciling an address that was hidden from it.
	res = ask("/app/page", "en", "navigate")
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/app/en/page" {
		t.Errorf("got %d %q, want the language put in the address bar",
			res.StatusCode, res.Header.Get("Location"))
	}
	// A directory keeps its trailing slash: /app/en and /app/en/ are two
	// different resources for more upstreams than one would like.
	res = ask("/app/", "en", "navigate")
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/app/en/" {
		t.Errorf("got %d %q, want /app/en/", res.StatusCode, res.Header.Get("Location"))
	}
}

// The query parameter follows the same rule as the path: the gateway already
// overwrites it on the way upstream, so an old value left in the address bar
// names a language the page is not in.
func TestLocaleQueryFollowsThePerson(t *testing.T) {
	up := htmlUpstream(t, "/app/page")
	r := pathRoute("q", "q", 1, "/app/**", up.URL)
	r.IsUI = true
	r.Locales = &store.LocalesConfig{Mechanism: "query", Param: "lg"}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingLanguages, []string{"en", "fr"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveRoute(ctx, r); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	ask := func(path, cookie string) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.AddCookie(&http.Cookie{Name: langCookie, Value: cookie})
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res
	}

	res := ask("/app/page?lg=fr&x=1", "en")
	if res.StatusCode != http.StatusFound {
		t.Fatalf("a stale language in the query was kept: %d", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); !strings.Contains(loc, "lg=en") || !strings.Contains(loc, "x=1") {
		t.Errorf("Location = %q, want the language corrected and the rest kept", loc)
	}
	// A route with no locale in its path must not have one invented there.
	if loc := res.Header.Get("Location"); strings.Contains(loc, "/app/en/") {
		t.Errorf("the path was rewritten on a query-only route: %q", loc)
	}
	if res := ask("/app/page?lg=en", "en"); res.StatusCode != http.StatusOK {
		t.Errorf("a query already in the right language answered %d", res.StatusCode)
	}
	// Nothing to correct: the parameter is simply added upstream, as before.
	if res := ask("/app/page", "en"); res.StatusCode != http.StatusOK {
		t.Errorf("a URL with no parameter answered %d", res.StatusCode)
	}
}

// Code is not a vault reference. A script writes $translate and $rootScope, a
// stylesheet writes a $ in a selector, and the expansion read them as names to
// resolve: saving was refused for "unknown vault entries: translate.use,
// rootScope.". The two syntaxes share the dollar and code owns it here - the
// same call already made for a template's own $i and $r.
func TestCodeBlocksAreNotVaultReferences(t *testing.T) {
	r := pathRoute("ui", "ui", 1, "/app/**", "http://up")
	r.IsUI = true
	r.UI = &store.RouteUI{
		CustomJS:  "if ($rootScope.ready) { $translate.use('fr'); }",
		CustomCSS: "a[href$='.pdf'] { color: red }",
	}
	r.Locales = &store.LocalesConfig{
		Mechanism: store.LocaleScript,
		OnChange:  "$translate.use(locale);\n$rootScope.$applyAsync();",
	}

	expanded, missing, err := ExpandRoute(r, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("code was read as vault references: %v", missing)
	}
	if expanded.Locales.OnChange != r.Locales.OnChange {
		t.Errorf("the script came back changed:\n%s", expanded.Locales.OnChange)
	}
	if expanded.UI.CustomJS != r.UI.CustomJS || expanded.UI.CustomCSS != r.UI.CustomCSS {
		t.Errorf("an injected block came back changed:\n%s\n%s", expanded.UI.CustomJS, expanded.UI.CustomCSS)
	}

	// What is NOT code still resolves: the upstream keeps its reference.
	r.Upstream = "$api-host"
	expanded, missing, err = ExpandRoute(r, map[string]string{"api-host": "http://real:8080"})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 || expanded.Upstream != "http://real:8080" {
		t.Errorf("the upstream reference stopped resolving: %q %v", expanded.Upstream, missing)
	}
}

// One mechanism, not a set. The list held transports only, where cumulating
// meant something; it decides the page's behaviour too now, and declaring both
// "the language is in the path" and "the application applies it itself" is two
// contradictory answers to one question. A route configured before the choice
// became exclusive still runs: the first of the old list wins.
func TestLocaleMechanismIsOneChoice(t *testing.T) {
	if got := (store.LocalesConfig{}).Mode(); got != store.LocaleAccept {
		t.Errorf("nothing declared should mean %q, got %q", store.LocaleAccept, got)
	}
	if got := (store.LocalesConfig{Mechanism: store.LocalePath}).Mode(); got != store.LocalePath {
		t.Errorf("Mode() = %q", got)
	}
	// The retired form, as an instance configured yesterday still holds it.
	legacy := store.LocalesConfig{Mechanisms: []string{store.LocaleScript, store.LocalePath}}
	if got := legacy.Mode(); got != store.LocaleScript {
		t.Errorf("a route saved with the old list lost its mechanism: %q", got)
	}
	// And the explicit choice wins over the leftover.
	both := store.LocalesConfig{Mechanism: store.LocaleQuery, Mechanisms: []string{store.LocalePath}}
	if got := both.Mode(); got != store.LocaleQuery {
		t.Errorf("the explicit mechanism must win: %q", got)
	}
}

// set-host is the one filter whose whole purpose is a header the proxy machine
// also writes: SetURL points the request at the upstream and takes the Host
// with it, which is right by default and wrong here - a service answering by
// virtual host needs a name the upstream URL does not carry. Pinned end to
// end, because the unit test on the filter passed while the gateway undid it
// three lines later.
func TestSetHostSurvivesTheProxyRewrite(t *testing.T) {
	var seen string
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Host
	}))
	t.Cleanup(upstream.Close)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	route := pathRoute("r-host", "host", 1, "/v/**", upstream.URL)
	route.Filters = []routing.Spec{
		{Type: "strip-prefix", Args: map[string]any{"parts": 1}},
		{Type: "set-host", Args: map[string]any{"host": "app.internal"}},
	}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/v/x")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if seen != "app.internal" {
		t.Fatalf("the upstream saw Host %q, want app.internal", seen)
	}
}
