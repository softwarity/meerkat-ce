// Package admin is Meerkat's control plane: the API served on the dedicated
// admin port (CONSOLE-11), consumed by the console. It is strictly separated
// from the data plane - nothing here is ever routable from the application
// port.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/cluster"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// API serves the admin endpoints. Every endpoint requires a session whose
// user is root.
type API struct {
	// Mailer sends outbound e-mail through the STORED config; nil answers "not
	// configured". Wired by main, faked in tests.
	Mailer func(ctx context.Context, msg mail.Message) error
	// MailerWith sends through an EXPLICIT config - the relay test tries what is
	// on screen, not what is stored. Defaults to mail.Send; faked in tests.
	MailerWith func(ctx context.Context, cfg mail.Config, msg mail.Message) error

	// DataAddr is the data plane's listen address (main's -addr), for the few
	// admin features that must name the sibling plane (e.g. OIDC callbacks).
	DataAddr string

	// TLS is the live HTTPS state (SSL-01/02). Wired by main; nil in the tests
	// that have no listener to open, where saving material still has to work.
	TLS *certs.Supervisor

	// Bus tells the other nodes what this one just reloaded (STORE-03). Wired
	// by main; nil wherever there is one node, which is every test and every
	// single-binary installation - see reload.go.
	Bus *cluster.Bus

	st     *store.Store
	sm     *session.Manager
	router *gateway.Router
}

// New builds the admin API. router receives a hot reload after every
// mutation - saving IS applying.
func New(st *store.Store, sm *session.Manager, router *gateway.Router) *API {
	return &API{st: st, sm: sm, router: router}
}

// Mux is the slice of *http.ServeMux the control plane registers onto.
//
// An interface rather than the concrete type for ONE reason: a *http.ServeMux
// cannot be asked what was registered on it, and a surface nobody can
// enumerate is a surface nobody can hold to a rule. A test passes a recorder
// here and checks that every endpoint has been classified - read or write
// (tokenscope.go), and covered by an agent tool or knowingly not
// (mcp_coverage_test.go). Adding an endpoint is then a decision, not an
// oversight waiting to be found.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Register mounts the API on mux. The routing plane is GATEWAY scope
// (RBAC-05): root or the infra-admin capability.
func (a *API) Register(mux Mux) {
	mux.Handle("GET /api/catalog", a.gw(a.catalog))
	mux.Handle("GET /api/routes", a.gw(a.listRoutes))
	mux.Handle("POST /api/routes/reorder", a.infraAdmin(a.reorderRoutes))
	mux.Handle("POST /api/routes/respond-preview", a.infraAdmin(a.previewRespond))
	mux.Handle("POST /api/routes/version-preview", a.infraAdmin(a.previewVersion))
	mux.Handle("GET /api/routes/{id}", a.gw(a.getRoute))
	mux.Handle("PUT /api/routes/{id}", a.infraAdmin(a.putRoute))
	mux.Handle("DELETE /api/routes/{id}", a.infraAdmin(a.deleteRoute))
	a.registerOpenAPI(mux)
	a.registerSigning(mux)
	a.registerVault(mux)
	a.registerMailRelay(mux)
	a.registerProbe(mux)
	a.registerAuthProviders(mux)
	a.registerIdentity(mux)
	a.registerThemes(mux)
	a.registerRBAC(mux)
	a.registerGroupRules(mux)
	a.registerAdminTokens(mux)
	a.registerMCP(mux)
	a.registerAPIDocs(mux)
	a.registerIssues(mux)
	a.registerProxyLimits(mux)
	a.registerRouteHealth(mux)
	a.registerServices(mux)
	a.registerEdition(mux)
	a.registerConfig(mux)
	a.registerConfigurations(mux)
	a.registerConfigPoints(mux)
	a.registerBackup(mux)
	a.registerCertificates(mux)
	a.auditRegisterViewer(mux)
}

// gw adapts a plain handler to the infra-admin guard (the routing-plane
// handlers predate the userHandler signature and never need the actor).
func (a *API) gw(next http.HandlerFunc) http.Handler {
	return a.infraAdmin(func(w http.ResponseWriter, r *http.Request, _ store.User) { next(w, r) })
}

