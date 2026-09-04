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
	"time"

	filtering "github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/openapi"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/signing"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// Router matches incoming requests against the compiled routes, first match
// wins in route order.
type Router struct {
	st *store.Store
	sm *session.Manager

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

	mu       sync.RWMutex
	routes   []compiledRoute
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

	// tagsMu guards the catalogue's tag -> role names table, read by routes
	// that narrow what they forward. Cached: the catalogue changes far less
	// often than requests arrive, and a few seconds late is invisible.
	tagsMu     sync.Mutex
	tagsCache  map[string][]string
	tagsReadAt time.Time
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
	return &Router{st: st, sm: sm, lottery: rand.Float64, simTokenKey: key, breakers: newBreakers()}
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
	var appLangs []string
	_ = rt.st.GetSetting(ctx, store.SettingLanguages, &appLangs)
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

	compiled := make([]compiledRoute, 0, len(stored))
	var allPreds []*routing.CompiledPredicates
	needDraw := false
	for _, raw := range stored {
		if !raw.Enabled {
			continue
		}
		r, missing, err := ExpandRoute(raw, values)
		if err != nil {
			return fmt.Errorf("gateway: route %q: %w", raw.Name, err)
		}
		if len(missing) > 0 {
			// Left OUT rather than compiled. A route whose references do not
			// resolve cannot serve anything - its upstream is "https://" with no
			// host - and failing the whole reload over it would take every other
			// route down with it. That is the normal state of a gateway just
			// seeded from a file (CFG-03): the configuration is in place, the
			// vault is not filled yet, and it has to start anyway.
			slog.Warn("route left out: it references vault entries that hold nothing",
				"route", raw.Name, "names", missing)
			continue
		}
		cr, err := rt.compile(r, appLangs, specs[raw.ID])
		if err != nil {
			return fmt.Errorf("gateway: route %q: %w", r.Name, err)
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

	rt.mu.Lock()
	rt.routes = compiled
	rt.needDraw = needDraw
	rt.signing = sset
	rt.maintenance, rt.maintenancePage = maint, renderMaintenance(maint)
	if simKey != nil {
		rt.simTokenKey = simKey
	}
	rt.mu.Unlock()
	slog.Info("routes reloaded", "count", len(compiled))
	return nil
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
			r.handler.ServeHTTP(w, hit)
			return
		}
		d, ok := rt.sessionIdentity(hit)
		who := rt.caller(hit, d, ok)
		if (ok && r.access.Grants(who)) || isSpecRead(hit.Context()) {
			r.handler.ServeHTTP(w, hit)
			return
		}
		// A closed door stays closed. Falling through a "deny" to whatever
		// matches next would turn the one rule written to shut a path into a
		// rule that merely redirects it.
		if r.access.Level == store.AccessDeny {
			rt.refuse(w, hit, r.access, r.isUI, ok, who, d.UserID)
			return
		}
		if cand == nil {
			cand, candReq, candOK, candWho, candUserID = r, hit, ok, who, d.UserID
		}
	}
	if cand != nil {
		rt.refuse(w, candReq, cand.access, cand.isUI, candOK, candWho, candUserID)
		return
	}
	http.NotFound(w, req)
}

