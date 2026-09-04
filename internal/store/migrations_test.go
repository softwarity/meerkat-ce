package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A database a LATER build wrote is refused, and the refusal names both
// versions. Rolling a binary back is a normal thing to do when a release goes
// wrong, and it is exactly when this matters.
func TestARollbackRefusesToOpenANewerDatabase(t *testing.T) {
	if err := checkNotNewer(55, 55); err != nil {
		t.Errorf("same version must open: %v", err)
	}
	if err := checkNotNewer(40, 55); err != nil {
		t.Errorf("an older database must open, that is the whole point: %v", err)
	}
	err := checkNotNewer(56, 55)
	if err == nil {
		t.Fatal("a database written by a newer build must be refused")
	}
	for _, want := range []string{"v56", "v55"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %s: %q", want, err)
		}
	}
}

func TestOnlyPendingStepsRun(t *testing.T) {
	steps := []migration{{Version: 10}, {Version: 20}, {Version: 30}}
	got := pending(steps, 20)
	if len(got) != 1 || got[0].Version != 30 {
		t.Errorf("from 20: %v, want only 30", versions(got))
	}
	if n := len(pending(steps, 0)); n != 3 {
		t.Errorf("from 0: %d steps, want 3", n)
	}
	if n := len(pending(steps, 30)); n != 0 {
		t.Errorf("from 30: %d steps, want none", n)
	}
}

func versions(ms []migration) []int {
	out := make([]int, len(ms))
	for i, m := range ms {
		out[i] = m.Version
	}
	return out
}

// The upgrade, run for real: an existing database, a step that reshapes data,
// and the proof that it happened exactly once - because the version is
// recorded as it goes, and a second start must not run it again.
func TestAStepRunsOnceAndIsRecorded(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenAt(dir, dbtest.URL(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Pretend this database predates the step.
	before := schemaVersion
	if err := st.db.setSchemaVersion(before - 1); err != nil {
		t.Fatalf("stamp: %v", err)
	}

	runs := 0
	steps := []migration{{
		Version: before,
		Name:    "count the times it ran",
		Up: func(ctx context.Context, tx *transaction) error {
			runs++
			// A real step writes; this one proves the transaction works.
			_, err := tx.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, "does-not-exist")
			return err
		},
	}}

	v, err := st.db.schemaVersion()
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if err := st.runMigrations(steps, v); err != nil {
		t.Fatalf("first upgrade: %v", err)
	}
	if runs != 1 {
		t.Fatalf("ran %d times on the first upgrade, want 1", runs)
	}
	if got, _ := st.db.schemaVersion(); got != before {
		t.Fatalf("version %d after the step, want %d recorded as it went", got, before)
	}

	// The second start: the step is behind us now, and running it again is
	// how an upgrade that reshapes data reshapes it twice.
	v, _ = st.db.schemaVersion()
	if err := st.runMigrations(steps, v); err != nil {
		t.Fatalf("second start: %v", err)
	}
	if runs != 1 {
		t.Errorf("ran %d times over two starts, want 1", runs)
	}
}

// A failing step leaves the version where it was, so the next start retries it
// rather than stepping over a database it never finished changing.
func TestAFailedStepIsNotRecorded(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenAt(dir, dbtest.URL(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = st.Close() }()

	before := schemaVersion
	if err := st.db.setSchemaVersion(before - 1); err != nil {
		t.Fatalf("stamp: %v", err)
	}
	steps := []migration{{
		Version: before,
		Name:    "one that cannot work",
		Up: func(ctx context.Context, tx *transaction) error {
			_, err := tx.ExecContext(ctx, `UPDATE no_such_table SET x = 1`)
			return err
		},
	}}
	err = st.runMigrations(steps, before-1)
	if err == nil {
		t.Fatal("a step that fails must stop the upgrade")
	}
	if !strings.Contains(err.Error(), "one that cannot work") {
		t.Errorf("the error must name the step: %q", err)
	}
	if got, _ := st.db.schemaVersion(); got != before-1 {
		t.Errorf("version %d after a failed step, want %d left alone", got, before-1)
	}
}

// The ledger's own shape: increasing, and never below the schema the current
// build carries minus itself. A step numbered under an already-shipped version
// never runs on the databases that needed it.
func TestTheLedgerIsOrdered(t *testing.T) {
	last := 0
	for _, m := range migrations {
		if m.Version <= last {
			t.Errorf("step %d (%s) does not come after %d: the order IS the contract",
				m.Version, m.Name, last)
		}
		if m.Name == "" {
			t.Errorf("step %d has no name, and its name is what an operator reads while it runs", m.Version)
		}
		if m.Up == nil {
			t.Errorf("step %d (%s) does nothing", m.Version, m.Name)
		}
		last = m.Version
	}
	if last > schemaVersion {
		t.Errorf("the ledger reaches v%d and the build carries v%d: bump schemaVersion", last, schemaVersion)
	}
}
