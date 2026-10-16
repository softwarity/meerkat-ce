package oidcprovider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The provider lives at the issuer's origin, and only an address can be one.
func TestIssuerOrigin(t *testing.T) {
	for issuer, want := range map[string]string{
		"https://gw.example.com":        "https://gw.example.com",
		"https://gw.example.com:8443/x": "https://gw.example.com:8443",
		"http://localhost:8080":         "http://localhost:8080",
		"http://127.0.0.1:18080":        "http://127.0.0.1:18080",
	} {
		got, err := IssuerOrigin(issuer)
		if err != nil || got != want {
			t.Errorf("%s: got %q %v, want %q", issuer, got, err, want)
		}
	}
	for _, issuer := range []string{"meerkat", "http://gw.example.com", "", "ftp://gw.example.com"} {
		if _, err := IssuerOrigin(issuer); !errors.Is(err, ErrIssuerNotURL) {
			t.Errorf("%q: got %v, want the sentence naming where to declare a public name", issuer, err)
		}
	}
}

// Without an implementation, the provider is there and does nothing: the
// paths stay the routes'.
func TestAnAbsentProviderMountsNothing(t *testing.T) {
	if Linked() {
		t.Skip("a provider is linked into this test binary")
	}
	p := New(Deps{})
	mux := http.NewServeMux()
	p.MountData(mux)
	p.MountInternal(mux)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	rec := httptest.NewRecorder()
	p.WrapData(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, DiscoveryPath, nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("the discovery path did not fall through: %d", rec.Code)
	}
	if p.Active(context.Background()) {
		t.Fatal("an absent provider says it is active")
	}
	if _, ok := p.Endpoints(context.Background(), "http://x"); ok {
		t.Fatal("an absent provider names endpoints")
	}
}
