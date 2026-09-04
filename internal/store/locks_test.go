package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The lock has to hold between two STORES, not just between two goroutines -
// that is the whole difference between a mutex and a cluster lock, and the
// only way to see it is to open the database twice.
func twoStores(t *testing.T) (a, b *Store) {
	t.Helper()
	url := dbtest.URL(t)
	if url == "" {
		t.Skip("set " + dbtest.Env + " to test the cluster lock: a mutex would pass without it")
	}
	for _, st := range []**Store{&a, &b} {
		s, err := OpenAt(t.TempDir(), url)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		*st = s
	}
	return a, b
}

func TestTwoNodesDoNotHoldTheSameLock(t *testing.T) {
	a, b := twoStores(t)
	ctx := context.Background()

	held := make(chan struct{})
	release := make(chan struct{})
	var overlapped atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := a.WithLock(ctx, "probe", func(context.Context) error {
			close(held)
			<-release
			return nil
		}); err != nil {
			t.Errorf("node A: %v", err)
		}
	}()

	<-held
	// While A holds it, B must not get in. A try is the readable way to ask:
	// a blocking wait would prove the same thing by hanging.
	ran, err := b.TryLock(ctx, "probe", func(context.Context) error {
		overlapped.Store(true)
		return nil
	})
	if err != nil {
		t.Fatalf("node B: %v", err)
	}
	if ran || overlapped.Load() {
		t.Fatal("both nodes held the same lock: two gateways would order the same certificate")
	}

	close(release)
	wg.Wait()

	// And once A is done, B gets it - a lock that never releases is the other
	// half of the bug, and it is the half that survives a restart.
	ran, err = b.TryLock(ctx, "probe", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("node B, second try: %v", err)
	}
	if !ran {
		t.Fatal("the lock was never released")
	}
}

// The reserved connection goes back to the POOL rather than being closed, so a
// lock left on it would be held by a session that goes on serving queries -
// and nothing would ever release it. Taking and releasing more times than the
// pool has connections is what catches that.
func TestTheLockIsReleasedOntoAPooledConnection(t *testing.T) {
	a, b := twoStores(t)
	ctx := context.Background()
	for i := range 25 {
		if err := a.WithLock(ctx, "probe", func(context.Context) error { return nil }); err != nil {
			t.Fatalf("round %d on A: %v", i, err)
		}
	}
	done := make(chan error, 1)
	go func() {
		done <- b.WithLock(ctx, "probe", func(context.Context) error { return nil })
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("node B: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("node B waited forever: a lock leaked onto a connection that went back to the pool")
	}
}

// A different name is a different lock, or every serialised thing in the
// product would queue behind every other one.
func TestDifferentNamesDoNotWaitForEachOther(t *testing.T) {
	a, b := twoStores(t)
	ctx := context.Background()

	held := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = a.WithLock(ctx, "acme:one.example", func(context.Context) error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	defer close(release)

	ran, err := b.TryLock(ctx, "acme:two.example", func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("node B: %v", err)
	}
	if !ran {
		t.Fatal("a lock on one name blocked another: certificates for two hosts would be issued one after the other")
	}
}

// On the embedded database the promise still has to hold - one process, but
// several goroutines - or the API would mean something different depending on
// which database it was pointed at.
func TestTheLockIsRealOnTheEmbeddedDatabaseToo(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var inside, peak atomic.Int64
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = st.WithLock(context.Background(), "probe", func(context.Context) error {
				n := inside.Add(1)
				if n > peak.Load() {
					peak.Store(n)
				}
				time.Sleep(time.Millisecond)
				inside.Add(-1)
				return nil
			})
		}()
	}
	wg.Wait()
	if peak.Load() > 1 {
		t.Fatalf("%d goroutines were inside at once: a lock is a lock on either database", peak.Load())
	}
}
