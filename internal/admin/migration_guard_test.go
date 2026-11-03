package admin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/license"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The license of ACME covers updates to the end of 2027.
func guardFixture(t *testing.T, expires string) (*store.Store, *MigrationGuard, string) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	file, err := license.Sign(license.License{ID: "LIC-A", Licensee: "ACME", Edition: license.EditionTeam,
		Tenants: 1, Instances: 1, IssuedAt: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2027, 12, 31, 12, 0, 0, 0, time.UTC), Expires: expires}, priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetLicense(context.Background(), string(file)); err != nil {
		t.Fatal(err)
	}
	g := &MigrationGuard{Keys: []ed25519.PublicKey{pub}, Built: "2028-01-10T08:00:00Z",
		Now: func() time.Time { return time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC) }}
	return st, g, string(file)
}

// A version the license does not cover, which would migrate the database: the
// gateway stops before the step nobody can take back, and says what to do.
func TestAMigrationPastTheLicenseIsRefusedBeforeItRuns(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("community build: no license to lose")
	}
	ctx := context.Background()
	st, g, _ := guardFixture(t, "")
	err := g.Check(ctx, 102, 105, st)
	if err == nil {
		t.Fatal("a build past the license migrated the database")
	}
	for _, said := range []string{"built on 2028-01-10", "ACME (LIC-A)", "ended on 2027-12-31",
		"schema v102 to v105", "an earlier version no longer opens it", "nothing was changed",
		"Run a version built by 2027-12-31", "renew the license", "take a backup first",
		MigratePastLicenseVar + "=true"} {
		if !strings.Contains(err.Error(), said) {
			t.Errorf("the refusal does not say %q: %v", said, err)
		}
	}

	// The last day of the term counts to its end.
	g.Built = "2027-12-31T23:30:00Z"
	if err := g.Check(ctx, 102, 105, st); err != nil {
		t.Fatalf("a build from the last day is covered: %v", err)
	}
}

// Forced, it migrates and runs as an evaluation: the license is kept as it is,
// and the trail says who went past it and how far.
func TestAForcedMigrationIsWrittenToTheTrail(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("community build: no license to lose")
	}
	ctx := context.Background()
	st, g, file := guardFixture(t, "")
	g.Force = true
	if err := g.Check(ctx, 102, 105, st); err != nil {
		t.Fatalf("forced, the migration goes ahead: %v", err)
	}
	g.Audit(ctx, st)
	events, err := st.ListAuditEvents(ctx, store.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Action != "license.migrate" || events[0].TargetID != "LIC-A" ||
		!strings.Contains(events[0].Detail, "schema v102 to v105") ||
		!strings.Contains(events[0].Detail, MigratePastLicenseVar) {
		t.Fatalf("the trail: %+v", events)
	}
	if held, _ := st.License(ctx); held != file {
		t.Fatal("forcing the migration must leave the license as it was")
	}
}

// The guard speaks for one case only: a license in force that this build would
// leave behind for good. Everything else migrates as it always did.
func TestOnlyALicenseInForceStopsAMigration(t *testing.T) {
	ctx := context.Background()
	if !edition.Enterprise {
		st, g, _ := guardFixture(t, "")
		if err := g.Check(ctx, 102, 105, st); err != nil {
			t.Fatalf("the community build holds no license: %v", err)
		}
		return
	}
	cases := map[string]func(*store.Store, *MigrationGuard){
		"a build from the term": func(_ *store.Store, g *MigrationGuard) { g.Built = "2027-06-01T00:00:00Z" },
		"a dev build":           func(_ *store.Store, g *MigrationGuard) { g.Built = "" },
		"no license":            func(st *store.Store, _ *MigrationGuard) { _ = st.DeleteLicense(ctx) },
		"a license that does not verify": func(_ *store.Store, g *MigrationGuard) {
			other, _, _ := ed25519.GenerateKey(rand.Reader)
			g.Keys = []ed25519.PublicKey{other}
		},
	}
	for name, change := range cases {
		st, g, _ := guardFixture(t, "")
		change(st, g)
		if err := g.Check(ctx, 102, 105, st); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A test license that ended has nothing left to lose.
	st, g, _ := guardFixture(t, license.ExpiresHard)
	if err := g.Check(ctx, 102, 105, st); err != nil {
		t.Errorf("an ended test license: %v", err)
	}
}

// A mounted license is the license, as on the License screen.
func TestTheMountedLicenseIsTheOneAsked(t *testing.T) {
	if !edition.Enterprise {
		t.Skip("community build: no license to lose")
	}
	ctx := context.Background()
	st, g, file := guardFixture(t, "")
	_ = st.DeleteLicense(ctx)
	g.LicenseFile = filepath.Join(t.TempDir(), "meerkat.lic")
	if err := os.WriteFile(g.LicenseFile, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.Check(ctx, 102, 105, st); err == nil {
		t.Fatal("the mounted license was not read")
	}
}
