package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

// The agent endpoint (MCP-01): ONE path on the control plane, behind the same
// guard as the API it sits next to.
//
// No port of its own. A port here is a listener, a row in the certificates
// table, a firewall rule, an ingress and an environment variable - and the
// administrator whose agent is asking already reaches this one, since it is
// where their console lives.
//
// The tools speak the PRODUCT, not the REST API. One tool per endpoint would
// be sixty tools, and an agent that recomposes HTTP calls instead of doing the
// work; a dozen that answer questions an administrator actually asks is the
// whole difference.
// originOf is this plane as the caller reached it.
func originOf(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (a *API) registerMCP(mux Mux) {
	mux.Handle("GET /api/settings/agent", a.rootOnly(a.getAgentSetting))
	mux.Handle("PUT /api/settings/agent", a.rootOnly(a.putAgentSetting))
	// The whole path, not "POST /mcp": the console's catch-all sits under "/",
	// so a GET here would be answered by the single-page app - a client
	// probing for the transport's optional stream would get HTML and a 200,
	// which is worse than a refusal. The transport answers for its own path
	// and says what it allows.
	mux.Handle("/mcp", a.agentDoor(a.authed(a.serveMCP)))
}

// agentSetting is the switch, and it ships OFF (MCP-01). A control-plane token
// is required either way, so this is not the lock - it is the decision that
// the door exists at all, taken by a person rather than inherited from an
// upgrade. Same rule as the issue tracker, same place: the screen where you
// would look for it, which is where the tokens are minted.
type agentSetting struct {
	Enabled bool `json:"enabled"`
}

func (a *API) agentEnabled(ctx context.Context) bool {
	var enabled bool
	_ = a.st.GetSetting(ctx, store.SettingAgentEnabled, &enabled)
	return enabled
}

func (a *API) getAgentSetting(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, agentSetting{Enabled: a.agentEnabled(r.Context())})
}

func (a *API) putAgentSetting(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body agentSetting
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	before := agentSetting{Enabled: a.agentEnabled(r.Context())}
	if err := a.st.SetSetting(r.Context(), store.SettingAgentEnabled, body.Enabled); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "agent.expose", "settings", "", "", "", before, body)
	writeJSON(w, http.StatusOK, body)
}

// mcpServer builds the tool set. Rebuilt per call rather than cached: it costs
// nothing next to the work the tools do, and a server value that outlives a
// request would have to be kept in step with a router reload.
func (a *API) mcpServer() *mcp.Server {
	return mcp.NewServer("meerkat", version.Version, a.tools())
}

