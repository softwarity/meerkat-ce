package gateway

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
)

// The authorities an upstream in HTTPS may be signed by (SSL-09): the image's
// own, plus the ones in the store. In addition to the system's, never instead:
// adding a cluster's CA must not stop a public upstream from verifying.

// upstreamRoots is the pool the upstream transports verify with; nil is the
// system's alone (no authority stored).
var upstreamRoots atomic.Pointer[x509.CertPool]

// trustState is the fingerprint of the pool the last reload installed, so an
// unchanged read changes nothing.
type trustState struct {
	mu  sync.Mutex
	sum string
}

// loadTrust reads every stored authority. The PEM was read when it was saved,
// so one that does not read now is a row somebody wrote by hand: it is left
// out, said in the log, and the others still count.
func (rt *Router) loadTrust(ctx context.Context) (*x509.CertPool, string, bool) {
	list, err := rt.st.ListTrustedAuthorities(ctx)
	if err != nil {
		slog.Error("trusted authorities not read, the upstreams verify with the previous set", "err", err)
		return nil, "", false
	}
	if len(list) == 0 {
		return nil, "", true
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	h := sha256.New()
	for _, a := range list {
		found, err := certs.ParseAuthorities([]byte(a.PEM))
		if err != nil {
			slog.Warn("a trusted authority does not count", "name", a.Name, "why", err)
			continue
		}
		for _, c := range found {
			pool.AddCert(c)
			_, _ = h.Write(c.Raw)
		}
	}
	return pool, hex.EncodeToString(h.Sum(nil)), true
}

// applyTrust installs what loadTrust read. A change drops the cached
// transports, so the routes compiled next verify with the new pool; a
// connection already open stays on the one it verified with.
func (rt *Router) applyTrust(ctx context.Context) {
	pool, sum, ok := rt.loadTrust(ctx)
	if !ok {
		return
	}
	rt.trust.mu.Lock()
	changed := sum != rt.trust.sum
	rt.trust.sum = sum
	rt.trust.mu.Unlock()
	if changed {
		setUpstreamRoots(pool)
	}
}

// setUpstreamRoots swaps the pool and drops the transports built on the old
// one, closing their idle connections.
func setUpstreamRoots(pool *x509.CertPool) {
	transportsMu.Lock()
	defer transportsMu.Unlock()
	for _, t := range transports {
		if c, ok := t.(interface{ CloseIdleConnections() }); ok {
			c.CloseIdleConnections()
		}
	}
	transports = map[[2]time.Duration]http.RoundTripper{}
	upstreamRoots.Store(pool)
}

// upstreamTLS is the client side of an upstream connection in HTTPS.
func upstreamTLS() *tls.Config {
	return &tls.Config{RootCAs: upstreamRoots.Load(), MinVersion: tls.VersionTLS12}
}
