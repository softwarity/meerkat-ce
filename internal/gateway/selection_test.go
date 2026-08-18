package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// A route's security takes part in CHOOSING it, not only in permitting it.
// Two routes may cover the same paths and differ only by who they are for -
// one /** per organisation, one per role - and until now the first one always
// won and the second was unreachable, whatever it said.
func TestTwoRoutesOnTheSamePathsDifferByWhoTheyAreFor(t *testing.T) {
	acme := upstreamSaying(t, "acme")
	globex := upstreamSaying(t, "globex")

	st, sm := storeWithTwoTenants(t)
	ctx := context.Background()
	for _, r := range []store.Route{
		{ID: "for-acme", Name: "for-acme", Order: 10, Enabled: true, IsUI: true, Upstream: acme.URL,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
			Access:     store.Access{Level: store.AccessTenants, Tenants: []string{"t-acme"}}},
		{ID: "for-globex", Name: "for-globex", Order: 20, Enabled: true, IsUI: true, Upstream: globex.URL,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
			Access:     store.Access{Level: store.AccessTenants, Tenants: []string{"t-globex"}}},
	} {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	// Each member is served by THEIR route, whatever the order says: the acme
	// one comes first and does not swallow the globex member.
	for _, c := range []struct{ user, tenant, want string }{
		{"u-acme", "t-acme", "acme"},
		{"u-globex", "t-globex", "globex"},
	} {
		body := getAs(t, rt, sm, "/anything", c.user, c.tenant)
		if body != c.want {
			t.Errorf("%s was served %q, want %q", c.user, body, c.want)
		}
	}
}

// Falling through must not cost the reason. When nothing answers, the first
// route that turned the caller away explains itself - a bare 404 is the one
// answer nobody can act on.
func TestARefusalSurvivesTheFallThrough(t *testing.T) {
	up := upstreamSaying(t, "up")
	st, sm := storeWithTwoTenants(t)
	ctx := context.Background()
	if err := st.SaveRoute(ctx, store.Route{
		ID: "only-acme", Name: "only-acme", Order: 10, Enabled: true, Upstream: up.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
		Access:     store.Access{Level: store.AccessTenants, Tenants: []string{"t-acme"}},
	}); err != nil {
		t.Fatal(err)
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/anything", nil)
	req.AddCookie(issueFor(t, sm, "u-globex", "t-globex"))
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403 - a fall-through with nothing after it must still explain", rec.Code)
	}
	if body := rec.Body.String(); body == "" || body == "404 page not found\n" {
		t.Fatalf("the refusal lost its reason: %q", body)
	}
}

// A closed door stays closed: falling through a deny would turn the one rule
// written to shut a path into a rule that merely redirects it.
func TestDenyIsTerminal(t *testing.T) {
	up := upstreamSaying(t, "open")
	st, sm := storeWithTwoTenants(t)
	ctx := context.Background()
	for _, r := range []store.Route{
		{ID: "shut", Name: "shut", Order: 10, Enabled: true, Upstream: up.URL,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/private/**"}}}},
			Access:     store.Access{Level: store.AccessDeny}},
		{ID: "open", Name: "open", Order: 20, Enabled: true, Upstream: up.URL,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}}},
	} {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/private/x", nil)
	req.AddCookie(issueFor(t, sm, "u-acme", "t-acme"))
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a denied path answered %d: the catch-all behind it took over", rec.Code)
	}
}

// ---- fixtures ----

func upstreamSaying(t *testing.T, what string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(what))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func storeWithTwoTenants(t *testing.T) (*store.Store, *session.Manager) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for _, ten := range []store.Tenant{
		{ID: "t-acme", Name: "acme", Enabled: true},
		{ID: "t-globex", Name: "globex", Enabled: true},
	} {
		if err := st.SaveTenant(ctx, ten); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []struct{ id, tenant string }{{"u-acme", "t-acme"}, {"u-globex", "t-globex"}} {
		if err := st.CreateUser(ctx, store.User{ID: u.id, Username: u.id, PasswordHash: "x", Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveMembership(ctx, store.Membership{
			UserID: u.id, TenantID: u.tenant, Type: store.MemberUser, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	return st, session.NewManager(st)
}

func issueFor(t *testing.T, sm *session.Manager, user, tenant string) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	if _, err := sm.IssueWith(context.Background(), rec, httptest.NewRequest("POST", "/login", nil),
		user, tenant, "", 0, "", "", ""); err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func getAs(t *testing.T, rt *Router, sm *session.Manager, path, user, tenant string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	req.AddCookie(issueFor(t, sm, user, tenant))
	rt.ServeHTTP(rec, req)
	return rec.Body.String()
}

// The probe has to answer the question the engine answers, or it is a second
// implementation of the routing table that agrees with the first only by luck.
// Since a route's security takes part in choosing it, the probe asks it too.
func TestTheProbeJudgesTheRuleAsTheEngineDoes(t *testing.T) {
	up := upstreamSaying(t, "up")
	st, sm := storeWithTwoTenants(t)
	ctx := context.Background()
	for _, r := range []store.Route{
		{ID: "for-acme", Name: "for-acme", Order: 10, Enabled: true, Upstream: up.URL,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
			Access:     store.Access{Level: store.AccessTenants, Tenants: []string{"t-acme"}}},
		{ID: "for-globex", Name: "for-globex", Order: 20, Enabled: true, Upstream: up.URL,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
			Access:     store.Access{Level: store.AccessTenants, Tenants: []string{"t-globex"}}},
	} {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	rt := New(st, sm)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	synth := routing.Synthesis{Method: "GET", Path: "/anything"}

	// As a globex member the FIRST route matches on paths and is turned away
	// by its rule, so the probe must name the second one - exactly as the
	// engine does.
	got := rt.Probe("", synth, ProbeAs{TenantID: "t-globex", Memberships: []string{"t-globex"}})
	if got.WinnerID != "for-globex" {
		t.Errorf("winner = %q, want for-globex", got.WinnerID)
	}
	if len(got.Steps) == 0 || got.Steps[0].Rule || got.Steps[0].RuleReason == "" {
		t.Errorf("the first step must show a rule that refused, and say why: %+v", got.Steps[0])
	}
	// The predicates still matched: a probe that hid that would send someone
	// looking for a path problem that is not there.
	if !got.Steps[0].Matched {
		t.Error("the refused route must still report its predicates as matching")
	}

	// Anonymous: neither rule grants, so nothing answers.
	if got := rt.Probe("", synth, ProbeAs{}); got.WinnerID != "" || got.Outcome != "none" {
		t.Errorf("anonymous winner = %q (%s), want none", got.WinnerID, got.Outcome)
	}
}
