package store

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// What the developer tunnel (DEV-11) needs from the store: the server's own
// identity, and the keys that are allowed to open one.
//
// Both live here rather than in the Enterprise package that uses them, for the
// same reason the certificates do: they are rows, they are sealed at rest with
// everything else, and the community build has to be able to READ the column
// even though it never starts an agent - a developer's key is deposited from
// the profile page, which is not Enterprise code.

// SettingPlugHostKey holds the tunnel's SSH host identity, SEALED (VAULT-01).
//
// plug standalone keeps it in a file, which is right for a container someone
// owns; embedded, a recreated pod would discard it and every developer's
// client would report a changed host key - the warning that is supposed to
// mean something. In the store it survives the pod, and being sealed it does
// not survive a stolen database file on its own.
const SettingPlugHostKey = "plug_host_key"

// PlugHostKey returns the agent's host key in PEM, "" when none was generated
// yet. The caller generates one and stores it on first use.
func (s *Store) PlugHostKey(ctx context.Context) (string, error) {
	var sealed string
	if err := s.GetSetting(ctx, SettingPlugHostKey, &sealed); err != nil {
		return "", nil //nolint:nilerr // absent is the first-use case, not a failure.
	}
	if sealed == "" {
		return "", nil
	}
	plain, err := s.vaultCipher.Open(sealed)
	if err != nil {
		return "", fmt.Errorf("store: plug host key: %w", err)
	}
	return plain, nil
}

// SetPlugHostKey seals and stores the host key.
func (s *Store) SetPlugHostKey(ctx context.Context, pemText string) error {
	sealed, err := s.vaultCipher.Seal(pemText)
	if err != nil {
		return fmt.Errorf("store: plug host key: %w", err)
	}
	return s.SetSetting(ctx, SettingPlugHostKey, sealed)
}

// SanitizeDevKey validates a developer's PUBLIC SSH key, in the one line
// format an authorized_keys file holds. "" clears it.
//
// A key and not a certificate, and that was the decision: a CA signing
// short-lived certificates was examined and dropped, because a certificate is
// only revocable by waiting for it to expire. Meerkat is the cluster's
// gateway - it is up by definition, so it can answer "is this key still
// allowed" at every connection, and someone who leaves stops being able to
// tunnel the moment their key is removed.
func SanitizeDevKey(text string) error {
	if text == "" {
		return nil
	}
	if len(text) > 8<<10 {
		return fmt.Errorf("key is too large (%d bytes): the limit is 8 KiB", len(text))
	}
	if strings.ContainsAny(text, "\r\n") && strings.TrimSpace(text) != strings.TrimSpace(strings.SplitN(text, "\n", 2)[0]) {
		return fmt.Errorf("paste a SINGLE key: an authorized_keys line, not a file")
	}
	key, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(text))
	if err != nil {
		return fmt.Errorf("this is not an SSH public key: paste the contents of a .pub file (ssh-ed25519, ecdsa-sha2-*, ssh-rsa), not a private key or a certificate")
	}
	_ = comment
	// A signed certificate would authenticate too, and it is exactly what was
	// ruled out: it would carry its own validity and stop answering to the
	// list here.
	if strings.Contains(key.Type(), "cert-v01@openssh.com") {
		return fmt.Errorf("a signed certificate is not accepted here: deposit the public key itself, so removing it takes effect at once")
	}
	return nil
}

// SetUserDevKey stores (or clears, with "") a developer's public key. Like the
// avatar and the certificate before it, it never rides the User struct: read
// it through GetUserDevKey only.
func (s *Store) SetUserDevKey(ctx context.Context, id, key string) error {
	if err := SanitizeDevKey(key); err != nil {
		return fmt.Errorf("store: user %q: %w", id, err)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET dev_key = ? WHERE id = ?`, strings.TrimSpace(key), id)
	if err != nil {
		return fmt.Errorf("store: set dev key for %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: user %q not found", id)
	}
	return nil
}

// GetUserDevKey returns the stored authorized_keys line ("" when none).
func (s *Store) GetUserDevKey(ctx context.Context, id string) (string, error) {
	var key string
	if err := s.db.QueryRowContext(ctx, `SELECT dev_key FROM users WHERE id = ?`, id).Scan(&key); err != nil {
		return "", fmt.Errorf("store: get dev key for %q: %w", id, err)
	}
	return key, nil
}

// DevKeyOwner is one line of the tunnel's admission list.
type DevKeyOwner struct {
	Username string
	Key      string // the authorized_keys line
}

// DevKeyOwners lists who may open a tunnel right now: an ENABLED account, with
// the developer capability, that has deposited a key. The developer-mode
// switch is not applied here - the caller already stops the agent when it goes
// off, and folding it in would make an empty list mean two different things.
//
// The whole list, because the caller matches on a fingerprint it computes
// itself and caches: SQL cannot compare keys, and the table is small.
func (s *Store) DevKeyOwners(ctx context.Context) ([]DevKeyOwner, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT username, dev_key FROM users WHERE dev = ? AND enabled = ? AND dev_key != '' ORDER BY username`, true, true)
	if err != nil {
		return nil, fmt.Errorf("store: dev key owners: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []DevKeyOwner
	for rows.Next() {
		var o DevKeyOwner
		if err := rows.Scan(&o.Username, &o.Key); err != nil {
			return nil, fmt.Errorf("store: dev key owners: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
