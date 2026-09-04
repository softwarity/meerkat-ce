package store

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
)

// One-at-a-time, across every gateway sharing this database (STORE-03).
//
// Two things in this product must not be done twice at once, and both are
// invisible when they go wrong:
//
//   - ISSUING A CERTIFICATE. Two nodes armed with the same authority answer
//     the same first handshake by ordering the same name, and the authority
//     counts both. Let's Encrypt allows five duplicates a week: five nodes
//     restarting together spend the week's allowance in a second, and the
//     failure arrives days later as a name that will not renew.
//   - THE PERIODIC PURGES. Harmless to repeat, but N nodes scanning the audit
//     table every minute is N times the work for one row deleted.
//
// A lock is local first and shared second: the local mutex is what stops two
// goroutines of ONE node queueing on the database, and the advisory lock is
// what stops two nodes. On the embedded database the second half is skipped,
// because there is no second node - and the promise still holds, which is why
// the local half is not conditional.
//
// PostgreSQL advisory locks are held by a SESSION, so the queries below run on
// one RESERVED connection rather than through the pool - through db.go's
// execOn/queryRowOn, so they keep this package's single `?` spelling like
// every other query here.

// TryLock runs fn only if nobody else holds name, and says whether it ran.
//
// For work that will come round again: a purge skipped because another node is
// already doing it is not a failure, and queueing behind it would only mean
// doing it again a second later.
func (s *Store) TryLock(ctx context.Context, name string, fn func(context.Context) error) (bool, error) {
	mu := s.localLock(name)
	if !mu.TryLock() {
		return false, nil
	}
	defer mu.Unlock()
	if !s.Clustered() {
		return true, fn(ctx)
	}
	conn, release, err := s.lockSession(ctx, name, `SELECT pg_try_advisory_lock(?)`, true)
	if err != nil || conn == nil {
		return false, err
	}
	defer release()
	return true, fn(ctx)
}

// WithLock runs fn once it holds name, waiting for whoever has it.
//
// For work that must happen and must happen once: the node that waits then
// finds the certificate the other one just put in the shared cache, and asks
// the authority for nothing.
func (s *Store) WithLock(ctx context.Context, name string, fn func(context.Context) error) error {
	mu := s.localLock(name)
	mu.Lock()
	defer mu.Unlock()
	if !s.Clustered() {
		return fn(ctx)
	}
	_, release, err := s.lockSession(ctx, name, `SELECT pg_advisory_lock(?)`, false)
	if err != nil {
		return err
	}
	defer release()
	return fn(ctx)
}

// lockSession reserves a connection and takes the lock on it. It returns a nil
// connection - with no error - when a TRY did not get it.
func (s *Store) lockSession(ctx context.Context, name, query string, try bool) (*sql.Conn, func(), error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("store: lock %q: %w", name, err)
	}
	key := lockKey(name)
	got := true
	if try {
		if err := s.db.queryRowOn(ctx, conn, query, key).Scan(&got); err != nil {
			_ = conn.Close()
			return nil, nil, fmt.Errorf("store: lock %q: %w", name, err)
		}
	} else if _, err := s.db.execOn(ctx, conn, query, key); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("store: lock %q: %w", name, err)
	}
	if !got {
		_ = conn.Close()
		return nil, nil, nil
	}
	return conn, func() {
		// Unreleased on purpose even when ctx is already dead: Close only
		// RETURNS this connection to the pool, so a lock left on it would be
		// held by a session that goes on serving other queries - the one leak
		// that never repairs itself.
		free := context.WithoutCancel(ctx)
		if _, err := s.db.execOn(free, conn, `SELECT pg_advisory_unlock(?)`, key); err != nil {
			slog.Error("could not release a cluster lock: this connection must not go back to the pool",
				"lock", name, "err", err)
		}
		_ = conn.Close()
	}, nil
}

// localLock is the per-process half. Named locks are few and long-lived (one
// per certificate name, one for the purges), so they are kept rather than
// reference-counted.
func (s *Store) localLock(name string) *sync.Mutex {
	actual, _ := s.locks.LoadOrStore(name, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// lockKey turns a name into the bigint PostgreSQL wants. A collision costs two
// unrelated pieces of work waiting for each other, never correctness, and with
// a handful of names it is not going to happen.
func lockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64()) //nolint:gosec // the wrap is the point: any 64 bits will do
}
