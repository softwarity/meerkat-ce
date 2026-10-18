package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// The sessions of the developer tunnel (DEV-13), as the replicas agree on them.
//
// A plug run opens several connections, and with several replicas behind one
// Service they land wherever the Service sends them: each replica's agent sees
// its own part of the run and reports it as a session of its own. What a
// person reads - the trail, the console - is one run. So each replica HOLDS the
// session while it has a part of it, and the row says it once: opened by the
// first hold, ended by the last release.

// PlugSession is one run.
type PlugSession struct {
	ID      string
	Who     string // the account the key belongs to
	Key     string // the key's SHA256 fingerprint
	From    string // the address the run's first connection came from
	Version string // the client's plug version
	Since   int64
	Ended   int64
	// End is how it ended (ended, dropped, closed, shutdown). A replica that
	// reports "closed" wins over the others: the session ended because somebody
	// ended it, whatever the last replica to let go saw.
	End    string
	Reason string
}

// End kinds, as the agent names them.
const (
	PlugEndClosed   = "closed"
	PlugEndShutdown = "shutdown"
	PlugEndDropped  = "dropped"
)

// HoldPlugSession records that node holds part of s, and reports whether this
// is the session's first hold anywhere - the one that says it opened.
func (s *Store) HoldPlugSession(ctx context.Context, ps PlugSession, node string) (first bool, err error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO plug_sessions (id, who, key_fp, from_addr, version, since) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO NOTHING`,
		ps.ID, ps.Who, ps.Key, ps.From, ps.Version, ps.Since)
	if err != nil {
		return false, fmt.Errorf("store: hold plug session: %w", err)
	}
	n, _ := res.RowsAffected()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO plug_session_holds (session_id, node, held_at) VALUES (?, ?, ?)
		 ON CONFLICT(session_id, node) DO NOTHING`, ps.ID, node, time.Now().Unix()); err != nil {
		return false, fmt.Errorf("store: hold plug session: %w", err)
	}
	return n == 1, nil
}

// ReleasePlugSession records that node no longer holds the session. When it
// was the last hold, the session ends and is returned with last true - exactly
// once, whichever replicas let go together.
func (s *Store) ReleasePlugSession(ctx context.Context, id, node, end, reason string) (PlugSession, bool, error) {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM plug_session_holds WHERE session_id = ? AND node = ?`, id, node); err != nil {
		return PlugSession{}, false, fmt.Errorf("store: release plug session: %w", err)
	}
	if end == PlugEndClosed {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE plug_sessions SET end_kind = ?, reason = ? WHERE id = ? AND end_kind <> ?`,
			end, reason, id, PlugEndClosed); err != nil {
			return PlugSession{}, false, fmt.Errorf("store: release plug session: %w", err)
		}
	}
	return s.endIfUnheld(ctx, id, end, reason)
}

// endIfUnheld ends a session nobody holds any more, keeping a "closed" already
// recorded, and returns it when this call is the one that ended it.
func (s *Store) endIfUnheld(ctx context.Context, id, end, reason string) (PlugSession, bool, error) {
	var held int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM plug_session_holds WHERE session_id = ?`, id).Scan(&held); err != nil {
		return PlugSession{}, false, fmt.Errorf("store: release plug session: %w", err)
	}
	if held > 0 {
		return PlugSession{}, false, nil
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE plug_sessions SET ended_at = ?,
		   reason = CASE WHEN end_kind = ? THEN reason ELSE ? END,
		   end_kind = CASE WHEN end_kind = ? THEN end_kind ELSE ? END
		 WHERE id = ? AND ended_at = 0`,
		time.Now().Unix(), PlugEndClosed, reason, PlugEndClosed, end, id)
	if err != nil {
		return PlugSession{}, false, fmt.Errorf("store: end plug session: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return PlugSession{}, false, nil
	}
	ps, err := s.plugSession(ctx, id)
	return ps, err == nil, err
}

func (s *Store) plugSession(ctx context.Context, id string) (PlugSession, error) {
	var ps PlugSession
	err := s.db.QueryRowContext(ctx,
		`SELECT id, who, key_fp, from_addr, version, since, ended_at, end_kind, reason
		 FROM plug_sessions WHERE id = ?`, id).
		Scan(&ps.ID, &ps.Who, &ps.Key, &ps.From, &ps.Version, &ps.Since, &ps.Ended, &ps.End, &ps.Reason)
	if err != nil {
		return ps, fmt.Errorf("store: plug session %s: %w", id, err)
	}
	return ps, nil
}

// ReleasePlugHolds lets go of every hold of the nodes no longer alive - a
// replica that died, or this one coming back under the same name - provided
// they are older than before, so a replica whose first report is still on its
// way is not taken for dead. The sessions this ends are returned, each once,
// ended as end.
func (s *Store) ReleasePlugHolds(ctx context.Context, alive []string, before int64, end string) ([]PlugSession, error) {
	q := `SELECT DISTINCT session_id, node FROM plug_session_holds WHERE held_at < ?`
	args := []any{before}
	if len(alive) > 0 {
		q += ` AND node NOT IN (` + strings.TrimSuffix(strings.Repeat("?,", len(alive)), ",") + `)`
		for _, n := range alive {
			args = append(args, n)
		}
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: stale plug holds: %w", err)
	}
	type hold struct{ id, node string }
	var stale []hold
	for rows.Next() {
		var h hold
		if err := rows.Scan(&h.id, &h.node); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("store: stale plug holds: %w", err)
		}
		stale = append(stale, h)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: stale plug holds: %w", err)
	}
	var ended []PlugSession
	for _, h := range stale {
		ps, last, err := s.ReleasePlugSession(ctx, h.id, h.node, end, "")
		if err != nil {
			return ended, err
		}
		if last {
			ended = append(ended, ps)
		}
	}
	return ended, nil
}

// ReleaseNodePlugHolds lets go of everything node holds: what a replica does as
// its tunnel starts, since a tunnel that starts holds nothing yet.
func (s *Store) ReleaseNodePlugHolds(ctx context.Context, node, end string) ([]PlugSession, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT session_id FROM plug_session_holds WHERE node = ?`, node)
	if err != nil {
		return nil, fmt.Errorf("store: node plug holds: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("store: node plug holds: %w", err)
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	var ended []PlugSession
	for _, id := range ids {
		ps, last, err := s.ReleasePlugSession(ctx, id, node, end, "")
		if err != nil {
			return ended, err
		}
		if last {
			ended = append(ended, ps)
		}
	}
	return ended, rows.Err()
}

// PurgePlugSessions drops the sessions ended before before: the trail keeps
// their story, and the table only has to outlive the replicas' last reports.
func (s *Store) PurgePlugSessions(ctx context.Context, before int64) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM plug_sessions WHERE ended_at > 0 AND ended_at < ?`, before); err != nil {
		return fmt.Errorf("store: purge plug sessions: %w", err)
	}
	return nil
}
