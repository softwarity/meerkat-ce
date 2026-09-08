// Package cluster keeps several gateways serving one installation in step
// (STORE-03): the contract they are wired against, and the seam the loop
// plugs into.
//
// Everything a node needs is in the shared database EXCEPT what it keeps in
// memory to answer fast: the compiled route plan and the certificates its
// listeners hold. Those are loaded once and reloaded when the node itself
// takes a write - which is why, before this, a route saved on node A was still
// absent from node B an hour later, and the operator looking at A saw a
// product that worked.
//
// The rule is one line: whatever a node RELOADS after its own write, it also
// ANNOUNCES. Every other node hears it and reloads the same thing. The store
// carries both halves - a version per topic in the database, a NOTIFY as the
// hint that it moved - and ee/changebus is the loop that reacts.
//
// WHY THE LOOP IS NOT HERE. Meerkat ships as two images (internal/edition).
// This one runs on the embedded database, where there is nothing to keep in
// step: one process owns the file, so a write and the reload it causes happen
// in the same memory, and the quiet implementation below is exactly right. The
// image that connects to a shared database carries the loop that talks to it.
//
// What stays here either way is the shape - the interface and the topics - so
// the call sites never know which they got, and there is no "if clustered"
// anywhere but in this file.
package cluster

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// Bus reacts to what other nodes change. One implementation talks to a shared
// database; the other is the quiet one below.
type Bus interface {
	// Register says what re-reads a topic when another node changes it.
	Register(topic string, reload func(context.Context) error)
	// OnSignal says what acts on a signal's argument.
	OnSignal(topic string, act func(arg string))
	// OnSignalFrom is OnSignal for a topic where WHO sent it is part of the
	// message - a node's own counters, say, where the receiver keeps one entry
	// per node and has to know which one to replace.
	OnSignalFrom(topic string, act func(from, arg string))
	// Node is this gateway's identity for the life of the process.
	Node() string
	// Signal passes an argument to the other nodes with nothing in the
	// database behind it - see the signal constants in internal/store.
	Signal(ctx context.Context, topic, arg string)
	// Announce records that this node changed a topic AND reloaded it, then
	// tells the others.
	Announce(ctx context.Context, topic string)
	// Run serves until ctx ends.
	Run(ctx context.Context)
}

// Config is what a bus is built with. A struct rather than options on the
// concrete type: the trunk hands it over without knowing what it builds.
type Config struct {
	// PollEvery is the slow safety net. The NOTIFY is what makes a change
	// arrive in milliseconds; this is what makes it arrive AT ALL when the
	// notification was lost - a connection that dropped between two
	// heartbeats, or a change made straight in SQL.
	PollEvery time.Duration
}

// DefaultPollEvery is half a minute rather than a second: it is a backstop,
// and a node that has to wait for it is already in a degraded case where
// thirty seconds is not what hurts.
const DefaultPollEvery = 30 * time.Second

// Option tweaks a Config. Tests mostly - the backstop is the one thing that
// cannot be observed in a run shorter than itself.
type Option func(*Config)

// WithPollEvery sets the backstop interval.
func WithPollEvery(d time.Duration) Option { return func(c *Config) { c.PollEvery = d } }

// build is the Enterprise implementation, registered from its init.
var (
	buildMu sync.RWMutex
	build   func(*store.Store, Config) Bus
)

// Register is called from ee/changebus's init.
func Register(f func(*store.Store, Config) Bus) {
	buildMu.Lock()
	defer buildMu.Unlock()
	build = f
}

// Available reports whether this build carries a real bus.
func Available() bool {
	buildMu.RLock()
	defer buildMu.RUnlock()
	return build != nil
}

// New returns the bus this build has: the real one, or the quiet one.
func New(st *store.Store, opts ...Option) Bus {
	cfg := Config{PollEvery: DefaultPollEvery}
	for _, o := range opts {
		o(&cfg)
	}
	buildMu.RLock()
	f := build
	buildMu.RUnlock()
	if f != nil {
		return f(st, cfg)
	}
	return quiet{node: NodeID()}
}

// NodeID is random rather than a host name: two gateways in one container
// image have the same host name often enough, and what this has to be is
// unique for the life of a process.
func NodeID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Entropy is gone. A duplicated id costs a doubled hub message, not
		// correctness, so this is not worth refusing to start over.
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

// quiet is one gateway: it still has an identity, because the code that keeps
// one entry per node reports the local one through the same door, and there is
// no privileged member. Everything else has nobody to tell.
type quiet struct{ node string }

func (quiet) Register(string, func(context.Context) error) {}
func (quiet) OnSignal(string, func(string))                {}
func (quiet) OnSignalFrom(string, func(string, string))    {}
func (q quiet) Node() string                               { return q.node }
func (quiet) Signal(context.Context, string, string)       {}
func (quiet) Announce(context.Context, string)             {}
func (quiet) Run(context.Context)                          {}
