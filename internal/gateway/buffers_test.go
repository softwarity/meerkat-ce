package gateway

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

type discardWriter struct{ h http.Header }

func (d *discardWriter) Header() http.Header         { return d.h }
func (d *discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardWriter) WriteHeader(int)             {}

// A proxied request must not allocate a copy buffer of its own. Before the
// pool, every response cost a fresh 32 KB slice, which on one core under load
// is the garbage collector's work, not the gateway's (see proxyBuffers). The
// bound below sits between what a request costs with the pool and what it cost
// without: a proxy built without the pool crosses it on the first request.
func TestProxyingBorrowsItsCopyBuffer(t *testing.T) {
	up := benchUpstream(t)
	rt := newRouter(t, pathRoute("r1", "bench", 1, "/**", up.URL))
	serve := func() {
		w := &discardWriter{h: http.Header{}}
		rt.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	}
	for range 50 {
		serve()
	}
	const n = 300
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range n {
		serve()
	}
	runtime.ReadMemStats(&after)
	perRequest := (after.TotalAlloc - before.TotalAlloc) / n
	if perRequest >= proxyBufferSize {
		t.Fatalf("a proxied request allocates %d bytes, at least a whole copy buffer (%d): the proxy is not borrowing it",
			perRequest, proxyBufferSize)
	}
}
