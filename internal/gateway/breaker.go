package gateway

import (
	"net/http"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// The circuit breaker (ROUTE-09), and the state the console reads to say
// whether a route is answering (SVC-04).
//
// Bounding the wait (ROUTE-07) stops one slow upstream from holding a
// connection forever. It does not stop the gateway from sending it a thousand
// more requests that will each wait their own bound - which is how a service
// that is merely down takes the gateway's own capacity with it, and how it
// comes back to a stampede the moment it recovers.
//
// So: after N consecutive failures a route stops calling, answers the
// unavailable page straight away, and tries ONE request after a cool-down. If
// that one works the circuit closes; if it fails, the cool-down starts again.
//
// What counts as a failure is deliberately narrow. A transport error, and a
// 502/503/504 - the answers a gateway itself produces when it could not reach
// or could not wait. NOT a 500: a service answering 500 is a service that is
// up and has a bug, and taking it out of rotation would turn one broken
// endpoint into a whole route nobody can reach, including the endpoints that
// work.
//
// The state is per node. Two gateways each keep their own opinion of an
// upstream, which is the right one to keep: they may genuinely differ - a
// network path, a DNS answer, a sidecar - and a shared verdict would let one
// node's bad minute close a circuit for everybody. The cost is that a service
// coming back is discovered N times rather than once, which is a probe each.

// breakerState is what a route's circuit is doing right now.
type breakerState string

const (
	circuitClosed breakerState = "closed" // calls go through
	circuitOpen   breakerState = "open"   // calls are refused, waiting out the cool-down
	circuitProbe  breakerState = "probe"  // one call is allowed through to find out
)

// breaker holds one route's circuit.
type breaker struct {
	mu sync.Mutex
	// failures counts CONSECUTIVE failures; any success resets it, because
	// what this is looking for is a service that stopped answering, not one
	// that fails occasionally under load.
	failures int
	openedAt time.Time
	probing  bool
	// The last outcome, kept for the console: an operator asking "is this
	// route working" wants the reason, not a colour.
	lastErr    string
	lastStatus int
	lastAt     time.Time
	okAt       time.Time
}

// breakers holds them per route id. A route that is deleted leaves its entry
// behind until the next reload sweeps it - see forget.
type breakers struct {
	mu sync.RWMutex
	by map[string]*breaker
}

func newBreakers() *breakers { return &breakers{by: map[string]*breaker{}} }

func (b *breakers) of(routeID string) *breaker {
	b.mu.RLock()
	br, ok := b.by[routeID]
	b.mu.RUnlock()
	if ok {
		return br
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if br, ok := b.by[routeID]; ok {
		return br
	}
	br = &breaker{}
	b.by[routeID] = br
	return br
}

// forget drops the circuits of routes that no longer exist. Called at reload:
// a deleted route's opinion has nobody to be about, and an id that comes back
// is a new route which deserves a clean slate.
func (b *breakers) forget(alive map[string]bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id := range b.by {
		if !alive[id] {
			delete(b.by, id)
		}
	}
}

// state reports what the circuit is doing, and whether THIS call may go
// through. A probe is handed to exactly one caller: the others keep getting
// the unavailable page, or the whole point of a cool-down is lost.
func (br *breaker) take(cfg store.CircuitBreaker, now time.Time) (breakerState, bool) {
	trip, cool := cfg.Settings()
	br.mu.Lock()
	defer br.mu.Unlock()
	if br.failures < trip {
		return circuitClosed, true
	}
	if now.Sub(br.openedAt) < cool {
		return circuitOpen, false
	}
	if br.probing {
		return circuitOpen, false
	}
	br.probing = true
	return circuitProbe, true
}

// succeeded closes the circuit: whatever was wrong is not any more.
func (br *breaker) succeeded(status int, now time.Time) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.failures, br.probing = 0, false
	br.lastErr, br.lastStatus, br.lastAt, br.okAt = "", status, now, now
}

// failed counts one, and starts the cool-down again if this was the probe.
func (br *breaker) failed(status int, reason string, now time.Time) {
	br.mu.Lock()
	defer br.mu.Unlock()
	br.failures++
	br.probing = false
	br.openedAt = now
	br.lastErr, br.lastStatus, br.lastAt = reason, status, now
}

// health is what the console reads (SVC-04).
type health struct {
	State      breakerState `json:"state"`
	Failures   int          `json:"failures"`
	LastStatus int          `json:"lastStatus,omitempty"`
	LastError  string       `json:"lastError,omitempty"`
	LastAt     int64        `json:"lastAt,omitempty"`
	LastOKAt   int64        `json:"lastOkAt,omitempty"`
}

func (br *breaker) health(cfg store.CircuitBreaker, now time.Time) health {
	trip, cool := cfg.Settings()
	br.mu.Lock()
	defer br.mu.Unlock()
	h := health{
		State: circuitClosed, Failures: br.failures,
		LastStatus: br.lastStatus, LastError: br.lastErr,
	}
	if !br.lastAt.IsZero() {
		h.LastAt = br.lastAt.Unix()
	}
	if !br.okAt.IsZero() {
		h.LastOKAt = br.okAt.Unix()
	}
	if br.failures >= trip {
		h.State = circuitOpen
		if now.Sub(br.openedAt) >= cool && !br.probing {
			h.State = circuitProbe
		}
	}
	return h
}

// failedStatus says whether an answer counts against the upstream. See the
// note at the top for why 500 does not.
func failedStatus(code int) bool {
	return code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}

// watched wraps a ResponseWriter to learn what status was written, and nothing
// else. It forwards Flush and Hijack: a proxy needs the first for streaming
// and the second for an upgrade, and a wrapper that swallows either turns
// server-sent events into silence and a websocket into a 500.
type watched struct {
	http.ResponseWriter
	status int
}

func (w *watched) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *watched) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *watched) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *watched) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Health is what the console reads to say whether a route is answering
// (SVC-04, ROUTE-11).
//
// It costs NOTHING to collect: the breaker already knows, because it watches
// every answer to decide whether to keep calling. A separate prober would have
// been a second opinion, formed on traffic nobody sent, about a path the real
// requests may not even take.
//
// A route with no breaker still reports: the circuit is what DECIDES, the
// watching is what OBSERVES, and an operator asking "does this route work"
// deserves an answer whether or not somebody armed a breaker on it.
func (rt *Router) Health() map[string]RouteHealth {
	rt.mu.RLock()
	routes := rt.routes
	rt.mu.RUnlock()

	now := time.Now()
	out := make(map[string]RouteHealth, len(routes))
	for _, c := range routes {
		br := rt.breakers.of(c.id)
		h := br.health(c.breaker, now)
		out[c.id] = RouteHealth{
			State:      string(h.State),
			Armed:      c.breaker.Enabled,
			Failures:   h.Failures,
			LastStatus: h.LastStatus,
			LastError:  h.LastError,
			LastAt:     h.LastAt,
			LastOKAt:   h.LastOKAt,
		}
	}
	return out
}

// RouteHealth is one route's, as the control plane hands it over.
type RouteHealth struct {
	// State is closed, open or probe. Without a breaker it is always closed -
	// nothing is being refused - and the counters below still say what the
	// last answers looked like.
	State string `json:"state"`
	// Armed says whether a breaker is actually on. A route with none can still
	// be failing, and a console that showed the same thing for "healthy" and
	// "nobody is watching" would be worse than showing nothing.
	Armed      bool   `json:"armed"`
	Failures   int    `json:"failures,omitempty"`
	LastStatus int    `json:"lastStatus,omitempty"`
	LastError  string `json:"lastError,omitempty"`
	LastAt     int64  `json:"lastAt,omitempty"`
	LastOKAt   int64  `json:"lastOkAt,omitempty"`
}
