package gateway

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// fakeNet answers connects by address, and counts them.
type fakeNet struct {
	mu     sync.Mutex
	errs   map[string]error
	dialed map[string]int
}

func (f *fakeNet) dial(_ context.Context, _, addr string) (net.Conn, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dialed == nil {
		f.dialed = map[string]int{}
	}
	f.dialed[addr]++
	if err, ok := f.errs[addr]; ok {
		return nil, err
	}
	c, s := net.Pipe()
	_ = s.Close()
	return c, nil
}

func swarmWith(services ...discovery.Service) func(context.Context) discovery.Result {
	return func(context.Context) discovery.Result {
		return discovery.Result{Source: "swarm", Services: services}
	}
}

func noProxy(*http.Request) (*url.URL, error) { return nil, nil }

// Every case the heart can be in, one router, one round.
func TestTheTargetCheckSaysWhetherEachTargetIsThere(t *testing.T) {
	redirect := pathRoute("redir", "redirect", 7, "/old/**", "",
		routing.Spec{Type: "redirect", Args: map[string]any{"location": "/new", "status": 301}})
	off := pathRoute("off", "off", 8, "/off/**", "http://off:80")
	off.Enabled = false
	rt := newRouter(t,
		pathRoute("ready", "ready", 1, "/a/**", "http://api:8080"),
		pathRoute("zero", "zero", 2, "/b/**", "http://worker:9000"),
		pathRoute("ext-ok", "ext-ok", 3, "/c/**", "https://billing.acme.example"),
		pathRoute("ext-ko", "ext-ko", 4, "/d/**", "http://10.0.0.9:7000"),
		pathRoute("ext-ok-2", "ext-ok-2", 5, "/e/**", "https://billing.acme.example/v2"),
		pathRoute("nohost", "nohost", 6, "/f/**", "http://nowhere.invalid"),
		redirect, off,
	)
	fn := &fakeNet{errs: map[string]error{
		"10.0.0.9:7000":      &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
		"nowhere.invalid:80": &net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "nowhere.invalid"}},
	}}
	flips := 0
	tc := TargetCheck{
		Discover: swarmWith(
			discovery.Service{Name: "stack_api", Names: []string{"api", "stack_api"}, Ready: 3, Wanted: 3, Reachable: true, Counted: true},
			discovery.Service{Name: "stack_worker", Names: []string{"worker", "stack_worker"}, Ready: 0, Wanted: 2, Reachable: true, Counted: true},
		),
		Dial: fn.dial, Proxy: noProxy, Flipped: func() { flips++ },
	}
	if !rt.CheckTargets(context.Background(), tc) || flips != 1 {
		t.Fatalf("a first round that learned everything must say so once, got %d", flips)
	}
	h := rt.Health()
	want := map[string][2]string{
		"ready":    {TargetUp, "3 of 3 ready in Swarm"},
		"zero":     {TargetDown, "0 of 2 ready"},
		"ext-ok":   {TargetUp, "answers"},
		"ext-ok-2": {TargetUp, "answers"},
		"ext-ko":   {TargetDown, "connection refused"},
		"nohost":   {TargetDown, "unknown host"},
	}
	for id, w := range want {
		if h[id].Target != w[0] || h[id].TargetWhy != w[1] || h[id].TargetAt == 0 {
			t.Errorf("%s: %q %q at %d, want %q %q", id, h[id].Target, h[id].TargetWhy, h[id].TargetAt, w[0], w[1])
		}
	}
	if h["redir"].Target != "" {
		t.Errorf("a route that answers by itself has no target, got %q", h["redir"].Target)
	}
	if _, listed := h["off"]; listed {
		t.Error("a disabled route is reported")
	}
	// Discovery answered for api and worker: nothing was dialled for them.
	// And one connect per address, however many routes share it.
	if fn.dialed["api:8080"] != 0 || fn.dialed["worker:9000"] != 0 {
		t.Errorf("a discovered service was dialled: %v", fn.dialed)
	}
	if fn.dialed["billing.acme.example:443"] != 1 {
		t.Errorf("two routes on one address cost %d connects, want 1", fn.dialed["billing.acme.example:443"])
	}

	// Same answers again: nothing flipped, nobody is woken.
	if rt.CheckTargets(context.Background(), tc) || flips != 1 {
		t.Errorf("a round that changed nothing woke the screen (%d)", flips)
	}

	// The external one goes away: that is a flip.
	fn.mu.Lock()
	delete(fn.errs, "10.0.0.9:7000")
	fn.errs["billing.acme.example:443"] = &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	fn.mu.Unlock()
	if !rt.CheckTargets(context.Background(), tc) || flips != 2 {
		t.Errorf("a target going down did not wake the screen (%d)", flips)
	}
	h = rt.Health()
	if h["ext-ok"].Target != TargetDown || h["ext-ko"].Target != TargetUp {
		t.Errorf("after the flip: %+v / %+v", h["ext-ok"], h["ext-ko"])
	}
}

