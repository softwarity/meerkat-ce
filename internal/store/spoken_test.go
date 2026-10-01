package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// What a gateway offers is the union of what its ROUTES say they speak.
//
// It used to be a pool an operator typed into a screen, with each route
// subtracting from it, which asked somebody to know something nobody knows.
// Deriving it means a route that arrives speaking a language brings it, and a
// route that leaves takes it away - with nothing to keep in step.
func TestTheOfferIsTheUnionOfWhatTheRoutesSpeak(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	save := func(id string, enabled bool, speaks ...string) {
		t.Helper()
		r := Route{
			ID: id, Name: id, Order: 1, Enabled: enabled, IsUI: true,
			Upstream:   "http://example.invalid",
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/" + id + "/**"}}}},
		}
		if speaks != nil {
			r.Locales = &LocalesConfig{Speaks: speaks}
		}
		if err := s.SaveRoute(ctx, r); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	spoken := func() string {
		t.Helper()
		got, err := s.SpokenLanguages(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(got, ",")
	}

	if got := spoken(); got != "" {
		t.Errorf("a gateway with no route speaks %q", got)
	}
	save("shop", true, "fr", "en")
	if got := spoken(); got != "en,fr" {
		t.Errorf("one route speaking fr and en gives %q", got)
	}
	// A second route OVERLAPS and adds: the union, not a list with duplicates.
	save("desk", true, "fr", "pl")
	if got := spoken(); got != "en,fr,pl" {
		t.Errorf("two routes give %q, want the union", got)
	}
	// A route nobody can reach speaks to nobody.
	save("desk", false, "fr", "pl")
	if got := spoken(); got != "en,fr" {
		t.Errorf("a disabled route still speaks: %q", got)
	}
	// And a route that declares nothing adds nothing rather than everything.
	save("api", true)
	if got := spoken(); got != "en,fr" {
		t.Errorf("a route declaring no language changed the offer: %q", got)
	}
	// Case is folded for the comparison, and the first spelling is kept: a tag
	// is compared case-insensitively and displayed as written.
	save("intl", true, "PT-br", "pt-BR")
	if got := spoken(); got != "PT-br,en,fr" {
		t.Errorf("two spellings of one tag give %q", got)
	}
}
