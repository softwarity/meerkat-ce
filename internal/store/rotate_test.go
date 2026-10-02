package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/vault"
)

const (
	keyA = "1111111111111111111111111111111111111111111111111111111111111111"
	keyB = "2222222222222222222222222222222222222222222222222222222222222222"
)

// Rotating the master key is a restart with the new key and the old one
// (SEC-06): everything the old one sealed is sealed again under the new one,
// and the old one can then go.
func TestTheMasterKeyRotates(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	ctx := context.Background()

	t.Setenv("MEERKAT_VAULT_KEY", keyA)
	st, err := OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveVaultEntry(ctx, vault.Entry{Name: "db", Kind: vault.KindSecret, Scope: vault.ScopeInfra, Value: "hunter2"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, User{ID: "u1", Username: "u1", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.EnableUserTOTP(ctx, "u1", "JBSWY3DPEHPK3PXP", nil); err != nil {
		t.Fatal(err)
	}
	if err := st.SetPlugHostKey(ctx, "host-key-pem"); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	// The new key alone cannot read what the old one sealed: the failure a
	// rotation is there to avoid.
	t.Setenv("MEERKAT_VAULT_KEY", keyB)
	st, err = OpenAt(dir, url)
	if err == nil {
		if _, err = st.GetVaultEntry(ctx, vault.ScopeInfra, "db"); err == nil {
			t.Fatal("a secret sealed by another key was read")
		}
		_ = st.Close()
	}

	// The new key with the old one: everything is sealed again.
	t.Setenv(vault.PreviousKeyEnv, keyA)
	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatalf("opening during the rotation: %v", err)
	}
	_ = st.Close()

	// The old key gone: everything still reads.
	t.Setenv(vault.PreviousKeyEnv, "")
	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatalf("opening after the rotation: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if e, err := st.GetVaultEntry(ctx, vault.ScopeInfra, "db"); err != nil || e.Value != "hunter2" {
		t.Errorf("vault secret after rotation: %v %q", err, e.Value)
	}
	if s, err := st.GetUserTOTP(ctx, "u1"); err != nil || s.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("TOTP secret after rotation: %v %+v", err, s)
	}
	if k, err := st.PlugHostKey(ctx); err != nil || k != "host-key-pem" {
		t.Errorf("plug host key after rotation: %v %q", err, k)
	}
	var raw string
	_ = st.db.QueryRowContext(ctx, `SELECT totp_secret FROM users WHERE id = 'u1'`).Scan(&raw)
	if !strings.HasPrefix(raw, sealedTOTP) {
		t.Errorf("the TOTP secret lost its marker: %q", raw)
	}
}
