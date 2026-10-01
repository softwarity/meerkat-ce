package live

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	livewire "github.com/softwarity/livewire/go"
	"github.com/softwarity/meerkat/internal/store"
)

// ChangesTopic is what every console screen subscribes to (CONSOLE-13).
const ChangesTopic = "changes"

// changesKept is how many writes the journal holds.
//
// A window, not a history: what a screen needs is "what happened since I last
// looked", and anything older than that is answered by reading the list again -
// which is what a screen falling off the end of this window does. Fifty is well
// past what a console produces between two frames; the library caps a window at
// 200 anyway.
const changesKept = 50

// Changes is what tells a screen that what it is showing has moved.
//
// WHAT IT CARRIES, AND WHY THAT AND NOT THE ROWS. One line per WRITE - the kind
// of object, which one, what was done to it and by whom - never the object
// itself. That is a security decision rather than a frugal one: livewire shares
// one read between every subscriber of the same key and calls Read with a
// context of its own (Registry.Watch), so a source cannot know WHO is reading.
// Publishing rows here would mean deciding who may see them somewhere other
// than where that is already decided - the endpoints - and the second copy of a
// rule is the one that drifts. So the socket says "route X was updated" and the
// screen asks the API it already asks, with the caller's own session behind it.
//
// ONE LINE PER WRITE AND NOT PER KIND, which is what makes a screen able to
// reload ONE row instead of its whole list. A line per kind holding the last
// write looks equivalent and is not: two routes saved inside one coalescing
// window, or four saved while a laptop was asleep, collapse into one id - and
// every id but the last is lost, silently, which is the worst shape a cache
// invalidation can take.
//
// WHERE IT COMES FROM. Every administrative mutation passes through one funnel
// (admin.audit): it names the actor, the kind and the instance, and it writes
// nothing at all for a save that changed nothing. So this needs no emission at
// the call sites, costs nothing while nobody writes, and stays complete when an
// endpoint is added next month. A write on ANOTHER node arrives by the change
// bus (store.TopicChanged) and is recorded here exactly the same way.
//
// WHAT IT IS NOT. Not a substitute for the revision on each row: a lost
// notification means a screen is late, and a late screen writing over somebody
// is still refused with a 409. The push makes the collision rare; the revision
// makes it harmless.
type Changes struct {
	mu sync.RWMutex
	// The newest write first. Bounded by changesKept.
	log []store.AuditEvent

	// ONE wake channel, created once - same reason as Traffic: livewire calls
	// Wake per pump and offers no way to stop what it hands back, so a fresh
	// channel per subscription would accumulate one per screen ever opened.
	once sync.Once
	wake chan struct{}
}

// NewChanges returns an empty journal. Nothing to seed: a screen that has heard
// nothing yet is a screen showing what it just read.
func NewChanges() *Changes {
	return &Changes{}
}

// Record takes one administrative event and wakes whoever is watching.
//
// A write that names no instance - a reorder, a configuration import, "everyone
// must change their password" - is recorded as it stands, with an empty
// targetId. That is what tells a screen to read its whole list again instead of
// one row, and it is the honest answer: those writes moved rows nobody named.
func (c *Changes) Record(ev store.AuditEvent) {
	if ev.Target == "" {
		return
	}
	c.mu.Lock()
	c.log = append([]store.AuditEvent{ev}, c.log...)
	if len(c.log) > changesKept {
		c.log = c.log[:changesKept]
	}
	c.mu.Unlock()
	c.signal()
}

// signal is a nudge, never a queue: a pump that has not woken yet will read the
// state that includes this event anyway.
func (c *Changes) signal() {
	select {
	case c.waking() <- struct{}{}:
	default:
	}
}

func (c *Changes) waking() chan struct{} {
	c.once.Do(func() { c.wake = make(chan struct{}, 1) })
	return c.wake
}

// For is one reader's view of the journal, and the partition is the trail's own
// (store.AuditTargets, admin.auditScope):
//
//   - named: the kinds whose writes are DESCRIBED - which object, what was done
//     to it, by whom - which is only ever the kinds the audit screen would show
//     this reader anyway,
//   - quiet: the kinds they administer but whose rows belong to an organisation,
//     so all they are told is that something of that kind moved,
//   - anything else: absent.
func (c *Changes) For(named, quiet []string) livewire.Source {
	v := &changesView{
		changes: c,
		named:   append([]string(nil), named...),
		quiet:   append([]string(nil), quiet...),
	}
	sort.Strings(v.named)
	sort.Strings(v.quiet)
	return v
}

// changesView is one perimeter's source. Several readers of the same perimeter
// share one, which is what Key is for.
type changesView struct {
	changes *Changes
	named   []string
	quiet   []string
}

// ReadQuery is the trust boundary, and there is nothing to read: what a reader
// may see is decided by which view it was handed, never by what it asks for.
// A filter here would be a filter a client could widen.
func (v *changesView) ReadQuery(_ json.RawMessage) (any, error) { return struct{}{}, nil }

// Key: one read per perimeter, however many screens are watching.
func (v *changesView) Key(_ any) string {
	return ChangesTopic + "|" + strings.Join(v.named, ",") + "|" + strings.Join(v.quiet, ",")
}

// Wake fires when anything was recorded. Coarser than it could be - a write to
// a kind this view does not publish still wakes its pump - and deliberately so:
// the read that follows walks fifty entries, and a channel per kind would be
// nineteen channels to fan one write out to.
func (v *changesView) Wake() <-chan struct{} { return v.changes.waking() }

// Read is the journal as it stands, newest first, filtered to this perimeter.
func (v *changesView) Read(_ context.Context, _ any) (livewire.Window, error) {
	v.changes.mu.RLock()
	defer v.changes.mu.RUnlock()
	rows := make([]livewire.Row, 0, len(v.changes.log))
	for _, ev := range v.changes.log {
		described, shows := v.shows(ev.Target)
		if !shows {
			continue
		}
		// The id IS the version, and that is not a shortcut: an event never
		// changes once recorded, so a line already on a screen is never sent
		// again - what crosses the socket on a write is one line. The id sorts
		// by time (store.NewEventID), so a client compares its own position
		// with a string comparison and needs no clock of its own.
		data := map[string]any{"kind": ev.Target, "at": ev.At}
		if described {
			data["action"] = ev.Action
			data["targetId"] = ev.TargetID
			data["targetName"] = ev.TargetName
			data["actor"] = ev.ActorName
			data["actorToken"] = ev.ActorToken
		}
		rows = append(rows, livewire.Row{ID: ev.ID, UpdatedAt: ev.ID, Data: data})
	}
	total := len(rows)
	return livewire.Window{Rows: rows, Total: &total}, nil
}

// shows answers whether this view publishes a kind, and whether it may name
// what happened.
//
// The kind that stands for all of them is published to EVERYONE, and quietly
// unless the reader administers it. That is not a hole in the partition, it is
// what the partition is for: importing or restoring a configuration replays the
// installation, so every screen is showing something that may no longer be
// true - including the screens of somebody who has no business reading which
// configuration was imported. What they are told is "read your list again",
// which names nothing.
func (v *changesView) shows(kind string) (described, shows bool) {
	if contains(v.named, kind) {
		return true, true
	}
	if contains(v.quiet, kind) {
		return false, true
	}
	return false, kind == store.AuditTargetAll
}

func contains(kinds []string, kind string) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}
