//go:build ee

package store

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// Across the two kinds, both ways: an embedded database copied onto
// PostgreSQL, and back onto a fresh embedded one, arrives whole each time -
// booleans, integers, a role after its parent.
func TestACopyCrossesBetweenTheTwoKinds(t *testing.T) {
	url := dbtest.URL(t)
	if url == "" {
		t.Skip("needs " + dbtest.Env)
	}
	ctx := context.Background()
	open := func(u string) *Store {
		s, err := OpenAt(t.TempDir(), u)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s
	}
	lite, pg, back := open(""), open(url), open("")
	if err := lite.CreateUser(ctx, User{ID: "u1", Username: "alice", PasswordHash: "x", Enabled: true, Root: true}); err != nil {
		t.Fatal(err)
	}
	if err := lite.SaveRole(ctx, Role{Name: "parent"}); err != nil {
		t.Fatal(err)
	}
	if err := lite.SaveRole(ctx, Role{Name: "child", Parent: "parent"}); err != nil {
		t.Fatal(err)
	}
	if _, err := lite.CopyTo(ctx, pg, nil); err != nil {
		t.Fatalf("embedded to PostgreSQL: %v", err)
	}
	if _, err := pg.CopyTo(ctx, back, nil); err != nil {
		t.Fatalf("PostgreSQL to embedded: %v", err)
	}
	for _, s := range []*Store{pg, back} {
		u, err := s.GetUserByID(ctx, "u1")
		if err != nil || !u.Root || !u.Enabled {
			t.Fatalf("on %s the account arrived as %+v (%v)", s.Dialect(), u, err)
		}
		if r, err := s.GetRole(ctx, "child"); err != nil || r.Parent != "parent" {
			t.Fatalf("on %s the role arrived as %+v (%v)", s.Dialect(), r, err)
		}
	}
}
