package certs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Which placed certificates a door serves with, in each mode.
func TestACertificateIsActiveInTheModeItFits(t *testing.T) {
	public := []string{"*.example.org"}
	cluster := []string{"meerkat.canopy.svc", "meerkat.canopy.svc.cluster.local"}
	runtime := []string{"meerkat.canopy.svc.cluster.local", "meerkat.canopy.svc", "meerkat.canopy", "meerkat"}

	gateway := Door{Names: []string{"shop.example.org"}}
	if !gateway.Active(public, runtime) || gateway.Active(cluster, runtime) {
		t.Error("this gateway: the public certificate serves, the cluster one does not")
	}
	proxied := Door{Proxied: true, Names: []string{"shop.example.org"}}
	if proxied.Active(public, runtime) || !proxied.Active(cluster, runtime) {
		t.Error("a proxy in front: the cluster certificate serves the hop, the public one does not")
	}
	if proxied.Active(public, nil) || !proxied.Active([]string{"gateway.lan"}, nil) {
		t.Error("with no runtime to name the plane, the public certificate is the proxy's and another serves the hop")
	}
	if !(Door{}).Active(cluster, runtime) {
		t.Error("a door nobody configured serves every placed certificate, as before")
	}
}

// The names a proxy in front served before doors existed become that door,
// behind that proxy; a door already said is not overwritten.
func TestTheProxiedNamesBecomeADoor(t *testing.T) {
	var s Settings
	if err := json.Unmarshal([]byte(`{"proxiedApp":["Shop.Example.org"],"proxiedConsole":["admin.example.org"],
		"consoleDoor":{"names":["console.example.org"]}}`), &s); err != nil {
		t.Fatal(err)
	}
	s = s.Normalized()
	if !s.AppDoor.Proxied || !slices.Equal(s.AppDoor.Names, []string{"shop.example.org"}) {
		t.Errorf("application door %+v", s.AppDoor)
	}
	if s.ConsoleDoor.Proxied || !slices.Equal(s.ConsoleDoor.Names, []string{"console.example.org"}) {
		t.Errorf("a door already said was overwritten: %+v", s.ConsoleDoor)
	}
	if s.LegacyProxiedApp != nil || s.LegacyProxiedConsole != nil {
		t.Error("the legacy fields stay behind")
	}
	if got := s.ProxiedOn(PlaneApp); !slices.Equal(got, []string{"shop.example.org"}) {
		t.Errorf("ProxiedOn = %v", got)
	}
}

// Whose forwarded headers a door believes.
func TestWhoADoorTrusts(t *testing.T) {
	if p, _ := (Door{}).Trusted(); p != nil {
		t.Errorf("nothing said: the filters' default (nil), got %v", p)
	}
	if p, _ := (Door{Names: []string{"shop.example.org"}}).Trusted(); p == nil || len(p) != 0 {
		t.Errorf("this gateway: nobody (empty, not nil), got %v", p)
	}
	if p, _ := (Door{Proxied: true}).Trusted(); p != nil {
		t.Errorf("a proxy with no address: the private ranges (nil), got %v", p)
	}
	p, err := (Door{Proxied: true, ProxyAddresses: []string{"130.211.0.0/22", "203.0.113.10"}}).Trusted()
	if err != nil || len(p) != 2 || p[1] != netip.MustParsePrefix("203.0.113.10/32") {
		t.Errorf("declared addresses: %v %v", p, err)
	}
	if _, err := (Door{Proxied: true, ProxyAddresses: []string{"not an address"}}).Trusted(); err == nil {
		t.Error("a bad address was accepted")
	}
}

// The supervisor serves a proxied door with the certificate carrying its
// runtime names, leaves the public one placed but unserved, and tells the
// filters whom the door trusts.
func TestASupervisorServesWhatTheDoorsModeFits(t *testing.T) {
	public := mustSelfSigned(t, Request{DNSNames: []string{"shop.example.org"}})
	cluster := mustSelfSigned(t, Request{DNSNames: []string{"meerkat.canopy.svc"}})
	src := &fakeSource{
		app: []*Material{public, cluster}, appFallback: public,
		settings: Settings{AppDoor: Door{Proxied: true, Names: []string{"shop.example.org"}, ProxyAddresses: []string{"203.0.113.0/24"}}},
	}
	sup := NewSupervisor(src, func(error) bool { return true },
		NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil), NewRedirect())
	sup.RuntimeNames = func(context.Context, string) []string { return []string{"meerkat.canopy.svc"} }
	told := map[string][]netip.Prefix{}
	sup.Trust = func(plane string, p []netip.Prefix, _ []string) { told[plane] = p }
	if err := sup.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })
	if !sup.App.Answers("meerkat.canopy.svc") || sup.App.Answers("shop.example.org") {
		t.Error("the proxied door must serve the cluster certificate and not the public one")
	}
	if len(told[PlaneApp]) != 1 || told[PlaneApp][0] != netip.MustParsePrefix("203.0.113.0/24") {
		t.Errorf("the application's trust: %v", told[PlaneApp])
	}
	if told[PlaneConsole] != nil {
		t.Errorf("a console nobody configured keeps the default: %v", told[PlaneConsole])
	}
}

