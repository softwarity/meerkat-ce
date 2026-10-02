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
// called has nothing to report. So this asks, on its own schedule:
//
//   - a service the runtime knows (internal/discovery: Swarm, Docker) is read
//     from its declared state - ready replicas above zero is up, zero is down.
//     No connection is made: the orchestrator already counted.
//   - anything else - an external host, a Kubernetes service (whose listing
//     carries no readiness, see discovery.fromKubernetes), a discovery that
//     could not be asked - gets a TCP connect with a short timeout. A connect
//     and nothing more: no HTTP request, so no authentication, no rate limit
//     and no line in the upstream's access log is spent on finding out.
//
// Never on the request path, never blocking it: one round every TargetInterval
// in the background, its results kept in memory and read by Health.
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

// Target states, as the console reads them.
const (
	TargetUp   = "up"
	TargetDown = "down"
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
	// or became known: the console's live channel hangs off it.
	Flipped func()
}

// DefaultTargetCheck asks the real runtime and the real network.
func DefaultTargetCheck(flipped func()) TargetCheck {
	return TargetCheck{
		Discover: discovery.Discover,
		Dial:     (&net.Dialer{Timeout: targetDialTimeout}).DialContext,
		Proxy:    http.ProxyFromEnvironment,
		Flipped:  flipped,
	}
}

// targetState is one route's verdict.
type targetState struct {
	State string
	Why   string
	At    int64
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

// WatchTargets runs the check every TargetInterval until ctx ends. The first
// round runs straight away: a console opened after a restart should not wait
// half a minute to see anything.
func (rt *Router) WatchTargets(ctx context.Context, tc TargetCheck) {
	t := time.NewTicker(TargetInterval)
	defer t.Stop()
	for {
		rt.CheckTargets(ctx, tc)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
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
			v = &targetState{State: TargetDown, Why: "not answering (circuit open)", At: now.Unix()}
		}
		next[p.id] = *v
	}

	rt.targets.mu.Lock()
	flipped := false
	for id, s := range next {
		if before, ok := rt.targets.by[id]; !ok || before.State != s.State {
			flipped = true
		}
	}
	rt.targets.by = next
	rt.targets.mu.Unlock()
	if flipped && tc.Flipped != nil {
		tc.Flipped()
	}
	return flipped
}

// planTarget decides how one route's target is found out.
func planTarget(id, upstream string, services []discovery.Service, source string,
	proxy func(*http.Request) (*url.URL, error), now time.Time) targetPlan {
	u, err := url.Parse(upstream)
	if err != nil || u.Hostname() == "" {
		return targetPlan{id: id, verdict: &targetState{State: TargetDown, Why: "the upstream cannot be read", At: now.Unix()}}
	}
	host := u.Hostname()
	// Swarm and Docker count their replicas, and that count is the answer.
	// Kubernetes is listed without readiness (it lives on Endpoints, which the
	// listing does not read), so its services are dialled like any other host:
	// a cluster IP with no ready pod behind it refuses the connection.
	if source != "kubernetes" {
		if s, ok := findService(services, host); ok {
			v := targetState{State: TargetUp, Why: fmt.Sprintf("%d ready in %s", s.Ready, sourceName(source)), At: now.Unix()}
			if s.Ready == 0 {
				v = targetState{State: TargetDown, Why: fmt.Sprintf("%d of %d ready", s.Ready, s.Wanted), At: now.Unix()}
			}
			return targetPlan{id: id, verdict: &v}
		}
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
