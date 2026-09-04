package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mint returns the clear value of a fresh control-plane token with the given
// perimeter.
func (f fixture) mint(t *testing.T, name, scope string) string {
	t.Helper()
	return f.mintWith(t, `{"name":"`+name+`","scope":"`+scope+`"}`, scope)
}

func (f fixture) mintWith(t *testing.T, payload, scope string) string {
	t.Helper()
	code, body := f.call(t, "POST", "/api/admin-tokens", payload, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("minting a %s token: %d %s", scope, code, body)
	}
	var created struct{ Token, Scope string }
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatal(err)
	}
	if created.Scope != scope {
		t.Fatalf("minted with perimeter %q, asked for %q", created.Scope, scope)
	}
	return created.Token
}

// withToken calls the control plane with a Bearer token and a body.
func (f fixture) withToken(t *testing.T, method, path, body, token string) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.adminSrv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res.StatusCode, string(b)
}

// The perimeter (MCP-02). The point of testing it HERE rather than on the
// agent endpoint: the same token opens the whole REST API on the same port, so
// a perimeter that only bound the tool set would bind nothing at all.
func TestAReadOnlyTokenCannotChangeAnything(t *testing.T) {
	f := setupBare(t)
	ro := f.mint(t, "agent", "readonly")

	if code, body := f.withToken(t, "GET", "/api/routes", "", ro); code != http.StatusOK {
		t.Fatalf("reading routes: %d %s", code, body)
	}
	// A tester is a POST that stores nothing, and refusing it would forbid
	// exactly what a read-only agent is for.
	code, body := f.withToken(t, "POST", "/api/routes/probe",
		`{"request":{"method":"GET","path":"/nothing"}}`, ro)
	if code != http.StatusOK {
		t.Fatalf("the routing tester must answer a read-only token: %d %s", code, body)
	}

	for _, w := range []struct{ method, path, body string }{
		{"POST", "/api/users", `{"username":"eve","timezone":"UTC"}`},
		{"PUT", "/api/settings", `{}`},
		{"POST", "/api/admin-tokens", `{"name":"another"}`},
		{"DELETE", "/api/routes/whatever", ""},
	} {
		code, body := f.withToken(t, w.method, w.path, w.body, ro)
		if code != http.StatusForbidden {
			t.Errorf("%s %s answered %d, want 403: %s", w.method, w.path, code, body)
		}
		if !strings.Contains(body, "read-only") {
			t.Errorf("%s %s must say WHY: %s", w.method, w.path, body)
		}
	}

	// A full token is unchanged: the perimeter only ever takes away.
	full := f.mint(t, "deploy", "full")
	if code, body := f.withToken(t, "POST", "/api/users",
		`{"username":"eve","timezone":"UTC"}`, full); code != http.StatusCreated {
		t.Fatalf("a full token must still create: %d %s", code, body)
	}
}

