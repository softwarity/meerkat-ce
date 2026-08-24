package certs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// Settings is the TLS configuration as an operator sets it: which doors are
// open, and whether an authority issues certificates on its own.
type Settings struct {
	// Data opens the application plane's HTTPS door.
	Data bool `json:"data"`
	// Admin opens HTTPS on the console plane. It is a separate switch because
	// the two ports are often published differently - but it is not a lesser
	// one: the same session manager sets the cookie's Secure flag from
	// r.TLS != nil, so an admin plane in the clear is an admin session cookie
	// that cannot be marked secure.
	Admin bool `json:"admin"`
	// Redirect sends the APPLICATION plane's plaintext port to its HTTPS one
	// (SSL-06). The console's plain port is never redirected - see redirect.go.
	Redirect bool `json:"redirect"`
	// The names each plane answers to, declared rather than guessed.
	//
	// They are two lists and not one because the two planes are not alike: a
	// console has ONE name, an application gateway fronts MANY - that is its
	// job. Flattening them into a single list leaves an operator to work out
	// in their head which certificate covers which door, which is exactly the
	// arithmetic a screen should be doing for them.
	//
	// Nothing enforces them: they are what "does this work?" is answered
	// against, and what a generation form is pre-filled from.
	ConsoleNames []string     `json:"consoleNames,omitempty"`
	AppNames     []string     `json:"appNames,omitempty"`
	ACME         ACMESettings `json:"acme"`
}

// ACMESettings is the automatic authority. Every field that makes it work
// offline is here on purpose: the directory is a URL, the root is a PEM, and
// the external account binding is what a corporate authority demands.
type ACMESettings struct {
	Enabled      bool   `json:"enabled"`
	DirectoryURL string `json:"directoryUrl"`
	Email        string `json:"email"`
	// Domains is the closed set the authority may be asked for. It is not a
	// third list to keep in step with the two above: the console ticks a box
	// on a declared name, and that is what puts it here. One authority, one
	// account - splitting ACME per plane would register twice with the same
	// authority for the same machine and double the exposure to its quotas,
	// for nothing.
	Domains    []string `json:"domains"`
	RootCA     string   `json:"rootCa"`
	EABKeyID   string   `json:"eabKeyId"`
	EABHMACKey string   `json:"eabHmacKey"`
	AcceptTOS  bool     `json:"acceptTos"`
}

// Source is what a supervisor reads from. It is an interface so this package
// stays free of the store, which already depends on it.
type Source interface {
	CacheStore
	CertificateMaterials(ctx context.Context) ([]*Material, *Material, []string, error)
	TLSSettings(ctx context.Context) Settings
}

// State is what the supervisor last managed to do, for the console to show.
type State struct {
	// Data and Admin are the HTTPS doors that are actually OPEN, which is not
	// the same as the ones that were asked for: a port already taken says no,
	// and switching HTTPS on with nothing to present is refused rather than
	// obeyed.
	Data  bool `json:"data"`
	Admin bool `json:"admin"`
	// ACME says an authority is configured and armed.
	ACME bool `json:"acme"`
	// Problems are the certificates that could not be loaded and the
	// authority that could not be built, each named. They are shown rather
	// than logged and forgotten: a certificate silently dropped at boot is
	// found months later, by an outage.
	Problems []string `json:"problems"`
	// DataAddr and AdminAddr are where the two HTTPS doors bind, so the console
	// can name the port to visit rather than making an operator guess. The
	// plain ones travel with them: an operator reads the two together, and a
	// screen that shows only half of a pair makes them go and look up the
	// other half in a stack file.
	DataAddr       string `json:"dataAddr"`
	AdminAddr      string `json:"adminAddr"`
	DataPlainAddr  string `json:"dataPlainAddr"`
	AdminPlainAddr string `json:"adminPlainAddr"`
	// Redirecting says the application plane's plain port is currently sending
	// callers to its HTTPS one. It can be false while the setting is true -
	// nothing valid to present stands it down.
	Redirecting bool `json:"redirecting"`
}

// Supervisor keeps the running TLS state equal to the stored one.
type Supervisor struct {
	Manager *Manager

	src      Source
	cache    autocert.Cache
	data     *Listener
	admin    *Listener
	redirect *Redirect

	mu    sync.Mutex
	state State
}

// NewSupervisor wires the manager to the two HTTPS doors and to the
// application plane's redirector. missing recognises the store's "no such
// row", so an ACME cache miss can be told from a real fault.
func NewSupervisor(src Source, missing func(error) bool, data, admin *Listener, redirect *Redirect) *Supervisor {
	s := &Supervisor{
		Manager:  New(),
		src:      src,
		cache:    StoreCache{S: src, Missing: missing},
		data:     data,
		admin:    admin,
		redirect: redirect,
	}
	s.state.DataAddr, s.state.AdminAddr = data.Addr, admin.Addr
	return s
}

