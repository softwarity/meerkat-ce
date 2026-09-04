package store

import (
	"context"
	"fmt"
	"time"
)

// The brute-force counter, shared between the gateways (AUTH-11, SEC-10).
//
// It used to be a map on the handler, which had two holes and only one of them
// was about clusters:
//
//   - N gateways meant N counters, so a limit of five attempts was five per
//     node. An attacker spraying a load balancer got the limit multiplied by
//     the very thing that was supposed to make the service sturdier.
//   - a restart forgot everything. Whoever was being throttled was let back in
//     by a deployment, and nobody would ever connect the two.
//
// One row per failed attempt is a lot of rows for something that could be a
// number - and it is still the right shape, because the window SLIDES: a
// counter with a reset time answers "five in the last fifteen minutes" wrongly
// at every boundary, and the rows answer it exactly. They are also cheap where
// it matters: a row is written when a sign-in FAILS, which on a healthy
// installation is rare, and read once per attempt.

// RecordAttempt notes one failed attempt against key.
func (s *Store) RecordAttempt(ctx context.Context, key string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO login_attempts (key, at) VALUES (?, ?)`, key, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("store: record attempt: %w", err)
	}
	return nil
}

// CountAttempts is how many failures key has collected since a moment.
func (s *Store) CountAttempts(ctx context.Context, key string, since time.Time) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM login_attempts WHERE key = ? AND at > ?`,
		key, since.UnixMilli()).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count attempts: %w", err)
	}
	return n, nil
}

// ClearAttempts forgives key: a sign-in succeeded.
func (s *Store) ClearAttempts(ctx context.Context, key string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM login_attempts WHERE key = ?`, key); err != nil {
		return fmt.Errorf("store: clear attempts: %w", err)
	}
	return nil
}

// PurgeAttemptsBefore drops what no window can still see. Called from the
// periodic upkeep, under the cluster lock like the rest of it.
func (s *Store) PurgeAttemptsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM login_attempts WHERE at <= ?`, cutoff.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("store: purge attempts: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