// agentDoor closes the whole endpoint when the switch is off (MCP-01), BEFORE
// authentication: a door that does not exist answers the same to everyone, and
// a client would otherwise be told to go and authorise for nothing.
func (a *API) agentDoor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.agentEnabled(r.Context()) {
			writeErr(w, http.StatusNotFound,
				"the agent endpoint is switched off: turn it on in the console, under Infra, MCP")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) serveMCP(w http.ResponseWriter, r *http.Request, actor store.User) {
	sess, err := a.sm.Resolve(r.Context(), r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caller := mcp.Caller{ReadOnly: sess.TokenScope == store.ScopeReadOnly}
	// The tools run as the caller: they read the actor from the context rather
	// than take it as an argument, so a tool added later cannot forget it.
	r = r.WithContext(context.WithValue(r.Context(), mcpActorKey{}, actor))
	a.mcpServer().Handle(w, r, caller, a.mcpAllowOrigin)
}

// mcpAllowOrigin refuses every browser origin. An agent is not a browser, so
// an Origin header on this endpoint is a page trying to drive the gateway with
// the operator's credentials - the rebinding attack the specification warns
// about. The console has the REST API and needs nothing here.
func (a *API) mcpAllowOrigin(string) bool { return false }

type mcpActorKey struct{}

// mcpActor returns the user the current tool call runs as.
func mcpActor(ctx context.Context) store.User {
	u, _ := ctx.Value(mcpActorKey{}).(store.User)
	return u
}

// Who may run a tool. These mirror the guards the REST endpoints carry
// (a.infraAdmin, a.appAdmin, a.rootOnly) on purpose: an agent must not be a
// second door into what the console refuses, and the day a capability moves,
// the two have to move together.
func administersRouting(ctx context.Context) bool {
	u := mcpActor(ctx)
	return u.Root || u.InfraAdmin
}

func administersIdentity(ctx context.Context) bool {
	u := mcpActor(ctx)
	return u.Root || u.AppAdmin
}

func isRoot(ctx context.Context) bool { return mcpActor(ctx).Root }

// administersSomething is the audit trail's own rule: whoever administers a
// domain sees that domain's events, and the tool then answers with exactly the
// window the console would show them.
func (a *API) administersSomething(ctx context.Context) bool {
	_, ok := a.auditScope(ctx, mcpActor(ctx))
	return ok
}

// object is the argument schema of a tool, spelled out. Written literally
// rather than derived from a Go type: this text is READ BY A MODEL, and the
// wording of a field description is what decides whether the call arrives
// correct on the first try.
func object(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func noArgs() map[string]any { return object(map[string]any{}) }

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

// decode reads a tool's arguments, refusing what it does not know: a model
// that invents a field must be told, or it will keep inventing it.
func decode(args json.RawMessage, into any) error {
	if len(args) == 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(args)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("bad arguments: %w", err)
	}
	return nil
}

// tools is the whole catalogue.
func (a *API) tools() []mcp.Tool {
	read := []mcp.Tool{
		{
			Name: "describe_gateway", Title: "Describe the gateway", ReadOnly: true,
			Description: "What this Meerkat installation is: edition (community or Enterprise), version, " +
				"organisation mode, and how many routes, users and organisations it holds. " +
				"Call this FIRST: it says which edition you are talking to, and what your own perimeter " +
				"is. Both matter for the same reason - an Enterprise capability is absent from the " +
				"community image, and a tool outside your perimeter is not in your list - so knowing them " +
				"is how you say 'that needs the Enterprise edition' or 'this token may not change " +
				"anything' instead of failing at it.",
			Schema: noArgs(),
			Call:   a.toolDescribe,
		},
		{
			Name: "list_routes", Allow: administersRouting, Title: "List the routes", ReadOnly: true,
			Description: "Every route in matching order, one line each: name, whether it is enabled, " +
				"the paths it matches, its target and its access rule. This is the map of the gateway. " +
				"Use get_route for the full definition of one of them.",
			Schema: noArgs(),
			Call:   a.toolListRoutes,
		},
		{
			Name: "get_route", Allow: administersRouting, Title: "Get one route", ReadOnly: true,
			Description: "The complete definition of one route: predicates, filters, target, access rule, " +
				"identity forwarding, per-endpoint security and OpenAPI declaration.",
			Schema: object(map[string]any{
				"id": str("The route id, as given by list_routes."),
			}, "id"),
			Call: a.toolGetRoute,
		},
		{
			Name: "test_routing", Allow: administersRouting, Title: "Test which route a request reaches", ReadOnly: true,
			Description: "Answer 'where does this call land, and why'. Give a request and it names the " +
				"winning route and what each route did with it; give a routeId and it builds a request " +
				"that SHOULD reach that route. Nothing is proxied and nothing is written. " +
				"This is the tool for diagnosing a route that does not catch what its author expected.",
			Schema: object(map[string]any{
				"routeId": str("Optional: the route you expect to win. Seeds a request from its predicates."),
				"request": map[string]any{
					"type":        "object",
					"description": "Optional: the request to try, overriding the seeded one field by field.",
					"properties": map[string]any{
						"method":     str("GET, POST..."),
						"path":       str("The path as the client sends it, e.g. /api/orders/42."),
						"host":       str("The Host header."),
						"remoteAddr": str("The client address, e.g. 10.0.0.4:51000."),
						"headers":    map[string]any{"type": "object", "description": "Request headers by name."},
					},
				},
				// Without this the tool answers for an anonymous caller only,
				// which is the wrong answer to the question people actually
				// ask: "why does ALICE not reach this". A route's rule takes
				// part in choosing it, so identity changes the winner.
				"as": map[string]any{
					"type": "object",
					"description": "Optional: who the request pretends to come from. Omit for an anonymous caller. " +
						"A route's access rule takes part in CHOOSING it - a caller a rule turns away falls " +
						"through to the next route that matches - so this is what answers 'why does this " +
						"person land somewhere else'.",
					"properties": map[string]any{
						"username":    str("The account, for a rule that names people."),
						"tenantId":    str("The organisation ACTIVE on the session, which is what a rule reads. Not the ones they belong to."),
						"roles":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The effective roles IN that organisation."},
						"memberships": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The organisations they could switch to. These grant nothing; they tell a final refusal from one a switch would fix."},
						"signedIn":    map[string]any{"type": "boolean", "description": "There is a session, with no organisation chosen - what a fresh account looks like."},
					},
				},
			}),
			Call: a.toolTestRouting,
		},
		{
			Name: "list_users", Allow: administersIdentity, Title: "List the accounts", ReadOnly: true,
			Description: "The local accounts: username, full name, e-mail, whether enabled, and the " +
				"administrative capabilities each one holds.",
			Schema: noArgs(),
			Call:   a.toolListUsers,
		},
		{
			Name: "list_tenants", Allow: administersIdentity, Title: "List the organisations", ReadOnly: true,
			Description: "The organisations (tenants) and their owner. In single-organisation mode there " +
				"is one, and the others are held back rather than deleted.",
			Schema: noArgs(),
			Call:   a.toolListTenants,
		},
		{
			Name: "list_services", Allow: administersRouting, Title: "List what the runtime can route to", ReadOnly: true,
			Description: "The services this gateway's runtime knows about - Swarm services, Docker containers or " +
				"Kubernetes services in its own namespace - with the ports they listen on inside and a suggested " +
				"upstream url. Ask this BEFORE writing an upstream by hand: a name that does not resolve or a port " +
				"belonging to the neighbouring service only shows up when traffic does. An installation with no " +
				"runtime to ask answers why, and an upstream typed by hand still works.",
			Schema: noArgs(),
			Call:   a.toolListServices,
		},
		{
			Name: "read_audit", Allow: a.administersSomething, Title: "Read the audit trail", ReadOnly: true,
			Description: "Who changed what, most recent first, with the value before and after each field. " +
				"Use it to answer 'when did this change and who did it'.",
			Schema: object(map[string]any{
				"limit":  map[string]any{"type": "integer", "description": "How many events, 1 to 200 (default 50)."},
				"target": str("Optional: only this kind of object - route, user, role, settings, tenant, token, issue."),
			}),
			Call: a.toolReadAudit,
		},
		{
			Name: "export_configuration", Allow: isRoot, Title: "Export the whole configuration", ReadOnly: true,
			Description: "The entire infrastructure as one YAML document: routes, roles, organisations, " +
				"settings and the vault entries it REFERENCES (never their values). " +
				"The cheapest way to get the whole picture in one call before answering a broad question. " +
				"Pictures are left out, as they are in a plain export.",
			Schema: noArgs(),
			Call:   a.toolExportConfiguration,
		},
	}
	// The half that changes something, and its own file says how (mcp_write.go).
	return append(read, a.writeTools()...)
}

// routeLine is a route as an agent needs it in a list: enough to choose one,
// not enough to flood the conversation. A full dump of fifty routes is tens of
// thousands of tokens for a question that was "which one handles /api".
type routeLine struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Order   int      `json:"order"`
	Enabled bool     `json:"enabled"`
	Paths   []string `json:"paths,omitempty"`
	Target  string   `json:"target"`
	Access  string   `json:"access"`
	IsUI    bool     `json:"isUi"`
}

