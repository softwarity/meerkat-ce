package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
)

// The change bus, database side (STORE-03).
//
// The problem it answers: a node that takes a write reloads what it caches -
// its routes, its certificates - and the other nodes do not, because nothing
// told them. They keep serving the previous plan until someone restarts them,
// which is the worst kind of bug: intermittent, dependent on which node the
// load balancer picked, and invisible on the node the operator is looking at.
//
// Two halves, deliberately:
//
//   - a TABLE (change_marks) holding one version per topic. This is the truth,
//     and it is what a node compares against. It works on both databases.
//   - a NOTIFY carrying the topic, which is only a HINT that the table moved.
//     PostgreSQL only, and lossy by nature: nobody is listening during a
//     restart, and a listening connection can die quietly. Losing one costs
//     latency, never correctness, because the table is still there.
//
// The embedded database has neither and needs neither: one process owns that
// file, so there is nobody to tell.

// The topics a node re-reads. Closed on purpose: a topic is something a node
// KEEPS IN MEMORY and would otherwise serve stale, and there are two of those.
// Anything read from the database on every request needs no bus.
const (
	// TopicRouting is the compiled route plan: routes, their deposited specs,
	// the vault values their references expand to, the application locales.
	TopicRouting = "routing"
	// TopicCertificates is the material the HTTPS listeners hold open.
	TopicCertificates = "certificates"
)

// The signals, which are the other kind of message on the same channel.
//
// A TOPIC says "something moved, read the table". A SIGNAL carries the thing
// itself - a token hash, a user id - and has NO table behind it. That is a
// deliberate weakening, and it is only allowed where losing a message costs
// exactly what is already accepted: these two drop an entry from a five-second
// memory cache, so a lost one means the other node notices five seconds later,
// which is what happens on a single gateway anyway.
//
// What travels is the token HASH, never the token. It is the form already
// stored in the sessions table, so anyone who can read the notification could
// read the row.
const (
	// TopicSession drops one session from the caches, by token hash: the
	// tenant or the login step it carries has just changed on another node.
	TopicSession = "session"
	// TopicSessionUser drops every session of one user, by user id: a password
	// reset revoked them all.
	TopicSessionUser = "session-user"
	// TopicEvent carries a live-channel message to the pages the OTHER nodes
	// hold open (internal/events). The argument is the hub topic, a space, and
	// the encoded message.
	TopicEvent = "event"
)

// MaxSignalBytes is what a notification may carry. PostgreSQL refuses a
// payload over 8000 bytes, and refusing it here - with a name for what was too
// big - beats an error from the driver on a path nobody is watching.
const MaxSignalBytes = 7000

// changeChannel is the PostgreSQL channel every node listens on. One channel
// for every topic: the payload says which, and a node with nothing registered
// for it does nothing. Channels are per-database, not per-schema.
const changeChannel = "meerkat_changes"

// ErrNotClustered says there is nothing to listen to: the embedded database is
// owned by one process, so a change is already visible to everyone who can see
// it. Callers treat it as "stop, quietly", not as a failure.
var ErrNotClustered = errors.New("store: the embedded database has one process, and nothing to tell it")

// Clustered reports whether other nodes can be reading this database.
func (s *Store) Clustered() bool { return s.db.dialect == dialectPostgres }

// MarkChanged bumps a topic's version and returns it.
//
// Called after the write it describes, in the same request: the version that
// comes back is what the caller has already applied locally, which is how its
// own announcement does not make it reload a second time.
func (s *Store) MarkChanged(ctx context.Context, topic string) (int64, error) {
	var v int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO change_marks (topic, version, at) VALUES (?, 1, ?)
		 ON CONFLICT (topic) DO UPDATE SET version = change_marks.version + 1, at = excluded.at
		 RETURNING version`, topic, time.Now().Unix()).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("store: mark %q changed: %w", topic, err)
	}
	return v, nil
}

// ChangeMarks reads every topic's current version.
//
// A topic that has never changed is absent rather than zero, which is the same
// thing to a reader comparing against what it last acted on.
func (s *Store) ChangeMarks(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT topic, version FROM change_marks`)
	if err != nil {
		return nil, fmt.Errorf("store: read change marks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int64{}
	for rows.Next() {
		var topic string
		var v int64
		if err := rows.Scan(&topic, &v); err != nil {
			return nil, err
		}
		out[topic] = v
	}
	return out, rows.Err()
}

// Signal sends a message carrying its own argument. See the signal constants
// above for when that is allowed instead of a topic.
func (s *Store) Signal(ctx context.Context, topic, arg string) error {
	if len(topic)+len(arg) > MaxSignalBytes {
		return fmt.Errorf("store: signal %q is %d bytes, over the %d a notification carries",
			topic, len(topic)+len(arg), MaxSignalBytes)
	}
	return s.Announce(ctx, topic+" "+arg)
}

// Announce tells the other nodes to look at a topic. Best effort by design:
// see the note at the top of this file.
func (s *Store) Announce(ctx context.Context, topic string) error {
	if !s.Clustered() {
		return nil
	}
	// pg_notify rather than the NOTIFY statement: the payload of a statement
	// cannot be a placeholder, and building it by concatenation is how a topic
	// name would one day carry a quote.
	if _, err := s.db.ExecContext(ctx, `SELECT pg_notify(?, ?)`, changeChannel, topic); err != nil {
		return fmt.Errorf("store: announce %q: %w", topic, err)
	}
	return nil
}

// Listen blocks, calling on for every announcement until ctx ends or the
// connection breaks - and returning the error either way, because a listener
// that dies silently is exactly the failure this whole file is about.
//
// on is called ONCE with an empty topic as soon as the connection is
// registered, and that call is not a formality: it is the moment from which
// nothing can be missed, so it is where the caller looks at what moved while
// it was not listening. Catching up BEFORE registering would leave a window -
// the width of one round trip - in which an announcement lands on nobody and
// waits for the slow timer.
//
// It holds one connection for its whole life. That is what LISTEN is: the
// notifications arrive on the session that registered, so it cannot be
// returned to the pool between two of them.
func (s *Store) Listen(ctx context.Context, on func(topic string)) error {
	if !s.Clustered() {
		return ErrNotClustered
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: listen: %w", err)
	}
	defer func() { _ = conn.Close() }()
	return conn.Raw(func(dc any) error {
		pc, ok := dc.(*stdlib.Conn)
		if !ok {
			return ErrNotClustered
		}
		raw := pc.Conn()
		if _, err := raw.Exec(ctx, `LISTEN `+changeChannel); err != nil {
			return fmt.Errorf("store: listen: %w", err)
		}
		on("")
		for {
			n, err := raw.WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("store: listening: %w", err)
			}
			on(n.Payload)
		}
	})
}
