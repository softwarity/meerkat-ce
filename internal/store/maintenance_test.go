package store

import (
	"strings"
	"testing"
	"time"
)

// The failure mode this feature actually has is not "the switch does not
// work": it is a Tuesday closure still on on Thursday. The console can only
// ask about that if the moment it went on is recorded - and kept, so that
// changing the reason halfway through does not make it look like it just
// started.
func TestTheMomentItWentOnSurvivesAnEdit(t *testing.T) {
	tuesday := time.Unix(1_700_000_000, 0)
	thursday := tuesday.Add(48 * time.Hour)

	on := Maintenance{Enabled: true, Reason: ReasonMaintenance}
	if err := SanitizeMaintenance(&on, Maintenance{}, tuesday); err != nil {
		t.Fatal(err)
	}
	if on.Since != tuesday.Unix() {
		t.Fatalf("Since = %d, want %d", on.Since, tuesday.Unix())
	}

	// Two days later somebody changes what is being said. It has been closed
	// since Tuesday.
	edited := Maintenance{Enabled: true, Reason: ReasonIncident}
	if err := SanitizeMaintenance(&edited, on, thursday); err != nil {
		t.Fatal(err)
	}
	if edited.Since != tuesday.Unix() {
		t.Errorf("changing the reason restarted the clock: %d, want %d", edited.Since, tuesday.Unix())
	}

	// Off clears everything, so the next closure is not dated - or explained -
	// by the last.
	off := Maintenance{Enabled: false, Reason: ReasonIncident, Until: thursday.Unix()}
	if err := SanitizeMaintenance(&off, edited, thursday); err != nil {
		t.Fatal(err)
	}
	if off.Since != 0 || off.Until != 0 || off.Reason != ReasonNone {
		t.Errorf("switching off left something behind: %+v", off)
	}
}

// The reason is a KEY, not a sentence: the page is read in twenty languages,
// and anything not in the catalogue would reach a visitor as debris. The
// refusal names what is allowed, like every other error in this package.
func TestOnlyAReasonThisProductCanSayIsAccepted(t *testing.T) {
	// A near miss, a sentence, the wrong case, and a plausible word that is
	// not in the list - the four ways a reason arrives wrong.
	for _, bad := range []string{"maintenace", "Because", "MAINTENANCE", "planned"} { //nolint:misspell // a typo is the point
		m := Maintenance{Enabled: true, Reason: bad}
		err := SanitizeMaintenance(&m, Maintenance{}, time.Now())
		if err == nil {
			t.Errorf("%q was accepted", bad)
			continue
		}
		for _, name := range []string{"maintenance", "upgrade", "incident"} {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("the refusal of %q does not name %q: %v", bad, name, err)
			}
		}
	}
	for _, ok := range []string{ReasonNone, ReasonMaintenance, ReasonUpgrade, ReasonIncident} {
		m := Maintenance{Enabled: true, Reason: ok}
		if err := SanitizeMaintenance(&m, Maintenance{}, time.Now()); err != nil {
			t.Errorf("%q should be allowed: %v", ok, err)
		}
	}
}

// How long is OPTIONAL and CHOSEN: empty means "we are not saying when", which
// the page reads as soon. The instant is computed here rather than accepted
// from a client - which is what makes "must be in the future" a rule nobody
// has to write.
func TestHowLongIsOptionalAndBecomesAnInstant(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	none := Maintenance{Enabled: true}
	if err := SanitizeMaintenance(&none, Maintenance{}, now); err != nil || none.Until != 0 {
		t.Errorf("saying nothing should be allowed: %d (%v)", none.Until, err)
	}

	day := Maintenance{Enabled: true, For: "P1D"}
	if err := SanitizeMaintenance(&day, Maintenance{}, now); err != nil {
		t.Fatal(err)
	}
	if want := now.Add(24 * time.Hour).Unix(); day.Until != want {
		t.Errorf("P1D landed on %d, want %d", day.Until, want)
	}
	if day.For != "P1D" {
		t.Errorf("the choice was not kept: %q", day.For)
	}

	// Re-anchored from NOW on every save, which is what somebody pushing a
	// window back means by it.
	later := now.Add(3 * time.Hour)
	again := Maintenance{Enabled: true, For: "P1D"}
	if err := SanitizeMaintenance(&again, day, later); err != nil {
		t.Fatal(err)
	}
	if want := later.Add(24 * time.Hour).Unix(); again.Until != want {
		t.Errorf("saving again did not re-anchor: %d, want %d", again.Until, want)
	}

	// And a duration this product cannot read is refused with the forms it can.
	bad := Maintenance{Enabled: true, For: "one day"}
	err := SanitizeMaintenance(&bad, Maintenance{}, now)
	if err == nil {
		t.Fatal("a free-text duration was accepted")
	}
	if !strings.Contains(err.Error(), "PT2H") {
		t.Errorf("the refusal does not show a form that works: %v", err)
	}
}
