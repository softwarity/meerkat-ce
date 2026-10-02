// Package gateway is Meerkat's data path: route matching and reverse
// proxying. Routes come from the store as declarative predicate/filter specs
// (internal/routing), compiled into an immutable snapshot swapped atomically
// on reload - the hot path takes a read lock and nothing else.
package gateway

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	filtering "github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/openapi"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/signing"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/tracing"
	"github.com/softwarity/meerkat/internal/vault"
)

// Router matches incoming requests against the compiled routes, first match
// wins in route order.
type Router struct {
	st *store.Store
	sm *session.Manager

	// loaded is false until the first Reload has finished. See Ready.
	loaded atomic.Bool

	// metrics counts what passes through (OBS-01). Each compiled route holds a
	// pointer into it, so the request path never looks anything up.
	metrics *metrics.Registry
	// traceExport and traceSample are the tracing setting as the last reload
	// read it, baked into the fragment injected in the pages of the routes
	// that asked for it (OBS-04). The rate is the gateway's own: a page opens
	// the journey instead of this gateway, it does not open a different KIND
	// of journey.
	// Written under the reload, read while compiling in the same call.
	traceExport bool
	traceSample float64

	// lottery draws the per-request value consumed by weight predicates
	// (canary). Overridable in tests for determinism.
	lottery func() float64

	// AdminAddr is the control plane's listen address (main's -admin-addr):
	// the data plane answers CORS for exactly that sibling origin (the admin
	// console's swagger Try it out) and no other. Empty disables it.
	AdminAddr string
	// AdminSessions resolves admin-plane sessions, ONLY to authorize identity
	// simulation (simulate.go). Nil disables simulation entirely.
	AdminSessions *session.Manager
	// Pages renders the built-in pages this router has to serve itself - the
	// unavailable page today (LIFE-05). Wired by main to the flow chrome, so
	// the page wears the installation's theme, layout, mark and the visitor's
	// language like every other page it serves; nil falls back to a
	// self-contained block, which is what the tests run on.
	Pages func(w http.ResponseWriter, r *http.Request, reason string, until int64, continueURL string)
	// Stripe is the reminder injected into every page seen through the
	// maintenance door, translated like the page it lands on. Wired by main;
	// nil falls back to the English block below it.
	Stripe func(r *http.Request) string
	// simTokenKey signs the ephemeral test tokens (simulate.go). Read from the
	// store at Reload, so every gateway on one database signs with the same
	// key: per boot, a token minted on one node was gibberish on the next, and
	// a restart invalidated one an operator had just copied. The random value
	// New puts here is only what a router with no usable store falls back on.
	simTokenKey []byte

	mu     sync.RWMutex
	routes []compiledRoute
	// problems is why each route that is NOT in routes was left out, by route
	// id. Written by Reload under the same lock, read by Problems.
	problems map[string]string
	needDraw bool // at least one route uses weight predicates

	// uiSims holds the running UI tests (uisim.go): a developer session
	// browsing ONE route as a simulated identity. Per process, TTL-bounded.
	uiSimMu sync.RWMutex
	uiSims  map[uiSimKey]uiSimEntry
	// signing holds the gateway's identity signing keys (signed-jwt). Loaded
	// at Reload; nil until a route uses signed-jwt or the admin generates them.
	signing *signing.Set
	// maintenance is the global switch (LIFE-05) and the page it answers,
	// rendered once here rather than per request. Both under rt.mu, swapped by
	// Reload like everything else this router holds.
	maintenance     store.Maintenance
	maintenancePage []byte

	// defaultTimeouts is what the INSTALLATION asks of every route that does
	// not say otherwise (ROUTE-07), read at every reload. Only written there,
	// only read while compiling, both under the same call - so it needs no
	// lock of its own.
	defaultTimeouts store.RouteTimeouts

	// breakers holds one circuit per route (ROUTE-09), and with it the state
	// the console reads to say whether a route is answering (SVC-04). Per
	// node, deliberately - see internal/gateway/breaker.go.
	breakers *breakers

	// opsSlots holds, per route id, WHICH operations its requests are
	// attributed to (endpoints.go). Kept OUTSIDE the compiled routes on
	// purpose: a reload rebuilds those, and the control plane pushes into
	// these on a schedule of its own - a push must not be lost because
	// somebody saved a route in between.
	opsMu    sync.Mutex
	opsSlots map[string]*opsSlot

	// tagsMu guards the catalogue's tag -> role names table, read by routes
	// that narrow what they forward. Cached: the catalogue changes far less
	// often than requests arrive, and a few seconds late is invisible.
	tagsMu     sync.Mutex
	tagsCache  map[string][]string
	tagsReadAt time.Time

	// identities remembers who a session is (identitycache.go), and identityEpoch
	// is what makes forgetting it all a single increment.
	identityMu    sync.Mutex
	identities    map[identityKey]identityEntry
	identityEpoch atomic.Uint64
}

type compiledRoute struct {
	id      string
	name    string
	preds   routing.CompiledPredicates
	handler http.Handler
	// access takes part in SELECTION, not only in permission: two routes may
	// match the same paths and differ only by who they are for - one /** for
	// each organisation, or one per role. Deciding that inside the handler
	// meant the first one always won and the second was unreachable.
	access store.Access
	isUI   bool
	// noTrace is the route's own tracing switch, off: this route is not one
	// the installation asked to follow, so the gateway opens no span for it
	// and injects no bundle into its pages. Read at COMPILE time like
	// everything else a request must not look up.
	noTrace bool
	// counters are this route's own, resolved at COMPILE time so answering a
	// request costs an atomic add and no lookup.
	counters *metrics.Route
	// ops names WHICH operation a request was, when the route carries enough
	// to say so without guessing. Shared with the router rather than owned
	// here, so what the control plane resolves survives a reload. See
	// endpoints.go.
	ops *opsSlot
	// breaker is the route's circuit configuration, kept on the compiled route
	// so the health view can report against the same numbers the guard uses.
	breaker store.CircuitBreaker
}

// New builds a Router over the store. sm may be nil when no route requires
// authentication (tests). Call Reload to load the routes.
//
// The simulation key it starts with is random and private to this process.
// Reload replaces it with the one in the database, which is what makes a test
// token minted on one gateway readable by the next; this is only what stands
// in until then, and what a router whose store cannot answer keeps.
func New(st *store.Store, sm *session.Manager) *Router {
	key := make([]byte, 32)
	if _, err := cryptorand.Read(key); err != nil {
		panic(err) // the OS entropy source is gone; nothing sensible remains
	}
	return &Router{st: st, sm: sm, lottery: rand.Float64, simTokenKey: key,
		breakers: newBreakers(), metrics: metrics.NewRegistry()}
}

// Reload compiles the enabled routes from the store and swaps them in
// atomically. Safe to call while serving. A route that fails to compile
// aborts the reload with a precise error - the previous snapshot keeps
// serving.
func (rt *Router) Reload(ctx context.Context) error {
	stored, err := rt.st.ListRoutes(ctx)
	if err != nil {
		return err
	}
	// The application locale pool feeds every route (each may exclude some).
	// It may be EMPTY (no declared app locale) - then routes forward no locale
	// and the user button shows no language submenu.
	// The navigation portal (PORTAL-01), read once here and baked into every
	// UI route's injection: on, the route wears the portal bar (which carries
	// the account button) instead of the standalone user button. Changing the
	// setting reloads the router, exactly as changing the branding does - both
	// are compiled into the routes below.
	portalOn := rt.st.Portal(ctx).Mode == store.PortalModePortal
	// The tracing setting (OBS-04), read here for the same reason: it is baked
	// into the injection of every route that asked for one, so changing the
	// setting reloads the router exactly as changing the branding does. WHICH
	// routes carry the bundle is the routes' own answer, read below.
	tel := rt.st.RawTelemetry(ctx)
	rt.traceExport = tel.ExportsTraces()
	rt.traceSample = tel.Sample
	// Which routes start their journeys in the page is what opens the relay
	// (OBS-04). Read from the routes on every reload rather than from a
	// setting: adding the first such route opens the path, removing the last
	// one closes it, and nobody has to remember a second switch.
	wantsBrowser := false
	for _, r := range stored {
		if r.Enabled && r.IsUI && !noTracing(r) && r.TelemetryUI {
			wantsBrowser = true
			break
		}
	}
	tracing.SetBrowserWanted(wantsBrowser)
	// Vault values feed the $name expansion below. A vault that cannot be read
	// is not a reason to stop serving: routes without references still work,
	// and the ones with references will report their unresolved names.
	values, err := rt.st.VaultValues(ctx, vault.ScopeInfra)
	if err != nil {
		slog.Warn("vault unavailable, route references will not resolve", "err", err)
		values = map[string]string{}
	}
	// The specs deposited on routes (SVC-06), in one query rather than route by
	// route: they are served from memory, so the conversion to JSON and the
	// retargeting are paid once here instead of on every read.
	specs, err := rt.st.RouteSpecContents(ctx)
	if err != nil {
		slog.Warn("deposited openapi specs unavailable", "err", err)
		specs = map[string][]byte{}
	}
	// What the installation asks of every route unless a route says otherwise
	// (PERF-02 for the memory ceiling, ROUTE-07 for the waits). Read BEFORE
	// compiling, because the compilation is what bakes the waits into a
	// transport.
	limits := rt.st.GetProxyLimits(ctx)
	filtering.SetMaxRewritableBody(int64(limits.BodyRewriteMiB) << 20)
	rt.defaultTimeouts = limits.Timeouts

	// What each route that will NOT be served says for itself. Replaced whole on
	// every reload, so a route that was mended stops being listed.
	problems := map[string]string{}
	compiled := make([]compiledRoute, 0, len(stored))
	var allPreds []*routing.CompiledPredicates
	needDraw := false
	for _, raw := range stored {
		if !raw.Enabled {
			continue
		}
		r, missing, err := ExpandRoute(raw, values)
		if err != nil {
			// Left out, not fatal - see the compile error below, which is the
			// same decision for the same reason.
			problems[raw.ID] = err.Error()
			slog.Error("route left out: its references could not be expanded",
				"route", raw.Name, "err", err)
			continue
		}
		if len(missing) > 0 {
			// Left OUT rather than compiled. A route whose references do not
			// resolve cannot serve anything - its upstream is "https://" with no
			// host - and failing the whole reload over it would take every other
			// route down with it. That is the normal state of a gateway just
			// seeded from a file (CFG-03): the configuration is in place, the
			// vault is not filled yet, and it has to start anyway.
			problems[raw.ID] = "references vault entries that hold nothing: " + strings.Join(missing, ", ")
			slog.Warn("route left out: it references vault entries that hold nothing",
				"route", raw.Name, "names", missing)
			continue
		}
		cr, err := rt.compile(r, specs[raw.ID], portalOn)
		if err != nil {
			// LEFT OUT rather than fatal. A route is refused at save time, so a
			// stored one that no longer compiles came from somewhere else: a
			// configuration imported from another version, a restore, an image
			// rolled back past the brick a route names. Taking the whole
			// gateway down then - every other route with it, on a boot that
			// never completes - is the worst possible answer to one bad line.
			// The route does not serve, the reason is logged, and it is MARKED:
			// a route silently missing is the second worst answer.
			problems[raw.ID] = err.Error()
			slog.Error("route left out: it does not compile", "route", raw.Name, "err", err)
			// It would have MATCHED, though, so it does not hand its traffic to
			// the next route in silence. Falling through is the right answer to
			// an ACCESS refusal - two routes on the same paths for two
			// audiences is a shape this product offers - and the wrong one to a
			// configuration defect: the caller would land somewhere nobody
			// meant, and the defect would show up as somebody else's 404. So
			// the route keeps matching, with its own access rule, and answers
			// the unavailable page. Only when its PREDICATES are what failed is
			// it gone entirely, because then there is nothing left to match on.
			if broken, ok := rt.brokenRoute(r); ok {
				compiled = append(compiled, broken)
				allPreds = append(allPreds, &compiled[len(compiled)-1].preds)
			}
			continue
		}
		compiled = append(compiled, cr)
		allPreds = append(allPreds, &compiled[len(compiled)-1].preds)
		needDraw = needDraw || cr.preds.HasWeight()
	}
	if err := routing.ResolveWeights(allPreds); err != nil {
		return fmt.Errorf("gateway: %w", err)
	}
	// Identity signing keys (signed-jwt): generate them the first time a route
	// actually needs them; otherwise just load what already exists (so the JWKS
	// keeps serving), leaving fresh installs that never sign untouched.
	needSigning := false
	for _, r := range stored {
		if r.Enabled && r.Identity != nil && r.Identity.Mechanism == "signed-jwt" {
			needSigning = true
			break
		}
	}
	var sset *signing.Set
	if needSigning {
		if sset, err = rt.st.EnsureSigningSet(ctx); err != nil {
			return fmt.Errorf("gateway: identity signing keys: %w", err)
		}
	} else if loaded, ok, e := rt.st.GetSigningSet(ctx); e == nil && ok {
		sset = loaded
	}
	// The simulation key, shared between the nodes. A failure is not fatal:
	// the router keeps the one it has and the simulator keeps working on this
	// node - refusing to serve routes because a developer tool cannot sign
	// would be the wrong trade by a wide margin.
	simKey, err := rt.st.EnsureSimulationKey(ctx)
	if err != nil {
		slog.Warn("simulation key unavailable, keeping this node's own", "err", err)
	}

	// The global maintenance switch. Read here so it applies at once and, via
	// the reload the control plane announces, on every node.
	maint := rt.st.GetMaintenance(ctx)
	// And the mark the unavailable page wears: it is the one page every
	// visitor meets during an outage, so it carries the installation's name
	// rather than looking like some gateway they never heard of.
	brand := store.DefaultBranding()
	if err := rt.st.GetSetting(ctx, store.SettingBranding, &brand); err != nil {
		brand = store.DefaultBranding()
	}
	routing.SetMaintenanceBrand(routing.MaintenanceBrand{
		LogoURL: brand.Logo, LogoSize: brand.LogoSize, AppName: brand.AppName,
	})

	// A deleted route's circuit has nobody to be about, and an id that comes
	// back is a new route deserving a clean slate.
	alive := make(map[string]bool, len(compiled))
	for _, c := range compiled {
		alive[c.id] = true
	}
	rt.breakers.forget(alive)
	// Same for what its requests were attributed to: a route that is gone has
	// no operations, and its slot would be a map entry nobody ever reads again.
	rt.pruneOps(alive)

	rt.mu.Lock()
	rt.routes = compiled
	rt.problems = problems
	rt.needDraw = needDraw
	rt.signing = sset
	rt.maintenance, rt.maintenancePage = maint, renderMaintenance(maint)
	if simKey != nil {
		rt.simTokenKey = simKey
	}
	rt.mu.Unlock()
	rt.loaded.Store(true)
	slog.Info("routes reloaded", "count", len(compiled), "left out", len(problems))
	return nil
}

// Problems answers, for each route the gateway is NOT serving, why. The console
// reads it: a route that quietly stopped existing is a support call, and the
// operator has to see the defect where they see the route.
func (rt *Router) Problems() map[string]string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	out := make(map[string]string, len(rt.problems))
	for id, msg := range rt.problems {
		out[id] = msg
	}
	return out
}

// Ready reports whether this router has ever finished a reload.
//
// For the READINESS probe. Between accepting connections and compiling its
// first table, a node answers 404 to everything it is about to route - which
// an orchestrator reads as a healthy instance serving a wrong answer, and a
// rolling update sends live traffic straight into it. Zero routes is a
// legitimate answer here: an installation with an empty table is ready to say
// so. Never having asked is not.
func (rt *Router) Ready() bool { return rt.loaded.Load() }

// Metrics is what the console reads and what the OTLP push sends.
func (rt *Router) Metrics() *metrics.Registry { return rt.metrics }

// record wraps the writer so a route learns what it answered and how long it
// took, and returns the function that writes it down.
//
// Called by EVERY exit that belongs to a route, refusals included: a route
// that turns everybody away with a 403 is answering, and showing it idle would
// hide exactly the route somebody is asking about. A handler that wrote
// nothing at all still answered 200 as far as net/http is concerned, which is
// why the zero is resolved here rather than counted as an unnameable status.
func record(w http.ResponseWriter) (*watched, time.Time) {
	return &watched{ResponseWriter: w}, time.Now()
}

