package metrics

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func sample(id string, n uint64) RouteSnapshot {
	r := RouteSnapshot{ID: id, ByClass: [6]uint64{0, 0, n, 0, 1, 2},
		Buckets: make([]uint64, len(Buckets)+1), SumSecs: float64(n) / 3}
	for i := range r.Buckets {
		r.Buckets[i] = n + uint64(i)
	}
	r.Failures = [4]uint64{n, 1, 2, 3}
	return r
}

func TestAMessageSurvivesTheRoundTrip(t *testing.T) {
	in := Snapshot{InFlight: 7, Logins: 12, Refused: 3, Unmatched: 99,
		Routes: []RouteSnapshot{sample("r1", 1000), sample("r2", 5)}}
	msgs := Encode(in, Chunk)
	if len(msgs) != 1 {
		t.Fatalf("%d messages for two routes, want 1", len(msgs))
	}
	out, err := Decode(msgs[0])
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out.InFlight != 7 || out.Logins != 12 || out.Refused != 3 || out.Unmatched != 99 {
		t.Errorf("gateway-wide numbers lost: %+v", out)
	}
	if len(out.Routes) != 2 {
		t.Fatalf("%d routes back, want 2", len(out.Routes))
	}
	for i, want := range in.Routes {
		got := out.Routes[i]
		if got.ID != want.ID || got.ByClass != want.ByClass || got.Failures != want.Failures {
			t.Errorf("route %s: %+v, want %+v", want.ID, got, want)
		}
		if len(got.Buckets) != len(want.Buckets) {
			t.Errorf("route %s: %d buckets, want %d", want.ID, len(got.Buckets), len(want.Buckets))
		}
		// Microseconds is the resolution on the wire, which is finer than any
		// latency worth drawing.
		if diff := got.SumSecs - want.SumSecs; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("route %s: sum %v, want %v", want.ID, got.SumSecs, want.SumSecs)
		}
	}
}

// A table bigger than one payload is SPLIT, never truncated: the order is
// stable, so a dropped tail would be the same routes missing every time.
func TestABigTableIsSplitAndNothingIsLost(t *testing.T) {
	in := Snapshot{}
	for i := range 300 {
		in.Routes = append(in.Routes, sample(fmt.Sprintf("route-number-%03d", i), 987654321))
	}
	msgs := Encode(in, Chunk)
	if len(msgs) < 2 {
		t.Fatalf("%d messages for 300 routes, want several", len(msgs))
	}
	seen := map[string]bool{}
	for _, m := range msgs {
		if len(m) > Chunk {
			t.Errorf("a message is %d bytes, over the %d a notification may carry", len(m), Chunk)
		}
		out, err := Decode(m)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		for _, r := range out.Routes {
			seen[r.ID] = true
		}
	}
	if len(seen) != 300 {
		t.Errorf("%d routes came back out of 300 - a split must lose nothing", len(seen))
	}
}

// During a rolling update two versions are live at once. A node that cannot
// read a message must say so rather than guess at it.
func TestAnotherVersionIsRefusedRatherThanGuessed(t *testing.T) {
	if _, err := Decode("m9 0 0 0 0"); !errors.Is(err, ErrWireVersion) {
		t.Errorf("err = %v, want the version refusal", err)
	}
	for _, bad := range []string{"", "m1", "m1 a 0 0 0", "m1 0 0 0 0 broken", "m1 0 0 0 0 r1:1:2:3"} {
		if _, err := Decode(bad); err == nil {
			t.Errorf("Decode(%q) accepted a message it should not have", bad)
		}
	}
}

// An id carrying a separator would split a record in two and turn a field into
// a number. Skipped rather than escaped - a missing curve beats a wrong one.
func TestAnIdThatWouldBreakTheStreamIsSkipped(t *testing.T) {
	in := Snapshot{Routes: []RouteSnapshot{sample("good", 1), sample("bad:id", 1), sample("bad id", 1)}}
	out, err := Decode(Encode(in, Chunk)[0])
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(out.Routes) != 1 || out.Routes[0].ID != "good" {
		t.Errorf("routes = %+v, want only the safe one", out.Routes)
	}
}

// Only what moved is sent: a hundred and fifty routes, most idle in any given
// five seconds, must not fill a payload every tick.
func TestOnlyWhatMovedIsSent(t *testing.T) {
	now := Snapshot{At: time.Now(), Routes: []RouteSnapshot{sample("r1", 10), sample("r2", 10)}}
	first, sent := Moved(now, nil)
	if len(first.Routes) != 2 {
		t.Fatalf("first report holds %d routes, want both", len(first.Routes))
	}
	next := Snapshot{At: time.Now(), Routes: []RouteSnapshot{sample("r1", 11), sample("r2", 10)}}
	moved, _ := Moved(next, sent)
	if len(moved.Routes) != 1 || moved.Routes[0].ID != "r1" {
		t.Errorf("moved = %+v, want only r1", moved.Routes)
	}
}