// No runtime to ask: every target is dialled, the discovered names included.
func TestWithoutDiscoveryEveryTargetIsDialled(t *testing.T) {
	rt := newRouter(t, pathRoute("r", "r", 1, "/**", "h2c://api:8080"))
	fn := &fakeNet{}
	rt.CheckTargets(context.Background(), TargetCheck{
		Discover: func(context.Context) discovery.Result { return discovery.Result{Unavailable: "no runtime"} },
		Dial:     fn.dial, Proxy: noProxy,
	})
	if fn.dialed["api:8080"] != 1 || rt.Health()["r"].Target != TargetUp {
		t.Errorf("dialled %v, health %+v", fn.dialed, rt.Health()["r"])
	}
}

// A Kubernetes service the runtime did not count (no selector, or pods this
// gateway may not read) is dialled rather than read as zero ready.
func TestAKubernetesServiceIsDialled(t *testing.T) {
	rt := newRouter(t, pathRoute("r", "r", 1, "/**", "http://api.shop.svc.cluster.local:80"))
	fn := &fakeNet{}
	rt.CheckTargets(context.Background(), TargetCheck{
		Discover: func(context.Context) discovery.Result {
			return discovery.Result{Source: "kubernetes", Services: []discovery.Service{
				{Name: "api", Names: []string{"api", "api.shop.svc"}, Reachable: true},
			}}
		},
		Dial: fn.dial, Proxy: noProxy,
	})
	if fn.dialed["api.shop.svc.cluster.local:80"] != 1 || rt.Health()["r"].Target != TargetUp {
		t.Errorf("dialled %v, health %+v", fn.dialed, rt.Health()["r"])
	}
}

// Behind a proxy, connecting to the target directly would test a path no
// request takes: the proxy is what is dialled, and the verdict names it.
func TestBehindAProxyTheProxyIsDialled(t *testing.T) {
	rt := newRouter(t, pathRoute("r", "r", 1, "/**", "https://billing.acme.example"))
	fn := &fakeNet{errs: map[string]error{"proxy.corp:3128": &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}}}
	rt.CheckTargets(context.Background(), TargetCheck{
		Dial: fn.dial,
		Proxy: func(r *http.Request) (*url.URL, error) {
			if r.URL.Scheme != "https" {
				t.Errorf("the proxy was asked about %s", r.URL)
			}
			return url.Parse("http://proxy.corp:3128")
		},
	})
	h := rt.Health()["r"]
	if fn.dialed["billing.acme.example:443"] != 0 || h.Target != TargetDown || h.TargetWhy != "proxy: connection refused" {
		t.Errorf("dialled %v, health %+v", fn.dialed, h)
	}
}

// Traffic failing is a target down, whatever the connect found - on the spot
// in Health, and at the next round as a flip.
func TestAnOpenCircuitIsATargetDown(t *testing.T) {
	r := pathRoute("r", "r", 1, "/**", "http://api:8080")
	r.Breaker = &store.CircuitBreaker{Enabled: true, Trip: 2, Cool: "PT1M"}
	rt := newRouter(t, r)
	fn := &fakeNet{}
	flips := 0
	tc := TargetCheck{Dial: fn.dial, Proxy: noProxy, Flipped: func() { flips++ }}
	rt.CheckTargets(context.Background(), tc)
	if rt.Health()["r"].Target != TargetUp {
		t.Fatalf("before: %+v", rt.Health()["r"])
	}
	rt.breakers.of("r").failed(0, "boom", time.Now())
	rt.breakers.of("r").failed(0, "boom", time.Now())
	if h := rt.Health()["r"]; h.Target != TargetDown || h.TargetWhy != "not answering (circuit open)" {
		t.Errorf("an open circuit reads %+v", h)
	}
	if !rt.CheckTargets(context.Background(), tc) || flips != 2 {
		t.Errorf("the circuit opening was not a flip (%d)", flips)
	}
}

// Nothing proxies: discovery is not even asked.
func TestNoUpstreamNoDiscovery(t *testing.T) {
	rt := newRouter(t, pathRoute("redir", "redirect", 1, "/**", "",
		routing.Spec{Type: "redirect", Args: map[string]any{"location": "/new", "status": 301}}))
	asked := false
	if rt.CheckTargets(context.Background(), TargetCheck{
		Discover: func(context.Context) discovery.Result { asked = true; return discovery.Result{} },
	}) {
		t.Error("a round with no target flipped")
	}
	if asked {
		t.Error("discovery was asked with no target to check")
	}
}

