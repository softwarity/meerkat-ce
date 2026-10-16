package gateway

import (
	"context"
	"reflect"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/vault"
)

// A route's oidc claims render as the jwt mechanism renders the same facts:
// a field from the Identity section, the roles through their expression - an
// array staying an array - and a fixed value as written. A fact the person
// lacks is left out.
func TestOIDCClaimsRenderLikeTheJWTMechanism(t *testing.T) {
	cfg := store.IdentityForward{Mechanism: store.IdentityOIDC, Client: "app",
		Attributes: []store.IdentityAttr{{Field: "username"}, {Field: "fullname"},
			{Field: "roles", Expr: `{{.Roles | json}}`}, {Field: "tenant"}},
		Claims: []store.OIDCClaim{
			{Name: "preferred_username", Field: "username"},
			{Name: "roles", Field: "roles"},
			{Name: "org", Field: "tenant"},
			{Name: "name", Field: "fullname"},
			{Name: "level", Value: "admin"},
		}}
	p := Person{d: identityData{UserID: "u1", Username: "jdoe", Tenant: "acme", Roles: []string{"A", "B"}}}
	got := (&Router{}).OIDCClaims(cfg, p)
	want := map[string]any{"preferred_username": "jdoe", "roles": []string{"A", "B"}, "org": "acme", "level": "admin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	// The same fact through the jwt mechanism, for comparison.
	jwt := identityClaims(store.IdentityForward{Attributes: cfg.Attributes}, "", "iss", p.d)
	if !reflect.DeepEqual(jwt["roles"], got["roles"]) || jwt["username"] != got["preferred_username"] {
		t.Fatalf("oidc %v and jwt %v disagree", got, jwt)
	}
}

// The provider sees the routes the router serves, in routing order: enabled,
// compiled, vault references expanded.
func TestOIDCRoutesAreTheServedOnes(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SaveVaultEntry(ctx, vault.Entry{Name: "level", Kind: vault.KindValue,
		Scope: vault.ScopeInfra, Value: "gold"}); err != nil {
		t.Fatal(err)
	}
	id := func(v string) *store.IdentityForward {
		return &store.IdentityForward{Mechanism: store.IdentityOIDC, Client: "app",
			Claims: []store.OIDCClaim{{Name: "level", Value: v}}}
	}
	paths := []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []string{"/a/**"}}}}
	for _, r := range []store.Route{
		{ID: "second", Name: "second", Order: 2, Enabled: true, Upstream: "http://up", Predicates: paths, Identity: id("two")},
		{ID: "first", Name: "first", Order: 1, Enabled: true, Upstream: "http://up", Predicates: paths, Identity: id("$level")},
		{ID: "off", Name: "off", Order: 3, Enabled: false, Upstream: "http://up", Predicates: paths, Identity: id("off")},
		{ID: "other", Name: "other", Order: 4, Enabled: true, Upstream: "http://up", Predicates: paths,
			Identity: &store.IdentityForward{Mechanism: store.IdentityOIDC, Client: "else"}},
	} {
		if err := st.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	rt := New(st, nil)
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	got := rt.OIDCRoutes("app")
	if len(got) != 2 || got[0].ID != "first" || got[1].ID != "second" {
		t.Fatalf("routes: %+v", got)
	}
	if v := got[0].Identity.Claims[0].Value; v != "gold" {
		t.Fatalf("the vault reference was not expanded: %q", v)
	}
}
