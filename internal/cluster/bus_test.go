package cluster_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/cluster"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// Two nodes, one database, and the question the whole package exists for: does
// the one that did NOT take the write find out?
//
// It needs a real PostgreSQL, because that is where the answer lives - the
// embedded database has one process and no second node to convince. Set
// MEERKAT_TEST_DATABASE_URL (make pg-up) and this runs; CI always does.
func twoNodes(t *testing.T) (a, b *store.Store) {
	t.Helper()
	url := dbtest.URL(t)
	if url == "" {
		t.Skip("set " + dbtest.Env + " to test the change bus: it needs two connections to one database")
	}
	// Two stores on the SAME schema and two data directories, which is what
	// two gateways are: shared state in the database, everything else apart.
	for _, st := range []**store.Store{&a, &b} {
		s, err := store.OpenAt(t.TempDir(), url)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		*st = s
	}
	return a, b
}

func TestTheOtherNodeReloadsWhatThisOneChanged(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var onA, onB atomic.Int64
	busA := cluster.New(sa)
	busB := cluster.New(sb)
	busA.Register(store.TopicRouting, func(context.Context) error { onA.Add(1); return nil })
	busB.Register(store.TopicRouting, func(context.Context) error { onB.Add(1); return nil })
	go busA.Run(ctx)
	go busB.Run(ctx)

	// Announced in a loop rather than once, and that is not a retry: it is how
	// the test avoids caring whether B had finished connecting. Every
	// announcement is a real write with a real version, and B converges on
	// whichever one it was there for.
	deadline := time.Now().Add(10 * time.Second)
	for onB.Load() == 0 && time.Now().Before(deadline) {
		busA.Announce(ctx, store.TopicRouting)
		time.Sleep(50 * time.Millisecond)
	}
	if onB.Load() == 0 {
		t.Fatal("the second node never reloaded: a route saved on one gateway would not exist on the other")
	}
	// And the node that made the change does NOT reload again. PostgreSQL
	// delivers a notification to every listener including the sender, so
	// without the version it already acted on, every write would cost this
	// node a second full recompilation of its route plan.
	if n := onA.Load(); n != 0 {
		t.Errorf("the node that wrote reloaded %d extra times: it had already applied its own change", n)
	}
}

// The backstop. A notification is lost whenever nobody is listening at that
// instant, so the table has to be enough on its own - here the change is
// marked WITHOUT announcing it, which is exactly what a dropped connection
// looks like from the other side.
func TestALostNotificationStillArrives(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var reloaded atomic.Int64
	busB := cluster.New(sb, cluster.WithPollEvery(200*time.Millisecond))
	busB.Register(store.TopicRouting, func(context.Context) error { reloaded.Add(1); return nil })
	go busB.Run(ctx)
	// Let it settle on the current versions before moving one, or it would be
	// racing its own priming rather than testing the timer.
	time.Sleep(500 * time.Millisecond)

	if _, err := sa.MarkChanged(ctx, store.TopicRouting); err != nil {
		t.Fatalf("mark: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for reloaded.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if reloaded.Load() == 0 {
		t.Fatal("a change nobody was told about never arrived: the table is not being read")
	}
}

// A topic this node does not cache is not this node's business.
func TestAnUnregisteredTopicIsIgnored(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var reloaded atomic.Int64
	busB := cluster.New(sb, cluster.WithPollEvery(100*time.Millisecond))
	busB.Register(store.TopicRouting, func(context.Context) error { reloaded.Add(1); return nil })
	go busB.Run(ctx)
	time.Sleep(500 * time.Millisecond)

	cluster.New(sa).Announce(ctx, store.TopicCertificates)
	time.Sleep(time.Second)
	if n := reloaded.Load(); n != 0 {
		t.Errorf("reloaded the routes %d times for a certificate change", n)
	}
}

// The single-binary deployment must pay nothing for any of this: one process
// owns the embedded file, so there is nobody to tell and nothing to listen to.
// Run has to RETURN rather than sit in a loop that can never fire.
func TestOnTheEmbeddedDatabaseThereIsNothingToDo(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if st.Clustered() {
		t.Fatal("the embedded database is one process")
	}

	var reloaded atomic.Int64
	bus := cluster.New(st)
	bus.Register(store.TopicRouting, func(context.Context) error { reloaded.Add(1); return nil })

	done := make(chan struct{})
	go func() { bus.Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return: a single-binary gateway would carry a goroutine that can never fire")
	}
	// And announcing stays harmless: the same handlers call it whether there
	// is a cluster or not.
	bus.Announce(context.Background(), store.TopicRouting)
	if n := reloaded.Load(); n != 0 {
		t.Errorf("reloaded %d times with no second node", n)
	}
}
