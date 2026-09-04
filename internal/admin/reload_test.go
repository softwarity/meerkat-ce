package admin

import (
	"os"
	"strings"
	"testing"
)

// A node that reloads without announcing is the bug the change bus was
// written to close: its own routes are right, everyone else's are stale, and
// the operator is looking at the node that works.
//
// It comes back one endpoint at a time - somebody adds a handler, copies the
// two lines above it, and the copy is a year old. So the pair lives in
// reload.go and this refuses every other way to it. Same shape as the MCP
// coverage test: the mistake is not caught by a reviewer, it is caught here.
func TestNothingReloadsBehindTheBusBack(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "reload.go" {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			// certificates.go owns applyTLS, which is the funnel for the other
			// topic: it reloads AND announces, in one place, for the same
			// reason.
			if strings.Contains(line, "a.TLS.Reload(") && name != "certificates.go" {
				t.Errorf("%s:%d reloads TLS directly - go through applyTLS, which also announces", name, i+1)
			}
			if strings.Contains(line, "a.router.Reload(") {
				t.Errorf("%s:%d reloads the routes directly - call a.reloadRouting, "+
					"which also tells the other nodes:\n  %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
