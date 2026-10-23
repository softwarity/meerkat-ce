package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestAFreshInstallationAsksNothing: no setting is None for everyone, and
// nothing is written to say so.
func TestAFreshInstallationAsksNothing(t *testing.T) {
	st := openTemp(t)
	a := st.InternalAccess(context.Background())
	if a.Mode != InternalAccessNone || len(a.Callers) != 0 {
		t.Fatalf("a fresh installation: %+v", a)
	}
}

// TestAnUpgradeKeepsTheTokensItHandedOut: the v90 step, run for real on a
// database stamped by the previous build. One that holds a schedules token
// keeps asking for one; one that holds none gets None like a fresh one; one
// that already chose keeps its choice.
func TestAnUpgradeKeepsTheTokensItHandedOut(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name    string
		scope   string // the token held, "" for none
		chosen  string // a setting already stored, "" for none
		want    string
		written bool
	}{
		{name: "a schedules token", scope: ScopeSchedules, want: InternalAccessToken, written: true},
		{name: "an operator's token only", scope: ScopeReadOnly, want: InternalAccessNone},
		{name: "no token at all", want: InternalAccessNone},
		{name: "a choice already made", scope: ScopeSchedules, chosen: InternalAccessNone, want: InternalAccessNone, written: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, url := t.TempDir(), dbtest.URL(t)
			st, err := OpenAt(dir, url)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.CreateUser(ctx, User{ID: "root", Username: "root", PasswordHash: "x", Root: true, Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if c.scope != "" {
				if err := st.AddAPIToken(ctx, NewToken{ID: "t1", UserID: "root", Name: "stations", TokenHash: "h",
					Prefix: "mk_", Plane: PlaneAdmin, Scope: c.scope}); err != nil {
					t.Fatal(err)
				}
			}
			if c.chosen != "" {
				if err := st.SetInternalAccess(ctx, InternalAccess{Mode: c.chosen}); err != nil {
					t.Fatal(err)
				}
			}
			// The database as the build before v90 left it.
			if err := st.db.setSchemaVersion(89); err != nil {
				t.Fatal(err)
			}
			_ = st.Close()

			st, err = OpenAt(dir, url)
			if err != nil {
				t.Fatalf("the upgrade: %v", err)
			}
			defer func() { _ = st.Close() }()
			if got := st.InternalAccess(ctx).Mode; got != c.want {
				t.Fatalf("mode %q after the upgrade, want %q", got, c.want)
			}
			var raw InternalAccess
			written := st.GetSetting(ctx, SettingInternalAccess, &raw) == nil
			if written != c.written {
				t.Fatalf("setting written: %v, want %v", written, c.written)
			}
		})
	}
}

// TestTheListAdmitsWhatItNames: empty is everyone, identified or not; a list
// admits only a name it holds, without regard to case, and never nobody.
func TestTheListAdmitsWhatItNames(t *testing.T) {
	everyone := InternalAccess{Mode: InternalAccessNone}
	if !everyone.Admits("") || !everyone.Admits("anything") {
		t.Fatal("no list must admit everyone, the unidentified included")
	}
	listed := InternalAccess{Mode: InternalAccessNone, Callers: []string{"canopy/Deployment/reference-data"}}
	if listed.Admits("") || listed.Admits() {
		t.Fatal("a list admitted an unidentified caller")
	}
	if !listed.Admits("", "Canopy/Deployment/Reference-Data") {
		t.Fatal("a listed identity was refused")
	}
	if listed.Admits("canopy/Deployment/other") {
		t.Fatal("an identity the list does not hold was admitted")
	}
}

// TestTheSettingIsSanitised: a mode nobody knows is refused naming the three,
// and the list is trimmed and deduplicated.
func TestTheSettingIsSanitised(t *testing.T) {
	a := InternalAccess{Mode: " Token ", Callers: []string{" stations", "stations", "", "STATIONS", "billing"}}
	if err := SanitizeInternalAccess(&a); err != nil {
		t.Fatal(err)
	}
	if a.Mode != InternalAccessToken || strings.Join(a.Callers, ",") != "stations,billing" {
		t.Fatalf("%+v", a)
	}
	bad := InternalAccess{Mode: "open"}
	if err := SanitizeInternalAccess(&bad); err == nil || !strings.Contains(err.Error(), "none, token, cluster") {
		t.Fatalf("an unknown mode: %v", err)
	}
	empty := InternalAccess{}
	if err := SanitizeInternalAccess(&empty); err != nil || empty.Mode != InternalAccessNone || empty.Callers == nil {
		t.Fatalf("an empty setting: %+v %v", empty, err)
	}
}

// TestTheTokensAreListedByName: every account's schedules tokens, enabled,
// by name - not the operators' tokens.
func TestTheTokensAreListedByName(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	if err := st.CreateUser(ctx, User{ID: "root", Username: "root", PasswordHash: "x", Root: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for i, tok := range []NewToken{
		{Name: "stations", Scope: ScopeSchedules},
		{Name: "billing", Scope: ScopeSchedules},
		{Name: "agent", Scope: ScopeReadOnly},
	} {
		tok.ID, tok.UserID, tok.TokenHash, tok.Prefix, tok.Plane = "t"+string(rune('a'+i)), "root", "h"+tok.Name, "mk_", PlaneAdmin
		if err := st.AddAPIToken(ctx, tok); err != nil {
			t.Fatal(err)
		}
	}
	names, err := st.ScheduleTokenNames(ctx)
	if err != nil || strings.Join(names, ",") != "billing,stations" {
		t.Fatalf("%v %v", names, err)
	}
}
