package admin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/openapi"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// specClient fetches OpenAPI specs from upstreams server-side. The per-request
// context carries the real deadline; the client Timeout is a backstop.
//
// It verifies the upstream the way the route's own traffic does: the system's
// roots AND the trusted authorities (SSL-09). A plain client knew only the
// system's, so a service signed by the cluster's CA answered its pages through
// the route and refused its spec with "certificate signed by unknown authority"
// (2026-10-16). Built per call, because the trusted pool changes at runtime.
func specClient() *http.Client { return upstreamClient(20 * time.Second) }

// upstreamClient is a short-lived client to an upstream, trusting what the
// router trusts. No keep-alive: these calls are rare, and a pool held by a
// client nobody keeps would only leak its connections.
//
// A connect gets five seconds at most: a service that is there answers one at
// once, and one that is not would otherwise hold the screen for the whole
// timeout.
func upstreamClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DialContext:       (&net.Dialer{Timeout: min(timeout, 5*time.Second)}).DialContext,
			TLSClientConfig:   &tls.Config{RootCAs: gateway.UpstreamRoots(), MinVersion: tls.VersionTLS12},
			DisableKeepAlives: true,
		},
	}
}

// registerOpenAPI mounts the endpoint-security surface (RBAC-07): read the
// route's OpenAPI operations, and pose per-endpoint access rules. Routing plane
// (GATEWAY scope): root or infra-admin.
func (a *API) registerOpenAPI(mux Mux) {
	mux.Handle("GET /api/routes/{id}/operations", a.gw(a.getRouteOperations))
	mux.Handle("PUT /api/routes/{id}/security", a.infraAdmin(a.putRouteSecurity))
	mux.Handle("PUT /api/routes/{id}/audit", a.infraAdmin(a.putRouteAudit))
	mux.Handle("PUT /api/routes/{id}/role-map", a.infraAdmin(a.putRouteRoleMap))
	mux.Handle("PUT /api/routes/{id}/spec", a.infraAdmin(a.putRouteSpec))
	mux.Handle("DELETE /api/routes/{id}/spec", a.infraAdmin(a.deleteRouteSpec))
}

// routeOperations is what the console consumes to draw the swagger-like editor:
// the API metadata, the flat operation list fetched live from the upstream, and
// the currently saved per-endpoint security to overlay onto it.
type routeOperations struct {
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
	Format  string `json:"format"`
	// Access is the route's base security (the "whole route" default); Endpoints
	// override it per operation.
	Access     store.Access            `json:"access"`
	Operations []openapi.Operation     `json:"operations"`
	Security   *store.EndpointSecurity `json:"security,omitempty"`
	// Audit is the operations whose calls are audited (AUD-04).
	Audit []store.EndpointAudit `json:"audit,omitempty"`
	// Prefix is the part of the route's own path that every operation below
	// carries (gateway.KeptPrefix). It is IN those paths - they are the
	// coordinate the guard compares - and it is the same on every line, so the
	// screen says it once instead of repeating it fifteen times in a column
	// where what differs is the rest.
	Prefix string `json:"prefix,omitempty"`
	// DeclaredRoles is every role the spec declares, in the application's
	// names; RoleMap what each one is tied to in the catalogue.
	DeclaredRoles []string          `json:"declaredRoles,omitempty"`
	RoleMap       map[string]string `json:"roleMap,omitempty"`
}

// routeSecurityPayload is the endpoint-security screen's PUT body: the route's
// base Access plus the per-operation overrides.
type routeSecurityPayload struct {
	// Access is READ-ONLY here. The whole-route rule takes part in CHOOSING
	// the route - a caller it turns away falls through to the next route
	// matching the same paths - so it belongs with the route, and is edited
	// there. This screen shows it as the default its overrides refine.
	Access    store.Access           `json:"access"`
	Endpoints []store.EndpointPolicy `json:"endpoints"`
	// DenyUnlisted: only the listed operations are reachable.
	DenyUnlisted bool `json:"denyUnlisted,omitempty"`
}

