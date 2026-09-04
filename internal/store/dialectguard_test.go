package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Two rules that only PostgreSQL enforces, checked here so the embedded
// database is not the only judge.
//
// Both come from a real failure. The suite is now run twice - see
// internal/store/dbtest - but the second pass needs a PostgreSQL to be there,
// and a laptop has none: these two catch the mistake at `go test ./...`,
// where it was made, instead of in CI half an hour later.

// TestTheSchemaNeverSaysINTEGER guards the width of a whole number.
//
// On the embedded database INTEGER is 64 bits; on PostgreSQL it is 32, and
// every whole number in this schema is either a Unix second or a counter that
// has no reason to stop at two billion. A session valid until 2100 overflowed
// it on the first try, and the timestamps that did not would have waited for
// 2038 to explain themselves.
func TestTheSchemaNeverSaysINTEGER(t *testing.T) {
	for i, line := range strings.Split(schemaSQL, "\n") {
		if strings.Contains(line, "INTEGER") {
			t.Errorf("schemaSQL line %d declares INTEGER, which is 32 bits on PostgreSQL - write BIGINT:\n  %s",
				i+1, strings.TrimSpace(line))
		}
	}
}

// sqliteOnly maps a spelling this package must not use to the portable one.
// The value is printed as the fix, so it has to BE the fix.
var sqliteOnly = map[string]string{
	"INSERT OR IGNORE":  "INSERT ... ON CONFLICT DO NOTHING, which both databases accept",
	"INSERT OR REPLACE": "INSERT ... ON CONFLICT DO UPDATE, which both databases accept",
	"AUTOINCREMENT":     "nothing: this schema mints its own identifiers",
	"IFNULL(":           "COALESCE(",
	"GROUP_CONCAT(":     "reading the rows and joining them in Go",
	"STRFTIME(":         "a Unix second computed in Go, like every other date in here",
	"JULIANDAY(":        "a Unix second computed in Go",
	"ROWID":             "a column of this table's own - PostgreSQL has no implicit one",
}

// TestNoQuerySpeaksOnlySQLite reads the SQL this package actually sends.
//
// String literals only, through the parser rather than a grep: half of these
// words appear in the comments EXPLAINING why they are not used, and a guard
// that trips on its own rationale gets deleted the first time it is in the
// way.
func TestNoQuerySpeaksOnlySQLite(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		// db.go is the dialect boundary itself: the one file allowed to spell
		// out what is true of one database only, because branching on it is
		// its whole job.
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "db.go" {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			upper := strings.ToUpper(lit.Value)
			for bad, instead := range sqliteOnly {
				if strings.Contains(upper, bad) {
					t.Errorf("%s:%d speaks SQLite only (%s) - use %s",
						name, fset.Position(lit.Pos()).Line, bad, instead)
				}
			}
			return true
		})
	}
}