func (rt *Router) compile(r store.Route, appLangs []string, deposited []byte) (compiledRoute, error) {
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
	// The route may EXCLUDE application locales its UI does not support:
	// they leave the button's menu and the forwarding resolution.
	localeCodes := make([]string, 0, len(appLangs))
	for _, code := range appLangs {
		if !slices.ContainsFunc(localeCfg.Disabled, func(d string) bool { return strings.EqualFold(d, code) }) {
			localeCodes = append(localeCodes, code)
		}
	}
	if len(localeCodes) > 0 {
		// The literal head of the route's path patterns is where the language
		// gets inserted - see insertLocale.
		filters.Request = append(filters.Request,
			localeForwardFilter(localeCodes, localeCfg, routing.PathPrefixes(r.Predicates)))
	}

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
	if frag := pageAgentFragment(r, localeCodes) + userButtonFragment(r, localeCodes); frag != "" {
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
		handler, err = buildProxy(r, filters, rt.defaultTimeouts)
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
	hasOverrides := r.API != nil && r.API.Security != nil && len(r.API.Security.Endpoints) > 0
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
	// Gates (ROUTE-04) go on LAST, so they sit outermost: what a route refuses
	// to carry is decided before the access rule reads a session, before a
	// modifier touches a header, and before a redirect sends the caller round
	// again. Refusing early is the whole point - an oversized body must not be
	// read to be turned away.
	handler = gateChain(filters.Gates, handler)
	cfg := store.CircuitBreaker{}
	if r.Breaker != nil {
		cfg = *r.Breaker
	}
	return compiledRoute{id: r.ID, name: r.Name, preds: preds, handler: handler, breaker: cfg,
		access: selectAccess, isUI: r.IsUI}, nil
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
		if target.Scheme != "http" && target.Scheme != "https" {
			return fmt.Errorf("bad upstream %q: scheme %q is not supported: only http and https", r.Upstream, target.Scheme)
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
			if !slices.Contains(store.IdentityFields, a.Field) {
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
		// The two that only mean something on a page: one puts the language in
		// the URL it serves, the other runs in it.
		if (mode == store.LocalePath || mode == store.LocaleScript) && !r.IsUI {
			return fmt.Errorf("locales mechanism %q is only allowed on UI routes: a service route serves no page", mode)
		}
		if mode == store.LocaleCustom && !headerNameOK.MatchString(lc.Header) {
			return fmt.Errorf("locales custom header %q is not allowed: letters, digits and - only", lc.Header)
		}
		if lc.Param != "" && !headerNameOK.MatchString(lc.Param) {
			return fmt.Errorf("locales query parameter %q is not allowed: letters, digits and - only", lc.Param)
		}
		for _, d := range lc.Disabled {
			if !headerNameOK.MatchString(d) {
				return fmt.Errorf("disabled locale %q is not allowed: letters, digits and - only", d)
			}
		}
	}
	if r.UI == nil {
		return nil
	}
	if s := r.UI.Scheme; s != nil {
		switch s.Mechanism {
		case "", "attribute", "class":
		default:
			return fmt.Errorf("scheme mechanism %q is not allowed: allowed mechanisms are \"\" (color-scheme only), attribute, class", s.Mechanism)
		}
		if s.Mechanism == "attribute" && !schemeTokenOK.MatchString(s.Attribute) {
			return fmt.Errorf("scheme attribute %q is not allowed: letters, digits, - and _ only", s.Attribute)
		}
		for _, v := range []string{s.Light, s.Dark} {
			if v != "" && !schemeTokenOK.MatchString(v) {
				return fmt.Errorf("scheme value %q is not allowed: letters, digits, - and _ only", v)
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
			if !slices.Contains(store.PageUserFields, field) {
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
	Roles      []string
}

// sessionIdentity resolves the caller for per-request injections and
// forwarding; ok is false without a completed session. A simulated identity
// (simulate.go) replaces the session wholesale.
func (rt *Router) sessionIdentity(req *http.Request) (identityData, bool) {
	if d, ok := simulatedIdentity(req.Context()); ok {
		return d, true
	}
	sess, err := rt.sm.Resolve(req.Context(), req)
	if err != nil || sess.Pending != "" {
		return identityData{}, false
	}
	u, err := rt.st.GetUserByID(req.Context(), sess.UserID)
	if err != nil || !u.Enabled {
		return identityData{}, false
	}
	d := identityData{UserID: u.ID, Username: u.Username, Fullname: u.Fullname,
		Email: u.Email, Timezone: u.Timezone, Locale: u.Locale}
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
		if only, ok := rt.onlyMembership(req, u.ID); ok {
			tenantID = only
		}
	}
	if tenantID != "" {
		d.TenantID = tenantID
		if t, err := rt.st.GetTenant(req.Context(), tenantID); err == nil {
			d.Tenant = t.Name
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
	return d, true
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

// onlyMembership returns the organisation a caller belongs to when there is
// exactly one enabled membership - the same rule sign-in applies. More than
// one is a choice nobody may make on their behalf, and none is none.
func (rt *Router) onlyMembership(req *http.Request, userID string) (string, bool) {
	all, err := rt.st.ListUserTenants(req.Context(), userID)
	if err != nil {
		return "", false
	}
	found := ""
	for _, t := range all {
		if !t.Enabled {
			continue
		}
		if found != "" {
			return "", false
		}
		found = t.TenantID
	}
	return found, found != ""
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
	return ""
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
		pr.Out.Header.Set("Accept-Language", promoteLocale(pr.In.Header.Get("Accept-Language"), loc))
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
	if isUI {
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
	}
	http.Error(w, refusalReason(a, who), http.StatusForbidden)
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
func (rt *Router) caller(req *http.Request, d identityData, ok bool) store.Caller {
	if !ok {
		return store.Caller{}
	}
	c := store.Caller{Authenticated: true, Username: d.Username, TenantID: d.TenantID, Roles: d.Roles}
	if ms, err := rt.st.ListUserTenants(req.Context(), d.UserID); err == nil {
		for _, m := range ms {
			if m.Enabled {
				c.Memberships = append(c.Memberships, m.TenantID)
			}
		}
	}
	return c
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
	eps := make([]compiledEP, 0, len(sec.Endpoints))
	for i, e := range sec.Endpoints {
		cp, err := routing.CompilePath(e.Path)
		if err != nil {
			return nil, fmt.Errorf("endpoint %d (%s %s): %w", i, e.Method, e.Path, err)
		}
		eps = append(eps, compiledEP{method: strings.ToUpper(e.Method), path: cp, gate: rt.accessGate(e.Access, isUI, next)})
	}
	strip := stripPrefixCount(filters)
	// The route's base Access is the default for any operation with no override
	// (an empty Access passes through, delegating to the upstream).
	routeGate := rt.accessGate(routeAccess, isUI, next)

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
			d, ok := rt.sessionIdentity(pr.In)
			if !ok {
				return
			}
			claims := identityClaims(cfg, routeName, d)
			var tok string
			var err error
			if signed {
				set := rt.currentSigning()
				if set == nil {
					return
				}
				tok, err = set.SignJWT(alg, claims)
			} else {
				tok, err = mintUnsignedJWT(claims)
			}
			if err != nil {
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
		d, ok := rt.sessionIdentity(pr.In)
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
	if scheme := r.UI.Scheme != nil && r.UI.Scheme.Select; scheme {
		s := r.UI.Scheme
		attrs += ` data-scheme="select"`
		if s.Mechanism != "" {
			attrs += fmt.Sprintf(` data-scheme-mechanism="%s" data-scheme-light="%s" data-scheme-dark="%s"`,
				s.Mechanism, s.Light, s.Dark)
			if s.Mechanism == "attribute" {
				attrs += fmt.Sprintf(` data-scheme-attribute="%s"`, s.Attribute)
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
	return `<script defer src="/meerkat/page.js"` + attrs + `></script>`
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
	if s := r.UI.Scheme; s != nil && s.Select {
		attrs += ` scheme="select"`
	}
	// The ROUTE's locale offer feeds the button's language submenu (codes are
	// validated BCP 47, HTML-safe; the component renders the endonyms).
	if len(localeCodes) > 0 {
		attrs += fmt.Sprintf(` languages="%s"`, strings.Join(localeCodes, ","))
	}
	return `<script defer src="/meerkat/user-button.js"></script>` +
		`<meerkat-user-button` + attrs + `></meerkat-user-button>`
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
		MaxIdleConnsPerHost:   8,
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

func buildProxy(r store.Route, cf routing.CompiledFilters, defaults store.RouteTimeouts) (http.Handler, error) {
	target, err := url.Parse(r.Upstream)
	if err != nil {
		return nil, fmt.Errorf("bad upstream %q: %w", r.Upstream, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("bad upstream %q: scheme and host required", r.Upstream)
	}

	connect, response := r.Timeouts.Durations(defaults)
	proxy := &httputil.ReverseProxy{
		Transport: cookieStrippingTransport{transportFor(connect, response)},
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetXForwarded()
			// Whatever the caller sent under this name goes: it tells the
			// service where it is published, so a caller who poses their own
			// makes the service write its links wherever they asked. Purged
			// unconditionally, even on a route that never sets it - a route
			// without strip-prefix must not carry a stranger's idea of its
			// own prefix either. Same reasoning as the identity headers.
			pr.Out.Header.Del(routing.ForwardedPrefixHeader)
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
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
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
		// and an internal spec read was authorized on the control plane.
		if _, ok := simulatedIdentity(req.Context()); ok || isSpecRead(req.Context()) {
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
