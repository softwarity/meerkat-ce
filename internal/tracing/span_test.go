package tracing

import (
	"testing"
)

func reset(t *testing.T) {
	t.Helper()
	before, beforeMax := SampleRate(), MaxPerSecond()
	t.Cleanup(func() { SetSampleRate(before); SetMaxPerSecond(beforeMax) })
	SetMaxPerSecond(0)
}

// A rate is a dial. A dial that throws is a dial nobody turns.
func TestSampleRateIsADial(t *testing.T) {
	reset(t)
	for _, c := range []struct {
		in   float64
		want float64
	}{{-1, 0}, {0, 0}, {0.1, 0.1}, {0.5, 0.5}, {1, 1}, {7, 1}} {
		SetSampleRate(c.in)
		if got := SampleRate(); got != c.want {
			t.Errorf("SetSampleRate(%v) then SampleRate() = %v, want %v", c.in, got, c.want)
		}
	}
}

// The decision belongs to whoever opened the journey. Rolling the dice again
// is how a trace ends up with a parent nobody reported.
func TestACallersDecisionIsNotRolledAgain(t *testing.T) {
	reset(t)
	SetSampleRate(0) // ours says no to everything

	yes := SpanContext{TraceID: "a", SpanID: "b", Inbound: true, Sampled: true}
	if !ShouldRecord(yes) {
		t.Error("a caller who is recording was overruled by our own rate")
	}
	no := SpanContext{TraceID: "a", SpanID: "b", Inbound: true, Sampled: false}
	SetSampleRate(1) // ours says yes to everything
	if ShouldRecord(no) {
		t.Error("a caller who asked not to be recorded was recorded anyway")
	}
}

// A journey we opened is ours to decide, and the two ends of the dial mean
// what they say.
func TestOurOwnJourneysFollowTheRate(t *testing.T) {
	reset(t)
	ours := SpanContext{TraceID: "a"}

	SetSampleRate(0)
	for i := 0; i < 50; i++ {
		if ShouldRecord(ours) {
			t.Fatal("a journey was recorded at a zero rate")
		}
	}
	SetSampleRate(1)
	for i := 0; i < 50; i++ {
		if !ShouldRecord(ours) {
			t.Fatal("a journey was dropped at a full rate")
		}
	}
}

// The ceiling is what bounds the abuse: the sampling flag is a byte anybody
// can send, and somebody sending 01 on every request must not be able to
// spend an unbounded amount of somebody else's money.
func TestTheCeilingBoundsWhatACallerCanSpend(t *testing.T) {
	reset(t)
	SetSampleRate(0) // ours would record nothing at all
	SetMaxPerSecond(10)

	// A thousand requests all claiming to be sampled.
	insistent := SpanContext{TraceID: "a", SpanID: "b", Inbound: true, Sampled: true}
	recorded := 0
	for i := 0; i < 1000; i++ {
		if ShouldRecord(insistent) {
			recorded++
		}
	}
	if recorded == 0 {
		t.Fatal("the ceiling refused everything: a legitimate caller is never recorded")
	}
	if recorded > 10 {
		t.Errorf("%d journeys recorded in one second under a ceiling of 10", recorded)
	}
}

// And it bounds OUR own sampling just the same: a ceiling that only applied to
// strangers would be a ceiling an operator could not use as a budget.
func TestTheCeilingBoundsOurOwnSamplingToo(t *testing.T) {
	reset(t)
	SetSampleRate(1)
	SetMaxPerSecond(5)

	ours := SpanContext{TraceID: "a"}
	recorded := 0
	for i := 0; i < 200; i++ {
		if ShouldRecord(ours) {
			recorded++
		}
	}
	if recorded > 5 {
		t.Errorf("%d journeys recorded under a ceiling of 5", recorded)
	}
}

// No ceiling means no ceiling, which is what an installation with a collector
// of its own and a small traffic wants.
func TestNoCeilingMeansNoCeiling(t *testing.T) {
	reset(t)
	SetSampleRate(1)
	SetMaxPerSecond(0)

	ours := SpanContext{TraceID: "a"}
	for i := 0; i < 5000; i++ {
		if !ShouldRecord(ours) {
			t.Fatalf("a journey was dropped at iteration %d with no ceiling set", i)
		}
	}
}
