package config

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// The OIDC clients (SAUTH-05) travel in a configuration, and the hash of
// their secret with them: the one place it leaves the database, so that an
// application keeps its secret wherever the configuration lands.

// oidcRoute is a route that signs people into client.
func oidcRoute(id, client string) store.Route {
	return store.Route{
		ID: id, Name: id, Order: 1, Enabled: true, Upstream: "http://app:8080",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/" + id + "/**"}}}},
		Identity:   &store.IdentityForward{Mechanism: store.IdentityOIDC, Client: client},
	}
}

// confidentialClient declares a client with a drawn secret, and returns it.
func confidentialClient(t *testing.T, st *store.Store, id string) string {
	t.Helper()
	secret, hash, prefix, err := store.NewOIDCSecret()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateOIDCClient(context.Background(), store.OIDCClient{
		ID: id, Confidential: true, SecretHash: hash, SecretPrefix: prefix, Audience: id + "-api",
	}); err != nil {
		t.Fatalf("CreateOIDCClient: %v", err)
	}
	return secret
}

// travel exports from, and reads the file back the way an import does.
func travel(t *testing.T, from *store.Store) *Document {
	t.Helper()
	doc, _, err := Export(context.Background(), from)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	file, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	back, err := Unmarshal(file)
	if err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, file)
	}
	return back
}

func TestAnExportCarriesTheOIDCClientsWithTheHashOfTheirSecret(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	confidentialClient(t, st, "app")
	if _, err := st.CreateOIDCClient(ctx, store.OIDCClient{ID: "spa", Confidential: false}); err != nil {
		t.Fatal(err)
	}
	stored, _ := st.GetOIDCClient(ctx, "app")

	doc, _, err := Export(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	file, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(file)
	if !strings.Contains(text, "oidcClients:") || !strings.Contains(text, "secretHash: "+stored.SecretHash) {
		t.Fatalf("the export must carry the clients and the hash of the secret:\n%s", text)
	}
	if strings.Count(text, "secretHash:") != 1 {
		t.Errorf("a public client carries no hash:\n%s", text)
	}
	if !slices.ContainsFunc(Inventory(doc), func(s Section) bool { return s.Kind == "oidcClient" && s.Count == 2 }) {
		t.Errorf("the inventory must count two OIDC clients, got %+v", Inventory(doc))
	}
	// The store's own type still never serialises it: the document's type is
	// the only door out.
	raw, _ := json.Marshal(stored)
	if strings.Contains(string(raw), stored.SecretHash) {
		t.Errorf("store.OIDCClient serialised the hash: %s", raw)
	}
}

func TestAnImportedOIDCClientKeepsItsSecret(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("the OIDC provider is Enterprise: the community image leaves the clients out")
	}
	ctx := context.Background()
	from := openTemp(t)
	secret := confidentialClient(t, from, "app")
	if err := from.SaveRoute(ctx, oidcRoute("app", "app")); err != nil {
		t.Fatal(err)
	}

	to := openTemp(t)
	plan, err := Apply(ctx, to, travel(t, from), false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := to.GetOIDCClient(ctx, "app")
	if err != nil {
		t.Fatalf("the client was not imported: %v (plan %+v)", err, plan.Changes)
	}
	if !got.SecretMatches(secret) {
		t.Error("the secret drawn on the first gateway must verify on the second")
	}
	if got.Audience != "app-api" || !strings.HasPrefix(secret, got.SecretPrefix) {
		t.Errorf("imported client: %+v", got)
	}
	// Imported again, nothing moves.
	again, err := Preview(ctx, to, travel(t, from), false)
	if err != nil {
		t.Fatal(err)
	}
	if again.Touches() {
		t.Errorf("a re-import of the same document must change nothing, got %+v", again.Changes)
	}
}

func TestAConfidentialClientWithoutHashKeepsItsSecretOrHasNone(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("the OIDC provider is Enterprise")
	}
	ctx := context.Background()
	st := openTemp(t)
	secret := confidentialClient(t, st, "app")
	doc, err := Unmarshal([]byte(`{"version":1,"oidcClients":[
		{"id":"app","confidential":true,"tokenTtl":"PT10M"},
		{"id":"new","confidential":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, st, doc, false); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	app, _ := st.GetOIDCClient(ctx, "app")
	if !app.SecretMatches(secret) || app.TokenTTL != "PT10M" {
		t.Errorf("an update without hash keeps the secret in place: %+v", app)
	}
	created, err := st.GetOIDCClient(ctx, "new")
	if err != nil {
		t.Fatal(err)
	}
	if !created.Confidential || created.SecretHash != "" || created.SecretPrefix != "" {
		t.Errorf("a client created without hash has no secret yet: %+v", created)
	}
}

func TestPruneRemovesTheOIDCClientsADocumentDoesNotCarry(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("the OIDC provider is Enterprise")
	}
	ctx := context.Background()
	st := openTemp(t)
	confidentialClient(t, st, "old")
	doc := func() *Document {
		d, err := Unmarshal([]byte(`{"version":1,"oidcClients":[{"id":"fresh","confidential":false}]}`))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if _, err := Apply(ctx, st, doc(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOIDCClient(ctx, "old"); err != nil {
		t.Fatalf("a merge must leave the other clients alone: %v", err)
	}
	plan, err := Apply(ctx, st, doc(), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOIDCClient(ctx, "old"); !errors.Is(err, store.ErrNoOIDCClient) {
		t.Errorf("prune must remove the client the document does not carry, got %v", err)
	}
	if !slices.ContainsFunc(plan.Changes, func(c Change) bool {
		return c.Kind == "oidcClient" && c.ID == "old" && c.Action == ActionRemove
	}) {
		t.Errorf("the plan must say the client goes: %+v", plan.Changes)
	}
}

// A route and the client it names, in one document: the client is written
// first, and the route lands bound to it.
func TestARouteBoundToAClientOfTheSameDocumentImports(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	doc := &Document{
		Version:     Version,
		OIDCClients: []OIDCClient{{ID: "app", Confidential: false}},
		Routes:      []store.Route{oidcRoute("app", "app")},
	}
	plan, err := Apply(ctx, st, doc, false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	r, err := st.GetRoute(ctx, "app")
	if err != nil || r.Identity == nil || r.Identity.Client != "app" {
		t.Fatalf("the route must be imported with its client: %+v, %v", r, err)
	}
	_, clientErr := st.GetOIDCClient(ctx, "app")
	if edition.Enterprise {
		if clientErr != nil {
			t.Fatalf("the client must be imported: %v", clientErr)
		}
		first := slices.IndexFunc(plan.Changes, func(c Change) bool { return c.Kind == "oidcClient" })
		route := slices.IndexFunc(plan.Changes, func(c Change) bool { return c.Kind == "route" })
		if first < 0 || first > route {
			t.Errorf("the clients come before the routes: %+v", plan.Changes)
		}
	} else if clientErr == nil {
		t.Error("the community image must not import a client")
	}
}

// An import never breaks a route's sign-in, and never refuses for it either: a
// configuration exported before clients travelled (routes, no oidcClients)
// must still apply. A route naming a client nobody declares imports and is
// said in the plan; a client a route still uses is kept by a prune.
func TestAnImportKeepsWhatARouteSignsInto(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("the community image does not serve the mechanism, and checks nothing about it")
	}
	ctx := context.Background()
	st := openTemp(t)
	plan, err := Apply(ctx, st, &Document{Version: Version, Routes: []store.Route{oidcRoute("app", "ghost")}}, false)
	if err != nil {
		t.Fatalf("a route naming an undeclared client must import as it did before: %v", err)
	}
	if len(plan.Unbound) != 1 || !strings.Contains(plan.Unbound[0], `"ghost"`) {
		t.Errorf("the plan must name the missing client, got %v", plan.Unbound)
	}

	confidentialClient(t, st, "app")
	if err := st.SaveRoute(ctx, oidcRoute("app", "app")); err != nil {
		t.Fatal(err)
	}
	// A prune by a document listing other clients.
	plan, err = Apply(ctx, st, &Document{Version: Version, OIDCClients: []OIDCClient{{ID: "other"}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetOIDCClient(ctx, "app"); err != nil {
		t.Errorf("a client a route still uses must survive a prune: %v", err)
	}
	if len(plan.Unbound) != 1 || !strings.Contains(plan.Unbound[0], `"app"`) {
		t.Errorf("the plan must say the client was kept, got %v", plan.Unbound)
	}
	// An older configuration: its routes, no oidcClients section, activated.
	if _, err := Switch(ctx, st, &Document{Version: Version, Routes: []store.Route{oidcRoute("app", "app")}}); err != nil {
		t.Fatalf("activating a configuration exported before clients travelled: %v", err)
	}
	if _, err := st.GetOIDCClient(ctx, "app"); err != nil {
		t.Errorf("activating an older configuration must not take the sign-in away: %v", err)
	}
}

func TestTheCommunityImageLeavesTheOIDCClientsOut(t *testing.T) {
	if edition.Enterprise {
		t.Skip("the community image's behaviour")
	}
	ctx := context.Background()
	st := openTemp(t)
	confidentialClient(t, st, "kept")
	doc, err := Unmarshal([]byte(`{"version":1,"oidcClients":[{"id":"app","confidential":false}],
		"roles":[{"name":"A"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Apply(ctx, st, doc, true)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !slices.ContainsFunc(plan.NotApplied, func(s string) bool { return strings.Contains(s, "OIDC client") }) {
		t.Errorf("the plan must say the clients were left out, got %q", plan.NotApplied)
	}
	if _, err := st.GetOIDCClient(ctx, "app"); err == nil {
		t.Error("a client was imported on the community image")
	}
	if _, err := st.GetOIDCClient(ctx, "kept"); err != nil {
		t.Errorf("prune must not remove the clients the community image does not manage: %v", err)
	}
	if _, err := st.GetRole(ctx, "A"); err != nil {
		t.Errorf("the rest of the document must be applied: %v", err)
	}
}

func TestAnOIDCClientsSecretHashIsChecked(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"oidcClients":[{"id":"app","confidential":true,"secretHash":"not-a-hash"}]}`,
		`{"version":1,"oidcClients":[{"id":"app","confidential":false,"secretHash":"` + strings.Repeat("a", 64) + `"}]}`,
		`{"version":1,"oidcClients":[{"id":"app","confidential":true,"secretPrefix":"mko_abc"}]}`,
		`{"version":1,"oidcClients":[{"id":"App!","confidential":false}]}`,
	} {
		doc, err := Unmarshal([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Preview(context.Background(), openTemp(t), doc, false); err == nil {
			t.Errorf("%s: want a refusal", body)
		}
	}
}

// Keeping an OIDC client out of a plan keeps the gateway's own.
func TestKeepingAnOIDCClientOut(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	confidentialClient(t, st, "app")
	refs, err := ParseRefs([]string{"oidcClient:app"})
	if err != nil {
		t.Fatal(err)
	}
	doc := &Document{Version: Version, OIDCClients: []OIDCClient{}}
	out, err := Keep(ctx, st, doc, refs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.OIDCClients) != 1 || out.OIDCClients[0].ID != "app" || out.OIDCClients[0].SecretHash == "" {
		t.Errorf("the kept client must come back whole, got %+v", out.OIDCClients)
	}
}

func TestCompareNamesTheOIDCClients(t *testing.T) {
	a := &Document{Version: Version, OIDCClients: []OIDCClient{{ID: "app", Confidential: true}}}
	b := &Document{Version: Version, OIDCClients: []OIDCClient{{ID: "app", Confidential: true, TokenTTL: "PT10M"}}}
	changes := Compare(a, b)
	if len(changes) != 1 || changes[0].Kind != "oidcClient" || changes[0].Action != ActionUpdate ||
		!slices.Equal(changes[0].Fields, []string{"tokenTtl"}) {
		t.Errorf("want one OIDC client updated on tokenTtl, got %+v", changes)
	}
}
