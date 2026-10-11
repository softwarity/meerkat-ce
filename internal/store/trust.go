package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TrustedAuthority is one authority the gateway trusts when it calls an
// upstream in HTTPS (SSL-09): its PEM, one certificate or several.
type TrustedAuthority struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PEM       string `json:"pem"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// ErrTrustedAuthorityNotFound is what a caller checks instead of a string.
var ErrTrustedAuthorityNotFound = errors.New("store: trusted authority not found")

// CheckTrustedAuthority refuses what cannot be one, naming what is allowed.
// Whether the PEM actually holds certificates is the reader's to say
// (certs.ParseAuthorities): this layer stores, it does not read X.509.
func CheckTrustedAuthority(a *TrustedAuthority) error {
	a.Name = strings.TrimSpace(a.Name)
	a.PEM = strings.TrimSpace(a.PEM)
	switch {
	case a.Name == "":
		return errors.New("a trusted authority needs a name")
	case len(a.Name) > 120:
		return errors.New("a trusted authority's name is limited to 120 characters")
	case a.PEM == "":
		return errors.New("a trusted authority needs its certificate, as PEM")
	case len(a.PEM) > 1<<20:
		return errors.New("the PEM is limited to 1 MB: a bundle of authorities, not a database")
	}
	return nil
}

// SaveTrustedAuthority inserts or updates one.
func (s *Store) SaveTrustedAuthority(ctx context.Context, a *TrustedAuthority) error {
	if err := CheckTrustedAuthority(a); err != nil {
		return err
	}
	now := time.Now().Unix()
	if a.CreatedAt <= 0 {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO trusted_authorities (id, name, pem, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, pem = excluded.pem, updated_at = excluded.updated_at`,
		a.ID, a.Name, a.PEM, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("a trusted authority is already named %q", a.Name)
		}
		return fmt.Errorf("store: save trusted authority %q: %w", a.Name, err)
	}
	return nil
}

// ListTrustedAuthorities returns them by name.
func (s *Store) ListTrustedAuthorities(ctx context.Context) ([]TrustedAuthority, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, pem, created_at, updated_at FROM trusted_authorities ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list trusted authorities: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []TrustedAuthority{}
	for rows.Next() {
		var a TrustedAuthority
		if err := rows.Scan(&a.ID, &a.Name, &a.PEM, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan trusted authority: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetTrustedAuthority returns one.
func (s *Store) GetTrustedAuthority(ctx context.Context, id string) (TrustedAuthority, error) {
	var a TrustedAuthority
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, pem, created_at, updated_at FROM trusted_authorities WHERE id = ?`, id).
		Scan(&a.ID, &a.Name, &a.PEM, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedAuthority{}, ErrTrustedAuthorityNotFound
	}
	if err != nil {
		return TrustedAuthority{}, fmt.Errorf("store: read trusted authority: %w", err)
	}
	return a, nil
}

// DeleteTrustedAuthority removes one.
func (s *Store) DeleteTrustedAuthority(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM trusted_authorities WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete trusted authority: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrTrustedAuthorityNotFound
	}
	return nil
}
