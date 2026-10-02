package store

import (
	"context"
	"fmt"
	"strings"
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
// THE DAY HAS COME. This comment used to say the ledger was empty because no
// version had ever shipped, and that every non-additive change would be a step
// here from the first release on. v1.0.0 shipped on 2026-10-02 and v71 is the
// first such change, so the first step is below.
//
// The two halves run in this order: the build's own DDL, then the ledger, then
// the additive top-up (see migrate). A step that rebuilds a table has to come
// before anything tries to add columns to the shape it is replacing.
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

var migrations = []migration{
	{Version: 71, Name: "a role is its name", Up: rolesKeyedByName},
}

// rolesKeyedByName rebuilds the catalogue around the name (v71).
//
// Before: roles(id PK, name UNIQUE, parent_id -> roles.id) and
// group_roles(group_id, role_id -> roles.id). After: roles(name PK, parent ->
// roles.name) and group_roles(group_id, role -> roles.name), both cascading on
// update so a rename stays one statement.
//
// The classic rebuild, because neither SQLite nor a primary key change is
// something ALTER can do: move the old tables aside, create the new shape,
// copy through the id->name join, drop the old ones. The DDL is written out
// here rather than taken from schemaSQL on purpose - a step describes the
// schema AT ITS OWN version, and borrowing the current one would make this
// step produce whatever shape a later version invents.
//
// The order of the drops matters with foreign keys on (the DSN turns them on,
// and PRAGMA cannot be changed inside a transaction): group_roles_old goes
// first, so that nothing references roles_old when it goes.
func rolesKeyedByName(ctx context.Context, tx *transaction) error {
	steps := []string{
		`ALTER TABLE group_roles RENAME TO group_roles_old`,
		`ALTER TABLE roles RENAME TO roles_old`,
		`CREATE TABLE roles (
		   name        TEXT PRIMARY KEY,
		   description TEXT NOT NULL DEFAULT '',
		   parent      TEXT REFERENCES roles(name) ON DELETE SET NULL,
		   tags        TEXT NOT NULL DEFAULT '[]',
		   system      BOOLEAN NOT NULL DEFAULT FALSE,
		   created_at  BIGINT NOT NULL DEFAULT 0,
		   updated_at  BIGINT NOT NULL DEFAULT 0,
		   rev         BIGINT NOT NULL DEFAULT 0
		 )`,
		// The parent comes through a self-join on the OLD table: what was an id
		// is now the name that id pointed at. A parent whose row is gone lands
		// NULL, which is what ON DELETE SET NULL meant anyway.
		`INSERT INTO roles (name, description, parent, tags, system, created_at, updated_at, rev)
		 SELECT r.name, r.description, p.name, r.tags, r.system, r.created_at, r.updated_at, r.rev
		   FROM roles_old r LEFT JOIN roles_old p ON p.id = r.parent_id`,
		`CREATE TABLE group_roles (
		   group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
		   role     TEXT NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
		   PRIMARY KEY (group_id, role)
		 )`,
		// An INNER join: a link to a role that no longer exists granted nothing
		// and cannot be expressed any more.
		`INSERT INTO group_roles (group_id, role)
		 SELECT gr.group_id, r.name FROM group_roles_old gr JOIN roles_old r ON r.id = gr.role_id`,
		`DROP TABLE group_roles_old`,
		`DROP TABLE roles_old`,
	}
	for _, q := range steps {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("%w (running: %s)", err, firstLine(q))
		}
	}
	return nil
}

// firstLine names the statement that failed without printing a whole CREATE.
func firstLine(q string) string {
	q = strings.TrimSpace(q)
	if i := strings.IndexByte(q, '\n'); i > 0 {
		return q[:i] + " ..."
	}
	return q
}

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
