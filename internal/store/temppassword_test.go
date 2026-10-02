package store

import (
	"context"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// An administrator-issued password is temporary until its owner replaces it,
// and only that one: forcing a change at the next sign-in leaves the owner's
// own password standing, and expiring it would lock them out (CONSOLE-07).
func TestOnlyAnIssuedPasswordIsTemporary(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, User{ID: "u1", Username: "ann", PasswordHash: "h1", MustChangePassword: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	u, _ := st.GetUserByID(ctx, "u1")
	if !u.PasswordTemporary || u.PasswordChangedAt == 0 {
		t.Fatalf("a password issued at creation is temporary and dated: %+v", u)
	}
	if err := st.SetUserPassword(ctx, "u1", "h2", false); err != nil {
		t.Fatal(err)
	}
	if u, _ = st.GetUserByID(ctx, "u1"); u.PasswordTemporary {
		t.Fatal("the owner's own password is still marked temporary")
	}
	if err := st.SetMustChangePassword(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if u, _ = st.GetUserByID(ctx, "u1"); u.PasswordTemporary || !u.MustChangePassword {
		t.Fatal("forcing a change made the owner's password temporary")
	}
	if err := st.SetUserPassword(ctx, "u1", "h3", true); err != nil {
		t.Fatal(err)
	}
	if u, _ = st.GetUserByID(ctx, "u1"); !u.PasswordTemporary {
		t.Fatal("a reset password is not temporary")
	}

	// Aged by hand: the rule reads the issue time.
	if _, err := st.db.ExecContext(ctx, `UPDATE users SET password_changed_at = ? WHERE id = 'u1'`,
		time.Now().Add(-100*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	u, _ = st.GetUserByID(ctx, "u1")
	if !u.TemporaryPasswordExpired(72, time.Now()) {
		t.Error("a 100-hour-old temporary password works under a 72-hour limit")
	}
	if u.TemporaryPasswordExpired(0, time.Now()) {
		t.Error("no limit, yet the temporary password expired")
	}
	if u.TemporaryPasswordExpired(200, time.Now()) {
		t.Error("a 100-hour-old temporary password expired under a 200-hour limit")
	}
}

// A policy saved before the limit existed gets the default, not "no limit".
func TestAnOlderPolicyGetsTheDefaultLimit(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.SetSetting(ctx, SettingPasswordPolicy, map[string]int{"minLength": 10}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetPasswordPolicy(ctx).TemporaryHours; got != 72 {
		t.Errorf("an older policy got %d hours, want the default 72", got)
	}
	if err := st.SetSetting(ctx, SettingPasswordPolicy, PasswordPolicy{MinLength: 10, TemporaryHours: 0}); err != nil {
		t.Fatal(err)
	}
	if got := st.GetPasswordPolicy(ctx).TemporaryHours; got != 0 {
		t.Errorf("a policy that says no limit got %d hours", got)
	}
}
