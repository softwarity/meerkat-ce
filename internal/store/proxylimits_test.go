package store

import "testing"

// A number quietly clamped on the way in is a screen that lies about what is
// running, so the refusal has to name the range - the house rule for every
// error in this package.
func TestTheBodyCeilingRefusesWhatCannotWorkAndSaysWhy(t *testing.T) {
	for _, n := range []int{-1, 0, 257, 100000} {
		p := ProxyLimits{BodyRewriteMiB: n}
		err := SanitizeProxyLimits(&p)
		if n == 0 {
			// Absent is not wrong: it means "whatever this build ships with".
			if err != nil || p.BodyRewriteMiB != DefaultBodyRewriteMiB {
				t.Errorf("an unset ceiling should become the default, got %d (%v)", p.BodyRewriteMiB, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%d MiB was accepted", n)
			continue
		}
		if !containsAll(err.Error(), "1", "256") {
			t.Errorf("the refusal of %d does not name the range: %v", n, err)
		}
	}
	for _, n := range []int{1, 20, 256} {
		p := ProxyLimits{BodyRewriteMiB: n}
		if err := SanitizeProxyLimits(&p); err != nil || p.BodyRewriteMiB != n {
			t.Errorf("%d MiB should be allowed: %v", n, err)
		}
	}
}

// A setting this build cannot read must not take the gateway's ceiling with
// it: proxying goes on, with what it always had.
func TestAnUnreadableSettingFallsBackToTheDefault(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()

	if got := s.GetProxyLimits(ctx); got != DefaultProxyLimits() {
		t.Fatalf("a fresh install should run the defaults, got %+v", got)
	}
	if err := s.SetSetting(ctx, SettingProxyLimits, "not a limit"); err != nil {
		t.Fatal(err)
	}
	if got := s.GetProxyLimits(ctx); got != DefaultProxyLimits() {
		t.Fatalf("unreadable setting: got %+v, want the defaults", got)
	}
	// And a stored value out of range - written by an older build with wider
	// bounds, say - is treated the same way.
	if err := s.SetSetting(ctx, SettingProxyLimits, ProxyLimits{BodyRewriteMiB: 9999}); err != nil {
		t.Fatal(err)
	}
	if got := s.GetProxyLimits(ctx); got != DefaultProxyLimits() {
		t.Fatalf("out-of-range setting: got %+v, want the defaults", got)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		found := false
		for i := 0; i+len(p) <= len(s); i++ {
			if s[i:i+len(p)] == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
