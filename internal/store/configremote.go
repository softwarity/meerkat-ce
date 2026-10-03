package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Named git locations a configuration travels through (CFG-07).
//
// This package stores them and nothing more: what a repository is, how it is
// reached and what a commit costs belongs to internal/confrepo and its driver.
// What matters here is that a location is a ROW - named, listed, chosen from a
// dialog - and that a saved configuration points at one.

// ConfigRemote is one place a configuration lives: a repository, a branch, and
// a directory inside it.
//
// TokenRef is a vault reference (${name}), never a value. The distinction is
// the whole reason this row can be read by the console, dumped in a snapshot
// and shown in an audit diff: a reference is public, a literal never is.
type ConfigRemote struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Provider is the forge picked when it was set up (confrepo.Providers),
	// "" for one set up before the choice existed.
	Provider string `json:"provider,omitempty"`
	URL      string `json:"url"`
	Branch   string `json:"branch"`
	// Dir is the directory inside the repository, "" for its root. It is what
	// lets several locations share one repository, one per customer platform.
	Dir      string `json:"dir"`
	TokenRef string `json:"tokenRef,omitempty"`
	// TokenUser is the HTTP basic username beside the token, "" for the forge's
	// default. A field rather than a constant because the forges disagree and
	// all of them report a wrong one as an authentication failure.
	TokenUser   string `json:"tokenUser,omitempty"`
	AuthorName  string `json:"authorName,omitempty"`
	AuthorEmail string `json:"authorEmail,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// ErrConfigRemoteNotFound is what a caller checks instead of comparing strings.
var ErrConfigRemoteNotFound = errors.New("store: git location not found")

// SanitizeConfigRemoteName trims and refuses the empty name: a location is
// picked out of a dialog by its name, so an unnamed one is unpickable.
func SanitizeConfigRemoteName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("a git location needs a name")
	}
	if len(trimmed) > 120 {
		return "", errors.New("a git location name is limited to 120 characters")
	}
	return trimmed, nil
}

// SaveConfigRemote inserts or updates one.
func (s *Store) SaveConfigRemote(ctx context.Context, r *ConfigRemote) error {
	name, err := SanitizeConfigRemoteName(r.Name)
	if err != nil {
		return fmt.Errorf("store: save git location: %w", err)
	}
	now := time.Now().Unix()
	created := r.CreatedAt
	if created <= 0 {
		created = now
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO config_remotes
		   (id, name, url, branch, dir, token_ref, token_user, author_name, author_email,
		    created_at, updated_at, provider)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, url = excluded.url, branch = excluded.branch,
		   dir = excluded.dir, token_ref = excluded.token_ref, token_user = excluded.token_user,
		   author_name = excluded.author_name, author_email = excluded.author_email,
		   updated_at = excluded.updated_at, provider = excluded.provider`,
		r.ID, name, r.URL, r.Branch, r.Dir, r.TokenRef, r.TokenUser,
		r.AuthorName, r.AuthorEmail, created, now, r.Provider)
	if err != nil {
		return fmt.Errorf("store: save git location %q: %w", name, err)
	}
	r.Name, r.CreatedAt, r.UpdatedAt = name, created, now
	return nil
}

const configRemoteColumns = `id, name, url, branch, dir, token_ref, token_user,
	author_name, author_email, created_at, updated_at, provider`

