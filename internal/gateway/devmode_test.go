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
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The dev capability (DEV-01) is the one gate. What this pins is that taking it
// away reaches EVERY door, not just the menu that advertises them. A developer
// who bookmarked /meerkat/apidocs/ or whose browser still holds the UI test bar
// has to meet the same closed gateway as someone reading the user button -
// otherwise clearing the capability hides the entrance and leaves the door
// open, and the operator believes it is closed.
func TestTheDevCapabilityClosesEveryDoor(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><body>ok</body></html>`)
	}))
	t.Cleanup(upstream.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
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

	if code := call("GET", "/meerkat/apidocs/catalog.json", ""); code != http.StatusOK {
		t.Fatalf("docs catalog with the capability: %d, want 200", code)
	}
	if code := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane"}`); code != http.StatusOK {
		t.Fatalf("UI test mode with the capability: %d, want 200", code)
	}

	if _, err := st.ClearDevCapability(ctx); err != nil {
		t.Fatal(err)
	}

	// Without the capability, the same developer, with the same session, meets
	// a gateway that develops nowhere.
	if code := call("GET", "/meerkat/apidocs/catalog.json", ""); code == http.StatusOK {
		t.Fatal("the route API docs answered a developer without the capability")
	}
	if code := call("POST", "/meerkat/dev-sim", `{"route":"r-open","user":"jane"}`); code != http.StatusUnauthorized {
		t.Fatalf("UI test mode without the capability: %d, want 401", code)
	}
	// And the identity simulation the docs and the bar drive: refused at the
	// source, so a bar left open in a tab cannot keep posing as someone else.
	req := httptest.NewRequest("GET", "/open/x", nil)
	req.AddCookie(devC)
	if _, _, ok := rt.simulationActor(req); ok {
		t.Fatal("a data-plane session could still simulate without the capability")
	}

	// And back, on the same session: the capability is read on each request.
	u, err := st.GetUserByID(ctx, "d")
	if err != nil {
		t.Fatal(err)
	}
	u.Dev = true
	if err := st.UpdateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if code := call("GET", "/meerkat/apidocs/catalog.json", ""); code != http.StatusOK {
		t.Fatalf("docs catalog with the capability back: %d, want 200", code)
	}
}
