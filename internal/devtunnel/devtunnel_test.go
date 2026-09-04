package devtunnel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// What is served reaches every open page, developer or not: an override
// changes what the application in front of a visitor IS.
func TestServingANameReachesEveryPage(t *testing.T) {
	hub := events.NewHub()
	anyone, err := hub.Subscribe(events.TopicAll)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(hub)

	since := time.Unix(1700000000, 0).UTC()
	reg.Set(Served{Name: "checkout", Who: "alice", Ports: []string{"8080"}, Since: since})

	var msg events.Message
	if err := json.Unmarshal(<-anyone.Events(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != EventServed {
		t.Fatalf("type = %q", msg.Type)
	}
	// The attribution is the point of the Enterprise edition: "by Alice", not
	// "by somebody".
	if raw, _ := json.Marshal(msg.Data); !strings.Contains(string(raw), `"who":"alice"`) {
		t.Fatalf("the event does not name who is serving: %s", raw)
	}

	if list := reg.List(); len(list) != 1 || list[0].Name != "checkout" || list[0].Who != "alice" {
		t.Fatalf("registry = %+v", list)
	}

	reg.Drop("checkout")
	if err := json.Unmarshal(<-anyone.Events(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != EventUnserved {
		t.Fatalf("type = %q", msg.Type)
	}
	if list := reg.List(); len(list) != 0 {
		t.Fatalf("a dropped name is still listed: %+v", list)
	}

	// Dropping what was never served says nothing: the agent sweeps its leases
	// at startup, and a sweep that announced every name it did not find would
	// be noise on every restart.
	reg.Drop("checkout")
	if len(anyone.Events()) != 0 {
		t.Fatal("dropping an unknown name published an event")
	}
}

// The list a page reads at load is ordered, so an unchanged state looks
// unchanged between two reads.
func TestTheServedListIsOrdered(t *testing.T) {
	reg := NewRegistry(events.NewHub())
	for _, n := range []string{"zeta", "alpha", "mid"} {
		reg.Set(Served{Name: n})
	}
	got := reg.List()
	if len(got) != 3 || got[0].Name != "alpha" || got[1].Name != "mid" || got[2].Name != "zeta" {
		t.Fatalf("list = %+v", got)
	}
}

// No agent linked in: the community image says which edition runs one and
// what to use instead, rather than failing silently or pretending.
func TestWithoutAnAgentTheTunnelExplainsItself(t *testing.T) {
	if Linked() {
		t.Skip("an agent is linked into this test binary")
	}
	err := Run(context.Background(), Deps{})
	if err == nil {
		t.Fatal("Run started something with no agent linked in")
	}
	if !edition.Enterprise && !strings.Contains(err.Error(), "Enterprise edition") {
		t.Fatalf("the community image does not name the edition: %v", err)
	}
	// And nothing is supervised: no goroutine, no port, no log line.
	Supervise(context.Background(), Deps{})
}

// Empty address, no tunnel: the outright off switch, for a cluster that wants
// the gateway without the tunnel. Distinct from the usual case, where a port
// IS named and the tunnel decides for itself whether it can do the job.
func TestAnEmptyAddressTurnsTheTunnelOff(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if !st.DevMode(context.Background()) {
		t.Fatal("developer mode is expected on by default, which is what makes this case worth guarding")
	}
	// Returns at once rather than looping: the address is a startup decision,
	// so there is nothing to wait for. A timeout here would mean it started
	// supervising something that can never run.
	done := make(chan struct{})
	go func() { defer close(done); Supervise(context.Background(), Deps{Store: st}) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Supervise is watching a tunnel that has no port to open")
	}
}

// The control path, with a stub agent: the tunnel runs when it reports itself
// available, and stays shut - without announcing anything - when it does not.
// The refusal is the case a workstation lives in, so it is the one worth
// pinning: no port, no goroutine, and no "listening" line in front of it.
func TestTheTunnelIsAskedBeforeItIsAnnounced(t *testing.T) {
	saved, savedPoll := hooks, pollEvery
	pollEvery = 50 * time.Millisecond
	t.Cleanup(func() { hooks, pollEvery = saved, savedPoll })

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var ran atomic.Bool
	refuse := errors.New("no orchestrator here: " + ErrNoResources.Error())
	var available atomic.Pointer[error]
	available.Store(&refuse)
	Register(Hooks{
		Available: func() error { return *available.Load() },
		Run: func(ctx context.Context, _ Deps) error {
			ran.Store(true)
			<-ctx.Done()
			return nil
		},
	})

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go Supervise(ctx, Deps{Store: st, Registry: NewRegistry(events.NewHub()), Addr: ":0"})

	time.Sleep(3 * pollEvery)
	if ran.Load() {
		t.Fatal("the tunnel ran although it reported itself unavailable")
	}

	// It becomes available. The supervisor picks it up on a later tick, but
	// not the very next one: each refusal doubles the wait, which is the point
	// of the backoff - a machine that has been refusing for hours is not
	// probed every ten seconds forever.
	var ok error
	available.Store(&ok)
	deadline := time.Now().Add(100 * pollEvery)
	for time.Now().Before(deadline) && !ran.Load() {
		time.Sleep(200 * time.Millisecond)
	}
	if !ran.Load() {
		t.Fatal("the tunnel never started once it could")
	}
}
