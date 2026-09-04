// Package cluster keeps several gateways serving one installation in step
// (STORE-03).
//
// Everything a node needs is in the shared database EXCEPT what it keeps in
// memory to answer fast: the compiled route plan and the certificates its
// listeners hold. Those are loaded once and reloaded when the node itself
// takes a write - which is why, before this package, a route saved on node A
// was still absent from node B an hour later, and the operator looking at A
// saw a product that worked.
//
// The rule is one line: whatever a node RELOADS after its own write, it also
// ANNOUNCES. Every other node hears it and reloads the same thing. The store
// carries both halves - a version per topic in the database, a NOTIFY as the
// hint that it moved - and this package is the loop that reacts.
//
// On the embedded database there is nothing to do: one process owns the file,
// so the write and the reload happened in the same memory. Run returns at
// once, Announce is a no-op, and nothing in the single-binary deployment pays
// for any of this.
package cluster

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// defaultPollEvery is the slow safety net. The NOTIFY is what makes a change
// arrive in milliseconds; this is what makes it arrive AT ALL when the
// notification was lost - a connection that dropped between two heartbeats, or
// a change made straight in SQL.
//
// Half a minute rather than a second: it is a backstop, and a node that has to
// wait for it is already in a degraded case where thirty seconds is not what
// hurts.
const defaultPollEvery = 30 * time.Second

// retryEvery is how long a broken listener waits before opening another one.
// The catch-up runs BEFORE each attempt, so the reconnection is also when the
// gap gets closed.
const retryEvery = 5 * time.Second

// Bus reacts to what other nodes change.
type Bus struct {
	st *store.Store
	// node identifies this gateway for the life of the process. It rides on
	// every signal so a node can tell its own back: PostgreSQL delivers a
	// notification to every listener including the sender, and a hub message
	// relayed back to the node that published it would reach that node's pages
	// twice.
	node string

	pollEvery time.Duration

	mu sync.Mutex
	// on maps a topic to what re-reads it.
	on map[string]func(context.Context) error
	// onSignal maps a signal to what acts on its argument. Separate from on
	// because the two are answered differently: a topic sends the node back to
	// the table, a signal carries everything it needs.
	onSignal map[string]func(string)
	// acted records the version this node has already applied, whether it made
	// the change itself or reloaded because another one did. It is the whole
	// self-skip: a node that wrote does not reload twice, and a notification
	// it has already acted on costs one integer comparison.
	acted map[string]int64
}

