package store

import (
	"context"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The trap: Go decodes a missing "businessAccess" as the zero value, whose
// Inherited is FALSE. The resolution reads that as "this level overrides, with
// no restriction" and stops - so a membership created by the API, an import or
// a group rule silently escaped the hours set on its organisation, and the
// organisation escaped the global ones. Nothing on screen said so.
func TestABusinessWindowNobodyStatedIsInherited(t *testing.T) {
	var zero BusinessAccess
	SanitizeBusinessAccess(&zero)
	if !zero.Inherited {
		t.Error("a window with nothing in it must inherit, not override with nothing")
	}

	// An EXPLICIT "no hours for this person" stays expressible: a timezone
	// with no days says somebody opened the editor and cleared it.
	explicit := BusinessAccess{Timezone: "Europe/Paris"}
	SanitizeBusinessAccess(&explicit)
	if explicit.Inherited {
		t.Error("an explicit empty window must stay an override")
	}

	// And an inherited one carries nothing of its own: a leftover would come
	// back to life the day somebody unticks the box.
	leftover := BusinessAccess{Inherited: true, Timezone: "UTC", Days: []DayRange{{Day: 1, From: "08:00", To: "18:00"}}}
	SanitizeBusinessAccess(&leftover)
	if leftover.Timezone != "" || len(leftover.Days) != 0 {
		t.Errorf("an inherited window kept %+v", leftover)
	}
}

// End to end: hours set on an organisation apply to somebody joined through
// the API, which is what they visibly did not do.
func TestOrganisationHoursReachAMemberJoinedByTheAPI(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	if err := st.CreateUser(ctx, User{ID: "u-alice", Username: "alice", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	// The organisation opens on Mondays, 08:00 to 18:00 UTC.
	if err := st.SaveTenant(ctx, Tenant{
		ID: "acme", Name: "acme", Enabled: true,
		BusinessAccess: BusinessAccess{Timezone: "UTC", Days: []DayRange{{Day: 1, From: "08:00", To: "18:00"}}},
	}); err != nil {
		t.Fatal(err)
	}
	// Joined with no window mentioned - the shape an API call, an import or a
	// group rule produces.
	if err := st.SaveMembership(ctx, Membership{
		UserID: "u-alice", TenantID: "acme", Type: MemberUser, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	ba, err := st.ResolveBusinessAccess(ctx, "u-alice", "acme")
	if err != nil {
		t.Fatal(err)
	}
	if len(ba.Days) == 0 {
		t.Fatalf("the organisation's hours did not reach the member: %+v", ba)
	}
	monday10 := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC) // a Monday, inside
	monday23 := time.Date(2026, 8, 31, 23, 0, 0, 0, time.UTC) // the same Monday, outside
	if ok, err := WithinBusinessAccess(ba, monday10); err != nil || !ok {
		t.Errorf("inside the window: ok=%v err=%v", ok, err)
	}
	if ok, err := WithinBusinessAccess(ba, monday23); err != nil || ok {
		t.Errorf("outside the window she was let in anyway: ok=%v err=%v", ok, err)
	}
}
