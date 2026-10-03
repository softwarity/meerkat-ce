package session

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

// A cookie set over HTTPS and one set in the clear never share a name: a
// browser forbids a plain page to overwrite a Secure cookie, which made
// signing in over HTTP impossible after a session over HTTPS.
func TestTheTwoSchemesNameTheirCookiesApart(t *testing.T) {
	plain := httptest.NewRequest("GET", "http://gw.example/", nil)
	secure := httptest.NewRequest("GET", "https://gw.example/", nil)
	secure.TLS = &tls.ConnectionState{}
	if got := ForScheme("MEERKAT_ADMIN_SESSION", plain); got != "MEERKAT_ADMIN_SESSION" {
		t.Errorf("plain: %q", got)
	}
	if got := ForScheme("MEERKAT_ADMIN_SESSION", secure); got != "__Host-MEERKAT_ADMIN_SESSION" {
		t.Errorf("secure: %q", got)
	}
}