// observed writes down what record watched. A plain call and not a closure
// returned by record: a closure capturing the writer and the start escapes to
// the heap, which is a second allocation on the path of every request for the
// convenience of writing `defer` at the call site.
func observed(r *compiledRoute, req *http.Request, ww *watched, start time.Time) {
	// Whatever this route answered - its upstream, a refusal, its unavailable
	// page - it is the route the router chose, and a route that is not traced
	// emits nothing. The handler said so already for the call out (see
	// compile); this covers the refusals, which never reach a handler.
	if r.noTrace {
		dropSpan(req.Context())
	}
	status := ww.status
	if status == 0 {
		status = http.StatusOK
	}
	took := time.Since(start)
	r.counters.Observe(status, took)
	// And the operation, when the route can name one. The lookup happens once
	// the request is ANSWERED rather than before it: it costs a handful of
	// template comparisons, and paying them on the way out keeps them off the
	// path of a request that is still waiting for its upstream.
	endpoint := ""
	if op := r.ops.of(req); op != nil {
		op.Observe(status, took)
		endpoint = op.Path
	}
	// The same exit writes the access line, refusals included: a 403 is the
	// line an audit wants most, and it is the one the service behind can never
	// write because it never saw the call (OBS-03).
	writeAccess(req, r.name, endpoint, status, ww.bytes, took)
	finishSpan(req, r.name, endpoint, status)
}

// adminOrigin reports whether origin is this gateway's own admin console: the
// same hostname the data-plane request came in on, at the control plane's
// port. A route pinned to another hostname by a host predicate falls outside
// this rule - its Try it out would need the application's own CORS.
func (rt *Router) adminOrigin(r *http.Request, origin string) bool {
	if rt.AdminAddr == "" {
		return false
	}
	_, adminPort, err := net.SplitHostPort(rt.AdminAddr)
	if err != nil || adminPort == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" {
		return false
	}
	originPort := u.Port()
	if originPort == "" {
		if u.Scheme == "https" {
			originPort = "443"
		} else {
			originPort = "80"
		}
	}
	hostname := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		hostname = h
	}
	return originPort == adminPort && strings.EqualFold(u.Hostname(), hostname)
}

// stripGatewayCookies removes Meerkat's own session cookies from an outgoing
// upstream request, keeping every other cookie the application may rely on.
func stripGatewayCookies(r *http.Request) {
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cookies {
		if c.Name == session.CookieName || c.Name == session.AdminCookieName {
			continue
		}
		r.AddCookie(c)
	}
}

// simKey returns the key the test tokens are signed with. Read under the lock
// for the same reason as the signing set: Reload swaps it while requests are
// being served.
func (rt *Router) simKey() []byte {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.simTokenKey
}

// currentSigning returns the active signing set (nil when none). Read under the
// lock so a Reload swap is race-free.
func (rt *Router) currentSigning() *signing.Set {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.signing
}

