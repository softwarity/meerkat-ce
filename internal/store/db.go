package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/softwarity/meerkat/internal/vault"
)

// A thin wrapper over database/sql that rebinds the placeholders on the way
// through.
//
// It exists so the queries in this package stay written ONE way. Without it,
// supporting a second database meant editing every call site - and a hundred
// and sixty-nine mechanical edits is a hundred and sixty-nine chances to get
// one wrong, on a change no test would notice until that query ran.
//
// Nothing else is added: no query builder, no logging, no retry. It is a
// translation and a pass-through, and it should stay that.

type database struct {
	*sql.DB
	dialect string
}

func (d *database) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.DB.ExecContext(ctx, rebind(d.dialect, query), args...)
}

func (d *database) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.DB.QueryContext(ctx, rebind(d.dialect, query), args...)
}

func (d *database) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.DB.QueryRowContext(ctx, rebind(d.dialect, query), args...)
}

func (d *database) Exec(query string, args ...any) (sql.Result, error) {
	return d.DB.Exec(rebind(d.dialect, query), args...)
}

func (d *database) Query(query string, args ...any) (*sql.Rows, error) {
	return d.DB.Query(rebind(d.dialect, query), args...)
}

func (d *database) QueryRow(query string, args ...any) *sql.Row {
	return d.DB.QueryRow(rebind(d.dialect, query), args...)
}

func (d *database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*transaction, error) {
	t, err := d.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &transaction{Tx: t, dialect: d.dialect}, nil
}

// transaction is the same translation, inside a transaction.
type transaction struct {
	*sql.Tx
	dialect string
}

func (t *transaction) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, rebind(t.dialect, query), args...)
}

func (t *transaction) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, rebind(t.dialect, query), args...)
}

func (t *transaction) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx.QueryRowContext(ctx, rebind(t.dialect, query), args...)
}

// Reading a database ABOUT itself: the schema version it is at, and the
// columns it holds. Every database answers these, and every one answers them
// differently - SQLite through pragmas, PostgreSQL through information_schema.
//
// This is the whole reason a dialect exists as a concept here. The queries
// that carry the product are portable already (three column types, no vendor
// functions, placeholders aside); it is the questions ABOUT the schema that
// are not.

// schemaVersion reports the version this database was last migrated to. Zero
// on a database that has never been migrated.
func (d *database) schemaVersion() (int, error) {
	var v int
	switch d.dialect {
	case dialectPostgres:
		// A table rather than a pragma, created on the way in: PostgreSQL has
		// nowhere to hang a number of ours otherwise.
		if _, err := d.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
			return 0, fmt.Errorf("store: read schema version: %w", err)
		}
		err := d.DB.QueryRow(`SELECT version FROM schema_version`).Scan(&v)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		if err != nil {
			return 0, fmt.Errorf("store: read schema version: %w", err)
		}
	default:
		if err := d.DB.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
			return 0, fmt.Errorf("store: read schema version: %w", err)
		}
	}
	return v, nil
}

// setSchemaVersion records the version reached.
func (d *database) setSchemaVersion(v int) error {
	switch d.dialect {
	case dialectPostgres:
		if _, err := d.DB.Exec(`DELETE FROM schema_version`); err != nil {
			return fmt.Errorf("store: set schema version: %w", err)
		}
		// Through the wrapper, with the placeholder every query in this
		// package is written with - the rule holds inside the dialect too.
		if _, err := d.Exec(`INSERT INTO schema_version (version) VALUES (?)`, v); err != nil {
			return fmt.Errorf("store: set schema version: %w", err)
		}
	default:
		// Interpolated, and it has to be: a pragma takes no parameters. The
		// value is a constant of this build, never anything a caller supplies.
		if _, err := d.DB.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v)); err != nil {
			return fmt.Errorf("store: set schema version: %w", err)
		}
	}
	return nil
}

// tableNames lists the tables this database holds.
func (d *database) tableNames() ([]string, error) {
	query := `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
	if d.dialect == dialectPostgres {
		query = `SELECT table_name FROM information_schema.tables
		         WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'`
	}
	rows, err := d.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("store: list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// columnsOf describes one table's columns, in the shape ALTER TABLE ADD COLUMN
// needs to rebuild a missing one.
func (d *database) columnsOf(table string) ([]column, error) {
	query := `SELECT name, type, "notnull", dflt_value FROM pragma_table_info(?)`
	if d.dialect == dialectPostgres {
		query = `SELECT column_name, data_type, is_nullable = 'NO', column_default
		         FROM information_schema.columns
		         WHERE table_schema = current_schema() AND table_name = ?
		         ORDER BY ordinal_position`
	}
	rows, err := d.Query(query, table)
	if err != nil {
		return nil, fmt.Errorf("store: columns of %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	var out []column
	for rows.Next() {
		var c column
		if err := rows.Scan(&c.name, &c.dataType, &c.notNull, &c.dflt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// openPostgres opens the external database and applies the same schema.
//
// The same schema, literally: every column type in it is spelled the same on
// both sides, which is what the boolean pass bought. What differs is answered
// by the dialect - placeholders, the schema version, introspection - and
// nothing else had to be written twice.
func openPostgres(dataDir, url string) (*Store, error) {
	raw, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("store: open database: %w", err)
	}
	if err := raw.Ping(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("store: the database named by MEERKAT_DATABASE_URL does not answer: %w", err)
	}
	s := &Store{db: &database{DB: raw, dialect: dialectPostgres}, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		_ = raw.Close()
		return nil, err
	}
	// The vault key stays a FILE beside the gateway even here, and that is
	// deliberate: it seals what is in the database, so keeping it in the
	// database would seal the door with the key in the lock.
	key, err := vault.LoadOrCreateKey(dataDir, os.Getenv("MEERKAT_VAULT_KEY"))
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	if s.vaultCipher, err = vault.NewCipher(key); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return s, nil
}
