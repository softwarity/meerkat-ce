// Package metrics counts what passes through the gateway.
//
// WHAT IS COUNTED, AND WHERE IT IS READ. One set of counters feeds two very
// different readers, and the split is the whole design:
//
//   - The console's own screens read a RING of samples taken every few
//     seconds. They want a curve over the last hour, live, with no monitoring
//     stack to install - which is the promise the community edition makes.
//   - A Prometheus reads the raw totals through /metrics. It scrapes on its
//     own schedule and does its own differencing, so it wants counters that
//     only ever go up, never a window.
//
// The counters are therefore monotonic and the window is derived from them by
// a sampler, rather than the other way round. Deriving totals from a window
// would lose whatever fell off its end.
//
// THE HOT PATH. This gateway is on the path of every request, so recording one
// must cost an atomic add and nothing else: no map lookup, no string built, no
// allocation. Each route therefore OWNS its block of counters, handed to the
// request when the route is matched - the same shape the circuit breaker
// already uses.
//
// CARDINALITY. The labels are bounded by construction: a route id, a status
// class, a bucket index, and an operation TEMPLATE. Never a raw path, never a
// user, never an address. A path label on /orders/{id} is one time series per
// order, chosen by whoever sends the requests, and that is how a monitoring
// stack is brought down by the thing meant to watch it. Where no template was
// declared the gateway folds one out of the path's shape - bounded twice, by
// the fold and by a per-route budget. See internal/gateway/deduce.go.
package metrics

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Buckets are the latency boundaries, in seconds, shared by every route.
//
// Fixed and few: a histogram is one counter per bucket per route, so the
// number here is multiplied by the size of an installation's routing table.
// Twelve covers a gateway's useful range - a millisecond is noise on a network
// hop, and past ten seconds the only question left is whether it answered.
var Buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// counters is one thing's numbers. A route has a block; so does an operation
// inside one, and they are counted identically - which is the point of having
// the block rather than two near-copies that drift.
type counters struct {
	// Requests by status class: index 0 is 1xx, 4 is 5xx. Anything outside
	// 100..599 lands in 5: a handler that wrote nonsense is not nothing.
	byClass [6]atomic.Uint64
	// The histogram: one counter per bucket, cumulative at read time.
	buckets [12]atomic.Uint64
	// Sum of observed seconds, in microseconds to stay whole - a float summed
	// atomically would need a CAS loop on the hot path.
	sumMicros atomic.Uint64
	// Upstream failures, by kind. See FailureKind.
	failures [4]atomic.Uint64
}

// Route is one route's counters. Held BY the compiled route, so the hot path
// reaches it without looking anything up.
type Route struct {
	// ID and Name are what a reader sees. The id is the series identity, the
	// name is what a human recognises - a route renamed keeps its curve.
	ID   string
	Name string
	counters
}

// Endpoint is one OPERATION's counters, inside a route.
//
// The unit is the TEMPLATE and never the path: "/orders/{id}" is one series
// and "/orders/12345" is one per order, which is how a monitoring stack is
// brought down by the thing meant to watch it.
//
// A template comes from one of two places, and which one is part of the
// answer. DECLARED: a deposited OpenAPI spec, the per-endpoint rules somebody
// wrote, or a spec the control plane resolved from the service - somebody
// wrote it down, so it is exact. DEDUCED: the shape of a path this gateway
// saw, with the segments that look like identifiers folded away. Deduction is
// bounded (see internal/gateway/deduce.go) and it is sometimes WRONG - the
// year in /files/2024/report is not an id - so it is marked, never presented
// as the same kind of fact.
type Endpoint struct {
	RouteID string
	Method  string
	// Path is the operation's template, in the spec's own coordinates.
	Path string
	// Deduced says this template was inferred from traffic rather than read
	// from something somebody wrote.
	Deduced bool
	counters
	// hist is a coarse history - one point a minute, and only for the minutes
	// something happened - so this endpoint can answer for the same PERIOD the
	// screen's window covers rather than for the life of the process. See
	// history.go, which is also where it is said why it is coarse.
	hist []point
}

// FailureKind says what went wrong between the gateway and a service. Bounded
// on purpose: an error string as a label is a cardinality bomb, and the string
// itself belongs in the log, where a person can read it.
type FailureKind int

const (
	// FailConnect means the service was not reached at all.
	FailConnect FailureKind = iota
	// FailTimeout means it was reached and did not answer in time.
	FailTimeout
	// FailRefused means the circuit was open, so nothing was even attempted.
	FailRefused
	// FailUpstream means it answered, with 502, 503 or 504.
	FailUpstream
)

