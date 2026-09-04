// Package dbtest names the database ONE test opens.
//
// It exists so the suite can be run twice - once on the embedded database,
// once on PostgreSQL - without a second suite being written. Every test that
// used to hardcode the embedded one now asks this package instead, and the
// answer comes from the environment:
//
//	go test ./...                                    the embedded database
//	MEERKAT_TEST_DATABASE_URL=postgres://... go test  a real PostgreSQL
//
// Empty by default is the point: a laptop with nothing installed keeps
// running the whole suite exactly as before. What the second pass buys is the
// only thing a mock could never say - that the queries in internal/store,
// written once with `?` placeholders and rebound on the way out, actually run
// on the other dialect. A hundred and sixty-nine of them were being taken on
// faith, with one smoke test as the entire evidence (QUAL-01).
//
// This package deliberately does NOT import internal/store: the store's own
// tests live in that package, and a helper they cannot use would only be half
// a helper.
package dbtest

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// Env is the variable that turns the PostgreSQL pass on. Named here so a test
// that wants to say "skipped unless" says it the same way.
const Env = "MEERKAT_TEST_DATABASE_URL"

// URL returns what to pass as store.OpenAt's second argument.
//
// On PostgreSQL each test gets a schema of its own, created here and dropped
// when the test ends. Schemas rather than databases because CREATE DATABASE
// cannot run inside a transaction, takes a template lock that serialises the
// suite, and costs about fifty times a schema - and what the tests need is
// isolation, not a separate server. Every table this product creates is
// unqualified, so it lands in the search path and nothing else had to change.
func URL(t testing.TB) string {
	t.Helper()
	base := strings.TrimSpace(os.Getenv(Env))
	if base == "" {
		return ""
	}
	schema := schemaName()
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("dbtest: open %s: %v", Env, err)
	}
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		_ = admin.Close()
		t.Fatalf("dbtest: create schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		// CASCADE: the schema is the unit of cleanup, and listing the tables
		// here would be a second copy of the schema to keep in step.
		if _, err := admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Logf("dbtest: dropping %s: %v", schema, err)
		}
		_ = admin.Close()
	})
	return withSearchPath(t, base, schema)
}

// schemaName is random rather than derived from t.Name(): packages test in
// parallel processes, so a name has to be unique across the whole run, and a
// leftover from a killed run must never be adopted by the next one.
func schemaName() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "mk_" + hex.EncodeToString(b[:])
}

// withSearchPath points the connection at the schema. Both DSN spellings are
// accepted because both are valid to pgx and people paste whichever their
// hosting gave them.
func withSearchPath(t testing.TB, base, schema string) string {
	if !strings.Contains(base, "://") {
		return base + " search_path=" + schema
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("dbtest: %s is not a URL: %v", Env, err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}