// staleFirst answers the console's materials late and empty on its first
// read - a reload that started before cert-manager's Secret was seen - and
// with the certificate on every read after.
type staleFirst struct {
	fakeSource
	cert    *Material
	reads   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (f *staleFirst) CertificateMaterials(ctx context.Context, plane string) ([]*Material, *Material, []string, error) {
	if plane != PlaneConsole {
		return f.fakeSource.CertificateMaterials(ctx, plane)
	}
	if f.reads.Add(1) == 1 {
		close(f.started)
		<-f.release
		return nil, nil, nil, nil
	}
	return []*Material{f.cert}, f.cert, nil, nil
}

// Reloads take turns: one that read before a certificate arrived never
// closes the door a fresher one opened (2026-10-15, on the eval).
func TestAStaleReloadNeverClosesAFresherDoor(t *testing.T) {
	cert := mustSelfSigned(t, Request{DNSNames: []string{"meerkat-admin.canopy.svc"}})
	src := &staleFirst{cert: cert, started: make(chan struct{}), release: make(chan struct{})}
	sup := NewSupervisor(src, func(error) bool { return true },
		NewListener("data", "127.0.0.1:0", http.NotFoundHandler(), nil),
		NewListener("admin", "127.0.0.1:0", http.NotFoundHandler(), nil), NewRedirect())
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = sup.Reload(context.Background()) }()
	<-src.started
	go func() { defer wg.Done(); _ = sup.Reload(context.Background()) }()
	// Give the second reload the time to overtake the first, were it allowed.
	time.Sleep(50 * time.Millisecond)
	close(src.release)
	wg.Wait()
	if !sup.Console.Answers("meerkat-admin.canopy.svc") {
		t.Fatal("the stale reload applied last and closed the console's door")
	}
}

// The console's doors as the cluster sees them: a request counted, told to the
// others, and idle again past the window. Health checks are not traffic.
func TestTheConsoleDoorsTrafficIsPooledAndExpires(t *testing.T) {
	clock := time.Unix(1_800_000_000, 0)
	tr := NewTraffic()
	tr.now = func() time.Time { return clock }
	var moved int
	var told [][]Seen
	tr.Moved = func() { moved++ }
	relayed := make(chan []Seen, 4)
	tr.Relay = func(l []Seen) { relayed <- l }
	h := tr.Wrap(PlaneConsole, http.NotFoundHandler())
	call := func(path, peer string, secure bool) {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = peer + ":5000"
		if secure {
			r.TLS = &tls.ConnectionState{}
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	call("/healthz", "10.0.0.9", false)
	if len(tr.Recent()) != 0 || moved != 0 {
		t.Fatal("a health check is not traffic")
	}
	call("/api/me", "10.42.1.178", false)
	told = append(told, <-relayed)
	if peer, ok := tr.Receiving(PlaneConsole, "http"); !ok || peer != "10.42.1.178" || moved != 1 {
		t.Fatalf("receiving %v %q, moved %d", ok, peer, moved)
	}
	// Another node sees the HTTPS door called.
	tr.Report("node-b", []Seen{{Plane: PlaneConsole, Scheme: "https", Peer: "10.42.2.7", At: clock.Unix()}})
	if _, ok := tr.Receiving(PlaneConsole, "https"); !ok || moved != 2 {
		t.Fatalf("another node's report: moved %d", moved)
	}
	clock = clock.Add(TrafficWindow + time.Second)
	tr.Check()
	if len(tr.Recent()) != 0 || moved != 3 {
		t.Fatalf("past the window both doors are idle: %v, moved %d", tr.Recent(), moved)
	}
	if len(told[0]) != 1 || told[0][0].Scheme != "http" {
		t.Fatalf("what was told: %v", told)
	}
}