// Observe records one finished request. The only thing the hot path calls.
func (r *counters) Observe(status int, took time.Duration) {
	if r == nil {
		return
	}
	class := status / 100
	if class < 1 || class > 5 {
		class = 5
	}
	r.byClass[class].Add(1)
	r.sumMicros.Add(uint64(took.Microseconds()))
	secs := took.Seconds()
	for i, b := range Buckets {
		if secs <= b {
			r.buckets[i].Add(1)
			return
		}
	}
	// Past the last boundary: the overflow bucket, which is what makes the
	// histogram able to say "slower than ten seconds" rather than losing it.
	r.buckets[len(Buckets)].Add(1)
}

// Failed records why a request never got a real answer from the service.
func (r *counters) Failed(kind FailureKind) {
	if r == nil || int(kind) >= len(r.failures) {
		return
	}
	r.failures[kind].Add(1)
}

// RouteSnapshot is a route's counters at one instant, as plain numbers.
type RouteSnapshot struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	ByClass  [6]uint64 `json:"byClass"`
	Buckets  []uint64  `json:"buckets"`
	SumSecs  float64   `json:"sumSecs"`
	Failures [4]uint64 `json:"failures"`
}

// Total is every request this route answered, whatever the outcome.
func (s RouteSnapshot) Total() uint64 {
	var n uint64
	for _, c := range s.ByClass {
		n += c
	}
	return n
}

func (r *counters) read() RouteSnapshot {
	out := RouteSnapshot{Buckets: make([]uint64, len(r.buckets))}
	for i := range r.byClass {
		out.ByClass[i] = r.byClass[i].Load()
	}
	for i := range r.buckets {
		out.Buckets[i] = r.buckets[i].Load()
	}
	for i := range r.failures {
		out.Failures[i] = r.failures[i].Load()
	}
	out.SumSecs = float64(r.sumMicros.Load()) / 1e6
	return out
}

func (r *Route) snapshot() RouteSnapshot {
	out := r.read()
	out.ID, out.Name = r.ID, r.Name
	return out
}