func (a *API) catalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, routing.Catalog())
}

func (a *API) listRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := a.st.ListRoutes(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	if routes == nil {
		routes = []store.Route{}
	}
	writeJSON(w, http.StatusOK, routes)
}

func (a *API) getRoute(w http.ResponseWriter, r *http.Request) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	writeJSON(w, http.StatusOK, route)
}

// previewRespond renders a respond template against the witness caller, so the
// editor can show what an application would receive WHILE it is being written.
//
// It runs the engine's own code, not a re-implementation: a preview that agreed
// with a second parser and disagreed with the gateway would be worse than none.
// Nothing is persisted and nothing is read - the template only ever sees the
// fictional caller.
func (a *API) previewRespond(w http.ResponseWriter, r *http.Request, _ store.User) {
	var in struct {
		Body string `json:"body"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed body: expected {\"body\": \"...\"}")
		return
	}
	out, err := routing.PreviewRespond(in.Body)
	if err != nil {
		// 200 with the error INSIDE: this is a live preview of something being
		// typed, and half of what someone types is not valid yet. A 4xx here
		// would light up the console's error handling on every keystroke.
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": out, "caller": routing.PreviewCaller()})
}

// previewVersion runs a version predicate's arguments against a sample, so the
// editor can show what will be extracted and whether it falls in the range
// while someone is still typing the pattern.
//
// The ENGINE answers, not the browser: Go's regexps are RE2, which refuses the
// lookarounds JavaScript allows, so a preview computed in the console would
// report a match on a pattern the gateway turns down at save.
func (a *API) previewVersion(w http.ResponseWriter, r *http.Request, _ store.User) {
	var in struct {
		Args   map[string]any `json:"args"`
		Sample string         `json:"sample"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed body: expected {\"args\": {...}, \"sample\": \"...\"}")
		return
	}
	extracted, matches, err := routing.PreviewVersion(in.Args, in.Sample)
	if err != nil {
		// 200 with the error inside, like the respond preview: half of what
		// someone types is not valid yet, and a 4xx per keystroke would light
		// up the console's error handling for nothing.
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"extracted": extracted, "matches": matches})
}

// reorderRoutes persists a new route order (first-match-wins, so order matters)
// from a JSON array of route ids, then reloads the data-plane snapshot.
func (a *API) reorderRoutes(w http.ResponseWriter, r *http.Request, actor store.User) {
	var ids []string
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ids); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed order: expected a JSON array of route ids")
		return
	}
	if err := a.st.ReorderRoutes(r.Context(), ids); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("reordered, but reload failed: %w", err))
		return
	}
	a.auditEvent(r.Context(), actor, "route.reorder", "route", "", "", "", fmt.Sprintf("%d routes reordered", len(ids)))
	writeJSON(w, http.StatusOK, map[string]int{"reordered": len(ids)})
}