// Ports records where the two PLAIN doors listen. They are not the
// supervisor's to manage - main owns them - but they belong on the same screen
// as the HTTPS ones.
func (s *Supervisor) Ports(dataPlain, adminPlain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.DataPlainAddr, s.state.AdminPlainAddr = dataPlain, adminPlain
}

// State reports the last reconciled state.
func (s *Supervisor) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	st.Problems = append([]string{}, s.state.Problems...)
	return st
}

// Reload reads everything and makes the running gateway match it. It is called
// at boot and after every mutation - saving IS applying, here as everywhere.
//
// Certificates are swapped BEFORE the doors are reconciled, so an operator who
// replaces an expiring certificate and turns nothing on or off never sees the
// port move.
func (s *Supervisor) Reload(ctx context.Context) error {
	materials, fallback, problems, err := s.src.CertificateMaterials(ctx)
	if err != nil {
		return err
	}
	cfg := s.src.TLSSettings(ctx)

	armed := false
	if cfg.ACME.Enabled {
		am, aerr := NewACME(ACMEOptions{
			DirectoryURL: cfg.ACME.DirectoryURL,
			Email:        cfg.ACME.Email,
			Hosts:        cfg.ACME.Domains,
			RootCA:       cfg.ACME.RootCA,
			EABKeyID:     cfg.ACME.EABKeyID,
			EABHMACKey:   cfg.ACME.EABHMACKey,
			AcceptTOS:    cfg.ACME.AcceptTOS,
			Cache:        s.cache,
		})
		if aerr != nil {
			// A broken authority must not take the installed certificates down
			// with it: the imported one that was serving yesterday keeps
			// serving, and the problem is named on screen.
			problems = append(problems, "automatic authority: "+aerr.Error())
		} else {
			s.Manager.SetACME(am, lower(cfg.ACME.Domains))
			armed = true
		}
	}
	if !armed {
		s.Manager.SetACME(nil, nil)
	}
	s.Manager.Set(materials, fallback)

	// Asking for HTTPS with nothing to present would bind a port that refuses
	// every visitor. Say it, and leave the door shut.
	want := cfg
	if !s.Manager.Serves() {
		if cfg.Data || cfg.Admin {
			problems = append(problems, "HTTPS is switched on but no certificate is installed and no authority is configured")
		}
		want.Data, want.Admin = false, false
	}
	// Arming is a lock and two pointers: no listener is created, destroyed or
	// rebound, so nothing here can fail on a port that is already taken - the
	// failure mode that made the previous generation's hot swap dangerous
	// simply has no place left to happen.
	// The redirect is the one thing that can strand an application behind a
	// door nobody can open, so it stands down when nothing valid can be
	// presented. The CONSOLE needs no such guard: its plain port was never
	// wired to move.
	redirect := cfg.Redirect
	if redirect && !s.Manager.Live(time.Now()) {
		redirect = false
		problems = append(problems,
			"every installed certificate has expired: the application plane keeps answering in the clear rather than sending callers to a door none of them will open")
	}

	tlsCfg := s.Manager.TLSConfig()
	s.data.SetConfig(tlsCfg)
	s.admin.SetConfig(tlsCfg)

	var failed error
	if err := s.data.Sync(ctx, want.Data); err != nil {
		problems = append(problems, err.Error())
		failed = err
	}
	if err := s.admin.Sync(ctx, want.Admin); err != nil {
		problems = append(problems, err.Error())
		if failed == nil {
			failed = err
		}
	}
	// Redirecting to a door that failed to open would be worse than not
	// redirecting at all.
	redirect = redirect && s.data.Running()
	s.redirect.Set(redirect, s.data.Addr)

	s.mu.Lock()
	s.state.Data, s.state.Admin = s.data.Running(), s.admin.Running()
	s.state.ACME = armed
	s.state.Redirecting = redirect
	s.state.Problems = problems
	s.mu.Unlock()
	for _, p := range problems {
		slog.Warn("tls", "problem", p)
	}
	return failed
}

// Stop closes both HTTPS doors, for a graceful shutdown.
func (s *Supervisor) Stop(ctx context.Context) error {
	err := s.data.Stop(ctx)
	if aerr := s.admin.Stop(ctx); err == nil {
		err = aerr
	}
	return err
}

// Check builds an authority from settings WITHOUT arming it, so the console
// can refuse a broken configuration at save time instead of at renewal time.
func Check(cfg ACMESettings, cache autocert.Cache) error {
	if !cfg.Enabled {
		return nil
	}
	_, err := NewACME(ACMEOptions{
		DirectoryURL: cfg.DirectoryURL,
		Email:        cfg.Email,
		Hosts:        cfg.Domains,
		RootCA:       cfg.RootCA,
		EABKeyID:     cfg.EABKeyID,
		EABHMACKey:   cfg.EABHMACKey,
		AcceptTOS:    cfg.AcceptTOS,
		Cache:        cache,
	})
	if err != nil {
		return fmt.Errorf("automatic authority: %w", err)
	}
	return nil
}

// lower normalises the domain list once, so the manager compares a server name
// against the same spelling every time.
func lower(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out = append(out, v)
		}
	}
	return out
}
