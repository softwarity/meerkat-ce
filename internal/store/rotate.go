package store

import (
	"context"
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
// tunnel's host key and the TOTP secrets. Rotating it is a restart with two
// keys: MEERKAT_VAULT_KEY is the new one, MEERKAT_VAULT_KEY_PREVIOUS the one it
// replaces. Each node then reads both, and the first to start seals again,
// under the new key, everything the old one sealed. Once every node runs with
// the new key, the old one is dropped. Nothing is ever unreadable in between:
// a node still on the old key alone is the only thing that breaks, and it
// breaks loudly, as a wrong key always has.

// loadCipher reads the key (and, during a rotation, the previous one), then
// seals again whatever needs it.
func (s *Store) loadCipher(dataDir string) error {
	key, err := vault.LoadOrCreateKey(dataDir, os.Getenv("MEERKAT_VAULT_KEY"))
	if err != nil {
		return err
	}
	previous, err := vault.LoadPreviousKey(os.Getenv(vault.PreviousKeyEnv))
	if err != nil {
		return err
	}
	if s.vaultCipher, err = vault.NewCipher(key, previous); err != nil {
		return err
	}
	ctx := context.Background()
	if s.vaultCipher.Rotating() {
		n, err := s.resealStale(ctx)
		if err != nil {
			return err
		}
		slog.Info("vault key rotation: sealed again under the new key", "values", n,
			"next", "once every node runs with the new key, remove "+vault.PreviousKeyEnv)
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
	// The developer tunnel's host key is a setting, stored as JSON.
	var sealed string
	if err := s.GetSetting(ctx, SettingPlugHostKey, &sealed); err == nil && sealed != "" {
		plain, stale, err := s.vaultCipher.OpenStale(sealed)
		if err != nil {
			return total, fmt.Errorf("store: vault key rotation, plug host key: %w", err)
		}
		if stale {
			if err := s.SetPlugHostKey(ctx, plain); err != nil {
				return total, err
			}
			total++
		}
	}
	return total, nil
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
