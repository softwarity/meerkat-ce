package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The installation-wide developer switch (DEV-01). The capability says WHO may
// develop here; this says whether HERE is a place one develops at all.
//
// What this pins is that the switch reaches EVERY door, not just the menu that
// advertises them. A developer who bookmarked /meerkat/apidocs/ or whose
// browser still holds the UI test bar has to meet the same closed gateway as
// someone reading the user button - otherwise turning developer mode off means
// hiding the entrance while leaving the door open, which is worse than not
// having the switch: the operator believes it is closed.
func TestDeveloperModeClosesEveryDoor(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><body>ok</body></html>`)
	}))
	t.Cleanup(upstream.Close)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	route := pathRoute("r-open", "open", 1, "/open/**", upstream.URL)
	route.IsUI = true
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "d", Username: "devon", PasswordHash: "x", Enabled: true, Dev: true}); err != nil {
		t.Fatal(err)
	}

	sm := session.NewManager(st)
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	rt.RegisterUISim(mux)
	rt.RegisterDevDocs(mux)
	mux.Handle("/", rt)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	rec := httptest.NewRecorder()
	if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "d"); err != nil {
		t.Fatal(err)
	}
	devC := rec.Result().Cookies()[0]

	call := func(method, path, body string) int {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.AddCookie(devC)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		return res.StatusCode
	}

	// A fresh installation develops: the capability was the only gate before
	// this setting existed, and an upgrade must not take the tooling away.
	if !st.DevMode(ctx) {
		t.Fatal("a fresh installation came up with developer mode off")
	}
	if code := call("GET", "/meerkat/apidocs/catalog.json", ""); code != http.StatusOK {
		t.Fatalf("docs catalog with the switch on: %d, want 200", code)
	}
	if code := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane"}`); code != http.StatusOK {
		t.Fatalf("UI test mode with the switch on: %d, want 200", code)
	}

	if err := st.SetDevMode(ctx, false); err != nil {
		t.Fatal(err)
	}

	// Switched off, the same developer, with the same session and the same
	// capability, meets a gateway that develops nowhere.
	if code := call("GET", "/meerkat/apidocs/catalog.json", ""); code == http.StatusOK {
		t.Fatal("the route API docs answered a developer with developer mode off")
	}
	if code := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane"}`); code != http.StatusUnauthorized {
		t.Fatalf("UI test mode with the switch off: %d, want 401", code)
	}
	// And the identity simulation the docs and the bar drive: refused at the
	// source, so a bar left open in a tab cannot keep posing as someone else.
	req := httptest.NewRequest("GET", "/open/x", nil)
	req.AddCookie(devC)
	if _, _, ok := rt.simulationActor(req); ok {
		t.Fatal("a data-plane session could still simulate with developer mode off")
	}

	// And back: nothing is one-way, or an operator who closed a demo would
	// have to remember which accounts to re-flag.
	if err := st.SetDevMode(ctx, true); err != nil {
		t.Fatal(err)
	}
	if code := call("GET", "/meerkat/apidocs/catalog.json", ""); code != http.StatusOK {
		t.Fatalf("docs catalog after switching back on: %d, want 200", code)
	}
}
