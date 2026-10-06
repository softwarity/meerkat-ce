package store

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Moving a database whole (Configuration > Snapshot): into another database
// of either kind, or into a file of either kind.
//
// GENERIC, and that is what keeps it true: the tables and their columns are
// read from the schema this build creates (schemaSQL), not listed here, so a
// table added next month travels without anybody remembering this file. The
// two databases speak the same schema with the same three column types, so a
// row moves as it is, its values only normalised to what the target's driver
// expects - a SQLite boolean is an integer, PostgreSQL's is a boolean.
//
// What does NOT travel, by design: the schema version, which the target sets
// itself when it is created, and the vault key, which never lives in the
// database - the secrets arrive sealed, and open only where the same key is.

// ErrTargetInUse refuses a target that already serves somebody.
var ErrTargetInUse = errors.New("the target database already holds accounts or routes: copy into an empty database, never over a gateway")

// schemaTable is one table as the schema declares it.
type schemaTable struct {
	Name  string
	Cols  []string
	Types []string // TEXT, BIGINT, BOOLEAN, REAL, one per column
	// Parent names the column that points at another row of the same table
	// (a role's parent): those rows go in parents first.
	Parent, ParentKey string
}

var (
	schemaOnce   sync.Once
	schemaTables []schemaTable
	tableStart   = regexp.MustCompile(`^CREATE TABLE IF NOT EXISTS (\w+)`)
	columnLine   = regexp.MustCompile(`^\s+([a-z_]+)\s+(TEXT|BIGINT|BOOLEAN|REAL|INTEGER)\b(.*)$`)
	selfRef      = regexp.MustCompile(`REFERENCES (\w+)\((\w+)\)`)
)

// tables reads the schema once, in the order it creates the tables - which is
// the order a foreign key needs: a table is created after the ones it points
// at.
func tables() []schemaTable {
	schemaOnce.Do(func() {
		var cur *schemaTable
		for _, line := range strings.Split(schemaSQL, "\n") {
			if m := tableStart.FindStringSubmatch(line); m != nil {
				schemaTables = append(schemaTables, schemaTable{Name: m[1]})
				cur = &schemaTables[len(schemaTables)-1]
				continue
			}
			if cur == nil {
				continue
			}
			if strings.HasPrefix(line, ");") {
				cur = nil
				continue
			}
			m := columnLine.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			typ := m[2]
			if typ == "INTEGER" {
				typ = "BIGINT"
			}
			cur.Cols = append(cur.Cols, m[1])
			cur.Types = append(cur.Types, typ)
			if r := selfRef.FindStringSubmatch(m[3]); r != nil && r[1] == cur.Name {
				cur.Parent, cur.ParentKey = m[1], r[2]
			}
		}
	})
	return schemaTables
}

// Dialect names the kind of database: "sqlite" or "postgres".
func (s *Store) Dialect() string { return s.db.dialect }

// TableCount is one table's rows, on both sides of a copy.
type TableCount struct {
	Table  string `json:"table"`
	Source int64  `json:"source"`
	Target int64  `json:"target"`
}

// CopyTo copies every row of this database into dst, in one transaction on
// dst, and checks the counts before committing. dst must hold no account and
// no route; the rows a freshly created database seeds itself with (the default
// organisation, the settings) are replaced by this one's.
//
// progress is told each table as it is done, for a screen to follow.
func (s *Store) CopyTo(ctx context.Context, dst *Store, progress func(TableCount)) ([]TableCount, error) {
	for _, t := range []string{"users", "routes"} {
		var n int64
		if err := dst.db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+t).Scan(&n); err != nil {
			return nil, fmt.Errorf("store: copy: read the target's %s: %w", t, err)
		}
		if n > 0 {
			return nil, ErrTargetInUse
		}
	}
	// SQLite checks a foreign key at each row and cannot defer it; the order
	// below satisfies them, but a single connection is all it takes to say so
	// once rather than trust it.
	if dst.db.dialect == dialectSQLite {
		if _, err := dst.db.DB.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return nil, fmt.Errorf("store: copy: %w", err)
		}
		defer func() { _, _ = dst.db.DB.ExecContext(context.Background(), "PRAGMA foreign_keys = ON") }()
	}
	tx, err := dst.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store: copy: begin on the target: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	ts := tables()
	for i := len(ts) - 1; i >= 0; i-- {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+ts[i].Name); err != nil {
			return nil, fmt.Errorf("store: copy: clear the target's %s: %w", ts[i].Name, err)
		}
	}
	var counts []TableCount
	for _, t := range ts {
		n, err := s.copyTable(ctx, tx, dst.db.dialect, t)
		if err != nil {
			return nil, err
		}
		var got int64
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+t.Name).Scan(&got); err != nil {
			return nil, fmt.Errorf("store: copy: count the target's %s: %w", t.Name, err)
		}
		c := TableCount{Table: t.Name, Source: n, Target: got}
		if n != got {
			return nil, fmt.Errorf("store: copy: %s holds %d rows on the target and %d on the source", t.Name, got, n)
		}
		counts = append(counts, c)
		if progress != nil {
			progress(c)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: copy: commit on the target: %w", err)
	}
	return counts, nil
}

