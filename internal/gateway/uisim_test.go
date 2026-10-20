package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The UI test mode (DEV-10): dead while the switch is off, dev-only, scoped
// to the caller's session AND one route, visible in the served page (roles
// stamp) and to the upstream (X-Meerkat-Test markers), reversible, bounded.
func TestUISim(t *testing.T) {
	// The upstream records the test markers it receives.
	var lastVia, lastBy atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastVia.Store(r.Header.Get("X-Meerkat-Test"))
		lastBy.Store(r.Header.Get("X-Meerkat-Test-By"))
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><head></head><body>ok</body></html>`)
	}))
	t.Cleanup(upstream.Close)

	open := pathRoute("r-open", "open", 1, "/open/**", upstream.URL)
	open.IsUI = true
	open.UI = &store.RouteUI{Roles: &store.RolesConfig{Enabled: true}}
	gated := pathRoute("r-gated", "gated", 2, "/gated/**", upstream.URL)
	gated.Access = store.Access{Roles: []string{"admin"}}

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for _, r := range []store.Route{open, gated} {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateUser(ctx, store.User{ID: "d", Username: "devon", PasswordHash: "x", Enabled: true, Dev: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "b", Username: "bob", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// A small hierarchy: ops implies ops-read and ops-write (RBAC-01).
	for _, role := range []store.Role{
		{Name: "ops"},
		{Name: "ops-read", Parent: "ops"},
		{Name: "ops-write", Parent: "ops"},
	} {
		if err := st.SaveRole(ctx, role); err != nil {
			t.Fatal(err)
		}
	}
	sm := session.NewManager(st)
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	// Same mux shape as main: the dev-sim endpoints beside the router.
	mux := http.NewServeMux()
	rt.RegisterUISim(mux)
	mux.Handle("/", rt)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// A browser per person: the running test rides a cookie, so each keeps
	// its own jar, seeded with its session.
	base, _ := url.Parse(srv.URL)
	browserFor := func(userID string) *http.Client {
		rec := httptest.NewRecorder()
		if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), userID); err != nil {
			t.Fatal(err)
		}
		jar, _ := cookiejar.New(nil)
		jar.SetCookies(base, rec.Result().Cookies()[:1])
		return &http.Client{Jar: jar}
	}
	devC, bobC := browserFor("d"), browserFor("b")

	call := func(method, path, body string, client *http.Client) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return res.StatusCode, string(b)
	}

	t.Run("dev-only, live routes only, roles filtered", func(t *testing.T) {
		if code, _ := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane"}`, bobC); code != http.StatusUnauthorized {
			t.Fatalf("non-dev: %d, want 401", code)
		}
		if code, _ := call("POST", "/meerkat/dev-sim", `{"route":"nope","user":"jane"}`, devC); code != http.StatusUnprocessableEntity {
			t.Fatalf("unknown route: %d, want 422", code)
		}
		if code, _ := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":""}`, devC); code != http.StatusUnprocessableEntity {
			t.Fatalf("empty user: %d, want 422", code)
		}
		code, body := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane","roles":["sales","bad token"]}`, devC)
		if code != http.StatusOK || !strings.Contains(body, `"active":true`) || strings.Contains(body, "bad token") {
			t.Fatalf("set: %d %s", code, body)
		}
		if _, body := call("GET", "/meerkat/dev-sim?route=r-open", "", devC); !strings.Contains(body, `"user":"jane"`) {
			t.Fatalf("state: %s", body)
		}
	})

	t.Run("the tested route serves AS the simulated identity, marked as a test", func(t *testing.T) {
		if _, body := call("GET", "/open/page", "", devC); !strings.Contains(body, `<body class="sales">`) {
			t.Fatalf("simulated roles must stamp the page: %s", body)
		}
		if lastVia.Load() != "ui-test" || lastBy.Load() != "devon" {
			t.Fatalf("upstream markers: via=%v by=%v", lastVia.Load(), lastBy.Load())
		}
		// Another session on the same route stays untouched.
		if _, body := call("GET", "/open/page", "", bobC); strings.Contains(body, "sales") {
			t.Fatalf("bob must not inherit the simulation: %s", body)
		}
		if via := lastVia.Load(); via != "" {
			t.Fatalf("bob's request must not be marked: %v", via)
		}
	})

	t.Run("a refused simulated identity gets the exit page, not a lockout", func(t *testing.T) {
		if code, body := call("GET", "/gated/page", "", devC); code != http.StatusForbidden || strings.Contains(body, "Exit UI test") {
			t.Fatalf("no sim on gated: %d %s", code, body)
		}
		if code, _ := call("POST", "/meerkat/dev-sim", `{"route":"r-gated","user":"jane","roles":["sales"]}`, devC); code != http.StatusOK {
			t.Fatalf("set gated sim: %d", code)
		}
		code, body := call("GET", "/gated/page", "", devC)
		if code != http.StatusForbidden || !strings.Contains(body, "Exit UI test mode") || !strings.Contains(body, "jane") {
			t.Fatalf("refusal page: %d %s", code, body)
		}
	})

	t.Run("a posed role implies its descendants (RBAC-01)", func(t *testing.T) {
		code, body := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane","roles":["ops"]}`, devC)
		if code != http.StatusOK || !strings.Contains(body, "ops-read") || !strings.Contains(body, "ops-write") {
			t.Fatalf("set with a parent role must answer the expansion: %d %s", code, body)
		}
		if _, body := call("GET", "/open/page", "", devC); !strings.Contains(body, `<body class="ops ops-read ops-write">`) {
			t.Fatalf("descendants must stamp the page too: %s", body)
		}
	})

	t.Run("another node of the cluster serves the same test", func(t *testing.T) {
		// Two gateways on one database: the Apply lands on one, the reloaded
		// page on the other - which used to know nothing of it.
		other := New(st, sm)
		if err := other.Reload(ctx); err != nil {
			t.Fatal(err)
		}
		mux2 := http.NewServeMux()
		other.RegisterUISim(mux2)
		mux2.Handle("/", other)
		srv2 := httptest.NewServer(mux2)
		t.Cleanup(srv2.Close)
		if code, _ := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane","roles":["sales"]}`, devC); code != http.StatusOK {
			t.Fatalf("set: %d", code)
		}
		req, _ := http.NewRequest("GET", srv2.URL+"/open/page", nil)
		for _, ck := range devC.Jar.Cookies(base) {
			req.AddCookie(ck)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if !strings.Contains(string(b), `<body class="sales">`) {
			t.Fatalf("the other node must serve the simulated identity: %s", b)
		}
	})

	t.Run("starting and ending a test are in the trail", func(t *testing.T) {
		if code, _ := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane","roles":["sales"]}`, devC); code != http.StatusOK {
			t.Fatalf("set: %d", code)
		}
		if code, _ := call("DELETE", "/meerkat/dev-sim?route=r-open", "", devC); code != http.StatusOK {
			t.Fatalf("clear: %d", code)
		}
		// Cancel on a route with no test running writes nothing.
		if code, _ := call("DELETE", "/meerkat/dev-sim?route=r-open", "", devC); code != http.StatusOK {
			t.Fatalf("idle clear: %d", code)
		}
		events, err := st.ListAuditEvents(ctx, store.AuditFilter{ActorID: "d"})
		if err != nil {
			t.Fatal(err)
		}
		var starts, stops int
		for _, ev := range events {
			switch ev.Action {
			case "uitest.start":
				// Earlier steps started tests too; this one's line is the
				// one naming exactly what was just applied.
				if ev.ActorName == "devon" && ev.Detail == "route open: as jane, roles sales" {
					starts++
				}
			case "uitest.stop":
				stops++
			}
		}
		if starts == 0 || stops != 1 {
			t.Fatalf("trail: %d matching starts, %d stops (want a start, and exactly one stop)", starts, stops)
		}
	})

	t.Run("taking the dev capability ends a running test at once", func(t *testing.T) {
		if code, _ := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane","roles":["sales"]}`, devC); code != http.StatusOK {
			t.Fatalf("set: %d", code)
		}
		u, err := st.GetUserByID(ctx, "d")
		if err != nil {
			t.Fatal(err)
		}
		u.Dev = false
		if err := st.UpdateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
		defer func() {
			back, err := st.GetUserByID(ctx, "d")
			if err != nil {
				t.Fatal(err)
			}
			back.Dev = true
			if err := st.UpdateUser(ctx, back); err != nil {
				t.Fatal(err)
			}
		}()
		if _, body := call("GET", "/open/page", "", devC); strings.Contains(body, "sales") {
			t.Fatalf("a developer without the capability must not keep a running test: %s", body)
		}
	})

	t.Run("exit and expiry both end the test", func(t *testing.T) {
		if code, _ := call("DELETE", "/meerkat/dev-sim?route=r-open", "", devC); code != http.StatusOK {
			t.Fatalf("clear: %d", code)
		}
		if _, body := call("GET", "/open/page", "", devC); strings.Contains(body, "sales") {
			t.Fatalf("cleared sim still applies: %s", body)
		}
		// An expired cookie answers inactive, whatever the browser still holds.
		var hash string
		for _, ck := range devC.Jar.Cookies(base) {
			if s, err := sm.Resolve(ctx, sessionRequest(ck)); err == nil {
				hash = s.TokenHash
			}
		}
		stale := rt.uiSimSign(uiSimClaims{Session: hash, Route: "r-gated", User: "jane", Roles: []string{"sales"}, By: "devon", Exp: time.Now().Add(-time.Minute).Unix()})
		devC.Jar.SetCookies(base, []*http.Cookie{{Name: uiSimCookie(httptest.NewRequest("GET", "/", nil), "r-gated"), Value: stale, Path: "/"}})
		if _, body := call("GET", "/gated/page", "", devC); strings.Contains(body, "Exit UI test mode") {
			t.Fatalf("expired sim still applies: %s", body)
		}
	})
}

// sessionRequest is a request carrying one cookie, for resolving a session.
func sessionRequest(ck *http.Cookie) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(ck)
	return r
}
