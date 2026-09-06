package gateway

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/openapi"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// Which OPERATION a request was, so a slow endpoint can be named.
//
// The unit is the template - "/orders/{id}" - and never the path. A path label
// is one series per order, which is the mistake this whole package is written
// to avoid. So an operation only exists here when the gateway can name it
// WITHOUT guessing, from templates somebody else already wrote:
//
//   - the per-endpoint security rules (RBAC-07), which name a method and a
//     template each, and a deposited OpenAPI spec (SVC-06) - both held in the
//     database, so the router reads them while it compiles;
//   - a spec the route fetches from its upstream, which the CONTROL PLANE
//     resolves and pushes in (see SetOperations). Fetching it here would be a
//     network call per route on every reload, and a reload that hangs because
//     a service is slow is a gateway that stops taking configuration.
//
// A route with none of those has its requests named by SHAPE instead, folded
// and capped - see deduce.go, which is also where it is said why counting raw
// paths would be a denial of service rather than a shortcut. Those lines are
// marked deduced everywhere they are shown: a guess and a declaration are two
// different kinds of fact.
type endpointIndex struct {
	// Buckets by method, then by how many segments the template has. A request
	// therefore compares against the handful of operations that could possibly
	// match rather than against all of them.
	//
	// This matters: matching one request against two hundred templates on
	// every request is the same shape as the per-route path split that made
	// selection allocate once per route - fixed one level up, and not worth
	// reintroducing one level down.
	byMethod map[string]map[int][]compiledOp
	// tails are the templates ending in "**", which match any length and so
	// cannot be bucketed by segment count.
	tails map[string][]compiledOp
	// strip is how many leading segments the route removes before the upstream
	// sees the path, so an inbound path maps back to the spec's coordinates -
	// the same mapping the endpoint guard makes (see endpointGuard).
	strip int
	// seen dedupes while the index is built: two sources naming the same
	// operation must not count the same request twice.
	seen  map[string]bool
	empty bool
}

type compiledOp struct {
	path     routing.CompiledPath
	counters *metrics.Endpoint
}

// Operation is one method-and-template pair, as somebody else discovered it.
type Operation struct{ Method, Path string }

// opsSlot is a route's operations, and the only thing its compiled form keeps
// of them: a pointer the request path reads and everybody else swaps.
//
// The indirection is what lets the two sources arrive at different times. The
// router compiles what the database holds; the control plane pushes what it
// resolved from the service minutes later, on its own schedule. Neither waits
// for the other, and neither recompiles a route to be heard.
type opsSlot struct {
	index atomic.Pointer[endpointIndex]

	// Fixed for the life of the slot, so the request path reads them without
	// taking anything.
	reg     *metrics.Registry
	routeID string

	mu     sync.Mutex
	strip  int
	base   []Operation // what compiling the route found
	pushed []Operation // what the control plane resolved

	// What this route INVENTED, for the requests nothing declared covers.
	// Nested by method so a lookup needs no key built, and capped - see
	// deduce.go for why counting raw paths is a denial of service.
	dedMu    sync.RWMutex
	deduced  map[string]map[string]*metrics.Endpoint
	dedOther *metrics.Endpoint
	dedCount int
	dedFull  bool
}

// of returns the counters for the operation this request is.
//
// Declared first, always: a template somebody wrote beats one this gateway
// guessed at, and a route with a spec never grows a second, deduced line for
// an operation the spec already names. Only what the spec does not cover is
// deduced - which for a route with no spec at all is everything.
func (s *opsSlot) of(r *http.Request) *metrics.Endpoint {
	if s == nil {
		return nil
	}
	idx := s.index.Load()
	if e := idx.of(r); e != nil {
		return e
	}
	strip := 0
	if idx != nil {
		strip = idx.strip
	}
	return s.deduce(r, strip)
}

// setBase replaces what compiling the route found, keeping whatever the
// control plane had already pushed: a reload must not blank the endpoints
// until the next refresh comes round.
func (s *opsSlot) setBase(strip int, ops []Operation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strip, s.base = strip, ops
	s.rebuild()
}

// setPushed replaces what the control plane resolved, keeping the base.
func (s *opsSlot) setPushed(ops []Operation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushed = ops
	s.rebuild()
}

// rebuild compiles both sources into one index and swaps it in. Called under
// s.mu; the swap itself is what a request sees, and it is atomic.
func (s *opsSlot) rebuild() {
	idx := newEndpointIndex(s.strip)
	for _, op := range s.base {
		idx.add(s.reg, s.routeID, op.Method, op.Path)
	}
	for _, op := range s.pushed {
		idx.add(s.reg, s.routeID, op.Method, op.Path)
	}
	s.index.Store(&idx)
}