const copyBatch = 200

func (s *Store) copyTable(ctx context.Context, tx *sql.Tx, dialect string, t schemaTable) (int64, error) {
	rows, err := s.readTable(ctx, t)
	if err != nil {
		return 0, err
	}
	insert := func(batch [][]any) error {
		if len(batch) == 0 {
			return nil
		}
		ph := "(" + strings.TrimSuffix(strings.Repeat("?, ", len(t.Cols)), ", ") + ")"
		vals := make([]string, len(batch))
		args := make([]any, 0, len(batch)*len(t.Cols))
		for i, r := range batch {
			vals[i] = ph
			args = append(args, r...)
		}
		q := "INSERT INTO " + t.Name + " (" + strings.Join(t.Cols, ", ") + ") VALUES " + strings.Join(vals, ", ")
		if _, err := tx.ExecContext(ctx, rebind(dialect, q), args...); err != nil {
			return fmt.Errorf("store: copy %s: %w", t.Name, err)
		}
		return nil
	}
	var n int64
	batch := make([][]any, 0, copyBatch)
	for r := range rows.each {
		batch = append(batch, r)
		n++
		if len(batch) == copyBatch {
			if err := insert(batch); err != nil {
				return 0, err
			}
			batch = batch[:0]
		}
	}
	if err := rows.err(); err != nil {
		return 0, err
	}
	return n, insert(batch)
}

// tableRows walks one table's rows, normalised, in an order that inserts.
type tableRows struct {
	each func(yield func([]any) bool)
	err  func() error
}

// readTable reads t whole. A table that points at itself is read into memory
// and put in parents-first order; every other one is streamed.
func (s *Store) readTable(ctx context.Context, t schemaTable) (tableRows, error) {
	q := "SELECT " + strings.Join(t.Cols, ", ") + " FROM " + t.Name
	rs, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return tableRows{}, fmt.Errorf("store: read %s: %w", t.Name, err)
	}
	var scanErr error
	scan := func() ([]any, bool) {
		raw := make([]any, len(t.Cols))
		ptrs := make([]any, len(t.Cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			scanErr = fmt.Errorf("store: read %s: %w", t.Name, err)
			return nil, false
		}
		for i, v := range raw {
			raw[i] = normalise(t.Types[i], v)
		}
		return raw, true
	}
	if t.Parent == "" {
		return tableRows{
			each: func(yield func([]any) bool) {
				defer func() { _ = rs.Close() }()
				for rs.Next() {
					r, ok := scan()
					if !ok || !yield(r) {
						return
					}
				}
			},
			err: func() error {
				if scanErr != nil {
					return scanErr
				}
				return rs.Err()
			},
		}, nil
	}
	var all [][]any
	for rs.Next() {
		r, ok := scan()
		if !ok {
			_ = rs.Close()
			return tableRows{}, scanErr
		}
		all = append(all, r)
	}
	_ = rs.Close()
	if err := rs.Err(); err != nil {
		return tableRows{}, err
	}
	ordered := parentsFirst(t, all)
	return tableRows{
		each: func(yield func([]any) bool) {
			for _, r := range ordered {
				if !yield(r) {
					return
				}
			}
		},
		err: func() error { return nil },
	}, nil
}

