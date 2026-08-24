package certs

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Listener is one HTTPS door that can be opened and closed while the gateway
// runs, without touching the plain-HTTP one beside it.
//
// The order inside Start is Archway's lesson turned into code: bind FIRST,
// and return the failure before anything has changed. There, applying a
// certificate stopped the running server and then tried to start the new one -
// so a taken port or an unreadable keystore left the operator with no HTTPS at
// all and a configuration pointing at something that cannot start.
type Listener struct {
	// Plane is what this door is called in a log line and in an error.
	Plane string
	// Addr is where it binds.
	Addr string

	handler http.Handler
	config  *tls.Config

	mu  sync.Mutex
	srv *http.Server
	ln  net.Listener
}

// NewListener describes a door without opening it.
func NewListener(plane, addr string, handler http.Handler, config *tls.Config) *Listener {
	return &Listener{Plane: plane, Addr: addr, handler: handler, config: config}
}

// Running reports whether the door is currently open.
func (l *Listener) Running() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.srv != nil
}

// Start opens the door. It is idempotent: asking twice is not an error, it is
// what a reconciliation loop does.
func (l *Listener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.srv != nil {
		return nil
	}
	ln, err := net.Listen("tcp", l.Addr)
	if err != nil {
		return fmt.Errorf("cannot open the %s HTTPS port %s: %w", l.Plane, l.Addr, err)
	}
	srv := &http.Server{
		Handler:           l.handler,
		TLSConfig:         l.config,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// HTTP/2 stated rather than inherited. Go negotiates it from ALPN on its
	// own, but only while nothing else touches the protocol set - and this
	// listener does touch it, to carry the ACME challenge protocol.
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	srv.Protocols = protocols

	l.srv, l.ln = srv, ln
	go func() {
		// The certificate and key are empty on purpose: they come from
		// TLSConfig.GetCertificate, one call per handshake, which is what makes
		// a swap invisible to the port.
		if err := srv.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
			slog.Error("https listener stopped", "plane", l.Plane, "addr", l.Addr, "err", err)
		}
	}()
	slog.Info("meerkat https listening", "plane", l.Plane, "addr", l.Addr)
	return nil
}

// Stop closes the door, letting in-flight requests finish.
func (l *Listener) Stop(ctx context.Context) error {
	l.mu.Lock()
	srv := l.srv
	l.srv, l.ln = nil, nil
	l.mu.Unlock()
	if srv == nil {
		return nil
	}
	slog.Info("meerkat https stopping", "plane", l.Plane, "addr", l.Addr)
	return srv.Shutdown(ctx)
}

// Sync reconciles the door with what the configuration asks for.
func (l *Listener) Sync(ctx context.Context, want bool) error {
	if want {
		return l.Start()
	}
	return l.Stop(ctx)
}

// SetConfig points the door at a TLS configuration. It is called before every
// Sync so a door opened later picks up the current one; the configuration
// object itself never changes afterwards, because what varies - the
// certificates - varies behind GetCertificate.
func (l *Listener) SetConfig(c *tls.Config) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.srv == nil {
		l.config = c
	}
}