func (a *API) toolDescribe(ctx context.Context, _ json.RawMessage) (any, error) {
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	users, err := a.st.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	tenants, err := a.st.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"product":    "Meerkat, an app-gateway that also holds the identity of the applications behind it",
		"version":    version.Version,
		"edition":    edition.Name,
		"enterprise": edition.Enterprise,
		"tenancy":    a.st.Tenancy(ctx),
		"routes":     len(routes),
		"users":      len(users),
		"tenants":    len(tenants),
		"actingAs":   mcpActor(ctx).Username,
		"perimeter":  tokenPerimeter(ctx),
	}
	if !edition.Enterprise {
		// So the agent can SAY why, instead of stopping at a tool it cannot
		// find. The community image simply does not carry that code.
		out["enterpriseWouldAdd"] = []string{
			"more than one organisation",
			"arranging the built-in pages (layouts)",
			"the orchestrator directories",
			"removing the Meerkat mark",
		}
	}
	return out, nil
}

func (a *API) toolListRoutes(ctx context.Context, _ json.RawMessage) (any, error) {
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	lines := make([]routeLine, 0, len(routes))
	for _, r := range routes {
		lines = append(lines, routeLine{
			ID: r.ID, Name: r.Name, Order: r.Order, Enabled: r.Enabled,
			Paths: routing.MatchPaths(r.Predicates), Target: routeTarget(r),
			Access: accessSummary(r.Access), IsUI: r.IsUI,
		})
	}
	return map[string]any{"routes": lines}, nil
}

func (a *API) toolGetRoute(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("which route: pass the id given by list_routes")
	}
	route, err := a.st.GetRoute(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("no route %q: call list_routes for the ids", in.ID)
	}
	return route, nil
}

func (a *API) toolTestRouting(ctx context.Context, args json.RawMessage) (any, error) {
	var p probePayload
	if err := decode(args, &p); err != nil {
		return nil, err
	}
	if p.TargetRouteID == "" && p.Request == nil {
		return nil, fmt.Errorf("nothing to test: pass a request, a routeId, or both")
	}
	return a.probe(ctx, p)
}

