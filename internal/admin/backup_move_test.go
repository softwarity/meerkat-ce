package admin

import (
	"net/http"
	"strings"
	"testing"
)

// A migration waits for the pause; a backup of the same kind does not.
func TestAMigrationWaitsForThePause(t *testing.T) {
	f := setup(t)
	if code, _ := f.call(t, "GET", "/api/backup?format=postgres", "", f.rootC); code != http.StatusConflict {
		t.Fatalf("a dump to the other kind, unpaused, answered %d", code)
	}
	if code, _ := f.call(t, "GET", "/api/backup", "", f.rootC); code != http.StatusOK {
		t.Fatalf("a hot backup answered %d", code)
	}
	if code, _ := f.call(t, "POST", "/api/backup/copy", `{"host":"db","database":"m","user":"u"}`, f.rootC); code != http.StatusConflict && code != http.StatusForbidden {
		t.Fatalf("a copy, unpaused, answered %d", code)
	}
}

// The parts of a target become one URL, the password escaped by the URL
// package and never by hand.
func TestTheTargetIsAssembledFromItsParts(t *testing.T) {
	u, err := copyRequest{Host: "canopydb-primary", Database: "meerkat", User: "meerkat", Password: "p@ss/w:rd", SSLMode: "require"}.target()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "postgres://meerkat:p%40ss%2Fw%3Ard@canopydb-primary:5432/meerkat?sslmode=require") {
		t.Fatalf("assembled as %s", u)
	}
	if _, err := (copyRequest{Host: "h"}).target(); err == nil {
		t.Fatal("a target without database or user was accepted")
	}
}

// A certificate nobody here can verify is explained, not left as an x509 line.
func TestAnUnknownAuthorityIsExplained(t *testing.T) {
	msg := redactURL("tls: failed to verify certificate: x509: certificate signed by unknown authority", "postgres://u:secret@h/d")
	if !strings.Contains(msg, "sslmode require") {
		t.Fatalf("unexplained: %s", msg)
	}
}
