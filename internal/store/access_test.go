package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A rule must say on disk exactly what the engine reads. The case that bit:
// a name left in the users list of a rule that poses no condition - ignored by
// the gateway, drawn as a restriction by the console, and a way in the day
// somebody raises the level.
func TestSanitizeAccess(t *testing.T) {
	cases := []struct {
		name string
		in   Access
		want Access
	}{
		{
			name: "a name on a rule that conditions nothing is dropped",
			in:   Access{Level: AccessDelegated, Users: []string{"alice"}},
			want: Access{Level: AccessDelegated},
		},
		{
			// With a role asked, the rule DOES pose a condition, and a named
			// user is a real exception to it.
			name: "a name beside a role is an exception and stays",
			in:   Access{Level: AccessDelegated, Roles: []string{"ops"}, Users: []string{"alice"}},
			want: Access{Level: AccessDelegated, Roles: []string{"ops"}, Users: []string{"alice"}},
		},
		{
			name: "a name on a closed rule is what makes the rule mean something",
			in:   Access{Level: AccessDeny, Users: []string{"alice"}},
			want: Access{Level: AccessDeny, Users: []string{"alice"}},
		},
		{
			name: "a tenant list belongs to the level that reads it",
			in:   Access{Level: AccessAuth, Tenants: []string{"acme"}},
			want: Access{Level: AccessAuth},
		},
		{
			name: "and stays where it is read",
			in:   Access{Level: AccessTenants, Tenants: []string{"acme"}},
			want: Access{Level: AccessTenants, Tenants: []string{"acme"}},
		},
		{
			name: "blanks are not entries",
			in:   Access{Level: AccessDeny, Users: []string{"alice", "  ", ""}},
			want: Access{Level: AccessDeny, Users: []string{"alice"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			if err := SanitizeAccess(&got); err != nil {
				t.Fatal(err)
			}
			if got.Level != tc.want.Level ||
				strings.Join(got.Users, ",") != strings.Join(tc.want.Users, ",") ||
				strings.Join(got.Roles, ",") != strings.Join(tc.want.Roles, ",") ||
				strings.Join(got.Tenants, ",") != strings.Join(tc.want.Tenants, ",") {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	// An invented level used to behave like "signed in", silently. It is
	// refused, naming what is allowed.
	err := SanitizeAccess(&Access{Level: "authenticated"})
	if err == nil || !strings.Contains(err.Error(), "auth") {
		t.Errorf("an invented level answered %v, want a refusal naming the allowed ones", err)
	}
}

// The cleaning happens where every writer goes through, so an import or an
// agent cannot put back what the console cannot show.
func TestSavingARouteCleansItsRule(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	route := Route{
		ID: "ops-app", Name: "ops-app", Enabled: true, Upstream: "http://ops.invalid",
		Access: Access{Level: AccessDelegated, Users: []string{"alice"}},
		API: &RouteAPI{Security: &EndpointSecurity{Endpoints: []EndpointPolicy{
			{Method: "GET", Path: "/x", Access: Access{Level: AccessDelegated, Users: []string{"bob"}}},
		}}},
	}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	saved, err := st.GetRoute(ctx, "ops-app")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Access.Users) != 0 {
		t.Errorf("the route kept %v on a rule that conditions nothing", saved.Access.Users)
	}
	if u := saved.API.Security.Endpoints[0].Users; len(u) != 0 {
		t.Errorf("the endpoint override kept %v", u)
	}
}
