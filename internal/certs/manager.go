package certs

import (
	"crypto/tls"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Manager holds what is currently served and answers each handshake.
//
// It is the piece that makes Archway's stop-and-restart dance unnecessary:
// tls.Config.GetCertificate is a callback, so replacing a certificate is
// swapping a slice behind a lock. The listener never moves, no connection is
// dropped, and there is no window during which the gateway answers nothing.
type Manager struct {
	mu        sync.RWMutex
	materials []*Material
	fallback  *Material
	acme      *autocert.Manager
	acmeHosts []string
}

// New builds an empty manager: it serves nothing until Set is called, and says
// so rather than presenting a certificate nobody configured.
func New() *Manager { return &Manager{} }

// Set replaces the served material in one gesture.
//
// fallback is what answers a handshake that names no host: a client reaching
// the gateway by IP address sends no SNI, and so does anything older than
// 2010. Without one, those clients get an error naming what IS served, which
// is more useful than a random certificate they will reject anyway.
func (m *Manager) Set(materials []*Material, fallback *Material) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.materials = materials
	m.fallback = fallback
}

// SetACME installs (or removes, with nil) the automatic authority, and the
// closed list of hosts it may ask for.
func (m *Manager) SetACME(am *autocert.Manager, hosts []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acme, m.acmeHosts = am, hosts
}

// Serves reports whether anything at all can answer a handshake. Switching TLS
// on with nothing to present would turn a working port into one that refuses
// every visitor.
func (m *Manager) Serves() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.materials) > 0 || m.fallback != nil || m.acme != nil
}

// Live reports whether anything that can be presented is CURRENTLY VALID.
//
// It is a different question from Serves, and the difference decides one
// thing: whether plaintext may be redirected to https. An expired certificate
// still completes a handshake - the server does not check its own expiry - and
// every client then refuses it. Redirecting to that would close the last door
// an operator has on the same port. An armed authority counts as live: it
// fetches on demand, so what it has not fetched yet is not a fault.
func (m *Manager) Live(now time.Time) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.acme != nil {
		return true
	}
	for _, mat := range m.materials {
		if live(mat, now) {
			return true
		}
	}
	return false
}

// GetCertificate is the TLS callback. The order it tries things in is the
// whole design, so it is worth reading top to bottom.
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	materials, fallback, am, hosts := m.materials, m.fallback, m.acme, m.acmeHosts
	m.mu.RUnlock()

	// FIRST, before anything else: a TLS-ALPN-01 challenge. The authority
	// connects asking for the very domain we already hold a certificate for,
	// and expects the challenge certificate rather than the real one. Answering
	// with the real one here is how a renewal silently stops working weeks
	// before the expiry nobody is watching.
	if am != nil && slices.Contains(hello.SupportedProtos, acme.ALPNProto) {
		return am.GetCertificate(hello)
	}

	name := strings.ToLower(strings.TrimSuffix(hello.ServerName, "."))
	if name != "" {
		if c := pick(materials, name, time.Now()); c != nil {
			return c.TLS(), nil
		}
		if am != nil && slices.Contains(hosts, name) {
			return am.GetCertificate(hello)
		}
	}
	if fallback != nil {
		return fallback.TLS(), nil
	}
	// An error that names what failed and lists what is allowed - a handshake
	// failure is otherwise the most opaque event in an operator's day.
	return nil, fmt.Errorf("no certificate for %s: %s", named(name), served(materials, hosts))
}

// pick chooses among the certificates that can answer for name. A currently
// valid one always beats an expired one, and among valid ones the one that
// lasts longest wins: during the overlap of a renewal both are present, and
// serving the older one would be a countdown to an outage.
func pick(materials []*Material, name string, now time.Time) *Material {
	var best *Material
	for _, m := range materials {
		if m.Leaf().VerifyHostname(name) != nil {
			continue
		}
		if best == nil {
			best = m
			continue
		}
		bestLive := live(best, now)
		mLive := live(m, now)
		switch {
		case mLive && !bestLive:
			best = m
		case mLive == bestLive && m.Info.NotAfter > best.Info.NotAfter:
			best = m
		}
	}
	return best
}

func live(m *Material, now time.Time) bool {
	t := now.Unix()
	return m.Info.NotBefore <= t && t < m.Info.NotAfter
}

func named(name string) string {
	if name == "" {
		return "a client that sent no server name (it reached the gateway by IP address, or is very old)"
	}
	return fmt.Sprintf("%q", name)
}

func served(materials []*Material, acmeHosts []string) string {
	var all []string
	for _, m := range materials {
		all = append(all, m.Names()...)
	}
	slices.Sort(all)
	all = slices.Compact(all)
	switch {
	case len(all) == 0 && len(acmeHosts) == 0:
		return "no certificate is installed"
	case len(all) == 0:
		return "the only names an authority may be asked for are " + strings.Join(acmeHosts, ", ")
	case len(acmeHosts) == 0:
		return "the installed certificates serve " + strings.Join(all, ", ")
	}
	return "the installed certificates serve " + strings.Join(all, ", ") +
		", and an authority may be asked for " + strings.Join(acmeHosts, ", ")
}

// TLSConfig is the configuration a listener runs on.
//
// NextProtos carries h2 explicitly: HTTP/2 over TLS is what every browser
// prefers, it costs nothing here, and leaving the field to chance is how a
// gateway ends up serving HTTP/1.1 to everyone without anyone noticing. The
// ACME protocol joins the list when an authority is configured, because the
// challenge arrives on this same listener.
func (m *Manager) TLSConfig() *tls.Config {
	m.mu.RLock()
	withACME := m.acme != nil
	m.mu.RUnlock()
	protos := []string{"h2", "http/1.1"}
	if withACME {
		protos = append(protos, acme.ALPNProto)
	}
	return &tls.Config{
		GetCertificate: m.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		NextProtos:     protos,
	}
}

// Covers reports whether a certificate carrying these names answers for host.
//
// It works from the NAMES a row already stores rather than re-parsing the
// certificate, so a screen can answer "is this host covered" for twenty rows
// without twenty DER decodings. The rule is the one x509 applies: an exact
// match, or a wildcard that stands for exactly one label - *.example.com
// covers app.example.com and not example.com, nor a.b.example.com.
func Covers(names []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return false
	}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(n), "."))
		if n == host {
			return true
		}
		suffix, ok := strings.CutPrefix(n, "*.")
		if !ok {
			continue
		}
		rest, found := strings.CutSuffix(host, "."+suffix)
		if found && rest != "" && !strings.Contains(rest, ".") {
			return true
		}
	}
	return false
}
