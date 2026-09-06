package metrics

import (
	"testing"
	"time"
)

// The reason this history exists: a route's endpoints have to add up to the
// route's own row, and the row covers a WINDOW. Answering with totals since
// the process started put one failure on the route and three under it.
func TestOperationsAnswerForAPeriod(t *testing.T) {
	reg := NewRegistry()
	e := reg.Deduced("r1", "GET", "/orders/{id}")

	base := time.Now().Add(-30 * time.Minute)
	// Old traffic, outside the period anybody will ask about.
	for i := 0; i < 7; i++ {
		e.Observe(200, 10*time.Millisecond)
	}
	e.Observe(500, 10*time.Millisecond)
	reg.rollAt(base)

	// And the traffic of the last few minutes.
	for i := 0; i < 3; i++ {
		e.Observe(200, 20*time.Millisecond)
	}
	e.Observe(503, 20*time.Millisecond)
	reg.rollAt(base.Add(25 * time.Minute))

	all := one(t, reg.Operations(time.Time{}))
	if all.Requests != 12 || all.Errors != 2 {
		t.Fatalf("since the start: %d requests, %d errors; want 12 and 2", all.Requests, all.Errors)
	}
	recent := one(t, reg.Operations(base.Add(10*time.Minute)))
	if recent.Requests != 4 || recent.Errors != 1 {
		t.Fatalf("over the period: %d requests, %d errors; want 4 and 1", recent.Requests, recent.Errors)
	}
	if got := recent.SumSecs; got < 0.079 || got > 0.081 {
		t.Errorf("over the period: %.3f seconds spent, want 0.08", got)
	}
}

// A minute nothing happened in writes no point, and that is what makes the
// answer right rather than what makes it small: differencing against the last
// point BEFORE the period still yields the period's traffic, because a gap
// means there was none.
func TestIdleEndpointsCostNothingAndStillAnswer(t *testing.T) {
	reg := NewRegistry()
	e := reg.Deduced("r1", "GET", "/quiet")
	start := time.Now().Add(-time.Hour)

	e.Observe(200, time.Millisecond)
	reg.rollAt(start)
	for i := 1; i <= 20; i++ { // twenty silent minutes
		reg.rollAt(start.Add(time.Duration(i) * time.Minute))
	}
	if n := len(e.hist); n != 1 {
		t.Fatalf("a silent endpoint kept %d points, want 1", n)
	}
	// Asked for the last five minutes, it answers zero rather than one.
	if got := one(t, reg.Operations(start.Add(15*time.Minute))).Requests; got != 0 {
		t.Errorf("over a period it was silent in: %d requests, want 0", got)
	}
	// And an endpoint never called at all keeps nothing.
	never := reg.Deduced("r1", "GET", "/never")
	reg.rollAt(start.Add(30 * time.Minute))
	if len(never.hist) != 0 {
		t.Errorf("an endpoint nobody called kept %d points, want 0", len(never.hist))
	}
}

// The ring is bounded: a busy endpoint keeps the last hour and a bit, not the
// life of the process.
func TestHistoryIsBounded(t *testing.T) {
	reg := NewRegistry()
	e := reg.Deduced("r1", "GET", "/busy")
	start := time.Now().Add(-4 * time.Hour)
	for i := 0; i < 240; i++ {
		e.Observe(200, time.Millisecond)
		reg.rollAt(start.Add(time.Duration(i) * time.Minute))
	}
	if len(e.hist) != historyPoints {
		t.Fatalf("kept %d points, want %d", len(e.hist), historyPoints)
	}
}

// rollAt is RollEndpoints with the minute guard defeated, so a test can move
// through an hour without waiting one.
func (reg *Registry) rollAt(at time.Time) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.rolledAt = at
	for _, e := range reg.endpoints {
		e.roll(at)
	}
}

func one(t *testing.T, ops []EndpointSnapshot) EndpointSnapshot {
	t.Helper()
	for _, o := range ops {
		if o.Path != "/never" {
			return o
		}
	}
	t.Fatal("no operation")
	return EndpointSnapshot{}
}
