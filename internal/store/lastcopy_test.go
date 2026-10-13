package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The record is read from the URL the copy used, and the password stops there.
func TestTheLastCopyKeepsNoPassword(t *testing.T) {
	c, err := LastCopyOf("postgres://meerkat:s3cr%40t@canopydb-primary:6432/gw?sslmode=require")
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "canopydb-primary" || c.Port != 6432 || c.Database != "gw" || c.User != "meerkat" || c.SSLMode != "require" {
		t.Fatalf("read as %+v", c)
	}
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	// Remembered while paused: the copy is taken paused, and the pause is
	// not about this row.
	s.Pause(true)
	c.Tables, c.Rows = 40, 1234
	if err := s.RememberCopy(ctx, c); err != nil {
		t.Fatalf("remembered while paused: %v", err)
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, lastCopyKey).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "s3cr") || strings.Contains(raw, `"password"`) {
		t.Fatalf("the stored record carries a password: %s", raw)
	}
	got, err := s.LastCopy(ctx)
	if err != nil || got == nil || got.Rows != 1234 || got.From != s.Location() || got.At == 0 {
		t.Fatalf("read back as %+v (%v)", got, err)
	}
	// Dismissed, it is gone - paused or not.
	if err := s.ForgetCopy(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.LastCopy(ctx); got != nil {
		t.Fatalf("still remembered after being forgotten: %+v", got)
	}
}

// A record that travelled into another database, in a copy or a snapshot, is
// that other database's story: not shown, and cleared.
func TestACopyRememberedElsewhereIsNotThisOne(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	if err := s.SetSetting(ctx, lastCopyKey, LastCopy{Host: "db", Port: 5432, Database: "gw", From: "sqlite:/elsewhere/meerkat.db"}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.LastCopy(ctx); err != nil || got != nil {
		t.Fatalf("another database's copy was shown: %+v (%v)", got, err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM settings WHERE key = ?`, lastCopyKey).Scan(&n); err != nil || n != 0 {
		t.Fatalf("the stale record was kept (%d, %v)", n, err)
	}
}

// Running on the database it was copied into, the switch is done: the record
// no longer stands, whatever it says it came from.
func TestTheCopyEndsOnceTheGatewayRunsOnIt(t *testing.T) {
	c := LastCopy{Host: "CanopyDB-primary", Port: 5432, Database: "gw", From: "sqlite:/data/meerkat.db"}
	here := postgresLocation("postgres://meerkat:x@canopydb-primary/gw?sslmode=require")
	if c.standing(dialectPostgres, here) {
		t.Fatal("still standing on the database it was copied into")
	}
	if !c.standing(dialectSQLite, "sqlite:/data/meerkat.db") {
		t.Fatal("not standing on the database it was copied from")
	}
	if l := postgresLocation("host=canopydb-primary port=5432 dbname=gw user=meerkat"); l != here {
		t.Fatalf("the key=value form reads as %q, the URL as %q", l, here)
	}
}
