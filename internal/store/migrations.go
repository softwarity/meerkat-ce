package store

import (
	"context"
	"fmt"
)

// The ledger: what a CREATE cannot say.
//
// Two halves keep a database current, and they answer different questions.
//
// The ADDITIVE half runs on every start and needs no ledger: the build's own
// DDL is applied with CREATE TABLE IF NOT EXISTS, then addMissingColumns adds
// any column the live database has not got. A new table, a new column, a new
// index - all of it lands without anybody writing a step, which is why the
// design phase needed nothing else.
//
// The other half is everything that half cannot express: a column renamed, a
// type changed, a value reshaped, a row backfilled, something dropped. Those
// are ORDERED, run ONCE, and recorded as they go - so an upgrade interrupted
// halfway resumes where it stopped rather than starting over on a database
// that is already part way there.
//
// WHY THE LEDGER IS EMPTY, AND WHY THAT IS NOT A HOLE. It opens at the schema
// this build carries, and it opens there because no version of this product
// has ever shipped: there is no database in the world older than the current
// schema except a developer's own, and a developer's own is disposable. From
// the first release on, every change that is not additive is a step here, and
// the day one lands this comment stops being true - which is the point of
// writing it down rather than leaving the slice bare.
type migration struct {
	// Version is what the database is stamped with once this step has run. It
	// must be greater than the schemaVersion of the build that shipped before
	// it, and the steps must be in increasing order.
	Version int
	// Name says what it does, in the log line an operator reads while their
	// gateway is down for the length of it.
	Name string
	Up   func(ctx context.Context, tx *transaction) error
}

var migrations []migration

// pending returns the steps a database at version `from` has not run, in the
// order they have to run in.
func pending(steps []migration, from int) []migration {
	var out []migration
	for _, m := range steps {
		if m.Version > from {
			out = append(out, m)
		}
	}
	return out
}

// runMigrations applies the pending steps, each in its own transaction, each
// recorded before the next begins.
//
// One transaction per step and not one for all of them: a step that has run is
// a fact, and wrapping ten of them in a single transaction means the tenth
// failing undoes nine that worked - on SQLite, where DDL is transactional but
// a long write blocks every reader, that is a worse outcome than stopping
// halfway and saying where.
func (s *Store) runMigrations(steps []migration, from int) error {
	for _, m := range pending(steps, from) {
		ctx := context.Background()
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("store: migration %d (%s): %w", m.Version, m.Name, err)
		}
		if err := m.Up(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: migration %d (%s): %w", m.Version, m.Name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: migration %d (%s): commit: %w", m.Version, m.Name, err)
		}
		if err := s.db.setSchemaVersion(m.Version); err != nil {
			return fmt.Errorf("store: migration %d (%s): recording it: %w", m.Version, m.Name, err)
		}
	}
	return nil
}

// checkNotNewer refuses to open a database a LATER build wrote.
//
// Rolling a binary back is a normal thing to do when a release goes wrong, and
// it is exactly when this matters: the older binary does not know the columns
// the newer one added, would write rows missing them, and would leave the
// database in a shape neither version can read. Refusing costs a restart;
// carrying on costs the data.
func checkNotNewer(found, known int) error {
	if found <= known {
		return nil
	}
	return fmt.Errorf(
		"store: this database was written by a newer Meerkat (schema v%d) and this build knows v%d. "+
			"Run the newer version, or restore the backup taken before the upgrade: "+
			"an older build cannot write a schema it does not know without losing what the newer one added",
		found, known)
}
