package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/softwarity/meerkat/internal/admin/apidocs"
	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
)

// The whole control plane, for an agent (MCP-09).
//
// What a person can do in the console, an agent can do. The tools before
// this one were written one by one, and the endpoint-by-endpoint audit of
// 2026-10-17 found what that leaves: about forty actions of sections called
// "covered" - creating an organisation, undoing a change, comparing two
// configurations, a theme, most settings - with no tool, and refusals that
// were not refusals, since import_configuration wrote what they guarded.
//
// So the agent now calls the control plane itself: the same mux, through the
// chain main wraps it in, with the credentials the agent came with. The
// guards are the endpoints' own (an infra admin's token cannot write what an
// infra admin cannot), a read-only token stays read-only twice over (the tool
// is not offered, and the endpoint refuses), and the trail names the token.
// Nothing is reimplemented, so nothing drifts.
//
// What stays closed is short, named below with its reason, and enforced here
// - not in a comment.

// agentClosedSections are the parts of the control plane no agent reaches.
var agentClosedSections = map[string]string{
	"certificates": "certificates and their private keys: a key travels into this gateway from a person, " +
		"and never out of it to a model's context",
	"acme": "an ACME account is opened by accepting an authority's terms - a legal act, taken by a person",
	"trusted-authorities": "which authorities this gateway trusts when it calls out is the other half of " +
		"TLS, decided by the person who knows which PKI signs what",
	"vault": "the vault answers references, never values: an agent works with $names, " +
		"and a secret is typed by a person",
	"admin-tokens": "minting a control-plane token from an agent holding one is how a perimeter " +
		"stops meaning anything",
	"backup": "a snapshot is a file to download, and a copy moves the database under the running " +
		"gateway; export_configuration is the readable half",
	"license": "a license is a contract between a company and Softwarity, pasted by a person of " +
		"that company; describe_gateway says which edition this is",
	"live": "the console's websocket: an agent does not hold a socket open, it asks again",
	"mcp":  "the agent endpoint itself",
}

// agentClosedEndpoints are the single endpoints closed inside open sections.
var agentClosedEndpoints = map[string]string{
	"PUT /api/settings/tls": "the names this gateway answers to and the redirect to HTTPS: a change can " +
		"cut the console off, the agent's own door with it",
	"PUT /api/settings/tls/doors/{plane}": "a plane's public names and its proxy: a change can cut the " +
		"console off, the agent's own door with it",
	"PUT /api/settings/tls/ports/{plane}": "a plane's ports: a change can close the console's, the " +
		"agent's own door with it",
	"PUT /api/settings/agent": "whether agents may connect at all is not a question an agent answers",
	"POST /api/apidocs/token": "a test token for the developers' API docs: a credential, minted for a " +
		"browser page",
}

// agentClosedSettings are the settings a document an agent applies may not
// change: the same doors as above, reached the other way.
var agentClosedSettings = map[string]string{
	store.SettingTLS: "the TLS names, doors and certificate authorities are changed by a person " +
		"(Domains & TLS): leave the document's tls as export_configuration gave it",
}

// agentClosed says why an endpoint is out of an agent's reach, "" when it is
// not.
func agentClosed(pattern string) string {
	if why, ok := agentClosedEndpoints[pattern]; ok {
		return why
	}
	return agentClosedSections[section(pattern)]
}

