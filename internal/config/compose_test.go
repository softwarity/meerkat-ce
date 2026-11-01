package config

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// Two sides with differences at every level: an object on one side only, an
// object that differs by a field, a list, a nested field; a reference one
// side cannot satisfy; the same route under two ids.
func composeSides() (*Document, *Document) {
	left := &Document{
		Version: Version,
		Roles: []store.Role{
			{Name: "ROLE_USER"}, {Name: "ROLE_OPS", Parent: "ROLE_USER", Description: "ops"},
			{Name: "ROLE_LEGACY", Parent: "ROLE_USER"},
		},
		Tenants: []store.Tenant{{ID: "t-acme", Name: "Acme", Enabled: true}},
		Groups:  []store.Group{{ID: "g-ops", TenantID: "t-acme", Name: "Operators", Roles: []string{"ROLE_OPS"}}},
		Routes: []store.Route{
			{ID: "r-checkout", Name: "checkout", Enabled: true, Order: 20, Upstream: "http://checkout:8080",
				Predicates: []routing.Spec{{Type: "Path", Args: map[string]any{"pattern": "/checkout/**"}}},
				Access:     store.Access{Level: "roles", Roles: []string{"ROLE_USER"}},
				Identity:   &store.IdentityForward{Mechanism: "signed-jwt", TTL: "PT2M"}},
			{ID: "r-legacy", Name: "legacy", Enabled: false, Order: 50, Upstream: "http://old:8080",
				Predicates: []routing.Spec{{Type: "Path", Args: map[string]any{"pattern": "/old/**"}}},
				Access:     store.Access{Level: "roles", Roles: []string{"ROLE_LEGACY"}}},
			{ID: "r-123", Name: "status", Enabled: true, Order: 70, Upstream: "http://status:8080",
				Predicates: []routing.Spec{{Type: "Path", Args: map[string]any{"pattern": "/status"}}}},
		},
		Settings: map[string]json.RawMessage{
			"tls": json.RawMessage(`{"mode":"gateway","names":["gw.local","portal.local"]}`),
		},
	}
	right := &Document{
		Version: Version,
		Roles: []store.Role{
			{Name: "ROLE_USER"}, {Name: "ROLE_OPS", Parent: "ROLE_USER", Description: "ops (acme)"},
			{Name: "ROLE_AUDIT", Parent: "ROLE_USER"}, {Name: "ROLE_BILLING", Parent: "ROLE_USER"},
		},
		Tenants: []store.Tenant{{ID: "t-acme", Name: "Acme", Enabled: true}},
		Groups:  []store.Group{{ID: "g-ops", TenantID: "t-acme", Name: "Operators", Roles: []string{"ROLE_OPS", "ROLE_AUDIT"}}},
		Routes: []store.Route{
			{ID: "r-checkout", Name: "checkout", Enabled: true, Order: 20, Upstream: "http://checkout-v2:8080",
				Predicates: []routing.Spec{{Type: "Path", Args: map[string]any{"pattern": "/checkout/**"}}, {Type: "Host", Args: map[string]any{"pattern": "shop.acme.io"}}},
				Access:     store.Access{Level: "roles", Roles: []string{"ROLE_USER", "ROLE_AUDIT"}},
				Identity:   &store.IdentityForward{Mechanism: "signed-jwt", TTL: "PT5M", Audience: "checkout"}},
			{ID: "r-billing", Name: "billing", Enabled: true, Order: 45, Upstream: "http://billing:8080",
				Predicates: []routing.Spec{{Type: "Path", Args: map[string]any{"pattern": "/billing/**"}}},
				Access:     store.Access{Level: "roles", Roles: []string{"ROLE_BILLING", "ROLE_SUPPORT"}}},
			{ID: "r-9f0", Name: "status", Enabled: true, Order: 70, Upstream: "http://status:8080",
				Predicates: []routing.Spec{{Type: "Path", Args: map[string]any{"pattern": "/status"}}}},
		},
		Settings: map[string]json.RawMessage{
			"tls": json.RawMessage(`{"mode":"gateway","names":["gw.acme.io","portal.local"]}`),
		},
	}
	return left, right
}

