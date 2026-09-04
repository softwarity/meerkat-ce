package gateway

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// Bounding the wait stops one slow upstream holding a connection forever. It
// does not stop the gateway sending a thousand more requests that will each
// wait their own bound - which is how a service that is down takes the
// gateway's capacity with it, and how it is met by a stampede the moment it
// recovers.
func TestAfterEnoughFailuresTheUpstreamStopsBeingCalled(t *testing.T) {
	var calls atomic.Int64
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(down.Close)

	r := pathRoute("r1", "flaky", 1, "/**", down.URL)
	r.Breaker = &store.CircuitBreaker{Enabled: true, Trip: 3, Cool: "PT1S"}
	rt := newRouter(t, r)

	for range 3 {
		if res, _ := get(t, rt, "/x"); res.StatusCode != http.StatusBadGateway {
			t.Fatalf("before the trip: %d", res.StatusCode)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("the upstream was called %d times, want 3", calls.Load())
	}

	// Open: the caller is answered at once, and the service is left alone.
	for range 5 {
		res, body := get(t, rt, "/x")
		if res.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("with the circuit open: %d, want 503", res.StatusCode)
		}
		if len(body) == 0 {
			t.Error("nothing was served in place of the upstream")
		}
	}
	if calls.Load() != 3 {
		t.Errorf("the upstream was called %d times while the circuit was open", calls.Load())
	}
}

// After the cool-down exactly ONE request goes through to find out. The others
// keep being answered, or the cool-down means nothing.
func TestOnlyOneRequestProbesAfterTheCoolDown(t *testing.T) {
	var calls atomic.Int64
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(down.Close)

	r := pathRoute("r1", "flaky", 1, "/**", down.URL)
	r.Breaker = &store.CircuitBreaker{Enabled: true, Trip: 2, Cool: "PT1S"}
	rt := newRouter(t, r)
	for range 2 {
		get(t, rt, "/x")
	}
	before := calls.Load()

	time.Sleep(1100 * time.Millisecond)
	for range 4 {
		get(t, rt, "/x")
	}
	if got := calls.Load() - before; got != 1 {
		t.Errorf("%d requests went through after the cool-down, want exactly 1", got)
	}
}

// And a service that comes back closes the circuit.
func TestAnUpstreamThatAnswersAgainClosesTheCircuit(t *testing.T) {
	var healthy atomic.Bool
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("well"))
	}))
	t.Cleanup(up.Close)

	r := pathRoute("r1", "flaky", 1, "/**", up.URL)
	r.Breaker = &store.CircuitBreaker{Enabled: true, Trip: 2, Cool: "PT1S"}
	rt := newRouter(t, r)
	for range 2 {
		get(t, rt, "/x")
	}
	if res, _ := get(t, rt, "/x"); res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the circuit did not open: %d", res.StatusCode)
	}

	healthy.Store(true)
	time.Sleep(1100 * time.Millisecond)
	if res, body := get(t, rt, "/x"); res.StatusCode != http.StatusOK || body != "well" {
		t.Fatalf("the probe did not go through: %d %q", res.StatusCode, body)
	}
	// Closed again: everybody is served, not just the probe.
	for range 3 {
		if res, _ := get(t, rt, "/x"); res.StatusCode != http.StatusOK {
			t.Fatalf("still refusing after a success: %d", res.StatusCode)
		}
	}
}

// A 500 is a service that is UP and has a bug. Taking it out of rotation would
// turn one broken endpoint into a whole route nobody can reach, including the
// endpoints that work.
func TestAFiveHundredDoesNotOpenTheCircuit(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(up.Close)

	r := pathRoute("r1", "buggy", 1, "/**", up.URL)
	r.Breaker = &store.CircuitBreaker{Enabled: true, Trip: 2, Cool: "PT1M"}
	rt := newRouter(t, r)
	for range 6 {
		if res, _ := get(t, rt, "/x"); res.StatusCode != http.StatusInternalServerError {
			t.Fatalf("a 500 was turned into %d", res.StatusCode)
		}
	}
}

// Off is the default, and off means the upstream is called every time however
// badly it behaves - a breaker changes one failure into another, and an
// installation should meet that because somebody chose it.
func TestWithoutABreakerNothingIsRefused(t *testing.T) {
	var calls atomic.Int64
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(down.Close)

	rt := newRouter(t, pathRoute("r1", "flaky", 1, "/**", down.URL))
	for range 8 {
		if res, _ := get(t, rt, "/x"); res.StatusCode != http.StatusBadGateway {
			t.Fatalf("got %d", res.StatusCode)
		}
	}
	if calls.Load() != 8 {
		t.Errorf("the upstream was called %d times, want 8", calls.Load())
	}
}
