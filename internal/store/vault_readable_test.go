package store

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/vault"
)

// After a migration under another master key the names are all there and no
// secret opens. The vault list says which, without a value ever leaving: the
// secret sealed under key A reads as unreadable to a gateway on key B, and
// readable again on A; a plain value is never in question.
func TestASecretTheMasterKeyCannotOpenIsNamed(t *testing.T) {
	ctx := context.Background()
	dir, url := t.TempDir(), dbtest.URL(t)
	open := func(key string) *Store {
		t.Helper()
		t.Setenv("MEERKAT_VAULT_KEY", key)
		st, err := OpenAt(dir, url)
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	st := open(keyA)
	for _, e := range []vault.Entry{
		{Name: "db", Kind: vault.KindSecret, Scope: vault.ScopeInfra, Value: "hunter2"},
		{Name: "region", Kind: vault.KindValue, Scope: vault.ScopeInfra, Value: "eu-west"},
	} {
		if err := st.SaveVaultEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := st.UnreadableVaultSecrets(ctx, []string{vault.ScopeInfra}); err != nil || len(got) != 0 {
		t.Fatalf("on its own key: %v %v", got, err)
	}
	_ = st.Close()

	st = open(keyB)
	got, err := st.UnreadableVaultSecrets(ctx, []string{vault.ScopeInfra})
	if err != nil {
		t.Fatal(err)
	}
	if !got[vault.ScopeInfra+"/db"] || got[vault.ScopeInfra+"/region"] || len(got) != 1 {
		t.Fatalf("on another key, want only the secret named: %v", got)
	}
	_ = st.Close()

	st = open(keyA)
	defer func() { _ = st.Close() }()
	if got, err := st.UnreadableVaultSecrets(ctx, []string{vault.ScopeInfra}); err != nil || len(got) != 0 {
		t.Fatalf("back on its key: %v %v", got, err)
	}
}
