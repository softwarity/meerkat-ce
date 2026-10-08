package store

import (
	"context"
	"fmt"
)

// SpendAssertion records that a SAML assertion has been used, and reports
// whether it was the first time (AUTH-19). An assertion is a bearer credential
// for its validity window: one captured in a browser's history, or a proxy's
// log, would otherwise open a session as often as it is posted. The id is the
// authority's own, prefixed with the provider by the caller so two authorities
// can never collide.
//
// until is when the assertion would expire anyway; the row lives that long.
func (s *Store) SpendAssertion(ctx context.Context, id string, until int64) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO saml_assertions (id, expires) VALUES (?, ?) ON CONFLICT (id) DO NOTHING`, id, until)
	if err != nil {
		return false, fmt.Errorf("store: record a SAML assertion: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// PurgeSpentAssertions drops the assertions past their expiry: nobody can post
// those again, the validity window having closed.
func (s *Store) PurgeSpentAssertions(ctx context.Context, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM saml_assertions WHERE expires < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("store: purge spent SAML assertions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
