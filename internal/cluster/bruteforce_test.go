package cluster_test

import (
	"context"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// A limit of five attempts was five PER GATEWAY: an attacker spraying the load
// balancer got the limit multiplied by the very thing that was supposed to
// make the service sturdier. The counter now lives in the database, so the
// attempts add up wherever they land.
func TestFailedAttemptsAddUpAcrossGateways(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx := context.Background()
	key := "login|203.0.113.7|alice"

	for range 3 {
		if err := sa.RecordAttempt(ctx, key); err != nil {
			t.Fatalf("node A: %v", err)
		}
	}
	for range 2 {
		if err := sb.RecordAttempt(ctx, key); err != nil {
			t.Fatalf("node B: %v", err)
		}
	}
	// Either gateway must see all five, or a limit of five never trips.
	for name, st := range map[string]*store.Store{"A": sa, "B": sb} {
		n, err := st.CountAttempts(ctx, key, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("node %s: %v", name, err)
		}
		if n != 5 {
			t.Errorf("node %s counts %d of the 5 attempts: the throttle is per node, so N nodes is N times the quota", name, n)
		}
	}

	// A success on one gateway forgives on all of them.
	if err := sb.ClearAttempts(ctx, key); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if n, err := sa.CountAttempts(ctx, key, time.Now().Add(-15*time.Minute)); err != nil || n != 0 {
		t.Errorf("node A still counts %d (%v): a sign-in forgives everywhere or nowhere", n, err)
	}
}

// The window slides. A counter reset on a schedule answers "five in the last
// fifteen minutes" wrongly at every boundary; one row per attempt answers it
// exactly, and this is the half of that which a test can see.
func TestOnlyAttemptsInsideTheWindowCount(t *testing.T) {
	st, _ := twoNodes(t)
	ctx := context.Background()
	key := "login|203.0.113.8|bob"

	if err := st.RecordAttempt(ctx, key); err != nil {
		t.Fatal(err)
	}
	if n, err := st.CountAttempts(ctx, key, time.Now().Add(time.Minute)); err != nil || n != 0 {
		t.Errorf("an attempt older than the window counted: %d (%v)", n, err)
	}
	if n, err := st.CountAttempts(ctx, key, time.Now().Add(-time.Minute)); err != nil || n != 1 {
		t.Errorf("an attempt inside the window did not count: %d (%v)", n, err)
	}
}
