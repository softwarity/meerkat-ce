package certs

import (
	"context"
	"crypto/tls"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// A cache that already holds the certificate, which is how these tests reach
// autocert's answer without an authority: the order is the one thing that must
// not happen twice, and it is also the one thing a test cannot make happen.
type warmCache map[string][]byte

func (c warmCache) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := c[key]
	if !ok {
		return nil, autocert.ErrCacheMiss
	}
	return v, nil
}
func (c warmCache) Put(_ context.Context, key string, data []byte) error { c[key] = data; return nil }
func (c warmCache) Delete(_ context.Context, key string) error           { delete(c, key); return nil }

// armed builds a manager whose authority can answer for name from the cache.
//
// The key type is not incidental: autocert stores the ECDSA certificate under
// the bare host name and an RSA one under "<host>+rsa", and it rejects a
// cached certificate whose key does not match what the handshake can use. Get
// either wrong and the cache misses - at which point the test reaches for a
// real authority over the network, which is why the client below points at
// nothing.
func armed(t *testing.T, name string) (*Manager, *autocert.Manager) {
	t.Helper()
	mat := mustSelfSigned(t, Request{DNSNames: []string{name}, KeyType: KeyECDSAP256, Days: 90})
	am := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      warmCache{name: []byte(mat.KeyPEM + mat.CertPEM)},
		HostPolicy: autocert.HostWhitelist(name),
		// A directory that cannot be reached. A unit test must never be able
		// to order a certificate from a real authority - not by accident, not
		// on somebody's laptop, and certainly not from CI - so the fallback
		// path fails at once instead of succeeding somewhere it should not.
		Client: &acme.Client{DirectoryURL: "https://127.0.0.1:1/acme/directory"},
	}
	m := New()
	m.SetACME(am, []string{name})
	return m, am
}

// ecdsaHello is a handshake autocert will answer with an ECDSA certificate.
//
// It matters because autocert files the two key types under different cache
// keys - "host" and "host+rsa" - and decides which by reading the hello. A
// hello that does not clearly support ECDSA misses the cache and goes looking
// for an authority, which is the opposite of what these tests are about.
func ecdsaHello(name string, protos ...string) *tls.ClientHelloInfo {
	h := hello(name, protos...)
	h.SignatureSchemes = []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256}
	h.SupportedCurves = []tls.CurveID{tls.CurveP256}
	h.CipherSuites = []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, tls.TLS_AES_128_GCM_SHA256}
	return h
}

// The lock is taken for the ORDER, and the order happens once. Taking it on
// every handshake would put a database round trip in front of each one, which
// is a cost paid forever to avoid an event that happens twice a year.
func TestIssuanceLocksOnceAndThenStopsAsking(t *testing.T) {
	m, _ := armed(t, "app.example.com")

	var taken atomic.Int64
	var named string
	m.SerialiseIssuance(func(ctx context.Context, name string, fn func(context.Context) error) error {
		taken.Add(1)
		named = name
		return fn(ctx)
	})

	if _, err := m.GetCertificate(ecdsaHello("app.example.com")); err != nil {
		t.Fatalf("first handshake: %v", err)
	}
	if taken.Load() != 1 {
		t.Fatalf("the first handshake took the lock %d times, want 1", taken.Load())
	}
	if named != "acme:app.example.com" {
		t.Errorf("locked on %q: the name has to be per host, or two certificates queue for each other", named)
	}

	for range 5 {
		if _, err := m.GetCertificate(ecdsaHello("app.example.com")); err != nil {
			t.Fatalf("later handshake: %v", err)
		}
	}
	if n := taken.Load(); n != 1 {
		t.Errorf("the lock was taken %d times for one certificate: every handshake pays a round trip", n)
	}
}

// The challenge is the authority calling US, in the middle of the order the
// lock is being held for. Serialising it would be the gateway waiting for
// itself.
func TestTheChallengeNeverWaitsForTheLock(t *testing.T) {
	m, _ := armed(t, "app.example.com")
	m.SerialiseIssuance(func(context.Context, string, func(context.Context) error) error {
		t.Error("the TLS-ALPN challenge took the issuance lock: that is a gateway waiting for itself")
		return nil
	})
	// It fails - there is no challenge in flight - and that is fine: what is
	// being tested is which path it took, and the serialiser above would have
	// spoken.
	_, _ = m.GetCertificate(ecdsaHello("app.example.com", acme.ALPNProto))
}

// A single-binary gateway has nobody to race, and must not need a lock to
// serve. Same code path, no serialiser wired.
func TestWithNoLockItStillServes(t *testing.T) {
	m, _ := armed(t, "app.example.com")
	got, err := m.GetCertificate(ecdsaHello("app.example.com"))
	if err != nil {
		t.Fatalf("a gateway with no cluster could not answer: %v", err)
	}
	if got == nil || got.Leaf == nil || got.Leaf.NotAfter.Before(time.Now()) {
		t.Fatal("no usable certificate came back")
	}
}
