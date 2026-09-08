package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

func TestSanitizeRateLimitsNamesWhatIsAllowed(t *testing.T) {
	cases := []struct {
		what  string
		limit RateLimit
		says  string
	}{
		{"an invented key", RateLimit{Per: "moon", Requests: 1, Window: "PT1M"}, "route, user, token, tenant, ip"},
		{"no requests", RateLimit{Per: PerRoute, Window: "PT1M"}, "1 to"},
		{"a window nobody can hold", RateLimit{Per: PerRoute, Requests: 1, Window: "P30D"}, "outside what this counter holds"},
		{"a window that is not one", RateLimit{Per: PerRoute, Requests: 1, Window: "every minute"}, "window"},
	}
	for _, c := range cases {
		err := SanitizeRateLimits([]RateLimit{c.limit})
		if err == nil {
			t.Errorf("%s was accepted", c.what)
			continue
		}
		if !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %q does not say %q - an error that does not name what is allowed teaches nothing",
				c.what, err, c.says)
		}
	}
	// And an empty key is the ordinary one rather than a refusal: a rule with
	// a number and a window and nothing else means the whole route.
	l := []RateLimit{{Requests: 10, Window: "PT1M"}}
	if err := SanitizeRateLimits(l); err != nil {
		t.Fatalf("a rule with no key was refused: %v", err)
	}
	if l[0].Per != PerRoute {
		t.Errorf("an unspecified key became %q, want %q", l[0].Per, PerRoute)
	}
}

// Which half of a rule needs a session, and it is not only the key: a bound on
// the whole route that applies to one tier cannot be decided without knowing
// who is calling.
func TestNeedsIdentity(t *testing.T) {
	cases := []struct {
		limit RateLimit
		want  bool
	}{
		{RateLimit{Per: PerRoute}, false},
		{RateLimit{Per: PerIP}, false},
		{RateLimit{Per: PerUser}, true},
		{RateLimit{Per: PerToken}, true},
		{RateLimit{Per: PerTenant}, true},
		{RateLimit{Per: PerRoute, Applies: Access{Roles: []string{"trial"}, Level: AccessAuth}}, true},
	}
	for _, c := range cases {
		if got := c.limit.NeedsIdentity(); got != c.want {
			t.Errorf("per %q applies %+v: NeedsIdentity is %v, want %v",
				c.limit.Per, c.limit.Applies, got, c.want)
		}
	}
}

// The one that took a gateway down.
//
// `limits` was ROUTE-04's size caps, retired into the gate bricks - and
// addMissingColumns never drops anything, so every database old enough to have
// had it still carries the dead column, holding an OBJECT. Reusing the name
// meant reading rate limits out of it, which is not a migration problem: it is
// a process that will not start, on somebody's database and not on a fresh one.
func TestARetiredColumnOfTheSameNameIsNotRead(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	// The database as it stands on a machine that has been running since
	// August: the retired column, with what ROUTE-04 used to put in it.
	if _, err := st.db.Exec(`ALTER TABLE routes ADD COLUMN limits TEXT NOT NULL DEFAULT '{}'`); err != nil {
		t.Skipf("this database will not take the legacy column: %v", err)
	}
	route := Route{ID: "r1", Name: "demo", Enabled: true, Upstream: "http://x",
		Limits: []RateLimit{{Per: PerRoute, Requests: 5, Window: "PT1M"}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatalf("SaveRoute: %v", err)
	}
	if _, err := st.db.Exec(`UPDATE routes SET limits = '{"body":1048576}'`); err != nil {
		t.Fatal(err)
	}
	got, lErr := st.ListRoutes(ctx)
	if lErr != nil {
		t.Fatalf("a route beside a retired column of the same name: %v", lErr)
	}
	if len(got) != 1 || len(got[0].Limits) != 1 || got[0].Limits[0].Requests != 5 {
		t.Fatalf("the rate limits did not survive: %+v", got)
	}
}
