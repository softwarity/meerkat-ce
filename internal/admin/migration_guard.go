package admin

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/license"
	"github.com/softwarity/meerkat/internal/store"
)

// MigratePastLicenseVar is what lets a build the license does not cover
// migrate the database anyway.
const MigratePastLicenseVar = "MEERKAT_MIGRATE_PAST_LICENSE"

// MigrationGuard refuses to migrate the database for a build the license held
// does not cover.
//
// Such a build runs as an evaluation, and that is reversible on its own: going
// back to a build from the term puts the license back. A migration is not: the
// earlier build no longer opens the database (store.checkNotNewer), and the
// way back is the backup made before the upgrade, if there is one. So the step
// that cannot be undone is the one asked about, before it is taken, and the
// gateway stops with a sentence saying what to do rather than turning into an
// evaluation it cannot leave. Nothing is written: the old version starts again
// on the same database.
//
// Only that case. No license, a license refused, an ended test license, a
// build without a date (a dev build), a fresh database, or one with nothing to
// migrate: the guard says nothing, since none of them has a license to lose.
type MigrationGuard struct {
	// LicenseFile is MEERKAT_LICENSE_FILE: when set, it is the license.
	LicenseFile string
	// Keys verify the license, the production ones when nil.
	Keys []ed25519.PublicKey
	// Built is this build's date (version.Date), RFC 3339, empty for a dev build.
	Built string
	// Force is MEERKAT_MIGRATE_PAST_LICENSE: migrate anyway, and run as an
	// evaluation. The license is kept, untouched.
	Force bool
	Now   func() time.Time

	// forced is what a forced migration went past, for Audit to write once
	// the database is open.
	forced   *license.License
	from, to int
}

// Check is the store.MigrationCheck.
func (g *MigrationGuard) Check(ctx context.Context, from, to int, st *store.Store) error {
	if !edition.Enterprise {
		return nil
	}
	built, err := time.Parse(time.RFC3339, g.Built)
	if err != nil {
		return nil
	}
	now := time.Now()
	if g.Now != nil {
		now = g.Now()
	}
	lic := g.held(ctx, st, now)
	if lic == nil || lic.Ended(now) || lic.Covered(built) {
		return nil
	}
	until := lic.ExpiresAt.Format(time.DateOnly)
	if g.Force {
		g.forced, g.from, g.to = lic, from, to
		slog.Warn("database migrated past the license, as "+MigratePastLicenseVar+" asked: "+
			"this version runs as an evaluation, and an earlier one no longer opens the database",
			"licensee", lic.Licensee, "license", lic.ID, "updatesUntil", until,
			"built", built.Format(time.DateOnly), "schema", fmt.Sprintf("v%d to v%d", from, to))
		return nil
	}
	return fmt.Errorf("this version was built on %s, after the updates of the license of %s (%s) ended on %s, "+
		"and it has to migrate the database (schema v%d to v%d). Once migrated, an earlier version no longer opens it, "+
		"and this one would run as an evaluation: nothing was changed, so the version you ran before starts again as it was. "+
		"Run a version built by %s, or renew the license and start this one again. "+
		"To migrate anyway and run as an evaluation, take a backup first, then set %s=true",
		built.Format(time.DateOnly), lic.Licensee, lic.ID, until, from, to, until, MigratePastLicenseVar)
}

// held is the license in force, read the way the License screen reads it:
// the mounted file when there is one, the pasted one otherwise. A license that
// does not verify is none.
func (g *MigrationGuard) held(ctx context.Context, st *store.Store, now time.Time) *license.License {
	var data []byte
	if g.LicenseFile != "" {
		b, err := os.ReadFile(g.LicenseFile)
		if err != nil {
			return nil
		}
		data = b
	} else {
		text, err := st.License(ctx)
		if err != nil || text == "" {
			return nil
		}
		data = []byte(text)
	}
	keys := g.Keys
	if keys == nil {
		keys = license.ProductionKeys()
	}
	lic, err := license.Parse(data, keys, now)
	if err != nil {
		return nil
	}
	return lic
}

// Audit writes a forced migration to the trail, once the database is open: the
// guard runs before anything can be written.
func (g *MigrationGuard) Audit(ctx context.Context, st *store.Store) {
	if g.forced == nil {
		return
	}
	_ = st.AddAuditEvent(ctx, store.AuditEvent{
		At: time.Now().Unix(), Action: "license.migrate", Target: "license",
		TargetID: g.forced.ID, TargetName: g.forced.Licensee,
		Detail: fmt.Sprintf("schema v%d to v%d by a version built after the updates of this license ended on %s, "+
			"as %s asked: this version runs as an evaluation", g.from, g.to,
			g.forced.ExpiresAt.Format(time.DateOnly), MigratePastLicenseVar),
	})
}
