package certs

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The loop this closes: a load balancer terminates TLS and forwards plain
// HTTP, the gateway sees no TLS and redirects to https, the balancer
// terminates it again and forwards plain HTTP... forever, on the one
// deployment shape a clustered gateway is put in.
func TestATerminatedRequestIsNotSentBackToHTTPS(t *testing.T) {
	served := false
	d := NewRedirect()
	d.Set(true, ":8443")
	h := d.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true }))

	// Plain, and nothing in front: redirected, which is the point of the door.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("a plain request was not redirected: %d", rec.Code)
	}

	// Terminated in front: the browser IS on https, so this must be served.
	served = false
	// The load balancer is a private peer: httptest's default RemoteAddr is a
	// public documentation address, whose X-Forwarded-Proto is not believed.
	req := httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil)
	req.RemoteAddr = "10.0.0.2:5000"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !served {
		t.Fatalf("a TLS-terminated request was redirected again (%d -> %q): that is the loop",
			rec.Code, rec.Header().Get("Location"))
	}

	// The same header from a stranger reaching the plain port directly: it does
	// not talk the gateway out of the redirect.
	served = false
	req = httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil)
	req.RemoteAddr = "203.0.113.9:4000"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if served || rec.Code != http.StatusMovedPermanently {
		t.Fatalf("a public peer's X-Forwarded-Proto skipped the redirect: %d", rec.Code)
	}
}
