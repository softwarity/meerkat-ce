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

// Planes, matching store.PlaneConsole / store.PlaneApp.
const (
	PlaneConsole = "console"
	PlaneApp     = "app"
)

// Settings is the TLS configuration as an operator sets it.
//
// There is no "switch HTTPS on" here, and that absence is the design: a switch
// that can be on with nothing behind it is a switch that lies. HAVING a
// certificate for a name is what opens that plane's HTTPS door, and deleting
// it is what closes it.
type Settings struct {
	// ConsoleName is the single name the console answers to. One, because a
	// console is one thing reached one way - unlike the application, which
	// fronts as many hosts as it serves.
	ConsoleName string `json:"consoleName"`
	// AppNames are the hosts the application answers to, wildcards included.
	AppNames []string `json:"appNames"`
	// Redirect forces the application's plain port over to HTTPS (SSL-06).
	// The console's plain port is never redirected - see redirect.go.
	Redirect bool         `json:"redirect"`
	ACME     ACMESettings `json:"acme"`
}

// Names returns every declared name, console first.
func (s Settings) Names() []string {
	out := make([]string, 0, len(s.AppNames)+1)
	if n := strings.ToLower(strings.TrimSpace(s.ConsoleName)); n != "" {
		out = append(out, n)
	}
	return append(out, lower(s.AppNames)...)
}

// ACMESettings is the automatic authority. Every field that makes it work
// offline is here on purpose: the directory is a URL, the root is a PEM, and
// the external account binding is what a corporate authority demands.
type ACMESettings struct {
	Enabled      bool   `json:"enabled"`
	DirectoryURL string `json:"directoryUrl"`
	Email        string `json:"email"`
	// Domains is the closed set the authority may be asked for. It is not a
	// list to maintain: the console ticks a box on a declared name, and that
	// is what puts it here. One authority, one account - splitting ACME per
	// plane would register twice with the same authority for the same machine,
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
	CertificateMaterials(ctx context.Context, plane string) ([]*Material, *Material, []string, error)
	TLSSettings(ctx context.Context) Settings
}

// State is what the supervisor last managed to do, for the console to show.
type State struct {
	// Console and App are the HTTPS doors that are actually open. They follow
	// from the certificates, never from a switch.
	Console bool `json:"console"`
	App     bool `json:"app"`
	ACME    bool `json:"acme"`
	// Problems are the certificates that could not be loaded and the authority
	// that could not be built, each named. They are shown rather than logged
	// and forgotten: a certificate silently dropped at boot is found months
	// later, by an outage.
	Problems []string `json:"problems"`
	// The four addresses an operator reads together.
	ConsoleAddr      string `json:"consoleAddr"`
	ConsolePlainAddr string `json:"consolePlainAddr"`
	AppAddr          string `json:"appAddr"`
	AppPlainAddr     string `json:"appPlainAddr"`
	// Redirecting says the application's plain port is currently sending
	// callers to its HTTPS one. It can be false while the setting is true -
	// nothing valid to present stands it down.
	Redirecting bool `json:"redirecting"`
}

// Supervisor keeps the running TLS state equal to the stored one.
//
// One manager per plane, because a certificate now belongs to a plane: the
// console's door never presents an application certificate, and the two can
// hold the same material without sharing an object.
type Supervisor struct {
	Console *Manager
	App     *Manager

	src      Source
	cache    autocert.Cache
	appLn    *Listener
	adminLn  *Listener
	redirect *Redirect

	mu    sync.Mutex
	state State
}

// NewSupervisor wires the two managers to the two HTTPS doors and to the
// application's redirector. missing recognises the store's "no such row", so
// an ACME cache miss can be told from a real fault.
func NewSupervisor(src Source, missing func(error) bool, appLn, adminLn *Listener, redirect *Redirect) *Supervisor {
	s := &Supervisor{
		Console:  New(),
		App:      New(),
		src:      src,
		cache:    StoreCache{S: src, Missing: missing},
		appLn:    appLn,
		adminLn:  adminLn,
		redirect: redirect,
	}
	s.state.AppAddr, s.state.ConsoleAddr = appLn.Addr, adminLn.Addr
	return s
}

