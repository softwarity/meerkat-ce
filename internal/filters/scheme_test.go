package filters

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Behind a load balancer that terminates TLS, r.TLS is nil on EVERY request.
// Six places decided the scheme on that field alone, and all six were wrong at
// once in exactly the deployment this product is built for.
func TestTerminatedTLSIsStillTLS(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil)
	if Secure(plain) {
		t.Error("a plain request was called secure")
	}
	if got := Origin(plain); got != "http://gw.example" {
		t.Errorf("origin = %q", got)
	}

	direct := httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil)
	direct.TLS = &tls.ConnectionState{}
	if !Secure(direct) {
		t.Error("a request that arrived over TLS was not called secure")
	}

	// The terminated case: no TLS on this hop, the header says there was one.
	fronted := httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil)
	fronted.Header.Set("X-Forwarded-Proto", "https")
	if !Secure(fronted) {
		t.Error("a TLS-terminated request was called plain")
	}
	if got := Origin(fronted); got != "https://gw.example" {
		t.Errorf("origin = %q, want the address the BROWSER used", got)
	}

	// Only https counts. "http" said out loud does not undo a real handshake.
	lying := httptest.NewRequest(http.MethodGet, "http://gw.example/x", nil)
	lying.TLS = &tls.ConnectionState{}
	lying.Header.Set("X-Forwarded-Proto", "http")
	if !Secure(lying) {
		t.Error("a header talked a real TLS connection out of being one")
	}
}
