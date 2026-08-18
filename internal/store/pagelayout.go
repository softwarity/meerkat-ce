package store

import (
	"fmt"
	"slices"
	"strings"
)

// PageLayout is how the served pages are ARRANGED (PAGE-02) - where the brand
// sits, whether there is a card, what the background image is allowed to do.
// Global, like the theme and the branding: one sign-in procedure serves every
// application behind the gateway (THEME-01), so it has one shape.
//
// It is a setting of its own and NOT a field of the theme, although the theme
// already carries a look switch (Flat). The console shows the theme carousel
// while the layout is being chosen: were the layout part of the theme, trying
// another colour pill would rearrange the page under the eyes of whoever is
// comparing them. Colours and arrangement are two axes.
type PageLayout struct {
	Name string `json:"name"`
	// Side is which edge the brand takes, for the two layouts that have two
	// halves. Ignored - and cleared - by the others.
	Side string `json:"side,omitempty"`
}

// The catalogue, closed on purpose: each name is a block of CSS shipped with
// the product (internal/auth/layout.go). An integrator bringing their own HTML
// is the other half of PAGE-02 and is not this.
const (
	// LayoutCentered is the original and the default: the brand above, the
	// card in the middle, the background image behind everything.
	LayoutCentered = "centered"
	// LayoutSplit gives the image a full-height half of the screen, with the
	// brand on it, and the form the other half.
	LayoutSplit = "split"
	// LayoutDrawer keeps the image whole and lays an opaque panel against one
	// edge - the drawer pulled out, carrying brand and form.
	LayoutDrawer = "drawer"
	// LayoutBanner puts the brand in a band across the top, the card under it.
	LayoutBanner = "banner"
	// LayoutBare drops the card: the fields sit on the background itself.
	LayoutBare = "bare"
)

// PageLayouts is the catalogue in the order the console offers it.
var PageLayouts = []string{LayoutCentered, LayoutSplit, LayoutDrawer, LayoutBanner, LayoutBare}

// LayoutHasSide says whether a layout is built of two halves, and therefore
// whether the side means anything to it.
func LayoutHasSide(name string) bool {
	return name == LayoutSplit || name == LayoutDrawer
}

// DefaultPageLayout is what an installation that never chose gets: the
// arrangement every version until now served.
func DefaultPageLayout() PageLayout {
	return PageLayout{Name: LayoutCentered}
}

// SanitizePageLayout normalises and validates in place. An unknown name is an
// error that NAMES what is allowed - a refusal one cannot act on is a refusal
// that gets reported as a bug.
func SanitizePageLayout(l *PageLayout) error {
	l.Name = strings.TrimSpace(l.Name)
	if l.Name == "" {
		l.Name = LayoutCentered
	}
	if !slices.Contains(PageLayouts, l.Name) {
		return fmt.Errorf("page layout %q: allowed are %s", l.Name, strings.Join(PageLayouts, ", "))
	}
	l.Side = strings.TrimSpace(l.Side)
	// A side on a layout that has no halves would be a setting one can change
	// with nothing to show for it: cleared rather than kept and ignored.
	if !LayoutHasSide(l.Name) {
		l.Side = ""
		return nil
	}
	switch l.Side {
	case "":
		l.Side = "left"
	case "left", "right":
	default:
		return fmt.Errorf("page layout side %q: allowed are left, right", l.Side)
	}
	return nil
}
