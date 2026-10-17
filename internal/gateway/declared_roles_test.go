package gateway

import (
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A route that maps the roles its application declares sends those by
// default; one that writes its own expression keeps it.
func TestAMappedRouteSendsTheDeclaredRolesByDefault(t *testing.T) {
	d := identityData{Roles: []string{"demo-ROLE_ADMIN", "billing"},
		Declared: map[string]string{"ROLE_ADMIN": "demo-ROLE_ADMIN", "ROLE_USER": "demo-ROLE_USER"}}
	if got, _ := renderRoles("", d); got != "ROLE_ADMIN" {
		t.Errorf("default on a mapped route %q, want ROLE_ADMIN", got)
	}
	if got, _ := renderRoles(`{{join ";" .Roles}}`, d); got != "demo-ROLE_ADMIN;billing" {
		t.Errorf("an expression of its own %q, want it obeyed", got)
	}
	d.Declared = nil
	if got, _ := renderRoles("", d); got != "demo-ROLE_ADMIN,billing" {
		t.Errorf("default without a map %q, want every role", got)
	}
}

// The map is checked with the route: a catalogue name a session can never
// carry, or a declared name that would split a header list, is refused.
func TestARoleMapIsValidatedWithTheRoute(t *testing.T) {
	for name, m := range map[string]map[string]string{
		"catalogue name with a colon": {"ROLE_ADMIN": "demo:admin"},
		"declared name with a comma":  {"ROLE_A,ROLE_B": "demo-admin"},
	} {
		r := store.Route{API: &store.RouteAPI{RoleMap: m}}
		if err := validateRouteType(r); err == nil {
			t.Errorf("%s: accepted, want a refusal", name)
		}
	}
	ok := store.Route{API: &store.RouteAPI{RoleMap: map[string]string{"orders:read": "demo-orders_read"}}}
	if err := validateRouteType(ok); err != nil {
		t.Errorf("a scope-shaped declared role refused: %v", err)
	}
}
