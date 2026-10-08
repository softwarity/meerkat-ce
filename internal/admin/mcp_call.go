package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/openapi"
	"github.com/softwarity/meerkat/internal/store"
)

// Calling the services behind the routes (MCP-08).
//
// An administrator's agent tests what the gateway serves the way the console's
// Try it out does: it reads what a route's service offers, then calls it AS
// somebody - by default an identity holding every role of the catalogue, so a
// root's agent is not stopped by rights it would have to manage, and when
// asked, as a named user with chosen roles, to see a rule refuse. The call is
// handed to the application plane in process: the route's rule, the endpoint
// rules, the filters, the quotas and the identity forwarded to the service
// all apply, and the service receives the marker headers of a simulated call.
//
// Two tools and not one per operation: the catalogue an agent reads stays the
// same size whatever the services publish, and the operations are discovered
// when they are needed.

// callBodyLimit bounds what comes back to the conversation. A service's answer
// can be megabytes; the agent needs to read it, not to hold it.
const callBodyLimit = 64 << 10

// callTimeout bounds one call, like a scheduled call's default.
const callTimeout = 30 * time.Second

// agentUser is the name a call answers to when the agent names nobody.
const agentUser = "agent"

func (a *API) callTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_operations", Allow: administersRouting, Title: "List what a route's service offers", ReadOnly: true,
			Description: "The operations a route's OpenAPI spec declares - method, path, operationId, summary - " +
				"with the PUBLIC path to pass to call_route (the route's prefix added when the route strips it). " +
				"Give an operation (its operationId, or \"METHOD /path\") to get its full definition instead: " +
				"parameters, request body and responses, with the schemas they reference. " +
				"A route without a spec has no operations to list; call_route still calls any path on it.",
			Schema: object(map[string]any{
				"routeId":   str("The route id, as given by list_routes."),
				"operation": str("Optional: an operationId, or \"GET /orders/{id}\", for that operation's full definition."),
			}, "routeId"),
			Call: a.toolListOperations,
		},
		{
			Name: "call_route", Allow: administersRouting, Title: "Call a route's service",
			Description: "Send a real request through a route, as a client would, and get the answer back: status, " +
				"headers and body (cut at 64 KiB). It goes through the gateway - the route's access rule, the " +
				"per-endpoint security, filters, quotas, and the identity the route forwards to the service - " +
				"so this is how to test what a service answers and what a rule lets through. " +
				"By default the caller is \"agent\" holding EVERY role of the catalogue. Pass `as` and `roles` " +
				"to call as somebody shaped otherwise, and `tenant` for a route reserved to organisations. " +
				"A 403 says why in its body: the gateway names the role or the organisation missing, and a " +
				"service may name its own reason. The call is REAL: a POST, PUT or DELETE changes the " +
				"service's data. Every call is written to the audit trail.",
			Schema: object(map[string]any{
				"routeId": str("The route id, as given by list_routes."),
				"method":  str("The HTTP method. GET when omitted."),
				"path":    str("The public path, as list_operations gives it (/orders/42), query string allowed."),
				"query": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"},
					"description": "Optional query parameters, added to the path's."},
				"headers": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"},
					"description": "Optional request headers. The identity is not one of them: use as and roles."},
				"body": map[string]any{"description": "Optional request body: a JSON value is sent as " +
					"application/json, a string as it is (set Content-Type in headers)."},
				"as": str("Optional: the username the call is made as. \"agent\" when omitted."),
				"roles": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
					"description": "Optional: the roles the caller holds, expanded through the hierarchy. Every role of the catalogue when omitted; [] for none."},
				"tenant": str("Optional: the organisation the call is made in, by id or name."),
			}, "routeId", "path"),
			Call: a.toolCallRoute,
		},
	}
}

type listOperationsArgs struct {
	RouteID   string `json:"routeId"`
	Operation string `json:"operation"`
}

