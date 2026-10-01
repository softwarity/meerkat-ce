package live

import (
	"context"
	"testing"

	livewire "github.com/softwarity/livewire/go"
	"github.com/softwarity/meerkat/internal/store"
)

// read answers the window's rows, newest first.
func read(t *testing.T, source livewire.Source) []map[string]any {
	t.Helper()
	window, err := source.Read(context.Background(), struct{}{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := make([]map[string]any, 0, len(window.Rows))
	for _, row := range window.Rows {
		out = append(out, row.Data)
	}
	return out
}

// ofKind picks the writes to one kind of object.
func ofKind(rows []map[string]any, kind string) []map[string]any {
	var out []map[string]any
	for _, row := range rows {
		if row["kind"] == kind {
			out = append(out, row)
		}
	}
	return out
}

// A write reaches the screens of the kind it touched, and it NAMES the row it
// touched - which is what lets a screen reload one line instead of its list.
func TestOneWriteNamesOneRow(t *testing.T) {
	c := NewChanges()
	view := c.For([]string{"route", "user"}, nil)
	if rows := read(t, view); len(rows) != 0 {
		t.Fatalf("a gateway that has taken no write announces %d writes", len(rows))
	}
	c.Record(store.AuditEvent{
		ID: store.NewEventID(), At: 1789999188, Target: "route",
		Action: "route.update", TargetID: "r1", TargetName: "rabbitmq", ActorName: "root",
	})
	rows := read(t, view)
	if len(rows) != 1 {
		t.Fatalf("one write announced %d lines: %v", len(rows), rows)
	}
	if rows[0]["targetId"] != "r1" || rows[0]["targetName"] != "rabbitmq" || rows[0]["actor"] != "root" {
		t.Errorf("a named kind must say WHICH row moved and who moved it: %v", rows[0])
	}
}

// Two rows of one kind written back to back are two lines, and that is the
// whole point of a line per write: a line per KIND would have kept the last id
// and dropped the other, so a screen reloading one row would reload the wrong
// one and keep showing the stale one for ever.
func TestTwoRowsOfOneKindAreTwoLines(t *testing.T) {
	c := NewChanges()
	view := c.For([]string{"route"}, nil)
	for _, id := range []string{"alpha", "bravo"} {
		c.Record(store.AuditEvent{
			ID: store.NewEventID(), At: 1789999188, Target: "route",
			Action: "route.update", TargetID: id, TargetName: id,
		})
	}
	rows := ofKind(read(t, view), "route")
	if len(rows) != 2 {
		t.Fatalf("two routes saved in one window announced %d lines: %v", len(rows), rows)
	}
	// Newest first, the way a list is read.
	if rows[0]["targetId"] != "bravo" || rows[1]["targetId"] != "alpha" {
		t.Errorf("the newest write must come first: %v", rows)
	}
}

// A write that names no row - a reorder, an import, "everyone must change their
// password" - is published as it stands. The empty targetId is what tells a
// screen to read its whole list again, and it is the honest answer: those moved
// rows nobody named.
func TestAWriteThatNamesNoRowSaysSo(t *testing.T) {
	c := NewChanges()
	view := c.For([]string{"route"}, nil)
	c.Record(store.AuditEvent{
		ID: store.NewEventID(), At: 1789999188, Target: "route", Action: "route.reorder",
	})
	rows := ofKind(read(t, view), "route")
	if len(rows) != 1 || rows[0]["targetId"] != "" {
		t.Fatalf("a reorder must announce itself without naming a row: %v", rows)
	}
}

// The journal is a window, not a history: it holds what a screen can have
// missed between two frames, and a screen that falls off the end reads its list
// again rather than replaying an hour.
func TestTheJournalIsBounded(t *testing.T) {
	c := NewChanges()
	view := c.For([]string{"route"}, nil)
	for i := 0; i < changesKept+20; i++ {
		c.Record(store.AuditEvent{ID: store.NewEventID(), At: 1789999188, Target: "route"})
	}
	if rows := read(t, view); len(rows) != changesKept {
		t.Fatalf("the journal holds %d lines, expected %d", len(rows), changesKept)
	}
}

// Two writes inside the same SECOND are two lines with two versions. The trail
// stamps an instant in seconds, and a version built from it alone would have
// made the second write invisible - the library publishes nothing for a window
// whose versions did not move.
func TestTwoWritesInOneSecondAreTwoVersions(t *testing.T) {
	c := NewChanges()
	view := c.For([]string{"route"}, nil)
	const second = 1789999188
	c.Record(store.AuditEvent{ID: store.NewEventID(), At: second, Target: "route", Action: "route.create"})
	c.Record(store.AuditEvent{ID: store.NewEventID(), At: second, Target: "route", Action: "route.update"})
	rows, err := view.Read(context.Background(), struct{}{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(rows.Rows) != 2 || rows.Rows[0].UpdatedAt == rows.Rows[1].UpdatedAt {
		t.Fatalf("two writes in one second are not two versions: %v", rows.Rows)
	}
}

// A configuration import replays the whole installation. It names no row and it
// is published under the kind that stands for all of them - the screens read
// that as "read your list again", which is what an import leaves true.
func TestImportingAConfigurationIsPublishedAsSuch(t *testing.T) {
	c := NewChanges()
	view := c.For(store.AuditTargetKinds(), nil)
	c.Record(store.AuditEvent{
		ID: store.NewEventID(), At: 1789999188, Target: store.AuditTargetAll,
		Action: "config.import", ActorName: "root",
	})
	rows := read(t, view)
	if len(rows) != 1 || rows[0]["kind"] != store.AuditTargetAll {
		t.Fatalf("an import announced %v", rows)
	}
	if rows[0]["action"] != "config.import" || rows[0]["targetId"] != "" {
		t.Errorf("a screen must read WHY its list changed, and that no row was named: %v", rows[0])
	}
}

// What a reader may be told is what the trail would show them. A kind whose rows
// belong to an organisation is partitioned by organisation, which one shared
// read cannot check per subscriber - so the reader hears that it moved and
// nothing about it.
func TestAQuietKindSaysThatItMovedAndNothingElse(t *testing.T) {
	c := NewChanges()
	view := c.For([]string{"user"}, []string{"group"})
	c.Record(store.AuditEvent{
		ID: store.NewEventID(), At: 1789999188, Target: "group", Action: "group.update",
		TargetID: "g1", TargetName: "pilots", TenantID: "t1", ActorName: "root",
	})
	rows := ofKind(read(t, view), "group")
	if len(rows) != 1 {
		t.Fatalf("a quiet kind must still say that it moved: %v", rows)
	}
	group := rows[0]
	for _, field := range []string{"action", "targetId", "targetName", "actor"} {
		if _, said := group[field]; said {
			t.Errorf("a quiet kind named %q: %v", field, group)
		}
	}
	if group["at"] != int64(1789999188) {
		t.Errorf("a quiet kind must say which kind moved and when: %v", group)
	}
}

// A kind outside the perimeter is absent altogether: a reader who administers
// nothing of the routing plane is not told that its routes moved.
func TestAKindOutsideThePerimeterIsAbsent(t *testing.T) {
	c := NewChanges()
	c.Record(store.AuditEvent{ID: store.NewEventID(), At: 1789999188, Target: "route", Action: "route.update"})
	rows := read(t, c.For([]string{"user"}, nil))
	if len(rows) != 0 {
		t.Fatalf("a reader outside the routing plane was told its routes moved: %v", rows)
	}
}

// Two readers with the same perimeter must share one read, and two perimeters
// must not: the key is what the library groups subscribers by, so a key that
// ignored the perimeter would serve one reader's rows to another.
func TestThePerimeterIsInTheKey(t *testing.T) {
	c := NewChanges()
	infra := c.For([]string{"route"}, nil)
	app := c.For([]string{"user"}, nil)
	quiet := c.For(nil, []string{"route"})
	if infra.Key(struct{}{}) != c.For([]string{"route"}, nil).Key(struct{}{}) {
		t.Error("two readers of the same perimeter do not share a read")
	}
	if infra.Key(struct{}{}) == app.Key(struct{}{}) {
		t.Error("two perimeters share one read")
	}
	if infra.Key(struct{}{}) == quiet.Key(struct{}{}) {
		t.Error("naming a kind and staying quiet about it share one read")
	}
}

// The wake channel is created once and shared. livewire calls Wake per pump and
// offers no way to stop what it hands back, so a fresh channel per subscription
// would leak one per screen ever opened.
func TestWakeIsOneChannel(t *testing.T) {
	c := NewChanges()
	first := c.For([]string{"route"}, nil).Wake()
	second := c.For([]string{"user"}, nil).Wake()
	if first != second {
		t.Fatal("each view woke on a channel of its own")
	}
	c.Record(store.AuditEvent{ID: store.NewEventID(), At: 1, Target: "route"})
	select {
	case <-first:
	default:
		t.Fatal("a write woke nobody")
	}
}

// A configuration import is published to every reader, named or not: it replays
// the installation, so every screen is showing something that may no longer be
// true - including the screens of somebody who may not read which configuration
// was imported. What they get is "read your list again", which names nothing.
func TestAnImportReachesEveryReader(t *testing.T) {
	c := NewChanges()
	c.Record(store.AuditEvent{
		ID: store.NewEventID(), At: 1789999188, Target: store.AuditTargetAll,
		Action: "config.import", TargetName: "nightly.yaml", ActorName: "root",
	})
	rows := read(t, c.For([]string{"user"}, nil))
	if len(rows) != 1 || rows[0]["kind"] != store.AuditTargetAll {
		t.Fatalf("an application administrator was not told of an import: %v", rows)
	}
	if _, named := rows[0]["targetName"]; named {
		t.Errorf("the import was NAMED to a reader who does not administer it: %v", rows[0])
	}
}
