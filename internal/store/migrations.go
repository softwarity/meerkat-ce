package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/m3color"
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
	{Version: 73, Name: "certificates are a pool, placed on planes", Up: certificatesPlaced},
	{Version: 77, Name: "a theme is its source colours", Up: themesFromColours},
	{Version: 80, Name: "a route's custom code is an ordered list", Up: injectionsFromBlocks},
}

// injectionsFromBlocks turns the two free blocks of a UI route (customCss,
// customJs) into the ordered list that replaced them (v80), at the place they
// used to land, so no page changes. RouteUI's reader does the conversion -
// it has to for packages exported by v1.0 - and this step writes its result,
// so no row keeps a shape nothing else writes.
func injectionsFromBlocks(ctx context.Context, tx *transaction) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, ui FROM routes`)
	if err != nil {
		return err
	}
	type row struct{ id, ui string }
	var todo []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.ui); err != nil {
			_ = rows.Close()
			return err
		}
		if strings.Contains(r.ui, `"customCss"`) || strings.Contains(r.ui, `"customJs"`) {
			todo = append(todo, r)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, r := range todo {
		var ui RouteUI
		if err := json.Unmarshal([]byte(r.ui), &ui); err != nil {
			return fmt.Errorf("route %s: its ui block does not read: %w", r.id, err)
		}
		b, err := json.Marshal(ui)
		if err != nil {
			return fmt.Errorf("route %s: %w", r.id, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE routes SET ui = ?, rev = rev + 1 WHERE id = ?`, string(b), r.id); err != nil {
			return fmt.Errorf("route %s: %w", r.id, err)
		}
	}
	return nil
}

// themesFromColours turns every theme typed token by token into the six
// colours that make it (v77), and stores what those make.
//
// The typed form has no future: the editor makes themes from colours, the
// builder's JSON carries colours, and a second shape kept alive for old rows
// is a second shape every reader has to handle. So the old rows are converted
// rather than tolerated - ColorsFromTokens picks the colours, Generate writes
// the schemes, and the look moves as little as Material 3 allows: a primary
// comes back almost exactly, surfaces and outlines take the spec's tones.
//
// The columns may already be there: v76 added them without a step, and a
// database that ran v76 has them; one coming from v75 or earlier does not, and
// this step adds them itself - it cannot lean on the additive pass, which runs
// after it.
func themesFromColours(ctx context.Context, tx *transaction) error {
	for _, col := range []struct{ name, ddl string }{
		{"colors", `ALTER TABLE themes ADD COLUMN colors TEXT NOT NULL DEFAULT '{}'`},
		{"contrast", `ALTER TABLE themes ADD COLUMN contrast TEXT NOT NULL DEFAULT ''`},
		{"color_match", `ALTER TABLE themes ADD COLUMN color_match BOOLEAN NOT NULL DEFAULT FALSE`},
	} {
		has, err := tx.hasColumn(ctx, "themes", col.name)
		if err != nil {
			return err
		}
		if !has {
			if _, err := tx.ExecContext(ctx, col.ddl); err != nil {
				return fmt.Errorf("%s: %w", firstLine(col.ddl), err)
			}
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, name, colors, dark, light FROM themes`)
	if err != nil {
		return err
	}
	type typed struct{ id, name, dark, light string }
	var todo []typed
	for rows.Next() {
		var r typed
		var colors string
		if err := rows.Scan(&r.id, &r.name, &colors, &r.dark, &r.light); err != nil {
			_ = rows.Close()
			return err
		}
		var c m3color.Core
		_ = json.Unmarshal([]byte(colors), &c)
		if c.Primary == "" {
			todo = append(todo, r)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, r := range todo {
		t := Theme{Name: r.name}
		_ = json.Unmarshal([]byte(r.dark), &t.Dark)
		_ = json.Unmarshal([]byte(r.light), &t.Light)
		if err := t.Normalize(); err != nil {
			// A row with no readable primary: it starts again from the default's
			// colours rather than stopping every gateway that holds one.
			def := DefaultTheme()
			t.Colors, t.Contrast, t.ColorMatch = def.Colors, def.Contrast, def.ColorMatch
			if err := t.Generate(); err != nil {
				return fmt.Errorf("theme %q: %w", r.name, err)
			}
		}
		cj, _ := json.Marshal(t.Colors)
		dj, _ := json.Marshal(t.Dark)
		lj, _ := json.Marshal(t.Light)
		if _, err := tx.ExecContext(ctx,
			`UPDATE themes SET colors = ?, contrast = ?, color_match = ?, dark = ?, light = ?, rev = rev + 1 WHERE id = ?`,
			string(cj), t.Contrast, t.ColorMatch, string(dj), string(lj), r.id); err != nil {
			return fmt.Errorf("theme %q: %w", r.name, err)
		}
	}
	return nil
}

// certificatesPlaced turns the certificate of a NAME into material PLACED on
// a plane (v73).
//
// Before: certificates(plane, host) - one row per declared name, material two
// planes shared imported twice. After: certificates(on_console, on_app) and
// the names read from the material itself. Each row keeps the plane it was
// filed under; two rows holding the same certificate become one, placed on
// both - the oldest kept, so the fallback a plane presents does not change.
// The declared names leave with the columns: the TLS settings carry them
// until their next read (certs.Settings.Normalized), and the marker that
// seeded them has nothing left to mark.
func certificatesPlaced(ctx context.Context, tx *transaction) error {
	steps := []string{
		`ALTER TABLE certificates ADD COLUMN on_console BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE certificates ADD COLUMN on_app BOOLEAN NOT NULL DEFAULT FALSE`,
		`UPDATE certificates SET on_console = (plane = 'console'), on_app = (plane = 'app') WHERE csr_pem = '' OR cert_pem <> ''`,
	}
	for _, q := range steps {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("%s: %w", firstLine(q), err)
		}
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, cert_pem, plane FROM certificates WHERE cert_pem <> '' ORDER BY created_at, id`)
	if err != nil {
		return err
	}
	first := map[string]string{}
	var merges [][3]string // keep, drop, plane
	for rows.Next() {
		var id, pem, plane string
		if err := rows.Scan(&id, &pem, &plane); err != nil {
			_ = rows.Close()
			return err
		}
		if keep, ok := first[pem]; ok {
			merges = append(merges, [3]string{keep, id, plane})
			continue
		}
		first[pem] = id
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, m := range merges {
		col := "on_app"
		if m[2] == "console" {
			col = "on_console"
		}
		if _, err := tx.ExecContext(ctx, `UPDATE certificates SET `+col+` = TRUE WHERE id = ?`, m[0]); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM certificates WHERE id = ?`, m[1]); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`ALTER TABLE certificates DROP COLUMN host`,
		`ALTER TABLE certificates DROP COLUMN plane`,
		`DELETE FROM settings WHERE key = 'tls_seeded'`,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("%s: %w", firstLine(q), err)
		}
	}
	return nil
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
