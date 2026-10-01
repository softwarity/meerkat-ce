package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// The document a person hands an agent: two routes and a name for the
// installation. Written as YAML, because that is what somebody has in a file.
const twoRoutes = `
version: 1
routes:
  - id: rabbitmq
    name: RabbitMQ
    enabled: true
    order: 110
    upstream: http://rabbitmq.internal:15672
    isUi: true
    predicates:
      - type: path
        args:
          patterns: ["/rabbitmq/**"]
  - id: jaeger
    name: Jaeger
    enabled: true
    order: 130
    upstream: http://jaeger.internal:16686
    isUi: true
    predicates:
      - type: path
        args:
          patterns: ["/jaeger/**"]
`

func routeIDs(t *testing.T, f fixture) []string {
	t.Helper()
	list, err := f.api.st.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("list routes: %v", err)
	}
	ids := make([]string, 0, len(list))
	for _, r := range list {
		ids = append(ids, r.ID)
	}
	return ids
}

// An agent sets an installation up from a file, which is the whole point of the
// tool: setting up a gateway was a thing only a person could do, through the
// console, while everything the file contains had a tool of its own.
func TestImportConfigurationAppliesADocument(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))

	// A route that the document does not mention, to prove the default is a
	// merge: an import must not quietly delete what it never knew about.
	if err := f.api.st.SaveRoute(ctx, store.Route{
		ID: "keepme", Name: "keep me", Enabled: true, Upstream: "http://x.internal",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/keep/**"}}}},
	}); err != nil {
		t.Fatalf("seed a route: %v", err)
	}

	args, _ := json.Marshal(map[string]any{"document": twoRoutes})
	out, err := f.api.toolImportConfiguration(ctx, args)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	body, _ := json.Marshal(out)
	if !strings.Contains(string(body), `"rabbitmq"`) || !strings.Contains(string(body), `"jaeger"`) {
		t.Errorf("the answer must name what it changed: %s", body)
	}
	ids := routeIDs(t, f)
	for _, want := range []string{"rabbitmq", "jaeger", "keepme"} {
		if !contains(ids, want) {
			t.Errorf("after a merge the gateway must hold %q: %v", want, ids)
		}
	}
}

// prune is the other half, and it is opt-in: aligning the gateway ON the
// document is what resets an installation, and it is not what "import" means by
// default.
func TestImportConfigurationPrunesOnlyWhenAsked(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))
	if err := f.api.st.SaveRoute(ctx, store.Route{
		ID: "leftover", Name: "leftover", Enabled: true, Upstream: "http://x.internal",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/left/**"}}}},
	}); err != nil {
		t.Fatalf("seed a route: %v", err)
	}
	args, _ := json.Marshal(map[string]any{"document": twoRoutes, "prune": true})
	if _, err := f.api.toolImportConfiguration(ctx, args); err != nil {
		t.Fatalf("import: %v", err)
	}
	ids := routeIDs(t, f)
	if contains(ids, "leftover") {
		t.Errorf("prune must remove what the document does not carry: %v", ids)
	}
	if len(ids) != 2 {
		t.Errorf("the gateway must hold the document's two routes and nothing else: %v", ids)
	}
}

// A dry run answers the same plan and touches nothing - what an agent asks for
// when the document did not come from this gateway.
func TestImportConfigurationDryRunChangesNothing(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))
	args, _ := json.Marshal(map[string]any{"document": twoRoutes, "dryRun": true})
	out, err := f.api.toolImportConfiguration(ctx, args)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	body, _ := json.Marshal(out)
	if !strings.Contains(string(body), `"rabbitmq"`) {
		t.Errorf("a dry run must say what it would do: %s", body)
	}
	if ids := routeIDs(t, f); len(ids) != 0 {
		t.Errorf("a dry run wrote something: %v", ids)
	}
}

// A package cannot travel through a tool call, and the refusal has to say what
// to do instead - otherwise the YAML parser reports that the first byte is not
// a key, which sends nobody anywhere.
func TestImportConfigurationRefusesAPackageWithAWayOut(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))
	args, _ := json.Marshal(map[string]any{"document": "PK\x03\x04 and then some bytes"})
	_, err := f.api.toolImportConfiguration(ctx, args)
	if err == nil {
		t.Fatal("a ZIP was accepted as a document")
	}
	if !strings.Contains(err.Error(), "console") {
		t.Errorf("the refusal must name the way out: %v", err)
	}
}

// Switching to a saved configuration, by the name a person says out loud.
func TestActivateConfigurationSwitchesByName(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))

	// The state worth coming back to, saved under a name.
	if err := f.api.st.SaveRoute(ctx, store.Route{
		ID: "before", Name: "before", Enabled: true, Upstream: "http://x.internal",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/before/**"}}}},
	}); err != nil {
		t.Fatalf("seed a route: %v", err)
	}
	saveArgs, _ := json.Marshal(map[string]any{"name": "before the two routes"})
	if _, err := f.api.toolSaveConfiguration(ctx, saveArgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Then the gateway moves on.
	importArgs, _ := json.Marshal(map[string]any{"document": twoRoutes, "prune": true})
	if _, err := f.api.toolImportConfiguration(ctx, importArgs); err != nil {
		t.Fatalf("import: %v", err)
	}
	if ids := routeIDs(t, f); contains(ids, "before") {
		t.Fatalf("the import did not replace the state: %v", ids)
	}

	// And comes back, by name.
	actArgs, _ := json.Marshal(map[string]any{"name": "before the two routes"})
	if _, err := f.api.toolActivateConfiguration(ctx, actArgs); err != nil {
		t.Fatalf("activate: %v", err)
	}
	ids := routeIDs(t, f)
	if !contains(ids, "before") || contains(ids, "jaeger") {
		t.Errorf("activating replaces the WHOLE state: %v", ids)
	}
}

// A name nobody saved is answered with the names that exist: an agent that
// guessed wrong has what it needs to guess right, in the same answer.
func TestActivateConfigurationNamesWhatExists(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))
	saveArgs, _ := json.Marshal(map[string]any{"name": "the good one"})
	if _, err := f.api.toolSaveConfiguration(ctx, saveArgs); err != nil {
		t.Fatalf("save: %v", err)
	}
	args, _ := json.Marshal(map[string]any{"name": "the other one"})
	_, err := f.api.toolActivateConfiguration(ctx, args)
	if err == nil {
		t.Fatal("a configuration that does not exist was activated")
	}
	if !strings.Contains(err.Error(), "the good one") {
		t.Errorf("the refusal must list what this gateway holds: %v", err)
	}
}