// A counted service is read from its replicas, in three colours, and a
// replica or an image moving is news even when the colour does not change.
// An external host carries no replicas.
func TestReplicasMakeThreeStates(t *testing.T) {
	rt := newRouter(t,
		pathRoute("full", "full", 1, "/a/**", "http://orders:8080"),
		pathRoute("part", "part", 2, "/b/**", "http://billing:8080"),
		pathRoute("none", "none", 3, "/c/**", "http://stock.shop.svc:8080"),
		pathRoute("ext", "ext", 4, "/d/**", "https://partner.example"),
	)
	img := func(tag string, n int) []discovery.Image {
		return []discovery.Image{{Ref: "orders:" + tag, Tag: tag, Count: n, Ready: n}}
	}
	services := []discovery.Service{
		{Name: "orders", Names: []string{"orders", "orders.shop.svc"}, Ready: 3, Wanted: 3, Counted: true, Reachable: true, Images: img("1.4.0", 3)},
		{Name: "billing", Names: []string{"billing"}, Ready: 2, Wanted: 3, Counted: true, Reachable: true},
		{Name: "stock", Names: []string{"stock", "stock.shop.svc"}, Counted: true, Reachable: true},
	}
	flips := 0
	tc := TargetCheck{
		Discover: func(context.Context) discovery.Result {
			return discovery.Result{Source: "kubernetes", Services: services}
		},
		Dial: (&fakeNet{}).dial, Proxy: noProxy, Flipped: func() { flips++ },
	}
	rt.CheckTargets(context.Background(), tc)
	h := rt.Health()
	for id, w := range map[string][2]string{
		"full": {TargetUp, "3 of 3 ready in Kubernetes"},
		"part": {TargetDegraded, "2 of 3 ready"},
		"none": {TargetDown, "stopped (0 of 0)"},
		"ext":  {TargetUp, "answers"},
	} {
		if h[id].Target != w[0] || h[id].TargetWhy != w[1] {
			t.Errorf("%s: %q %q, want %q %q", id, h[id].Target, h[id].TargetWhy, w[0], w[1])
		}
	}
	if r := h["full"].Replicas; r == nil || r.Ready != 3 || len(r.Images) != 1 || r.Images[0].Tag != "1.4.0" {
		t.Errorf("full replicas: %+v", r)
	}
	if h["ext"].Replicas != nil {
		t.Errorf("an external host carries replicas: %+v", h["ext"].Replicas)
	}

	// A rolling update: still all ready, but on two images. Same colour, news.
	services[0].Images = append(img("1.4.1", 1), discovery.Image{Ref: "orders:1.4.0", Tag: "1.4.0", Count: 2, Ready: 2})
	if !rt.CheckTargets(context.Background(), tc) || flips != 2 {
		t.Errorf("an image moving did not wake the screen (%d)", flips)
	}
}

// Without anything to dial, no timer runs: a round comes from the runtime's
// events and from reloads only.
func TestRoundsFollowEventsWithoutATimer(t *testing.T) {
	rt := newRouter(t, pathRoute("r", "r", 1, "/**", "http://orders:8080"))
	ready := 1
	var mu sync.Mutex
	changed := make(chan struct{}, 1)
	rounds := make(chan struct{}, 8)
	tc := TargetCheck{
		Discover: func(context.Context) discovery.Result {
			mu.Lock()
			defer mu.Unlock()
			rounds <- struct{}{}
			return discovery.Result{Source: "swarm", Services: []discovery.Service{
				{Name: "orders", Names: []string{"orders"}, Ready: ready, Wanted: 2, Counted: true, Reachable: true}}}
		},
		Dial: (&fakeNet{}).dial, Proxy: noProxy, Changed: changed,
	}
	// The reload that built the router already poked: a round of its own.
	select {
	case <-rt.reloaded:
	default:
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go rt.WatchTargets(ctx, tc)
	becomes := func(want string) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for rt.Health()["r"].Target != want && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if got := rt.Health()["r"]; got.Target != want {
			t.Fatalf("want %s, got %+v", want, got)
		}
	}
	<-rounds
	becomes(TargetDegraded)
	mu.Lock()
	ready = 2
	mu.Unlock()
	changed <- struct{}{}
	<-rounds
	becomes(TargetUp)
	select {
	case <-rounds:
		t.Error("a round ran with no event and nothing to dial")
	case <-time.After(300 * time.Millisecond):
	}
}