// EndpointSnapshot is one operation over a PERIOD - not at an instant, which
// is the difference from a route's snapshot and the whole point of it.
//
// Three numbers, deliberately. A ranking is read on how much, how much of it
// went wrong, and how long it took; a status-class breakdown and a
// twelve-bucket histogram per endpoint would be what the history has to keep,
// and the finer analysis is what a Prometheus is for.
type EndpointSnapshot struct {
	RouteID string `json:"routeId"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	// Deduced marks a template inferred from traffic rather than declared.
	// A reader has to be able to tell the two apart: one is what the service
	// says it exposes, the other is this gateway's guess at the shape.
	Deduced  bool    `json:"deduced,omitempty"`
	Requests uint64  `json:"requests"`
	Errors   uint64  `json:"errors"`
	SumSecs  float64 `json:"sumSecs"`
}

// Registry holds every route's counters and the gateway-wide ones.
//
// Routes come and go with each reload, and a route DELETED keeps its counters
// until the next sample: a curve that lost its last minute because somebody
// edited a route is a curve nobody trusts.
type Registry struct {
	// since is when these counters started, which is what makes a running
	// total readable: "9 300 requests" says nothing until a reader knows over
	// what. The window has its own interval; the totals have this.
	since time.Time

	mu     sync.RWMutex
	routes map[string]*Route
	// Operations, keyed by route, method and template. Resolved at COMPILE
	// time like the routes are, so the request path only ever holds a pointer.
	endpoints map[string]*Endpoint

	// rolledAt is when the endpoint histories last advanced. Under mu with
	// them, so a minute is a minute for all of them at once.
	rolledAt time.Time

	inFlight atomic.Int64
	// Sign-ins, by outcome. Gateway-wide: they do not belong to a route.
	logins  atomic.Uint64
	refused atomic.Uint64
	// Requests that matched no route at all. A rising number is a
	// misconfiguration somebody should see, and it belongs to no route.
	unmatched atomic.Uint64
}

// NewRegistry returns an empty registry. One per gateway.
func NewRegistry() *Registry {
	return &Registry{since: time.Now(), routes: map[string]*Route{}, endpoints: map[string]*Endpoint{}}
}

// Since is when this node started counting.
func (reg *Registry) Since() time.Time { return reg.since }

// For returns the counters of a route, creating them on first sight. Called at
// COMPILE time, never per request: the compiled route keeps the pointer.
func (reg *Registry) For(id, name string) *Route {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	r, ok := reg.routes[id]
	if !ok {
		r = &Route{ID: id, Name: name}
		reg.routes[id] = r
	}
	// A rename must not start a new series: same id, same counters, new label.
	r.Name = name
	return r
}

// Endpoint returns one DECLARED operation's counters, creating them on first
// sight. Called at COMPILE time and when the control plane pushes, never per
// request.
//
// A template already deduced becomes declared here, KEEPING its counters: the
// spec arriving ten minutes after the traffic must not restart the series it
// has been filling. That is the whole reason the flag lives on the counters
// rather than being decided at read time.
func (reg *Registry) Endpoint(routeID, method, path string) *Endpoint {
	return reg.endpoint(routeID, method, path, false)
}

// Deduced returns the counters of a template this gateway inferred. Called on
// the request path, but only for a route that has nothing declared covering
// it, and only until that route's budget is spent.
func (reg *Registry) Deduced(routeID, method, path string) *Endpoint {
	return reg.endpoint(routeID, method, path, true)
}

func (reg *Registry) endpoint(routeID, method, path string, deduced bool) *Endpoint {
	key := routeID + " " + method + " " + path
	reg.mu.Lock()
	defer reg.mu.Unlock()
	e, ok := reg.endpoints[key]
	switch {
	case !ok:
		e = &Endpoint{RouteID: routeID, Method: method, Path: path, Deduced: deduced}
		reg.endpoints[key] = e
	case !deduced:
		// Promoted, under the same lock the snapshot reads it under.
		e.Deduced = false
	}
	return e
}

// Started and Finished bracket a request for the in-flight gauge, which is the
// one number that says "saturated" rather than "busy".
func (reg *Registry) Started() { reg.inFlight.Add(1) }

// Unmatched records a request no route took.
func (reg *Registry) Unmatched() { reg.unmatched.Add(1) }

// Finished is the other half of Started.
func (reg *Registry) Finished() { reg.inFlight.Add(-1) }

// SignIn records an authentication attempt. Failures are what a brute-force
// attempt looks like from here.
func (reg *Registry) SignIn(ok bool) {
	if ok {
		reg.logins.Add(1)
		return
	}
	reg.refused.Add(1)
}

// Snapshot is everything the registry holds, at one instant.
type Snapshot struct {
	At        time.Time       `json:"at"`
	Routes    []RouteSnapshot `json:"routes"`
	InFlight  int64           `json:"inFlight"`
	Logins    uint64          `json:"logins"`
	Refused   uint64          `json:"refused"`
	Unmatched uint64          `json:"unmatched"`
}

// Snapshot reads every counter. Walked by the sampler, once per interval,
// so the request path never does.
func (reg *Registry) Snapshot() Snapshot {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	out := Snapshot{
		At:        time.Now(),
		Routes:    make([]RouteSnapshot, 0, len(reg.routes)),
		InFlight:  reg.inFlight.Load(),
		Logins:    reg.logins.Load(),
		Refused:   reg.refused.Load(),
		Unmatched: reg.unmatched.Load(),
	}
	for _, r := range reg.routes {
		out.Routes = append(out.Routes, r.snapshot())
	}
	return out
}

// Origin is the zero instant, and asking Operations for it asks for everything
// since the process started - which is what a scraper wants, since it does its
// own differencing. Named rather than written as a bare time.Time{} at the
// call sites: the meaning is the point, not the value.
var Origin time.Time

// Operations is every operation over the period starting at from, for the
// ranking. A zero from asks for everything since the process started.
//
// A PERIOD and not a running total, so that opening a route on the screen
// answers over the same window the row above it covers - two clocks on one
// table is a table whose lines do not add up. It is not the sampled window
// though: see history.go for what these endpoints keep instead, and why it is
// deliberately coarser than a curve.
func (reg *Registry) Operations(from time.Time) []EndpointSnapshot {
	reg.mu.RLock()
	out := make([]EndpointSnapshot, 0, len(reg.endpoints))
	for _, e := range reg.endpoints {
		requests, errors, secs, _ := e.since(from)
		out = append(out, EndpointSnapshot{
			RouteID: e.RouteID, Method: e.Method, Path: e.Path, Deduced: e.Deduced,
			Requests: requests, Errors: errors, SumSecs: secs,
		})
	}
	reg.mu.RUnlock()
	// Costliest first, and never map order. Two reasons: a reader who takes
	// only the head takes the half that matters, and a list that reshuffles
	// between two reads of the same numbers is a list nobody can compare.
	sort.Slice(out, func(i, j int) bool {
		if out[i].SumSecs != out[j].SumSecs {
			return out[i].SumSecs > out[j].SumSecs
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}
