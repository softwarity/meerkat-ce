package store

import (
	"regexp"
	"strings"
	"testing"
)

// The embedded database is written the way the queries are, so nothing is
// translated for it. Paying a rewrite on every query of the default mode to
// serve the cluster one would be the wrong way round.
func TestTheEmbeddedDatabaseIsNotTranslated(t *testing.T) {
	q := `SELECT id FROM users WHERE username = ? AND enabled = ?`
	if got := rebind(dialectSQLite, q); got != q {
		t.Fatalf("the query was rewritten: %s", got)
	}
}

func TestPlaceholdersAreNumberedForPostgres(t *testing.T) {
	cases := []struct{ in, want string }{
		{`SELECT a FROM t WHERE b = ?`, `SELECT a FROM t WHERE b = $1`},
		{`UPDATE t SET a = ?, b = ? WHERE c = ?`, `UPDATE t SET a = $1, b = $2 WHERE c = $3`},
		{`SELECT a FROM t`, `SELECT a FROM t`},
		// A question mark inside a string literal is a character, not a
		// placeholder. There is none in this package today; there will be one
		// the day somebody stores a sentence, and it must not be renumbered
		// into a parameter that does not exist.
		{`SELECT a FROM t WHERE b = ? AND c != 'really?'`, `SELECT a FROM t WHERE b = $1 AND c != 'really?'`},
		{`INSERT INTO t (a) VALUES ('what?')`, `INSERT INTO t (a) VALUES ('what?')`},
	}
	for _, c := range cases {
		if got := rebind(dialectPostgres, c.in); got != c.want {
			t.Fatalf("rebind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Past nine, the numbering has to carry. The queries in this package go well
// past nine parameters, and getting $10 wrong would be silent: the driver
// would bind the wrong column, not fail.
func TestNumberingCarriesPastNine(t *testing.T) {
	got := rebind(dialectPostgres, strings.Repeat("?,", 12))
	want := ""
	for i := 1; i <= 12; i++ {
		want += "$" + itoa(i) + ","
	}
	if got != want {
		t.Fatalf("rebind = %q, want %q", got, want)
	}
}

// Every query in this package must be written with `?`. One written with $1
// would work today and break on the other database, silently - the wrapper
// would leave it alone and the driver would see a placeholder nobody bound.
func TestNoQueryIsWrittenForOneDatabase(t *testing.T) {
	dollar := regexp.MustCompile(`\$[0-9]`)
	for _, f := range goFilesOf(t, ".") {
		if strings.HasSuffix(f.name, "_test.go") {
			continue
		}
		for _, line := range strings.Split(f.body, "\n") {
			if !strings.Contains(line, "SELECT ") && !strings.Contains(line, "INSERT ") &&
				!strings.Contains(line, "UPDATE ") && !strings.Contains(line, "DELETE ") {
				continue
			}
			if dollar.MatchString(line) {
				t.Fatalf("%s: a query is written for PostgreSQL alone: %s", f.name, strings.TrimSpace(line))
			}
		}
	}
}
