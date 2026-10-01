package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The revision a row is on, and what it is for.
//
// Every object somebody EDITS carries one. A read says which revision it is,
// a write says which one it was built on, and a write built on a version the
// row has moved past is refused instead of winning silently. Without it the
// last writer wins with whatever copy it happens to hold: a console screen
// opened five minutes ago, an agent that read the object a minute ago, an API
// call in between - and the work of the other two is gone with nothing to say
// so. That is not hypothetical; it cost an evening's endpoint rules here.
//
// ZERO MEANS "I READ NO VERSION", and such a write still wins. It is what a
// seed, a configuration import, or a server-side read-modify-write is honestly
// saying: they did not hold a copy somebody else could have replaced.
//
// It guards HUMAN-speed races, which is what loses work. Two writes landing in
// the same millisecond are not what this is for - and a row is never corrupted
// by them, only the later of two intentions is.

// ErrStale is what a write built on a replaced version comes back as. A
// CONFLICT, not a failure: nothing is wrong with the request except that the
// world moved under it, and the answer is to read again and redo the change.
var ErrStale = errors.New("stale write")

// checkRev refuses a write whose revision is not the one the row is on.
//
// The table name comes from this package's own call sites and never from a
// request: it is a constant in the line above each call, which is what keeps
// this one string interpolation honest.
func (s *Store) checkRev(ctx context.Context, table, what, id string, rev int64) error {
	if rev == 0 {
		return nil
	}
	var current int64
	err := s.db.QueryRowContext(ctx, `SELECT rev FROM `+table+` WHERE id = ?`, id).Scan(&current)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Gone since it was read. Writing it back would resurrect what somebody
		// deleted, which is the same surprise seen from the other side.
		return fmt.Errorf("%w: this %s no longer exists - it was deleted since you read it", ErrStale, what)
	case err != nil:
		return fmt.Errorf("store: %s %q: %w", what, id, err)
	case current != rev:
		return fmt.Errorf("%w: this %s changed since you read it (you have revision %d, it is now at %d) - read it again and apply your change to that",
			ErrStale, what, rev, current)
	}
	return nil
}