// serveJWKS publishes the public halves of the signing keys. Empty (but valid)
// when no key exists yet, so a backend can always fetch and cache it.
func (rt *Router) serveJWKS(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	set := rt.currentSigning()
	if set == nil {
		_, _ = w.Write([]byte(`{"keys":[]}`))
		return
	}
	body, err := set.JWKS()
	if err != nil {
		http.Error(w, "jwks unavailable", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

// ExpandRoute resolves the $name references a route carries (VAULT-01) against
// values, returning the route the engine will actually run plus the names that
// did not resolve. The expansion is IN MEMORY only: the stored route keeps its
// references, so a secret never lands in the database or in an export.
//
// It walks the route's decoded JSON, so a reference works in any string field
// (upstream, filter arguments, header names...) without listing them one by one.
func ExpandRoute(r store.Route, values map[string]string) (store.Route, []string, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return r, nil, fmt.Errorf("gateway: route %q: %w", r.Name, err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return r, nil, fmt.Errorf("gateway: route %q: %w", r.Name, err)
	}
	expanded, missing := vault.ExpandAny(doc, func(name string) (string, bool) {
		v, ok := values[name]
		return v, ok
	})
	out, err := json.Marshal(expanded)
	if err != nil {
		return r, missing, fmt.Errorf("gateway: route %q: %w", r.Name, err)
	}
	var resolved store.Route
	if err := json.Unmarshal(out, &resolved); err != nil {
		return r, missing, fmt.Errorf("gateway: route %q: %w", r.Name, err)
	}
	missing = restoreLiteralArgs(r, &resolved, missing)
	missing = restoreCode(r, &resolved, missing)
	return resolved, missing, nil
}

// restoreCode puts back the blocks a route carries as SOURCE, and drops the
// vault names they invented on the way through. A script writes $translate and
// $rootScope, a stylesheet writes $ in a selector, and the expansion read them
// as references: the route was refused for "unknown vault entries:
// translate.use, rootScope.". Same collision as the template variables next
// door, same answer - the two syntaxes share the dollar and code owns it here.
//
// A secret has no business in these anyway: they travel verbatim into a page
// anyone can read, and into a configuration export that is public by
// construction.
func restoreCode(original store.Route, resolved *store.Route, missing []string) []string {
	invented := map[string]bool{}
	note := func(raw string) {
		for _, ref := range vault.Refs(raw) {
			invented[ref] = true
		}
	}
	if original.UI != nil && resolved.UI != nil {
		note(original.UI.CustomJS)
		note(original.UI.CustomCSS)
		resolved.UI.CustomJS, resolved.UI.CustomCSS = original.UI.CustomJS, original.UI.CustomCSS
	}
	if original.Locales != nil && resolved.Locales != nil {
		note(original.Locales.OnChange)
		resolved.Locales.OnChange = original.Locales.OnChange
	}
	if len(invented) == 0 {
		return missing
	}
	kept := missing[:0]
	for _, name := range missing {
		if !invented[name] {
			kept = append(kept, name)
		}
	}
	return kept
}

// restoreLiteralArgs puts back the filter arguments that must not be expanded,
// and drops the "missing" names they invented on the way through.
//
// A Go template writes $i and $r for its own loop variables; the expansion read
// them as vault references and the route was refused for "unknown vault
// entries: i, r". The two syntaxes share the dollar and only one of them owns
// it here - the template's, since its body is taken verbatim.
func restoreLiteralArgs(original store.Route, resolved *store.Route, missing []string) []string {
	invented := map[string]bool{}
	for i, spec := range original.Filters {
		if i >= len(resolved.Filters) {
			break
		}
		for _, name := range routing.LiteralArgs(spec.Type) {
			raw, ok := spec.Args[name].(string)
			if !ok {
				continue
			}
			for _, ref := range vault.Refs(raw) {
				invented[ref] = true
			}
			resolved.Filters[i].Args[name] = raw
		}
	}
	if len(invented) == 0 {
		return missing
	}
	kept := missing[:0]
	for _, name := range missing {
		if !invented[name] {
			kept = append(kept, name)
		}
	}
	return kept
}

// JWKSPath is the well-known location where the gateway publishes the public
// halves of its identity signing keys, for upstreams verifying signed-jwt.
const JWKSPath = "/.well-known/jwks.json"

// ServeHTTP dispatches to the first route whose predicates all match; nothing
// matched is a plain 404. The TRAP (ROUTE-10) is not a special case: it is an
// ordinary catch-all route ("/**") the admin orders last.
func (rt *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// A name for this request, before anything can refuse it (OBS-04). Here
	// rather than in the proxy on purpose: a call turned away by an access
	// rule never reaches an upstream, and that refusal is exactly the line an
	// audit wants to be able to join to something.
	sc := tracing.FromRequest(req)
	req = req.WithContext(tracing.With(req.Context(), sc))
	// And the gateway's own span, when somebody is exporting and this journey
	// was sampled. Nothing is allocated otherwise (OBS-04).
	req = withSpan(req, sc)
	// And a place to write down who this turns out to be, filled below where
	// the router resolves it anyway. Nil - and free - when nobody asked for an
	// access log (OBS-03).
	req = withAccess(req)
	// And the caller gets the name of their own request back. This is the
	// whole support gesture - "give me the identifier on the page", pasted
	// into a search, landing on the line - and it costs one header. Not a
	// secret: it is the name of a journey the caller is already on.
	w.Header().Set(tracing.HeaderOut, tracing.ID(req.Context()))
	// The JWKS is a gateway-internal endpoint: it wins over any route (a
	// catch-all trap must never swallow it).
	if req.Method == http.MethodGet && req.URL.Path == JWKSPath {
		rt.serveJWKS(w)
		return
	}
	// The admin console's swagger page (Try it out) calls the routes straight
	// on this plane, from the control plane's origin: answer CORS for THAT one
	// sibling origin - and no other, the applications behind the gateway keep
	// their own policies.
	if origin := req.Header.Get("Origin"); origin != "" && rt.adminOrigin(req, origin) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Add("Vary", "Origin")
		if req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
			if reqHeaders := req.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
				h.Set("Access-Control-Allow-Headers", reqHeaders)
			}
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	// Maintenance (LIFE-05), before any route is looked at: what it answers
	// for is EVERY route, which is the point of a switch that needs no route
	// edited. Below the JWKS above it on purpose - a backend still holding a
	// token has to be able to finish verifying it - and above everything else.
	if rt.takeBypass(w, req) {
		return
	}
	req, answer, down := rt.underMaintenance(w, req)
	if down {
		rt.serveMaintenance(w, req, answer)
		return
	}
	// Identity simulation (Try it out): validated headers replace the session
	// for this request; unauthorized simulation is an explicit 403, not a
	// silent fallback to the caller's real identity.
	req, simErr := rt.applySimulation(req)
	if simErr != nil {
		http.Error(w, simErr.Error(), http.StatusForbidden)
		return
	}
	rt.mu.RLock()
	routes, needDraw := rt.routes, rt.needDraw
	rt.mu.RUnlock()
	if needDraw {
		req = req.WithContext(routing.WithLottery(req.Context(), rt.lottery()))
	}
	// FIRST match wins, and a route's security is part of matching: two routes
	// may cover the same paths and differ only by who they are for. A caller
	// the rule turns away falls through to the next route that matches.
	//
	// The first route that turned them away is kept as the CANDIDATE. If
	// nothing else answers, that candidate produces the refusal - with the
	// reason it knows and the organisation switch it can offer. Without it a
	// refusal would arrive as a bare 404, which is the one answer nobody can
	// act on, and the whole point of naming what is missing would be lost.
	var (
		cand       *compiledRoute
		candReq    *http.Request
		candOK     bool
		candWho    store.Caller
		candUserID string
	)
	for i := range routes {
		if !routes[i].preds.Match(req) {
			continue
		}
		// A running UI test (uisim.go) poses its identity on every request the
		// dev session sends through the tested route - so it is applied BEFORE
		// the rule is evaluated, or the test would be judged on the real
		// person rather than on the identity under test.
		r, hit := &routes[i], rt.applyUISim(req, routes[i].id)
		if r.access.Empty() {
			// A route with no rule is still a route that was chosen, and the
			// span should say which one answered.
			spanEvent(hit.Context(), "route_chosen", tracing.String("meerkat.route", r.name))
			// And who called it, which nothing here needed to know. Only for a
			// request being recorded, and only when the spans name the caller:
			// the session lookup is not free.
			if n := spanOf(hit.Context()); n != nil && !n.fromPage && tracing.Caller() {
				rt.sessionIdentity(hit)
			}
			ww, start := record(w)
			rt.answerOrRefuse(ww, hit, r, start, cand, candReq, candOK, candWho, candUserID)
			return
		}
		d, ok := rt.sessionIdentity(hit)
		noteIdentity(hit.Context(), d)
		who := rt.caller(hit, d, ok)
		if (ok && r.access.Grants(who)) || isSpecRead(hit.Context()) {
			// Two instants worth naming inside the span, for the cost of two
			// names: which route won, and that the door opened. A span apiece
			// would be thousands per request.
			spanEvent(hit.Context(), "route_chosen", tracing.String("meerkat.route", r.name))
			spanEvent(hit.Context(), "access_granted")
			ww, start := record(w)
			rt.answerOrRefuse(ww, hit, r, start, cand, candReq, candOK, candWho, candUserID)
			return
		}
		// A closed door stays closed. Falling through a "deny" to whatever
		// matches next would turn the one rule written to shut a path into a
		// rule that merely redirects it.
		if r.access.Level == store.AccessDeny {
			spanEvent(hit.Context(), "access_refused", tracing.String("meerkat.route", r.name))
			ww, start := record(w)
			rt.refuse(ww, hit, r.access, r.isUI, ok, who, d.UserID)
			observed(r, hit, ww, start)
			return
		}
		if cand == nil {
			cand, candReq, candOK, candWho, candUserID = r, hit, ok, who, d.UserID
		}
	}
	if cand != nil {
		spanEvent(candReq.Context(), "access_refused", tracing.String("meerkat.route", cand.name))
		ww, start := record(w)
		rt.refuse(ww, candReq, cand.access, cand.isUI, candOK, candWho, candUserID)
		observed(cand, candReq, ww, start)
		return
	}
	// No route at all. Counted gateway-wide rather than nowhere: a rising
	// number of requests that match nothing is a misconfiguration somebody
	// should see, and it belongs to no route by definition.
	rt.metrics.Unmatched()
	ww, start := record(w)
	http.NotFound(ww, req)
	writeAccess(req, "", "", ww.status, ww.bytes, time.Since(start))
	// Not traced: tracing is each route's answer, and no route answered. A
	// 404 is nobody's journey - an asset a front end asks for at the root, a
	// path that used to exist - and it was what filled the backend with
	// traces from routes that had said no. It stays counted (Unmatched) and
	// logged.
	dropSpan(req.Context())
}

func (rt *Router) compile(r store.Route, deposited []byte, portalOn bool) (compiledRoute, error) {
	preds, err := routing.CompilePredicates(r.Predicates)
	if err != nil {
		return compiledRoute{}, err
	}
	filters, err := routing.CompileFilters(r.Filters)
	if err != nil {
		return compiledRoute{}, err
	}

	// The language offer is the APPLICATION's; the route only adds transport
	// mechanisms. Accept-Language is ALWAYS forwarded with the resolved
	// locale promoted to the front - every proxied route, no opt-out.
	var localeCfg store.LocalesConfig
	if r.Locales != nil {
		localeCfg = *r.Locales
	}
	// What this route SAYS IT SPEAKS, and nothing else. There is no pool to
	// subtract from any more: the route is where the knowledge is, and the
	// gateway's own offer is the union of these (store.SpokenLanguages).
	//
	// ALWAYS FITTED, declared languages or not. Accept-Language carrying the
	// person's own language is the FLOOR - it is not one of the mechanisms,
	// and an API route that declares nothing still serves somebody who chose a
	// language. Guarding this on a non-empty list was how the pivot to Speaks
	// quietly stopped promoting it on every route that had not been given one.
	localeCodes := localeCfg.Speaks
	filters.Request = append(filters.Request,
		// The literal head of the route's path patterns is where the language
		// gets inserted - see insertLocale.
		localeForwardFilter(localeCodes, localeCfg, routing.PathPrefixes(r.Predicates)))

	// What the gateway adds to a UI route's HTML pages, at the TOP OF THE BODY:
	// a custom element inside <head> closes it where it stands, and everything
	// after it - charset, title, and <base href> - lands in the body, where a
	// base is ignored. An application served under a prefix then resolves from
	// the root.
	//
	// ONE injection for the two, and in this order: both scripts are deferred,
	// so they run in document order, and the button's element must not upgrade
	// before window.meerkatPage exists. Two InjectAtBodyStart filters would
	// each insert right after <body>, putting the second one FIRST.
	//
	// With a portal configured (PORTAL-01), the standalone user button gives
	// way to the portal bar, which mounts the same button inside itself: the
	// navigation and the account menu are one surface, not two corners.
	// The tracing bundle FIRST, before anything else we inject: it patches
	// fetch and XMLHttpRequest, and a call made by one of our own scripts
	// before that patch is in place is a call missing from the trace.
	frag := rt.telemetryFragment(r)
	frag += pageAgentFragment(r, localeCodes)
	if portalOn && r.IsUI {
		frag += portalFragment(r, localeCodes)
	} else {
		frag += userButtonFragment(r, localeCodes)
	}
	if frag != "" {
		filters.Response = append(filters.Response, filtering.InjectAtBodyStart(frag))
	}
	// Page injections (UIF): the session's effective roles and the user's
	// identity are stamped SERVER-SIDE onto the served HTML - roles as a class
	// or attribute on the target tag (default body) or a meta, user fields
	// likewise. No client JS, no callback home (/meerkat/page.js stays served
	// for by-hand use). The gate is a cheap session check so anonymous requests
	// never buffer the response.
	if r.IsUI && r.UI != nil &&
		((r.UI.Roles != nil && r.UI.Roles.Enabled) || (r.UI.UserInfo != nil && r.UI.UserInfo.Enabled)) {
		route, codes := r, localeCodes
		filters.Response = append(filters.Response,
			filtering.RewriteHTMLFunc(
				func(res *http.Response) bool { return rt.hasIdentity(res.Request) },
				func(res *http.Response, body []byte) []byte { return rt.pageStamp(route, codes, res, body) },
			))
	}
	// Identity forwarding (both route types): the signed-in user rides
	// upstream headers; inbound values are purged first (spoofing guard).
	if r.Identity != nil && r.Identity.Mechanism != "" {
		filters.Request = append(filters.Request, rt.identityForwardFilter(*r.Identity, r.Name))
	}
	// A UI route's custom CSS rides a <style> tag ("</style" is refused at
	// validation, so the block cannot break out).
	if r.IsUI && r.UI != nil && r.UI.CustomCSS != "" {
		filters.Response = append(filters.Response,
			filtering.InjectAfterHead("<style>\n"+r.UI.CustomCSS+"\n</style>"))
	}
	// Same deal for the custom JS, on a <script> tag ("</script" refused).
	if r.IsUI && r.UI != nil && r.UI.CustomJS != "" {
		filters.Response = append(filters.Response,
			filtering.InjectAfterHead("<script>\n"+r.UI.CustomJS+"\n</script>"))
	}
	// The language hook (I18N-04): declared per route, it lands as a function
	// on the window rather than as an attribute on the button - it is code,
	// and code belongs in a <script>, not in HTML quoted twice over. The
	// button looks it up by name when someone picks a language.
	if r.IsUI && localeCfg.OnChange != "" && localeCfg.Mode() == store.LocaleScript {
		filters.Response = append(filters.Response, filtering.InjectAfterHead(
			"<script>\nwindow."+store.LocaleHookName+" = function (locale) {\n"+localeCfg.OnChange+"\n};\n</script>"))
	}

	// The maintenance stripe (LIFE-05), on EVERY route: an administrator who
	// took the door browses an application that is down for everyone else, and
	// nothing said so - which is the same silence the door was opened to fix,
	// moved one step along. Costs a Content-Type check on HTML answers and
	// nothing at all otherwise: the fragment is empty unless this very request
	// went through the door.
	filters.Response = append(filters.Response,
		filtering.InjectAfterHeadFunc(func(res *http.Response) string {
			if res.Request == nil || !bypassing(res.Request.Context()) {
				return ""
			}
			// Personal for the same reason a stamped page is: only the
			// administrator who took the door sees this band, and a cache
			// that kept the page would show "under maintenance" to people
			// who are not in maintenance - or, worse, the reverse.
			filtering.Personal(res.Header)
			if rt.Stripe != nil {
				return rt.Stripe(res.Request)
			}
			return maintenanceStripe
		}))

	var handler http.Handler
	if filters.Terminal != nil {
		handler = filters.Terminal
		// Outgoing filters apply to what the route answers itself, exactly as
		// they do to a proxied response: a CORS or Cache-Control header on an
		// identity endpoint is the same need either way.
		if len(filters.Response) > 0 {
			handler = filterOwnResponse(handler, filters.Response)
		}
		// A terminal that answers FROM the caller gets one resolved and carried
		// in. Only that kind pays the read: redirect and maintenance answer the
		// same thing to everyone.
		if filters.TerminalNeedsIdentity {
			handler = rt.withIdentity(handler)
		}
	} else {
		unavailable := ""
		if portalOn && r.IsUI {
			unavailable = unavailablePage(pageAgentFragment(r, localeCodes) + portalFragment(r, localeCodes))
		}
		handler, err = buildProxy(r, filters, rt.defaultTimeouts, unavailable)
		if err != nil {
			return compiledRoute{}, err
		}
		// The circuit sits around the PROXY and nothing else: a route that
		// answers by itself - a redirect, the unavailable page, a template -
		// has no upstream to stop calling.
		handler = rt.guarded(r, handler)
	}
	// Per-endpoint security (RBAC-07): when an API route poses operation
	// policies, wrap the handler with the endpoint guard INSIDE the route-level
	// auth, so a route-wide gate (if any) is applied first and the per-operation
	// rule refines it. The guard maps the inbound path back to the OpenAPI
	// coordinate by undoing the route's strip-prefix.
	// Route security (RBAC-06/07): the route's base Access gates the whole route.
	// When the API route also poses per-operation policies, the endpoint guard
	// refines that base per operation (mapping the inbound path back to the
	// OpenAPI coordinate by undoing the strip-prefix); operations with no
	// override fall back to the route's base Access.
	// selectAccess is the rule that takes part in CHOOSING the route. A route
	// with per-operation overrides keeps its rule inside, in the guard: the
	// overrides may REOPEN an operation the route-wide rule closes, and a
	// selection that judged the route first would refuse before the guard ever
	// got to reopen anything. Per-operation rules live within one route by
	// definition, so they were never what selection had to tell apart.
	selectAccess := r.Access
	hasOverrides := r.API != nil && r.API.Security != nil &&
		(len(r.API.Security.Endpoints) > 0 || r.API.Security.DenyUnlisted)
	if hasOverrides {
		selectAccess = store.Access{}
		if rt.sm == nil {
			return compiledRoute{}, fmt.Errorf("route poses endpoint security but no session manager is configured")
		}
		guard, err := rt.endpointGuard(*r.API.Security, r.Access, r.IsUI, r.Filters, handler)
		if err != nil {
			return compiledRoute{}, err
		}
		handler = guard
	} else if !r.Access.Empty() && rt.sm == nil {
		return compiledRoute{}, fmt.Errorf("route poses security but no session manager is configured")
	}
	// Endpoint audit (AUD-04), around the guard: it sees the answer the caller
	// got, refusals included, and decides nothing.
	if r.API != nil && len(r.API.Audit) > 0 {
		if err := store.ValidateAudit(r.API.Audit); err != nil {
			return compiledRoute{}, err
		}
		auditor, err := rt.endpointAuditor(r.Name, r.API.Audit, r.Filters, handler)
		if err != nil {
			return compiledRoute{}, err
		}
		handler = auditor
	}
	// The route-level gate is NOT wrapped here any more: ServeHTTP evaluates it
	// while choosing, so a caller the rule turns away can be served by the next
	// route that matches. The endpoint guard above keeps its own copy of the
	// rule, as the fallback for operations with no override of their own.
	// The URL is kept in step with the person, outside the gates: it costs one
	// string comparison and says nothing about who is asking. Only where the
	// language LIVES in the URL - elsewhere a path segment that happens to read
	// like a code means nothing.
	inPath := localeCfg.Mode() == store.LocalePath
	inQuery := localeCfg.Mode() == store.LocaleQuery
	if len(localeCodes) > 0 && (inPath || inQuery) {
		param := ""
		if inQuery {
			param = orDefault(localeCfg.Param, "lg")
		}
		handler = redirectToLocale(handler, localeCodes, routing.PathPrefixes(r.Predicates), inPath, param)
	}
	// A deposited spec (SVC-06) answers on the route's own prefix, ahead of the
	// upstream. OUTSIDE the endpoint guard on purpose: the file inherits the
	// route's access rule, not its per-operation policies - a deny-by-default
	// posed on the operations would otherwise refuse the very document that
	// lists them, and the contract would become unreadable exactly where it is
	// most needed. When overrides exist the route's own rule no longer takes
	// part in selection (it lives inside the guard), so it is applied here.
	if spec := r.Spec(); spec.Type == store.SpecFile && len(deposited) > 0 {
		body, err := openapi.Normalize(deposited)
		if err != nil {
			return compiledRoute{}, fmt.Errorf("deposited openapi spec: %w", err)
		}
		// Served from the route, so the operations it declares are reachable at
		// the very prefix it was fetched from - no reader has to guess which
		// base the gateway exposes.
		if rewritten, rErr := openapi.Rewrite(body, routeMatchPrefix(r)); rErr == nil {
			body = rewritten
		}
		served := specFileHandler(body)
		if hasOverrides {
			served = rt.accessGate(r.Access, r.IsUI, served)
		}
		handler = serveSpecFile(routeMatchPrefix(r)+"/"+spec.Path, served, handler)
	}
	// An application published under a path is entered by a URL people type,
	// paste and bookmark, and the one they write is the prefix bare. The page
	// comes back - a single trailing slash is ignored when the route matches -
	// but the BROWSER then resolves every relative link in it against the parent
	// directory: from /rmq, src="js/main.js" is /js/main.js, and the whole
	// application 404s while the page that asked for it looks fine.
	//
	// Nothing on the server can answer that, because the page is not what is
	// wrong: serving the slashed path instead would return the same bytes and
	// leave the browser on the same URL, with the same idea of its own
	// directory. Only a REDIRECT changes that, so it is a 3xx or nothing.
	//
	// Two conditions, and the second was learned from a test that broke:
	//
	//   - a UI ROUTE, because a service route's mount path is often a resource
	//     of its own - GET /orders on /orders/** is the collection - and machine
	//     callers do not all follow redirects;
	//   - which STRIPS its prefix, because that is what republishing means: the
	//     application behind sees / and writes its links relative to that, while
	//     the gateway shows it under /rmq. A UI route that does NOT strip knows
	//     its own prefix, its pages are already right, and the redirect would
	//     only ask the upstream for a path it never published.
	//
	// Nothing below the prefix is touched, which is also what makes a loop
	// impossible: the target is not the path that is answered.
	if r.IsUI && stripsPrefix(r) {
		handler = redirectToMountSlash(routeMatchPrefix(r), handler)
	}
	// Gates (ROUTE-04) go on LAST, so they sit outermost: what a route refuses
	// to carry is decided before the access rule reads a session, before a
	// modifier touches a header, and before a redirect sends the caller round
	// again. Refusing early is the whole point - an oversized body must not be
	// read to be turned away.
	handler = gateChain(filters.Gates, handler)
	// And the rate limits (ROUTE-08) go on after them, so they sit further out
	// still: a request over the bound is refused before its Content-Length is
	// even looked at. The two kinds are wrapped at the same depth but in this
	// order, so the bounds that need NO identity answer before the ones that
	// cost a session resolve - a flood is turned away without ever touching
	// the session store.
	freeLimits, identifiedLimits := compileLimits(r.Limits)
	handler = rt.rateGate(identifiedLimits, handler)
	handler = rt.rateGate(freeLimits, handler)
	cfg := store.CircuitBreaker{}
	if r.Breaker != nil {
		cfg = *r.Breaker
	}
	ops := rt.opsFor(r.ID)
	ops.setBase(stripPrefixCount(r.Filters), baseOperations(r, deposited))
	if noTracing(r) {
		handler = dropTracing(handler)
	}
	return compiledRoute{id: r.ID, name: r.Name, preds: preds, handler: handler, breaker: cfg,
		access: selectAccess, isUI: r.IsUI, noTrace: noTracing(r), counters: rt.routeCounters(r),
		ops: ops}, nil
}

// noTracing reads the route's tracing switch. A route that never mentioned the
// subject is traced: an installation that turned the export on wants its
// traffic traced, and leaves out what it does not want to follow.
// routeCounters is the route's block in the registry, told whether its own
// series leave: the route's OpenTelemetry switch covers its traces AND its
// metrics. Still counted either way - the console's screen and the gateway's
// totals include every route.
func (rt *Router) routeCounters(r store.Route) *metrics.Route {
	c := rt.metrics.For(r.ID, r.Name)
	c.Exclude(noTracing(r))
	return c
}

func noTracing(r store.Route) bool {
	return r.Telemetry != nil && !*r.Telemetry
}

// brokenRoute is what is left of a route that does not compile: its predicates,
// its access rule, and the unavailable page. See the call site for why it keeps
// matching rather than disappearing. False when the predicates are themselves
// what failed - there is then nothing to match on, and the route is simply gone.
func (rt *Router) brokenRoute(r store.Route) (compiledRoute, bool) {
	preds, err := routing.CompilePredicates(r.Predicates)
	if err != nil {
		return compiledRoute{}, false
	}
	return compiledRoute{
		id: r.ID, name: r.Name, preds: preds,
		handler:  http.HandlerFunc(rt.serveUnreachable),
		access:   r.Access,
		isUI:     r.IsUI,
		noTrace:  noTracing(r),
		counters: rt.routeCounters(r),
		ops:      rt.opsFor(r.ID),
	}, true
}

// Validate checks that a route would compile - same checks as Reload, minus
// the session-manager wiring. The admin API uses it to refuse invalid routes
// with the engine's precise error before anything is persisted.
func Validate(r store.Route) error {
	if _, err := routing.CompilePredicates(r.Predicates); err != nil {
		return err
	}
	cf, err := routing.CompileFilters(r.Filters)
	if err != nil {
		return err
	}
	if cf.Terminal == nil {
		target, err := url.Parse(r.Upstream)
		if err != nil {
			return fmt.Errorf("bad upstream %q: %w", r.Upstream, err)
		}
		if target.Scheme == "" || target.Host == "" {
			return fmt.Errorf("bad upstream %q: scheme and host required", r.Upstream)
		}
		if !slices.Contains(upstreamSchemes, target.Scheme) {
			return fmt.Errorf("bad upstream %q: scheme %q is not supported: %s",
				r.Upstream, target.Scheme, strings.Join(upstreamSchemes, ", "))
		}
	}
	return validateRouteType(r)
}

// validateRouteType guards the route options: forwarding configs for every
// route, plus the UI extras when the UI toggle is on (ROUTE-02).
func validateRouteType(r store.Route) error {
	for _, role := range r.Access.Roles {
		if !schemeTokenOK.MatchString(role) {
			return fmt.Errorf("route access role %q is not allowed: letters, digits, - and _ only", role)
		}
	}
	// Endpoint-level security (RBAC-07): paths must compile, methods and access
	// modes be known. Validate also upper-cases the methods in place.
	if r.API != nil {
		if err := r.API.Security.Validate(); err != nil {
			return err
		}
	}
	// Identity forwarding is valid for BOTH types (an API service wants the
	// caller too).
	if id := r.Identity; id != nil && id.Mechanism != "" {
		switch id.Mechanism {
		case "headers", "jwt":
		case "signed-jwt":
			if id.Algorithm != "" && !signing.Valid(id.Algorithm) {
				return fmt.Errorf("identity signature algorithm %q is not allowed: allowed algorithms are %s",
					id.Algorithm, strings.Join(signing.Algorithms, ", "))
			}
		default:
			return fmt.Errorf("identity mechanism %q is not allowed: allowed mechanisms are headers, jwt, signed-jwt", id.Mechanism)
		}
		seen := make(map[string]bool, len(id.Attributes))
		for _, a := range id.Attributes {
			if !slices.Contains(store.IdentityFields, a.Field) && !store.ValidFieldName(a.Field) {
				return fmt.Errorf("identity attribute %q is not allowed: allowed attributes are %s",
					a.Field, strings.Join(store.IdentityFields, ", "))
			}
			if seen[a.Field] {
				return fmt.Errorf("identity attribute %q is set twice", a.Field)
			}
			seen[a.Field] = true
			if a.Expr != "" {
				if a.Field != "roles" {
					return fmt.Errorf("identity attribute %q takes no expression: only roles are shaped", a.Field)
				}
				// Proven to run here, so a broken one is refused at save time
				// rather than on the first request that needs it.
				if _, err := routing.CompileRoleExpr(a.Expr); err != nil {
					return err
				}
			}
			if a.As != "" {
				if id.Mechanism == "headers" && !headerNameOK.MatchString(a.As) {
					return fmt.Errorf("identity header %q for %s is not allowed: letters, digits and - only", a.As, a.Field)
				}
				if (id.Mechanism == "jwt" || id.Mechanism == "signed-jwt") && !claimNameOK.MatchString(a.As) {
					return fmt.Errorf("identity claim %q for %s is not allowed: letters, digits, and _ - . only", a.As, a.Field)
				}
			}
		}
		if id.TTL != "" {
			if _, err := store.ParseISODuration(id.TTL); err != nil {
				return fmt.Errorf("identity token ttl %q is not a valid ISO-8601 duration: %w", id.TTL, err)
			}
		}
	}
	// Locale MECHANISMS are valid for both types; only the path one demands a
	// UI route (an API takes the locale as a header or query parameter).
	// Accept-Language always goes, it is not an option here.
	if lc := r.Locales; lc != nil {
		mode := lc.Mode()
		if !slices.Contains(store.LocaleMechanisms, mode) {
			return fmt.Errorf("locales mechanism %q is not allowed: allowed mechanisms are %s",
				mode, strings.Join(store.LocaleMechanisms, ", "))
		}
		// A SERVICE ROUTE HAS NO CHOICE, and that is not a restriction: an API
		// reads Accept-Language, which every route carries anyway with the
		// person's own language in front. Nobody puts a language in the path
		// or the query string of an API, and a header of one's own is a UI
		// application's convention rather than a service's.
		if mode != store.LocaleAccept && mode != "" && !r.IsUI {
			return fmt.Errorf("locales mechanism %q is only allowed on UI routes: a service route "+
				"reads Accept-Language, which it already gets with the caller's language in front", mode)
		}
		if mode == store.LocaleCustom && !headerNameOK.MatchString(lc.Header) {
			return fmt.Errorf("locales custom header %q is not allowed: letters, digits and - only", lc.Header)
		}
		if lc.Param != "" && !headerNameOK.MatchString(lc.Param) {
			return fmt.Errorf("locales query parameter %q is not allowed: letters, digits and - only", lc.Param)
		}
		if len(lc.Speaks) > 0 && !r.IsUI {
			return fmt.Errorf("a service route declares no language: it serves no page, and the " +
				"caller's own reaches it through Accept-Language")
		}
		for _, code := range lc.Speaks {
			if !headerNameOK.MatchString(code) {
				return fmt.Errorf("spoken locale %q is not allowed: letters, digits and - only", code)
			}
		}
	}
	if r.UI == nil {
		return nil
	}
	if s := r.UI.Scheme; s != nil {
		if s.Mechanism != "" && !slices.Contains(store.SchemeMechanisms, s.Mechanism) {
			return fmt.Errorf("scheme mechanism %q is not allowed: allowed mechanisms are \"\" (color-scheme only), %s",
				s.Mechanism, strings.Join(store.SchemeMechanisms, ", "))
		}
		if s.Mechanism == store.SchemeScript && strings.TrimSpace(s.Script) == "" {
			return fmt.Errorf("the script mechanism needs a script: it is the body of a function " +
				"called as function(colorScheme) with \"light\", \"dark\" or \"auto\"")
		}
		if len(s.Script) > store.SchemeScriptMax {
			return fmt.Errorf("scheme script is %d characters: at most %d are allowed, since the gateway "+
				"serves it to every page this route serves", len(s.Script), store.SchemeScriptMax)
		}
		if store.SchemeSetsAttribute(s.Mechanism) && !schemeTokenOK.MatchString(s.Attribute) {
			return fmt.Errorf("scheme attribute %q is not allowed: letters, digits, - and _ only", s.Attribute)
		}
		if s.Tag != "" && !tagNameOK.MatchString(s.Tag) {
			return fmt.Errorf("scheme tag %q is not allowed: a tag name starts with a letter, then letters, digits and -", s.Tag)
		}
		if s.Button != "" && s.Button != "light" && s.Button != "dark" {
			return fmt.Errorf("button scheme %q is not allowed: allowed schemes are \"\" (follow the visitor), light, dark", s.Button)
		}
		for _, v := range []string{s.Light, s.Dark} {
			if v != "" && !schemeTokenOK.MatchString(v) {
				return fmt.Errorf("scheme value %q is not allowed: letters, digits, - and _ only", v)
			}
		}
		// A storage key is the application's own: the vendors' run to
		// "vuetify:theme", "ng-app.theme", "nuxt-color-mode". Wider than a
		// scheme token, and still nothing that could break out of the HTML
		// attribute it is written into.
		if s.Storage != "" && !storageKeyOK.MatchString(s.Storage) {
			return fmt.Errorf("scheme storage key %q is not allowed: letters, digits, and . : - _ / only", s.Storage)
		}
		for _, v := range []string{s.StorageLight, s.StorageDark, s.StorageAuto} {
			if v != "" && !storageKeyOK.MatchString(v) {
				return fmt.Errorf("scheme stored value %q is not allowed: letters, digits, and . : - _ / only", v)
			}
		}
	}
	btn := r.UI.UserButton
	if btn.Position != "" && !slices.Contains(store.UserButtonPositions, btn.Position) {
		return fmt.Errorf("user button position %q is not allowed: allowed positions are %s",
			btn.Position, strings.Join(store.UserButtonPositions, ", "))
	}
	if btn.Height != 0 && (btn.Height < 16 || btn.Height > 96) {
		return fmt.Errorf("user button height %d is out of range: allowed heights are 16-96 px", btn.Height)
	}
	if btn.PadX < 0 || btn.PadX > 500 || btn.PadY < 0 || btn.PadY > 500 {
		return fmt.Errorf("user button padding is out of range: allowed paddings are 0-500 px")
	}
	switch btn.Shape {
	case "", "round", "square":
	default:
		return fmt.Errorf("user button shape %q is not allowed: allowed shapes are round, square", btn.Shape)
	}
	switch btn.Name {
	case "", "before", "after":
	default:
		return fmt.Errorf("user button name %q is not allowed: allowed values are \"\" (hidden), before, after", btn.Name)
	}
	if ro := r.UI.Roles; ro != nil {
		switch ro.Mechanism {
		case "", "class", "attribute", "meta":
		default:
			return fmt.Errorf("roles mechanism %q is not allowed: allowed mechanisms are class, attribute, meta", ro.Mechanism)
		}
		if ro.Tag != "" && !tagNameOK.MatchString(ro.Tag) {
			return fmt.Errorf("roles tag %q is not allowed: a tag name starts with a letter, then letters, digits and -", ro.Tag)
		}
		if ro.Attribute != "" && !schemeTokenOK.MatchString(ro.Attribute) {
			return fmt.Errorf("roles attribute %q is not allowed: letters, digits, - and _ only", ro.Attribute)
		}
	}
	if ui := r.UI.UserInfo; ui != nil {
		switch ui.Mechanism {
		case "", "attribute", "meta":
		default:
			return fmt.Errorf("user-info mechanism %q is not allowed: allowed mechanisms are attribute, meta", ui.Mechanism)
		}
		if ui.Tag != "" && !tagNameOK.MatchString(ui.Tag) {
			return fmt.Errorf("user-info tag %q is not allowed: a tag name starts with a letter, then letters, digits and -", ui.Tag)
		}
		for field, name := range ui.Fields {
			if !slices.Contains(store.PageUserFields, field) && !store.ValidFieldName(field) {
				return fmt.Errorf("user-info field %q is not allowed: allowed fields are %s",
					field, strings.Join(store.PageUserFields, ", "))
			}
			if name != "" && !schemeTokenOK.MatchString(name) {
				return fmt.Errorf("user-info name %q for %s is not allowed: letters, digits, - and _ only", name, field)
			}
		}
	}
	// The custom CSS travels verbatim inside a <style> tag: a closing tag
	// would break out of it, and 64 KiB is plenty for page tweaks.
	if css := r.UI.CustomCSS; css != "" {
		if strings.Contains(strings.ToLower(css), "</style") {
			return fmt.Errorf("custom css must not contain \"</style\"")
		}
		if len(css) > 64<<10 {
			return fmt.Errorf("custom css is too large (%d bytes): the limit is 64 KiB", len(css))
		}
	}
	// The custom JS travels verbatim inside a <script> tag: same escape rule.
	if js := r.UI.CustomJS; js != "" {
		if strings.Contains(strings.ToLower(js), "</script") {
			return fmt.Errorf("custom js must not contain \"</script\"")
		}
		if len(js) > 64<<10 {
			return fmt.Errorf("custom js is too large (%d bytes): the limit is 64 KiB", len(js))
		}
	}
	// And the language hook, which rides the same way.
	if r.Locales != nil && r.Locales.OnChange != "" {
		if strings.Contains(strings.ToLower(r.Locales.OnChange), "</script") {
			return fmt.Errorf("the locale-change script must not contain \"</script\"")
		}
		if len(r.Locales.OnChange) > 64<<10 {
			return fmt.Errorf("the locale-change script is too large (%d bytes): the limit is 64 KiB",
				len(r.Locales.OnChange))
		}
	}
	return nil
}

// schemeTokenOK bounds the attribute/class/value tokens that travel into the
// injected HTML - validated here, so the fragment never carries free text.
var schemeTokenOK = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// storageKeyOK is a localStorage key as applications write them - dots, colons
// and slashes are common in the wild ("vuetify:theme"); quotes and spaces are
// not, and this lands in an HTML attribute.
var storageKeyOK = regexp.MustCompile(`^[A-Za-z0-9_.:/-]+$`)

// headerNameOK bounds the upstream header names a route may configure.
var headerNameOK = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// claimNameOK bounds the JWT claim names a route may map an attribute onto.
var claimNameOK = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// tagNameOK bounds the page tag a stamp may target (custom elements included).
var tagNameOK = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)

// identityData is the per-request resolution of the caller: what the page
// stamp writes into the HTML and what identity forwarding sends upstream.
// Every token is validated (validateRouteType) or HTML-escaped, so no free
// text ever reaches the page.
type identityData struct {
	// TagsOfRole maps a role NAME to its catalogue tags, for expressions that
	// narrow what they forward. Read from a short-lived cache: a catalogue
	// changes far less often than requests arrive.
	TagsOfRole map[string][]string
	UserID     string
	Username   string
	Fullname   string
	Email      string
	Timezone   string
	Locale     string
	TenantID   string
	Tenant     string
	// Group is the session's chosen group (exclusive mode, RBAC-03), by name.
	Group string
	// Fields are the installation's own facts about this person (store's
	// userfields.go). Carried beside the built-ins because to everything
	// downstream they are the same kind of thing: one more fact about the
	// caller, selected and renamed the same way.
	Fields map[string]string
	Roles  []string
	// Memberships are the organisations this account is an ENABLED member of,
	// whichever one the session is in: an access rule naming an organisation
	// admits its members (store.Caller). Read with the rest, so the rule does
	// not ask the database again on every request.
	Memberships []string
}

// sessionIdentity resolves the caller for per-request injections and
// forwarding; ok is false without a completed session. A simulated identity
// (simulate.go) replaces the session wholesale. Who the session is comes from
// memory between writes (identitycache.go).
func (rt *Router) sessionIdentity(req *http.Request) (identityData, bool) {
	if d, ok := simulatedIdentity(req.Context()); ok {
		spanPerson(req.Context(), d)
		return d, true
	}
	// A scheduled call carries no session and no account: its identity is
	// posed in the context by the scheduler, in process (scheduled.go).
	if d, ok := rt.scheduledIdentity(req.Context()); ok {
		spanPerson(req.Context(), d)
		return d, true
	}
	sess, err := rt.sm.Resolve(req.Context(), req)
	if err != nil || sess.Pending != "" {
		return identityData{}, false
	}
	now := time.Now()
	key := identityKey{user: sess.UserID, tenant: sess.TenantID, group: sess.GroupID}
	e, ok := rt.cachedIdentity(key, now)
	if !ok {
		epoch := rt.identityEpoch.Load()
		if e, ok = rt.readIdentity(req, sess); !ok {
			return identityData{}, false
		}
		rt.rememberIdentity(key, e, epoch, now)
	}
	// Outside its validity window counts as disabled, and for the same reason:
	// the decision was taken in advance rather than on the day (SEC-07). A
	// clock question, so it is asked on every request, remembered or not.
	if !e.owner.Enabled || !e.owner.ValidAt(now) {
		return identityData{}, false
	}
	spanPerson(req.Context(), e.data)
	return e.data.clone(), true
}

// readIdentity is what sessionIdentity used to do on every request: the
// account, the organisation, the roles the session's group mode grants.
func (rt *Router) readIdentity(req *http.Request, sess store.Session) (identityEntry, bool) {
	u, err := rt.st.GetUserByID(req.Context(), sess.UserID)
	if err != nil {
		return identityEntry{}, false
	}
	d := identityData{UserID: u.ID, Username: u.Username, Fullname: u.Fullname,
		Email: u.Email, Timezone: u.Timezone, Locale: u.Locale, Fields: u.Fields}
	if ms, err := rt.st.ListUserTenants(req.Context(), u.ID); err == nil {
		for _, m := range ms {
			if m.Enabled {
				d.Memberships = append(d.Memberships, m.TenantID)
			}
		}
	}
	tenantID := sess.TenantID
	if tenantID == "" {
		// The session was opened BEFORE this account joined an organisation.
		// Sign-in adopts the only membership when there is exactly one; this
		// does the same afterwards, so being added to an organisation takes
		// effect without asking the person to sign out and back in.
		//
		// Roles are held IN an organisation (RBAC-06), so without one the
		// caller carries none - which is how a group full of roles could look
		// like it did nothing at all.
		if len(d.Memberships) == 1 {
			tenantID = d.Memberships[0]
		}
	}
	if tenantID != "" {
		d.TenantID = tenantID
		if t, err := rt.st.GetTenant(req.Context(), tenantID); err == nil {
			d.Tenant = t.Name
		}
		if sess.GroupID != "" {
			if g, err := rt.st.GetGroup(req.Context(), sess.GroupID); err == nil {
				d.Group = g.Name
			}
		}
		// SessionRoleNames applies the group mode (RBAC-03): cumulative =
		// every group, exclusive = the session's chosen group only.
		if names, err := rt.st.SessionRoleNames(req.Context(), u.ID, tenantID, sess.GroupID); err == nil {
			for _, n := range names {
				if schemeTokenOK.MatchString(n) {
					d.Roles = append(d.Roles, n)
				}
			}
		}
		if len(d.Roles) > 0 {
			d.TagsOfRole = rt.roleTags(req)
		}
	}
	owner := store.User{ID: u.ID, Enabled: u.Enabled, ValidFrom: u.ValidFrom, ValidUntil: u.ValidUntil}
	return identityEntry{data: d, owner: owner}, true
}

// roleTags returns the catalogue as role name -> tags, from a short-lived
// cache.
func (rt *Router) roleTags(req *http.Request) map[string][]string {
	rt.tagsMu.Lock()
	defer rt.tagsMu.Unlock()
	if time.Since(rt.tagsReadAt) < 5*time.Second && rt.tagsCache != nil {
		return rt.tagsCache
	}
	roles, err := rt.st.ListRoles(req.Context())
	if err != nil {
		// Keeping the previous table beats forwarding nothing: a database
		// hiccup must not silently strip every role from every request.
		return rt.tagsCache
	}
	byRole := make(map[string][]string, len(roles))
	for _, r := range roles {
		if len(r.Tags) > 0 {
			byRole[r.Name] = r.Tags
		}
	}
	rt.tagsCache, rt.tagsReadAt = byRole, time.Now()
	return byRole
}

// hasIdentity is the cheap gate for the page stamp: a completed session exists.
// Resolve is cached, so anonymous requests are turned away before the response
// body is ever buffered.
func (rt *Router) hasIdentity(req *http.Request) bool {
	if _, ok := simulatedIdentity(req.Context()); ok {
		return true
	}
	sess, err := rt.sm.Resolve(req.Context(), req)
	return err == nil && sess.Pending == ""
}

// pageStamp applies a UI route's roles/user-info to the served HTML entirely
// SERVER-SIDE: roles as a class or attribute on the target tag (default body)
// or a meta; each selected user field as an attribute or a meta. The body is
// returned unchanged when there is no completed session. Everything embedded
// is validated config or HTML-escaped identity, so nothing can break out.
func (rt *Router) pageStamp(r store.Route, localeCodes []string, res *http.Response, body []byte) []byte {
	ui := r.UI
	d, ok := rt.sessionIdentity(res.Request)
	if !ok {
		return body
	}
	// From here the bytes belong to ONE person, whatever the application said
	// about caching them: a CDN or a shared browser would otherwise hand this
	// page, name and roles included, to the next visitor. An anonymous request
	// returned above keeps the upstream's caching untouched.
	filtering.Personal(res.Header)
	var metas []byte
	if ui.Roles != nil && ui.Roles.Enabled {
		tag := orDefault(ui.Roles.Tag, "body")
		joined := strings.Join(d.Roles, " ")
		switch orDefault(ui.Roles.Mechanism, "class") {
		case "class":
			body = stampClass(body, tag, d.Roles)
		case "attribute":
			body = stampAttr(body, tag, orDefault(ui.Roles.Attribute, "data-roles"), joined)
		case "meta":
			metas = append(metas, metaTag(orDefault(ui.Roles.Attribute, "meerkat-roles"), joined)...)
		}
	}
	if ui.UserInfo != nil && ui.UserInfo.Enabled {
		// The language the page is being served in, resolved against THIS
		// route's offer - what an application needs to render itself, and the
		// one thing here that is not a fact about the person alone.
		if len(localeCodes) > 0 {
			d.Locale = resolveLocale(res.Request, localeCodes)
		}
		tag := orDefault(ui.UserInfo.Tag, "body")
		asMeta := orDefault(ui.UserInfo.Mechanism, "attribute") == "meta"
		for field, name := range ui.UserInfo.Fields {
			value := userFieldValue(d, field)
			if value == "" {
				continue
			}
			attr := orDefault(name, field)
			if asMeta {
				metas = append(metas, metaTag(attr, value)...)
			} else {
				body = stampAttr(body, tag, attr, value)
			}
		}
	}
	if len(metas) > 0 {
		body = insertAfterHead(body, metas)
	}
	return body
}

// userFieldValue maps a PageUserFields key to its resolved value.
func userFieldValue(d identityData, field string) string {
	switch field {
	case "username":
		return d.Username
	case "userid":
		return d.UserID
	case "fullname":
		return d.Fullname
	case "email":
		return d.Email
	case "tenant":
		return d.Tenant
	case "tenantid":
		return d.TenantID
	case "timezone":
		return d.Timezone
	case "locale":
		// The language this page was SERVED in, not the raw preference: an
		// application reading it at bootstrap wants to know what to render,
		// and someone who never chose still gets a page in some language.
		// Resolved by the caller against the route's own offer.
		return d.Locale
	}
	// A custom field, last: a built-in name always wins, and the definition
	// refuses a custom field that would shadow one - so this is reached only
	// by a name the installation added.
	return d.Fields[field]
}

// openTagRe matches the FIRST opening <tag ...> in the document (case-insensitive,
// attributes may span lines). Group 1 = "<tag", group 2 = the attributes (with
// their leading space) or empty; the closing ">" follows. A trailing word
// boundary keeps <body> from matching <bodyfoo>.
func openTagRe(tag string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(<` + regexp.QuoteMeta(tag) + `)((?:\s[^>]*)?)>`)
}

// stampClass merges roles into the target tag's class attribute (server-side
// equivalent of classList.add): appends to an existing class="...", or adds one.
func stampClass(body []byte, tag string, roles []string) []byte {
	if len(roles) == 0 {
		return body
	}
	m := openTagRe(tag).FindSubmatchIndex(body)
	if m == nil {
		return body
	}
	attrs := string(body[m[4]:m[5]])
	return spliceOpenTag(body, m, addClassTokens(attrs, roles))
}

// stampAttr sets name="value" on the target tag's opening tag (replacing an
// existing one, or appending). value is attribute-escaped.
func stampAttr(body []byte, tag, name, value string) []byte {
	m := openTagRe(tag).FindSubmatchIndex(body)
	if m == nil {
		return body
	}
	attrs := string(body[m[4]:m[5]])
	return spliceOpenTag(body, m, setAttrToken(attrs, name, value))
}

// spliceOpenTag rebuilds the matched opening tag with newAttrs in place of the
// original attribute list. m is openTagRe's submatch index set.
func spliceOpenTag(body []byte, m []int, newAttrs string) []byte {
	out := make([]byte, 0, len(body)+len(newAttrs))
	out = append(out, body[:m[3]]...) // up to and including "<tag"
	out = append(out, newAttrs...)
	out = append(out, '>')
	out = append(out, body[m[1]:]...) // after the original ">"
	return out
}

var classAttrRe = regexp.MustCompile(`(?i)(\sclass\s*=\s*")([^"]*)(")`)

// addClassTokens appends roles to a double-quoted class="..." inside attrs
// (skipping tokens already present), or adds a class attribute when none.
func addClassTokens(attrs string, roles []string) string {
	if classAttrRe.MatchString(attrs) {
		return classAttrRe.ReplaceAllStringFunc(attrs, func(s string) string {
			g := classAttrRe.FindStringSubmatch(s)
			existing := strings.Fields(g[2])
			for _, role := range roles {
				if !slices.Contains(existing, role) {
					existing = append(existing, role)
				}
			}
			return g[1] + strings.Join(existing, " ") + g[3]
		})
	}
	return attrs + ` class="` + html.EscapeString(strings.Join(roles, " ")) + `"`
}

// setAttrToken replaces a double-quoted name="..." inside attrs, or appends one.
func setAttrToken(attrs, name, value string) string {
	esc := html.EscapeString(value)
	re := regexp.MustCompile(`(?i)(\s` + regexp.QuoteMeta(name) + `\s*=\s*")[^"]*(")`)
	if re.MatchString(attrs) {
		return re.ReplaceAllStringFunc(attrs, func(s string) string {
			g := re.FindStringSubmatch(s)
			return g[1] + esc + g[2]
		})
	}
	return attrs + " " + name + `="` + esc + `"`
}

// metaTag builds a <meta name="..." content="..."> (both attribute-escaped).
func metaTag(name, content string) []byte {
	return []byte(`<meta name="` + html.EscapeString(name) + `" content="` + html.EscapeString(content) + `">`)
}

var headOpenRe = regexp.MustCompile(`(?i)<head[^>]*>`)

// insertAfterHead inserts frag right after the opening <head>, or at the very
// start when there is no <head>.
func insertAfterHead(body, frag []byte) []byte {
	loc := headOpenRe.FindIndex(body)
	out := make([]byte, 0, len(body)+len(frag))
	if loc == nil {
		out = append(out, frag...)
		return append(out, body...)
	}
	out = append(out, body[:loc[1]]...)
	out = append(out, frag...)
	return append(out, body[loc[1]:]...)
}

// localeForwardFilter carries the resolved locale to the upstream:
// Accept-Language ALWAYS goes, rewritten with the choice first; the route's
// extra mechanisms ride on top - a custom header, a query parameter (an
// API's natural shape), a path segment (UI only, Angular-style /fr/ builds).
func localeForwardFilter(codes []string, lc store.LocalesConfig, prefixes []string) routing.RequestFilter {
	return func(pr *httputil.ProxyRequest) {
		loc := resolveLocale(pr.In, codes)
		if loc == "" {
			// Nobody expressed a language and the route declares none: there
			// is nothing to promote, and an empty Accept-Language would be
			// worse than the one the browser sent.
			return
		}
		pr.Out.Header.Set("Accept-Language", promoteLocale(pr.In.Header.Get("Accept-Language"), loc))
		if len(codes) == 0 {
			// The three below put the language WHERE THE APPLICATION EXPECTS
			// IT, which needs the list of languages that application serves.
			// With none declared the header is all there is to carry.
			return
		}
		switch lc.Mode() {
		case store.LocalePath:
			pr.Out.URL.Path = insertLocale(pr.Out.URL.Path, loc, codes, prefixes)
			pr.Out.URL.RawPath = ""
		case store.LocaleQuery:
			q := pr.Out.URL.Query()
			q.Set(orDefault(lc.Param, "lg"), loc)
			pr.Out.URL.RawQuery = q.Encode()
		case store.LocaleCustom:
			pr.Out.Header.Set(lc.Header, loc)
		}
	}
}

// insertLocale puts the language where the APPLICATION expects it: after what
// the route matched on. A request the predicate took as /app/route reaches its
// upstream as /app/fr/route, because /app is the application's own root and a
// locale in front of it would be a path the upstream never serves. With a
// strip-prefix the head is already gone by the time this runs, so the same
// rule lands the locale in front of what remains - one rule, and the filter
// never needs to know which other filters ran.
//
// A locale ALREADY sitting at that spot is left alone, whatever put it there:
// a portal driving the language of a framed page, or an application whose own
// links carry it. That segment belongs to someone else, and overriding it
// would mean two languages in one path.
func insertLocale(path, loc string, codes, prefixes []string) string {
	at := localeAt(path, prefixes)
	if localeSegment(path, codes, prefixes) != "" {
		return path
	}
	rest := strings.TrimPrefix(path[at:], "/")
	out := path[:at] + "/" + loc
	if rest != "" {
		out += "/" + rest
	} else if strings.HasSuffix(path, "/") {
		// /app/ asked for the directory, and /app/fr is a different resource
		// from /app/fr/ for more upstreams than one would like.
		out += "/"
	}
	return out
}

// langCookie is where the visitor's language choice lives: written by the flow
// pages and by the injected button, read here.
const langCookie = "MEERKAT_LANG"

// localeAt is where the language segment sits: right after the longest literal
// prefix the route matched on, or at the front once a strip-prefix removed it.
func localeAt(path string, prefixes []string) int {
	at := 0
	for _, p := range prefixes {
		if len(p) > at && (path == p || strings.HasPrefix(path, p+"/")) {
			at = len(p)
		}
	}
	return at
}

// localeSegment returns the language THIS path carries at that spot, or "".
func localeSegment(path string, codes, prefixes []string) string {
	rest := strings.TrimPrefix(path[localeAt(path, prefixes):], "/")
	first, _, _ := strings.Cut(rest, "/")
	if first != "" && containsFold(codes, first) {
		return first
	}
	return ""
}

// redirectToLocale keeps the URL in step with the PERSON. A language in the
// path is not a decision, it is where the application lives: someone who has
// chosen Vietnamese and opens a link a colleague sent in French should read
// it in Vietnamese, and the address bar should say so rather than lie.
//
// The target is what THIS ROUTE resolves for them, not the raw preference: a
// route may serve fewer languages than the application offers, and
// resolveLocale already falls back (cookie, then Accept-Language, then the
// first the route serves). So opening a French link with a Vietnamese
// preference on a route that speaks neither leaves you where you were.
//
// Navigations only, and only GET or HEAD: an asset fetched by a page that was
// itself just corrected has nothing to correct, and redirecting a POST would
// drop what it carries.
// stripsPrefix reports whether the route republishes its application under a
// prefix the application does not know about.
func stripsPrefix(r store.Route) bool {
	for _, f := range r.Filters {
		if f.Type == "strip-prefix" {
			return true
		}
	}
	return false
}

// redirectToMountSlash answers the bare mount path with the same path and its
// final slash, so the pages of an application published under a prefix resolve
// their relative links under it. Everything else passes through, the target
// included - see the call site for why this is a redirect and not a rewrite.
//
// 308 rather than 301: it keeps the method, so the rare POST to an entry point
// stays a POST, and browsers remember it as permanent, which it is for as long
// as the application is published there.
func redirectToMountSlash(mount string, next http.Handler) http.Handler {
	mount = strings.TrimRight(mount, "/")
	if mount == "" {
		// Mounted at the root, or matched by host alone: no missing slash.
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != mount {
			next.ServeHTTP(w, r)
			return
		}
		target := mount + "/"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	})
}

func redirectToLocale(next http.Handler, codes, prefixes []string, inPath bool, param string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) || !isNavigation(r) {
			next.ServeHTTP(w, r)
			return
		}
		want := resolveLocale(r, codes)
		u := *r.URL
		changed := false
		// The path segment, where the language lives in the URL. Guarded by the
		// flag and not by the prefixes: a route matching /** has no literal
		// prefix, and the language then sits at the very front.
		if inPath {
			seg := localeSegment(r.URL.Path, codes, prefixes)
			switch {
			case seg == "":
				// MISSING, and the browser has to be told. Inserting it only on
				// the way upstream was enough until an application built per
				// locale stamped <base href="/app/en/">: every link it then
				// draws lives under /app/en/ while the address bar still says
				// /app/, and its router, asked to reconcile the two, resolves
				// its default route against the base and lands on
				// /app/en/app. It looks like the gateway doubled a segment.
				// It is the application, doing exactly what it was built to do
				// with an address that was hidden from it.
				if next := insertLocale(r.URL.Path, want, codes, prefixes); next != r.URL.Path {
					u.Path, u.RawPath = next, ""
					changed = true
				}
			case !strings.EqualFold(seg, want):
				at := localeAt(r.URL.Path, prefixes)
				_, tail, _ := strings.Cut(strings.TrimPrefix(r.URL.Path[at:], "/"), "/")
				u.Path = r.URL.Path[:at] + "/" + want
				if tail != "" {
					u.Path += "/" + tail
				}
				u.RawPath = ""
				changed = true
			}
		}
		// And the query parameter, for the same reason: the gateway already
		// overwrites it on the way upstream, so leaving the old one in the
		// address bar means the URL names a language the page is not in.
		if param != "" {
			if q := r.URL.Query(); q.Has(param) && !strings.EqualFold(q.Get(param), want) {
				q.Set(param, want)
				u.RawQuery = q.Encode()
				changed = true
			}
		}
		if !changed {
			next.ServeHTTP(w, r)
			return
		}
		// Temporary: the right language for the next visitor is not this one.
		http.Redirect(w, r, u.RequestURI(), http.StatusFound)
	})
}