// putRoute upserts a route: the body is validated by COMPILING it - the
// exact same code path the engine uses - so an invalid route is refused with
// the engine's precise error and never persisted.
func (a *API) putRoute(w http.ResponseWriter, r *http.Request, actor store.User) {
	var route store.Route
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&route); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed route: "+err.Error())
		return
	}
	route.ID = r.PathValue("id")
	saved, err := a.saveRoute(r.Context(), actor, route)
	if err != nil {
		if isInvalid(err) {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

// saveRoute creates or updates one route and applies it: validate, store,
// reload, record. Shared by the endpoint and by the agent's save_route tool,
// because a second implementation would be a second set of rules about what a
// route may be - and the one an agent uses would be the one nobody reviews.
func (a *API) saveRoute(ctx context.Context, actor store.User, route store.Route) (store.Route, error) {
	if strings.TrimSpace(route.ID) == "" {
		return route, invalid("a route needs an id: it is what names it in an export and in the audit")
	}
	if strings.TrimSpace(route.Name) == "" {
		return route, invalid("route name is required")
	}
	if len(route.Predicates) == 0 {
		return route, invalid("a route needs at least one predicate")
	}
	if route.API != nil && route.API.Spec != nil {
		if route.API.Spec.Empty() {
			route.API.Spec = nil
		} else if err := store.SanitizeRouteSpec(route.API.Spec); err != nil {
			return route, invalidErr(err)
		}
	}
	if err := a.validateRoute(ctx, route); err != nil {
		return route, invalidErr(err)
	}
	// The prior route (if any) so the audit tells a create from an update diff.
	old, hadRoute := store.Route{}, false
	if prev, err := a.st.GetRoute(ctx, route.ID); err == nil {
		old, hadRoute = prev, true
	}
	// A route that stops declaring a deposited spec drops the file with it:
	// bytes nothing names any more are bytes nobody will ever delete on
	// purpose, and they would come back to life the day the declaration does.
	if hadRoute && old.Spec().Type == store.SpecFile && route.Spec().Type != store.SpecFile {
		if err := a.st.DeleteRouteSpec(ctx, route.ID); err != nil {
			return route, err
		}
	}
	if err := a.st.SaveRoute(ctx, route); err != nil {
		return route, err
	}
	if err := a.reloadRouting(ctx); err != nil {
		// This route is valid, so a reload failure means another stored route
		// is broken - surface it instead of pretending everything applied.
		return route, fmt.Errorf("saved, but reload failed: %w", err)
	}
	if hadRoute {
		a.auditUpdate(ctx, actor, "route.update", "route", route.ID, route.Name, "", old, route)
	} else {
		a.auditEvent(ctx, actor, "route.create", "route", route.ID, route.Name, "", "")
	}
	return route, nil
}

// invalidError marks what the CALLER got wrong, as opposed to what broke. The
// two answer differently over HTTP (422 against 500) and read differently to
// an agent ("fix your arguments" against "something is wrong here").
type invalidError struct{ err error }

func (e invalidError) Error() string { return e.err.Error() }
func (e invalidError) Unwrap() error { return e.err }

func invalid(format string, args ...any) error { return invalidError{fmt.Errorf(format, args...)} }
func invalidErr(err error) error               { return invalidError{err} }

func isInvalid(err error) bool {
	var t invalidError
	// store.ErrInvalid counts too: the sanitisers live in the store so that
	// every writer of a route passes through them, and a refusal that crossed
	// that boundary used to reach the operator as "internal error" while the
	// sentence naming what is allowed stayed in the log.
	return errors.As(err, &t) || errors.Is(err, store.ErrInvalid)
}

// validateRoute compiles the route the ENGINE will run: the stored route with
// its $name references expanded (VAULT-01). Validating the raw form would
// reject a perfectly good "upstream: $api-host"; validating the expansion also
// catches a typo in a reference at save time rather than at reload time.
func (a *API) validateRoute(ctx context.Context, route store.Route) error {
	values, err := a.st.VaultValues(ctx, vault.ScopeInfra)
	if err != nil {
		return fmt.Errorf("vault unavailable: %w", err)
	}
	expanded, missing, err := gateway.ExpandRoute(route, values)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("unknown vault entries: %s", strings.Join(missing, ", "))
	}
	return gateway.Validate(expanded)
}

func (a *API) deleteRoute(w http.ResponseWriter, r *http.Request, actor store.User) {
	switch err := a.dropRoute(r.Context(), actor, r.PathValue("id")); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case isInvalid(err):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		a.internal(w, err)
	}
}

// dropRoute removes one route and applies it. Shared with the agent's tool for
// the same reason saveRoute is.
func (a *API) dropRoute(ctx context.Context, actor store.User, id string) error {
	route, _ := a.st.GetRoute(ctx, id) // capture the name before deletion
	existed, err := a.st.DeleteRoute(ctx, id)
	if err != nil {
		return err
	}
	if !existed {
		return invalid("route not found")
	}
	if err := a.reloadRouting(ctx); err != nil {
		return fmt.Errorf("deleted, but reload failed: %w", err)
	}
	a.auditEvent(ctx, actor, "route.delete", "route", id, route.Name, "", "")
	return nil
}

func (a *API) internal(w http.ResponseWriter, err error) {
	slog.Error("admin api error", "err", err)
	writeErr(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		slog.Error("admin api encode", "err", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
