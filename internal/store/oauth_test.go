package store

import (
	"context"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A spent refresh token is served again within its window - the same agent
// refreshing twice at once - and refused past it, and by a revocation at once.
func TestASpentRefreshTokenHasAShortGrace(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, User{ID: "u1", Username: "root", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddAPIToken(ctx, NewToken{ID: "t1", UserID: "u1", Name: "agent", TokenHash: "h",
		Prefix: "mk_abc", Plane: PlaneAdmin, Scope: ScopeFull}); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).Unix()
	if err := st.SaveRefreshToken(ctx, "r1", "t1", expires); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if id, err := st.TakeRefreshToken(ctx, "r1", RefreshReuseWindow); err != nil || id != "t1" {
			t.Fatalf("presentation %d within the window: %q %v", i+1, id, err)
		}
	}
	if _, err := st.TakeRefreshToken(ctx, "r1", 0); err == nil {
		t.Error("a revocation must not accept a spent refresh token")
	}
	past := time.Now().Add(-RefreshReuseWindow - time.Second).Unix()
	if _, err := st.db.ExecContext(ctx, `UPDATE oauth_refresh SET used_at = ? WHERE token_hash = ?`, past, "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.TakeRefreshToken(ctx, "r1", RefreshReuseWindow); err == nil {
		t.Error("a refresh token spent past its window was accepted")
	}
	if err := st.PurgeOAuthLeftovers(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM oauth_refresh`).Scan(&n); err != nil || n != 0 {
		t.Errorf("a refresh token spent past its window is kept: %d rows (%v)", n, err)
	}
	if _, err := st.TakeRefreshToken(ctx, "unknown", RefreshReuseWindow); err == nil {
		t.Error("an unknown refresh token was accepted")
	}
}
