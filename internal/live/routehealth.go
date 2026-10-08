package live

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	livewire "github.com/softwarity/livewire/go"
)

// RouteHealthTopic is what the routes screen subscribes to, to hear that a
// route's target went up, degraded or down, or that its replicas or images
// moved (SVC-04, SVC-07).
const RouteHealthTopic = "route-health"

// RouteHealth tells the routes screen that a target flipped.
//
// It carries a counter and nothing else, on the same argument as Changes: the
// screen then asks GET /api/routes/health, which already says everything about
// every route and is the one place that decides who may read it. One row whose
// version moves is the smallest thing livewire will push.
//
// Served to the routing plane's perimeter only - the readers that endpoint
// answers.
type RouteHealth struct {
	mu      sync.Mutex
	version int64

	// ONE wake channel, created once - same reason as Traffic and Changes.
	once sync.Once
	wake chan struct{}
}

// NewRouteHealth returns a source at version zero.
func NewRouteHealth() *RouteHealth { return &RouteHealth{} }

// Flipped records that a target changed state and wakes whoever is watching.
func (h *RouteHealth) Flipped() {
	h.mu.Lock()
	h.version++
	h.mu.Unlock()
	select {
	case h.waking() <- struct{}{}:
	default:
	}
}

func (h *RouteHealth) waking() chan struct{} {
	h.once.Do(func() { h.wake = make(chan struct{}, 1) })
	return h.wake
}

// ReadQuery has nothing to read: every reader watches the same counter.
func (h *RouteHealth) ReadQuery(_ json.RawMessage) (any, error) { return struct{}{}, nil }

// Key is constant: one read however many screens watch.
func (h *RouteHealth) Key(_ any) string { return RouteHealthTopic }

// Wake fires on a flip.
func (h *RouteHealth) Wake() <-chan struct{} { return h.waking() }

// Read is one row whose version is the number of flips so far.
func (h *RouteHealth) Read(_ context.Context, _ any) (livewire.Window, error) {
	h.mu.Lock()
	v := strconv.FormatInt(h.version, 10)
	h.mu.Unlock()
	total := 1
	return livewire.Window{
		Rows:  []livewire.Row{{ID: "targets", UpdatedAt: v, Data: map[string]any{"version": v}}},
		Total: &total,
	}, nil
}
