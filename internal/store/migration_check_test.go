package store

import (
	"context"
	"errors"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A migration can be refused before it starts, and a refusal leaves the
// database exactly where it was: the version that wrote it opens it again.
func TestAMigrationIsAskedAboutBeforeAnythingMoves(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	asked := 0
	check := func(_ context.Context, from, to int, st *Store) error {
		asked++
		if st == nil || to != schemaVersion {
			t.Fatalf("asked with from=%d to=%d", from, to)
		}
		if from != schemaVersion-1 {
			t.Fatalf("from %d, want %d", from, schemaVersion-1)
		}
		// Readable, the way the license is read before anything moves.
		if _, err := st.License(context.Background()); err != nil {
			t.Fatalf("the database cannot be read before the migration: %v", err)
		}
		return errors.New("not this one")
	}

	st, err := OpenChecked(dir, url, check)
	if err != nil {
		t.Fatalf("a fresh database has nothing to migrate and is not asked about: %v", err)
	}
	if asked != 0 {
		t.Fatal("a fresh database was asked about")
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if st, err = OpenChecked(dir, url, check); err != nil || asked != 0 {
		t.Fatalf("a database at this build's schema is not asked about: %v, asked %d", err, asked)
	}
	if err := st.db.setSchemaVersion(schemaVersion - 1); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenChecked(dir, url, check); err == nil || err.Error() != "not this one" || asked != 1 {
		t.Fatalf("a refused migration must stop the open with the reason: %v, asked %d", err, asked)
	}
	// Nothing moved: asked again, from the same version.
	if _, err := OpenChecked(dir, url, check); err == nil || asked != 2 {
		t.Fatalf("the refusal wrote something: %v, asked %d", err, asked)
	}
}
