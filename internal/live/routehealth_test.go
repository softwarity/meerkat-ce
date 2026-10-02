package live

import (
	"context"
	"testing"
)

// A flip wakes the routes screen, and the row it reads has moved - which is
// what makes livewire push it rather than decide nothing changed.
func TestATargetFlipWakesTheRoutesScreen(t *testing.T) {
	h := NewRouteHealth()
	before, err := h.Read(context.Background(), struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-h.Wake():
		t.Fatal("woken before anything flipped")
	default:
	}
	h.Flipped()
	select {
	case <-h.Wake():
	default:
		t.Fatal("a flip did not wake the source")
	}
	after, _ := h.Read(context.Background(), struct{}{})
	if before.Rows[0].UpdatedAt == after.Rows[0].UpdatedAt {
		t.Errorf("the row kept its version across a flip (%s): nothing would be pushed", after.Rows[0].UpdatedAt)
	}
	// Two flips before anybody reads are one nudge, never a queue.
	h.Flipped()
	h.Flipped()
	<-h.Wake()
	select {
	case <-h.Wake():
		t.Error("two flips left two nudges")
	default:
	}
}
