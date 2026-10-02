package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A TOTP secret never sits in the database in clear (SEC-06): written sealed,
// read back as the secret, and one written before is sealed at the next start.
func TestTOTPSecretsAreSealedAtRest(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	st, err := OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, id := range []string{"u1", "u2"} {
		if err := st.CreateUser(ctx, User{ID: id, Username: id, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	const secret = "JBSWY3DPEHPK3PXP"
	if err := st.SetUserTOTPPending(ctx, "u1", secret); err != nil {
		t.Fatal(err)
	}
	if err := st.EnableUserTOTP(ctx, "u1", secret, []string{"h"}); err != nil {
		t.Fatal(err)
	}
	raw := func(id string) string {
		var v string
		if err := st.db.QueryRowContext(ctx, `SELECT totp_secret FROM users WHERE id = ?`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got := raw("u1"); !strings.HasPrefix(got, sealedTOTP) || strings.Contains(got, secret) {
		t.Fatalf("the secret is stored as %q", got)
	}
	if s, _ := st.GetUserTOTP(ctx, "u1"); s.Secret != secret || !s.Enrolled {
		t.Fatalf("read back as %+v", s)
	}

	// A secret from before: in clear, until the next start seals it.
	if _, err := st.db.ExecContext(ctx, `UPDATE users SET totp_secret = ? WHERE id = 'u2'`, secret); err != nil {
		t.Fatal(err)
	}
	if s, _ := st.GetUserTOTP(ctx, "u2"); s.Secret != secret {
		t.Fatalf("a secret in clear no longer reads: %+v", s)
	}
	_ = st.Close()
	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if got := raw("u2"); !strings.HasPrefix(got, sealedTOTP) {
		t.Fatalf("a secret in clear survived a restart: %q", got)
	}
	if s, _ := st.GetUserTOTP(ctx, "u2"); s.Secret != secret {
		t.Fatalf("the sealed secret reads as %+v", s)
	}
}
