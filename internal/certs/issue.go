package certs

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// Asking the authority once, not once per gateway (STORE-03, PERF-03).
//
// autocert already serialises issuance INSIDE one process: two handshakes for
// the same name wait on the same in-flight order. Across processes it has no
// idea. So N nodes armed with the same authority, meeting their first
// handshake for a name at the same moment - a rollout, a restart, a health
// check hitting every replica - order N certificates for it. Let's Encrypt
// allows five duplicates a week: five nodes spend the week's allowance in one
// second, and the bill arrives days later as a name that will not renew.
//
// The fix is one cluster-wide lock around the order. The node that waits then
// finds the certificate the winner has just written to the SHARED cache
// (StoreCache, in the database for exactly this reason) and asks the authority
// for nothing at all.
//
// What it must NOT do is take that lock on every handshake: a round trip to
// the database in front of each one would be a cost paid forever to avoid an
// event that happens twice a year. So the manager remembers when the
// certificate it last served expires, and reaches for the lock only when it
// has none or when renewal is due.

// Serialiser runs fn while holding a lock that every gateway on this database
// respects. It is (*store.Store).WithLock, wired by main; nil where there is
// one node, which is every single-binary installation.
type Serialiser func(ctx context.Context, name string, fn func(context.Context) error) error

// SerialiseIssuance installs the lock. Called once at wiring time.
func (m *Manager) SerialiseIssuance(fn Serialiser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serialise = fn
}

// defaultRenewBefore mirrors autocert's own default: it renews when a
// certificate has less than 30 days left. Reading the manager's field when it
// is set keeps the two in step if an operator ever moves it.
const defaultRenewBefore = 30 * 24 * time.Hour

// issue answers a handshake for a name the authority may be asked for.
func (m *Manager) issue(hello *tls.ClientHelloInfo, am *autocert.Manager, serialise Serialiser, name string) (*tls.Certificate, error) {
	if serialise == nil || m.holds(name, renewBefore(am)) {
		return am.GetCertificate(hello)
	}
	// crypto/tls always sets one; a hand-built hello (a test, a future caller)
	// may not, and handing a nil context to a database call is a panic in the
	// middle of a handshake.
	ctx := hello.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	var cert *tls.Certificate
	var inner error
	err := serialise(ctx, "acme:"+name, func(context.Context) error {
		cert, inner = am.GetCertificate(hello)
		return inner
	})
	if inner != nil {
		return nil, inner
	}
	if err != nil {
		// The lock itself failed - the database is unreachable, say. Ordering
		// a duplicate certificate is a worse outcome than not serving, but
		// only just: refusing every handshake because a lock could not be
		// taken would turn a database blip into an outage on a name that may
		// already be in the cache. So it is tried anyway, loudly.
		slog.Error("could not take the issuance lock: another gateway may order the same certificate",
			"host", name, "err", err)
		return am.GetCertificate(hello)
	}
	m.remember(name, cert)
	return cert, nil
}

// holds reports whether this process already served a certificate for name
// that is not yet due for renewal - in which case autocert will answer from
// its own memory and there is nothing to serialise.
func (m *Manager) holds(name string, before time.Duration) bool {
	v, ok := m.issued.Load(name)
	if !ok {
		return false
	}
	// One hour of margin so the switch to the locked path happens BEFORE
	// autocert decides to renew, rather than during the same handshake.
	return time.Until(v.(time.Time)) > before+time.Hour
}

func (m *Manager) remember(name string, cert *tls.Certificate) {
	if cert == nil || cert.Leaf == nil {
		return
	}
	m.issued.Store(name, cert.Leaf.NotAfter)
}

func renewBefore(am *autocert.Manager) time.Duration {
	if am.RenewBefore > 0 {
		return am.RenewBefore
	}
	return defaultRenewBefore
}

// issuedAt is the type of Manager.issued, kept here so the field reads as what
// it is: name -> when the certificate this process last served for it expires.
type issuedAt = sync.Map