// isNavigation: a document the browser is going TO, as opposed to something a
// page is fetching. Sec-Fetch-Mode answers it outright where it is sent; the
// Accept header is the old way of asking the same question.
func isNavigation(r *http.Request) bool {
	if m := r.Header.Get("Sec-Fetch-Mode"); m != "" {
		return m == "navigate"
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func containsFold(list []string, v string) bool {
	for _, s := range list {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

// promoteLocale rewrites an Accept-Language value with the resolved locale
// FIRST (implicit weight 1), keeping the caller's other preferences behind
// (their own q-values intact); a duplicate of the choice is dropped.
func promoteLocale(orig, loc string) string {
	out := []string{loc}
	for _, part := range strings.Split(orig, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		tag, _, _ := strings.Cut(p, ";")
		if strings.EqualFold(strings.TrimSpace(tag), loc) {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

// resolveLocale picks the request's locale among the route's codes: the
// MEERKAT_LANG cookie first, then the best Accept-Language match (exact,
// then same base language), then the first code.
func resolveLocale(r *http.Request, codes []string) string {
	if len(codes) == 0 {
		// A route that declares no language is not choosing between any: it
		// carries the person's own, which is their stored choice when there is
		// one and what their browser asks for otherwise. This is the case of
		// every API route, where Accept-Language is the only transport.
		if c, err := r.Cookie(langCookie); err == nil && c.Value != "" {
			return c.Value
		}
		for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
			tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
			if tag != "" && tag != "*" {
				return tag
			}
		}
		return ""
	}
	pick := func(tag string) string {
		for _, code := range codes {
			if strings.EqualFold(code, tag) {
				return code
			}
		}
		base, _, _ := strings.Cut(tag, "-")
		for _, code := range codes {
			cb, _, _ := strings.Cut(code, "-")
			if strings.EqualFold(cb, base) {
				return code
			}
		}
		return ""
	}
	if c, err := r.Cookie(langCookie); err == nil && c.Value != "" {
		if code := pick(c.Value); code != "" {
			return code
		}
	}
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if tag == "" || tag == "*" {
			continue
		}
		if code := pick(tag); code != "" {
			return code
		}
	}
	return codes[0]
}

// accessGate wraps next with a unified access rule (RBAC-06/07): an empty rule
// passes through, everything else requires a valid session (401/redirect via
// requireSession) and a caller satisfying the rule.
//
// Whatever the level, the upstream still applies its own rules afterwards -
// Meerkat gates IN ADDITION to the service, never instead of it.
//
// isUI decides what a REFUSAL looks like, and that is most of the point of the
// tenant levels: on an application, being refused for lack of an organisation
// is not an error, it is a step missing. A person whose account is confirmed
// but not yet granted anything is sent to the waiting room that explains it; a
// member of several organisations who is active in the wrong one is sent to
// choose. An API answers 403 and names what is missing - there is nobody to
// read a page.
func (rt *Router) accessGate(a store.Access, isUI bool, next http.Handler) http.Handler {
	if a.Empty() {
		return next
	}
	return requireSession(rt.sm, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// An internal spec read (admin API docs) was authorized on the control
		// plane; the route's own rules gate CALLS, not reading its contract.
		if isSpecRead(req.Context()) {
			next.ServeHTTP(w, req)
			return
		}
		d, ok := rt.sessionIdentity(req)
		caller := rt.caller(req, d, ok)
		if ok && a.Grants(caller) {
			next.ServeHTTP(w, req)
			return
		}
		// The SAME refusal as the one the selection produces (refuse): one
		// decision, in one place, or the day they diverge a person is told two
		// different things about the same rule.
		rt.refuse(w, req, a, isUI, ok, caller, d.UserID)
	}))
}

// refuse answers the caller the selection turned away. It is the tail of what
// used to be two wrappers - requireSession and accessGate - moved here because
// the decision moved: a rule that takes part in choosing cannot also be the
// handler that answers when nothing was chosen.
//
// The order is the order of what a person can do about it: sign in, finish the
// login flow, choose another organisation, wait to be granted something, and
// only then a plain refusal that names what was missing.
func (rt *Router) refuse(w http.ResponseWriter, req *http.Request, a store.Access, isUI, ok bool, who store.Caller, userID string) {
	if !ok {
		// No usable session. A browser going TO a page is sent to sign in with
		// a way back; anything else gets a plain 401 - there is nobody to read
		// a login page at the end of a fetch.
		if sess, err := rt.sm.Resolve(req.Context(), req); err == nil && sess.Pending != "" {
			// AUTH-05: the login flow is unfinished, and every navigation goes
			// to the step that finishes it.
			if isNavigation(req) {
				http.Redirect(w, req, "/"+sess.Pending+"?next="+url.QueryEscape(req.URL.RequestURI()), http.StatusSeeOther)
				return
			}
			http.Error(w, "login flow incomplete", http.StatusUnauthorized)
			return
		}
		if isNavigation(req) {
			http.Redirect(w, req, "/login?next="+url.QueryEscape(req.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		w.Header().Set("WWW-Authenticate", "Session")
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	// Someone who IS signed in and is turned away anyway: a WARN, once, for
	// both outcomes below. Not an error - the gateway did its job - but the
	// operator has to be able to see that people are hitting a rule, which no
	// redirection to the organisation chooser will ever tell them. The cause is
	// the same word the page shows, so a support call and a log line say the
	// same thing.
	slog.Warn("access refused", "path", req.URL.Path, "method", req.Method,
		"user", who.Username, "tenant", who.TenantID, "why", refusalCode(a, who))

	// A refused SIMULATED identity during a UI test must not lock the
	// developer out (a bare 403 has no developer bar): explain and offer the
	// exit instead.
	if m, simOn := simulationMeta(req.Context()); simOn && m.Via == "ui-test" {
		uiSimRefusalPage(w, req)
		return
	}
	// A BROWSER on a UI route gets a page it can act on. Anything else - a
	// fetch, an API client, a scheduled call - gets the refusal itself: the
	// same reason a signed-out fetch gets a 401 rather than the login page.
	// Redirecting a machine to a page it cannot read turns "you may not" into
	// "303 See Other", which is what the scheduler recorded before this.
	if isUI && isNavigation(req) {
		if offer := rt.switchWouldHelp(req.Context(), a, userID, a.Switchable(who)); len(offer) > 0 {
			// WHY, carried to the page: an organisation chooser that reappears
			// saying nothing is the reason someone switches back and forth
			// wondering what they did wrong.
			http.Redirect(w, req, "/select-tenant?next="+url.QueryEscape(req.URL.RequestURI())+
				"&why="+refusalCode(a, who), http.StatusSeeOther)
			return
		}
		// No organisation at all and none to switch to: the waiting room says
		// who to ask, which a 403 does not.
		if len(who.Memberships) == 0 && needsTenant(a) {
			http.Redirect(w, req, "/account-pending", http.StatusSeeOther)
			return
		}
		// Nothing to switch to and nothing to wait for: still a browser, and
		// still someone who did nothing wrong. The page says which rule turned
		// them away and what this session CAN open - the same answer the other
		// two refusals give, instead of a line of text on a blank page.
		http.Redirect(w, req, "/refused?next="+url.QueryEscape(req.URL.RequestURI())+
			"&why="+refusalCode(a, who), http.StatusSeeOther)
		return
	}
	http.Error(w, refusalReason(a, who), http.StatusForbidden)
}

// answerOrRefuse lets a route answer, with one arbitration when a refusal is
// pending: an ERROR from the route that answers means nobody had anything for
// this caller, and the refusal kept aside is then the truthful answer. See
// cover.go for why, and for what is held (the status, never the body).
//
// With no refusal pending it is the plain call it always was.
func (rt *Router) answerOrRefuse(ww *watched, hit *http.Request, r *compiledRoute, start time.Time,
	cand *compiledRoute, candReq *http.Request, candOK bool, candWho store.Caller, candUserID string) bool {
	if cand == nil {
		r.handler.ServeHTTP(ww, hit)
		observed(r, hit, ww, start)
		return true
	}
	cw := &coveringWriter{ResponseWriter: ww}
	r.handler.ServeHTTP(cw, hit)
	if !cw.covered {
		observed(r, hit, ww, start)
		return true
	}
	// Its headers went into the map before it chose its status: written now,
	// they would dress the refusal in the other route's answer.
	clear(ww.Header())
	spanEvent(candReq.Context(), "access_refused", tracing.String("meerkat.route", cand.name))
	slog.Info("refusal restored over an error from the next route",
		"refused_by", cand.name, "answered_by", r.name, "path", hit.URL.Path)
	rt.refuse(ww, candReq, cand.access, cand.isUI, candOK, candWho, candUserID)
	observed(cand, candReq, ww, start)
	return true
}

// switchWouldHelp keeps only the organisations where this caller would
// actually get through.
//
// Access.Switchable answers about the LEVEL alone - it is a pure function on
// the rule and cannot know what somebody holds elsewhere. So a rule that names
// two organisations AND asks for a role used to send a member of both to the
// chooser, refuse them whichever they picked, and offer the other one again:
// alice bouncing between acme and globex, told nothing, reaching nothing. The
// offer has to be true, or it is a door painted on a wall.
//
// A refusal is not the hot path, so the extra read costs nothing where it
// matters, and it is only taken when the rule asks for roles at all.
func (rt *Router) switchWouldHelp(ctx context.Context, a store.Access, userID string, offer []string) []string {
	if len(a.Roles) == 0 || userID == "" || len(offer) == 0 {
		return offer
	}
	var kept []string
	for _, id := range offer {
		roles, err := rt.st.RolesReachableIn(ctx, userID, id)
		if err != nil {
			continue
		}
		if slices.ContainsFunc(roles, func(r string) bool { return slices.Contains(a.Roles, r) }) {
			kept = append(kept, id)
		}
	}
	return kept
}

// needsTenant reports whether the rule asks for an organisation at all.
func needsTenant(a store.Access) bool {
	return a.Level == store.AccessTenant || a.Level == store.AccessTenants || len(a.Roles) > 0
}

// refusalCode is refusalReason for a PAGE: the same analysis, reduced to a
// word the flow pages translate. An API caller reads a sentence in the body; a
// person gets a screen, and until now that screen was the organisation chooser
// reappearing with nothing on it - the one thing they could not act on.
func refusalCode(a store.Access, c store.Caller) string {
	switch {
	case a.Level == store.AccessTenants && !slices.Contains(a.Tenants, c.TenantID):
		return "tenant" // this page belongs to another organisation
	case len(a.Roles) > 0:
		return "roles" // right organisation, none of the roles it asks for
	case len(a.Users) > 0:
		return "user" // a named list, and this caller is not on it
	}
	return "access"
}

// refusalReason names what is missing rather than saying "forbidden": the
// caller can act on "you have no organisation selected", not on a bare 403.
func refusalReason(a store.Access, c store.Caller) string {
	switch {
	case a.Level == store.AccessDeny:
		return "forbidden: this endpoint is closed"
	case needsTenant(a) && c.TenantID == "" && len(c.Memberships) == 0:
		return "forbidden: your account belongs to no organisation yet"
	case needsTenant(a) && c.TenantID == "":
		return "forbidden: no organisation is active on this session - choose one first"
	case a.Level == store.AccessTenants && !slices.Contains(a.Tenants, c.TenantID):
		return "forbidden: this endpoint is reserved to another organisation"
	case len(a.Roles) > 0:
		return "forbidden: this endpoint requires one of the roles " + strings.Join(a.Roles, ", ")
	}
	return "forbidden: you may not call this endpoint"
}

// caller assembles what a rule is evaluated against. Memberships are read only
// when the rule could care (a switch offer), never on the hot path of a public
// or plain-authenticated route.
func (rt *Router) caller(_ *http.Request, d identityData, ok bool) store.Caller {
	if !ok {
		return store.Caller{}
	}
	return store.Caller{Authenticated: true, Username: d.Username, TenantID: d.TenantID,
		Roles: d.Roles, Memberships: d.Memberships}
}

// endpointGuard enforces per-operation security (RBAC-07) inside an API route.
// Every override path is precompiled once at reload; per request the guard
// undoes the route's strip-prefix to recover the OpenAPI coordinate, then
// applies the first matching override, or the route-wide default when none
// matches. Operations with neither fall through to the route's own auth.
func (rt *Router) endpointGuard(sec store.EndpointSecurity, routeAccess store.Access, isUI bool, filters []routing.Spec, next http.Handler) (http.Handler, error) {
	type compiledEP struct {
		method string
		path   routing.CompiledPath
		gate   http.Handler
	}
	// The route's base Access is the default for any operation with no override
	// (an empty Access passes through, delegating to the upstream) - unless
	// the route closes what it does not list, and then nobody passes.
	fallback := routeAccess
	if sec.DenyUnlisted {
		fallback = store.Access{Level: store.AccessDeny}
	}
	eps := make([]compiledEP, 0, len(sec.Endpoints))
	for i, e := range sec.Endpoints {
		cp, err := routing.CompilePath(e.Path)
		if err != nil {
			return nil, fmt.Errorf("endpoint %d (%s %s): %w", i, e.Method, e.Path, err)
		}
		// An entry that poses no access rule (a bound alone) keeps the
		// route's: matching it must not be a way around the route's rule.
		access := e.Access
		if e.Inherit {
			access = fallback
		}
		// The operation's own bounds (QUOTA-05), OUTSIDE its access gate for
		// the same reason the route's are outside the route's: a bound is
		// there to refuse before work happens, and somebody hammering with
		// credentials that do not work is exactly who it is for.
		guarded := rt.accessGate(access, isUI, next)
		free, identified := compileLimits(e.Limits)
		guarded = rt.rateGate(identified, guarded)
		guarded = rt.rateGate(free, guarded)
		eps = append(eps, compiledEP{method: strings.ToUpper(e.Method), path: cp, gate: guarded})
	}
	strip := stripPrefixCount(filters)
	routeGate := rt.accessGate(fallback, isUI, next)

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		specPath := routing.StripSegments(req.URL.Path, strip)
		for i := range eps {
			if (eps[i].method == "*" || eps[i].method == req.Method) && eps[i].path.Match(specPath) {
				eps[i].gate.ServeHTTP(w, req)
				return
			}
		}
		routeGate.ServeHTTP(w, req)
	}), nil
}

// KeptPrefix is the part of a route's own path prefix that a request still
// carries when the guard compares it: the prefix it matches on, minus the
// segments its strip-prefix filters remove.
//
// It is the missing half of an operation's coordinate. The guard compares the
// request path minus what the route strips, so an application published at
// /otel-demo and mounted there itself - stripping nothing - has its operations
// at /otel-demo/..., while one published at /demo and stripping that segment
// has them at /... Reading the spec's paths as the coordinate is right only in
// the second case, and the first is how a rule comes to name a path no request
// ever has.
//
// Exported because the console lists the same operations and writes rules in
// the same coordinates: two spellings of one operation is exactly the bug this
// closes.
func KeptPrefix(r store.Route) string {
	return strings.TrimSuffix(routing.StripSegments(routeMatchPrefix(r), stripPrefixCount(r.Filters)), "/")
}

// stripPrefixCount sums the leading segments the route's strip-prefix filters
// remove, so the endpoint guard can map an inbound path back to the OpenAPI
// coordinate its operation paths live in. A strip-prefix with no explicit
// "parts" uses the schema default of 1.
func stripPrefixCount(filters []routing.Spec) int {
	n := 0
	for _, f := range filters {
		if f.Type != "strip-prefix" {
			continue
		}
		parts := 1
		if v, ok := f.Args["parts"]; ok {
			switch p := v.(type) {
			case float64:
				parts = int(p)
			case int:
				parts = p
			case int64:
				parts = int(p)
			}
		}
		n += parts
	}
	return n
}

// gatewayIssuer is the "iss" claim of identity tokens: this gateway is the
// authority the upstream trusts. A configurable issuer arrives with the signing
// identity (signed-jwt, Lot 2).
const gatewayIssuer = "meerkat"

// identityForwardFilter sends the signed-in caller upstream, per the mechanism.
// Only the SELECTED attributes travel, each optionally renamed. Inbound values
// are purged first so a client can never spoof them: the header transport drops
// every catalogue header (and each mapped target); the jwt transport drops the
// Authorization it is about to write.
func (rt *Router) identityForwardFilter(cfg store.IdentityForward, routeName string) routing.RequestFilter {
	if cfg.Mechanism == "jwt" || cfg.Mechanism == "signed-jwt" {
		signed := cfg.Mechanism == "signed-jwt"
		alg := cfg.Algorithm
		if alg == "" {
			alg = signing.ES256
		}
		return func(pr *httputil.ProxyRequest) {
			pr.Out.Header.Del("Authorization")
			// The gateway's own detail, when asked for (tracing.Step): the
			// whole handover, then who the caller is and the signature inside
			// it - the two halves that can cost, and whose split is the answer
			// to "where did the gateway's time go".
			ctx, endForward := tracing.Step(pr.In.Context(), "identity forward",
				tracing.String("meerkat.identity.mechanism", cfg.Mechanism),
				tracing.Int64("meerkat.identity.attributes", int64(len(cfg.Attributes))))
			var ferr error
			defer func() { endForward(ferr) }()

			rctx, endResolve := tracing.Step(ctx, "identity resolve")
			d, ok := rt.sessionIdentity(pr.In.WithContext(rctx))
			endResolve(nil)
			if !ok {
				return
			}
			claims := identityClaims(cfg, routeName, d)
			var tok string
			if signed {
				set := rt.currentSigning()
				if set == nil {
					return
				}
				_, endSign := tracing.Step(ctx, "identity sign", tracing.String("meerkat.identity.algorithm", alg))
				tok, ferr = set.SignJWT(alg, claims)
				endSign(ferr)
			} else {
				tok, ferr = mintUnsignedJWT(claims)
			}
			if ferr != nil {
				return
			}
			pr.Out.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	return func(pr *httputil.ProxyRequest) {
		for _, f := range store.IdentityFields {
			pr.Out.Header.Del(f)
		}
		for _, a := range cfg.Attributes {
			if a.As != "" {
				pr.Out.Header.Del(a.As)
			}
		}
		// Purged like the rest: an inbound one is a caller claiming to be
		// someone, and these are the headers applications trust most.
		for _, name := range remoteUserNames {
			pr.Out.Header.Del(name)
		}
		ctx, endForward := tracing.Step(pr.In.Context(), "identity forward",
			tracing.String("meerkat.identity.mechanism", "headers"),
			tracing.Int64("meerkat.identity.attributes", int64(len(cfg.Attributes))))
		defer endForward(nil)
		d, ok := rt.sessionIdentity(pr.In.WithContext(ctx))
		if !ok {
			return
		}
		for _, h := range identityHeaderPairs(cfg, d) {
			pr.Out.Header.Set(h.Name, h.Value)
		}
	}
}

// IdentityHeader is one header the "headers" mechanism emits.
type IdentityHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// The signed-in account under the two names an application may already read.
//
// There is no standard here. REMOTE_USER is a CGI meta-variable (RFC 3875),
// which a web server DERIVES from its own authentication - it was never a
// header, and no such field is registered with IANA. What exists are proxy
// conventions, and Meerkat writes the two that matter:
//
//   - REMOTE_USER, which is what Spring (and NEO's own gateway) sends, so an
//     application ported from behind one keeps working untouched;
//   - X-Remote-User, because an underscore in a header name is dropped on the
//     way by default in nginx (underscores_in_headers off) and mistrusted by
//     several others - the first name alone can silently never arrive.
//
// Both are purged inbound: they are the headers an upstream trusts most.
const (
	RemoteUserHeader  = "REMOTE_USER"
	RemoteUserHeaderX = "X-Remote-User"
)

// remoteUserNames is what carries the account, and what is purged before it.
var remoteUserNames = []string{RemoteUserHeader, RemoteUserHeaderX}

// identityHeaderPairs renders the headers cfg emits for the caller d, in
// attribute order: each selected fact under its mapped name, roles as a JSON
// array or a comma-separated string. Shared by the forwarding filter and the
// admin preview, so what the console shows is what the upstream receives.
//
// The username also travels as REMOTE_USER, always, in addition to whatever
// name the route gave it: it is the one header an application can be assumed
// to already read.
func identityHeaderPairs(cfg store.IdentityForward, d identityData) []IdentityHeader {
	out := make([]IdentityHeader, 0, len(cfg.Attributes)+1)
	for _, a := range cfg.Attributes {
		if a.Field != "username" {
			continue
		}
		v := userFieldValue(d, "username")
		if v == "" {
			break
		}
		for _, name := range remoteUserNames {
			// Named by hand on the route: sent once, under that name.
			if strings.EqualFold(a.As, name) {
				continue
			}
			out = append(out, IdentityHeader{Name: name, Value: v})
		}
		break
	}
	for _, a := range cfg.Attributes {
		name := a.As
		if name == "" {
			name = a.Field
		}
		if a.Field == "roles" {
			if len(d.Roles) == 0 {
				continue
			}
			v, err := renderRoles(a.Expr, d)
			if err != nil || v == "" {
				continue
			}
			out = append(out, IdentityHeader{Name: name, Value: v})
			continue
		}
		if v := userFieldValue(d, a.Field); v != "" {
			out = append(out, IdentityHeader{Name: name, Value: v})
		}
	}
	return out
}

// renderRoles shapes the caller's roles the way this route asks. A broken
// expression cannot reach here - routes are validated on save and on reload -
// and if one ever did, sending nothing beats sending something half-built.
func renderRoles(expr string, d identityData) (string, error) {
	compiled, err := routing.CompileRoleExpr(expr)
	if err != nil {
		return "", err
	}
	return compiled.Render(d.Roles, func(role string) []string { return d.TagsOfRole[role] })
}

// sampleIdentity is the FICTIONAL caller the identity preview describes. It is
// fixed here on purpose: the preview mints a real (and, for signed-jwt, really
// signed) token, so letting a caller choose the claim values would let a
// infra admin forge a token for anyone the upstream trusts.
var sampleIdentity = identityData{
	UserID: "usr_123", Username: "jdoe", Fullname: "Jane Doe",
	Email: "jdoe@example.com", Timezone: "Europe/Paris",
	TenantID: "tnt_123", Tenant: "acme", Roles: []string{"role-a", "role-b"},
}

// PreviewHeaders renders the headers cfg would send for the sample caller.
func PreviewHeaders(cfg store.IdentityForward) []IdentityHeader {
	return identityHeaderPairs(cfg, sampleIdentity)
}

// PreviewClaims builds the JWT payload cfg would send for the sample caller.
func PreviewClaims(cfg store.IdentityForward, routeName string) map[string]any {
	return identityClaims(cfg, routeName, sampleIdentity)
}

// MintUnsignedJWT encodes an unsigned (alg:none) token, the "jwt" mechanism's
// wire form. Exported for the admin preview.
func MintUnsignedJWT(claims map[string]any) (string, error) {
	return mintUnsignedJWT(claims)
}

// identityClaims builds the JWT payload: the registered claims (iss/sub/aud/
// iat/exp) plus one custom claim per selected attribute, under its mapped name.
func identityClaims(cfg store.IdentityForward, routeName string, d identityData) map[string]any {
	now := time.Now()
	ttl := 2 * time.Minute
	if cfg.TTL != "" {
		if parsed, err := store.ParseISODuration(cfg.TTL); err == nil {
			ttl = parsed
		}
	}
	claims := map[string]any{
		"iss": gatewayIssuer,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	if routeName != "" {
		claims["aud"] = routeName
	}
	if d.UserID != "" {
		claims["sub"] = d.UserID
	}
	for _, a := range cfg.Attributes {
		name := a.As
		if name == "" {
			name = a.Field
		}
		if a.Field == "roles" {
			// A claim may legitimately be a JSON array, so an expression that
			// renders one lands as an array rather than as a string of one.
			v, err := renderRoles(a.Expr, d)
			if err != nil {
				continue
			}
			var arr []string
			if json.Unmarshal([]byte(v), &arr) == nil {
				claims[name] = arr
			} else {
				claims[name] = v
			}
			continue
		}
		if v := userFieldValue(d, a.Field); v != "" {
			claims[name] = v
		}
	}
	return claims
}

// mintUnsignedJWT encodes an unsigned (alg:none) JWT: header.payload with an
// empty signature. It carries structure, not trust - the verifiable variant is
// signed-jwt (Lot 2).
func mintUnsignedJWT(claims map[string]any) (string, error) {
	seg := func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	header, err := seg(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := seg(claims)
	if err != nil {
		return "", err
	}
	return header + "." + payload + ".", nil
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// pageAgentFragment builds the script tag for /meerkat/page.js: what the
// gateway makes the PAGE do - the language someone chose, the light/dark
// scheme they picked - carried on the script's own dataset.
//
// Injected on EVERY UI route, never for the button: the two used to travel
// together, and a route offering the light/dark switch with the button turned
// off got nothing at all. It carries the session watch too, which is why no
// route opts out - a page with no agent is a page that goes on looking signed
// in for hours after the session ended.
func pageAgentFragment(r store.Route, localeCodes []string) string {
	if !r.IsUI || r.UI == nil {
		return ""
	}
	attrs := ""
	// Two different things, and they used to ride together (see SchemeConfig):
	//
	//   - OFFERING the switch is chrome. It belongs to the user button - and
	//     with a portal there is no per-route button to hang it on.
	//   - HOW the application consumes a scheme (its mechanism, its tag, its
	//     stored key) is the ROUTE's, true whoever offers the switch.
	//
	// So the mechanism is written whenever the route describes one, and
	// data-scheme="select" says only whether this route offers the switch.
	// An application with ONE look (no mechanism, and the route says which):
	// the page is told what it wears, so the document carries that color-scheme
	// and everything that inherits it - the portal bar first, whose colours are
	// the document's - matches the application instead of the visitor's choice,
	// which this application does not take anyway.
	if s := r.UI.Scheme; s != nil && s.Mechanism == store.SchemeNone && (s.Button == "light" || s.Button == "dark") {
		attrs += fmt.Sprintf(` data-scheme-wear="%s"`, s.Button)
	}
	if s := r.UI.Scheme; s != nil && s.Mechanism != store.SchemeNone {
		if s.Select {
			attrs += ` data-scheme="select"`
		}
		// The application has no follow-the-system state, so the agent resolves
		// the visitor's auto before applying it - see SchemeConfig.NoAuto.
		if s.NoAuto {
			attrs += ` data-scheme-no-auto="1"`
		}
		switch {
		case s.Mechanism == store.SchemeScript:
			// Only the NAME rides here. The body itself is served as its own
			// same-origin file (/meerkat/scheme.js), because an application
			// worth a gateway sends a Content-Security-Policy and the ordinary
			// one - "script-src 'self'", which is RabbitMQ's - refuses
			// new Function while allowing that script. Measured in a browser,
			// against the real console, after the attribute had been written.
			attrs += fmt.Sprintf(` data-scheme-mechanism="%s"`, store.SchemeScript)
		case s.Mechanism != "":
			// Tag included, always: the browser half applies the mechanism to
			// what it names, and defaulting it there rather than here would be
			// a second place where "which element" is decided.
			attrs += fmt.Sprintf(` data-scheme-mechanism="%s" data-scheme-tag="%s" data-scheme-light="%s" data-scheme-dark="%s"`,
				s.Mechanism, orDefault(s.Tag, "html"), s.Light, s.Dark)
			if store.SchemeSetsAttribute(s.Mechanism) {
				attrs += fmt.Sprintf(` data-scheme-attribute="%s"`, s.Attribute)
			}
		}
		if s.StorageOverride && s.Storage != "" {
			attrs += fmt.Sprintf(` data-scheme-storage="%s" data-scheme-storage-light="%s" data-scheme-storage-dark="%s"`,
				s.Storage, orDefault(s.StorageLight, "light"), orDefault(s.StorageDark, "dark"))
			if s.StorageAuto != "" {
				attrs += fmt.Sprintf(` data-scheme-storage-auto="%s"`, s.StorageAuto)
			}
		}
	}
	if len(localeCodes) > 0 {
		// The ROUTE's locale offer (codes are validated BCP 47, HTML-safe).
		attrs += fmt.Sprintf(` data-languages="%s"`, strings.Join(localeCodes, ","))
		// The path and query mechanisms have a browser half, and this is it.
		// Where the language lives in the URL, picking one has to NAVIGATE: an
		// application built per locale stamps <base href="/app/fr/">, so every
		// link it draws afterwards carries fr, and a reload would keep asking
		// for the same French page forever - the menu would look broken while
		// doing exactly as it was told. The attribute says WHERE the segment
		// sits: after the route's own prefix.
		switch {
		case r.Locales != nil && r.Locales.Mode() == store.LocalePath:
			attrs += fmt.Sprintf(` data-locale-paths="%s"`, htmlEscape(strings.Join(routing.PathPrefixes(r.Predicates), ",")))
		case r.Locales != nil && r.Locales.Mode() == store.LocaleQuery:
			attrs += fmt.Sprintf(` data-locale-param="%s"`, htmlEscape(orDefault(r.Locales.Param, "lg")))
		}
	}
	agent := `<script defer src="/meerkat/page.js"` + attrs + `></script>`
	// The route's own scheme script, before the agent so it is defined by the
	// time the agent's catch-up calls it. The body's hash is in the URL: the
	// answer is cacheable, and an edit reaches a browser holding the last one.
	if s := r.UI.Scheme; s != nil && s.Mechanism == store.SchemeScript && s.Script != "" {
		sum := sha256.Sum256([]byte(s.Script))
		agent = fmt.Sprintf(`<script defer src="/meerkat/scheme.js?r=%s&amp;v=%x"></script>`,
			url.QueryEscape(r.ID), sum[:4]) + agent
	}
	// The application owns its colour scheme, and the way to work WITH it is to
	// speak its own storage, not to fight it on the document.
	//
	// ng-m3-theme (understory) is the case that taught this: its service keeps
	// "system | light | dark" under a key, and in system mode it CLEARS
	// document.documentElement.style.colorScheme on every run. Anything the
	// gateway set was wiped a tick later - and with nothing stored, its default
	// IS system. Writing the visitor's choice into that key instead lets the
	// application apply it the way it already knows how, before its first paint,
	// and the chrome then simply inherits the document.
	//
	// Inline and not deferred, so it lands before the app's own boot script. Only
	// when a scheme was actually chosen; with none, the app keeps its own memory
	// and follows the system, which is what "auto" means.
	if s := r.UI.Scheme; s != nil && s.StorageOverride && s.Storage != "" && s.Mechanism != store.SchemeNone {
		light, dark := orDefault(s.StorageLight, "light"), orDefault(s.StorageDark, "dark")
		// An empty value is a choice of its own: remove the entry and let the
		// application fall back to whatever it does with nothing stored.
		setAuto := `localStorage.removeItem(k);`
		if s.StorageAuto != "" {
			setAuto = `localStorage.setItem(k,'` + s.StorageAuto + `');`
		}
		agent = `<script>(function(){try{var k='` + s.Storage + `';` +
			`var m=document.cookie.match(/(^|;\s*)MEERKAT_SCHEME=(light|dark|auto)/);if(!m)return;` +
			`if(m[2]==='auto'){` + setAuto + `return;}` +
			`localStorage.setItem(k,m[2]==='dark'?'` + dark + `':'` + light + `');` +
			`}catch(e){}})();</script>` + agent
	}
	return agent
}

// userButtonFragment builds the HTML injected at the top of <body> for a UI
// route with the user button on: the element positions itself fixed, so where
// it sits in the body does not matter - but it must not sit in the HEAD, which
// is where it used to land and where it truncated the document's own metadata.
// Values are validated (validateRouteType) - no free text reaches HTML.
func userButtonFragment(r store.Route, localeCodes []string) string {
	if !r.IsUI || r.UI == nil || !r.UI.UserButton.Enabled {
		return ""
	}
	btn := r.UI.UserButton
	height := btn.Height
	if height == 0 {
		height = 24
	}
	position := btn.Position
	if position == "" {
		position = "top-right"
	}
	// The route id feeds the developer bar (uisim.go): the UI test is scoped
	// to THIS route. Server-generated id, HTML-safe.
	attrs := fmt.Sprintf(` height="%d" position="%s" route="%s"`, height, position, htmlEscape(r.ID))
	if btn.PadX != 0 {
		attrs += fmt.Sprintf(` pad-x="%d"`, btn.PadX)
	}
	if btn.PadY != 0 {
		attrs += fmt.Sprintf(` pad-y="%d"`, btn.PadY)
	}
	if btn.Shape == "square" {
		attrs += ` shape="square"`
	}
	if btn.Name != "" {
		attrs += fmt.Sprintf(` name="%s"`, btn.Name)
	}
	// The component keeps itself out of a framed page unless this says
	// otherwise; the markup is the same either way, so nothing varies per
	// context and no cache can serve one context the other's page.
	if btn.InFrame {
		attrs += ` in-frame`
	}
	// Whether the menu SHOWS the light/dark switch. What the switch then does
	// to the page is the agent's, and its configuration travels there.
	// A UI with no colour scheme of its own has nothing to switch between, so
	// the menu does not draw the switch whatever Select says.
	if s := r.UI.Scheme; s != nil && s.Select && s.Mechanism != store.SchemeNone {
		attrs += ` scheme="select"`
		// Two states instead of three: an application that knows only light and
		// dark must not be offered a switch with a position it cannot hold.
		if s.NoAuto {
			attrs += ` no-auto`
		}
	} else if s != nil && (s.Button == "light" || s.Button == "dark") {
		// No switch here, so nothing for the button to follow: the route says
		// what it wears. An application with one look otherwise gets a button
		// following the visitor's system, light on a page that is always dark.
		attrs += fmt.Sprintf(` scheme-wear="%s"`, s.Button)
	}
	// The ROUTE's locale offer feeds the button's language submenu (codes are
	// validated BCP 47, HTML-safe; the component renders the endonyms).
	if len(localeCodes) > 0 {
		attrs += fmt.Sprintf(` languages="%s"`, strings.Join(localeCodes, ","))
	}
	return `<script defer src="/meerkat/user-button.js"></script>` +
		`<meerkat-user-button` + attrs + `></meerkat-user-button>`
}

// portalFragment builds the HTML injected at the top of <body> for a UI route
// when a portal is configured (PORTAL-01): the navigation bar, plus the user
// button it mounts inside itself. The bar reads everything else - the modules,
// the theme, the account - from /meerkat/portal.json and /meerkat/user-button.json,
// so nothing route-specific rides in the markup, and it stays out of a framed
// page on its own (self !== top).
//
// user-button.js loads BEFORE portal.js so the element the bar creates is
// already defined when the bar mounts it; page.js (from pageAgentFragment)
// comes first of all, so window.meerkatPage exists before either upgrades.
// schemeOf is a route's colour-scheme configuration, or nil. Two nils to walk
// and both are ordinary: a route may serve pages without configuring any
// injection at all.
func schemeOf(r store.Route) *store.SchemeConfig {
	if r.UI == nil {
		return nil
	}
	return r.UI.Scheme
}

func portalFragment(r store.Route, localeCodes []string) string {
	if !r.IsUI {
		return ""
	}
	// THIS ROUTE's languages, like the standalone button's. The bar mounts the
	// account button, and the language a menu offers is the one the page behind
	// it is written in - not the gateway's whole offer, which is what the
	// payload carries for want of knowing which page asked. A route written in
	// a language Meerkat does not embed still offers it here, which is the
	// point: the sign-in page cannot, and this page can.
	attr := ""
	if len(localeCodes) > 0 {
		attr = ` languages="` + htmlEscape(strings.Join(localeCodes, ",")) + `"`
	}
	// Route-specific, like the languages above: the bar is global, the
	// application behind THIS route is not. The bar mounts the user button, so
	// what that button offers is decided per route here.
	//
	// r.UI may be nil: a route can serve pages without configuring any of the
	// injections, and this one asked its Scheme before checking.
	if s := schemeOf(r); s != nil && s.Mechanism == store.SchemeNone {
		// NOTHING TO SWITCH. An application declared to have no colour scheme
		// takes no choice, so a switch on top of it moves the bar and leaves
		// the page as it was - a control that appears to do something and does
		// not. What the route may still say is what the BAR wears there, so a
		// bar does not float light over a page that is always dark.
		if s.Button == "light" || s.Button == "dark" {
			attr += ` scheme-wear="` + htmlEscape(s.Button) + `"`
		} else {
			attr += ` scheme="none"`
		}
	} else if s != nil && s.NoAuto {
		// Two positions instead of three: an application that knows only light
		// and dark must not be offered a position nothing behind it can hold.
		attr += ` no-auto`
	}
	return `<script defer src="/meerkat/user-button.js"></script>` +
		`<script defer src="/meerkat/portal.js"></script>` +
		`<meerkat-portal-nav` + attr + `></meerkat-portal-nav>`
}

// unavailablePage is the HTML served when a UI route's upstream does not answer
// while a portal is on (PORTAL-01): the SAME injected chrome (page agent + bar)
// rides on it, so the visitor keeps the menu and can open another application
// instead of being stranded on a bare 502. The message is deliberately plain -
// the point is the bar, not the notice.
func unavailablePage(fragment string) string {
	return `<!doctype html><html><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1"><title>Unavailable</title></head>` +
		`<body>` + fragment +
		`<div style="max-width:32rem;margin:18vh auto 0;padding:0 24px;text-align:center;` +
		`font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#5b5f66;">` +
		`<h1 style="font-size:20px;font-weight:600;margin:0 0 8px;">This application is not responding</h1>` +
		`<p style="margin:0;font-size:15px;">Use the menu to open another one.</p>` +
		// The name of this request, for whoever ends up reporting it. The page
		// is built once per route, so the identifier goes in at the moment it
		// is written - a replacement on a 502 rather than on every request.
		`<p style="margin:24px 0 0;font-size:12px;color:#9aa0a6;font-family:ui-monospace,monospace;">` +
		tracing.Placeholder + `</p>` +
		`</div></body></html>`
}

// The transports, one per pair of bounds (ROUTE-07).
//
// A hung upstream hangs the client forever without them; with them the request
// fails fast as a 502. Body streaming is NOT bounded, deliberately - a long
// download and a websocket must live. What is bounded is how long an upstream
// may take to ACCEPT a connection and to START answering.
//
// They used to be one global pair, which meant the slowest legitimate endpoint
// in an installation set the wait for every other one. A route names its own
// now, and routes that name the same numbers - which is every route on the
// defaults, so almost all of them - SHARE a transport. That matters: a
// transport is a connection pool, and one per route would multiply the
// sockets held against every upstream by the number of routes pointing at it.
var (
	transportsMu sync.Mutex
	transports   = map[[2]time.Duration]http.RoundTripper{}
)

// idlePerUpstream is how many idle connections a pool KEEPS to one upstream
// once a burst is over. It is not a cap on how many it opens.
//
// It was 8, and that number was the gateway's ceiling. Past eight requests in
// flight to the same upstream, every connection beyond the eighth was CLOSED
// when its response ended and dialled again for the next request: a TCP
// handshake per request, and a socket per request left in TIME_WAIT. On one
// core the benchmark (tools/bench) read 4,164 req/s where Traefik, Kong and
// APISIX read 34,000 to 44,000 - and a longer run exhausted the ephemeral ports
// and answered 502s that looked like a broken upstream. At 256 the same run
// reads 28,000 req/s with no error. Traefik keeps 200.
//
// Keeping them costs nothing that was not already spent: an idle connection
// exists only because that many were needed a moment ago, and IdleConnTimeout
// closes it when the burst does not come back.
const idlePerUpstream = 256

func transportFor(connect, response time.Duration) http.RoundTripper {
	key := [2]time.Duration{connect, response}
	transportsMu.Lock()
	defer transportsMu.Unlock()
	if t, ok := transports[key]; ok {
		return t
	}
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: connect}).DialContext,
		TLSHandshakeTimeout:   connect,
		ResponseHeaderTimeout: response,
		ForceAttemptHTTP2:     true,
		MaxIdleConnsPerHost:   idlePerUpstream,
		// Below the common load-balancer keep-alive (AWS ELB: 60s): OUR side
		// drops an idle connection first, so a request never rides one the
		// upstream already closed.
		IdleConnTimeout: 55 * time.Second,
	}
	transports[key] = t
	return t
}

// cookieStrippingTransport removes Meerkat's own session cookies at the very
// last moment before the wire: they are gateway-internal credentials and must
// never reach an upstream (cookies are host-scoped, not port-scoped, so on a
// same-host deployment the browser sends them with every data-plane request -
// identity travels through the route's Identity mechanism instead). Stripping
// here, after every proxy hook, keeps the original request on the response so
// ModifyResponse (pageStamp) still resolves the session.
type cookieStrippingTransport struct{ base http.RoundTripper }

func (t cookieStrippingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !carriesGatewayInternals(req) {
		return t.base.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	stripGatewayCookies(clone)
	// Same story for the simulation knobs: gateway-internal, already
	// consumed - the upstream sees the resulting identity, not the knobs.
	clone.Header.Del(SimulateUserHeader)
	clone.Header.Del(SimulateRolesHeader)
	if strings.HasPrefix(clone.Header.Get("Authorization"), "Bearer "+SimTokenPrefix) {
		clone.Header.Del("Authorization")
	}
	// A simulated call is a TEST through swagger, not a genuine user action:
	// mark the upstream request so the backend's own action log can tell them
	// apart (X-Meerkat-Test = the tool, -By = the real developer behind it).
	if meta, ok := simulationMeta(clone.Context()); ok {
		clone.Header.Set("X-Meerkat-Test", meta.Via)
		if meta.By != "" {
			clone.Header.Set("X-Meerkat-Test-By", meta.By)
		}
	}
	res, err := t.base.RoundTrip(clone)
	if res != nil {
		res.Request = req
	}
	return res, err
}

// carriesGatewayInternals says whether RoundTrip has anything to take out:
// a session cookie, a simulation knob, a test token, a simulation mark to add.
// A request with none of them - every anonymous call, every API call on a
// token - goes out as it is, without the clone of its headers that stripping
// needs. A false positive (a cookie merely containing the name) only takes
// the careful path.
func carriesGatewayInternals(req *http.Request) bool {
	if c := req.Header.Get("Cookie"); c != "" &&
		(strings.Contains(c, session.CookieName) || strings.Contains(c, session.AdminCookieName)) {
		return true
	}
	if req.Header.Get(SimulateUserHeader) != "" || req.Header.Get(SimulateRolesHeader) != "" {
		return true
	}
	if strings.HasPrefix(req.Header.Get("Authorization"), "Bearer "+SimTokenPrefix) {
		return true
	}
	_, simulated := simulationMeta(req.Context())
	return simulated
}

func buildProxy(r store.Route, cf routing.CompiledFilters, defaults store.RouteTimeouts, unavailable string) (http.Handler, error) {
	target, err := url.Parse(r.Upstream)
	if err != nil {
		return nil, fmt.Errorf("bad upstream %q: %w", r.Upstream, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("bad upstream %q: scheme and host required", r.Upstream)
	}

	connect, response := r.Timeouts.Durations(defaults)
	// The scheme said which transport; the request itself is plain http from
	// here on, so everything downstream - filters, SetURL, the stamp - sees
	// what it has always seen.
	rt := transportFor(connect, response)
	if target.Scheme == SchemeH2C {
		rt = h2cTransportFor(connect, response)
		target = h2cTarget(target)
	}
	proxy := &httputil.ReverseProxy{
		BufferPool: proxyBuffers,
		Transport:  tracedTransport{cookieStrippingTransport{rt}},
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetXForwarded()
			// Whatever the caller sent under this name goes: it tells the
			// service where it is published, so a caller who poses their own
			// makes the service write its links wherever they asked. Purged
			// unconditionally, even on a route that never sets it - a route
			// without strip-prefix must not carry a stranger's idea of its
			// own prefix either. Same reasoning as the identity headers.
			pr.Out.Header.Del(routing.ForwardedPrefixHeader)
			// The journey's name goes on, always (OBS-04). What the caller
			// sent travels untouched - it is what stitches their spans to the
			// service's - and when nobody sent anything we put ours, with the
			// sampling bit off: the service can write the identifier in its
			// own audit without anybody being asked to export a thing.
			if tc, ok := tracing.From(pr.In.Context()); ok {
				pr.Out.Header.Set(tracing.Header, tc.Header())
			}
			// Request filters transform the request path/headers first, THEN
			// the upstream base path is prepended by SetURL - so strip-prefix
			// and friends reason on the request path, never on the upstream's.
			for _, f := range cf.Request {
				f(pr)
			}
			// SetURL points the request at the upstream AND takes the Host
			// with it, which is right by default and wrong for the filters
			// whose whole purpose is that header: a service answering by
			// virtual host needs a name the upstream URL does not carry, and
			// one building its own links needs the name the CALLER used.
			//
			// The signal is the Host HEADER, not a changed value. Comparing
			// values before and after made preserve-host a no-op nobody could
			// explain: it writes the caller's Host, which is what Out already
			// carried, so "nothing changed" and the upstream's name went out.
			// A header, on the other hand, says a filter asked - the client
			// never sends one (Go moves it to Request.Host), so its presence
			// is unambiguous.
			asked := pr.Out.Header.Get("Host")
			pr.SetURL(target)
			if asked != "" {
				pr.Out.Host = asked
			}
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			// A body that blew through the route's cap while it was being
			// copied upstream (no declared length, so the guard could only
			// find out on the way through). It is the CALLER who is at fault,
			// not the service - and the service, in this case, was never
			// even asked.
			if mbe, ok := filtering.OversizeBody(err); ok {
				slog.Info("request refused: body over the route limit",
					"route", r.Name, "limit", mbe.Limit)
				filtering.RefuseSize(w, req, http.StatusRequestEntityTooLarge,
					"request body too large", -1, mbe.Limit)
				return
			}
			slog.Warn("upstream error", "route", r.Name, "upstream", r.Upstream, "err", err)
			// On a UI route under a portal, a dead upstream must not strand the
			// visitor: serve an HTML page that still carries the bar, so they can
			// open another application. Elsewhere, the plain 502 stands.
			if unavailable != "" {
				h := w.Header()
				h.Set("Content-Type", "text/html; charset=utf-8")
				h.Set("Cache-Control", "no-store")
				h.Set("Retry-After", "30")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w,
					strings.Replace(unavailable, tracing.Placeholder, tracing.ID(req.Context()), 1))
				return
			}
			// The name of the request goes in the TEXT as well as the header:
			// this is the 502 a person actually sees - in a curl, in a browser
			// console, pasted into a ticket - and a header is invisible there.
			http.Error(w, "upstream unavailable (request "+tracing.ID(req.Context())+")",
				http.StatusBadGateway)
		},
	}
	proxy.ModifyResponse = func(res *http.Response) error {
		// Make upstream failures visible in OUR logs: a 5xx reaching the client
		// through the proxy comes from the application, not from the gateway
		// (the gateway's own failure is the 502 in ErrorHandler).
		if res.StatusCode >= 500 {
			slog.Warn("upstream answered 5xx", "route", r.Name, "upstream", r.Upstream, "status", res.StatusCode)
		}
		for _, f := range cf.Response {
			if err := f(res); err != nil {
				return err
			}
		}
		return nil
	}
	return proxy, nil
}

// guarded puts the route's circuit breaker (ROUTE-09) in front of a handler.
//
// Off, it is one comparison against a threshold nothing can reach - "off" and
// "on" travel the same path rather than the caller branching. On, a route
// whose upstream has stopped answering stops being called at all: the caller
// gets the unavailable page immediately instead of waiting out the timeout,
// and the service is met by ONE request after the cool-down rather than by
// everything that piled up.
func (rt *Router) guarded(r store.Route, next http.Handler) http.Handler {
	cfg := store.CircuitBreaker{}
	if r.Breaker != nil {
		cfg = *r.Breaker
	}
	if !cfg.Enabled {
		return next
	}
	id := r.ID
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		br := rt.breakers.of(id)
		state, allowed := br.take(cfg, time.Now())
		if !allowed {
			slog.Debug("circuit open, upstream not called", "route", r.Name)
			rt.serveUnreachable(w, req)
			return
		}
		ww := &watched{ResponseWriter: w}
		next.ServeHTTP(ww, req)
		switch {
		case failedStatus(ww.status):
			br.failed(ww.status, http.StatusText(ww.status), time.Now())
			if state == circuitProbe {
				slog.Info("upstream still down after the cool-down", "route", r.Name)
			}
		default:
			if state != circuitClosed {
				slog.Info("upstream answering again, circuit closed", "route", r.Name)
			}
			br.succeeded(ww.status, time.Now())
		}
	})
}

// serveUnreachable answers for a route whose circuit is open. The SAME page
// the global switch and the maintenance brick serve, with the reason this
// actually is: nobody planned this one.
func (rt *Router) serveUnreachable(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if rt.Pages != nil {
		rt.Pages(w, req, store.ReasonIncident, 0, "")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Retry-After", "30")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write(routing.MaintenancePage())
}

// filterOwnResponse runs the route's outgoing filters over an answer the
// gateway produced itself.
//
// It buffers, which is the honest trade: a filter takes an *http.Response, and
// a handler writes straight to the client. What a terminal answers is a page or
// a small document, so holding it in memory to let a header filter see it costs
// nothing measurable - and the alternative is telling admins that outgoing
// filters work everywhere except here.
func filterOwnResponse(next http.Handler, filters []routing.ResponseFilter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		res := &http.Response{
			StatusCode: rec.status,
			Header:     rec.header.Clone(),
			Body:       io.NopCloser(bytes.NewReader(rec.body.Bytes())),
			Request:    r,
		}
		for _, f := range filters {
			if err := f(res); err != nil {
				slog.Warn("outgoing filter failed on the route's own answer", "err", err)
				http.Error(w, "the gateway could not build this answer", http.StatusInternalServerError)
				return
			}
		}
		body, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			http.Error(w, "the gateway could not build this answer", http.StatusInternalServerError)
			return
		}
		out := w.Header()
		for k := range out {
			out.Del(k)
		}
		for k, vs := range res.Header {
			for _, v := range vs {
				out.Add(k, v)
			}
		}
		out.Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(res.StatusCode)
		_, _ = w.Write(body)
	})
}

// bufferedResponse collects a handler's answer so response filters can see it.
type bufferedResponse struct {
	header  http.Header
	status  int
	written bool
	body    bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(status int) {
	if !b.written {
		b.status, b.written = status, true
	}
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	b.written = true
	return b.body.Write(p)
}

// withIdentity resolves the caller and hands it to a terminal that answers
// from it (the "respond" brick). No session is not an error here: the template
// asks {{if .SignedIn}} and decides what an anonymous caller is told - which is
// what lets one route serve both the signed-in shape and the public one.
func (rt *Router) withIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var id routing.Identity
		if d, ok := rt.sessionIdentity(req); ok {
			id = routing.Identity{
				Username: d.Username, UserID: d.UserID, Fullname: d.Fullname,
				Email: d.Email, Tenant: d.Tenant, TenantID: d.TenantID,
				Timezone: d.Timezone, Roles: d.Roles,
			}
		}
		next.ServeHTTP(w, req.WithContext(routing.WithIdentity(req.Context(), id)))
	})
}

// requireSession gates a route handler behind a valid session: browsers
// navigating to HTML get redirected to the gateway's login page with a
// return-to path, API-style requests get a plain 401.
func requireSession(sm *session.Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// A simulated identity (simulate.go) IS the session for this request,
		// so is a scheduled call's (scheduled.go), and an internal spec read
		// was authorized on the control plane.
		_, simulated := simulatedIdentity(req.Context())
		_, scheduled := scheduledCaller(req.Context())
		if simulated || scheduled || isSpecRead(req.Context()) {
			next.ServeHTTP(w, req)
			return
		}
		sess, err := sm.Resolve(req.Context(), req)
		if err == nil && sess.Pending != "" {
			// AUTH-05: until every login step is satisfied, all navigation is
			// redirected to the current step.
			if wantsHTML(req) {
				http.Redirect(w, req, "/"+sess.Pending+"?next="+url.QueryEscape(req.URL.RequestURI()), http.StatusSeeOther)
			} else {
				http.Error(w, "login flow incomplete", http.StatusUnauthorized)
			}
			return
		}
		if err != nil {
			if wantsHTML(req) {
				http.Redirect(w, req, "/login?next="+url.QueryEscape(req.URL.RequestURI()), http.StatusSeeOther)
				return
			}
			w.Header().Set("WWW-Authenticate", "Session")
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func wantsHTML(req *http.Request) bool {
	return req.Method == http.MethodGet && strings.Contains(req.Header.Get("Accept"), "text/html")
}

// serveSpecFile answers exactly one path with the route's deposited spec and
// hands everything else to the route. It is a decoration of the route, not a
// second target: same prefix, same access rule, one document.
//
// It SHADOWS whatever the upstream serves at the same path, which is the point
// - replacing an incomplete or unannotated spec is why a file gets deposited -
// so the console announces the collision when the file is deposited rather
// than leaving it to be discovered.
func serveSpecFile(path string, served, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			next.ServeHTTP(w, r)
			return
		}
		served.ServeHTTP(w, r)
	})
}

