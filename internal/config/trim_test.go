package config

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// An export should read as the decisions someone made. Saving a route writes
// its whole shape, defaults included - a button nobody configured comes back
// as height 24, padX 12, shape round - and a reader then has to know every
// default to know which lines mean anything.
func TestExportDropsUntouchedDefaults(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)

	if err := st.SaveRoute(ctx, store.Route{
		ID: "r1", Name: "app", Enabled: true, IsUI: true, Upstream: "http://x:1",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}},
		UI: &store.RouteUI{
			// Everything at its default, except the one thing that was chosen.
			UserButton: store.UserButton{Enabled: true, Height: 24, Position: "top-left", Shape: "round", PadX: 12, PadY: 12},
			Scheme:     &store.SchemeConfig{},
			Roles:      &store.RolesConfig{Enabled: false},
		},
		Identity: &store.IdentityForward{Mechanism: "headers", Attributes: []store.IdentityAttr{
			{Field: "username", As: "username"}, // says nothing: that IS the name
			{Field: "email", As: "X-Mail"},      // a real decision
		}},
	}); err != nil {
		t.Fatal(err)
	}

	doc, _, err := Export(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	r := doc.Routes[0]

	b := r.UI.UserButton
	if b.Height != 0 || b.Shape != "" || b.PadX != 0 || b.PadY != 0 {
		t.Errorf("untouched button defaults survived: %+v", b)
	}
	if b.Position != "top-left" {
		t.Errorf("a chosen position was dropped: %q", b.Position)
	}
	if r.UI.Scheme != nil || r.UI.Roles != nil {
		t.Errorf("empty ui blocks survived: scheme=%v roles=%v", r.UI.Scheme, r.UI.Roles)
	}
	if got := r.Identity.Attributes[0].As; got != "" {
		t.Errorf("a mapping that repeats the field name survived: %q", got)
	}
	if got := r.Identity.Attributes[1].As; got != "X-Mail" {
		t.Errorf("a real mapping was dropped: %q", got)
	}

	// Trimming is safe because ZERO ALREADY MEANS "the default" everywhere in
	// the product: the gateway reads height 0 and serves 24 (router.go), which
	// is why an untouched 24 and an absent height are the same route. What must
	// survive the round trip is every DECISION.
	dst := openTemp(t)
	if _, err := Apply(ctx, dst, doc, false); err != nil {
		t.Fatal(err)
	}
	back, err := dst.GetRoute(ctx, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if back.UI.UserButton.Position != "top-left" || !back.UI.UserButton.Enabled {
		t.Errorf("a decision did not survive the round trip: %+v", back.UI.UserButton)
	}
	if len(back.Identity.Attributes) != 2 || back.Identity.Attributes[1].As != "X-Mail" {
		t.Errorf("the identity mapping did not survive: %+v", back.Identity.Attributes)
	}
}
