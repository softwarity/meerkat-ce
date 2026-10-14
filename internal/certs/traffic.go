package certs

import (
	"net/http"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/filters"
)

// Which door of each plane is actually called, and by whom (SSL-14).
//
// A certificate placed for the hop from a proxy serves nobody until the proxy
// is pointed at the HTTPS port, and nothing said so: an Ingress kept calling
// the plain port, the screen showed HTTPS open, and the operator believed the
// hop encrypted (2026-10-15, Cilium in front of the eval). The same fact is
// what makes closing a port safe or not: the port the proxy still calls is the
// one that must stay open.
//
// Counted per node and pooled across the cluster the way the served names are:
// a node tells its own list over the bus while it has traffic, the others keep
// one entry per node, and what nobody said for TrafficWindow is idle. Nothing
// is written to the database - a request is not worth a write.

// TrafficWindow is how long a door is called "receiving" after its last
// request: short, so that pointing the proxy elsewhere shows within the
// minute, and long enough to cover a few quiet seconds of real traffic.
const TrafficWindow = 30 * time.Second

// tellEvery is how often a node repeats what it sees while its traffic lasts:
// often enough that the others never let a busy door expire.
const tellEvery = 5 * time.Second

// Seen is the last request one door of one plane received.
type Seen struct {
	Plane  string `json:"plane"`
	Scheme string `json:"scheme"` // http or https
	// Peer is the TCP peer: the proxy in front, or the caller when nothing is.
	Peer string `json:"peer"`
	At   int64  `json:"at"` // unix seconds
}

type doorKey struct{ plane, scheme string }

// Traffic watches the doors of this node and pools what the others report.
type Traffic struct {
	mu     sync.Mutex
	local  map[doorKey]Seen
	told   map[doorKey]time.Time
	others map[string]map[doorKey]Seen
	// shown is the set of receiving doors (and their peer) last announced,
	// so the screen is told only when it changes - a door starting or
	// stopping to receive, or another peer - never on every request.
	shown map[doorKey]string

	// Relay sends this node's list to the others; Moved tells the screens.
	Relay func([]Seen)
	Moved func()
	now   func() time.Time
}

// NewTraffic builds an empty watch.
func NewTraffic() *Traffic {
	return &Traffic{local: map[doorKey]Seen{}, told: map[doorKey]time.Time{},
		others: map[string]map[doorKey]Seen{}, shown: map[doorKey]string{}, now: time.Now}
}

// Wrap counts the requests a plane's handler receives, on either door: a
// request with TLS came through the HTTPS one. Health checks are not traffic.
// The loopback is: on the console it is an operator - a port-forward, a
// gateway run on their own machine - never a probe, which asks the
// application's port.
func (t *Traffic) Wrap(plane string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			if a, err := netip.ParseAddr(filters.PeerIP(r)); err == nil {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				t.saw(plane, scheme, a.String())
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (t *Traffic) saw(plane, scheme, peer string) {
	now := t.now()
	k := doorKey{plane, scheme}
	t.mu.Lock()
	prev := t.local[k]
	t.local[k] = Seen{Plane: plane, Scheme: scheme, Peer: peer, At: now.Unix()}
	tell := prev.Peer != peer || now.Sub(t.told[k]) >= tellEvery
	if tell {
		t.told[k] = now
	}
	list := t.listLocked(t.local)
	t.mu.Unlock()
	if tell && t.Relay != nil {
		go t.Relay(list)
	}
	if tell {
		t.Check()
	}
}

// Report keeps what another node said it sees.
func (t *Traffic) Report(from string, list []Seen) {
	m := make(map[doorKey]Seen, len(list))
	for _, s := range list {
		m[doorKey{s.Plane, s.Scheme}] = s
	}
	t.mu.Lock()
	t.others[from] = m
	t.mu.Unlock()
	t.Check()
}

// Check tells the screens when the set of receiving doors changed - called on
// every report, and by a clock so a door that stopped receiving is said idle.
func (t *Traffic) Check() {
	now := t.Recent()
	shown := make(map[doorKey]string, len(now))
	for _, s := range now {
		shown[doorKey{s.Plane, s.Scheme}] = s.Peer
	}
	t.mu.Lock()
	changed := len(shown) != len(t.shown)
	for k, p := range shown {
		if t.shown[k] != p {
			changed = true
		}
	}
	t.shown = shown
	t.mu.Unlock()
	if changed && t.Moved != nil {
		t.Moved()
	}
}

// Recent is, per door, its latest request across the cluster within
// TrafficWindow; a door absent from it receives nothing.
func (t *Traffic) Recent() []Seen {
	since := t.now().Add(-TrafficWindow).Unix()
	t.mu.Lock()
	defer t.mu.Unlock()
	best := map[doorKey]Seen{}
	take := func(m map[doorKey]Seen) {
		for k, s := range m {
			if s.At >= since && s.At > best[k].At {
				best[k] = s
			}
		}
	}
	take(t.local)
	for _, m := range t.others {
		take(m)
	}
	return t.listLocked(best)
}

// Receiving says whether a door had a request within TrafficWindow, and from
// whom last.
func (t *Traffic) Receiving(plane, scheme string) (string, bool) {
	for _, s := range t.Recent() {
		if s.Plane == plane && s.Scheme == scheme {
			return s.Peer, true
		}
	}
	return "", false
}

func (t *Traffic) listLocked(m map[doorKey]Seen) []Seen {
	out := make([]Seen, 0, len(m))
	for _, s := range m {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plane != out[j].Plane {
			return out[i].Plane < out[j].Plane
		}
		return out[i].Scheme < out[j].Scheme
	})
	return out
}
