package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The identity the router remembers is forgotten after a write served by the
// gateway's own pages and API, and ONLY then: not after a read, and not after a
// POST proxied to an application - the one kind of write that arrives by the
// thousand, and none of this cache's business.
func TestAfterWritesForgetsOnTheGatewaysOwnWritesOnly(t *testing.T) {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	mux.HandleFunc("POST /login", ok)
	mux.HandleFunc("GET /profile", ok)
	mux.HandleFunc("/", ok) // the router

	forgotten := 0
	h := afterWrites(mux, "/", func() { forgotten++ })
	serve := func(method, path string) int {
		before := forgotten
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
		return forgotten - before
	}

	if n := serve(http.MethodPost, "/login"); n != 1 {
		t.Errorf("a sign-in wrote and forgot %d times, want 1", n)
	}
	if n := serve(http.MethodGet, "/profile"); n != 0 {
		t.Errorf("a read forgot %d times, want 0", n)
	}
	if n := serve(http.MethodPost, "/orders"); n != 0 {
		t.Errorf("a POST proxied to an application forgot %d times, want 0", n)
	}

	// The control plane names no proxied pattern: every write there forgets.
	admin := afterWrites(mux, "", func() { forgotten++ })
	before := forgotten
	admin.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/groups/g1", nil))
	if forgotten-before != 1 {
		t.Errorf("an admin write did not forget")
	}
}