// The audit names the TOKEN, not merely the account it was minted on (MCP-03).
// Without this the trail reads as if root had done it by hand, and the one
// thing anybody wants to know after an agent went wrong is which agent.
func TestTheAuditNamesTheActingToken(t *testing.T) {
	f := setupBare(t)
	token := f.mint(t, "claude-desktop", "full")

	if code, body := f.withToken(t, "POST", "/api/users",
		`{"username":"mallory","timezone":"UTC"}`, token); code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	_, trail := f.call(t, "GET", "/api/audit?target=user", "", f.rootC)
	if !strings.Contains(trail, `"actorToken":"claude-desktop"`) {
		t.Fatalf("the trail must name the token that acted:\n%s", trail)
	}

	// And a human working from the console names no token at all.
	if code, body := f.call(t, "POST", "/api/users",
		`{"username":"alice","timezone":"UTC"}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("create by hand: %d %s", code, body)
	}
	_, trail = f.call(t, "GET", "/api/audit?target=user", "", f.rootC)
	if strings.Count(trail, `"actorToken"`) != 1 {
		t.Errorf("only the agent's event carries a token:\n%s", trail)
	}
}

// The endpoint end to end, the way an agent meets it: handshake, catalogue,
// call.
func TestAnAgentCanDriveTheGateway(t *testing.T) {
	f := setupBare(t)
	token := f.mint(t, "agent", "readonly")

	// It ships OFF: a token alone does not open a door nobody decided to open
	// (MCP-01).
	if code, _ := f.withToken(t, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"ping"}`, token); code != http.StatusNotFound {
		t.Fatalf("the agent endpoint answered %d before anyone switched it on", code)
	}
	if code, body := f.call(t, "PUT", "/api/settings/agent", `{"enabled":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switching the agent endpoint on: %d %s", code, body)
	}

	rpc := func(body string) map[string]any {
		t.Helper()
		code, raw := f.withToken(t, "POST", "/mcp", body, token)
		if code != http.StatusOK {
			t.Fatalf("%s answered %d: %s", body, code, raw)
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("undecodable: %s", raw)
		}
		if e := out["error"]; e != nil {
			t.Fatalf("%s answered an error: %v", body, e)
		}
		return out["result"].(map[string]any)
	}

	hello := rpc(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	if hello["protocolVersion"] != "2025-06-18" {
		t.Errorf("handshake answered %v", hello["protocolVersion"])
	}
	if caps, _ := hello["capabilities"].(map[string]any); caps["tools"] == nil {
		t.Errorf("the server must announce its tools: %v", hello)
	}

	// A read-only token is offered the read tools and NOT the others: the
	// perimeter shapes the catalogue, so the agent plans with what it has.
	list := rpc(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools, _ := list["tools"].([]any)
	offered := map[string]bool{}
	for _, item := range tools {
		m := item.(map[string]any)
		offered[m["name"].(string)] = true
		if m["annotations"].(map[string]any)["readOnlyHint"] != true {
			t.Errorf("a read-only token was offered %v", m["name"])
		}
	}
	if !offered["list_routes"] || offered["write_configuration"] {
		t.Fatalf("the catalogue a read-only token sees is wrong: %v", offered)
	}

	called := rpc(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"describe_gateway","arguments":{}}}`)
	text := called["content"].([]any)[0].(map[string]any)["text"].(string)
	// The edition is the first thing an agent has to know, so that "this needs
	// the Enterprise edition" is a sentence it can say instead of a tool it
	// cannot find.
	if !strings.Contains(text, `"edition"`) || !strings.Contains(text, `"enterprise"`) {
		t.Errorf("describe_gateway must name the edition:\n%s", text)
	}
	if !strings.Contains(text, `"actingAs": "root"`) {
		t.Errorf("a tool runs as the token's owner:\n%s", text)
	}
	// And it can name its own perimeter, so the agent says "this token may not
	// change anything" instead of hunting for a tool that is not there.
	if !strings.Contains(text, `"mayChange": false`) || !strings.Contains(text, `"token": "agent"`) {
		t.Errorf("describe_gateway must name the perimeter:\n%s", text)
	}

	// An invented tool is refused by name rather than silently ignored.
	code, raw := f.withToken(t, "POST", "/mcp",
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"delete_everything"}}`, token)
	if code != http.StatusOK || !strings.Contains(raw, "unknown tool delete_everything") {
		t.Errorf("an unknown tool answered %d %s", code, raw)
	}

	// And the endpoint itself is behind the same door as the API.
	req, _ := http.NewRequest("POST", f.adminSrv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("an anonymous agent answered %d, want 401", res.StatusCode)
	}
}

// The tools are not a second door. An agent runs with its owner's
// capabilities, and a tool that read the store without asking who is calling
// would be the way around everything the API enforces (RBAC-05).
func TestAnAgentInheritsItsOwnersCapabilities(t *testing.T) {
	f := setupBare(t)
	if code, body := f.call(t, "PUT", "/api/settings/agent", `{"enabled":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switching on: %d %s", code, body)
	}

	list := func(cookie *http.Cookie) []string {
		t.Helper()
		code, raw := f.call(t, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, cookie)
		if code != http.StatusOK {
			t.Fatalf("tools/list: %d %s", code, raw)
		}
		var out struct {
			Result struct {
				Tools []struct{ Name string } `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range out.Result.Tools {
			names = append(names, tool.Name)
		}
		return names
	}

	if n := len(list(f.rootC)); n != len((&API{}).tools()) {
		t.Errorf("root sees %d tools, want all %d", n, len((&API{}).tools()))
	}
	// bob administers nothing: he is authenticated, so the endpoint answers -
	// with a catalogue holding only what needs no capability.
	plain := list(f.plainC)
	if len(plain) != 1 || plain[0] != "describe_gateway" {
		t.Errorf("a plain user is offered %v, want describe_gateway alone", plain)
	}
	// And calling a tool he was not offered is refused, saying why.
	code, raw := f.call(t, "POST", "/mcp",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_users"}}`, f.plainC)
	if code != http.StatusOK || !strings.Contains(raw, "does not administer") {
		t.Errorf("a plain user calling list_users got %d %s", code, raw)
	}
}

// An agent that changes something changes it FOR REAL, the way any other agent
// with a token does. The first cut sent every write through a draft a person
// then activated; that was a ceremony nobody asked for, and the safety it was
// meant to provide already exists - a restore point is recorded automatically
// after every change on this plane (CFG-06).
func TestAnAgentChangesTheGatewayForReal(t *testing.T) {
	f := setupBare(t)
	if code, body := f.call(t, "PUT", "/api/settings/agent", `{"enabled":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switching on: %d %s", code, body)
	}
	token := f.mint(t, "deploy-bot", "full")

	call := func(name, args string) map[string]any {
		t.Helper()
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
		code, raw := f.withToken(t, "POST", "/mcp", body, token)
		if code != http.StatusOK {
			t.Fatalf("%s: %d %s", name, code, raw)
		}
		var out struct {
			Result struct {
				Content []struct{ Text string } `json:"content"`
				IsError bool                    `json:"isError"`
			} `json:"result"`
			Error *struct{ Message string } `json:"error"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatal(err)
		}
		if out.Error != nil {
			t.Fatalf("%s answered a protocol error: %s", name, out.Error.Message)
		}
		return map[string]any{"text": out.Result.Content[0].Text, "isError": out.Result.IsError}
	}

	// The question people actually ask is "why does ALICE not reach this", and
	// a route's rule takes part in choosing it - so the tester must be able to
	// carry an identity, or it answers a different question convincingly.
	who := call("test_routing", `{"request":{"method":"GET","path":"/x"},"as":{"username":"alice","tenantId":"acme","roles":["ops"]}}`)
	if who["isError"] == true {
		t.Fatalf("the tester must accept an identity: %s", who["text"])
	}

	// The vocabulary first: the types are a closed list, and an agent that
	// guessed one would be refused at save time.
	bricks := call("list_route_bricks", `{"kind":"predicate"}`)
	if bricks["isError"] == true || !strings.Contains(bricks["text"].(string), `"path"`) {
		t.Fatalf("the brick catalogue must name the path predicate: %v", bricks)
	}

	// A named snapshot, because a careful administrator asked for one in
	// words. It is a tool, not a rite: nobody else pays for it.
	if snap := call("save_configuration", `{"name":"before the agent"}`); snap["isError"] == true {
		t.Fatalf("saving a snapshot: %s", snap["text"])
	}

	// Then the actual work, and it is live when the tool answers.
	route := `{"id":"billing","name":"billing","order":10,"enabled":true,"isUi":true,` +
		`"upstream":"http://billing.svc:8080",` +
		`"predicates":[{"type":"path","args":{"patterns":["/billing/**"]}}],` +
		`"access":{"level":"auth"}}`
	if saved := call("save_route", route); saved["isError"] == true {
		t.Fatalf("saving the route: %s", saved["text"])
	}
	code, body := f.call(t, "GET", "/api/routes/billing", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(body, "billing.svc") {
		t.Fatalf("the route is not live: %d %s", code, body)
	}

	// A restore point was taken, so the change is undoable without anyone
	// having thought about it beforehand.
	_, points := f.call(t, "GET", "/api/config/history", "", f.rootC)
	if !strings.Contains(points, "route.create billing") {
		t.Errorf("no restore point names the change:\n%s", points)
	}

	// A route the console would have refused is refused here too: one
	// implementation, one set of rules.
	bad := call("save_route", `{"id":"x","name":"x","predicates":[{"type":"invented"}]}`)
	if bad["isError"] != true || !strings.Contains(bad["text"].(string), "invented") {
		t.Errorf("an unknown predicate was accepted: %v", bad)
	}

	// And it can take it back.
	if gone := call("delete_route", `{"id":"billing"}`); gone["isError"] == true {
		t.Fatalf("deleting: %s", gone["text"])
	}
	if code, _ := f.call(t, "GET", "/api/routes/billing", "", f.rootC); code != http.StatusNotFound {
		t.Errorf("the route survived its deletion: %d", code)
	}

	// The trail names the agent for every act.
	_, trail := f.call(t, "GET", "/api/audit", "", f.rootC)
	if strings.Count(trail, `"actorToken":"deploy-bot"`) < 3 {
		t.Errorf("the agent's acts must be named in the trail:\n%s", trail)
	}
}

// The domain is the second axis of the perimeter (MCP-02), and it works by
// MASKING what its owner holds rather than by a second rights model: a gateway
// token minted by root runs the routing plane and nothing else. That is what
// makes handing one to a deployment agent a smaller decision than handing over
// root.
func TestATokenIsConfinedToItsDomain(t *testing.T) {
	f := setupBare(t)
	if code, body := f.call(t, "PUT", "/api/settings/agent", `{"enabled":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switching on: %d %s", code, body)
	}
	gw := f.mintWith(t, `{"name":"deploy","scope":"full","domain":"gateway"}`, "full")
	app := f.mintWith(t, `{"name":"hr","scope":"full","domain":"app"}`, "full")

	// The routing plane answers the gateway token and refuses the other.
	if code, _ := f.withToken(t, "GET", "/api/routes", "", gw); code != http.StatusOK {
		t.Errorf("a gateway token cannot read the routes: %d", code)
	}
	if code, _ := f.withToken(t, "GET", "/api/routes", "", app); code != http.StatusForbidden {
		t.Errorf("an app token reached the routing plane: %d", code)
	}
	// And the accounts, the other way round.
	if code, _ := f.withToken(t, "GET", "/api/users", "", app); code != http.StatusOK {
		t.Errorf("an app token cannot read the accounts: %d", code)
	}
	if code, _ := f.withToken(t, "GET", "/api/users", "", gw); code != http.StatusForbidden {
		t.Errorf("a gateway token reached the accounts: %d", code)
	}
	// Root-only screens are nobody's: a domain that left root standing would
	// confine nothing.
	if code, _ := f.withToken(t, "GET", "/api/admin-tokens", "", gw); code != http.StatusForbidden {
		t.Errorf("a confined token kept root: %d", code)
	}

	// The agent's catalogue narrows with it, so the agent plans with what it
	// actually has.
	tools := func(token string) map[string]bool {
		t.Helper()
		code, raw := f.withToken(t, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, token)
		if code != http.StatusOK {
			t.Fatalf("tools/list: %d %s", code, raw)
		}
		var out struct {
			Result struct {
				Tools []struct{ Name string } `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, tool := range out.Result.Tools {
			names[tool.Name] = true
		}
		return names
	}
	if got := tools(gw); !got["list_routes"] || got["list_users"] || got["export_configuration"] {
		t.Errorf("the gateway token is offered %v", got)
	}
	if got := tools(app); !got["list_users"] || got["list_routes"] {
		t.Errorf("the app token is offered %v", got)
	}
}

// Where a token may be used from, judged on the TCP peer - a forwarding header
// is written by whoever sends it.
func TestATokenCanBeTiedToAnAddress(t *testing.T) {
	f := setupBare(t)
	// The tests reach the server over loopback, so a range that excludes it
	// must refuse and one that includes it must pass.
	elsewhere := f.mintWith(t, `{"name":"offsite","scope":"readonly","from":"192.0.2.0/24"}`, "readonly")
	if code, _ := f.withToken(t, "GET", "/api/routes", "", elsewhere); code != http.StatusUnauthorized {
		t.Errorf("a token used outside its range answered %d, want 401", code)
	}
	here := f.mintWith(t, `{"name":"onsite","scope":"readonly","from":"127.0.0.0/8, ::1"}`, "readonly")
	if code, body := f.withToken(t, "GET", "/api/routes", "", here); code != http.StatusOK {
		t.Errorf("a token used inside its range answered %d %s", code, body)
	}
	// A range that does not parse is refused at minting, not silently kept.
	if code, body := f.call(t, "POST", "/api/admin-tokens",
		`{"name":"typo","scope":"readonly","from":"10.0.0"}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Errorf("a malformed range was accepted: %d %s", code, body)
	}
}
