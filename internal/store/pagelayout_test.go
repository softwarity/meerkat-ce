package store

import (
	"strings"
	"testing"
)

// A refusal one cannot act on gets reported as a bug: an unknown layout must
// say what IS allowed, the way every other refusal in this product does.
func TestAnUnknownLayoutNamesTheOnesThatExist(t *testing.T) {
	l := PageLayout{Name: "carousel"}
	err := SanitizePageLayout(&l)
	if err == nil {
		t.Fatal("a layout nobody wrote was accepted")
	}
	for _, want := range PageLayouts {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal never mentions %q: %v", want, err)
		}
	}
}

// Empty is not an error: an installation that never chose keeps the
// arrangement every version until now served.
func TestNoChoiceIsTheOriginalArrangement(t *testing.T) {
	l := PageLayout{}
	if err := SanitizePageLayout(&l); err != nil {
		t.Fatalf("an unset layout was refused: %v", err)
	}
	if l.Name != LayoutCentered {
		t.Errorf("empty resolved to %q, want %q", l.Name, LayoutCentered)
	}
}

// A side on a layout with no halves is a control someone can move with
// nothing to show for it: cleared, not kept and quietly ignored.
func TestASideOnlySurvivesOnALayoutMadeOfHalves(t *testing.T) {
	l := PageLayout{Name: LayoutBanner, Side: "right"}
	if err := SanitizePageLayout(&l); err != nil {
		t.Fatal(err)
	}
	if l.Side != "" {
		t.Errorf("banner kept a side %q", l.Side)
	}

	// The two that DO have halves get a side even when none was given: which
	// edge the brand takes is not a question the pages can leave open.
	for _, name := range []string{LayoutSplit, LayoutDrawer} {
		l := PageLayout{Name: name}
		if err := SanitizePageLayout(&l); err != nil {
			t.Fatal(err)
		}
		if l.Side != "left" {
			t.Errorf("%s resolved to side %q, want left", name, l.Side)
		}
	}

	if err := SanitizePageLayout(&PageLayout{Name: LayoutSplit, Side: "middle"}); err == nil {
		t.Error("an edge that is not an edge was accepted")
	}
}
