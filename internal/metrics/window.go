package metrics

import (
	"sync"
	"time"
)

// The window the console reads.
//
// A sampler takes a snapshot every few seconds and keeps the DIFFERENCE from
// the one before, which is what a curve is: requests per second, not requests
// since the process started. The counters stay monotonic underneath, because
// that is what a Prometheus needs - the window is derived from them, never the
// reverse. Deriving totals from a window would lose whatever fell off its end.
//
// It is a ring, and its size is the whole retention policy: nothing is written
// down, nothing grows, and a restart starts a new hour. That is the honest
// shape for a built-in view - a gateway is not a time-series database, and
// pretending otherwise is how a product ends up maintaining one badly. An
// installation that wants a year of history scrapes it into the one it already
// runs.

// Sample is one interval's worth of traffic.
type Sample struct {
	// At is the END of the interval: the moment this was measured.
	At time.Time `json:"at"`
	// Seconds the interval covered, so a reader can turn counts into rates
	// without assuming the period - a sampler that ran late must not read as
	// a spike.
	Seconds float64 `json:"seconds"`
	// Per route, the counts ADDED during the interval.
	Routes []RouteSnapshot `json:"routes"`
	// Gateway-wide, as they stood at the end, summed over the nodes counted.
	InFlight  int64  `json:"inFlight"`
	Logins    uint64 `json:"logins"`
	Refused   uint64 `json:"refused"`
	Unmatched uint64 `json:"unmatched"`
	// Nodes is how many gateways this interval covers. One on a plain
	// installation; on a cluster it is what stops a partial curve passing for
	// a total, and the console says so rather than leaving it to be assumed.
	Nodes int `json:"nodes"`
}

// Window keeps the last N samples.
type Window struct {
	mu      sync.RWMutex
	samples []Sample
	next    int
	filled  bool
	// listeners are woken when a sample lands. The console's live channel
	// subscribes; a poller does not have to.
	listeners map[chan struct{}]struct{}
}

// NewWindow returns a ring of the given size. Fewer than two is meaningless -
// a window needs a before and an after - so it is raised.
func NewWindow(size int) *Window {
	if size < 2 {
		size = 2
	}
	return &Window{samples: make([]Sample, size), listeners: map[chan struct{}]struct{}{}}
}

// Add stores one interval and wakes whoever is watching.
func (w *Window) Add(sample Sample) {
	w.mu.Lock()
	w.samples[w.next] = sample
	w.next = (w.next + 1) % len(w.samples)
	if w.next == 0 {
		w.filled = true
	}
	woken := make([]chan struct{}, 0, len(w.listeners))
	for c := range w.listeners {
		woken = append(woken, c)
	}
	w.mu.Unlock()

	// Outside the lock, and never blocking: a listener that has not drained
	// its channel is a listener that already knows there is something new.
	for _, c := range woken {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}

// Samples returns the window oldest first. A copy: the caller renders it while
// the sampler keeps writing.
func (w *Window) Samples() []Sample {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.filled {
		out := make([]Sample, w.next)
		copy(out, w.samples[:w.next])
		return out
	}
	out := make([]Sample, 0, len(w.samples))
	out = append(out, w.samples[w.next:]...)
	return append(out, w.samples[:w.next]...)
}

// Listen returns a channel woken whenever a sample lands, and the function
// that stops it.
func (w *Window) Listen() (<-chan struct{}, func()) {
	c := make(chan struct{}, 1)
	w.mu.Lock()
	w.listeners[c] = struct{}{}
	w.mu.Unlock()
	return c, func() {
		w.mu.Lock()
		delete(w.listeners, c)
		w.mu.Unlock()
	}
}
