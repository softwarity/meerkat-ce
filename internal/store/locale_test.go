package store

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The layer holds what DIFFERS, and nothing else. A row that stores a whole
// copy would freeze that language at the day it was copied, hiding every fix a
// later release brings.
func TestALocaleOverrideKeepsOnlyWhatWasChanged(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	if err := s.SetLocaleOverride(ctx, "fr", map[string]string{
		"signIn": "Se connecter", "  ": "ignore", "blank": "   ",
	}); err != nil {
		t.Fatal(err)
	}
	all, err := s.LocaleOverrides(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fr := all["fr"]
	if fr["signIn"] != "Se connecter" {
		t.Errorf("the correction was not kept: %v", fr)
	}
	// An empty value is a RESET, not a string set to nothing: blank on a page
	// is invisible, so it can never be what somebody meant to store.
	if _, there := fr["blank"]; there {
		t.Errorf("an empty value was stored: %v", fr)
	}
	if len(fr) != 1 {
		t.Errorf("the layer holds %d entries, want the one that changed: %v", len(fr), fr)
	}
}

// A language with nothing in it still exists. That is what makes "add a
// language" possible: the code is created first and the wordings come after, and
// a row that vanished the moment it was emptied would take the language with it.
// It is not reported as EDITED, which is a different question.
func TestAnEmptiedLocaleStillExists(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_ = s.SetLocaleOverride(ctx, "de", map[string]string{"signIn": "Anmelden"})
	if err := s.SetLocaleOverride(ctx, "de", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	all, _ := s.LocaleOverrides(ctx)
	entries, there := all["de"]
	if !there {
		t.Fatalf("an emptied language is gone, so nobody could create one: %v", all)
	}
	if len(entries) != 0 {
		t.Errorf("an emptied language still holds %v", entries)
	}
	// And deleting it is what removes it.
	if err := s.DeleteLocaleOverride(ctx, "de"); err != nil {
		t.Fatal(err)
	}
	all, _ = s.LocaleOverrides(ctx)
	if _, there := all["de"]; there {
		t.Errorf("a deleted language is still there: %v", all)
	}
}
