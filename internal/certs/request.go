package certs

import (
	"context"
	"crypto/tls"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
)

// Asking the authority when the order is placed, not at the first visit.
//
// autocert asks on a handshake: nothing leaves until somebody opens the page,
// and the first person to do so waits through the whole exchange - or meets
// its failure, with nobody on the screen that could have read why. So once an
// order is placed on a door, the gateway asks at once, in the background, as
// if a browser had come by; the pool shows Requesting, then the certificate,
// or the authority's own reason it refused.
//
// The state is this node's: the node that asked is the one that knows how it
// went. A certificate issued is in the shared cache, which every node reads,
// so what matters - the certificate - is never node-local.

// Issue states.
const (
	IssueRequesting = "requesting"
	IssueFailed     = "failed"
)

// IssueStatus is the last thing that happened to one name of one authority.
type IssueStatus struct {
	State string `json:"state"`
	Error string `json:"error,omitempty"`
	At    int64  `json:"at"`
}

type issueBook struct {
	mu sync.Mutex
	by map[string]IssueStatus
}

func issueKey(authority, name string) string { return authority + "|" + strings.ToLower(name) }

func (b *issueBook) get(key string) (IssueStatus, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.by[key]
	return st, ok
}

// claim marks a name as being asked, unless something already happened to it:
// one request in flight per name, and a failure stays until somebody retries.
func (b *issueBook) claim(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.by == nil {
		b.by = map[string]IssueStatus{}
	}
	if _, seen := b.by[key]; seen {
		return false
	}
	b.by[key] = IssueStatus{State: IssueRequesting, At: time.Now().Unix()}
	return true
}

func (b *issueBook) set(key string, st IssueStatus, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ok {
		delete(b.by, key)
		return
	}
	b.by[key] = st
}

func (b *issueBook) forget(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.by, key)
}

// Issue reports what happened to a name asked of an authority, on this node.
// Nothing reported: it was issued, or never asked yet.
func (s *Supervisor) Issue(authority, name string) (IssueStatus, bool) {
	return s.issues.get(issueKey(authority, name))
}

// Retry forgets a failure and asks again.
func (s *Supervisor) Retry(ctx context.Context, authority string, names []string) {
	for _, n := range names {
		s.issues.forget(issueKey(authority, n))
	}
	s.request(s.src.TLSSettings(ctx))
}

// request asks for every name placed on a door that the cache does not hold
// and nobody is already asking for. Called after every reload: placing an
// order IS asking for it.
func (s *Supervisor) request(cfg Settings) {
	if !edition.Enterprise {
		return
	}
	for _, o := range cfg.ACME.Orders {
		auth, ok := cfg.ACME.AuthorityByID(o.Authority)
		if !ok || (!o.Console && !o.App) {
			continue
		}
		door := s.App
		if !o.App {
			door = s.Console
		}
		cache := PrefixedCache{C: s.cache, Prefix: auth.CachePrefix}
		for _, name := range lower(o.Names) {
			if _, held := CachedInfo(context.Background(), cache, name); held {
				continue
			}
			key := issueKey(auth.ID, name)
			if !s.issues.claim(key) {
				continue
			}
			go s.ask(door, key, name, orName(auth))
		}
	}
}

// ask runs one handshake's worth of issuance through the door's own path -
// the cluster lock included - with a hello that says what a browser says: it
// takes ECDSA, so the certificate is the one browsers will be served.
func (s *Supervisor) ask(door *Manager, key, name, authority string) {
	hello := &tls.ClientHelloInfo{
		ServerName:        name,
		SupportedProtos:   []string{"h2", "http/1.1"},
		SignatureSchemes:  []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256, tls.PSSWithSHA256},
		SupportedCurves:   []tls.CurveID{tls.X25519, tls.CurveP256},
		CipherSuites:      []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, tls.TLS_AES_128_GCM_SHA256},
		SupportedVersions: []uint16{tls.VersionTLS13, tls.VersionTLS12},
	}
	slog.Info("asking the authority", "authority", authority, "host", name)
	_, err := door.GetCertificate(hello)
	if err != nil {
		slog.Warn("the authority refused", "authority", authority, "host", name, "err", err)
		s.issues.set(key, IssueStatus{State: IssueFailed, Error: explain(name, err), At: time.Now().Unix()}, false)
		return
	}
	slog.Info("certificate issued", "authority", authority, "host", name)
	s.issues.set(key, IssueStatus{}, true)
}

// explain turns autocert's refusals into what an operator can act on. Its
// own words are kept after the explanation, because they are what a search
// or the authority's support will recognise.
func explain(name string, err error) string {
	raw := err.Error()
	switch {
	case strings.Contains(raw, "no viable challenge type found"):
		// autocert tried the TLS-ALPN challenge, the authority could not
		// complete it, and the reason it gave is lost on the way: what is
		// left is the list of things that make it fail.
		return "the authority could not check " + name + ": it connects to that name on port 443 and must find this gateway there - " +
			"a DNS record pointing to the address that reaches it (DNS only, not proxied, at Cloudflare), port 443 forwarded to " +
			"this gateway's HTTPS port, and no CAA record reserving the domain for another authority (" + raw + ")"
	case strings.Contains(raw, "rateLimited"):
		return "the authority's rate limit is reached for this domain: wait for it to reset, or test with the staging authority (" + raw + ")"
	case strings.Contains(raw, "caa"), strings.Contains(raw, "CAA"):
		return "a CAA record of the domain does not allow this authority to issue for it (" + raw + ")"
	case strings.Contains(raw, "externalAccountRequired"):
		return "the authority requires an account binding: fill in its key ID and HMAC key (" + raw + ")"
	}
	return raw
}