// SetOperations tells the router what a route's operations are, as the control
// plane resolved them.
//
// Pushed in rather than fetched here, and that is the whole point: resolving a
// route's spec means reading it from the database or FETCHING IT from the
// service, and a network call on the reload path is a reload that hangs
// because a service is slow. The control plane already knows how to read a
// spec, either kind - see (*admin.API).RefreshOperations.
func (rt *Router) SetOperations(routeID string, ops []Operation) {
	rt.opsFor(routeID).setPushed(ops)
}

// opsFor is a route's slot, created on first sight and kept across reloads:
// the compiled route is rebuilt every time, the slot is not, which is what
// makes a push outlive the next configuration change.
func (rt *Router) opsFor(routeID string) *opsSlot {
	rt.opsMu.Lock()
	defer rt.opsMu.Unlock()
	if rt.opsSlots == nil {
		rt.opsSlots = map[string]*opsSlot{}
	}
	slot, ok := rt.opsSlots[routeID]
	if !ok {
		slot = &opsSlot{reg: rt.metrics, routeID: routeID}
		rt.opsSlots[routeID] = slot
	}
	return slot
}

// pruneOps drops the slots of routes that no longer exist. Called at the end
// of a reload, with the ids that survived it.
func (rt *Router) pruneOps(live map[string]bool) {
	rt.opsMu.Lock()
	defer rt.opsMu.Unlock()
	for id := range rt.opsSlots {
		if !live[id] {
			delete(rt.opsSlots, id)
		}
	}
}

func newEndpointIndex(strip int) endpointIndex {
	return endpointIndex{
		byMethod: map[string]map[int][]compiledOp{},
		tails:    map[string][]compiledOp{},
		strip:    strip,
		seen:     map[string]bool{},
		empty:    true,
	}
}

// add compiles one template into the index. A template that will not compile
// is skipped rather than approximated: the ROUTE's counters still hold the
// request, so nothing is lost but the detail.
func (idx *endpointIndex) add(reg *metrics.Registry, routeID, method, path string) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "*"
	}
	if path == "" || idx.seen[method+" "+path] {
		return
	}
	cp, err := routing.CompilePath(path)
	if err != nil {
		return
	}
	idx.seen[method+" "+path] = true
	idx.empty = false
	op := compiledOp{path: cp, counters: reg.Endpoint(routeID, method, path)}
	if strings.HasSuffix(path, "/**") {
		idx.tails[method] = append(idx.tails[method], op)
		return
	}
	n := routing.CountSegments(path)
	if idx.byMethod[method] == nil {
		idx.byMethod[method] = map[int][]compiledOp{}
	}
	idx.byMethod[method][n] = append(idx.byMethod[method][n], op)
}

// baseOperations is what compiling a route can name on its own, from what the
// database already holds - no network call, whatever the route declares.
func baseOperations(r store.Route, deposited []byte) []Operation {
	var out []Operation
	if len(deposited) > 0 {
		if spec, err := openapi.Parse(deposited); err == nil {
			for _, op := range spec.Operations {
				out = append(out, Operation{Method: op.Method, Path: op.Path})
			}
		}
	}
	// Security is a POINTER on the route: a route with an api block and no
	// per-endpoint rules has none, and reaching through it is a nil dereference
	// on the reload path - which takes the gateway down, not one screen.
	if r.API != nil && r.API.Security != nil {
		for _, e := range r.API.Security.Endpoints {
			out = append(out, Operation{Method: e.Method, Path: e.Path})
		}
	}
	return out
}

// of returns the counters for the operation this request is, or nil.
//
// nil is a normal answer, not a failure: a route with no spec has no
// operations, and a path the spec does not describe is a request the gateway
// cannot name. Either way the ROUTE's own counters already hold it.
func (idx *endpointIndex) of(r *http.Request) *metrics.Endpoint {
	if idx == nil || idx.empty {
		return nil
	}
	path := routing.StripSegments(r.URL.Path, idx.strip)
	n := routing.CountSegments(path)
	for _, method := range [2]string{r.Method, "*"} {
		if bucket, ok := idx.byMethod[method]; ok {
			for i := range bucket[n] {
				if bucket[n][i].path.Match(path) {
					return bucket[n][i].counters
				}
			}
		}
		for i := range idx.tails[method] {
			if idx.tails[method][i].path.Match(path) {
				return idx.tails[method][i].counters
			}
		}
	}
	return nil
}
