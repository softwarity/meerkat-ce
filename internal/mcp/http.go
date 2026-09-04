package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

// The Streamable HTTP transport: ONE endpoint that receives commands.
//
// A POST carries a JSON-RPC message and the answer comes back in the response,
// which is all a tool server needs. The optional half of the transport - a GET
// held open for server-initiated messages, and the Mcp-Session-Id that goes
// with it - is deliberately not implemented: this server is stateless, every
// call stands alone, and a session id would be a thing to expire, to store and
// to get wrong across two nodes.

// protocolVersions are the specification revisions this server speaks, newest
// first. A client that asks for one of them is answered with ITS version; a
// client that asks for anything else is answered with our newest and decides
// for itself whether to go on. That is the negotiation the spec describes, and
// it is what keeps a client from a future revision working.
var protocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// maxRequestBytes bounds one JSON-RPC message. Arguments are small; a
// megabyte is already a hundred times any real call.
const maxRequestBytes = 1 << 20

// JSON-RPC error codes (the standard ones; MCP adds no others for a tool
// server).
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // absent = a notification
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// AllowOrigin decides whether a browser origin may talk to this endpoint.
// Nil refuses every request that carries an Origin header at all, which is the
// right default: an agent is not a browser, so an Origin on this endpoint is
// a page trying to drive the gateway with the operator's credentials. The
// specification requires this check, and DNS rebinding is the reason.
type AllowOrigin func(origin string) bool

// Handle answers one request of the Streamable HTTP transport. The caller is
// resolved by the host (a control-plane token), never by anything in the body.
func (s *Server) Handle(w http.ResponseWriter, r *http.Request, c Caller, allow AllowOrigin) {
	if origin := r.Header.Get("Origin"); origin != "" {
		if allow == nil || !allow(origin) {
			http.Error(w, "this endpoint does not answer browser origins", http.StatusForbidden)
			return
		}
	}
	if r.Method != http.MethodPost {
		// The transport's optional GET (a stream of server-initiated
		// messages) and DELETE (end a session) belong to a stateful server.
		w.Header().Set("Allow", "POST")
		http.Error(w, "this endpoint answers POST: one JSON-RPC message per request", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{codeParse, "request too large or unreadable"}})
		return
	}
	// A batch was removed from the protocol in 2025-06-18, but an older client
	// may still send one, and answering an array with an object reads as a
	// server crash. Cheaper to keep than to explain.
	if trimmed := strings.TrimLeft(string(body), " \t\r\n"); strings.HasPrefix(trimmed, "[") {
		var batch []rpcRequest
		if err := json.Unmarshal(body, &batch); err != nil {
			writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{codeParse, "malformed JSON-RPC batch"}})
			return
		}
		var out []rpcResponse
		for _, req := range batch {
			if resp, ok := s.dispatch(r, req, c); ok {
				out = append(out, resp)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, out)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", Error: &rpcError{codeParse, "malformed JSON-RPC message"}})
		return
	}
	resp, ok := s.dispatch(r, req, c)
	if !ok {
		// A notification: acknowledged, nothing to answer.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeRPC(w, resp)
}

// dispatch runs one message. ok is false for a notification, which gets no
// response at all.
func (s *Server) dispatch(r *http.Request, req rpcRequest, c Caller) (rpcResponse, bool) {
	notification := len(req.ID) == 0
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	fail := func(code int, msg string) (rpcResponse, bool) {
		resp.Error = &rpcError{code, msg}
		return resp, !notification
	}
	if req.JSONRPC != "2.0" {
		return fail(codeInvalidRequest, `every message carries "jsonrpc":"2.0"`)
	}
	switch req.Method {
	case "initialize":
		resp.Result = s.initialize(req.Params)
	case "ping":
		resp.Result = struct{}{}
	case "tools/list":
		resp.Result = s.listTools(r.Context(), c)
	case "tools/call":
		result, code, err := s.callTool(r, req.Params, c)
		if err != nil {
			return fail(code, err.Error())
		}
		resp.Result = result
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			// initialized, cancelled, progress: nothing to do for a stateless
			// tool server, and a notification expects no answer anyway.
			return resp, false
		}
		return fail(codeMethodNotFound, "unknown method "+req.Method+": this server offers tools (tools/list, tools/call)")
	}
	return resp, !notification
}

func (s *Server) initialize(params json.RawMessage) any {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &p)
	version := protocolVersions[0]
	if slices.Contains(protocolVersions, p.ProtocolVersion) {
		version = p.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		// Only tools. Declaring resources or prompts we do not serve is how a
		// client ends up showing an empty drawer.
		"capabilities": map[string]any{"tools": map[string]any{}},
		"serverInfo":   map[string]any{"name": s.Name, "title": "Meerkat", "version": s.Version},
	}
}

func (s *Server) listTools(ctx context.Context, c Caller) any {
	offered := s.offered(ctx, c)
	list := make([]map[string]any, 0, len(offered))
	for _, t := range offered {
		list = append(list, map[string]any{
			"name":        t.Name,
			"title":       t.Title,
			"description": t.Description,
			"inputSchema": t.Schema,
			"annotations": map[string]any{
				"title":           t.Title,
				"readOnlyHint":    t.ReadOnly,
				"destructiveHint": !t.ReadOnly,
			},
		})
	}
	return map[string]any{"tools": list}
}

func (s *Server) callTool(r *http.Request, params json.RawMessage, c Caller) (any, int, error) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, codeInvalidParams, fmt.Errorf("malformed tools/call parameters")
	}
	tool, known := s.byName[p.Name]
	// A tool outside the perimeter is answered exactly like one that does not
	// exist, plus the reason: the caller was never offered it, and a different
	// answer would turn the list into a map of what a wider token could do.
	if !known || !s.mayCall(r.Context(), c, tool) {
		msg := "unknown tool " + p.Name
		switch {
		case known && c.ReadOnly && !tool.ReadOnly:
			msg += ": this token is read-only, and that tool changes the gateway"
		case known:
			msg += ": your account does not administer what that tool reads"
		}
		return nil, codeInvalidParams, fmt.Errorf("%s", msg)
	}
	out, err := tool.Call(r.Context(), p.Arguments)
	if err != nil {
		// A tool that refuses is not a broken protocol: the model must READ
		// the reason and try again with better arguments, which it cannot do
		// with a JSON-RPC error.
		return map[string]any{
			"content": []any{map[string]any{"type": "text", "text": err.Error()}},
			"isError": true,
		}, 0, nil
	}
	text, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, codeInternal, fmt.Errorf("tool %s answered something unserializable", p.Name)
	}
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": string(text)}},
	}, 0, nil
}

func writeRPC(w http.ResponseWriter, resp rpcResponse) { writeJSON(w, resp) }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The response is already committed; there is nowhere left to report.
		_ = err
	}
}
