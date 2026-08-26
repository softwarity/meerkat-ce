package store

import (
	"context"
	"os"
	"testing"
)

// The same suite, against a real PostgreSQL. Skipped unless one is named, so
// `go test ./...` on a laptop stays what it was.
//
// It is deliberately not a mock: what this proves is that the SCHEMA applies
// and that the values come back as themselves - and a fake that speaks our own
// dialect proves neither.
func TestAgainstARealPostgres(t *testing.T) {
	url := os.Getenv("MEERKAT_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set MEERKAT_TEST_DATABASE_URL to run against a real PostgreSQL")
	}
	st, err := OpenAt(t.TempDir(), url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()

	// A boolean and a timestamp on the same row: the pair that a mistaken
	// column type would separate, and the reason the classification was done
	// from the Go rather than from the DDL.
	if err := st.CreateUser(ctx, User{ID: "u1", Username: "alice", PasswordHash: "h", Enabled: true, Dev: true}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	u, err := st.GetUserByID(ctx, "u1")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !u.Enabled || !u.Dev || u.Root {
		t.Fatalf("booleans came back wrong: %+v", u)
	}
	if u.CreatedAt == 0 {
		t.Fatal("the timestamp came back empty")
	}

	// A settings round trip: JSON in a TEXT column, the shape most of the
	// product's configuration travels in.
	if err := st.SetSetting(ctx, "probe", map[string]string{"a": "b"}); err != nil {
		t.Fatalf("set setting: %v", err)
	}
	var back map[string]string
	if err := st.GetSetting(ctx, "probe", &back); err != nil || back["a"] != "b" {
		t.Fatalf("setting round trip: %v %v", back, err)
	}

	// The upsert every setting write uses (ON CONFLICT DO UPDATE), which both
	// databases spell the same way - proven rather than assumed.
	if err := st.SetSetting(ctx, "probe", map[string]string{"a": "c"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := st.GetSetting(ctx, "probe", &back); err != nil || back["a"] != "c" {
		t.Fatalf("the upsert did not update: %v %v", back, err)
	}

	// And the version this build migrated it to.
	v, err := st.db.schemaVersion()
	if err != nil || v != schemaVersion {
		t.Fatalf("schema version = %d (%v), want %d", v, err, schemaVersion)
	}
}
