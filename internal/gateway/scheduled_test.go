package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// What a SCHEDULED call is, end to end on the real router (SCHED-01).
//
// There is no account behind it and no credential on the wire: the identity is
// posed in the context of an in-process request, and from there it is an
// ordinary caller - the route's rule reads its roles, and the service receives
// them in whatever the route forwards.
func TestAScheduledCallIsACallerWithRoles(t *testing.T) {
	// The upstream repeats what it was told about its caller.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"user":  r.Header.Get("X-Remote-User"),
			"roles": r.Header.Get("X-Roles"),
			"job":   r.Header.Get("Meerkat-Job-Run"),
		})
	}))
	t.Cleanup(upstream.Close)

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	// A route reserved to one role, which forwards who is calling.
	r := pathRoute("r-jobs", "jobs", 1, "/jobs/**", upstream.URL)
	r.Access = store.Access{Level: store.AccessAuth, Roles: []string{"station_fetch"}}
	r.Identity = &store.IdentityForward{
		Mechanism: "headers",
		Attributes: []store.IdentityAttr{
			{Field: "username", As: "X-Remote-User"},
			{Field: "roles", As: "X-Roles"},
		},
	}
	if err := st.SaveRoute(ctx, r); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	call := func(roles []string) (int, map[string]string) {
		req := httptest.NewRequest("POST", "http://localhost/jobs/fetch", nil)
		req = req.WithContext(WithScheduledCaller(req.Context(), roles, ""))
		req.Header.Set("Meerkat-Job-Run", "GvbDqbNG")
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		var out map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	// With the role the route asks for: it goes through, and the service is
	// told who called - a name that is not an account, and the roles asked for.
	code, got := call([]string{"station_fetch"})
	if code != http.StatusOK {
		t.Fatalf("a scheduled call with the role answered %d, want 200", code)
	}
	if got["user"] != ScheduledUser {
		t.Errorf("the service was told the caller is %q, want %q", got["user"], ScheduledUser)
	}
	if got["roles"] != "STATION_FETCH" && got["roles"] != "station_fetch" {
		t.Errorf("the service was told the roles are %q", got["roles"])
	}
	if got["job"] != "GvbDqbNG" {
		t.Errorf("the run identifier did not reach the service: %q", got["job"])
	}

	// Without it: refused by the SAME rule that refuses a person, and refused
	// as an API client - a 403 that says so, not a redirection to a page.
	if code, _ := call([]string{"something_else"}); code != http.StatusForbidden {
		t.Fatalf("a scheduled call without the role answered %d, want 403", code)
	}
	if code, _ := call(nil); code != http.StatusForbidden {
		t.Fatalf("a scheduled call with no role at all answered %d, want 403", code)
	}
}

// The same call, on an operation the route's endpoint security names: the
// endpoint's gate asked for a session, which a scheduled call never has, and
// answered 401 whatever roles the schedule carried.
func TestAScheduledCallPassesAnEndpointRule(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	r := pathRoute("r-orders", "orders", 1, "/orders/**", upstream.URL)
	r.Access = store.Access{Level: store.AccessAuth}
	r.API = &store.RouteAPI{Security: &store.EndpointSecurity{Endpoints: []store.EndpointPolicy{
		{Method: "POST", Path: "/orders/reindex", Access: store.Access{Roles: []string{"ops"}}},
	}}}
	if err := st.SaveRoute(ctx, r); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	call := func(roles []string) int {
		req := httptest.NewRequest("POST", "http://localhost/orders/reindex", nil)
		req = req.WithContext(WithScheduledCaller(req.Context(), roles, ""))
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := call([]string{"ops"}); code != http.StatusOK {
		t.Fatalf("a scheduled call with the endpoint's role answered %d, want 200", code)
	}
	if code := call([]string{"sales"}); code != http.StatusForbidden {
		t.Fatalf("a scheduled call without it answered %d, want 403", code)
	}
}