// ListConfigRemotes returns them by name.
func (s *Store) ListConfigRemotes(ctx context.Context) ([]ConfigRemote, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+configRemoteColumns+` FROM config_remotes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list git locations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []ConfigRemote{}
	for rows.Next() {
		var r ConfigRemote
		if err := rows.Scan(&r.ID, &r.Name, &r.URL, &r.Branch, &r.Dir, &r.TokenRef, &r.TokenUser,
			&r.AuthorName, &r.AuthorEmail, &r.CreatedAt, &r.UpdatedAt, &r.Provider); err != nil {
			return nil, fmt.Errorf("store: scan git location: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetConfigRemote returns one.
func (s *Store) GetConfigRemote(ctx context.Context, id string) (ConfigRemote, error) {
	var r ConfigRemote
	err := s.db.QueryRowContext(ctx,
		`SELECT `+configRemoteColumns+` FROM config_remotes WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.URL, &r.Branch, &r.Dir, &r.TokenRef, &r.TokenUser,
			&r.AuthorName, &r.AuthorEmail, &r.CreatedAt, &r.UpdatedAt, &r.Provider)
	if errors.Is(err, sql.ErrNoRows) {
		return ConfigRemote{}, ErrConfigRemoteNotFound
	}
	if err != nil {
		return ConfigRemote{}, fmt.Errorf("store: read git location: %w", err)
	}
	return r, nil
}

// DeleteConfigRemote removes one and UNBINDS whatever pointed at it, in one
// transaction.
//
// Leaving the bindings behind would leave rows claiming to come from a location
// nobody can name any more - and a push that resolves to nothing is worse than
// a configuration that admits it lives only here.
func (s *Store) DeleteConfigRemote(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: delete git location: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `DELETE FROM config_remotes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete git location: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrConfigRemoteNotFound
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE configurations SET remote_id = ?, remote_rev = ?, remote_at = ? WHERE remote_id = ?`,
		"", "", 0, id); err != nil {
		return fmt.Errorf("store: delete git location: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: delete git location: %w", err)
	}
	return nil
}

// ConfigRemoteNameTaken reports whether another location answers to that name.
func (s *Store) ConfigRemoteNameTaken(ctx context.Context, name, exceptID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM config_remotes WHERE name = ? AND id <> ?`, name, exceptID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("store: git location name: %w", err)
	}
	return n > 0, nil
}

// ConfigRemoteInUse names the configurations bound to a location, so that
// deleting it says what it would unbind rather than doing it silently.
func (s *Store) ConfigRemoteInUse(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM configurations WHERE remote_id = ? ORDER BY name`, id)
	if err != nil {
		return nil, fmt.Errorf("store: git location in use: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("store: git location in use: %w", err)
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// BindConfiguration points a saved configuration at a git location, or at none
// when remoteID is empty.
//
// It CLEARS the revision, always: a new destination has never been read, and a
// revision kept from the previous one would let the next push believe it knew
// where that branch stood.
func (s *Store) BindConfiguration(ctx context.Context, id, remoteID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE configurations SET remote_id = ?, remote_rev = ?, remote_at = ? WHERE id = ?`,
		remoteID, "", 0, id)
	if err != nil {
		return fmt.Errorf("store: bind configuration: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrConfigurationNotFound
	}
	return nil
}

// MarkConfigurationSynced records the revision a pull read or a push wrote.
//
// Separate from SaveConfiguration for the same reason the active flag is: a
// save is a document changing, a sync is a fact about a repository, and an
// ordinary save that moved the revision would make the next push believe the
// branch had not advanced.
func (s *Store) MarkConfigurationSynced(ctx context.Context, id, rev string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE configurations SET remote_rev = ?, remote_at = ? WHERE id = ?`,
		rev, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: record configuration revision: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrConfigurationNotFound
	}
	return nil
}

// ForgetConfigRemoteRevisions clears the recorded revision of every
// configuration bound to a location, without unbinding them.
//
// Called when a location is edited to point somewhere else: a revision is a
// fact about one branch in one directory, and carrying it across a change of
// either would let the next push believe it knew where that branch stood.
func (s *Store) ForgetConfigRemoteRevisions(ctx context.Context, remoteID string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE configurations SET remote_rev = ?, remote_at = ? WHERE remote_id = ?`,
		"", 0, remoteID); err != nil {
		return fmt.Errorf("store: forget revisions of git location: %w", err)
	}
	return nil
}
