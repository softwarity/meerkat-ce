package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The calls of audited operations (AUD-04), kept in the trail.
//
// They used to leave for the collector and nowhere else, so an installation
// that had ticked an operation in Endpoint audit and not switched the export on
// had a trail that looked kept and was not. They are written here now, beside
// the changes and the sign-ins, and still go to the collector when it is on.
//
// Written OFF the request: the gateway hands a call over and goes on serving,
// and a writer puts them in the table a batch at a time. A queue that is full
// drops the call rather than holding the request - the trail is kept for the
// traffic, never at its expense - and counts what it dropped, which the
// Endpoint audit screen says.
//
// Their own lifetime: the only kind of the trail whose volume follows the
// traffic, so it is not allowed to impose that volume on the changes, which
// are kept a year.

const (
	callQueue = 4096                   // calls waiting to be written
	callBatch = 200                    // calls written in one statement
	callEvery = 500 * time.Millisecond // how long a short batch waits
)

type callWriter struct {
	once    sync.Once
	queue   chan AuditEvent
	quit    chan struct{}
	done    chan struct{}
	dropped atomic.Int64
	closing atomic.Bool
}

// RecordEndpointCall queues one audited call. Never blocks: a full queue drops
// it and counts it (EndpointCallsDropped).
func (s *Store) RecordEndpointCall(ev AuditEvent) {
	w := &s.calls
	if w.closing.Load() {
		return
	}
	w.once.Do(func() {
		w.queue = make(chan AuditEvent, callQueue)
		w.quit = make(chan struct{})
		w.done = make(chan struct{})
		go s.writeCalls(w)
	})
	ev.Target = AuditTargetEndpoint
	if ev.ID == "" {
		ev.ID = NewEventID()
	}
	if ev.At == 0 {
		ev.At = time.Now().Unix()
	}
	select {
	case w.queue <- ev:
	default:
		w.dropped.Add(1)
	}
}

// EndpointCallsDropped is how many calls a full queue has dropped since start.
func (s *Store) EndpointCallsDropped() int64 { return s.calls.dropped.Load() }

func (s *Store) writeCalls(w *callWriter) {
	defer close(w.done)
	batch := make([]AuditEvent, 0, callBatch)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.insertCalls(context.Background(), batch); err != nil {
			slog.Warn("could not write the audited calls", "count", len(batch), "err", err)
		}
		batch = batch[:0]
	}
	tick := time.NewTicker(callEvery)
	defer tick.Stop()
	for {
		select {
		case ev := <-w.queue:
			batch = append(batch, ev)
			if len(batch) == callBatch {
				flush()
			}
		case <-tick.C:
			flush()
		case <-w.quit:
			// What is already queued is written; the queue itself is never
			// closed, so a call handed over while stopping is dropped rather
			// than sent on a closed channel.
			for {
				select {
				case ev := <-w.queue:
					batch = append(batch, ev)
					if len(batch) == callBatch {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

// stop writes what is queued and ends the writer. A no-op when it never ran.
func (w *callWriter) stop() {
	if !w.closing.CompareAndSwap(false, true) {
		return
	}
	// Through the same Once that starts it: a writer that never ran is never
	// started now, and one that did is seen whole.
	started := true
	w.once.Do(func() { started = false })
	if !started {
		return
	}
	close(w.quit)
	select {
	case <-w.done:
	case <-time.After(5 * time.Second):
	}
}

func (s *Store) insertCalls(ctx context.Context, evs []AuditEvent) error {
	const cols = 11
	rows := make([]string, len(evs))
	args := make([]any, 0, len(evs)*cols)
	for i, ev := range evs {
		data := ""
		if ev.Data != nil {
			b, err := json.Marshal(ev.Data)
			if err != nil {
				return fmt.Errorf("store: audited call %q: encode: %w", ev.Action, err)
			}
			data = string(b)
		}
		rows[i] = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
		args = append(args, ev.ID, ev.At, ev.ActorID, ev.Action, ev.Target, ev.TargetID, ev.TargetName,
			ev.TenantID, ev.Detail, ev.IP, data)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_events (id, at, actor_id, action, target, target_id, target_name, tenant_id, detail, ip, data)
		 VALUES `+strings.Join(rows, ", "), args...)
	if err != nil {
		return fmt.Errorf("store: write %d audited calls: %w", len(evs), err)
	}
	return nil
}

// PurgeEndpointCallsBefore drops the audited calls older than cutoff.
func (s *Store) PurgeEndpointCallsBefore(ctx context.Context, cutoff int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_events WHERE target = ? AND at < ?`, AuditTargetEndpoint, cutoff)
	if err != nil {
		return 0, fmt.Errorf("store: purge audited calls: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// SettingEndpointCallRetention is how many days the trail keeps an audited
// call. Root's, like the trail's own retention.
const SettingEndpointCallRetention = "endpointCallRetention"

// EndpointCallRetentionChoices are the lifetimes offered, in days.
var EndpointCallRetentionChoices = []int{7, 30, 90, 180, 365}

// DefaultEndpointCallRetention is a month.
const DefaultEndpointCallRetention = 30

// EndpointCallRetentionDays reads the retention, the default when unset or
// invalid.
func (s *Store) EndpointCallRetentionDays(ctx context.Context) int {
	var d int
	if err := s.GetSetting(ctx, SettingEndpointCallRetention, &d); err != nil || !slices.Contains(EndpointCallRetentionChoices, d) {
		return DefaultEndpointCallRetention
	}
	return d
}

// SetEndpointCallRetentionDays stores it, refusing a lifetime that is not
// offered.
func (s *Store) SetEndpointCallRetentionDays(ctx context.Context, days int) error {
	if !slices.Contains(EndpointCallRetentionChoices, days) {
		return fmt.Errorf("endpointRetentionDays %d: expected one of %v", days, EndpointCallRetentionChoices)
	}
	return s.SetSetting(ctx, SettingEndpointCallRetention, days)
}
