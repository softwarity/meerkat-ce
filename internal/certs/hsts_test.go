package certs

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Strict-Transport-Security, gateway-wide (SSL-06): on every HTTPS answer when
// it is set, never on plain HTTP, never over a value the service or a route
// already chose, and max-age=0 when it is off - which makes a browser forget a
// promise made before, instead of refusing plain HTTP for a day.
func TestHSTSIsStampedOnHTTPSAnswersOnly(t *testing.T) {
	d := NewRedirect()
	plain := d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	own := d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=60")
		w.WriteHeader(http.StatusNoContent)
	}))
	call := func(h http.Handler, secure bool) string {
		r := httptest.NewRequest("GET", "http://app.example/x", nil)
		if secure {
			r.TLS = &tls.ConnectionState{}
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if n := len(rec.Result().Header.Values("Strict-Transport-Security")); n > 1 {
			t.Fatalf("the header was sent %d times", n)
		}
		return rec.Header().Get("Strict-Transport-Security")
	}

	if got := call(plain, true); got != "max-age=0" {
		t.Fatalf("off: want the promise withdrawn, got %q", got)
	}
	d.SetHSTS(86400)
	if got := call(plain, true); got != "max-age=86400" {
		t.Fatalf("over HTTPS: %q", got)
	}
	if got := call(plain, false); got != "" {
		t.Fatalf("over plain HTTP: %q", got)
	}
	if got := call(own, true); got != "max-age=60" {
		t.Fatalf("the service's own value was overridden: %q", got)
	}
}

// Never to localhost - the promise covers every port of a host, so a dev
// gateway would force HTTPS on all its developer's local applications - and
// never to an IP address, which browsers ignore anyway.
func TestHSTSSparesLocalhostAndAddresses(t *testing.T) {
	d := NewRedirect()
	d.SetHSTS(86400)
	h := d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for host, want := range map[string]bool{
		"app.example.com":     true,
		"app.example.com:443": true,
		"localhost:8443":      false,
		"dev.localhost":       false,
		"10.0.0.7:8443":       false,
		"[::1]:8443":          false,
	} {
		r := httptest.NewRequest("GET", "https://"+host+"/", nil)
		r.Host = host
		r.TLS = &tls.ConnectionState{}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if got := rec.Header().Get("Strict-Transport-Security") != ""; got != want {
			t.Errorf("%s: sent=%v, want %v", host, got, want)
		}
	}
}

// HSTS follows the redirect: nothing without it, a day when no length was
// chosen, the chosen length otherwise.
func TestHSTSFollowsTheRedirect(t *testing.T) {
	if got := (Settings{HSTSMaxAge: 3600}).HSTS(false); got != 0 {
		t.Fatalf("without the redirect: %d", got)
	}
	if got := (Settings{}).HSTS(true); got != DefaultHSTS {
		t.Fatalf("no length chosen: %d", got)
	}
	if got := (Settings{HSTSMaxAge: 3600}).HSTS(true); got != 3600 {
		t.Fatalf("a chosen length: %d", got)
	}
}

// A browser keeps the port when HSTS switches it to HTTPS, so a promise made
// on 8443 sends http://name:8080 to https://name:8080, a plain port. Off 443
// the promise is withdrawn instead, and the redirect does the work.
func TestHSTSIsPromisedOnlyOn443(t *testing.T) {
	d := NewRedirect()
	d.SetHSTS(86400)
	h := d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for host, want := range map[string]string{
		"app.example.com":      "max-age=86400",
		"app.example.com:443":  "max-age=86400",
		"app.example.com:8443": "max-age=0",
	} {
		r := httptest.NewRequest("GET", "https://"+host+"/", nil)
		r.Host = host
		r.TLS = &tls.ConnectionState{}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if got := rec.Header().Get("Strict-Transport-Security"); got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
}

// The plain port sends the caller to where the world reaches HTTPS - the
// published port - not to where the container listens.
func TestTheRedirectGoesToThePublishedPort(t *testing.T) {
	d := NewRedirect()
	d.Set(true, ":8443")
	d.SetPublished(func(inside int) int {
		if inside == 8443 {
			return 8444
		}
		return 0
	})
	h := d.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	r := httptest.NewRequest("GET", "http://app.example:8081/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if got := rec.Header().Get("Location"); got != "https://app.example:8444/x" {
		t.Fatalf("redirected to %q", got)
	}
}
