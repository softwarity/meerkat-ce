// Package limits counts requests against a bound, per caller.
//
// A SLIDING WINDOW, and not a bucket that resets. A counter with a reset time
// answers "a hundred in the last minute" wrongly at every boundary: two
// hundred requests land in two seconds, one on each side of the reset, and the
// bound that was supposed to be a hundred a minute let two hundred through.
// The shape here is the standard two-window estimate - the current window's
// count plus the previous one's, weighted by how far into the current one we
// are - which costs two integers per key and is wrong by less than a percent.
//
// IN MEMORY, PER NODE, and that is a decision rather than an omission. An
// exact shared counter costs a round trip to the database on every request,
// which is not a price a gateway can pay on the path of all traffic. This is
// the protective half: approximate, free, and enough against abuse. The half
// that gets billed or shown to a customer is a different mechanism, written in
// batches (QUOTA-03), and it is not this one. On a cluster each node holds its
// own bound, so the installation allows up to N times what one node does - the
// console says so rather than letting the number be read as a total.
//
// BOUNDED, which is the whole reason this package has a Counter type rather
// than a map. A limit keyed per address makes one counter per address, and the
// set of addresses is chosen by whoever sends the requests: a flood from a
// botnet would grow the gateway's memory until it fell over, through the very
// mechanism installed to survive a flood. So the keys are capped, expired ones
// are swept first, and what does not fit shares ONE counter - the long tail
// gets a single caller's worth of allowance between them, while the callers
// already being tracked keep their own. Refusing the untracked outright would
// let anybody deny service by rotating addresses; allowing them would let
// anybody bypass the bound the same way.
package limits

import (
	"sync"
	"time"
)

// MaxKeys is how many distinct callers one rule tracks. Ten thousand is more
// than any real installation has active in a minute and small enough to be
// invisible: a key is two integers and two instants, so this is a few hundred
// kilobytes per rule at worst.
const MaxKeys = 10_000

// overflowKey is where everything past the budget shares one counter. It
// cannot collide with a real key: those are user ids, addresses and the empty
// string, never this.
const overflowKey = "\x00overflow"

// window is one key's two-window estimate.
type window struct {
	// start is when the current window opened.
	start time.Time
	// cur counts the current window, prev the one before it.
	cur, prev int
	// seen is the last time this key was touched, for eviction.
	seen time.Time
}

// Counter is one rule's counters, one per key.
type Counter struct {
	limit int
	every time.Duration

	mu   sync.Mutex
	keys map[string]*window
	// swept is when expired keys were last removed, so a sweep costs O(n)
	// once a window rather than on every arrival that finds the map full.
	swept time.Time
}

// New returns a counter allowing n requests per window.
func New(n int, every time.Duration) *Counter {
	return &Counter{limit: n, every: every, keys: map[string]*window{}}
}

// Verdict is what a caller is told, and the last two fields are why it is a
// struct: a refusal without "how many are left" and "when to come back" is a
// door with no sign on it, and the standard headers exist to carry both.
type Verdict struct {
	OK        bool
	Limit     int
	Remaining int
	// ResetIn is how long until the caller's window has room again. Never
	// zero on a refusal: a Retry-After of 0 is an invitation to hammer.
	ResetIn time.Duration
}

// Allow records one request against key and says whether it passes.
func (c *Counter) Allow(key string, now time.Time) Verdict {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.windowFor(key, now)
	// Roll forward. Two windows or more of silence is a clean slate; one is
	// the current becoming the previous.
	switch elapsed := now.Sub(w.start); {
	case elapsed >= 2*c.every:
		w.cur, w.prev, w.start = 0, 0, now
	case elapsed >= c.every:
		w.cur, w.prev, w.start = 0, w.cur, w.start.Add(c.every)
	}
	w.seen = now

	// The estimate: what is left of the previous window, by how much of the
	// current one has not elapsed, plus the current one.
	into := now.Sub(w.start)
	weight := 1 - float64(into)/float64(c.every)
	if weight < 0 {
		weight = 0
	}
	estimate := float64(w.prev)*weight + float64(w.cur)

	if estimate+1 > float64(c.limit) {
		// Not counted. A refused request that still increments turns a burst
		// into a lockout that outlives it - the caller backs off, and the
		// counter they are backing off from is still being fed by their own
		// refusals.
		return Verdict{Limit: c.limit, Remaining: 0, ResetIn: c.resetIn(w, now)}
	}
	w.cur++
	remaining := c.limit - int(estimate+1)
	if remaining < 0 {
		remaining = 0
	}
	return Verdict{OK: true, Limit: c.limit, Remaining: remaining, ResetIn: c.resetIn(w, now)}
}

// resetIn is when this key's window turns over. Rounded up to a whole second,
// because Retry-After is expressed in seconds and rounding down would send the
// caller back a moment too early, to be refused again.
func (c *Counter) resetIn(w *window, now time.Time) time.Duration {
	d := w.start.Add(c.every).Sub(now)
	if d <= 0 {
		return time.Second
	}
	return d.Truncate(time.Second) + time.Second
}

// windowFor finds or makes a key's window, keeping the map bounded. Called
// under c.mu.
func (c *Counter) windowFor(key string, now time.Time) *window {
	if w, ok := c.keys[key]; ok {
		return w
	}
	if len(c.keys) >= MaxKeys {
		// Expired keys first: a window nobody has touched for two windows
		// holds nothing, and on any ordinary installation this is where the
		// room comes from.
		c.sweep(now)
	}
	if len(c.keys) >= MaxKeys {
		// Still full, so these are genuinely that many active callers. The
		// rest share one counter rather than being let through or refused as
		// a group - both of those are a bypass, in opposite directions.
		if w, ok := c.keys[overflowKey]; ok {
			return w
		}
		w := &window{start: now, seen: now}
		c.keys[overflowKey] = w
		return w
	}
	w := &window{start: now, seen: now}
	c.keys[key] = w
	return w
}

func (c *Counter) sweep(now time.Time) {
	if now.Sub(c.swept) < c.every {
		return // already swept this window; nothing new has expired
	}
	c.swept = now
	dead := now.Add(-2 * c.every)
	for k, w := range c.keys {
		if k != overflowKey && w.seen.Before(dead) {
			delete(c.keys, k)
		}
	}
}

// Tracked is how many keys this counter holds, for tests and for the day
// somebody wants to see it on a screen.
func (c *Counter) Tracked() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.keys)
}