// section is the part of the control plane a pattern belongs to:
// "PUT /api/tenants/{id}/groups/{groupId}" -> "tenants".
func section(pattern string) string {
	_, path, ok := strings.Cut(pattern, " ")
	if !ok {
		path = pattern
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if parts[0] == "api" && len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

// agentMayApply refuses a document that changes a setting closed to agents.
// Asked before an import or a switch applies anything.
func (a *API) agentMayApply(plan *config.Plan) error {
	for _, c := range plan.Changes {
		if c.Kind != "setting" || c.Action == config.ActionSame {
			continue
		}
		if why, closed := agentClosedSettings[c.ID]; closed {
			return fmt.Errorf("this document changes the %s setting, which an agent does not: %s", c.ID, why)
		}
	}
	return nil
}

// agentCalling says the request is an agent's: a tool, or a call it made
// through call_admin_api, whose context carries the tool's actor.
func agentCalling(ctx context.Context) bool {
	_, ok := ctx.Value(mcpActorKey{}).(store.User)
	return ok
}

// agentMayApplyDoc is agentMayApply asked where a document is applied - the
// import, a switch, a restore - so no way of applying one goes around it. A
// person's request passes untouched.
func (a *API) agentMayApplyDoc(ctx context.Context, doc *config.Document, prune, isSwitch bool) error {
	if !agentCalling(ctx) {
		return nil
	}
	var plan *config.Plan
	var err error
	if isSwitch {
		plan, err = config.PreviewSwitch(ctx, a.st, doc)
	} else {
		plan, err = config.Preview(ctx, a.st, doc, prune)
	}
	if err != nil {
		return err
	}
	return a.agentMayApply(plan)
}

// recordingMux registers through to the real mux and keeps the patterns.
type recordingMux struct {
	next     Mux
	patterns []string
}

func (m *recordingMux) Handle(pattern string, h http.Handler) {
	m.patterns = append(m.patterns, pattern)
	m.next.Handle(pattern, h)
}

func (m *recordingMux) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	m.patterns = append(m.patterns, pattern)
	m.next.HandleFunc(pattern, h)
}

// mcpCredsKey carries the agent's credentials to the calls it makes.
type mcpCredsKey struct{}

// mcpCreds are the headers that say who the /mcp request is: a bearer token,
// or a console session's cookie.
func mcpCreds(r *http.Request) http.Header {
	h := http.Header{}
	for _, k := range []string{"Authorization", "Cookie"} {
		if v := r.Header.Values(k); len(v) > 0 {
			h[k] = v
		}
	}
	return h
}

// maxAgentAnswer is how much of an answer goes back into a model's context.
const maxAgentAnswer = 256 << 10

// callAPI sends one request through the control plane as the agent.
func (a *API) callAPI(ctx context.Context, method, target string, body []byte, contentType string) (any, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return nil, fmt.Errorf("method %q: GET, POST, PUT, PATCH or DELETE", method)
	}
	target = strings.TrimSpace(target)
	if !strings.HasPrefix(target, "/api/") {
		return nil, fmt.Errorf("path %q: a control-plane path starts with /api/, as list_admin_api gives it", target)
	}
	if a.selfMux == nil {
		return nil, fmt.Errorf("the control plane is not mounted in this process")
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("path %q: %w", target, err)
	}
	if _, pattern := a.selfMux.Handler(req); pattern == "" || !strings.Contains(pattern, "/api/") {
		return nil, fmt.Errorf("no endpoint answers %s %s: list_admin_api lists them", method, req.URL.Path)
	} else if why := agentClosed(pattern); why != "" {
		return nil, fmt.Errorf("%s is closed to agents: %s", pattern, why)
	}
	if creds, ok := ctx.Value(mcpCredsKey{}).(http.Header); ok {
		for k, v := range creds {
			req.Header[k] = v
		}
	}
	// Built here, not by a page: what the CSRF check asks a cookie session
	// to prove is true of this request.
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if len(body) > 0 {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	h := a.Self
	if h == nil {
		h = a.selfMux
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	raw := rec.Body.Bytes()
	answer := map[string]any{"status": res.StatusCode}
	ct := res.Header.Get("Content-Type")
	switch {
	case len(raw) == 0:
	case len(raw) > maxAgentAnswer:
		answer["note"] = fmt.Sprintf("%d bytes of %s, too long to read here: narrow the request", len(raw), ct)
	case strings.Contains(ct, "json"):
		var v any
		if json.Unmarshal(raw, &v) == nil {
			answer["body"] = v
		} else {
			answer["body"] = string(raw)
		}
	case utf8.Valid(raw):
		answer["body"] = string(raw)
	default:
		answer["note"] = fmt.Sprintf("%d bytes of %s, which is not text", len(raw), ct)
	}
	if res.StatusCode >= 400 {
		msg := strings.TrimSpace(string(raw))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			msg = e.Error
		}
		return nil, fmt.Errorf("%s %s answered %d: %s", method, req.URL.Path, res.StatusCode, msg)
	}
	return answer, nil
}

// adminSpecOps are the summaries the admin API's OpenAPI document gives, by
// "METHOD /path", read once.
var adminSpecOps = sync.OnceValue(func() map[string]string {
	var doc struct {
		Paths map[string]map[string]struct {
			Summary     string `json:"summary"`
			Description string `json:"description"`
		} `json:"paths"`
	}
	out := map[string]string{}
	if json.Unmarshal(apidocs.AdminSpec, &doc) != nil {
		return out
	}
	for path, ops := range doc.Paths {
		for m, op := range ops {
			out[strings.ToUpper(m)+" "+path] = op.Summary
		}
	}
	return out
})

// apiEndpoint is one line of list_admin_api.
type apiEndpoint struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary,omitempty"`
	Closed  string `json:"closed,omitempty"`
}

