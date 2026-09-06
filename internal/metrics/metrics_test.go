package metrics

import (
	"context"
	"testing"
	"time"
)

func TestObserveSortsByClassAndBucket(t *testing.T) {
	r := &Route{ID: "r1", Name: "one"}
	r.Observe(200, 3*time.Millisecond)  // 2xx, first bucket
	r.Observe(204, 30*time.Millisecond) // 2xx, the 50ms bucket
	r.Observe(404, 7*time.Millisecond)  // 4xx
	r.Observe(503, 30*time.Second)      // 5xx, past the last boundary
	r.Observe(0, time.Millisecond)      // a handler that wrote nonsense

	s := r.snapshot()
	if s.ByClass[2] != 2 || s.ByClass[4] != 1 {
		t.Errorf("classes = %v, want two 2xx and one 4xx", s.ByClass)
	}
	// The 5xx one and the nonsense one both land in 5: a status nobody can
	// name is not nothing, and counting it nowhere loses a request.
	if s.ByClass[5] != 2 {
		t.Errorf("5xx = %d, want the 503 and the unnameable one", s.ByClass[5])
	}
	if s.Total() != 5 {
		t.Errorf("total = %d, want every request counted exactly once", s.Total())
	}
	// The overflow bucket is what lets the histogram say "slower than ten
	// seconds" rather than losing it.
	if s.Buckets[len(Buckets)] != 1 {
		t.Errorf("overflow = %d, want the thirty-second one", s.Buckets[len(Buckets)])
	}
	var counted uint64
	for _, b := range s.Buckets {
		counted += b
	}
	if counted != 5 {
		t.Errorf("buckets hold %d observations, want 5 - every one lands somewhere", counted)
	}
}

func TestARenameKeepsTheSeries(t *testing.T) {
	reg := NewRegistry()
	a := reg.For("r1", "before")
	a.Observe(200, time.Millisecond)
	b := reg.For("r1", "after")
	if a != b {
		t.Fatal("the same id must reach the same counters: a rename is not a new series")
	}
	s := reg.Snapshot()
	if len(s.Routes) != 1 || s.Routes[0].Name != "after" || s.Routes[0].Total() != 1 {
		t.Errorf("snapshot = %+v, want one series carrying the new name and the old count", s.Routes)
	}
}

// The first tick is a BASELINE and publishes nothing: a first point covering
// everything since startup would read as a spike at the moment somebody opened
// the screen.
func TestTheFirstTickIsABaseline(t *testing.T) {
	reg := NewRegistry()
	r := reg.For("r1", "one")
	for range 10 {
		r.Observe(200, time.Millisecond)
	}
	f := NewFleet()
	now := time.Now()
	s := reg.Snapshot()
	f.Report("me", s, now)
	if _, ok := f.Tick(now); ok {
		t.Fatal("the first tick must publish nothing")
	}
	r.Observe(200, time.Millisecond)
	now = now.Add(5 * time.Second)
	f.Report("me", reg.Snapshot(), now)
	sample, ok := f.Tick(now)
	if !ok {
		t.Fatal("the second tick must publish")
	}
	if n := sample.Routes[0].Total(); n != 1 {
		t.Errorf("the interval holds %d, want the 1 that happened in it, not the 11 since startup", n)
	}
}

// THE rule: differences per node, then summed. Never the sum, then
// differenced - a node leaving makes the sum fall, and differencing that fall
// prints the survivors' whole history as one five-second spike.
func TestANodeLeavingDoesNotSpikeTheCurve(t *testing.T) {
	f := NewFleet()
	f.StaleAfter = 10 * time.Second
	at := time.Now()
	totals := func(n uint64) Snapshot {
		return Snapshot{Routes: []RouteSnapshot{{ID: "r1", Name: "one",
			ByClass: [6]uint64{0, 0, n}, Buckets: make([]uint64, 12)}}}
	}
	f.Report("a", totals(1000), at)
	f.Report("b", totals(1000), at)
	f.Tick(at) // baseline

	at = at.Add(5 * time.Second)
	f.Report("a", totals(1010), at)
	f.Report("b", totals(1005), at)
	sample, _ := f.Tick(at)
	if got := sample.Routes[0].ByClass[2]; got != 15 {
		t.Fatalf("two nodes: %d, want 10 + 5", got)
	}

	// B goes away and stops reporting. Its share simply stops; nothing spikes.
	at = at.Add(30 * time.Second)
	f.Report("a", totals(1020), at)
	sample, _ = f.Tick(at)
	if got := sample.Routes[0].ByClass[2]; got != 10 {
		t.Errorf("after a node left: %d, want A's 10 alone - never the fall of the sum", got)
	}
	if sample.Nodes != 0 && f.Nodes() != 1 {
		t.Errorf("%d nodes counted, want just A", f.Nodes())
	}
}

