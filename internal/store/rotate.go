package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/softwarity/meerkat/internal/vault"
)

// The vault's master key, and its rotation (SEC-06).
//
// One key seals everything this database must not hold in clear: the vault's
// secrets, the certificates' private keys, the ACME account, the developer
// tunnel's host key, the TOTP secrets, the identity signing keys and the
// simulator's token key. Rotating it is a restart with two
// keys: MEERKAT_VAULT_KEY is the new one, MEERKAT_VAULT_KEY_PREVIOUS the one it
// replaces. Each node then reads both, and the first to start seals again,
// under the new key, everything the old one sealed. Once every node runs with
// the new key, the old one is dropped. Nothing is ever unreadable in between:
// a node still on the old key alone is the only thing that breaks, and it
// breaks loudly, as a wrong key always has.

// loadCipher reads the key (and, during a rotation, the previous one). It runs
// before the schema is migrated: a step of the ledger may have to seal what an
// older build stored in clear (v83).
func (s *Store) loadCipher(dataDir string) error {
	key, err := vault.LoadOrCreateKey(dataDir, os.Getenv("MEERKAT_VAULT_KEY"))
	if err != nil {
		return err
	}
	previous, err := vault.LoadPreviousKey(os.Getenv(vault.PreviousKeyEnv))
	if err != nil {
		return err
	}
	s.vaultCipher, err = vault.NewCipher(key, previous)
	return err
}

// sealAtRest runs once the schema is current: during a rotation it seals again
// whatever the previous key sealed, then seals what is still in clear.
func (s *Store) sealAtRest() error {
	ctx := context.Background()
	if s.vaultCipher.Rotating() {
		n, err := s.resealStale(ctx)
		if err != nil {
			return err
		}
		slog.Info("vault key rotation: sealed again under the new key", "values", n,
			"next", "once every node runs with the new key, remove "+vault.PreviousKeyEnv)
	}
	// What a node of the previous build may have written in clear beside a
	// migrated database, during a rolling upgrade (see sealPlainSettings).
	if err := sealPlainSettings(ctx, s.db, s.vaultCipher); err != nil {
		return fmt.Errorf("store: seal the signing keys: %w", err)
	}
	// The TOTP secrets written in clear before they were sealed.
	return s.sealStoredTOTP(ctx)
}

// sealedColumn is one place a sealed value lives.
type sealedColumn struct {
	table, id, value, where string
	// prefix marks the sealed part of the value ("" when it is all sealed).
	prefix string
}

var sealedColumns = []sealedColumn{
	{table: "vault_entries", id: "scope || '/' || name", value: "value", where: "kind = 'secret'"},
	{table: "certificates", id: "id", value: "key_sealed", where: "key_sealed != ''"},
	{table: "acme_cache", id: "key", value: "value", where: "value != ''"},
	{table: "users", id: "id", value: "totp_secret", where: "totp_secret LIKE 'sealed:%'", prefix: sealedTOTP},
	{table: "users", id: "id", value: "totp_pending", where: "totp_pending LIKE 'sealed:%'", prefix: sealedTOTP},
}

// resealStale seals again, under the current key, every value the previous
// key sealed, and says how many. A row is only rewritten if it still holds
// what was read, so two nodes rotating at once do not fight over it.
func (s *Store) resealStale(ctx context.Context) (int, error) {
	total := 0
	for _, c := range sealedColumns {
		n, err := s.resealColumn(ctx, c)
		if err != nil {
			return total, fmt.Errorf("store: vault key rotation, %s.%s: %w", c.table, c.value, err)
		}
		total += n
	}
	for _, c := range sealedSettings {
		n, err := s.resealSetting(ctx, c.key, c.prefix)
		if err != nil {
			return total, fmt.Errorf("store: vault key rotation, setting %s: %w", c.key, err)
		}
		total += n
	}
	return total, nil
}

// sealedSettings are the settings that hold a sealed value, stored as a JSON
// string: the blob after its marker. The tunnel's host key has none, it was
// sealed before the marker existed.
var sealedSettings = []struct{ key, prefix string }{
	{SettingPlugHostKey, ""},
	{SettingSigningKeys, sealedSetting},
	{SettingSimulationKey, sealedSetting},
}

// resealSetting is resealColumn for one setting: rewritten only if it still
// holds what was read.
func (s *Store) resealSetting(ctx context.Context, key, prefix string) (int, error) {
	var stored, text string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if json.Unmarshal([]byte(stored), &text) != nil || text == "" {
		return 0, nil
	}
	blob, sealed := strings.CutPrefix(text, prefix)
	if !sealed {
		return 0, nil
	}
	plain, stale, err := s.vaultCipher.OpenStale(blob)
	if err != nil || !stale {
		return 0, err
	}
	fresh, err := s.vaultCipher.Seal(plain)
	if err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(prefix + fresh)
	if err != nil {
		return 0, err
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE settings SET value = ? WHERE key = ? AND value = ?`, string(encoded), key, stored); err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Store) resealColumn(ctx context.Context, c sealedColumn) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+c.id+`, `+c.value+` FROM `+c.table+` WHERE `+c.where)
	if err != nil {
		return 0, err
	}
	type found struct{ id, value string }
	var all []found
	for rows.Next() {
		var f found
		if err := rows.Scan(&f.id, &f.value); err != nil {
			_ = rows.Close()
			return 0, err
		}
		all = append(all, f)
	}
	_ = rows.Close()
	n := 0
	for _, f := range all {
		blob := strings.TrimPrefix(f.value, c.prefix)
		plain, stale, err := s.vaultCipher.OpenStale(blob)
		if err != nil {
			return n, fmt.Errorf("%s: %w", f.id, err)
		}
		if !stale {
			continue
		}
		fresh, err := s.vaultCipher.Seal(plain)
		if err != nil {
			return n, err
		}
		if _, err := s.db.ExecContext(ctx,
			`UPDATE `+c.table+` SET `+c.value+` = ? WHERE `+c.id+` = ? AND `+c.value+` = ?`,
			c.prefix+fresh, f.id, f.value); err != nil {
			return n, fmt.Errorf("%s: %w", f.id, err)
		}
		n++
	}
	return n, nil
}
