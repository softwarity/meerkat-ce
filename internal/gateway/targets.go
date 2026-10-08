package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/softwarity/meerkat/internal/discovery"
)

// The target check: is the service behind each route THERE at all (SVC-04).
//
// The breaker (breaker.go) watches real traffic, and that stays the verdict on
// whether a route works. What it cannot answer is the question an operator asks
// first on a quiet route: is the thing it points at even up? A route nobody has
// called has nothing to report. So this finds out:
//
//   - a service the runtime counts (internal/discovery: Swarm, Docker, and a
//     Kubernetes service whose pods this gateway may read) is read from its
//     replicas: all ready is up, some is degraded, none - or none wanted - is
//     down, with the images they run. No connection is made: the orchestrator
//     already counted. And no timer either: the runtime is WATCHED
//     (discovery.Watcher), and its events are what bring a new round.
//   - anything else - an external host, a service the runtime does not count,
//     a runtime that could not be asked - gets a TCP connect with a short
//     timeout, every TargetInterval: nothing announces an external host going
//     away. A connect and nothing more: no HTTP request, so no authentication,
//     no rate limit and no line in the upstream's access log is spent on
//     finding out. A gateway with no such target runs no timer at all.
//
// Never on the request path, never blocking it: rounds run in the background,
// their results kept in memory and read by Health.
//
// Per NODE, like the breaker. Each gateway asks from where it stands - its own
// networks, its own DNS - which is the answer that matters for the traffic it
// sends. Kept only while the process lives: a gateway that starts knows nothing
// until its first round, and says so by showing nothing.

// TargetInterval is how often the targets are checked. Thirty seconds: a
// service that goes away is seen within half a minute, and a hundred routes
// cost a hundred connects (fewer, deduplicated) twice a minute.
const TargetInterval = 30 * time.Second

// targetDialTimeout bounds one connect. Short on purpose: a target that takes
// longer than this to accept a connection is not one anybody would call up.
const targetDialTimeout = 2 * time.Second

// targetDials is how many connects run at once, so a round over many dead
// targets takes a few timeouts rather than one per route.
const targetDials = 8

// Target states, as the console reads them. Degraded is a counted service
// with some of its replicas ready, not all: it answers, on less than it was
// given.
const (
	TargetUp       = "up"
	TargetDegraded = "degraded"
	TargetDown     = "down"
)

// TargetCheck is what a round needs from the world. Every field is a function
// so the tests can answer for the runtime, the network and the environment.
type TargetCheck struct {
	// Discover asks the runtime what services exist. Nil, or a result naming
	// why nothing answered, means every target is dialled.
	Discover func(context.Context) discovery.Result
	// Dial opens a connection, and is closed straight away.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)
	// Proxy is what the upstream transports use (http.ProxyFromEnvironment).
	// When it names a proxy for a target, connecting to the target directly
	// would test a path no request takes - so the PROXY is what is dialled,
	// and the verdict says so.
	Proxy func(*http.Request) (*url.URL, error)
	// Flipped is told after a round in which a route's target changed state,
	// replicas or images, or became known: the console's live channel hangs
	// off it.
	Flipped func()
	// Changed receives when the runtime's answer changed: a new round. Nil
	// means the runtime is not watched.
	Changed <-chan struct{}
}

// DefaultTargetCheck reads the watched runtime and dials the real network.
func DefaultTargetCheck(w *discovery.Watcher, flipped func()) TargetCheck {
	return TargetCheck{
		Discover: func(context.Context) discovery.Result {
			if r, ok := w.Result(); ok {
				return r
			}
			return discovery.Result{Unavailable: "the runtime has not answered yet"}
		},
		Dial:    (&net.Dialer{Timeout: targetDialTimeout}).DialContext,
		Proxy:   http.ProxyFromEnvironment,
		Flipped: flipped,
		Changed: w.Changed(),
	}
}

// targetState is one route's verdict.
type targetState struct {
	State string
	Why   string
	At    int64
	// Replicas is set for a counted service, nil for anything dialled.
	Replicas *Replicas
}

// Replicas is a counted service's replicas, as the routes screen shows them.
type Replicas struct {
	Ready  int               `json:"ready"`
	Wanted int               `json:"wanted"`
	Images []discovery.Image `json:"images,omitempty"`
	Source string            `json:"source"`
}

