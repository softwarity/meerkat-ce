package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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

// DevKey is one of a developer's public keys, as the profile lists it.
type DevKey struct {
	ID          string
	Line        string // the authorized_keys line
	Fingerprint string // SHA256, as `plug pubkey` and ssh-keygen -lf print it
	CreatedAt   int64
}

// ErrDevKeyTaken refuses a key already deposited, by this account or another.
// By another, because an accepted connection must name ONE person; by the same,
// because a second copy would be a line to remove twice.
var ErrDevKeyTaken = errors.New("this key is already deposited on this gateway")

// AddDevKey stores one more public key for a developer - one per workstation,
// since plug keeps a pair per profile.
func (s *Store) AddDevKey(ctx context.Context, userID, line string) (DevKey, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return DevKey{}, fmt.Errorf("store: user %q: paste a public key", userID)
	}
	if err := SanitizeDevKey(line); err != nil {
		return DevKey{}, fmt.Errorf("store: user %q: %w", userID, err)
	}
	pub, _, _, _, _ := ssh.ParseAuthorizedKey([]byte(line))
	k := DevKey{ID: NewEventID(), Line: line, Fingerprint: ssh.FingerprintSHA256(pub), CreatedAt: time.Now().Unix()}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dev_keys WHERE fingerprint = ?`, k.Fingerprint).Scan(&n); err != nil {
		return DevKey{}, fmt.Errorf("store: dev key: %w", err)
	}
	if n > 0 {
		return DevKey{}, ErrDevKeyTaken
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO dev_keys (id, user_id, key_line, fingerprint, created_at) VALUES (?, ?, ?, ?, ?)`,
		k.ID, userID, k.Line, k.Fingerprint, k.CreatedAt); err != nil {
		return DevKey{}, fmt.Errorf("store: add dev key for %q: %w", userID, err)
	}
	return k, nil
}

