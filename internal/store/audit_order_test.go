package store

import (
	"context"
	"testing"
)

// Events one node writes back to back read back in that order, even inside
// one millisecond: the trail tells an incident in the order it happened.
func TestEventIDsKeepTheOrderTheyWereMadeIn(t *testing.T) {
	prev := NewEventID()
	for i := 0; i < 5000; i++ {
		id := NewEventID()
		if len(id) != 32 {
			t.Fatalf("id %q is %d wide, want 32", id, len(id))
		}
		if id <= prev {
			t.Fatalf("id %d (%s) sorts before the one made before it (%s)", i, id, prev)
		}
		prev = id
	}
}

// A kind the filter does not know is refused with the two it does, rather than
// read as "both" - a typo must not widen what somebody believes they asked for.
func TestAnUnknownAuditKindIsRefused(t *testing.T) {
	st := openTemp(t)
	_, err := st.ListAuditEvents(context.Background(), AuditFilter{Kind: "logins"})
	if err == nil {
		t.Fatal("an unknown kind was accepted")
	}
}

// NotTarget is the console's own trail: everything but the applications'
// sign-ins, which the Data plane shows apart.
func TestNotTargetLeavesOneKindOut(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	for _, target := range []string{"route", AuditTargetAccount, AuditTargetConsole} {
		if err := st.AddAuditEvent(ctx, AuditEvent{Target: target, Action: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.ListAuditEvents(ctx, AuditFilter{NotTarget: AuditTargetAccount})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want route and console", len(got))
	}
	for _, e := range got {
		if e.Target == AuditTargetAccount {
			t.Fatal("an account event came through NotTarget=account")
		}
	}
}

// The calls of audited operations have their own lifetime: the trail's purge
// leaves them, theirs leaves the rest.
func TestEndpointCallsHaveTheirOwnRetention(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if err := st.AddAuditEvent(ctx, AuditEvent{Target: "route", Action: "route.update", At: 100}); err != nil {
		t.Fatal(err)
	}
	if err := st.insertCalls(ctx, []AuditEvent{{ID: NewEventID(), At: 100, Target: AuditTargetEndpoint, Action: "endpoint.call",
		Data: &EndpointCall{Route: "r", Status: 200}}}); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.PurgeAuditEventsBefore(ctx, 200); n != 1 {
		t.Fatalf("the trail's purge removed %d, want the change alone", n)
	}
	if n, _ := st.PurgeEndpointCallsBefore(ctx, 200); n != 1 {
		t.Fatalf("the calls' purge removed %d, want the call", n)
	}
	if d := st.EndpointCallRetentionDays(ctx); d != DefaultEndpointCallRetention {
		t.Fatalf("default retention %d", d)
	}
	if err := st.SetEndpointCallRetentionDays(ctx, 12); err == nil {
		t.Fatal("a retention that is not offered was accepted")
	}
}

// An assertion is spent once: the second post of the same one is refused,
// and the sweep forgets it only once it would have expired anyway.
func TestASAMLAssertionIsSpentOnce(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	first, err := st.SpendAssertion(ctx, "idp|_a1", 100)
	if err != nil || !first {
		t.Fatalf("first use: %v %v", first, err)
	}
	if again, _ := st.SpendAssertion(ctx, "idp|_a1", 100); again {
		t.Fatal("the same assertion was accepted twice")
	}
	if n, _ := st.PurgeSpentAssertions(ctx, 50); n != 0 {
		t.Fatal("an assertion still valid was forgotten")
	}
	if n, _ := st.PurgeSpentAssertions(ctx, 200); n != 1 {
		t.Fatal("an expired assertion was kept")
	}
}
