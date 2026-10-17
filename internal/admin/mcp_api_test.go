package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// agentCtx is a tool call's context: who the agent is, and the credentials
// its /mcp request came with.
func agentCtx(u store.User, cookie *http.Cookie) context.Context {
	ctx := context.WithValue(context.Background(), mcpActorKey{}, u)
	return context.WithValue(ctx, mcpCredsKey{}, http.Header{"Cookie": {cookie.String()}})
}

func apiTool(t *testing.T, a *API, name string) func(context.Context, string) (any, error) {
	t.Helper()
	for _, tl := range a.apiTools() {
		if tl.Name == name {
			return func(ctx context.Context, args string) (any, error) { return tl.Call(ctx, json.RawMessage(args)) }
		}
	}
	t.Fatalf("no tool %s", name)
	return nil
}

// An agent acts on the control plane through the endpoints themselves: their
// guards decide, the trail names the caller, and what is closed is closed.
func TestAnAgentActsThroughTheControlPlane(t *testing.T) {
	f := setupBare(t)
	root := agentCtx(store.User{ID: "root", Username: "root", Root: true}, f.rootC)
	read, call, list := apiTool(t, f.api, "read_admin_api"), apiTool(t, f.api, "call_admin_api"), apiTool(t, f.api, "list_admin_api")

	out, err := call(root, `{"method": "POST", "path": "/api/roles", "body": {"name": "AGENT_MADE"}}`)
	if err != nil || out.(map[string]any)["status"].(int) >= 300 {
		t.Fatalf("a role created by the agent: %v %v", out, err)
	}
	out, err = read(root, `{"path": "/api/roles"}`)
	if err != nil || !strings.Contains(mustJSON(t, out), "AGENT_MADE") {
		t.Fatalf("read back: %v %v", out, err)
	}
	events, err := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if strings.HasPrefix(e.Action, "role.") && e.ActorName == "root" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the trail does not name the agent's account: %+v", events)
	}

	// The endpoint's guard decides: bob administers nothing.
	bob := agentCtx(store.User{ID: "bob", Username: "bob"}, f.plainC)
	if _, err := call(bob, `{"method": "POST", "path": "/api/roles", "body": {"name": "BOB_MADE"}}`); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("a plain account's write: %v", err)
	}

	// What is closed is closed, with its reason, and said in the listing.
	for _, c := range []struct{ args, says string }{
		{`{"method": "GET", "path": "/api/vault"}`, "references, never values"},
		{`{"method": "PUT", "path": "/api/settings/agent", "body": {}}`, "not a question an agent answers"},
		{`{"method": "POST", "path": "/api/admin-tokens", "body": {"name": "x"}}`, "perimeter"},
		{`{"method": "GET", "path": "/api/nothing-here"}`, "no endpoint answers"},
		{`{"method": "GET", "path": "/mcp"}`, "starts with /api/"},
	} {
		if _, err := call(root, c.args); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Fatalf("%s: %v", c.args, err)
		}
	}
	out, err = list(root, `{"section": "vault"}`)
	if err != nil || !strings.Contains(mustJSON(t, out), "references, never values") {
		t.Fatalf("the listing of a closed section: %v %v", out, err)
	}
	out, err = list(root, `{}`)
	if err != nil || !strings.Contains(mustJSON(t, out), `"section":"tenants"`) {
		t.Fatalf("the sections: %v %v", out, err)
	}
}

// The TLS doors are closed to an agent whichever way a document comes in:
// the import tool, or the import endpoint called through call_admin_api. A
// person importing the same document is not asked.
func TestAnAgentsDocumentLeavesTheTLSDoorsAlone(t *testing.T) {
	f := setupBare(t)
	root := agentCtx(store.User{ID: "root", Username: "root", Root: true}, f.rootC)
	doc := "version: 1\nsettings:\n  tls:\n    redirect: true\n"
	if _, err := f.api.toolImportConfiguration(root, mustRaw(t, map[string]any{"document": doc, "dryRun": true})); err == nil || !strings.Contains(err.Error(), "tls") {
		t.Fatalf("the import tool: %v", err)
	}
	call := apiTool(t, f.api, "call_admin_api")
	if _, err := call(root, string(mustRaw(t, map[string]any{"method": "POST", "path": "/api/config/import",
		"body": doc, "contentType": "application/yaml"}))); err == nil || !strings.Contains(err.Error(), "tls") {
		t.Fatalf("the import endpoint, called by the agent: %v", err)
	}
	if code, out := f.call(t, "POST", "/api/config/import", doc, f.rootC); code != http.StatusOK {
		t.Fatalf("a person's import of the same document: %d %s", code, out)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func mustRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	return json.RawMessage(mustJSON(t, v))
}
