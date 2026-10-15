package store

import (
	"context"
	"testing"
)

// The developer surface follows its switch, and nothing else: no environment
// variable sits above it, on a production gateway as anywhere else.
func TestTheDeveloperSurfaceFollowsItsSwitch(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.CreateUser(ctx, User{ID: "d", Username: "neo", PasswordHash: "x", Dev: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	u, err := s.GetUserByID(ctx, "d")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEERKAT_PRODUCTION", "1") // no longer read
	for _, on := range []bool{true, false, true} {
		if err := s.SetDevMode(ctx, on); err != nil {
			t.Fatalf("switching it %v: %v", on, err)
		}
		if s.DevMode(ctx) != on || s.DevAllowed(ctx, u) != on {
			t.Fatalf("switched %v, read %v (allowed %v)", on, s.DevMode(ctx), s.DevAllowed(ctx, u))
		}
	}
}
