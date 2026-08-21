package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// The configuration's tape (CFG-06): a restore point per change, kept.
//
// Nobody asks for a point and nobody names one - that is the whole difference
// with a saved configuration. A point has an hour, an author and the sentence
// the audit trail wrote at the same second; it exists so that going back is
// possible, not so that anything is filed.

// The tape is NOT bounded, and it is the same in both editions.
//
// Going back is a safety function, not a comfort: an installation that can
// undo nothing is more dangerous for everybody, and shortening the free
// image's memory would not sell the paid one - it would only make the product
// riskier where it is most used. Worse, pruning happens on write, so a cap
// that differed by edition would DESTROY history the day an installation came
// down from Enterprise. The perpetual-fallback model refuses that.
//
// The cost is small enough to say out loud: a point is a few kilobytes (the
// document without its pictures), and a run of edits by one person folds into
// one. A busy month is a megabyte.

// CoalesceWindow folds a run of changes by the SAME person into one point.
// Dragging a colour picker writes twenty times in two minutes; twenty points
// for one decision make the tape unreadable, and going back into the middle of
// a gesture is not something anyone asks for. The price is stated: within the
// window, only the latest state is kept.
const CoalesceWindow = 2 * time.Minute

// ConfigPoint is one state of the configuration, as it was at a moment.
type ConfigPoint struct {
	ID string `json:"id"`
	At int64  `json:"at"`
	// ActorID survives the actor's deletion (no FK); ActorName is joined on read.
	ActorID   string `json:"actorId,omitempty"`
	ActorName string `json:"actorName,omitempty"`
	// Label is what the audit trail called the change that produced this state
	// ("route demo updated") - one vocabulary for both, so a point and its
	// audit line never tell two stories.
	Label  string `json:"label,omitempty"`
	Digest string `json:"digest"`
	// Document is left EMPTY by List, like a configuration's.
	Document string `json:"document,omitempty"`
}

// ErrNoConfigPoint means the tape is empty (a gateway that has never been
// changed), which is a legitimate state and not an error to shout about.
var ErrNoConfigPoint = errors.New("store: no restore point")

// LastConfigPoint returns the most recent point, document included - the one
// every new change is compared against.
func (s *Store) LastConfigPoint(ctx context.Context) (ConfigPoint, error) {
	var p ConfigPoint
	err := s.db.QueryRowContext(ctx,
		`SELECT id, at, actor_id, label, digest, document
		   FROM config_points ORDER BY at DESC, rowid DESC LIMIT 1`).
		Scan(&p.ID, &p.At, &p.ActorID, &p.Label, &p.Digest, &p.Document)
	if errors.Is(err, sql.ErrNoRows) {
		return ConfigPoint{}, ErrNoConfigPoint
	}
	if err != nil {
		return ConfigPoint{}, fmt.Errorf("store: read restore point: %w", err)
	}
	return p, nil
}

// AddConfigPoint writes a point.
//
// Within CoalesceWindow, a point by the SAME actor is REPLACED rather than
// added: the run of changes becomes one state, the latest.
func (s *Store) AddConfigPoint(ctx context.Context, p ConfigPoint) error {
	if p.At == 0 {
		p.At = time.Now().Unix()
	}
	last, err := s.LastConfigPoint(ctx)
	// Folding needs the new point to be NEWER than the last one as well as
	// close to it: a point stamped in the past (a tape being staged, a clock
	// that moved) would otherwise satisfy "within two minutes" by being an
	// hour early, and quietly overwrite the present.
	within := p.At >= last.At && p.At-last.At < int64(CoalesceWindow/time.Second)
	// And a state the tape ALREADY KNOWS is never folded, whoever made it and
	// however fast: landing on a state seen before is what going back IS, and
	// folding it would replace the very point being undone - the tape would
	// lose the state someone just rolled away from and show two identical
	// lines where a change used to be. Folding is for a run of small edits
	// heading somewhere, not for a return.
	known, err2 := s.configPointExists(ctx, p.Digest)
	if err2 != nil {
		return err2
	}
	switch {
	case err == nil && !known && last.ActorID == p.ActorID && within:
		_, err = s.db.ExecContext(ctx,
			`UPDATE config_points SET at = ?, label = ?, digest = ?, document = ? WHERE id = ?`,
			p.At, p.Label, p.Digest, p.Document, last.ID)
		if err != nil {
			return fmt.Errorf("store: fold restore point: %w", err)
		}
		return nil
	case err != nil && !errors.Is(err, ErrNoConfigPoint):
		return err
	}
	if p.ID == "" {
		p.ID = newAuditID()
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO config_points (id, at, actor_id, label, digest, document)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.At, p.ActorID, p.Label, p.Digest, p.Document); err != nil {
		return fmt.Errorf("store: write restore point: %w", err)
	}
	return nil
}

// configPointExists reports whether the tape already holds that exact state.
func (s *Store) configPointExists(ctx context.Context, digest string) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM config_points WHERE digest = ?`, digest).Scan(&n); err != nil {
		return false, fmt.Errorf("store: read restore points: %w", err)
	}
	return n > 0, nil
}

// ListConfigPoints returns the tape, newest first, WITHOUT the documents.
func (s *Store) ListConfigPoints(ctx context.Context) ([]ConfigPoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.at, p.actor_id, COALESCE(u.username, ''), p.label, p.digest
		   FROM config_points p LEFT JOIN users u ON u.id = p.actor_id
		  ORDER BY p.at DESC, p.rowid DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list restore points: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []ConfigPoint{}
	for rows.Next() {
		var p ConfigPoint
		if err := rows.Scan(&p.ID, &p.At, &p.ActorID, &p.ActorName, &p.Label, &p.Digest); err != nil {
			return nil, fmt.Errorf("store: scan restore point: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetConfigPoint returns one point, document included.
func (s *Store) GetConfigPoint(ctx context.Context, id string) (ConfigPoint, error) {
	var p ConfigPoint
	err := s.db.QueryRowContext(ctx,
		`SELECT p.id, p.at, p.actor_id, COALESCE(u.username, ''), p.label, p.digest, p.document
		   FROM config_points p LEFT JOIN users u ON u.id = p.actor_id
		  WHERE p.id = ?`, id).
		Scan(&p.ID, &p.At, &p.ActorID, &p.ActorName, &p.Label, &p.Digest, &p.Document)
	if errors.Is(err, sql.ErrNoRows) {
		return ConfigPoint{}, ErrNoConfigPoint
	}
	if err != nil {
		return ConfigPoint{}, fmt.Errorf("store: read restore point: %w", err)
	}
	return p, nil
}