// specFileHandler writes the spec, with the ETag that lets a client skip the
// bytes it already has. The spec is a JSON document whatever was deposited.
func specFileHandler(body []byte) http.Handler {
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "openapi.json", time.Time{}, bytes.NewReader(body))
	})
}

// telemetryFragment is what puts a trace at the CLICK rather than at the front
// door (OBS-04): the configuration inline, then the bundle this gateway serves
// itself.
//
// INLINE RATHER THAN FETCHED, and that is the whole reason the configuration
// is not a second endpoint: a round trip for it would happen while the page is
// already firing its first calls, and those are exactly the ones worth
// measuring. The bundle stays immutable and cacheable; this weighs a line.
//
// The propagate list is NOT written here. The page's own origin depends on the
// host it was asked for, and one gateway serves several - so the bundle works
// it out from location.origin, and what travels from here is only the extra
// names this gateway answers to that a page could not guess.
func (rt *Router) telemetryFragment(r store.Route) string {
	// Three answers, and all three have to be yes: this gateway exports at
	// all, this route is in the telemetry, and this route asked for the
	// journey to start in its pages.
	if !rt.traceExport || !r.IsUI || noTracing(r) || !r.TelemetryUI {
		return ""
	}
	cfg, err := json.Marshal(map[string]any{
		// The relay, not the collector: a page never learns where the
		// collector is, and never carries its credential.
		"endpoint":   "/meerkat/telemetry",
		"sample":     rt.traceSample,
		"service":    "browser",
		"sameOrigin": true,
	})
	if err != nil {
		return ""
	}
	// DEFERRED, and injected FIRST. Deferred so 25 KB never block the first
	// paint; first so that among the deferred scripts - which run in document
	// order, and an application's own bundle is one of them - fetch and
	// XMLHttpRequest are patched before the app makes its first call. What
	// this cannot catch is a call made by an INLINE script while the document
	// is still parsing, which is the price of not blocking the paint.
	return `<script>window.__MEERKAT_OTEL__=` + string(cfg) + `</script>` +
		`<script defer src="/meerkat/telemetry.js"></script>`
}
