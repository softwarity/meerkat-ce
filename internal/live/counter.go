package live

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	livewire "github.com/softwarity/livewire/go"
)

// Counter tells a screen that something it shows moved, and nothing else.
//
// The pattern behind RouteHealth and Presence, for the next topic: one row
// whose version is bumped by Moved, and the screen re-reads the API it already
// reads - the one place that decides who may see what. A screen that would
// otherwise ask every few seconds whether anything changed subscribes to one
// of these instead: the channel exists so the console does not poll.
type Counter struct {
	topic string

	mu      sync.Mutex
	version int64

	once sync.Once
	wake chan struct{}
}

// NewCounter returns a counter at version zero, served under topic.
func NewCounter(topic string) *Counter { return &Counter{topic: topic} }

// Moved bumps the version and wakes whoever is watching.
func (c *Counter) Moved() {
	c.mu.Lock()
	c.version++
	c.mu.Unlock()
	select {
	case c.waking() <- struct{}{}:
	default:
	}
}

func (c *Counter) waking() chan struct{} {
	c.once.Do(func() { c.wake = make(chan struct{}, 1) })
	return c.wake
}

// ReadQuery has nothing to read: every reader watches the same counter.
func (c *Counter) ReadQuery(_ json.RawMessage) (any, error) { return struct{}{}, nil }

// Key is the topic: one read however many screens watch.
func (c *Counter) Key(_ any) string { return c.topic }

// Wake fires on a move.
func (c *Counter) Wake() <-chan struct{} { return c.waking() }

// Read is one row whose version is the number of moves so far.
func (c *Counter) Read(_ context.Context, _ any) (livewire.Window, error) {
	c.mu.Lock()
	v := strconv.FormatInt(c.version, 10)
	c.mu.Unlock()
	total := 1
	return livewire.Window{
		Rows:  []livewire.Row{{ID: c.topic, UpdatedAt: v, Data: map[string]any{"version": v}}},
		Total: &total,
	}, nil
}

// CertificatesTopic is what the TLS screen subscribes to, to hear that an
// authority answered - a certificate issued, a request refused (SSL-05).
const CertificatesTopic = "certificates"

// PauseTopic is what every console subscribes to, to hear that the gateway was
// paused or resumed (store/pause.go).
const PauseTopic = "pause"
