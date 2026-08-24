package certs

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

// Sending plaintext to the HTTPS door (SSL-06).
//
// Two ports, and they have different jobs: one serves, the other tells the
// caller where to go. That is the shape every web server has settled on, and
// the reason is legibility - a port whose protocol you cannot name is a port
// nobody can write a firewall rule, a health check or a runbook against.
//
// This applies to the APPLICATION plane only. The console's plain port is
// never redirected: it is what a broken certificate gets repaired from, so it
// must never be what the breakage takes down. With two ports that is not a
// rule to defend, it is simply a door that was never wired to move.

// Redirect answers plaintext with the https form of the same request. It is
// flipped by the supervisor, so it holds its state behind a lock.
type Redirect struct {
	mu sync.RWMutex
	on bool
	// to is the HTTPS listen address the caller is sent to. The port has to be
	// swapped, not kept: the request arrived on the plain one.
	to string
}

// NewRedirect builds an inactive redirector.
func NewRedirect() *Redirect { return &Redirect{} }

// Set arms or disarms it, and says which address to point at.
func (d *Redirect) Set(on bool, tlsAddr string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.on, d.to = on, tlsAddr
}

func (d *Redirect) state() (bool, string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.on, d.to
}

// Wrap puts the redirector in front of a handler.
//
// The status follows the method, and that is not pedantry: a 301 makes clients
// re-issue a POST as a GET, so an API caller reaching the plain port would
// have its body silently dropped. GET and HEAD carry nothing worth preserving
// and take the 301 every client understands; everything else takes 308, which
// preserves method and body.
func (d *Redirect) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		on, to := d.state()
		if !on || r.TLS != nil || exemptFromRedirect(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		target := *r.URL
		target.Scheme = "https"
		target.Host = secureHost(r.Host, to)
		code := http.StatusPermanentRedirect
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			code = http.StatusMovedPermanently
		}
		http.Redirect(w, r, target.String(), code)
	})
}

// secureHost keeps the host the caller used and swaps in the HTTPS port. The
// host is theirs, not ours: a gateway fronts many domains, and answering with
// the one configured here would send half of them somewhere they never asked
// for. The port is ours, because it is the one thing they got wrong.
func secureHost(asked, tlsAddr string) string {
	host := asked
	if h, _, err := net.SplitHostPort(asked); err == nil {
		host = h
	}
	port := tlsAddr
	if i := strings.LastIndex(tlsAddr, ":"); i >= 0 {
		port = tlsAddr[i+1:]
	}
	// 443 is implied by the scheme, and a browser that is handed it back
	// writes it into the address bar for the rest of the session.
	if port == "" || port == "443" {
		return host
	}
	return net.JoinHostPort(host, port)
}

// exemptFromRedirect keeps the liveness probe answering in the clear. A health
// check asks whether the process is alive, not whether it is secured, and it
// is usually a container runtime with a fixed http:// URL that nobody can edit
// without a redeploy.
func exemptFromRedirect(path string) bool { return path == "/healthz" }
