package store

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/signing"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/vault"
)

// secretsOf returns, as they would appear in a dump, the private parts of the
// signing set (the PKCS#8 DER, base64 in the persisted JSON) and the
// simulator's key (base64).
func secretsOf(t *testing.T, set *signing.Set, simKey []byte) []string {
	t.Helper()
	blob, err := set.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Active []struct {
			Priv []byte `json:"priv"`
		} `json:"active"`
	}
	if err := json.Unmarshal(blob, &p); err != nil || len(p.Active) == 0 {
		t.Fatalf("persisted signing set: %v (%d keys)", err, len(p.Active))
	}
	var out []string
	for _, k := range p.Active {
		out = append(out, base64.StdEncoding.EncodeToString(k.Priv))
	}
	return append(out, base64.StdEncoding.EncodeToString(simKey))
}

// assertNothingInClear reads every setting, and on SQLite a snapshot of the
// whole database, looking for any of the secrets.
func assertNothingInClear(t *testing.T, st *Store, secrets []string) {
	t.Helper()
	ctx := context.Background()
	rows, err := st.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatal(err)
		}
		for _, s := range secrets {
			if strings.Contains(value, s) {
				t.Errorf("setting %s holds a private key in clear", key)
			}
		}
	}
	if st.db.dialect != dialectSQLite {
		return
	}
	dump := filepath.Join(t.TempDir(), "dump.db")
	if _, err := st.Snapshot(ctx, dump); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range secrets {
		if strings.Contains(string(raw), s) {
			t.Error("a snapshot of the database holds a private key in clear")
		}
	}
}

// verifyES256 checks a token against the set's current ES256 public key.
func verifyES256(t *testing.T, set *signing.Set, token string) bool {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %q", token)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("bad signature segment: %v", err)
	}
	h := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	pub, ok := set.PublicKey(signing.ES256).(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("no ES256 key in the set")
	}
	return ecdsa.Verify(pub, h[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:]))
}

// The identity signing keys and the simulator's token key are sealed under the
// master key (SEC-04): a dump of the database gives nothing to forge with.
func TestTheSigningKeysAreSealed(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	set, err := st.EnsureSigningSet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sim, err := st.EnsureSimulationKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{SettingSigningKeys, SettingSimulationKey} {
		var stored string
		if err := st.GetSetting(ctx, key, &stored); err != nil || !strings.HasPrefix(stored, sealedSetting) {
			t.Errorf("setting %s is not stored sealed: %v %.20q", key, err, stored)
		}
	}
	assertNothingInClear(t, st, secretsOf(t, set, sim))

	// And they read back as they were.
	again, ok, err := st.GetSigningSet(ctx)
	if err != nil || !ok {
		t.Fatalf("signing set after sealing: %v %v", ok, err)
	}
	if again.Kid(signing.ES256) != set.Kid(signing.ES256) {
		t.Error("the signing set changed on the way through the store")
	}
	if renewed, err := st.RenewSigningKeys(ctx); err != nil {
		t.Fatal(err)
	} else {
		assertNothingInClear(t, st, secretsOf(t, renewed, sim))
	}
	simAgain, err := st.EnsureSimulationKey(ctx)
	if err != nil || string(simAgain) != string(sim) {
		t.Errorf("simulation key after sealing: %v", err)
	}
}

// A database an older build wrote holds both keys in clear: the v83 step seals
// them, the keys themselves do not change, and a token signed before still
// verifies.
func TestTheSigningKeysOfAnOlderDatabaseAreSealed(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	ctx := context.Background()
	st, err := OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	// What a v82 build wrote: the set as a JSON object, the key as base64.
	set, err := signing.Generate()
	if err != nil {
		t.Fatal(err)
	}
	blob, err := set.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	sim := []byte("0123456789abcdef0123456789abcdef")
	if err := st.SetSetting(ctx, SettingSigningKeys, json.RawMessage(blob)); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, SettingSimulationKey, base64.StdEncoding.EncodeToString(sim)); err != nil {
		t.Fatal(err)
	}
	if err := st.db.setSchemaVersion(82); err != nil {
		t.Fatal(err)
	}
	token, err := set.SignJWT(signing.ES256, map[string]any{"sub": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatalf("opening the older database: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if v, _ := st.db.schemaVersion(); v != schemaVersion {
		t.Errorf("schema v%d after the upgrade, want v%d", v, schemaVersion)
	}
	assertNothingInClear(t, st, secretsOf(t, set, sim))
	loaded, ok, err := st.GetSigningSet(ctx)
	if err != nil || !ok {
		t.Fatalf("signing set after the step: %v %v", ok, err)
	}
	if !verifyES256(t, loaded, token) {
		t.Error("a token signed before the upgrade no longer verifies")
	}
	if got, err := st.EnsureSimulationKey(ctx); err != nil || string(got) != string(sim) {
		t.Errorf("the simulator's key changed in the upgrade: %v", err)
	}
}

// The rotation of the master key takes both keys with it.
func TestTheMasterKeyRotationTakesTheSigningKeys(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	ctx := context.Background()
	t.Setenv("MEERKAT_VAULT_KEY", keyA)
	st, err := OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	set, err := st.EnsureSigningSet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sim, err := st.EnsureSimulationKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	token, err := set.SignJWT(signing.ES256, map[string]any{"sub": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	t.Setenv("MEERKAT_VAULT_KEY", keyB)
	t.Setenv(vault.PreviousKeyEnv, keyA)
	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatalf("opening during the rotation: %v", err)
	}
	_ = st.Close()

	t.Setenv(vault.PreviousKeyEnv, "")
	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatalf("opening after the rotation: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	loaded, ok, err := st.GetSigningSet(ctx)
	if err != nil || !ok {
		t.Fatalf("signing set after the rotation: %v %v", ok, err)
	}
	if !verifyES256(t, loaded, token) {
		t.Error("a token signed before the rotation no longer verifies")
	}
	if got, err := st.EnsureSimulationKey(ctx); err != nil || string(got) != string(sim) {
		t.Errorf("the simulator's key after the rotation: %v", err)
	}
}

// During a rolling upgrade a node of the previous build runs beside the
// migrated database and can write the simulator's key in clear again: the next
// start of this build seals it.
func TestAKeyWrittenInClearAfterTheUpgradeIsSealedAtTheNextStart(t *testing.T) {
	dir, url := t.TempDir(), dbtest.URL(t)
	ctx := context.Background()
	st, err := OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	sim := []byte("fedcba9876543210fedcba9876543210")
	// What the older node writes: the key as plain base64, stamped v83.
	if err := st.SetSetting(ctx, SettingSimulationKey, base64.StdEncoding.EncodeToString(sim)); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	st, err = OpenAt(dir, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var stored string
	if err := st.GetSetting(ctx, SettingSimulationKey, &stored); err != nil || !strings.HasPrefix(stored, sealedSetting) {
		t.Fatalf("the key written in clear is still in clear: %v %.20q", err, stored)
	}
	if got, err := st.EnsureSimulationKey(ctx); err != nil || string(got) != string(sim) {
		t.Errorf("the key changed while being sealed: %v", err)
	}
}
