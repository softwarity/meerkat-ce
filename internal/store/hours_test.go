package store

import (
	"context"
	"testing"
	"time"
)

// When an open window closes: the end of the range in force, inclusive to the
// minute; a closing that runs straight into the next open range is not one;
// a last day closes at its midnight.
func TestBusinessAccessCloses(t *testing.T) {
	tue := time.Date(2026, 10, 20, 17, 55, 0, 0, time.UTC) // a Tuesday
	for _, c := range []struct {
		name string
		ba   BusinessAccess
		at   time.Time
		want time.Time
	}{
		{"weekday hours", BusinessAccess{Timezone: "UTC", Days: []DayRange{{Day: 2, From: "09:00", To: "18:00"}}},
			tue, time.Date(2026, 10, 20, 18, 1, 0, 0, time.UTC)},
		{"through midnight", BusinessAccess{Timezone: "UTC", Days: []DayRange{
			{Day: 2, From: "00:00", To: "23:59"}, {Day: 3, From: "00:00", To: "12:00"}}},
			tue, time.Date(2026, 10, 21, 12, 1, 0, 0, time.UTC)},
		{"last day", BusinessAccess{Timezone: "UTC", DateTo: "2026-10-20"},
			tue, time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)},
		{"in its own time zone", BusinessAccess{Timezone: "Asia/Bangkok", Days: []DayRange{{Day: 3, From: "00:00", To: "06:00"}}},
			tue, time.Date(2026, 10, 20, 23, 1, 0, 0, time.UTC)},
		{"never", BusinessAccess{Timezone: "UTC"}, tue, time.Time{}},
	} {
		got, err := businessAccessCloses(c.ba, c.at)
		if err != nil || !got.Equal(c.want) {
			t.Errorf("%s: closes %v (%v), want %v", c.name, got, err, c.want)
		}
	}
}

// The most precise window set wins whole: membership, then account, then
// organisation, then gateway. An override is another rule, not a narrower one:
// a member given evenings gets in at night when its organisation closes at 18.
func TestTheMostPreciseWindowWins(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	day := func(from, to string) BusinessAccess {
		return BusinessAccess{Timezone: "UTC", Days: []DayRange{{Day: 2, From: from, To: to}}}
	}
	if err := s.SetSetting(ctx, SettingBusinessAccess, day("06:00", "22:00")); err != nil {
		t.Fatal(err)
	}
	for _, u := range []User{
		{ID: "p", Username: "plain", PasswordHash: "x", Enabled: true},
		{ID: "a", Username: "part-time", PasswordHash: "x", Enabled: true, BusinessAccess: day("09:00", "12:00")},
		{ID: "r", Username: "boss", PasswordHash: "x", Enabled: true, Root: true},
	} {
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveTenant(ctx, Tenant{ID: "t", Name: "acme", Enabled: true, BusinessAccess: day("08:00", "18:00")}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"p", "a", "r"} {
		if err := s.SaveMembership(ctx, Membership{UserID: id, TenantID: "t", Type: MemberUser, Enabled: true,
			BusinessAccess: BusinessAccess{Inherited: true}}); err != nil {
			t.Fatal(err)
		}
	}
	from := func(user, tenant string) string {
		t.Helper()
		ba, err := s.ResolveBusinessAccess(ctx, user, tenant, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(ba.Days) == 0 {
			return "open"
		}
		return ba.Days[0].From
	}
	for _, c := range []struct{ user, tenant, want string }{
		{"p", "", "06:00"},  // no organisation: the gateway's
		{"a", "", "09:00"},  // the account's own
		{"p", "t", "08:00"}, // the organisation's over the gateway's
		{"a", "t", "09:00"}, // the account's over the organisation's
		{"r", "t", "open"},  // root, never held
	} {
		if got := from(c.user, c.tenant); got != c.want {
			t.Errorf("%s in %q: hours from %s, want %s", c.user, c.tenant, got, c.want)
		}
	}
	if err := s.SaveMembership(ctx, Membership{UserID: "a", TenantID: "t", Type: MemberUser, Enabled: true,
		BusinessAccess: day("18:00", "23:00")}); err != nil {
		t.Fatal(err)
	}
	if got := from("a", "t"); got != "18:00" {
		t.Errorf("the member's own hours lost to %s", got)
	}
	ba, _ := s.ResolveBusinessAccess(ctx, "a", "t", "")
	if open, _, _ := OpenBusinessAccess(ba, time.Date(2026, 10, 20, 20, 0, 0, 0, time.UTC)); !open {
		t.Error("the member's evening hours were narrowed by its organisation's")
	}
}

// A member's access dates hold when its hours are its organisation's, and when
// those are the gateway's: they bound the person, not the hours.
func TestMemberDatesHoldWhenItsHoursAreInherited(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.CreateUser(ctx, User{ID: "c", Username: "contractor", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTenant(ctx, Tenant{ID: "t", Name: "acme", Enabled: true, BusinessAccess: BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMembership(ctx, Membership{UserID: "c", TenantID: "t", Type: "USER", Enabled: true,
		BusinessAccess: BusinessAccess{Inherited: true, DateTo: "2026-10-31"}}); err != nil {
		t.Fatal(err)
	}
	ba, err := s.ResolveBusinessAccess(ctx, "c", "t", "")
	if err != nil || ba.DateTo != "2026-10-31" {
		t.Fatalf("the member's last day was lost on the way: %+v %v", ba, err)
	}
}

// A group's own hours hold between the member's and the account's, and only
// in exclusive mode, where one group is chosen per session: a night shift
// gets in at night while the account's own hours would keep it out.
func TestAGroupsHoursHoldInExclusiveModeOnly(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	day := func(from, to string) BusinessAccess {
		return BusinessAccess{Timezone: "UTC", Days: []DayRange{{Day: 2, From: from, To: to}}}
	}
	if err := s.CreateUser(ctx, User{ID: "n", Username: "nina", PasswordHash: "x", Enabled: true, BusinessAccess: day("09:00", "12:00")}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTenant(ctx, Tenant{ID: "t", Name: "acme", Enabled: true, BusinessAccess: BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMembership(ctx, Membership{UserID: "n", TenantID: "t", Type: MemberUser, Enabled: true,
		BusinessAccess: BusinessAccess{Inherited: true}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveGroup(ctx, Group{ID: "g", TenantID: "t", Name: "night", BusinessAccess: day("20:00", "23:59")}); err != nil {
		t.Fatal(err)
	}
	from := func() string {
		t.Helper()
		ba, err := s.ResolveBusinessAccess(ctx, "n", "t", "g")
		if err != nil {
			t.Fatal(err)
		}
		return ba.Days[0].From
	}
	if got := from(); got != "09:00" {
		t.Fatalf("cumulative mode: hours from %s, want the account's 09:00", got)
	}
	if err := s.SetGroupMode(ctx, "t", GroupModeSingle); err != nil {
		t.Fatal(err)
	}
	if got := from(); got != "20:00" {
		t.Fatalf("exclusive mode: hours from %s, want the group's 20:00", got)
	}
	if err := s.SaveMembership(ctx, Membership{UserID: "n", TenantID: "t", Type: MemberUser, Enabled: true,
		BusinessAccess: day("06:00", "08:00")}); err != nil {
		t.Fatal(err)
	}
	if got := from(); got != "06:00" {
		t.Fatalf("the member's own hours lost to %s", got)
	}
}