// getRouteOperations fetches the route's OpenAPI spec, parses it (Swagger 2.0 or
// OpenAPI 3.x) and returns the operation projection plus the saved security.
func (a *API) getRouteOperations(w http.ResponseWriter, r *http.Request) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	spec, err := a.readSpec(r.Context(), route)
	if err != nil {
		writeErr(w, specReadStatus(err), err.Error())
		return
	}
	// The coordinate the GUARD compares against, not the key in the document:
	// the route's own prefix minus what it strips (gateway.KeptPrefix). The
	// editor writes rules in what this list shows, so showing the spec's raw
	// paths is how a rule comes to name something no request ever carries.
	kept := gateway.KeptPrefix(route)
	ops := make([]openapi.Operation, 0, len(spec.Operations))
	for _, op := range spec.Operations {
		op.Path = kept + op.Path
		ops = append(ops, op)
	}
	writeJSON(w, http.StatusOK, routeOperations{
		Title: spec.Title, Version: spec.Version, Format: spec.Format,
		Access: route.Access, Operations: ops, Security: securityOf(route),
		Audit: auditOf(route), Prefix: kept,
		DeclaredRoles: spec.Roles, RoleMap: route.RoleMap(),
	})
}

// putRouteSecurity replaces the route's endpoint-security block, validating it
// through the same engine path the router uses, then reloading. An empty block
// (no endpoints, no deny-by-default) clears security entirely.
func (a *API) putRouteSecurity(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	var payload routeSecurityPayload
	if err := decodeStrict(r, &payload); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed security: "+err.Error())
		return
	}
	sec := store.EndpointSecurity{Endpoints: payload.Endpoints, DenyUnlisted: payload.DenyUnlisted}
	if err := sec.Validate(); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	before := routeSecurityPayload{Access: route.Access, Endpoints: endpointsOf(route)}
	if s := securityOf(route); s != nil {
		before.DenyUnlisted = s.DenyUnlisted
	}

	// The route's own Access is NOT written from here: this screen is about
	// operations. Sent along for context, ignored on the way in - a screen
	// that saves what it only meant to display is how a rule changes without
	// anyone deciding to change it.
	api := store.RouteAPI{}
	if route.API != nil {
		api = *route.API // keep openapiUrl and any future API options
	}
	if len(sec.Endpoints) == 0 && !sec.DenyUnlisted {
		api.Security = nil
	} else {
		api.Security = &sec
	}
	route.API = &api

	// Compile the whole route (predicates, filters, endpoint security) before
	// persisting, so a bad policy is refused with the engine's precise error.
	if err := a.validateRoute(r.Context(), route); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SaveRoute(r.Context(), route); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("saved, but reload failed: %w", err))
		return
	}
	after := routeSecurityPayload{Access: route.Access, Endpoints: sec.Endpoints, DenyUnlisted: sec.DenyUnlisted}
	a.auditUpdate(r.Context(), actor, "route.security", "route", route.ID, route.Name, "", before, after)
	after.Endpoints = orEmpty(after.Endpoints)
	writeJSON(w, http.StatusOK, after)
}

// routeRoleMapPayload ties the roles a route's application declares to the
// catalogue, creating the catalogue roles that do not exist yet.
type routeRoleMapPayload struct {
	RoleMap map[string]string `json:"roleMap"`
	Create  []store.Role      `json:"create,omitempty"`
}

// putRouteRoleMap saves a route's declared-role mapping. The roles to create
// are the import: named with the screen's prefix, tagged with its tag, and
// created first so every target exists when the map is checked. Creating
// them is the application's catalogue, so it takes that right on top of the
// route's; a role that already exists is left as it is.
func (a *API) putRouteRoleMap(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	var payload routeRoleMapPayload
	if err := decodeStrict(r, &payload); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed role map: "+err.Error())
		return
	}
	if len(payload.Create) > 0 && !actor.Root && !actor.AppAdmin {
		writeErr(w, http.StatusForbidden, "creating catalogue roles requires root or the app-admin capability: map onto existing roles, or ask an application administrator")
		return
	}
	before := route.RoleMap()
	api := store.RouteAPI{}
	if route.API != nil {
		api = *route.API
	}
	api.RoleMap = nil
	if len(payload.RoleMap) > 0 {
		api.RoleMap = payload.RoleMap
	}
	route.API = &api
	// The route first, against the engine: a name it refuses must not leave
	// roles created for a mapping that is never saved.
	if err := a.validateRoute(r.Context(), route); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// Every target exists, or is among the roles this call creates - checked
	// before creating any, so a refusal leaves the catalogue as it was.
	toCreate := map[string]bool{}
	for _, role := range payload.Create {
		toCreate[strings.TrimSpace(role.Name)] = true
	}
	for app, role := range api.RoleMap {
		if toCreate[role] {
			continue
		}
		if _, err := a.st.GetRole(r.Context(), role); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf("declared role %s maps to %s, which is not in the catalogue: create it, or map onto an existing role", app, role))
			return
		}
	}
	created := []string{}
	for _, role := range payload.Create {
		role.Name = strings.TrimSpace(role.Name)
		if _, err := a.st.GetRole(r.Context(), role.Name); err == nil {
			continue
		}
		if role.Description == "" {
			role.Description = "Declared by the application behind the route " + route.Name
		}
		role.System, role.Rev = false, 0
		if err := a.st.SaveRole(r.Context(), role); err != nil {
			if conflict(w, err) {
				return
			}
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		a.auditEvent(r.Context(), actor, "role.create", "role", role.Name, role.Name, "", "")
		created = append(created, role.Name)
	}
	if err := a.st.SaveRoute(r.Context(), route); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("saved, but reload failed: %w", err))
		return
	}
	a.auditUpdate(r.Context(), actor, "route.roles", "route", route.ID, route.Name, "", before, api.RoleMap)
	writeJSON(w, http.StatusOK, map[string]any{"roleMap": orEmptyMap(api.RoleMap), "created": created})
}

func orEmptyMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// routeAuditPayload is the endpoint-audit screen's PUT body.
type routeAuditPayload struct {
	Endpoints []store.EndpointAudit `json:"endpoints"`
}

// putRouteAudit replaces the route's audited operations (AUD-04), through the
// same engine validation as everything else on a route, then reloads. An
// empty list stops auditing the route.
func (a *API) putRouteAudit(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	var body routeAuditPayload
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed audit: "+err.Error())
		return
	}
	if err := store.ValidateAudit(body.Endpoints); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// Enterprise (AUD-04): the calls are kept in the trail and sent to the
	// collector, both Enterprise. Emptying the list stays allowed - a configuration
	// brought over from the other image must be able to drop its rules.
	if len(body.Endpoints) > 0 {
		if err := edition.Require("auditing a route's operations"); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
	}
	before := routeAuditPayload{Endpoints: auditOf(route)}
	api := store.RouteAPI{}
	if route.API != nil {
		api = *route.API
	}
	api.Audit = body.Endpoints
	if len(api.Audit) == 0 {
		api.Audit = nil
	}
	route.API = &api
	if err := a.validateRoute(r.Context(), route); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SaveRoute(r.Context(), route); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("saved, but reload failed: %w", err))
		return
	}
	after := routeAuditPayload{Endpoints: auditOf(route)}
	a.auditUpdate(r.Context(), actor, "route.audit", "route", route.ID, route.Name, "", before, after)
	after.Endpoints = orEmpty(after.Endpoints)
	writeJSON(w, http.StatusOK, after)
}

// auditOf returns the route's audited operations, or nil.
func auditOf(route store.Route) []store.EndpointAudit {
	if route.API == nil {
		return nil
	}
	return route.API.Audit
}

// securityOf returns the route's endpoint security, or nil.
func securityOf(route store.Route) *store.EndpointSecurity {
	if route.API == nil {
		return nil
	}
	return route.API.Security
}

// endpointsOf returns a route's per-operation overrides, or nil.
func endpointsOf(route store.Route) []store.EndpointPolicy {
	if route.API == nil || route.API.Security == nil {
		return nil
	}
	return route.API.Security.Endpoints
}

// errNoSpec is what a route with no spec declaration answers, and what tells a
// missing declaration (422: nothing to read) from an upstream that will not
// answer (502: the other end is the problem).
var errNoSpec = errors.New("this route declares no OpenAPI spec")

// errNoFile is a route declaring a deposited spec whose file is not there -
// the state an import leaves behind when it carried the configuration without
// the package that holds the media. It is a 422 for the same reason: the
// request is fine, the declaration is what is incomplete.
var errNoFile = errors.New("this route declares a deposited spec, but no file was deposited")

// errRouteDown is a spec not asked for: the service it lives on is known down.
var errRouteDown = errors.New("route down")

func specReadStatus(err error) int {
	if errors.Is(err, errNoSpec) || errors.Is(err, errNoFile) {
		return http.StatusUnprocessableEntity
	}
	if errors.Is(err, errRouteDown) {
		return http.StatusServiceUnavailable
	}
	return http.StatusBadGateway
}