// New returns a bus that reacts to nothing yet.
func New(st *store.Store, opts ...Option) *Bus {
	b := &Bus{
		st:        st,
		node:      nodeID(),
		pollEvery: defaultPollEvery,
		on:        map[string]func(context.Context) error{},
		onSignal:  map[string]func(string){},
		acted:     map[string]int64{},
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

// nodeID is random rather than a host name: two gateways in one container
// image have the same host name often enough, and what this has to be is
// unique for the life of a process.
func nodeID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Entropy is gone. A duplicated id costs a doubled hub message, not
		// correctness, so this is not worth refusing to start over.
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

// Option tweaks a Bus. Tests mostly - the backstop is the one thing that
// cannot be observed in a run shorter than itself.
type Option func(*Bus)

// WithPollEvery sets the backstop interval.
func WithPollEvery(d time.Duration) Option { return func(b *Bus) { b.pollEvery = d } }

// Register says what re-reads a topic. Called at wiring time, before Run.
func (b *Bus) Register(topic string, reload func(context.Context) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.on[topic] = reload
}

// OnSignal says what acts on a signal. Called at wiring time, before Run.
func (b *Bus) OnSignal(topic string, act func(arg string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onSignal[topic] = act
}

// Signal passes an argument to the other nodes with nothing in the database
// behind it - see the signal constants in internal/store for when that is the
// right trade.
//
// Nothing is recorded, so this node hears its own signal back and acts on it
// again. That is left alone rather than filtered: what a signal does is drop a
// cache entry, and dropping one twice is dropping it once.
func (b *Bus) Signal(ctx context.Context, topic, arg string) {
	if err := b.st.Signal(ctx, topic, b.node+" "+arg); err != nil {
		slog.Warn("could not signal the other nodes", "topic", topic, "err", err)
	}
}

// Announce records that this node changed a topic AND reloaded it, then tells
// the others.
//
// The order matters: the version is taken first and remembered as already
// acted on, so the node's own notification - PostgreSQL delivers it to every
// listener including the sender - finds nothing to do.
//
// It never returns an error. The write it follows has already succeeded and
// the local reload has already happened; refusing the request now would say
// "your change was not saved", which is false. What a failure costs is the
// other nodes waiting for the slow poll, and that is worth a log line, not a
// 500.
func (b *Bus) Announce(ctx context.Context, topic string) {
	v, err := b.st.MarkChanged(ctx, topic)
	if err != nil {
		slog.Error("could not mark a change: the other nodes will catch up on their timer",
			"topic", topic, "err", err)
		return
	}
	b.mu.Lock()
	b.acted[topic] = v
	b.mu.Unlock()
	if err := b.st.Announce(ctx, topic); err != nil {
		slog.Warn("could not announce a change: the other nodes will catch up on their timer",
			"topic", topic, "err", err)
	}
}

// Run listens until ctx ends. Returns immediately on the embedded database.
func (b *Bus) Run(ctx context.Context) {
	if !b.st.Clustered() {
		return
	}
	// The node has just loaded everything it caches, so the current versions
	// are by definition already applied. Without this it would reload every
	// registered topic once at boot, for nothing.
	if marks, err := b.st.ChangeMarks(ctx); err == nil {
		b.mu.Lock()
		for topic, v := range marks {
			b.acted[topic] = v
		}
		b.mu.Unlock()
	}
	slog.Info("cluster change bus started", "node", b.node, "topics", b.topics())

	go b.poll(ctx)
	for ctx.Err() == nil {
		err := b.st.Listen(ctx, func(payload string) {
			// A payload with an argument is a signal and is acted on as it
			// stands. Everything else is a topic: a hint about WHERE to look,
			// with the table saying whether anything actually moved. Reacting
			// to the table rather than to the message is what makes a lost
			// notification harmless - and it is why the empty payload Listen
			// sends on connecting, which means "you were away", needs no
			// special case here.
			if topic, rest, ok := strings.Cut(payload, " "); ok {
				from, arg, _ := strings.Cut(rest, " ")
				if from == b.node {
					return // our own, already acted on where it happened
				}
				slog.Debug("a node signalled", "topic", topic, "from", from)
				b.act(topic, arg)
				return
			}
			slog.Debug("a node announced a change", "topic", payload)
			b.catchUp(ctx)
		})
		if ctx.Err() != nil {
			return
		}
		slog.Warn("the change bus lost its connection, reconnecting",
			"err", err, "in", retryEvery)
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryEvery):
		}
	}
}

// act runs a signal's handler. A signal for something this node does not keep
// is nothing to it.
func (b *Bus) act(topic, arg string) {
	b.mu.Lock()
	fn := b.onSignal[topic]
	b.mu.Unlock()
	if fn != nil {
		fn(arg)
	}
}

// poll is the slow backstop. It runs alongside the listener rather than
// instead of it: the listener is what makes a change arrive at once, and this
// is what makes it arrive at all.
func (b *Bus) poll(ctx context.Context) {
	t := time.NewTicker(b.pollEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.catchUp(ctx)
		}
	}
}

// catchUp reloads every registered topic whose version moved past what this
// node has acted on.
func (b *Bus) catchUp(ctx context.Context) {
	marks, err := b.st.ChangeMarks(ctx)
	if err != nil {
		slog.Warn("could not read what changed", "err", err)
		return
	}
	for topic, version := range marks {
		b.mu.Lock()
		reload, registered := b.on[topic]
		stale := registered && version > b.acted[topic]
		b.mu.Unlock()
		if !stale {
			continue
		}
		if err := reload(ctx); err != nil {
			// Deliberately NOT recorded as acted on: the next notification or
			// the next tick tries again. A node serving the previous plan is
			// bad; a node that has decided it is up to date while serving the
			// previous plan is worse.
			slog.Error("a change from another node could not be applied",
				"topic", topic, "version", version, "err", err)
			continue
		}
		b.mu.Lock()
		b.acted[topic] = version
		b.mu.Unlock()
		slog.Info("reloaded after a change made on another node", "topic", topic, "version", version)
	}
}

func (b *Bus) topics() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.on))
	for t := range b.on {
		out = append(out, t)
	}
	return out
}
