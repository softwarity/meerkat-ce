package store

import (
	"context"
	"strings"
	"testing"
)

// A production gateway closes the developer surface whatever the database
// says. The stored switch travels - an export, a restored backup, a database
// copied from staging - and this is what stops it from arriving ON in front of
// customers.
func TestAProductionGatewayClosesTheDeveloperSurface(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.CreateUser(ctx, User{ID: "d", Username: "neo", PasswordHash: "x", Dev: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	u, err := s.GetUserByID(ctx, "d")
	if err != nil {
		t.Fatal(err)
	}
	// The switch is ON in the database, and stays ON: production does not
	// rewrite what an operator stored, it overrules it here.
	if err := s.SetDevMode(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !s.DevMode(ctx) || !s.DevAllowed(ctx, u) {
		t.Fatal("the developer surface is closed before production was declared")
	}

	SetProduction(true)
	t.Cleanup(func() { SetProduction(false) })

	if s.DevMode(ctx) {
		t.Fatal("a production gateway still offers the developer surface")
	}
	if s.DevAllowed(ctx, u) {
		t.Fatal("a developer is still allowed on a production gateway")
	}
	// And the switch is refused rather than saved and ignored: a stored ON
	// that every gate reads as OFF is an afternoon spent wondering why.
	err = s.SetDevMode(ctx, true)
	if err == nil {
		t.Fatal("the developer switch was accepted on a production gateway")
	}
	if !strings.Contains(err.Error(), "MEERKAT_PRODUCTION") {
		t.Fatalf("the refusal does not name what closed it: %v", err)
	}
	// Turning it OFF is always allowed: the environment closes a surface, it
	// never opens one.
	if err := s.SetDevMode(ctx, false); err != nil {
		t.Fatalf("closing the surface was refused: %v", err)
	}
}