func (a *API) listAPI(only string) any {
	if only == "" {
		type line struct {
			Section   string `json:"section"`
			Endpoints int    `json:"endpoints"`
			Closed    string `json:"closed,omitempty"`
		}
		count := map[string]int{}
		for _, p := range a.surface {
			if strings.Contains(p, "/api/") {
				count[section(p)]++
			}
		}
		out := make([]line, 0, len(count))
		for s, n := range count {
			out = append(out, line{Section: s, Endpoints: n, Closed: agentClosedSections[s]})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Section < out[j].Section })
		return map[string]any{"sections": out,
			"note": "Pass a section to list its endpoints. A dedicated tool, when there is one, says more " +
				"than the endpoint: prefer get_route to GET /api/routes/{id}."}
	}
	var out []apiEndpoint
	for _, p := range a.surface {
		if !strings.Contains(p, "/api/") || section(p) != only {
			continue
		}
		method, path, _ := strings.Cut(p, " ")
		out = append(out, apiEndpoint{Method: method, Path: path, Summary: adminSpecOps()[p], Closed: agentClosed(p)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return map[string]any{"section": only, "endpoints": out}
}

func (a *API) apiTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_admin_api", Allow: a.administersSomething, Title: "List the control plane's endpoints", ReadOnly: true,
			Description: "Every endpoint of the control plane - the API the console itself calls - so that " +
				"anything a person does in the console, an agent can do with read_admin_api and " +
				"call_admin_api. Without section: the sections and how many endpoints each has. With one: " +
				"its endpoints, method and path, a summary when the API's documentation has one, and why " +
				"it is closed to agents when it is (certificates, the vault, minting admin tokens, the TLS " +
				"doors...). The shape of a write is the shape the matching read returns: read, change, " +
				"write back. A dedicated tool, when there is one, says more: prefer it.",
			Schema: object(map[string]any{
				"section": str("Optional: a section, e.g. tenants, themes, settings, config."),
			}),
			Call: func(_ context.Context, args json.RawMessage) (any, error) {
				var in struct {
					Section string `json:"section"`
				}
				if err := decode(args, &in); err != nil {
					return nil, err
				}
				return a.listAPI(strings.TrimSpace(in.Section)), nil
			},
		},
		{
			Name: "read_admin_api", Allow: a.administersSomething, Title: "Read from the control plane", ReadOnly: true,
			Description: "GET one control-plane endpoint, as the console would, with this agent's own rights: " +
				"what the person behind the token may read. Paths and their parameters come from " +
				"list_admin_api; a query goes in the path (?limit=10).",
			Schema: object(map[string]any{
				"path": str("The path, e.g. /api/tenants or /api/config/history."),
			}, "path"),
			Call: func(ctx context.Context, args json.RawMessage) (any, error) {
				var in struct {
					Path string `json:"path"`
				}
				if err := decode(args, &in); err != nil {
					return nil, err
				}
				return a.callAPI(ctx, http.MethodGet, in.Path, nil, "")
			},
		},
		{
			Name: "call_admin_api", Allow: a.administersSomething, Title: "Act on the control plane",
			Description: "Send one request to the control plane, as the console would, with this agent's own " +
				"rights: the endpoint's guard decides, a refusal comes back with its reason, and the audit " +
				"trail names this token. What list_admin_api marks closed is refused here too. Applies at " +
				"once, like the console: read first (read_admin_api), write the same shape back. A " +
				"restore point is recorded before a configuration-wide change; the history is under " +
				"/api/config/history.",
			Schema: object(map[string]any{
				"method": str("GET, POST, PUT, PATCH or DELETE."),
				"path":   str("The path, as list_admin_api gives it, its {parameters} filled in."),
				"body": map[string]any{
					"description": "The request body: a JSON value, or a string sent as it is " +
						"(with contentType) for the few endpoints that take a file."},
				"contentType": str("Optional: the body's type when it is not JSON, e.g. text/css."),
			}, "method", "path"),
			Call: func(ctx context.Context, args json.RawMessage) (any, error) {
				var in struct {
					Method      string          `json:"method"`
					Path        string          `json:"path"`
					Body        json.RawMessage `json:"body"`
					ContentType string          `json:"contentType"`
				}
				if err := decode(args, &in); err != nil {
					return nil, err
				}
				body := []byte(in.Body)
				if string(body) == "null" {
					body = nil
				}
				// A string with a content type is a file, sent as its text.
				if in.ContentType != "" && !strings.Contains(in.ContentType, "json") {
					var s string
					if json.Unmarshal(body, &s) == nil {
						body = []byte(s)
					}
				}
				return a.callAPI(ctx, in.Method, in.Path, body, in.ContentType)
			},
		},
	}
}
