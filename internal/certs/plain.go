package certs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Plain is one plane's plain-HTTP door. Switched off (SSL-15) it does not
// stop: it listens on the loopback only, out of the network's reach, and
// still there for what runs inside the container or reaches it through a
// port-forward - the health check the image runs on itself, and the operator
// repairing a broken certificate. A door that vanished outright would take
// those two with it.
type Plain struct {
	// Plane is what this door is called in a log line and in an error.
	Plane string
	// Addr is where it binds when switched on.
	Addr string

	handler http.Handler
	// OnError is told when the door stops serving for any reason other than
	// being closed: main stops the gateway, as it always did.
	OnError func(error)

	mu       sync.Mutex
	srv      *http.Server
	ln       net.Listener
	loopback bool
}

// NewPlain describes a plain door without opening it.
func NewPlain(plane, addr string, handler http.Handler) *Plain {
	return &Plain{Plane: plane, Addr: addr, handler: handler}
}

// Open binds the door, on every address or on the loopback only. Asking for
// the mode it is already in is a no-op; switching rebinds, and a bind that
// fails puts the previous one back.
func (p *Plain) Open(loopback bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.srv != nil && p.loopback == loopback {
		return nil
	}
	was, wasLoopback := p.srv, p.loopback
	if was != nil {
		p.retire(was, p.ln)
	}
	if err := p.bind(loopback); err != nil {
		if was != nil {
			if rerr := p.bind(wasLoopback); rerr != nil {
				slog.Error("the plain door could not be put back", "plane", p.Plane, "err", rerr)
			}
		}
		return err
	}
	return nil
}

// Switch rebinds a door already open; one main has not opened yet is left
// alone - the gateway opens its ports last, once everything behind them is
// wired, and a reload at boot must not open them early.
func (p *Plain) Switch(loopback bool) error {
	p.mu.Lock()
	open := p.srv != nil
	p.mu.Unlock()
	if !open {
		return nil
	}
	return p.Open(loopback)
}

// Loopback reports whether the door listens on the loopback only.
func (p *Plain) Loopback() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.srv != nil && p.loopback
}

func (p *Plain) bind(loopback bool) error {
	addr := p.Addr
	if loopback {
		_, port, err := net.SplitHostPort(p.Addr)
		if err != nil {
			return fmt.Errorf("the %s plain address %q has no port: %w", p.Plane, p.Addr, err)
		}
		addr = net.JoinHostPort("127.0.0.1", port)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot open the %s plain port %s: %w", p.Plane, addr, err)
	}
	srv := &http.Server{Handler: p.handler, ReadHeaderTimeout: 10 * time.Second}
	p.srv, p.ln, p.loopback = srv, ln, loopback
	go func() {
		err := srv.Serve(ln)
		if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return
		}
		slog.Error("plain listener stopped", "plane", p.Plane, "addr", addr, "err", err)
		if p.OnError != nil {
			p.OnError(err)
		}
	}()
	return nil
}

// retire frees the port at once - it is wanted again right away - and lets
// the requests in flight finish.
func (p *Plain) retire(srv *http.Server, ln net.Listener) {
	_ = ln.Close()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
}

// Shutdown closes the door for good.
func (p *Plain) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	srv := p.srv
	p.srv = nil
	p.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}
