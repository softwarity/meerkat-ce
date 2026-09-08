package limits

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestAllowsUpToTheLimitThenRefuses(t *testing.T) {
	c := New(3, time.Minute)
	now := time.Now()
	for i := 1; i <= 3; i++ {
		v := c.Allow("alice", now)
		if !v.OK {
			t.Fatalf("request %d was refused, want allowed", i)
		}
		if want := 3 - i; v.Remaining != want {
			t.Errorf("request %d: %d remaining, want %d", i, v.Remaining, want)
		}
	}
	v := c.Allow("alice", now)
	if v.OK {
		t.Fatal("the fourth request passed a limit of three")
	}
	if v.ResetIn <= 0 {
		t.Error("a refusal with no Retry-After is a door with no sign on it")
	}
}

// Each key has its own budget: that is the whole point of writing a limit PER
// something rather than FOR someone.
func TestKeysAreCountedApart(t *testing.T) {
	c := New(2, time.Minute)
	now := time.Now()
	for _, who := range []string{"alice", "bob"} {
		for i := 0; i < 2; i++ {
			if !c.Allow(who, now).OK {
				t.Fatalf("%s was refused within budget", who)
			}
		}
		if c.Allow(who, now).OK {
			t.Errorf("%s went over budget", who)
		}
	}
}

// The failure a resetting bucket has and this does not: two hundred requests
// in two seconds, one hundred on each side of a boundary.
func TestTheWindowSlidesAcrossTheBoundary(t *testing.T) {
	c := New(100, time.Minute)
	start := time.Now()

	// A hundred in one burst. The window opens on the first of them.
	for i := 0; i < 100; i++ {
		if !c.Allow("alice", start).OK {
			t.Fatalf("refused at %d, inside the first hundred", i)
		}
	}
	// A second past the boundary, a bucket that resets would grant a fresh
	// hundred - two hundred in sixty-one seconds. The estimate still weighs
	// what just went, so a couple slip through and no more. Counted rather
	// than asserted as zero: the estimate spreads the previous window evenly,
	// which is an approximation, and pinning it to an exact number would be
	// pinning the arithmetic instead of the property.
	passed := 0
	for i := 0; i < 100; i++ {
		if c.Allow("alice", start.Add(61*time.Second)).OK {
			passed++
		}
	}
	if passed > 5 {
		t.Fatalf("%d more passed one second after the boundary: the limit is being doubled", passed)
	}
	// Halfway into the second window, half the burst has aged out: room comes
	// back GRADUALLY, which is the property a reset does not have.
	v := c.Allow("alice", start.Add(90*time.Second))
	if !v.OK {
		t.Fatal("half a window past the burst, nothing has come back")
	}
	if v.Remaining < 40 || v.Remaining > 55 {
		t.Errorf("%d remaining half a window after a full burst, want about half", v.Remaining)
	}
	// And a whole window later it is gone entirely.
	if v := c.Allow("alice", start.Add(125*time.Second)); v.Remaining < 95 {
		t.Errorf("%d remaining a full window later, want nearly all of it", v.Remaining)
	}
}

// A window nobody has touched for two windows holds nothing, and its key is
// let go - otherwise the map is a leak with a slow fuse.
func TestQuietKeysAreForgotten(t *testing.T) {
	c := New(5, time.Second)
	start := time.Now()
	for i := 0; i < 100; i++ {
		c.Allow("caller-"+strconv.Itoa(i), start)
	}
	if got := c.Tracked(); got != 100 {
		t.Fatalf("tracking %d keys, want 100", got)
	}
	// Fill past the cap so a sweep is asked for, long after the first hundred
	// went quiet.
	later := start.Add(time.Minute)
	for i := 0; i < MaxKeys; i++ {
		c.Allow("later-"+strconv.Itoa(i), later)
	}
	if got := c.Tracked(); got > MaxKeys {
		t.Fatalf("tracking %d keys, want at most %d", got, MaxKeys)
	}
	// And the quiet ones are the ones that went.
	c.mu.Lock()
	_, stillThere := c.keys["caller-0"]
	c.mu.Unlock()
	if stillThere {
		t.Error("a key quiet for a minute is still held")
	}
}

// The fence. A caller who rotates their key must not be able to grow this
// gateway's memory, nor to bypass the bound by never reusing a key.
func TestTheKeySetIsBounded(t *testing.T) {
	c := New(2, time.Minute)
	now := time.Now()
	allowed := 0
	for i := 0; i < MaxKeys*3; i++ {
		if c.Allow(fmt.Sprintf("throwaway-%d", i), now).OK {
			allowed++
		}
	}
	if got := c.Tracked(); got > MaxKeys+1 { // +1 for the shared overflow
		t.Fatalf("tracking %d keys after %d distinct callers, want at most %d",
			got, MaxKeys*3, MaxKeys+1)
	}
	// Past the cap the long tail shares ONE budget: it neither passes freely
	// nor takes everybody else down with it.
	if allowed > MaxKeys*2+2 {
		t.Errorf("%d requests passed by rotating the key: the bound is bypassable", allowed)
	}
	if allowed < MaxKeys {
		t.Errorf("only %d passed: tracked callers are being refused for somebody else's flood", allowed)
	}
}

// A refused request must not be counted. Otherwise a caller who backs off is
// kept out by their own refusals, and the lockout outlives the burst.
func TestRefusalsDoNotFeedTheCounter(t *testing.T) {
	c := New(2, time.Minute)
	start := time.Now()
	c.Allow("alice", start)
	c.Allow("alice", start)
	for i := 0; i < 50; i++ {
		c.Allow("alice", start.Add(time.Second))
	}
	// A window and a bit later, only the two that COUNTED are still weighing.
	if !c.Allow("alice", start.Add(2*time.Minute)).OK {
		t.Error("still refused two windows later: the refusals were counted")
	}
}

func TestConcurrentCallersAreCountedOnce(t *testing.T) {
	c := New(1000, time.Minute)
	now := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				if c.Allow("alice", now).OK {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	if allowed != 1000 {
		t.Errorf("%d of 2000 concurrent requests passed a limit of 1000", allowed)
	}
}

func BenchmarkAllow(b *testing.B) {
	c := New(1_000_000_000, time.Minute)
	now := time.Now()
	b.ReportAllocs()
	for b.Loop() {
		c.Allow("alice", now)
	}
}

func BenchmarkAllowParallel(b *testing.B) {
	c := New(1_000_000_000, time.Minute)
	now := time.Now()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Allow("alice", now)
		}
	})
}
