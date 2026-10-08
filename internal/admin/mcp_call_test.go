package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// An agent reads what a route's service offers, then calls it through the
// gateway: as an identity holding every role by default, and as somebody
// without the role when asked - the refusal then naming the role (MCP-08).
func TestAnAgentCallsTheServiceBehindARoute(t *testing.T) {
	f := setupBare(t)
	f.api.DataPlane = f.api.router

	// The service: it says what it received, and refuses a delete itself,
	// with its own reason.
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			http.Error(w, "orders: missing permission orders:delete", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"method": r.Method, "path": r.URL.RequestURI()})
	}))
	defer svc.Close()

	if err := f.api.st.SaveRole(context.Background(), store.Role{Name: "BILLING"}); err != nil {
		t.Fatal(err)
	}
	route := `{"id":"orders","name":"orders","order":10,"enabled":true,"upstream":"` + svc.URL + `",
		"predicates":[{"type":"path","args":{"patterns":["/orders-api/**"]}}],
		"filters":[{"type":"strip-prefix","args":{"parts":1}}],
		"access":{"level":"auth","roles":["BILLING"]}}`
	if code, body := f.call(t, "PUT", "/api/routes/orders", route, f.rootC); code != http.StatusOK {
		t.Fatalf("saving the route: %d %s", code, body)
	}
	spec := `{"openapi":"3.0.3","info":{"title":"Orders","version":"1.4.0"},"paths":{
		"/orders/{id}":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
			"get":{"operationId":"getOrder","summary":"One order","responses":{"200":{"description":"ok",
				"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Order"}}}}}}}},
		"components":{"schemas":{"Order":{"type":"object","properties":{"id":{"type":"string"}}}}}}`
	if code, body := f.call(t, "PUT", "/api/routes/orders/spec", spec, f.rootC); code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("depositing the spec: %d %s", code, body)
	}
	if code, body := f.call(t, "PUT", "/api/settings/agent", `{"enabled":true}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switching the agent endpoint on: %d %s", code, body)
	}
	token := f.mint(t, "claude", store.ScopeFull)

	tool := func(name, args string) (string, bool) {
		t.Helper()
		code, raw := f.withToken(t, "POST", "/mcp",
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":`+args+`}}`, token)
		if code != http.StatusOK {
			t.Fatalf("%s answered %d: %s", name, code, raw)
		}
		var out struct {
			Result struct {
				Content []struct{ Text string } `json:"content"`
				IsError bool                    `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil || len(out.Result.Content) == 0 {
			t.Fatalf("%s: undecodable %s", name, raw)
		}
		return out.Result.Content[0].Text, out.Result.IsError
	}

	// What the service offers, with the path a client calls.
	list, _ := tool("list_operations", `{"routeId":"orders"}`)
	if !strings.Contains(list, `"operationId": "getOrder"`) || !strings.Contains(list, `"callPath": "/orders-api/orders/{id}"`) {
		t.Fatalf("list_operations:\n%s", list)
	}
	one, _ := tool("list_operations", `{"routeId":"orders","operation":"getOrder"}`)
	if !strings.Contains(one, `"#/components/schemas/Order"`) || !strings.Contains(one, `"pathParameters"`) {
		t.Errorf("one operation must carry its parameters and the schemas it references:\n%s", one)
	}

	// By default, every role: the rule lets the call through and the service
	// sees the path without the route's prefix.
	ok, isErr := tool("call_route", `{"routeId":"orders","path":"/orders-api/orders/42"}`)
	if isErr || !strings.Contains(ok, `"status": 200`) || !strings.Contains(ok, `\"path\":\"/orders/42\"`) {
		t.Fatalf("a call with every role:\n%s", ok)
	}
	// Without the role, the gateway refuses and says which role is missing.
	refused, _ := tool("call_route", `{"routeId":"orders","path":"/orders-api/orders/42","as":"bob","roles":[]}`)
	if !strings.Contains(refused, `"status": 403`) || !strings.Contains(refused, "BILLING") {
		t.Errorf("a call without the role:\n%s", refused)
	}
	// A refusal of the service's own comes back with its reason.
	denied, _ := tool("call_route", `{"routeId":"orders","method":"DELETE","path":"/orders-api/orders/42"}`)
	if !strings.Contains(denied, `"status": 403`) || !strings.Contains(denied, "orders:delete") {
		t.Errorf("the service's own refusal:\n%s", denied)
	}

	// Every call is in the trail, with the token that made it.
	events, err := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, e := range events {
		if e.Action == "route.call" {
			calls++
			if e.ActorToken != "claude" {
				t.Errorf("a call audited without its token: %+v", e)
			}
		}
	}
	if calls != 3 {
		t.Errorf("%d calls audited, want 3", calls)
	}

	// A read-only token is not offered the call.
	ro := f.mint(t, "reader", store.ScopeReadOnly)
	_, raw := f.withToken(t, "POST", "/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, ro)
	if strings.Contains(raw, `"call_route"`) || !strings.Contains(raw, `"list_operations"`) {
		t.Errorf("a read-only token's catalogue: %s", raw)
	}
}