func (a *API) toolListOperations(ctx context.Context, args json.RawMessage) (any, error) {
	var p listOperationsArgs
	if err := decode(args, &p); err != nil {
		return nil, err
	}
	route, err := a.st.GetRoute(ctx, p.RouteID)
	if err != nil {
		return nil, fmt.Errorf("no route %q: list_routes gives the ids", p.RouteID)
	}
	spec, raw, err := a.readSpecRaw(ctx, route)
	if err != nil {
		return nil, fmt.Errorf("route %s: %w", route.Name, err)
	}
	if strings.TrimSpace(p.Operation) == "" {
		type op struct {
			Method      string `json:"method"`
			Path        string `json:"path"`
			CallPath    string `json:"callPath"`
			OperationID string `json:"operationId,omitempty"`
			Summary     string `json:"summary,omitempty"`
		}
		ops := make([]op, 0, len(spec.Operations))
		for _, o := range spec.Operations {
			ops = append(ops, op{Method: o.Method, Path: o.Path, CallPath: gateway.PublicPath(route, o.Path),
				OperationID: o.OperationID, Summary: o.Summary})
		}
		return map[string]any{"route": route.Name, "title": spec.Title, "version": spec.Version,
			"operations": ops}, nil
	}
	// One operation, found by id or by "METHOD /path".
	want := strings.TrimSpace(p.Operation)
	method, path := "", ""
	for _, o := range spec.Operations {
		if o.OperationID == want || strings.EqualFold(o.Method+" "+o.Path, want) {
			method, path = o.Method, o.Path
			break
		}
	}
	if method == "" {
		return nil, fmt.Errorf("route %s declares no operation %q: list_operations without operation lists them", route.Name, want)
	}
	def, err := operationDefinition(raw, spec.Base, method, path)
	if err != nil {
		return nil, err
	}
	def["callPath"] = gateway.PublicPath(route, path)
	return def, nil
}

// operationDefinition digs one operation out of the document, with the
// schemas it references - everything a caller needs to build the request, and
// nothing of the rest of the document.
func operationDefinition(raw []byte, base, method, path string) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		if n, nerr := openapi.Normalize(raw); nerr == nil {
			err = json.Unmarshal(n, &doc)
		}
		if err != nil {
			return nil, fmt.Errorf("the spec is not readable as JSON or YAML: %w", err)
		}
	}
	paths, _ := doc["paths"].(map[string]any)
	// The projection's paths carry the document's base; the document's own
	// keys do not.
	key := strings.TrimPrefix(path, base)
	if key == "" {
		key = "/"
	}
	item, _ := paths[key].(map[string]any)
	if item == nil {
		item, _ = paths[path].(map[string]any)
	}
	op, _ := item[strings.ToLower(method)].(map[string]any)
	if op == nil {
		return nil, fmt.Errorf("the spec lists %s %s but holds no definition for it", method, path)
	}
	out := map[string]any{"method": method, "path": path, "definition": op}
	// Parameters declared on the path apply to every method under it.
	if shared, ok := item["parameters"]; ok {
		out["pathParameters"] = shared
	}
	refs := map[string]any{}
	collectRefs(doc, op, refs, 0)
	if shared, ok := item["parameters"]; ok {
		collectRefs(doc, shared, refs, 0)
	}
	if len(refs) > 0 {
		out["referenced"] = refs
	}
	return out, nil
}

// collectRefs follows every $ref under v into the document, keeping each
// referenced object once, a few levels deep: a schema that references itself
// must not run forever, and an agent reading twenty levels of nesting reads
// nothing.
func collectRefs(doc map[string]any, v any, into map[string]any, depth int) {
	if depth > 6 {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		if ref, ok := t["$ref"].(string); ok {
			if _, seen := into[ref]; !seen {
				if target := resolveRef(doc, ref); target != nil {
					into[ref] = target
					collectRefs(doc, target, into, depth+1)
				}
			}
		}
		for _, child := range t {
			collectRefs(doc, child, into, depth)
		}
	case []any:
		for _, child := range t {
			collectRefs(doc, child, into, depth)
		}
	}
}

// resolveRef reads a local "#/a/b/c" pointer; anything else is left for the
// agent to read as it is.
func resolveRef(doc map[string]any, ref string) any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	var cur any = doc
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

