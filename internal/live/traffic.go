package live

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	livewire "github.com/softwarity/livewire/go"
	"github.com/softwarity/meerkat/internal/metrics"
)

// TrafficTopic is what the console subscribes to for the curves.
const TrafficTopic = "traffic"

// liveSamples is how many intervals the socket carries.
//
// The library caps a window at 200 rows, and at five seconds that is a little
// under twenty minutes - which is what a live view is for. Anything longer is
// history, and history is a question somebody asks once: GET /api/metrics
// answers it on demand rather than pushing an hour down a socket that then has
// to keep it in step.
const liveSamples = 200

// Traffic publishes the metrics window as a live list.
//
// The adapter lives HERE rather than in internal/metrics, which is the whole
// point of the arrangement: that package counts and knows nothing about how
// anybody watches. It exposes a window and a channel that fires when the
// window moves, and turning those two into a subscription is this package's
// business. The next screen - the audit trail, the issue list - gets its
// adapter here too, and its own package stays as free of the transport.
type Traffic struct {
	window *metrics.Window

	// ONE wake channel for the source, created once.
	//
	// livewire calls Wake once per pump and gives no way to stop what it hands
	// back - no context, no closer - so a source returning a fresh listener
	// each time would leak one per subscription ever opened, and the sampler
	// would walk every one of them on every tick. A constant Key means at most
	// one pump at a time, so one channel is both correct and the only shape
	// that does not accumulate.
	once sync.Once
	wake <-chan struct{}
}

// NewTraffic adapts a metrics window to the live channel.
func NewTraffic(window *metrics.Window) *Traffic { return &Traffic{window: window} }

// ReadQuery is the trust boundary: what arrives is JSON off a socket.
//
// There is nothing to read. Every subscriber watches the same list, and how far
// back a SCREEN chooses to draw is the screen's own business - trimming per
// subscriber would split one shared read into one per client for a difference
// the client can make itself.
func (t *Traffic) ReadQuery(_ json.RawMessage) (any, error) { return struct{}{}, nil }

// Key is constant: one question, one read, however many are watching.
func (t *Traffic) Key(_ any) string { return TrafficTopic }

// Wake fires whenever a sample lands.
func (t *Traffic) Wake() <-chan struct{} {
	t.once.Do(func() {
		// The stop function is deliberately dropped: this listener lives as
		// long as the gateway, which is the only lifetime livewire offers.
		c, _ := t.window.Listen()
		t.wake = c
	})
	return t.wake
}

// Read is the window as it stands.
func (t *Traffic) Read(_ context.Context, _ any) (livewire.Window, error) {
	samples := t.window.Samples()
	if len(samples) > liveSamples {
		samples = samples[len(samples)-liveSamples:]
	}
	rows := make([]livewire.Row, 0, len(samples))
	for _, s := range samples {
		// The instant IS the identity: a sample is one interval and there is
		// exactly one per instant. And it is the version too - a sample never
		// changes once measured, so a row already on the client never needs
		// sending again, which is what makes the socket carry one row a tick
		// instead of the whole window.
		id := strconv.FormatInt(s.At.UnixMilli(), 10)
		rows = append(rows, livewire.Row{
			ID:        id,
			UpdatedAt: id,
			Data: map[string]any{
				"at":        s.At,
				"seconds":   s.Seconds,
				"routes":    s.Routes,
				"inFlight":  s.InFlight,
				"logins":    s.Logins,
				"refused":   s.Refused,
				"unmatched": s.Unmatched,
				"nodes":     s.Nodes,
			},
		})
	}
	total := len(rows)
	return livewire.Window{Rows: rows, Total: &total}, nil
}
