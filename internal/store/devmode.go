package store

import (
	"context"
	"fmt"
	"time"
)

// DevAllowed answers the question every developer gate asks: may THIS user use
// the developer tooling here (DEV-01)? Root always may; any other account when
// it is enabled and carries the dev capability. Having one place to ask is what
// stops the next dev entry point from checking something else.
//
// There used to be an installation-wide switch above the capability (the
// dev_mode setting). It said the same thing a second time, in a second place
// to look when a developer lost their tools: a production gateway closes its
// developer surface by having no account carry the flag, which
// ClearDevCapability does in one act.
func (s *Store) DevAllowed(_ context.Context, u User) bool {
	return u.Enabled && (u.Root || u.Dev)
}

// ClearDevCapability takes the dev capability from every account but root's,
// and says how many it changed. Root keeps its tooling whatever it carries.
func (s *Store) ClearDevCapability(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET dev = ?, updated_at = ? WHERE dev = ? AND root = ?`,
		false, time.Now().Unix(), true, false)
	if err != nil {
		return 0, fmt.Errorf("store: take the dev capability from every account: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
