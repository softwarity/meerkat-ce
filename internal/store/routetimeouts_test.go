package store

import (
	"strings"
	"testing"
	"time"
)

// A route that never asked keeps what the whole product ran with when the
// bounds were global - which is what makes this change invisible to every
// existing installation.
func TestNoBoundsMeansTheDefaults(t *testing.T) {
	for _, tt := range []*RouteTimeouts{nil, {}, {Connect: "", Response: ""}} {
		c, r := tt.Durations(RouteTimeouts{})
		if c != DefaultConnectTimeout || r != DefaultResponseTimeout {
			t.Errorf("%+v gave %v/%v, want %v/%v", tt, c, r, DefaultConnectTimeout, DefaultResponseTimeout)
		}
	}
}

// Each bound is its own: naming one must not silently move the other.
func TestOneBoundNamedLeavesTheOtherAlone(t *testing.T) {
	c, r := (&RouteTimeouts{Response: "PT2M"}).Durations(RouteTimeouts{})
	if c != DefaultConnectTimeout {
		t.Errorf("connect became %v", c)
	}
	if r != 2*time.Minute {
		t.Errorf("response = %v, want 2m", r)
	}
}

// The refusal names the range, like every other error in this package - and
// there IS a maximum, because these are the guard rails: a response bound of
// an hour is a route with no bound at all, written to look like one.
func TestBoundsOutsideWhatAGatewayCanPromiseAreRefused(t *testing.T) {
	cases := map[string]RouteTimeouts{
		"connect too long":  {Connect: "PT5M"},
		"response too long": {Response: "PT2H"},
		"below a second":    {Connect: "PT0S"},
		"not ISO":           {Response: "30s"},
	}
	for what, tt := range cases {
		err := SanitizeRouteTimeouts(&tt)
		if err == nil {
			t.Errorf("%s was accepted", what)
			continue
		}
		if !strings.Contains(err.Error(), "connect") && !strings.Contains(err.Error(), "response") {
			t.Errorf("%s: the refusal does not say which bound: %v", what, err)
		}
	}
	ok := RouteTimeouts{Connect: "PT10S", Response: "PT5M"}
	if err := SanitizeRouteTimeouts(&ok); err != nil {
		t.Errorf("a legitimate pair was refused: %v", err)
	}
}

// The rule lives in SaveRoute, where every writer passes - the console, the
// agent, three OpenAPI paths and a configuration import.
func TestSavingARouteRefusesImpossibleBounds(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()

	r := Route{ID: "r1", Name: "api", Enabled: true, Upstream: "https://example.com",
		Timeouts: &RouteTimeouts{Response: "PT3H"}}
	if err := s.SaveRoute(ctx, r); err == nil {
		t.Fatal("a three-hour bound was saved")
	}

	r.Timeouts = &RouteTimeouts{Connect: "PT2S", Response: "PT45S"}
	if err := s.SaveRoute(ctx, r); err != nil {
		t.Fatal(err)
	}
	back, err := s.GetRoute(ctx, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if back.Timeouts == nil || back.Timeouts.Connect != "PT2S" || back.Timeouts.Response != "PT45S" {
		t.Fatalf("the bounds did not come back: %+v", back.Timeouts)
	}
}

// Three layers, narrowest first: what the route asked for, then what the
// INSTALLATION set, then the product's own. Two constants compiled into a
// binary cannot know what counts as slow on somebody else's network.
func TestARouteOverridesTheInstallationWhichOverridesTheProduct(t *testing.T) {
	house := RouteTimeouts{Connect: "PT3S", Response: "PT45S"}

	// Nothing on the route: the installation's.
	c, r := (*RouteTimeouts)(nil).Durations(house)
	if c != 3*time.Second || r != 45*time.Second {
		t.Errorf("no route bounds gave %v/%v, want the installation's 3s/45s", c, r)
	}

	// One named: that one is the route's, the other stays the installation's -
	// each bound falls back on its own, or naming one would quietly move the
	// other.
	c, r = (&RouteTimeouts{Response: "PT5S"}).Durations(house)
	if c != 3*time.Second || r != 5*time.Second {
		t.Errorf("got %v/%v, want 3s (house) / 5s (route)", c, r)
	}

	// And with no installation setting either, the product's own.
	c, r = (*RouteTimeouts)(nil).Durations(RouteTimeouts{})
	if c != DefaultConnectTimeout || r != DefaultResponseTimeout {
		t.Errorf("got %v/%v, want the product's %v/%v", c, r, DefaultConnectTimeout, DefaultResponseTimeout)
	}
}
