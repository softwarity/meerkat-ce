package instances

import (
	"testing"
	"time"
)

// A node that speaks is listed, a word that changes nothing moves nothing, and
// a node silent past StaleAfter is dropped - never this one.
func TestNodesComeAndGoByWhatTheySay(t *testing.T) {
	now := time.Date(2026, 11, 4, 6, 0, 0, 0, time.UTC)
	r := New("a")
	r.now = func() time.Time { return now }
	moves := 0
	r.Moved = func() { moves++ }

	r.Report(Instance{ID: "a", Host: "meerkat-a", Version: "1.23.0", Started: now, Ready: true}, now)
	r.Report(Instance{ID: "b", Host: "meerkat-b", Version: "1.23.0", Started: now.Add(time.Minute), Ready: true}, now)
	if moves != 2 || r.Count() != 2 {
		t.Fatalf("two nodes heard: %d moves, %d counted", moves, r.Count())
	}
	r.Report(Instance{ID: "b", Host: "meerkat-b", Version: "1.23.0", Started: now.Add(time.Minute), Ready: true}, now.Add(Every))
	if moves != 2 {
		t.Fatalf("a word that changes nothing moved the screen: %d", moves)
	}
	r.Report(Instance{ID: "b", Host: "meerkat-b", Version: "1.24.0", Started: now.Add(time.Minute), Ready: true}, now.Add(2*Every))
	if moves != 3 {
		t.Fatalf("a new version is a change: %d", moves)
	}

	list := r.List()
	if len(list) != 2 || list[0].ID != "a" || !list[0].Self || list[1].Self {
		t.Fatalf("oldest first, this node marked: %+v", list)
	}

	now = now.Add(2*Every + StaleAfter + time.Second)
	r.Check(now)
	if r.Count() != 1 || moves != 4 {
		t.Fatalf("a silent node must go, this one stay: %d counted, %d moves", r.Count(), moves)
	}
}

// The sender is who the bus says, whatever the message claims.
func TestAWordIsTheSendersOnly(t *testing.T) {
	in, err := Decode("real", Encode(Instance{ID: "forged", Host: "h", Self: true}))
	if err != nil || in.ID != "real" || in.Self {
		t.Fatalf("decoded %+v, %v", in, err)
	}
}

// Behind a load balancer the screen is told by one node and reads from another:
// a node past its silence is out of the list at the instant it is read, before
// any Check, so every node answers the same. And a node's last word drops it at
// once.
func TestASilentNodeIsOutWhenReadAndALeavingOneAtOnce(t *testing.T) {
	now := time.Date(2026, 11, 4, 6, 0, 0, 0, time.UTC)
	r := New("a")
	r.now = func() time.Time { return now }
	moves := 0
	r.Moved = func() { moves++ }
	r.Report(Instance{ID: "a", Version: "1"}, now)
	r.Report(Instance{ID: "b", Version: "1"}, now)
	r.Report(Instance{ID: "c", Version: "1"}, now)

	now = now.Add(StaleAfter + time.Second)
	r.Report(Instance{ID: "a", Version: "1"}, now)
	r.Report(Instance{ID: "c", Version: "1"}, now)
	if got := len(r.List()); got != 2 || r.Count() != 2 {
		t.Fatalf("b is past its silence: %d listed, %d counted", got, r.Count())
	}

	before := moves
	r.Report(Instance{ID: "c", Leaving: true}, now)
	if len(r.List()) != 1 || moves != before+1 {
		t.Fatalf("a leaving node goes at once, and says so: %+v, %d moves", r.List(), moves-before)
	}
}
