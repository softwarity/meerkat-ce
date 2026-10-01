package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestSaveRoleWorksByNameAndCannotRename: the catalogue was out of an agent's
// reach because every access rule names roles BY NAME, so renaming one silently
// changes who reaches what. The tool keeps that property by construction rather
// than by a check: it works by name, so there is no rename to perform - a name
// it does not know is a new role, a name it knows is that role.
func TestSaveRoleWorksByNameAndCannotRename(t *testing.T) {
	a, ctx := roleAPI(t)

	// A name nobody holds: a new role, top-level.
	out, err := a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_ADMIN","description":"admin","tags":["otel-demo"]}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if m := out.(map[string]any); m["created"] != true {
		t.Errorf("want created, got %+v", m)
	}

	// grantedBy is the role ABOVE: holding it grants this one. That is the
	// direction the store expands ("a role implies its descendants"), and the
	// one thing a reader gets backwards.
	if _, err := a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_USER","grantedBy":"ROLE_ADMIN"}`)); err != nil {
		t.Fatalf("create with a parent: %v", err)
	}
	roles, err := a.st.ListRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]store.Role{}
	for _, r := range roles {
		byName[r.Name] = r
	}
	if byName["ROLE_USER"].ParentID != byName["ROLE_ADMIN"].ID {
		t.Errorf("ROLE_USER should hang under ROLE_ADMIN, got %+v", byName["ROLE_USER"])
	}

	// The same name again is an EDIT, not a second role - and what is left out
	// keeps its value.
	if _, err := a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_ADMIN","description":"changed"}`)); err != nil {
		t.Fatalf("update: %v", err)
	}
	roles, _ = a.st.ListRoles(ctx)
	n := 0
	for _, r := range roles {
		if r.Name == "ROLE_ADMIN" {
			n++
			if r.Description != "changed" {
				t.Errorf("the description did not change: %q", r.Description)
			}
			if len(r.Tags) != 1 || r.Tags[0] != "otel-demo" {
				t.Errorf("a field left out lost its value: %v", r.Tags)
			}
		}
	}
	if n != 1 {
		t.Errorf("the same name made %d roles", n)
	}

	// A parent nobody holds says so, and says which way round it goes.
	_, err = a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_X","grantedBy":"ROLE_NOBODY"}`))
	if err == nil {
		t.Fatal("an unknown parent was accepted")
	}
	if !strings.Contains(err.Error(), "list_roles") || !strings.Contains(err.Error(), "ABOVE") {
		t.Errorf("the refusal helps nobody: %v", err)
	}
}

func roleAPI(t *testing.T) (*API, context.Context) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	// With a real router: a rename rewrites rules, and rewriting rules reloads
	// the plan - a nil router there is a panic, not a test double.
	a := &API{st: st, router: gateway.New(st, nil)}
	ctx := context.WithValue(context.Background(), mcpActorKey{}, store.User{Username: "admin", Root: true})
	return a, ctx
}

// TestRenameFollowsTheRulesThatNameTheRole: a rule names a role BY NAME, so a
// rename that did not follow them would leave rules granting nobody - silently,
// on routes nobody thought they had touched. That was the reason the catalogue
// stayed shut; it is now the reason this code exists.
func TestRenameFollowsTheRulesThatNameTheRole(t *testing.T) {
	a, ctx := roleAPI(t)
	if _, err := a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_OLD"}`)); err != nil {
		t.Fatal(err)
	}
	route := store.Route{
		ID: "r1", Name: "shop", Order: 1, Enabled: true, Upstream: "http://up",
		Access: store.Access{Level: "auth", Roles: []string{"ROLE_OLD", "other"}},
		API: &store.RouteAPI{Security: &store.EndpointSecurity{Endpoints: []store.EndpointPolicy{
			{Method: "POST", Path: "/things", Access: store.Access{Level: "auth", Roles: []string{"ROLE_OLD"}}},
		}}},
	}
	if err := a.st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}

	out, err := a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_OLD","newName":"ROLE_NEW"}`))
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if m := out.(map[string]any); m["rulesRewritten"] != 2 {
		t.Errorf("want 2 rules rewritten, got %+v", m)
	}
	saved, err := a.st.GetRoute(ctx, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if !namesRole(saved.Access.Roles, "ROLE_NEW") || namesRole(saved.Access.Roles, "ROLE_OLD") {
		t.Errorf("the route's rule did not follow: %v", saved.Access.Roles)
	}
	if !namesRole(saved.Access.Roles, "other") {
		t.Errorf("a role that was not renamed was touched: %v", saved.Access.Roles)
	}
	if got := saved.API.Security.Endpoints[0].Roles; !namesRole(got, "ROLE_NEW") {
		t.Errorf("the endpoint's rule did not follow: %v", got)
	}

	// And deleting it while a rule still names it is refused, with what to
	// change first: it would otherwise fail closed days later, elsewhere.
	_, err = a.toolDeleteRole(ctx, json.RawMessage(`{"name":"ROLE_NEW"}`))
	if err == nil {
		t.Fatal("a role two rules still name was deleted")
	}
	if !strings.Contains(err.Error(), "shop") {
		t.Errorf("the refusal does not say what names it: %v", err)
	}
}

// TestRoleReferencesAreWhatAnAgentGoesAndFixes: the list is only useful if each
// line carries what the next call needs - the route's id for get_route, and the
// method and path of the policy inside it. A label a human reads is not an
// address an agent can act on.
func TestRoleReferencesAreWhatAnAgentGoesAndFixes(t *testing.T) {
	a, ctx := roleAPI(t)
	route := store.Route{
		ID: "r1", Name: "shop", Order: 1, Enabled: true, Upstream: "http://up",
		Access: store.Access{Level: "auth", Roles: []string{"ROLE_GONE"}},
		API: &store.RouteAPI{Security: &store.EndpointSecurity{Endpoints: []store.EndpointPolicy{
			{Method: "POST", Path: "/things", Access: store.Access{Level: "auth", Roles: []string{"ROLE_GONE"}}},
		}}},
	}
	if err := a.st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}

	// A name the catalogue does not hold is a fair question: those rules grant
	// nobody, and the answer says so.
	out, err := a.toolRoleReferences(ctx, json.RawMessage(`{"name":"ROLE_GONE"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["inCatalogue"] != false {
		t.Errorf("a role nobody holds was reported as held: %+v", m)
	}
	refs := m["references"].([]roleReference)
	if len(refs) != 2 {
		t.Fatalf("want the route and its endpoint, got %+v", refs)
	}
	for _, r := range refs {
		if r.ID != "r1" {
			t.Errorf("no route id to go and fix: %+v", r)
		}
		if r.Kind == "endpoint" && (r.Method != "POST" || r.Path != "/things") {
			t.Errorf("the endpoint is not named precisely enough to change it: %+v", r)
		}
	}

	// And a role nothing names answers empty rather than failing.
	out, err = a.toolRoleReferences(ctx, json.RawMessage(`{"name":"ROLE_UNUSED"}`))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(out.(map[string]any)["references"].([]roleReference)); n != 0 {
		t.Errorf("want no reference, got %d", n)
	}
}
