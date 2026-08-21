package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Named configurations (CFG-01/02): several coexist, one is active.
//
// The store holds the document as OPAQUE bytes. Its shape lives in
// internal/config, which imports this package, so knowing it here would close
// the circle; and it costs nothing - nothing at this level ever asks what is
// inside, only which one is running and what it is called.

// Configuration is one saved configuration. Document is the portable file
// (YAML) exactly as an export would hand it over - which is what makes "export
// this one" a read rather than a re-serialisation, and guarantees the file an
// admin downloads is byte-for-byte the one that would be applied.
type Configuration struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Document is left EMPTY by List: a configuration carrying a logo weighs
	// hundreds of kilobytes, and a screen listing ten of them needs none of it.
	Document string `json:"document,omitempty"`
	// Digest fingerprints that document, and it DOES travel with the list - it
	// is what turns "this is saved" from a claim into a comparison. Save a
	// configuration, add a route, and what runs is no longer what was saved;
	// only two digests can say so.
	Digest    string `json:"digest,omitempty"`
	Active    bool   `json:"active"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// ErrConfigurationNotFound is what a caller checks instead of comparing error
// strings.
var ErrConfigurationNotFound = errors.New("store: configuration not found")

// DigestOf fingerprints a configuration document. Exported because the LIVE
// state has to be fingerprinted the same way to be compared with a saved one -
// two hashes computed by two functions are two answers waiting to disagree.
func DigestOf(document string) string {
	sum := sha256.Sum256([]byte(document))
	return hex.EncodeToString(sum[:])
}

// SanitizeConfigurationName trims and refuses the empty name. A configuration
// is chosen from a list by its name, so an unnamed one is one nobody can pick
// out of the three others.
func SanitizeConfigurationName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("a configuration needs a name")
	}
	if len(trimmed) > 120 {
		return "", errors.New("a configuration name is limited to 120 characters")
	}
	return trimmed, nil
}

// SaveConfiguration inserts or updates one. The active flag is NOT written
// here: activating is its own act, with its own audit line, and letting an
// ordinary save carry it would make "who switched production" a question with
// several possible answers.
// The stamps are written BACK into c: the console lists these by their dates,
// and a create that answered createdAt 0 would sit at the bottom of the list
// until the next reload.
func (s *Store) SaveConfiguration(ctx context.Context, c *Configuration) error {
	name, err := SanitizeConfigurationName(c.Name)
	if err != nil {
		return fmt.Errorf("store: save configuration: %w", err)
	}
	now := time.Now().Unix()
	created := c.CreatedAt
	if created <= 0 {
		created = now
	}
	digest := DigestOf(c.Document)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO configurations (id, name, description, document, digest, active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, description = excluded.description,
		   document = excluded.document, digest = excluded.digest,
		   updated_at = excluded.updated_at`,
		c.ID, name, c.Description, c.Document, digest, created, now)
	if err != nil {
		return fmt.Errorf("store: save configuration %q: %w", name, err)
	}
	c.Name, c.Digest, c.CreatedAt, c.UpdatedAt = name, digest, created, now
	return nil
}

// ListConfigurations returns the collection WITHOUT the documents, newest
// first among equals - the list a screen draws.
func (s *Store) ListConfigurations(ctx context.Context) ([]Configuration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, digest, active, created_at, updated_at
		   FROM configurations ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list configurations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Configuration{}
	for rows.Next() {
		var c Configuration
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Digest, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan configuration: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetConfiguration returns one, document included.
func (s *Store) GetConfiguration(ctx context.Context, id string) (Configuration, error) {
	return s.configurationRow(ctx,
		`SELECT id, name, description, document, digest, active, created_at, updated_at
		   FROM configurations WHERE id = ?`, id)
}

// ActiveConfiguration returns the running one. ErrConfigurationNotFound means
// this installation has never activated any - a legitimate state: the gateway
// serves what is in its database, named or not.
func (s *Store) ActiveConfiguration(ctx context.Context) (Configuration, error) {
	return s.configurationRow(ctx,
		`SELECT id, name, description, document, digest, active, created_at, updated_at
		   FROM configurations WHERE active = 1`)
}

func (s *Store) configurationRow(ctx context.Context, query string, args ...any) (Configuration, error) {
	var c Configuration
	err := s.db.QueryRowContext(ctx, query, args...).
		Scan(&c.ID, &c.Name, &c.Description, &c.Document, &c.Digest, &c.Active, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Configuration{}, ErrConfigurationNotFound
	}
	if err != nil {
		return Configuration{}, fmt.Errorf("store: read configuration: %w", err)
	}
	return c, nil
}

// MarkConfigurationActive moves the mark, in ONE transaction: the flag says
// what is running, and a window where nothing (or everything) claims to be is
// a window where a rollback has no destination.
//
// It records the mark only. Applying the document is the caller's business -
// the store has no idea what one contains.
func (s *Store) MarkConfigurationActive(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: activate configuration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE configurations SET active = 0 WHERE active = 1`); err != nil {
		return fmt.Errorf("store: activate configuration: %w", err)
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE configurations SET active = 1, updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: activate configuration: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrConfigurationNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: activate configuration: %w", err)
	}
	return nil
}

// DeleteConfiguration removes one. Deleting the ACTIVE one is allowed and
// changes nothing about what the gateway serves: a configuration is a saved
// copy, not the running state. What it costs is the name of what is running -
// hence the refusal is a question for the console, not a rule here.
func (s *Store) DeleteConfiguration(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM configurations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete configuration: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrConfigurationNotFound
	}
	return nil
}

// ConfigurationNameTaken reports whether another configuration already answers
// to that name, so a rename or a duplicate is refused with a sentence rather
// than a UNIQUE constraint message.
func (s *Store) ConfigurationNameTaken(ctx context.Context, name, exceptID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM configurations WHERE name = ? AND id <> ?`, name, exceptID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("store: configuration name: %w", err)
	}
	return n > 0, nil
}
