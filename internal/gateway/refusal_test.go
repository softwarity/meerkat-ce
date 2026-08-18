package gateway

import (
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A refusal that sends someone to the organisation chooser has to say what it
// wanted. Without it the page reappears with nothing on it, and the only way
// to find out is to read the database - which is exactly how this was found.
func TestARefusalNamesWhatItWanted(t *testing.T) {
	member := store.Caller{Authenticated: true, TenantID: "globex", Memberships: []string{"globex", "acme"}}
	cases := []struct {
		name string
		a    store.Access
		want string
	}{
		{"reserved to another organisation",
			store.Access{Level: store.AccessTenants, Tenants: []string{"acme"}, Roles: []string{"auditor"}},
			"tenant"},
		{"right organisation, missing roles",
			store.Access{Level: store.AccessTenants, Tenants: []string{"globex"}, Roles: []string{"auditor"}},
			"roles"},
		{"a named list",
			store.Access{Level: store.AccessAuth, Users: []string{"someone-else"}},
			"user"},
	}
	for _, c := range cases {
		if got := refusalCode(c.a, member); got != c.want {
			t.Errorf("%s: code = %q, want %q", c.name, got, c.want)
		}
	}
}