func (a *API) toolListUsers(ctx context.Context, _ json.RawMessage) (any, error) {
	users, err := a.st.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	type line struct {
		ID           string   `json:"id"`
		Username     string   `json:"username"`
		Fullname     string   `json:"fullname,omitempty"`
		Email        string   `json:"email,omitempty"`
		Enabled      bool     `json:"enabled"`
		Capabilities []string `json:"capabilities,omitempty"`
	}
	out := make([]line, 0, len(users))
	for _, u := range users {
		var caps []string
		for _, c := range []struct {
			on   bool
			name string
		}{
			{u.Root, "root"}, {u.InfraAdmin, "gateway-admin"}, {u.AppAdmin, "app-admin"},
			{u.TenantCreator, "tenant-creator"}, {u.Dev, "developer"}, {u.Tester, "tester"},
		} {
			if c.on {
				caps = append(caps, c.name)
			}
		}
		out = append(out, line{
			ID: u.ID, Username: u.Username, Fullname: u.Fullname, Email: u.Email,
			Enabled: u.Enabled, Capabilities: caps,
		})
	}
	return map[string]any{"users": out}, nil
}

func (a *API) toolListTenants(ctx context.Context, _ json.RawMessage) (any, error) {
	tenants, err := a.st.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	type line struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Enabled     bool   `json:"enabled"`
		Owner       string `json:"owner,omitempty"`
	}
	out := make([]line, 0, len(tenants))
	for _, t := range tenants {
		owner := t.OwnerID
		if u, err := a.st.GetUserByID(ctx, t.OwnerID); err == nil {
			owner = u.Username
		}
		out = append(out, line{
			ID: t.ID, Name: t.Name, Description: t.Description, Enabled: t.Enabled, Owner: owner,
		})
	}
	return map[string]any{"tenancy": a.st.Tenancy(ctx), "tenants": out}, nil
}

func (a *API) toolReadAudit(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Limit  int    `json:"limit"`
		Target string `json:"target"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	if in.Limit <= 0 {
		in.Limit = 50
	}
	if in.Limit > 200 {
		in.Limit = 200
	}
	scope, ok := a.auditScope(ctx, mcpActor(ctx))
	if !ok {
		return nil, fmt.Errorf("your account administers nothing, so there is no trail to show you")
	}
	f := store.AuditFilter{Limit: in.Limit, Target: in.Target, Scope: scope}
	events, err := a.st.ListAuditEvents(ctx, f)
	if err != nil {
		return nil, err
	}
	return map[string]any{"events": events}, nil
}

func (a *API) toolExportConfiguration(ctx context.Context, _ json.RawMessage) (any, error) {
	doc, literals, err := config.Export(ctx, a.st)
	if err != nil {
		return nil, err
	}
	stripped, err := config.WithoutImages(doc)
	if err != nil {
		return nil, err
	}
	file, err := config.Marshal(stripped)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"yaml": string(file)}
	if len(literals) > 0 {
		out["secretsLeftBehind"] = len(literals)
	}
	return out, nil
}

// routeTarget says where a route sends what it catches, in one word: the
// terminal filter it carries, or its upstream (ROUTE-16 - one target, chosen
// and not accumulated).
func routeTarget(r store.Route) string {
	if t := routing.TerminalType(r.Filters); t != "" {
		return t
	}
	if r.Upstream != "" {
		return "proxy " + r.Upstream
	}
	return "none"
}

// accessSummary renders a route's access rule as a phrase, because a nested
// object in a listing is read by nobody, model or human.
func accessSummary(a store.Access) string {
	var parts []string
	switch a.Level {
	case store.AccessDelegated:
		parts = append(parts, "open (the upstream decides)")
	case store.AccessAuth:
		parts = append(parts, "signed in")
	case store.AccessTenant:
		parts = append(parts, "signed in with an organisation chosen")
	case store.AccessTenants:
		parts = append(parts, "organisation in "+strings.Join(a.Tenants, ", "))
	case store.AccessDeny:
		parts = append(parts, "nobody")
	default:
		parts = append(parts, a.Level)
	}
	if len(a.Roles) > 0 {
		parts = append(parts, "roles "+strings.Join(a.Roles, ", "))
	}
	if len(a.Users) > 0 {
		parts = append(parts, "always "+strings.Join(a.Users, ", "))
	}
	return strings.Join(parts, "; ")
}

// toolListServices answers what the runtime can route to (SVC-02).
func (a *API) toolListServices(ctx context.Context, _ json.RawMessage) (any, error) {
	return discovery.Discover(ctx), nil
}