// SerialiseIssuance makes certificate ordering one-at-a-time across the
// gateways sharing one database. Wired by main to the store's advisory lock;
// left alone on a single node, where there is nobody to race.
func (s *Supervisor) SerialiseIssuance(fn Serialiser) {
	s.Console.SerialiseIssuance(fn)
	s.App.SerialiseIssuance(fn)
}

// Ports records where the two PLAIN doors listen. They are not the
// supervisor's to manage - main owns them - but they belong on the same screen
// as the HTTPS ones.
func (s *Supervisor) Ports(appPlain, consolePlain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.AppPlainAddr, s.state.ConsolePlainAddr = appPlain, consolePlain
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
func (s *Supervisor) Reload(ctx context.Context) error {
	cfg := s.src.TLSSettings(ctx)

	consoleList, consoleFallback, problems, err := s.src.CertificateMaterials(ctx, PlaneConsole)
	if err != nil {
		return err
	}
	appList, appFallback, appProblems, err := s.src.CertificateMaterials(ctx, PlaneApp)
	if err != nil {
		return err
	}
	problems = append(problems, appProblems...)

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
			// with it: what was serving yesterday keeps serving, and the
			// problem is named on screen.
			problems = append(problems, "automatic authority: "+aerr.Error())
		} else {
			// The authority answers for whichever names were ticked, on
			// whichever plane declared them.
			console, app := splitDomains(cfg)
			s.Console.SetACME(am, console)
			s.App.SetACME(am, app)
			armed = true
		}
	}
	if !armed {
		s.Console.SetACME(nil, nil)
		s.App.SetACME(nil, nil)
	}
	s.Console.Set(consoleList, consoleFallback)
	s.App.Set(appList, appFallback)

	// A door opens because there is something to present. There is no switch
	// to disagree with, so there is no "switched on but nothing installed"
	// state to explain.
	s.adminLn.SetConfig(s.Console.TLSConfig())
	s.appLn.SetConfig(s.App.TLSConfig())

	var failed error
	if err := s.adminLn.Sync(ctx, s.Console.Serves()); err != nil {
		problems = append(problems, err.Error())
		failed = err
	}
	if err := s.appLn.Sync(ctx, s.App.Serves()); err != nil {
		problems = append(problems, err.Error())
		if failed == nil {
			failed = err
		}
	}

	// The redirect is the one thing that can strand an application behind a
	// door nobody can open, so it stands down when nothing valid can be
	// presented, or when the door did not open at all. The CONSOLE needs no
	// such guard: its plain port was never wired to move.
	redirect := cfg.Redirect && s.appLn.Running()
	if redirect && !s.App.Live(time.Now()) {
		redirect = false
		problems = append(problems,
			"every application certificate has expired: the plain port keeps answering rather than sending callers to a door none of them will open")
	}
	s.redirect.Set(redirect, s.appLn.Addr)

	s.mu.Lock()
	s.state.Console, s.state.App = s.adminLn.Running(), s.appLn.Running()
	s.state.ACME = armed
	s.state.Redirecting = redirect
	s.state.Problems = problems
	s.mu.Unlock()
	for _, p := range problems {
		slog.Warn("tls", "problem", p)
	}
	return failed
}

// splitDomains sorts the authority's closed list by the plane that declared
// each name, so the console's door is never handed an application certificate.
func splitDomains(cfg Settings) (console, app []string) {
	want := map[string]bool{}
	for _, d := range lower(cfg.ACME.Domains) {
		want[d] = true
	}
	if n := strings.ToLower(strings.TrimSpace(cfg.ConsoleName)); n != "" && want[n] {
		console = append(console, n)
	}
	for _, n := range lower(cfg.AppNames) {
		if want[n] {
			app = append(app, n)
		}
	}
	return console, app
}

// Stop closes both HTTPS doors, for a graceful shutdown.
func (s *Supervisor) Stop(ctx context.Context) error {
	err := s.appLn.Stop(ctx)
	if aerr := s.adminLn.Stop(ctx); err == nil {
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

// lower normalises a name list once, so the manager compares a server name
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
