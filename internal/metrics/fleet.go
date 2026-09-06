package metrics

import (
	"sync"
	"time"
)

// Fleet turns what every node reports into one interval for the whole cluster.
//
// THE RULE THAT MAKES IT CORRECT: the difference is taken PER NODE and the
// differences are then summed. Never the other way round.
//
// Summing the totals first and differencing the sum looks equivalent and is
// not. The moment a node leaves - a rolling update, a crash, a scale-down -
// the sum falls by everything that node had ever counted, and a difference
// reads that fall as either a negative or, once clamped, as a spike of the
// remaining nodes' whole history landing in five seconds. Per node, a
// departure simply stops contributing, which is the truth.
//
// It is the same reason a monitoring stack sums rates rather than rating sums,
// and it is worth writing down because the wrong order is the natural one to
// write.
//
// WHAT IS EXCHANGED IS TOTALS, NOT INTERVALS. The bus is lossy by design - a
// notification reaches whoever is listening at that instant and nobody else -
// so a message carrying "what happened in the last five seconds" would lose
// that traffic for good. A message carrying the running totals repairs itself:
// the next one is complete, and the interval that spans the gap simply covers
// two ticks. A route absent from a message keeps the total it had, which is
// what lets a node send only what moved.
type Fleet struct {
	mu    sync.Mutex
	nodes map[string]*node
	// StaleAfter is how long a node stays counted after its last word. Beyond
	// it the node is dropped: a node that has gone must stop holding its share
	// of the curve open.
	StaleAfter time.Duration
	previousAt time.Time
	started    bool
}

type node struct {
	// totals as last reported, and as they stood at the previous tick.
	totals    map[string]RouteSnapshot
	atTick    map[string]RouteSnapshot
	inFlight  int64
	logins    uint64
	refused   uint64
	unmatched uint64
	lastSeen  time.Time
}

// DefaultStaleAfter is four intervals: enough that a node whose message was
// lost keeps its place, short enough that a node that is gone stops counting
// within twenty seconds.
const DefaultStaleAfter = 4 * DefaultInterval

// NewFleet returns an empty fleet.
func NewFleet() *Fleet {
	return &Fleet{nodes: map[string]*node{}, StaleAfter: DefaultStaleAfter}
}

// Report records what one node says its totals are. The local node reports
// through here too: there is no privileged member, which is what makes a
// single-node gateway and a cluster the same code path.
func (f *Fleet) Report(id string, s Snapshot, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.nodes[id]
	if !ok {
		n = &node{totals: map[string]RouteSnapshot{}, atTick: map[string]RouteSnapshot{}}
		f.nodes[id] = n
	}
	// Merged, not replaced: a node sends only the routes that moved, and one
	// missing from a message still has the total it had. That is also what
	// makes a split message safe - each piece is a complete update on its own,
	// so nothing has to be reassembled and a lost piece costs one route one
	// interval rather than making a whole batch unusable.
	//
	// ASSUMED: one node's messages arrive in the order it sent them. They do -
	// the bus listens on a single connection and PostgreSQL delivers in commit
	// order - and it matters, because an older total overwriting a newer one
	// would make the next difference read as a spike. Written down rather than
	// relied on silently: the day this bus is carried by something else, this
	// is the property that has to come with it.
	for _, r := range s.Routes {
		n.totals[r.ID] = r
	}
	n.inFlight, n.logins, n.refused, n.unmatched = s.InFlight, s.Logins, s.Refused, s.Unmatched
	n.lastSeen = at
}

// Nodes is how many are currently counted. The console says "this node" or
// names the number, rather than letting a partial curve pass for a total.
func (f *Fleet) Nodes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.nodes)
}

// Tick closes the interval and returns it. ok is false for the very first one:
// there is no interval before the first measurement, and publishing one would
// publish every count since startup as if it had happened in five seconds.
func (f *Fleet) Tick(now time.Time) (Sample, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for id, n := range f.nodes {
		if f.StaleAfter > 0 && now.Sub(n.lastSeen) > f.StaleAfter {
			delete(f.nodes, id)
		}
	}

	sample := Sample{At: now, Seconds: now.Sub(f.previousAt).Seconds()}
	byRoute := map[string]*RouteSnapshot{}
	order := make([]string, 0, 8)
	for _, n := range f.nodes {
		for id, total := range n.totals {
			d := delta(n.atTick[id], total)
			acc, seen := byRoute[id]
			if !seen {
				acc = &RouteSnapshot{ID: d.ID, Name: d.Name, Buckets: make([]uint64, len(d.Buckets))}
				byRoute[id] = acc
				order = append(order, id)
			}
			add(acc, d)
			n.atTick[id] = total
		}
		sample.InFlight += n.inFlight
		sample.Logins += n.logins
		sample.Refused += n.refused
		sample.Unmatched += n.unmatched
	}
	for _, id := range order {
		sample.Routes = append(sample.Routes, *byRoute[id])
	}

	first := !f.started
	f.started, f.previousAt = true, now
	return sample, !first
}

// delta is what happened on ONE node between two readings of one route. A
// counter that went down means that node restarted; its new value is the
// honest interval, and it is one node's worth rather than the cluster's.
func delta(before, now RouteSnapshot) RouteSnapshot {
	out := RouteSnapshot{ID: now.ID, Name: now.Name, Buckets: make([]uint64, len(now.Buckets))}
	for i := range now.ByClass {
		out.ByClass[i] = sub(before.ByClass[i], now.ByClass[i])
	}
	for i := range now.Buckets {
		var b uint64
		if i < len(before.Buckets) {
			b = before.Buckets[i]
		}
		out.Buckets[i] = sub(b, now.Buckets[i])
	}
	for i := range now.Failures {
		out.Failures[i] = sub(before.Failures[i], now.Failures[i])
	}
	if now.SumSecs >= before.SumSecs {
		out.SumSecs = now.SumSecs - before.SumSecs
	} else {
		out.SumSecs = now.SumSecs
	}
	return out
}

func add(into *RouteSnapshot, d RouteSnapshot) {
	if into.Name == "" {
		into.Name = d.Name
	}
	for i := range d.ByClass {
		into.ByClass[i] += d.ByClass[i]
	}
	for i := range d.Buckets {
		if i < len(into.Buckets) {
			into.Buckets[i] += d.Buckets[i]
		}
	}
	for i := range d.Failures {
		into.Failures[i] += d.Failures[i]
	}
	into.SumSecs += d.SumSecs
}

func sub(before, now uint64) uint64 {
	if now < before {
		return now
	}
	return now - before
}
