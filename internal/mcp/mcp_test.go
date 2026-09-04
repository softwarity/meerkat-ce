package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer() *Server {
	return NewServer("test", "1.0", []Tool{
		{
			Name: "read_thing", Title: "Read", Description: "reads", ReadOnly: true,
			Schema: map[string]any{"type": "object"},
			Call: func(context.Context, json.RawMessage) (any, error) {
				return map[string]any{"answer": 42}, nil
			},
		},
		{
			Name: "break_thing", Title: "Break", Description: "writes", Schema: map[string]any{"type": "object"},
			Call: func(context.Context, json.RawMessage) (any, error) { return "done", nil },
		},
	})
}

// call posts one JSON-RPC message and returns the decoded response.
func call(t *testing.T, s *Server, c Caller, body string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	s.Handle(rec, req, c, nil)
	var out map[string]any
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("undecodable answer (%d): %s", rec.Code, rec.Body)
		}
	}
	out["httpStatus"] = rec.Code
	return out
}

// The handshake: a client that asks for a revision we speak is answered with
// ITS revision, and one from the future is answered with ours rather than with
// silence - that is what keeps a newer client working.
func TestInitializeNegotiatesTheVersion(t *testing.T) {
	s := testServer()
	for _, tc := range []struct{ asked, want string }{
		{"2025-06-18", "2025-06-18"},
		{"2024-11-05", "2024-11-05"},
		{"2099-01-01", protocolVersions[0]},
		{"", protocolVersions[0]},
	} {
		got := call(t, s, Caller{}, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+tc.asked+`"}}`)
		result, _ := got["result"].(map[string]any)
		if result["protocolVersion"] != tc.want {
			t.Errorf("asked %q, answered %v, want %q", tc.asked, result["protocolVersion"], tc.want)
		}
	}
}

// A read-only caller is not shown the mutating tools at all. Hiding rather
// than refusing: an agent that cannot see a tool does not plan around it, and
// three turns spent building a call that ends in a refusal is the frustration
// the perimeter is supposed to prevent.
func TestAReadOnlyCallerSeesOnlyReadTools(t *testing.T) {
	s := testServer()

	full := call(t, s, Caller{}, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if n := len(toolNames(full)); n != 2 {
		t.Fatalf("a full caller sees %d tools, want 2", n)
	}
	ro := call(t, s, Caller{ReadOnly: true}, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	names := toolNames(ro)
	if len(names) != 1 || names[0] != "read_thing" {
		t.Fatalf("a read-only caller sees %v, want only read_thing", names)
	}

	// And calling the hidden one is refused, naming the perimeter - the agent
	// has to be able to TELL its user why, not merely fail.
	got := call(t, s, Caller{ReadOnly: true}, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"break_thing"}}`)
	e, _ := got["error"].(map[string]any)
	if e == nil || !strings.Contains(e["message"].(string), "read-only") {
		t.Fatalf("calling a hidden tool answered %v, want a refusal naming the perimeter", got)
	}
}

// A tool that refuses answers as a RESULT, not as a protocol error: the model
// has to read the reason and try again, and a JSON-RPC error is a dead end.
func TestAToolRefusalComesBackReadable(t *testing.T) {
	s := NewServer("test", "1.0", []Tool{{
		Name: "picky", Title: "Picky", Description: "refuses", ReadOnly: true,
		Schema: map[string]any{"type": "object"},
		Call: func(context.Context, json.RawMessage) (any, error) {
			return nil, errString("no route \"x\": call list_routes for the ids")
		},
	}})
	got := call(t, s, Caller{}, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"picky"}}`)
	if got["error"] != nil {
		t.Fatalf("a refusing tool must not be a protocol error: %v", got["error"])
	}
	result, _ := got["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("the result must be marked as an error: %v", result)
	}
	if !strings.Contains(textOf(result), "list_routes") {
		t.Errorf("the reason must reach the model: %v", result)
	}
}

// Browser origins are refused outright. An agent is not a browser, so an
// Origin on this endpoint is a page trying to drive the gateway with the
// operator's credentials - the rebinding attack the specification names.
func TestABrowserOriginIsRefused(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Origin", "https://evil.example")
	testServer().Handle(rec, req, Caller{}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an Origin header answered %d, want 403", rec.Code)
	}
}

// A notification carries no id and gets no answer - answering one is how a
// client ends up waiting for a response to a message it never asked about.
func TestANotificationIsAcknowledgedAndNotAnswered(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	testServer().Handle(rec, req, Caller{}, nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("a notification answered %d %s, want 202 and no body", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a notification must get no body, got %s", rec.Body)
	}
}

// The GET half of the transport belongs to a stateful server; this one says so
// rather than answering something a client would try to parse.
func TestTheStreamHalfIsDeclinedProperly(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer().Handle(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil), Caller{}, nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET answered %d, want 405", rec.Code)
	}
	if rec.Header().Get("Allow") != "POST" {
		t.Errorf("a 405 must say what IS allowed, got %q", rec.Header().Get("Allow"))
	}
}

// A tool set that would confuse an agent is a programming error, and it is
// caught at startup rather than one bad answer at a time.
func TestABadToolSetIsRefusedAtBuild(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tools []Tool
	}{
		{"no description", []Tool{{Name: "t", Schema: map[string]any{}, Call: func(context.Context, json.RawMessage) (any, error) { return nil, nil }}}},
		{"duplicate name", []Tool{
			{Name: "t", Description: "d", Schema: map[string]any{}, Call: func(context.Context, json.RawMessage) (any, error) { return nil, nil }},
			{Name: "t", Description: "d", Schema: map[string]any{}, Call: func(context.Context, json.RawMessage) (any, error) { return nil, nil }},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s was accepted", tc.name)
				}
			}()
			NewServer("test", "1.0", tc.tools)
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func toolNames(resp map[string]any) []string {
	result, _ := resp["result"].(map[string]any)
	list, _ := result["tools"].([]any)
	var out []string
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m["name"].(string))
		}
	}
	return out
}

func textOf(result map[string]any) string {
	content, _ := result["content"].([]any)
	var b strings.Builder
	for _, item := range content {
		if m, ok := item.(map[string]any); ok {
			b.WriteString(m["text"].(string))
		}
	}
	return b.String()
}
