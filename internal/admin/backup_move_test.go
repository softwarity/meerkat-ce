package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A migration waits for the pause; a backup of the same kind does not.
func TestAMigrationWaitsForThePause(t *testing.T) {
	f := setup(t)
	// The other kind is whichever this suite does not run on: the same test
	// runs on the embedded database and on PostgreSQL.
	other := "postgres"
	if f.api.st.Dialect() == "postgres" {
		other = "sqlite"
	}
	if code, _ := f.call(t, "GET", "/api/backup?format="+other, "", f.rootC); code != http.StatusConflict {
		t.Fatalf("a copy to the other kind (%s), unpaused, answered %d", other, code)
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

// The tab reads what the gateway runs on, and the copy a migration is in the
// middle of - never its password - and an operator may dismiss it.
func TestTheBackupInfoSaysThePlatformAndTheLastCopy(t *testing.T) {
	f := setup(t)
	f.api.LookupPlatform = func(context.Context) discovery.Platform {
		return discovery.Platform{Name: "kubernetes", Helm: true, Release: "edge", Namespace: "shop", Workload: "edge-meerkat"}
	}
	c, err := store.LastCopyOf("postgres://meerkat:hunter2@canopydb-primary:5432/gw?sslmode=require")
	if err != nil {
		t.Fatal(err)
	}
	c.Tables, c.Rows = 3, 42
	if err := f.api.st.RememberCopy(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	var info struct {
		Platform discovery.Platform `json:"platform"`
		LastCopy *store.LastCopy    `json:"lastCopy"`
	}
	code, body := f.call(t, "GET", "/api/backup/info", "", f.rootC)
	if code != http.StatusOK || json.Unmarshal([]byte(body), &info) != nil {
		t.Fatalf("info answered %d: %s", code, body)
	}
	if info.Platform.Release != "edge" || info.LastCopy == nil || info.LastCopy.Host != "canopydb-primary" || info.LastCopy.Rows != 42 {
		t.Fatalf("info read as %s", body)
	}
	if strings.Contains(body, "hunter2") {
		t.Fatalf("the info carries the password: %s", body)
	}
	if code, _ := f.call(t, "DELETE", "/api/backup/last-copy", "", f.plainC); code != http.StatusForbidden && code != http.StatusUnauthorized {
		t.Fatalf("a plain account dismissed the copy: %d", code)
	}
	if code, _ := f.call(t, "DELETE", "/api/backup/last-copy", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("dismissing answered %d", code)
	}
	if _, body := f.call(t, "GET", "/api/backup/info", "", f.rootC); strings.Contains(body, "lastCopy") {
		t.Fatalf("still remembered after the dismissal: %s", body)
	}
}

// A real copy into a real PostgreSQL is remembered by the database it left,
// paused as it is.
func TestACopyIsRememberedByTheDatabaseItLeft(t *testing.T) {
	target := dbtest.URL(t)
	if target == "" {
		t.Skip("needs " + dbtest.Env)
	}
	f := setup(t)
	f.api.st.Pause(true)
	t.Cleanup(func() { f.api.st.Pause(false) })
	body, _ := json.Marshal(map[string]string{"url": target})
	if code, out := f.call(t, "POST", "/api/backup/copy", string(body), f.rootC); code != http.StatusOK {
		t.Fatalf("the copy answered %d: %s", code, out)
	}
	c, err := f.api.st.LastCopy(context.Background())
	if err != nil || c == nil || c.Rows == 0 || c.Tables == 0 || c.By != "root" {
		t.Fatalf("remembered as %+v (%v)", c, err)
	}
}