// same says two verdicts would draw the same: the console is woken when the
// state, a count or an image moves, and not when only the time does.
func (a targetState) same(b targetState) bool {
	if a.State != b.State || (a.Replicas == nil) != (b.Replicas == nil) {
		return false
	}
	if a.Replicas == nil {
		return true
	}
	x, y := a.Replicas, b.Replicas
	if x.Ready != y.Ready || x.Wanted != y.Wanted || len(x.Images) != len(y.Images) {
		return false
	}
	for i := range x.Images {
		if x.Images[i] != y.Images[i] {
			return false
		}
	}
	return true
}

// targetStates holds the last round's verdicts, by route id.
type targetStates struct {
	mu sync.RWMutex
	by map[string]targetState
}

func newTargetStates() *targetStates { return &targetStates{by: map[string]targetState{}} }

func (t *targetStates) get(id string) (targetState, bool) {
	if t == nil {
		return targetState{}, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.by[id]
	return s, ok
}

// WatchTargets runs a round now, then on every change of the runtime and
// every reload of the routes, until ctx ends. Only while some target has to
// be dialled does a timer run as well, every TargetInterval: nothing tells
// this gateway that an external host went away.
func (rt *Router) WatchTargets(ctx context.Context, tc TargetCheck) {
	var ticker *time.Ticker
	var tick <-chan time.Time
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()
	for {
		_, dialled := rt.checkTargets(ctx, tc)
		switch {
		case dialled && ticker == nil:
			ticker = time.NewTicker(TargetInterval)
			tick = ticker.C
		case !dialled && ticker != nil:
			ticker.Stop()
			ticker, tick = nil, nil
		}
		select {
		case <-ctx.Done():
			return
		case <-tick:
		case <-tc.Changed:
		case <-rt.reloaded:
		}
	}
}

// targetPlan is what one route needs found out.
type targetPlan struct {
	id string
	// verdict is already decided (discovery answered, or the upstream cannot
	// be read); dial is the address to connect to otherwise.
	verdict *targetState
	dial    string
	// viaProxy says the address is a proxy's, which the verdict names.
	viaProxy bool
}

// CheckTargets runs one round and reports whether anything flipped. Exported
// for the wiring's first round and for tests; WatchTargets is the loop.
func (rt *Router) CheckTargets(ctx context.Context, tc TargetCheck) bool {
	flipped, _ := rt.checkTargets(ctx, tc)
	return flipped
}

// checkTargets runs one round, and also says whether anything was dialled.
func (rt *Router) checkTargets(ctx context.Context, tc TargetCheck) (bool, bool) {
	rt.mu.RLock()
	routes := rt.routes
	rt.mu.RUnlock()

	now := time.Now()
	var plans []targetPlan
	for _, c := range routes {
		if c.upstream != "" {
			plans = append(plans, targetPlan{id: c.id})
		}
	}
	// Discovery is asked only when there is something to ask it about: a
	// gateway whose routes all answer by themselves has no target to check.
	var services []discovery.Service
	source := ""
	if len(plans) > 0 && tc.Discover != nil {
		if res := tc.Discover(ctx); res.Unavailable == "" {
			services, source = res.Services, res.Source
		}
	}
	byID := map[string]compiledRoute{}
	for _, c := range routes {
		byID[c.id] = c
	}
	dials := map[string]struct{}{}
	for i := range plans {
		plans[i] = planTarget(plans[i].id, byID[plans[i].id].upstream, services, source, tc.Proxy, now)
		if plans[i].verdict == nil {
			dials[plans[i].dial] = struct{}{}
		}
	}

	// One connect per address, however many routes point at it, at most
	// targetDials at a time.
	dialed := map[string]error{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, targetDials)
	for addr := range dials {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			err := dialOnce(ctx, tc.Dial, addr)
			mu.Lock()
			dialed[addr] = err
			mu.Unlock()
		}(addr)
	}
	wg.Wait()

	next := make(map[string]targetState, len(plans))
	for _, p := range plans {
		v := p.verdict
		if v == nil {
			v = dialVerdict(dialed[p.dial], p.viaProxy, now)
		}
		// The breaker has the last word: traffic that fails is a target that
		// fails, whatever a connect or a replica count says.
		c := byID[p.id]
		if h := rt.breakers.of(p.id).health(c.breaker, now); h.State != circuitClosed {
			v = &targetState{State: TargetDown, Why: "not answering (circuit open)", At: now.Unix(), Replicas: v.Replicas}
		}
		next[p.id] = *v
	}

	rt.targets.mu.Lock()
	flipped := false
	for id, s := range next {
		if before, ok := rt.targets.by[id]; !ok || !before.same(s) {
			flipped = true
		}
	}
	rt.targets.by = next
	rt.targets.mu.Unlock()
	if flipped && tc.Flipped != nil {
		tc.Flipped()
	}
	return flipped, len(dials) > 0
}