// parentsFirst orders rows so a row comes after the row it points at. A row
// whose parent is missing goes in as it is - the database will say what it
// thinks of it, which is better than this guessing.
func parentsFirst(t schemaTable, rows [][]any) [][]any {
	pi, ki := -1, -1
	for i, c := range t.Cols {
		if c == t.Parent {
			pi = i
		}
		if c == t.ParentKey {
			ki = i
		}
	}
	if pi < 0 || ki < 0 {
		return rows
	}
	key := func(v any) string { return fmt.Sprint(v) }
	placed := map[string]bool{}
	out := make([][]any, 0, len(rows))
	left := rows
	for len(left) > 0 {
		var next [][]any
		for _, r := range left {
			if r[pi] == nil || r[pi] == "" || placed[key(r[pi])] {
				out = append(out, r)
				placed[key(r[ki])] = true
			} else {
				next = append(next, r)
			}
		}
		if len(next) == len(left) {
			return append(out, next...)
		}
		left = next
	}
	return out
}

// normalise turns what a driver handed back into what the target expects for
// the column's declared type.
func normalise(typ string, v any) any {
	if b, ok := v.([]byte); ok {
		v = string(b)
	}
	if v == nil {
		return nil
	}
	switch typ {
	case "BOOLEAN":
		switch x := v.(type) {
		case bool:
			return x
		case int64:
			return x != 0
		case string:
			return x == "1" || x == "t" || x == "true"
		}
	case "BIGINT":
		switch x := v.(type) {
		case int64:
			return x
		case bool:
			if x {
				return int64(1)
			}
			return int64(0)
		case float64:
			return int64(x)
		case string:
			n, _ := strconv.ParseInt(x, 10, 64)
			return n
		}
	case "REAL":
		switch x := v.(type) {
		case float64:
			return x
		case int64:
			return float64(x)
		}
	case "TEXT":
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
	return v
}

// WritePostgresDump writes this database as a PostgreSQL dump in the plain
// format pg_dump writes by default: the schema, the version, then each table's
// rows as COPY blocks, in one transaction. Loaded with psql -f into an empty
// database, it is this installation on PostgreSQL.
func (s *Store) WritePostgresDump(ctx context.Context, w io.Writer) error {
	bw := bufio.NewWriterSize(w, 64<<10)
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(bw, format, a...) }
	p("-- Meerkat database, schema version %d, in PostgreSQL's plain dump format.\n", schemaVersion)
	p("-- Load it into an EMPTY database: psql -v ON_ERROR_STOP=1 -f this-file.sql \"$MEERKAT_DATABASE_URL\"\n")
	p("-- The vault key is not in it: carry MEERKAT_VAULT_KEY, or the key file, to the new deployment.\n\n")
	p("BEGIN;\n\n%s;\n\n", strings.TrimSpace(schemaSQL))
	p("CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);\nDELETE FROM schema_version;\nINSERT INTO schema_version (version) VALUES (%d);\n\n", schemaVersion)
	ts := tables()
	// What a fresh schema seeds itself with is replaced, in the same order a
	// copy clears it: children first.
	for i := len(ts) - 1; i >= 0; i-- {
		p("DELETE FROM %s;\n", ts[i].Name)
	}
	p("\n")
	for _, t := range ts {
		rows, err := s.readTable(ctx, t)
		if err != nil {
			return err
		}
		p("COPY %s (%s) FROM stdin;\n", t.Name, strings.Join(t.Cols, ", "))
		for r := range rows.each {
			for i, v := range r {
				if i > 0 {
					p("\t")
				}
				_, _ = bw.WriteString(copyText(v))
			}
			p("\n")
		}
		if err := rows.err(); err != nil {
			return err
		}
		p("\\.\n\n")
	}
	p("COMMIT;\n")
	return bw.Flush()
}

// copyText is one value in COPY's text format: \N for NULL, t/f for a
// boolean, and the four characters that would break a line escaped.
func copyText(v any) string {
	switch x := v.(type) {
	case nil:
		return `\N`
	case bool:
		if x {
			return "t"
		}
		return "f"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		r := strings.NewReplacer(`\`, `\\`, "\t", `\t`, "\n", `\n`, "\r", `\r`)
		return r.Replace(x)
	default:
		return fmt.Sprint(x)
	}
}
