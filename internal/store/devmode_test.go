package store

import (
	"context"
	"testing"
)

// The developer tooling follows the account and nothing above it: root always,
// any other account by its dev capability, never a disabled one.
func TestTheDeveloperToolingFollowsTheAccount(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	for _, u := range []User{
		{ID: "d", Username: "neo", PasswordHash: "x", Dev: true, Enabled: true},
		{ID: "p", Username: "plain", PasswordHash: "x", Enabled: true},
		{ID: "r", Username: "boss", PasswordHash: "x", Root: true, Enabled: true},
		{ID: "x", Username: "gone", PasswordHash: "x", Dev: true},
	} {
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	allowed := func(id string) bool {
		u, err := s.GetUserByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return s.DevAllowed(ctx, u)
	}
	t.Setenv("MEERKAT_PRODUCTION", "1") // no longer read
	if !allowed("d") || allowed("p") || !allowed("r") || allowed("x") {
		t.Fatalf("dev %v, plain %v, root %v, disabled dev %v", allowed("d"), allowed("p"), allowed("r"), allowed("x"))
	}

	// Closing a gateway is taking the capability from every account but root.
	n, err := s.ClearDevCapability(ctx)
	if err != nil || n != 2 {
		t.Fatalf("cleared %d accounts (%v), want the two that carried it", n, err)
	}
	if allowed("d") || !allowed("r") {
		t.Fatalf("after clearing: dev %v, root %v", allowed("d"), allowed("r"))
	}
}
