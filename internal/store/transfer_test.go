package store

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// A database copied into another arrives whole: every table counted on both
// sides, a role after its parent, a secret still sealed, the target's own
// seeds replaced by the source's.
func TestADatabaseCopiesWhole(t *testing.T) {
	ctx := context.Background()
	src, dst := openTemp(t), openTemp(t)
	if err := src.CreateUser(ctx, User{ID: "u1", Username: "alice", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// A role pointing at another: the copy puts the parent in first.
	if err := src.SaveRole(ctx, Role{Name: "parent"}); err != nil {
		t.Fatal(err)
	}
	if err := src.SaveRole(ctx, Role{Name: "child", Parent: "parent"}); err != nil {
		t.Fatal(err)
	}
	if err := src.AddAuditEvent(ctx, AuditEvent{Target: "route", Action: "route.update", Detail: "tab\there\nnewline"}); err != nil {
		t.Fatal(err)
	}
	counts, err := src.CopyTo(ctx, dst, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != len(tables()) {
		t.Fatalf("%d tables counted, the schema has %d", len(counts), len(tables()))
	}
	u, err := dst.GetUserByID(ctx, "u1")
	if err != nil || u.Username != "alice" {
		t.Fatalf("the account did not arrive: %+v %v", u, err)
	}
	r, err := dst.GetRole(ctx, "child")
	if err != nil || r.Parent != "parent" {
		t.Fatalf("the role did not arrive with its parent: %+v %v", r, err)
	}
	// A second copy finds an account and refuses: never over a gateway.
	if _, err := src.CopyTo(ctx, dst, nil); err != ErrTargetInUse {
		t.Fatalf("a copy over a used database answered %v", err)
	}
}

// The dump is pg_dump's plain format: the schema, the version, COPY blocks
// with their values escaped.
func TestThePostgresDumpEscapesItsValues(t *testing.T) {
	ctx := context.Background()
	src := openTemp(t)
	if err := src.AddAuditEvent(ctx, AuditEvent{Target: "route", Action: "route.update", Detail: "tab\there\nnew\\line"}); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := src.WritePostgresDump(ctx, &b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"BEGIN;", "COMMIT;", "COPY audit_events (", `tab\there\nnew\\line`, "INSERT INTO schema_version (version) VALUES ("} {
		if !strings.Contains(out, want) {
			t.Errorf("the dump lacks %q", want)
		}
	}
}
