package metrics

import (
	"context"
	"time"
)

// DefaultInterval is how often the window gains a point.
//
// Five seconds, and the choice is between two failures. Too long and the
// built-in view stops being live, which is its whole reason to exist next to a
// Prometheus that samples every fifteen. Too short and an hour of history
// costs a lot of samples for a curve nobody can read at that resolution.
const DefaultInterval = 5 * time.Second

// DefaultWindow is how many samples are kept: an hour at the interval above.
// Nothing is written down - see window.go for why a gateway should not grow
// into a time-series database.
const DefaultWindow = int(time.Hour / DefaultInterval)

// Run feeds the window until the context ends.
//
// One goroutine for the whole gateway, doing the only expensive part of this
// package - walking every route and copying its counters - so the request path
// never does it. It runs on a ticker rather than on demand: a view that
// computed a sample when somebody opened a screen would show a first point
// covering the time since the last visitor, which is not an interval anybody
// asked about.
//
// The local node reports through the fleet like any other, and broadcast
// carries its totals to the rest. A plain installation passes nil and takes
// exactly the same path: one member, and no special case to get wrong the day
// a second one appears.
func Run(ctx context.Context, reg *Registry, f *Fleet, w *Window, self string, every time.Duration, broadcast func(Snapshot)) {
	if every <= 0 {
		every = DefaultInterval
	}
	tick := func() {
		// The endpoints' own history, coarser than this one and advanced from
		// here rather than from a second goroutine: one clock for the two, so
		// a minute boundary is the same minute for both.
		reg.RollEndpoints(time.Now())
		s := reg.Snapshot()
		f.Report(self, s, s.At)
		if broadcast != nil {
			broadcast(s)
		}
		if sample, ok := f.Tick(s.At); ok {
			sample.Nodes = f.Nodes()
			w.Add(sample)
		}
	}
	// The first tick establishes the baseline and publishes nothing, so the
	// first point appears one interval in rather than showing every request
	// since startup as a spike.
	tick()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}
