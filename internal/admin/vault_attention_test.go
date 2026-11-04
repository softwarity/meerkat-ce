package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/vault"
)

// What wants a look on the vault screen: an entry an import reserved and nobody
// filled - a secret included, whose value never travels but whose absence does
// - and an entry whose reminder date has come.
func TestTheVaultSaysWhatIsEmptyAndWhatHasLapsed(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, kind := range []string{vault.KindSecret, vault.KindValue} {
		if _, err := f.api.st.ReserveVaultEntry(ctx, vault.Entry{Name: "reserved-" + kind, Kind: kind, Scope: vault.ScopeInfra}); err != nil {
			t.Fatal(err)
		}
	}
	day := time.Now().UTC().Truncate(24 * time.Hour)
	for name, at := range map[string]time.Time{"lapsed": day.AddDate(0, 0, -1), "later": day.AddDate(0, 0, 30)} {
		if err := f.api.st.SaveVaultEntry(ctx, vault.Entry{Name: name, Kind: vault.KindSecret, Scope: vault.ScopeInfra,
			Value: "s3cret", ExpiresAt: at.Unix()}); err != nil {
			t.Fatal(err)
		}
	}
	code, out := f.call(t, "GET", "/api/vault", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("vault: %d %s", code, out)
	}
	var list []vaultEntryView
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatal(err)
	}
	got := map[string]vaultEntryView{}
	for _, e := range list {
		got[e.Name] = e
	}
	for name, want := range map[string]struct{ has, expired bool }{
		"reserved-secret": {false, false}, "reserved-value": {false, false},
		"lapsed": {true, true}, "later": {true, false},
	} {
		if e := got[name]; e.HasValue != want.has || e.Expired != want.expired {
			t.Errorf("%s: hasValue %t expired %t, want %t %t", name, e.HasValue, e.Expired, want.has, want.expired)
		}
	}
	if got["lapsed"].Value != "" {
		t.Fatal("a secret's value travelled")
	}
}