// A node restarting is one node's counters resetting, not the cluster's.
func TestARestartCostsOneNodesInterval(t *testing.T) {
	f := NewFleet()
	at := time.Now()
	totals := func(n uint64) Snapshot {
		return Snapshot{Routes: []RouteSnapshot{{ID: "r1", ByClass: [6]uint64{0, 0, n}, Buckets: make([]uint64, 12)}}}
	}
	f.Report("a", totals(500), at)
	f.Report("b", totals(500), at)
	f.Tick(at)

	at = at.Add(5 * time.Second)
	f.Report("a", totals(510), at)
	f.Report("b", totals(3), at) // b restarted
	sample, _ := f.Tick(at)
	if got := sample.Routes[0].ByClass[2]; got != 13 {
		t.Errorf("%d, want A's 10 plus B's 3 since its restart", got)
	}
}

// A route absent from a report keeps the total it had: that is what lets a
// node send only what moved, on a bus with a payload limit.
func TestAnAbsentRouteKeepsItsTotal(t *testing.T) {
	f := NewFleet()
	at := time.Now()
	two := Snapshot{Routes: []RouteSnapshot{
		{ID: "r1", ByClass: [6]uint64{0, 0, 100}, Buckets: make([]uint64, 12)},
		{ID: "r2", ByClass: [6]uint64{0, 0, 100}, Buckets: make([]uint64, 12)},
	}}
	f.Report("a", two, at)
	f.Tick(at)

	at = at.Add(5 * time.Second)
	// Only r1 moved, so only r1 is sent.
	f.Report("a", Snapshot{Routes: []RouteSnapshot{
		{ID: "r1", ByClass: [6]uint64{0, 0, 105}, Buckets: make([]uint64, 12)},
	}}, at)
	sample, _ := f.Tick(at)
	byID := map[string]RouteSnapshot{}
	for _, r := range sample.Routes {
		byID[r.ID] = r
	}
	if got := byID["r1"].ByClass[2]; got != 5 {
		t.Errorf("r1 = %d, want 5", got)
	}
	if got := byID["r2"].ByClass[2]; got != 0 {
		t.Errorf("r2 = %d, want 0 - unsent is unchanged, not vanished", got)
	}
}

func TestTheRingKeepsTheLastSamplesInOrder(t *testing.T) {
	w := NewWindow(3)
	for i := 1; i <= 5; i++ {
		w.Add(Sample{Routes: []RouteSnapshot{{ID: "r1", ByClass: [6]uint64{0, 0, uint64(i)}}}})
	}
	got := w.Samples()
	if len(got) != 3 {
		t.Fatalf("%d samples, want the ring's 3", len(got))
	}
	for i, want := range []uint64{3, 4, 5} {
		if n := got[i].Routes[0].ByClass[2]; n != want {
			t.Errorf("sample %d holds %d, want %d - oldest first", i, n, want)
		}
	}
}

func TestAListenerIsWokenAndNeverBlocksTheSampler(t *testing.T) {
	w := NewWindow(4)
	c, stop := w.Listen()
	defer stop()
	w.Add(Sample{})
	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("a sample landed and the listener was not woken")
	}
	// Nobody draining: the sampler must not block on it.
	done := make(chan struct{})
	go func() {
		for range 20 {
			w.Add(Sample{})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the sampler blocked on a listener that was not reading")
	}
}

func TestRunStopsWithItsContext(t *testing.T) {
	reg := NewRegistry()
	w := NewWindow(4)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Run(ctx, reg, NewFleet(), w, "me", 5*time.Millisecond, nil); close(done) }()
	time.Sleep(40 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the sampler outlived its context")
	}
	if len(w.Samples()) == 0 {
		t.Error("the sampler produced nothing")
	}
}