func routeByID(t *testing.T, doc *Document, id string) store.Route {
	t.Helper()
	for _, r := range doc.Routes {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("route %s not in the result: %v", id, routeIDs(doc))
	return store.Route{}
}

func routeIDs(doc *Document) []string {
	var out []string
	for _, r := range doc.Routes {
		out = append(out, r.ID)
	}
	return out
}

func roleNames(doc *Document) []string {
	var out []string
	for _, r := range doc.Roles {
		out = append(out, r.Name)
	}
	return out
}

func TestComposeStartsAsTheLeftSide(t *testing.T) {
	left, right := composeSides()
	doc, report, err := Compose(left, right, Recipe{})
	if err != nil {
		t.Fatal(err)
	}
	if got := routeIDs(doc); !slices.Equal(got, []string{"r-checkout", "r-legacy", "r-123"}) {
		t.Errorf("routes = %v, want the left side's", got)
	}
	if r := routeByID(t, doc, "r-checkout"); r.Upstream != "http://checkout:8080" {
		t.Errorf("checkout upstream = %s, want the left one", r.Upstream)
	}
	if len(report.Pulled) != 0 || len(report.Missing) != 0 {
		t.Errorf("a result that is the left side pulls and misses nothing: %+v", report)
	}
	// The same name under two ids is proposed, never paired by itself.
	if len(report.Pairs) != 1 || report.Pairs[0].Left != "r-123" || report.Pairs[0].Right != "r-9f0" {
		t.Errorf("pairs = %+v, want status r-123 / r-9f0", report.Pairs)
	}
}

func TestComposeTakesAnObjectAndPullsWhatItNames(t *testing.T) {
	left, right := composeSides()
	doc, report, err := Compose(left, right, Recipe{Picks: []Pick{{Path: "route:r-billing", From: "right"}}})
	if err != nil {
		t.Fatal(err)
	}
	routeByID(t, doc, "r-billing")
	if !slices.Contains(roleNames(doc), "ROLE_BILLING") {
		t.Errorf("roles = %v, want ROLE_BILLING pulled along with billing", roleNames(doc))
	}
	if len(report.Pulled) != 1 || report.Pulled[0].ID != "ROLE_BILLING" || report.Pulled[0].From != "right" || !slices.Equal(report.Pulled[0].By, []string{"billing"}) {
		t.Errorf("pulled = %+v", report.Pulled)
	}
	// ROLE_SUPPORT is declared on neither side: said, not refused, and soft.
	if len(report.Missing) != 1 || report.Missing[0].Key != "ROLE_SUPPORT" || report.Missing[0].Hard {
		t.Errorf("missing = %+v, want ROLE_SUPPORT, soft", report.Missing)
	}
	if err := report.Blocked(); err != nil {
		t.Errorf("a soft miss does not block: %v", err)
	}
}

func TestComposeLeavesAPulledDependencyOut(t *testing.T) {
	left, right := composeSides()
	_, report, err := Compose(left, right, Recipe{Picks: []Pick{
		{Path: "route:r-billing", From: "right"},
		{Path: "role:ROLE_BILLING", From: "none"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Pulled) != 0 {
		t.Errorf("pulled = %+v, want nothing: the operator said none", report.Pulled)
	}
	var keys []string
	for _, m := range report.Missing {
		keys = append(keys, m.Key)
	}
	if !slices.Contains(keys, "ROLE_BILLING") {
		t.Errorf("missing = %v, want ROLE_BILLING said as left out", keys)
	}
}

func TestComposeAHardDependencyBlocks(t *testing.T) {
	left, right := composeSides()
	// ROLE_AUDIT taken, its parent ROLE_USER left out: the store would refuse
	// the role, so Blocked says so.
	_, report, err := Compose(left, right, Recipe{Picks: []Pick{
		{Path: "role:ROLE_AUDIT", From: "right"},
		{Path: "role:ROLE_USER", From: "none"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Blocked(); err == nil || !strings.Contains(err.Error(), "ROLE_USER") {
		t.Errorf("Blocked = %v, want the parent named", err)
	}
}

func TestComposeFieldsFromEitherSide(t *testing.T) {
	left, right := composeSides()
	doc, report, err := Compose(left, right, Recipe{Picks: []Pick{
		{Path: "route:r-checkout/upstream", From: "right"},
		{Path: "route:r-checkout/identity.audience", From: "right"},
		{Path: "route:r-checkout/access.roles[ROLE_AUDIT]", Op: "add"},
		{Path: "setting:tls/names[gw.acme.io]", Op: "add"},
		{Path: "setting:tls/names[portal.local]", Op: "drop"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	r := routeByID(t, doc, "r-checkout")
	if r.Upstream != "http://checkout-v2:8080" {
		t.Errorf("upstream = %s, want the right one", r.Upstream)
	}
	if r.Identity == nil || r.Identity.TTL != "PT2M" || r.Identity.Audience != "checkout" {
		t.Errorf("identity = %+v, want ttl from the left and audience from the right", r.Identity)
	}
	if !slices.Equal(r.Access.Roles, []string{"ROLE_USER", "ROLE_AUDIT"}) {
		t.Errorf("roles = %v, want ROLE_AUDIT added after the left's", r.Access.Roles)
	}
	if !slices.Contains(roleNames(doc), "ROLE_AUDIT") {
		t.Errorf("roles = %v, want ROLE_AUDIT pulled: checkout names it now", roleNames(doc))
	}
	var tls struct {
		Names []string `json:"names"`
	}
	_ = json.Unmarshal(doc.Settings["tls"], &tls)
	if !slices.Equal(tls.Names, []string{"gw.local", "gw.acme.io"}) {
		t.Errorf("tls names = %v, want gw.local, gw.acme.io", tls.Names)
	}
	if len(report.Unresolved) != 0 {
		t.Errorf("unresolved = %v", report.Unresolved)
	}
}

func TestComposeUnsetAndOrder(t *testing.T) {
	left, right := composeSides()
	host := elementKey(map[string]any{"type": "Host", "args": map[string]any{"pattern": "shop.acme.io"}})
	path := elementKey(map[string]any{"type": "Path", "args": map[string]any{"pattern": "/checkout/**"}})
	doc, _, err := Compose(left, right, Recipe{Picks: []Pick{
		{Path: "route:r-checkout/identity", From: "unset"},
		{Path: "route:r-checkout/predicates[" + host + "]", Op: "add"},
		{Path: "route:r-checkout/predicates", Order: []string{host, path}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	r := routeByID(t, doc, "r-checkout")
	if r.Identity != nil {
		t.Errorf("identity = %+v, want unset", r.Identity)
	}
	if len(r.Predicates) != 2 || r.Predicates[0].Type != "Host" || r.Predicates[1].Type != "Path" {
		t.Errorf("predicates = %+v, want Host then Path", r.Predicates)
	}
}

func TestComposePairKeepsTheLeftID(t *testing.T) {
	left, right := composeSides()
	doc, report, err := Compose(left, right, Recipe{
		Pairs: []Pair{{Kind: "route", Left: "r-123", Right: "r-9f0"}},
		Picks: []Pick{{Path: "route:r-123", From: "right"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := routeIDs(doc)
	if !slices.Contains(ids, "r-123") || slices.Contains(ids, "r-9f0") {
		t.Errorf("routes = %v, want r-123 kept and r-9f0 gone", ids)
	}
	if len(report.Pairs) != 0 {
		t.Errorf("pairs = %+v, want none: it is paired", report.Pairs)
	}
}

func TestComposeSaysWhatDoesNotResolve(t *testing.T) {
	left, right := composeSides()
	doc, report, err := Compose(left, right, Recipe{Picks: []Pick{
		{Path: "route:r-nowhere", From: "right"},
		{Path: "route:r-checkout/nothing.here[x]", Op: "add"},
		{Path: "nonsense"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Unresolved) != 3 {
		t.Errorf("unresolved = %v, want three", report.Unresolved)
	}
	if got := routeIDs(doc); !slices.Equal(got, routeIDs(left)) {
		t.Errorf("a recipe that resolves nowhere answers the left side: %v", got)
	}
}

func TestComposeKeepsTheMediaWithTheRoute(t *testing.T) {
	left, right := composeSides()
	left.Specs = map[string][]byte{"r-checkout": []byte("left spec")}
	right.Specs = map[string][]byte{"r-checkout": []byte("right spec"), "r-billing": []byte("billing spec")}
	doc, _, err := Compose(left, right, Recipe{Picks: []Pick{{Path: "route:r-billing", From: "right"}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(doc.Specs["r-checkout"]) != "left spec" || string(doc.Specs["r-billing"]) != "billing spec" {
		t.Errorf("specs = %v, want each route's from the side it came from", doc.Specs)
	}
}

func TestComposeResultIsAConfiguration(t *testing.T) {
	left, right := composeSides()
	doc, _, err := Compose(left, right, Recipe{Picks: []Pick{{Path: "route:r-billing", From: "right"}}})
	if err != nil {
		t.Fatal(err)
	}
	file, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Unmarshal(file)
	if err != nil {
		t.Fatalf("the composed document does not read back: %v\n%s", err, file)
	}
	if len(back.Routes) != 4 || len(back.Roles) != 4 {
		t.Errorf("read back: %d routes, %d roles, want 4 and 4", len(back.Routes), len(back.Roles))
	}
}
