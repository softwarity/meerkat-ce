package store

import "errors"

// The pause: the gateway stops writing, everywhere at once.
//
// What it is for is moving the database (Configuration > Snapshot): a copy
// taken while writes go on loses whatever is written between the copy and the
// switch - sessions, the audit trail, the calls of audited operations. Paused,
// the source holds still, and the copy is the whole of it.
//
// Refused HERE, at the one door every write goes through, and not screen by
// screen: half of what writes is no screen at all - the minute's upkeep, the
// scheduler, an ACME renewal, the writer of the audited calls.
//
// In memory, never stored, and that is the point: the gateway that restarts on
// the other database must be working at once, not paused by a row it brought
// with it. A gateway that restarts on the SAME database comes back unpaused
// too, which is what a restart means.

// ErrPaused is what a write gets while the gateway is paused.
var ErrPaused = errors.New("the gateway is paused: nothing is written until it is resumed, or restarted on another database")

// Pause stops or resumes every write.
func (s *Store) Pause(on bool) { s.db.paused.Store(on) }

// Paused reports whether writes are stopped.
func (s *Store) Paused() bool { return s.db.paused.Load() }
