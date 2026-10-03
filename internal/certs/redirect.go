package certs

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/softwarity/meerkat/internal/filters"
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
	// hsts is the Strict-Transport-Security max-age stamped on HTTPS answers,
	// 0 for none.
	hsts int
	// published maps an inside port to the one the world reaches it on (a
	// Service publishing 8443 as 8444): the caller is sent where it can go,
	// not where the container listens. Nil: the inside port.
	published func(inside int) int
}

// SetPublished says where the runtime publishes each inside port.
func (d *Redirect) SetPublished(f func(inside int) int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.published = f
}

// SetHSTS says how long browsers are told to use HTTPS only, 0 for not at all.
func (d *Redirect) SetHSTS(seconds int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hsts = seconds
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
		if filters.Secure(r) {
			// Over HTTPS: served, and told to stay here (SSL-06). Never on
			// plain HTTP - a browser ignores it there, and a proxy that
			// believed it would be believing a request anyone could forge.
			//
			// ONLY ON 443. A browser applies HSTS to every port of a name, and
			// when it switches a request to HTTPS it keeps the port (80
			// becoming 443 is the one exception). A promise made on 8443 sends
			// http://name:8080 to https://name:8080 - a plain port - and every
			// address in the clear under that name, the console's included,
			// stops answering. Off 443 the redirect alone does the work, on
			// every visit.
			//
			// And everywhere else, max-age=0: the standard way to make a
			// browser FORGET a promise made earlier - when Force HTTPS is
			// switched off, or by a gateway that used to make it on 8443.
			// Without it the browser keeps refusing plain HTTP for the whole
			// length it was told, whatever the setting says now.
			d.mu.RLock()
			hsts := d.hsts
			d.mu.RUnlock()
			if hstsHost(r.Host) {
				value := "max-age=0"
				if hsts > 0 && standardHTTPS(r.Host) {
					value = "max-age=" + strconv.Itoa(hsts)
				}
				w = &hstsWriter{ResponseWriter: w, value: value}
			}
			next.ServeHTTP(w, r)
			return
		}
		if !on || exemptFromRedirect(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		target := *r.URL
		target.Scheme = "https"
		target.Host = secureHost(r.Host, d.publishedAddr(to))
		code := http.StatusPermanentRedirect
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			code = http.StatusMovedPermanently
		}
		http.Redirect(w, r, target.String(), code)
	})
}

// publishedAddr is the HTTPS address with the port the world reaches it on.
func (d *Redirect) publishedAddr(tlsAddr string) string {
	d.mu.RLock()
	f := d.published
	d.mu.RUnlock()
	if f == nil {
		return tlsAddr
	}
	i := strings.LastIndex(tlsAddr, ":")
	inside, err := strconv.Atoi(tlsAddr[i+1:])
	if err != nil {
		return tlsAddr
	}
	if out := f(inside); out > 0 {
		return ":" + strconv.Itoa(out)
	}
	return tlsAddr
}

// standardHTTPS says the caller reached HTTPS on 443, the one port a browser's
// HSTS upgrade lands on.
func standardHTTPS(hostport string) bool {
	_, port, err := net.SplitHostPort(hostport)
	return err != nil || port == "443"
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

// exemptFromRedirect keeps BOTH probes answering in the clear. A health check
// asks whether the process is alive, or whether it may take traffic - not
// whether it is secured - and it is usually a container runtime with a fixed
// http:// URL that nobody can edit without a redeploy. Redirecting readiness
// would make an orchestrator read a 308 as "not ready" and take the node out
// of rotation for having TLS on.
func exemptFromRedirect(path string) bool { return path == "/healthz" || path == "/readyz" }

// hstsWriter stamps Strict-Transport-Security as the answer leaves, and only
// when nobody set it first. A service that sends its own - or a route's
// security-headers filter - knows its application better than a gateway-wide
// default, and two of the header is a promise the browser reads the first of.
//
// It forwards Flush, and Unwrap for http.ResponseController: a wrapper that
// swallowed either would turn server-sent events into silence and a websocket
// upgrade into a 500.
type hstsWriter struct {
	http.ResponseWriter
	value   string
	stamped bool
}

func (w *hstsWriter) stamp() {
	if w.stamped {
		return
	}
	w.stamped = true
	if w.Header().Get("Strict-Transport-Security") == "" {
		w.Header().Set("Strict-Transport-Security", w.value)
	}
}

func (w *hstsWriter) WriteHeader(code int) {
	w.stamp()
	w.ResponseWriter.WriteHeader(code)
}

func (w *hstsWriter) Write(b []byte) (int, error) {
	w.stamp()
	return w.ResponseWriter.Write(b)
}

func (w *hstsWriter) Flush() {
	w.stamp()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *hstsWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// hstsHost says whether a host may be sent HSTS. Never localhost: the promise
// is per host name and ALL its ports, so a development gateway on
// localhost:8443 would force HTTPS on every other application its developer
// runs on localhost - and they would stop answering. Never an IP address:
// browsers ignore it there anyway.
func hstsHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	return net.ParseIP(host) == nil
}
