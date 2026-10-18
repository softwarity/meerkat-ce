package devtunnel

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// trail collects what the registries write through the funnel.
type trail struct {
	mu  sync.Mutex
	evs []store.AuditEvent
}

func (t *trail) add(_ context.Context, ev store.AuditEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.evs = append(t.evs, ev)
}

func (t *trail) of(action string) []store.AuditEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []store.AuditEvent
	for _, ev := range t.evs {
		if ev.Action == action {
			out = append(out, ev)
		}
	}
	return out
}

// fakeControl records what a node was asked to end.
type fakeControl struct {
	mu     sync.Mutex
	closed []string
}

func (c *fakeControl) CloseSession(id, reason string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = append(c.closed, "session "+id+" "+reason)
	return true
}

func (c *fakeControl) CloseKey(fp, reason string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = append(c.closed, "key "+fp+" "+reason)
	return 1
}

// Two replicas holding parts of one run: one session on the page, one line
// when it opens, one when it ends - and "closed" is what the line says, even
// when another replica was the last to let go.
func TestARunSpreadOverReplicasIsOneSession(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	tr := &trail{}
	var asked []string
	nodes := map[string]*Registry{}
	for _, node := range []string{"a", "b"} {
		r := NewRegistry(events.NewHub())
		r.Wire(Wiring{Store: st, Node: node, Audit: tr.add,
			Close: func(kind, target, _ string) { asked = append(asked, node+": "+kind+" "+target) }})
		nodes[node] = r
	}
	a, b := nodes["a"], nodes["b"]
	run := Session{ID: "run-1", Who: "alice", Key: "SHA256:abc", From: "10.0.0.9:51000", Version: "2.29.0",
		Since: time.Now().Add(-time.Minute)}

	a.SessionOpened(run)
	later := run
	later.Since = time.Now()
	b.SessionOpened(later)
	if n := len(tr.of("plug.session.open")); n != 1 {
		t.Fatalf("a run on two replicas opened %d times in the trail, want 1", n)
	}
	if got := tr.of("plug.session.open")[0]; got.IP != "10.0.0.9" || got.ActorName != "alice" ||
		!strings.Contains(got.Detail, "plug 2.29.0") {
		t.Errorf("the opening line: %+v", got)
	}

	a.ReportSessions("b", b.MySessions(), time.Now())
	a.Set(Served{Name: "checkout", Who: "alice", Session: "run-1", Since: time.Now()})
	list := a.Sessions()
	if len(list) != 1 || list[0].Nodes != 2 || !list[0].Since.Equal(run.Since) ||
		len(list[0].Names) != 1 || list[0].Names[0].Name != "checkout" {
		t.Fatalf("the merged list: %+v", list)
	}
	if n := len(tr.of("plug.serve")); n != 1 {
		t.Errorf("serving a name wrote %d lines, want 1", n)
	}

	// Ended by an administrator on b, while a's part is still there: no end yet.
	b.SessionClosed(later, store.PlugEndClosed, "closed by an administrator of this gateway")
	if n := len(tr.of("plug.session.close")); n != 0 {
		t.Fatalf("the run ended while a replica still held it")
	}
	a.Drop("checkout")
	a.SessionClosed(run, "ended", "")
	ends := tr.of("plug.session.close")
	if len(ends) != 1 || !strings.Contains(ends[0].Detail, "ended: closed - closed by an administrator") {
		t.Fatalf("the end of the run: %+v", ends)
	}

	// Ending a run is applied here and asked of the other nodes.
	ctl := &fakeControl{}
	a.SetControl(ctl)
	a.CloseSession("run-2", "why")
	a.CloseKey("SHA256:abc", "gone")
	if strings.Join(ctl.closed, "|") != "session run-2 why|key SHA256:abc gone" ||
		strings.Join(asked, "|") != "a: session run-2|a: key SHA256:abc" {
		t.Errorf("closing: here %v, asked of the others %v", ctl.closed, asked)
	}
}

// A replica that died holding part of a run is found by its silence, and the
// run ends in the trail when nobody else holds it.
func TestASilentReplicaLetsGoOfItsSessions(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	tr := &trail{}
	dead := NewRegistry(events.NewHub())
	dead.Wire(Wiring{Store: st, Node: "dead", Audit: tr.add})
	dead.SessionOpened(Session{ID: "run-9", Who: "bob", Since: time.Now()})

	alive := NewRegistry(events.NewHub())
	alive.Wire(Wiring{Store: st, Node: "alive", Audit: tr.add})
	// Not yet: the dead node's hold is younger than the silence that proves it.
	alive.sweepHolds(context.Background())
	if n := len(tr.of("plug.session.close")); n != 0 {
		t.Fatalf("a fresh hold was taken for a dead one")
	}
	alive.now = func() time.Time { return time.Now().Add(2 * alive.StaleAfter) }
	alive.sweepHolds(context.Background())
	ends := tr.of("plug.session.close")
	if len(ends) != 1 || !strings.Contains(ends[0].Detail, "ended: dropped") {
		t.Fatalf("the silent replica's run: %+v", ends)
	}
}