// planTarget decides how one route's target is found out.
func planTarget(id, upstream string, services []discovery.Service, source string,
	proxy func(*http.Request) (*url.URL, error), now time.Time) targetPlan {
	u, err := url.Parse(upstream)
	if err != nil || u.Hostname() == "" {
		return targetPlan{id: id, verdict: &targetState{State: TargetDown, Why: "the upstream cannot be read", At: now.Unix()}}
	}
	host := u.Hostname()
	// A service the runtime counted: its replicas are the answer. One it did
	// not count (a Kubernetes service without a selector, or whose pods this
	// gateway may not read) is dialled like any other host.
	if s, ok := findService(services, host); ok && s.Counted {
		return targetPlan{id: id, verdict: countedVerdict(s, source, now)}
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	plan := targetPlan{id: id, dial: net.JoinHostPort(host, port)}
	// The transports honour HTTP_PROXY and friends, and they ask with the
	// request's http(s) scheme - an h2c target travels as http.
	if proxy != nil {
		asked := *u
		if asked.Scheme == SchemeH2C {
			asked.Scheme = "http"
		}
		if p, err := proxy(&http.Request{URL: &asked}); err == nil && p != nil && p.Hostname() != "" {
			pport := p.Port()
			if pport == "" {
				pport = "80"
				if p.Scheme == "https" {
					pport = "443"
				}
			}
			plan.dial, plan.viaProxy = net.JoinHostPort(p.Hostname(), pport), true
		}
	}
	return plan
}

// findService matches a host against what the runtime says resolves to each
// service - the names the console offers as upstreams. A Kubernetes-style
// fully qualified name is matched without its cluster domain.
func findService(services []discovery.Service, host string) (discovery.Service, bool) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	host = strings.TrimSuffix(host, ".cluster.local")
	for _, s := range services {
		// A service on no network this gateway shares resolves under no name
		// from here: a host that happens to match it is somebody else, and
		// the connect says what really happens.
		if !s.Reachable {
			continue
		}
		if strings.EqualFold(s.Name, host) {
			return s, true
		}
		for _, n := range s.Names {
			if strings.EqualFold(n, host) {
				return s, true
			}
		}
	}
	return discovery.Service{}, false
}

// countedVerdict reads a counted service: every replica ready is up, some is
// degraded, none is down - and none WANTED is down too, said as stopped: a
// service scaled to zero answers nobody, deliberately or not.
func countedVerdict(s discovery.Service, source string, now time.Time) *targetState {
	v := &targetState{At: now.Unix(),
		Replicas: &Replicas{Ready: s.Ready, Wanted: s.Wanted, Images: s.Images, Source: sourceName(source)}}
	switch {
	case s.Wanted == 0 && s.Ready == 0:
		v.State, v.Why = TargetDown, "stopped (0 of 0)"
	case s.Ready == 0:
		v.State, v.Why = TargetDown, fmt.Sprintf("0 of %d ready", s.Wanted)
	case s.Ready < s.Wanted:
		v.State, v.Why = TargetDegraded, fmt.Sprintf("%d of %d ready", s.Ready, s.Wanted)
	default:
		v.State, v.Why = TargetUp, fmt.Sprintf("%d of %d ready in %s", s.Ready, s.Wanted, sourceName(source))
	}
	return v
}

func sourceName(source string) string {
	switch source {
	case "swarm":
		return "Swarm"
	case "docker":
		return "Docker"
	case "kubernetes":
		return "Kubernetes"
	}
	return source
}

func dialOnce(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error), addr string) error {
	ctx, cancel := context.WithTimeout(ctx, targetDialTimeout)
	defer cancel()
	conn, err := dial(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// dialVerdict turns a connect into words an operator reads in a tooltip.
func dialVerdict(err error, viaProxy bool, now time.Time) *targetState {
	if err == nil {
		why := "answers"
		if viaProxy {
			why = "proxy answers"
		}
		return &targetState{State: TargetUp, Why: why, At: now.Unix()}
	}
	why := dialError(err)
	if viaProxy {
		why = "proxy: " + why
	}
	return &targetState{State: TargetDown, Why: why, At: now.Unix()}
}

func dialError(err error) string {
	var dns *net.DNSError
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.As(err, &dns):
		return "unknown host"
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded):
		return "no answer in " + targetDialTimeout.String()
	case errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH):
		return "unreachable"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "no answer in " + targetDialTimeout.String()
	}
	return err.Error()
}