type callRouteArgs struct {
	RouteID string            `json:"routeId"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	As      string            `json:"as"`
	Roles   *[]string         `json:"roles"`
	Tenant  string            `json:"tenant"`
}

func (a *API) toolCallRoute(ctx context.Context, args json.RawMessage) (any, error) {
	var p callRouteArgs
	if err := decode(args, &p); err != nil {
		return nil, err
	}
	if a.DataPlane == nil || a.router == nil {
		return nil, fmt.Errorf("the application plane is not reachable from this control plane")
	}
	route, err := a.st.GetRoute(ctx, p.RouteID)
	if err != nil {
		return nil, fmt.Errorf("no route %q: list_routes gives the ids", p.RouteID)
	}
	method := strings.ToUpper(strings.TrimSpace(p.Method))
	if method == "" {
		method = http.MethodGet
	}
	path := strings.TrimSpace(p.Path)
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("path %q: a public path starts with /, as list_operations gives it", path)
	}
	u, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("path %q: %w", path, err)
	}
	if len(p.Query) > 0 {
		q := u.Query()
		for k, v := range p.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	// Who the call is: every role of the catalogue unless the agent says.
	user := strings.TrimSpace(p.As)
	if user == "" {
		user = agentUser
	}
	var roles []string
	if p.Roles != nil {
		roles = *p.Roles
	} else {
		all, err := a.st.ListRoles(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range all {
			roles = append(roles, r.Name)
		}
	}
	tenantID := ""
	if t := strings.TrimSpace(p.Tenant); t != "" {
		tenantID, err = a.tenantRef(ctx, t)
		if err != nil {
			return nil, err
		}
	}

	var body *bytes.Reader
	contentType := ""
	if len(p.Body) > 0 && string(p.Body) != "null" {
		var asString string
		if json.Unmarshal(p.Body, &asString) == nil {
			body = bytes.NewReader([]byte(asString))
		} else {
			body = bytes.NewReader(p.Body)
			contentType = "application/json"
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	actor := mcpActor(ctx)
	callCtx = a.router.WithAgentCaller(callCtx, user, roles, tenantID, actor.Username, actorToken(ctx))
	host := gateway.RouteHost(route)
	if host == "" {
		host = "gateway.internal"
	}
	var req *http.Request
	if body != nil {
		req, err = http.NewRequestWithContext(callCtx, method, "http://"+host+u.RequestURI(), body)
	} else {
		req, err = http.NewRequestWithContext(callCtx, method, "http://"+host+u.RequestURI(), nil)
	}
	if err != nil {
		return nil, fmt.Errorf("this does not make a request: %w", err)
	}
	req.Host = host
	req.Header.Set("User-Agent", "meerkat-agent")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	a.DataPlane.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	raw := rec.Body.Bytes()

	out := map[string]any{"status": res.StatusCode, "as": user}
	if p.Roles == nil {
		out["roles"] = "every role of the catalogue"
	} else {
		out["roles"] = roles
	}
	headers := map[string]string{}
	keys := make([]string, 0, len(res.Header))
	for k := range res.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.EqualFold(k, "Set-Cookie") {
			continue // a session the agent has no use for
		}
		headers[k] = strings.Join(res.Header[k], ", ")
	}
	out["headers"] = headers
	switch {
	case len(raw) == 0:
	case !utf8.Valid(raw):
		out["body"] = fmt.Sprintf("<binary, %d bytes>", len(raw))
	case len(raw) > callBodyLimit:
		out["body"] = string(raw[:callBodyLimit])
		out["truncated"] = fmt.Sprintf("the answer is %d bytes; the first %d are shown", len(raw), callBodyLimit)
	default:
		out["body"] = string(raw)
	}

	a.audit(ctx, store.AuditEvent{ActorID: actor.ID, Action: "route.call", Target: "route",
		TargetID: route.ID, TargetName: route.Name, TenantID: tenantID,
		Detail: fmt.Sprintf("%s %s as %s: %d", method, u.RequestURI(), user, res.StatusCode)})
	return out, nil
}

// tenantRef reads an organisation given by id or by name.
func (a *API) tenantRef(ctx context.Context, ref string) (string, error) {
	if t, err := a.st.GetTenant(ctx, ref); err == nil {
		return t.ID, nil
	}
	tenants, err := a.st.ListTenants(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range tenants {
		if strings.EqualFold(t.Name, ref) {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("no organisation %q, by id or by name", ref)
}