// readSpec parses the route's spec, from wherever it comes: fetched from the
// service, or read from what was deposited. One projection either way - which
// source it was is the console's business, not this screen's.
func (a *API) readSpec(ctx context.Context, route store.Route) (*openapi.Spec, error) {
	spec, _, err := a.readSpecRaw(ctx, route)
	return spec, err
}

// readSpecRaw is readSpec with the document itself, for a reader that needs
// more than the projection - an operation's parameters and body (MCP-08).
func (a *API) readSpecRaw(ctx context.Context, route store.Route) (*openapi.Spec, []byte, error) {
	decl := route.Spec()
	if decl.Empty() {
		return nil, nil, errNoSpec
	}
	if decl.Type == store.SpecFile {
		raw, ok, err := a.st.RouteSpecContent(ctx, route.ID)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, errNoFile
		}
		spec, err := openapi.Parse(raw)
		return spec, raw, err
	}
	specURL, err := resolveSpecURL(route)
	if err != nil {
		return nil, nil, err
	}
	// Known down: said at once, not after the timeout (gateway.Unreachable).
	if a.router != nil && gateway.SpecOnUpstream(route) {
		if why, down := a.router.Unreachable(route.ID); down {
			return nil, nil, fmt.Errorf("%w: the service behind %s is down (%s)", errRouteDown, route.Name, why)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return openapi.Fetch(ctx, specClient(), specURL)
}

// specDeposit is what the console gets back after depositing a file: what the
// spec turned out to be, where it is now served, and whether the upstream
// already answers there - the shadowing is announced at deposit time rather
// than discovered later. Replacing an incomplete spec IS the reason to deposit
// one, so this is a fact to state, never a refusal.
type specDeposit struct {
	Title    string `json:"title,omitempty"`
	Version  string `json:"version,omitempty"`
	Format   string `json:"format"`
	Count    int    `json:"operations"`
	Path     string `json:"path"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Shadows  int    `json:"shadows,omitempty"`
}

// putRouteSpec deposits a spec file on a route: the bytes are stored, and the
// route's declaration is switched to that file in the same move - a deposit
// that left the route pointing upstream would be a file nobody reads.
//
// The body is the file itself (JSON or YAML, the two forms the specification
// admits). It is PARSED before anything is written: what is not a spec is
// refused here, with the parser's own words, rather than discovered months
// later in an empty swagger.
func (a *API) putRouteSpec(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxSpecUpload+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not read the file: "+err.Error())
		return
	}
	if len(body) > maxSpecUpload {
		writeErr(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("this file is over %d bytes: a specification that big does not belong in a configuration package", maxSpecUpload))
		return
	}
	parsed, err := openapi.Parse(body)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	decl := store.RouteSpec{
		Type:     store.SpecFile,
		Path:     r.URL.Query().Get("path"),
		Filename: r.URL.Query().Get("filename"),
	}
	if decl.Filename == "" {
		decl.Filename = "openapi.json"
	}
	if err := store.SanitizeRouteSpec(&decl); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SetRouteSpec(r.Context(), route.ID, body); err != nil {
		a.internal(w, err)
		return
	}
	before := route.Spec()
	api := store.RouteAPI{}
	if route.API != nil {
		api = *route.API // the endpoint policies stay where they are
	}
	api.Spec = &decl
	route.API = &api
	if err := a.st.SaveRoute(r.Context(), route); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("saved, but reload failed: %w", err))
		return
	}
	a.auditUpdate(r.Context(), actor, "route.spec", "route", route.ID, route.Name, "", before, decl)
	out := specDeposit{
		Title: parsed.Title, Version: parsed.Version, Format: parsed.Format,
		Count: len(parsed.Operations), Path: decl.Path, Filename: decl.Filename,
		URL: routeSpecURL(route), Shadows: a.shadowedStatus(r.Context(), route),
	}
	writeJSON(w, http.StatusOK, out)
}

// deleteRouteSpec drops the deposited file AND the declaration that names it:
// a route left declaring a file that is gone is the dangling reference this
// whole feature is built to avoid.
func (a *API) deleteRouteSpec(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	before := route.Spec()
	if err := a.st.DeleteRouteSpec(r.Context(), route.ID); err != nil {
		a.internal(w, err)
		return
	}
	if route.API != nil && route.API.Spec != nil && route.API.Spec.Type == store.SpecFile {
		route.API.Spec = nil
		if err := a.st.SaveRoute(r.Context(), route); err != nil {
			a.internal(w, err)
			return
		}
		if err := a.reloadRouting(r.Context()); err != nil {
			a.internal(w, fmt.Errorf("saved, but reload failed: %w", err))
			return
		}
	}
	a.auditUpdate(r.Context(), actor, "route.spec", "route", route.ID, route.Name, "", before, store.RouteSpec{})
	w.WriteHeader(http.StatusNoContent)
}

// maxSpecUpload is the ceiling on a deposited file. It comes from the EXPORT,
// not from the parser: this file travels inside a configuration package.
const maxSpecUpload = 4 << 20

// routeSpecURL is where a deposited spec answers: inside the route's own
// prefix, so the document sits at the base it documents.
func routeSpecURL(route store.Route) string {
	decl := route.Spec()
	if decl.Type != store.SpecFile {
		return ""
	}
	return routing.MatchPrefix(route.Predicates) + "/" + decl.Path
}

// shadowedStatus asks the upstream whether it already answers where the
// deposited file now does, and returns its status code (0 when it does not
// answer at all). Best effort by design: an upstream that is down is not a
// reason to refuse a deposit.
func (a *API) shadowedStatus(ctx context.Context, route store.Route) int {
	decl := route.Spec()
	if decl.Type != store.SpecFile || route.Upstream == "" {
		return 0
	}
	target := strings.TrimRight(route.Upstream, "/") + routeUpstreamPrefix(route) + "/" + decl.Path
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0
	}
	res, err := specClient().Do(req)
	if err != nil {
		return 0
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 400 {
		return 0
	}
	return res.StatusCode
}

// resolveSpecURL yields the absolute URL of a route's OpenAPI spec: an absolute
// path is used as is; a relative one is resolved THE WAY THE ROUTE'S OWN
// TRAFFIC IS.
//
// That last part is the whole point. A route matching /fpl-svc/** with no
// strip-prefix hands the service paths that still start with /fpl-svc, so its
// spec sits at <upstream>/fpl-svc/openapi.json - resolving against the bare
// upstream asked for <upstream>/openapi.json and got a 404, with the admin
// left wondering which of the two ends was wrong. With a strip-prefix, the
// prefix is gone from what the service sees, and so it is from here.
func resolveSpecURL(route store.Route) (string, error) {
	decl := route.Spec()
	if decl.Type != store.SpecUpstream || decl.Path == "" {
		return "", errNoSpec
	}
	s := decl.Path
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s, nil
	}
	base := strings.TrimRight(route.Upstream, "/")
	if base == "" {
		return "", errors.New("a relative spec url needs the route to have an upstream")
	}
	return base + routeUpstreamPrefix(route) + "/" + strings.TrimLeft(s, "/"), nil
}

// routeUpstreamPrefix is the path prefix an upstream actually receives from
// this route: the literal head of its path predicate, minus whatever
// strip-prefix removes. "" when the route matches everything or strips it all.
func routeUpstreamPrefix(route store.Route) string {
	literal := ""
	for _, p := range route.Predicates {
		if p.Type != "path" {
			continue
		}
		pattern, ok := firstPattern(p.Args["patterns"])
		if !ok {
			continue
		}
		// Only the segments that are literally there: {id} and ** match many
		// things and name none of them.
		for _, seg := range strings.Split(strings.Trim(pattern, "/"), "/") {
			if seg == "" || seg == "**" || strings.HasPrefix(seg, "{") {
				break
			}
			literal += "/" + seg
		}
		break
	}
	if literal == "" {
		return ""
	}
	strip := 0
	for _, f := range route.Filters {
		if f.Type != "strip-prefix" {
			continue
		}
		switch n := f.Args["parts"].(type) {
		case float64:
			strip = int(n)
		case int:
			strip = n
		default:
			strip = 1
		}
	}
	if strip <= 0 {
		return literal
	}
	out := routing.StripSegments(literal, strip)
	if out == "/" {
		return ""
	}
	return out
}

// firstPattern reads the first entry of a path predicate's patterns argument,
// which crosses JSON as []any and Go as []string depending on where it comes
// from.
func firstPattern(v any) (string, bool) {
	switch list := v.(type) {
	case []string:
		if len(list) > 0 {
			return list[0], true
		}
	case []any:
		for _, item := range list {
			if s, ok := item.(string); ok {
				return s, true
			}
		}
	case string:
		return list, true
	}
	return "", false
}
