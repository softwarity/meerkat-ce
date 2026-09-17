package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// Who a session is comes from memory between writes: that is what took six to
// eight queries off every authenticated request. So a role granted in the
// store is NOT seen until the identity is forgotten - which the gateway does
// after every write it serves outside the proxied traffic (afterWrites,
// cmd/meerkat), and the bus does for the other gateways. This test is the one
// that says a write path which skipped that would leave a stale role.
func TestIdentityIsRememberedUntilAWriteForgetsIt(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "alice", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveTenant(ctx, store.Tenant{ID: "t1", Name: "Acme", Enabled: true,
		BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMembership(ctx, store.Membership{UserID: "u1", TenantID: "t1",
		Type: store.MemberUser, Enabled: true, BusinessAccess: store.BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"ROLE_A", "ROLE_B"} {
		if err := st.SaveRole(ctx, store.Role{ID: role, Name: role}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SaveGroup(ctx, store.Group{ID: "g1", TenantID: "t1", Name: "G", RoleIDs: []string{"ROLE_A"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMemberGroups(ctx, "t1", "u1", []string{"g1"}); err != nil {
		t.Fatal(err)
	}

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

	if got := call(); got != "ROLE_A" {
		t.Fatalf("first call: got %q, want ROLE_A", got)
	}
	if err := st.SaveGroup(ctx, store.Group{ID: "g1", TenantID: "t1", Name: "G", RoleIDs: []string{"ROLE_B"}}); err != nil {
		t.Fatal(err)
	}
	if got := call(); got != "ROLE_A" {
		t.Fatalf("without a forget, the remembered identity answers: got %q, want ROLE_A", got)
	}
	rt.ForgetIdentities()
	if got := call(); got != "ROLE_B" {
		t.Fatalf("after a forget, the group's new role is forwarded: got %q, want ROLE_B", got)
	}

	// And a caller cannot edit what the next request receives: each gets its
	// own copy of the roles.
	d, ok := rt.sessionIdentity(requestWithCookie(cookie))
	if !ok || len(d.Roles) != 1 {
		t.Fatalf("identity: %+v %v", d, ok)
	}
	d.Roles[0] = "TAMPERED"
	if again, _ := rt.sessionIdentity(requestWithCookie(cookie)); again.Roles[0] != "ROLE_B" {
		t.Fatalf("a caller's edit leaked into the remembered identity: %v", again.Roles)
	}
}

func requestWithCookie(c *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(c)
	return req
}
