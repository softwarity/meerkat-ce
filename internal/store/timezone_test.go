package store

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// v95: an account left at UTC - what an account got when nobody chose -
// follows the browser after the upgrade; a zone somebody actually chose stays.
func TestAZoneLeftAtUTCFollowsTheBrowserAfterTheUpgrade(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	ctx := context.Background()
	st, err := OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []User{
		{ID: "u-utc", Username: "left", PasswordHash: "h", Enabled: true},
		{ID: "u-paris", Username: "chose", PasswordHash: "h", Enabled: true},
	} {
		if err := st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	// What a v94 build stored: UTC by default, a name when somebody chose.
	if err := st.SetUserTimezone(ctx, "u-utc", "UTC"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserTimezone(ctx, "u-paris", "Europe/Paris"); err != nil {
		t.Fatal(err)
	}
	if err := st.db.setSchemaVersion(94); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatalf("opening the older database: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if v, _ := st.db.schemaVersion(); v != schemaVersion {
		t.Errorf("schema v%d after the upgrade, want v%d", v, schemaVersion)
	}
	for id, want := range map[string]string{"u-utc": "", "u-paris": "Europe/Paris"} {
		u, err := st.GetUserByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if u.Timezone != want {
			t.Errorf("%s: zone %q after the upgrade, want %q", id, u.Timezone, want)
		}
	}
	// And a choice made after the upgrade is kept, UTC included: the step
	// ran once, it does not undo somebody picking UTC on purpose.
	if err := st.SetUserTimezone(ctx, "u-utc", "UTC"); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if u, err := st.GetUserByID(ctx, "u-utc"); err != nil || u.Timezone != "UTC" {
		t.Errorf("UTC chosen after the upgrade: %q, %v", u.Timezone, err)
	}
}
