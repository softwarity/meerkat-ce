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
