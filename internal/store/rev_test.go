package store

import (
	"context"
	"errors"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestEveryEditedObjectCarriesARevision: the mechanism is only worth anything
// if it is on everything a person edits - one object without it is the one that
// loses somebody's work. Each kind is taken through the same three steps: read
// it, write it twice from the same copy, and the second write is refused.
func TestEveryEditedObjectCarriesARevision(t *testing.T) {
	s, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// Each kind: how to create one, how to read it back, and how to write it.
	kinds := []struct {
		name   string
		create func() error
		read   func() (int64, error)
		write  func(rev int64) error
	}{
		{
			name:   "role",
			create: func() error { return s.SaveRole(ctx, Role{ID: "r", Name: "ops"}) },
			read:   func() (int64, error) { r, err := s.GetRole(ctx, "r"); return r.Rev, err },
			write: func(rev int64) error {
				return s.SaveRole(ctx, Role{ID: "r", Name: "ops", Description: "changed", Rev: rev})
			},
		},
		{
			name:   "tenant",
			create: func() error { return s.SaveTenant(ctx, Tenant{ID: "t", Name: "Acme", Enabled: true}) },
			read:   func() (int64, error) { t, err := s.GetTenant(ctx, "t"); return t.Rev, err },
			write: func(rev int64) error {
				return s.SaveTenant(ctx, Tenant{ID: "t", Name: "Acme", Enabled: true, Description: "changed", Rev: rev})
			},
		},
		{
			name: "group",
			create: func() error {
				return s.SaveGroup(ctx, Group{ID: "g", TenantID: "t", Name: "team"})
			},
			read:  func() (int64, error) { g, err := s.GetGroup(ctx, "g"); return g.Rev, err },
			write: func(rev int64) error { return s.SaveGroup(ctx, Group{ID: "g", TenantID: "t", Name: "team", Rev: rev}) },
		},
		{
			name: "account",
			create: func() error {
				return s.CreateUser(ctx, User{ID: "u", Username: "jo", PasswordHash: "x", Enabled: true})
			},
			read: func() (int64, error) { u, err := s.GetUserByID(ctx, "u"); return u.Rev, err },
			write: func(rev int64) error {
				return s.UpdateUser(ctx, User{ID: "u", Username: "jo", Enabled: true, Fullname: "Jo", Rev: rev})
			},
		},
	}

	for _, k := range kinds {
		t.Run(k.name, func(t *testing.T) {
			if err := k.create(); err != nil {
				t.Fatalf("create: %v", err)
			}
			rev, err := k.read()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if rev == 0 {
				t.Fatalf("a %s comes back without a revision: nothing can be protected", k.name)
			}
			if err := k.write(rev); err != nil {
				t.Fatalf("a write on the current revision was refused: %v", err)
			}
			if err := k.write(rev); !errors.Is(err, ErrStale) {
				t.Fatalf("the second write from the same copy was accepted: %v", err)
			}
			// And a writer that read no revision still wins - a seed, an
			// import, a server-side read-modify-write.
			if err := k.write(0); err != nil {
				t.Errorf("a write with no revision was refused: %v", err)
			}
		})
	}
}
