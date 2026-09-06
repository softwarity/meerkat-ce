package admin

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The console and the product share one port, and one of them has to give way.
//
// Everything the API serves outside /api - the agent endpoint, the Prometheus
// exposition - is an EXACT path on the same mux the single-page app is mounted
// on, and an exact pattern beats the app's catch-all. So a console route named
// after one of them still WORKS while you click around, because the router
// never asks the server; it breaks on a reload, a bookmark, or a pasted link,
// where the browser gets the exposition instead of the app.
//
// That is a failure nobody meets until a user does, which is why it is a test.
// /metrics collided this way for exactly one afternoon.
func TestNoConsoleRouteTakesAProductPath(t *testing.T) {
	// The paths the product owns: what the control plane mounts outside /api.
	owned := map[string]bool{}
	for _, pattern := range controlPlaneSurface() {
		_, path, ok := strings.Cut(pattern, " ")
		if !ok {
			path = pattern
		}
		if !strings.HasPrefix(path, "/api/") && path != "/" {
			owned[strings.Trim(path, "/")] = true
		}
	}
	if len(owned) == 0 {
		t.Fatal("no non-/api paths found: this test would pass on anything")
	}

	// The console's TOP-LEVEL routes, read from the file that declares them.
	// Only the top level can collide: a nested route answers under its
	// parent's path, so the console's own /infra/mcp screen and the agent
	// endpoint at /mcp never meet - which the first version of this test got
	// wrong, and reported as a bug.
	//
	// Recognised by indentation, since that is what tells the two apart in the
	// source, and guarded by a count: the day the formatter moves these lines,
	// this fails loudly rather than quietly checking nothing.
	source, err := os.ReadFile(filepath.Join("..", "..", "console", "src", "app", "app.routes.ts"))
	if err != nil {
		t.Skipf("console sources not here: %v", err) // the CE mirror builds Go alone
	}
	declared := regexp.MustCompile(`(?m)^    path: '([^']*)'`).FindAllStringSubmatch(string(source), -1)
	if len(declared) < 8 {
		t.Fatalf("found %d top-level routes in app.routes.ts, which cannot be right: "+
			"the pattern stopped matching, and this test now checks nothing", len(declared))
	}
	for _, m := range declared {
		route := strings.Trim(m[1], "/")
		if route == "" {
			continue
		}
		if owned[route] {
			t.Errorf(`the console route %q is a path the control plane serves itself.

It works while you click, because the Angular router never asks the server -
and it breaks on a reload, a bookmark or a pasted link, where the browser is
answered by %s instead of the app. Rename the console route: outside /api,
these paths are the product's.`, route, "/"+route)
		}
	}
}