// ListDevKeys lists one developer's keys, oldest first - the order they were
// deposited in, which is the order a person remembers their machines.
func (s *Store) ListDevKeys(ctx context.Context, userID string) ([]DevKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, key_line, fingerprint, created_at FROM dev_keys WHERE user_id = ? ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: dev keys of %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []DevKey
	for rows.Next() {
		var k DevKey
		if err := rows.Scan(&k.ID, &k.Line, &k.Fingerprint, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: dev keys of %q: %w", userID, err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// DeleteDevKey removes one of a developer's own keys. The owner is part of the
// match: an id alone would let one developer remove another's key. The removed
// key is returned, so the trail can name it by its fingerprint.
func (s *Store) DeleteDevKey(ctx context.Context, userID, id string) (DevKey, bool, error) {
	var k DevKey
	err := s.db.QueryRowContext(ctx,
		`SELECT id, key_line, fingerprint, created_at FROM dev_keys WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&k.ID, &k.Line, &k.Fingerprint, &k.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DevKey{}, false, nil
	}
	if err != nil {
		return DevKey{}, false, fmt.Errorf("store: dev key %q: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM dev_keys WHERE id = ? AND user_id = ?`, id, userID); err != nil {
		return DevKey{}, false, fmt.Errorf("store: remove dev key %q: %w", id, err)
	}
	return k, true, nil
}

// DevKeyOwner is one line of the tunnel's admission list.
type DevKeyOwner struct {
	Username string
	Key      string // the authorized_keys line
}

// DevKeyOwners lists who may open a tunnel right now, one line per KEY: an
// ENABLED account, with the developer capability, and each key it deposited.
// The developer-mode switch is not applied here - the caller already stops the
// agent when it goes off, and folding it in would make an empty list mean two
// different things.
//
// The whole list, because the caller matches on a fingerprint it computes
// itself and caches: the table is small.
func (s *Store) DevKeyOwners(ctx context.Context) ([]DevKeyOwner, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.username, k.key_line FROM dev_keys k JOIN users u ON u.id = k.user_id
		 WHERE u.dev = ? AND u.enabled = ? ORDER BY u.username, k.created_at`, true, true)
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

// SettingPlug is the developer tunnel's own switch and the address developers
// use to reach it (DEV-11). INFRASTRUCTURE, beside developer mode rather than
// under it: developer mode is a CONFIGURATION - the installation offers its
// developers the tooling, the API docs, the simulated sign-ins - while this
// opens a port on the cluster and exercises the deployment's rights over it
// (a Docker socket, a Kubernetes role). The tunnel runs only when both say so.
const SettingPlug = "plug"

// PlugSetting is that setting.
type PlugSetting struct {
	// Enabled opens the tunnel. Ships OFF, like the agent endpoint: a port
	// that reaches into the cluster is a decision somebody takes, not a
	// side effect of an upgrade.
	Enabled bool `json:"enabled"`
	// Host and Port are what a DEVELOPER types: the published address, which
	// the gateway cannot work out - a NodePort, a LoadBalancer or a published
	// Docker port sits between it and the laptop. Every command the console
	// and the profile page print carries them, so they are copied rather
	// than translated. Empty: localhost and the tunnel's own port.
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

// Plug reads the setting; missing means off.
func (s *Store) Plug(ctx context.Context) PlugSetting {
	var p PlugSetting
	_ = s.GetSetting(ctx, SettingPlug, &p)
	return p
}

// SanitizePlug refuses what cannot be typed into a terminal.
func SanitizePlug(p *PlugSetting) error {
	p.Host = strings.TrimSpace(p.Host)
	if strings.ContainsAny(p.Host, " /:@\t") {
		return fmt.Errorf("the host developers use is a name or an address, such as dev.example.com - no scheme, no port, no path (got %q)", p.Host)
	}
	if p.Port < 0 || p.Port > 65535 {
		return fmt.Errorf("the port developers use is a number from 1 to 65535, or empty for the tunnel's own (got %d)", p.Port)
	}
	return nil
}

// PlugOpen says whether the tunnel may run: developer mode on (which the
// production environment closes) AND the tunnel's own switch.
func (s *Store) PlugOpen(ctx context.Context) bool {
	return s.DevMode(ctx) && s.Plug(ctx).Enabled
}

// PlugAllowed is DevAllowed for the tunnel: the account may use the
// developer tooling AND the tunnel is open. It guards the page where a key is
// deposited - a key for a door that does not exist is a key nobody can use,
// and a page offering it says the door is there.
func (s *Store) PlugAllowed(ctx context.Context, u User) bool {
	return s.DevAllowed(ctx, u) && s.Plug(ctx).Enabled
}

// DefaultPlugPort is the tunnel's own port (devtunnel.DefaultAddr), which is
// also what a developer types when nothing is published in between.
const DefaultPlugPort = 22222

// DefaultPlugHost is the host the commands carry until somebody records the
// published one: what a port-forward, or a gateway on the developer's own
// machine, answers on - and a command that reads as a command.
const DefaultPlugHost = "localhost"

// Address is what a developer types to reach the tunnel: the published host
// and port, or the defaults.
func (p PlugSetting) Address() (host string, port int) {
	host, port = p.Host, p.Port
	if host == "" {
		host = DefaultPlugHost
	}
	if port == 0 {
		port = DefaultPlugPort
	}
	return host, port
}

// Developer is an account holding the developer capability, as the tunnel's
// page lists it: who could open one, and whether they have deposited a key.
type Developer struct {
	Username string
	Fullname string
	Keys     []string // the authorized_keys lines, none when nothing was deposited
}

// Developers lists every enabled account with the developer capability, key
// or not - the ones without are the people the page has to point at the
// profile, which DevKeyOwners, asked only who may connect, leaves out.
func (s *Store) Developers(ctx context.Context) ([]Developer, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.username, u.fullname, COALESCE(k.key_line, '') FROM users u
		 LEFT JOIN dev_keys k ON k.user_id = u.id
		 WHERE u.dev = ? AND u.enabled = ? ORDER BY u.username, k.created_at`, true, true)
	if err != nil {
		return nil, fmt.Errorf("store: developers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Developer
	for rows.Next() {
		var name, full, line string
		if err := rows.Scan(&name, &full, &line); err != nil {
			return nil, fmt.Errorf("store: developers: %w", err)
		}
		if len(out) == 0 || out[len(out)-1].Username != name {
			out = append(out, Developer{Username: name, Fullname: full})
		}
		if line != "" {
			out[len(out)-1].Keys = append(out[len(out)-1].Keys, line)
		}
	}
	return out, rows.Err()
}
