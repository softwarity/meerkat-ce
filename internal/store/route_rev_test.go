package store

import (
	"context"
	"errors"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestAStaleWriteIsRefused: three writers on one route - a console screen
// opened five minutes ago, an agent, an API call - and in a last-write-wins
// world the one with the oldest copy destroys what the other two decided,
// silently. A writer that says which revision it edited is told instead.
func TestAStaleWriteIsRefused(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	base := Route{ID: "r1", Name: "shop", Order: 1, Enabled: true, Upstream: "http://up",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []string{"/shop/**"}}}}}
	if err := s.SaveRoute(ctx, base); err != nil {
		t.Fatal(err)
	}

	// Two readers hold the same revision.
	first, err := s.GetRoute(ctx, "r1")
	if err != nil {
		t.Fatal(err)
	}
	second := first
	if first.Rev == 0 {
		t.Fatal("a read must say which revision it is")
	}

	// The first writes: it moves on.
	first.Upstream = "http://one"
	if err := s.SaveRoute(ctx, first); err != nil {
		t.Fatalf("the first write should pass: %v", err)
	}
	after, _ := s.GetRoute(ctx, "r1")
	if after.Rev <= first.Rev {
		t.Errorf("the revision did not move: %d then %d", first.Rev, after.Rev)
	}

	// The second still holds the old one, and its write is refused - with what
	// to do about it.
	second.Upstream = "http://two"
	err = s.SaveRoute(ctx, second)
	if !errors.Is(err, ErrStale) {
		t.Fatalf("a stale write was accepted: %v", err)
	}
	if got, _ := s.GetRoute(ctx, "r1"); got.Upstream != "http://one" {
		t.Errorf("the stale write landed anyway: %q", got.Upstream)
	}

	// Read again, redo, and it passes.
	fresh, _ := s.GetRoute(ctx, "r1")
	fresh.Upstream = "http://two"
	if err := s.SaveRoute(ctx, fresh); err != nil {
		t.Fatalf("a write on the current revision was refused: %v", err)
	}

	// And a writer that read no revision still wins: a seed, an import, a
	// server-side read-modify-write. Saying nothing is saying "whatever is
	// there".
	blind := fresh
	blind.Rev = 0
	blind.Upstream = "http://blind"
	if err := s.SaveRoute(ctx, blind); err != nil {
		t.Errorf("a write with no revision was refused: %v", err)
	}
}
