package certs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/softwarity/meerkat/internal/edition"
)

// Planes, matching store.PlaneConsole / store.PlaneApp.
const (
	PlaneConsole = "console"
	PlaneApp     = "app"
)

// Settings is the TLS configuration as an operator sets it.
//
// There is no "switch HTTPS on" here, and that absence is the design: a switch
// that can be on with nothing behind it is a switch that lies. A certificate
// PLACED on a plane is what opens that plane's HTTPS door, and taking it off
// is what closes it. Nor is there a list of names: a certificate carries its
// own, and asking for them a second time is where the two drift apart.
type Settings struct {
	// Redirect forces the application's plain port over to HTTPS (SSL-06).
	// The console's plain port is never redirected - see redirect.go.
	Redirect bool `json:"redirect"`
	// HSTSMaxAge is how long browsers are told to use HTTPS only, in seconds
	// (SSL-06). It FOLLOWS the redirect rather than being a switch of its own:
	// forcing HTTPS is already the commitment - a 301 is remembered by the
	// browser too - and HSTS closes what the redirect cannot, the first request
	// that leaves in clear before being redirected. 0 means DefaultHSTS.
	HSTSMaxAge int          `json:"hstsMaxAge,omitempty"`
	ACME       ACMESettings `json:"acme"`

	// The names each plane was declared under, before certificates carried
	// their own (v73). Read once, to sort the authority's names by plane, and
	// never written again: see Normalized.
	LegacyConsoleName string   `json:"consoleName,omitempty"`
	LegacyAppNames    []string `json:"appNames,omitempty"`
}

// Normalized folds what an older installation - or a configuration exported
// by one - said about the authority into orders (v73). Two shapes came
// before: a name declared per plane with the automatic ones ticked, then a
// list of names per door. Either way the names one door could ask for become
// one order placed there, and names both doors could ask for one order placed
// on both. What the authority may be asked for, and on which door, is
// unchanged; the old fields go.
func (s Settings) Normalized() Settings {
	console := strings.ToLower(strings.TrimSpace(s.LegacyConsoleName))
	if console != "" || len(s.LegacyAppNames) > 0 {
		app := map[string]bool{}
		for _, n := range lower(s.LegacyAppNames) {
			app[n] = true
		}
		var appDomains []string
		for _, d := range lower(s.ACME.LegacyDomains) {
			if d == console && !slices.Contains(s.ACME.LegacyConsoleDomains, d) {
				s.ACME.LegacyConsoleDomains = append(s.ACME.LegacyConsoleDomains, d)
			}
			if d != console || app[d] {
				appDomains = append(appDomains, d)
			}
		}
		s.ACME.LegacyDomains = appDomains
	}
	s.LegacyConsoleName, s.LegacyAppNames = "", nil
	if len(s.ACME.LegacyDomains) > 0 || len(s.ACME.LegacyConsoleDomains) > 0 {
		app, console := lower(s.ACME.LegacyDomains), lower(s.ACME.LegacyConsoleDomains)
		var both, appOnly, consoleOnly []string
		for _, d := range app {
			if slices.Contains(console, d) {
				both = append(both, d)
			} else {
				appOnly = append(appOnly, d)
			}
		}
		for _, d := range console {
			if !slices.Contains(app, d) {
				consoleOnly = append(consoleOnly, d)
			}
		}
		for _, o := range []ACMEOrder{
			{Names: both, Console: true, App: true},
			{Names: consoleOnly, Console: true},
			{Names: appOnly, App: true},
		} {
			if len(o.Names) == 0 {
				continue
			}
			o.ID = OrderID("", o.Names)
			if !slices.ContainsFunc(s.ACME.Orders, func(x ACMEOrder) bool { return x.ID == o.ID }) {
				s.ACME.Orders = append(s.ACME.Orders, o)
			}
		}
	}
	s.ACME.LegacyDomains, s.ACME.LegacyConsoleDomains = nil, nil
	// The one account becomes the first authority, and keeps the cache as it
	// is - no prefix - so what it already issued is still found. Its orders,
	// which named no authority, are its own.
	a := &s.ACME
	if len(a.Authorities) == 0 && (a.LegacyEnabled || a.LegacyDirectoryURL != "" || a.LegacyAcceptTOS || len(a.Orders) > 0) {
		provider := ProviderCustom
		switch strings.TrimSpace(a.LegacyDirectoryURL) {
		case "", LetsEncryptURL:
			provider = ProviderLetsEncrypt
		case LetsEncryptStagingURL:
			provider = ProviderLetsEncryptStaging
		}
		a.Authorities = []Authority{Authority{
			ID: "ca-default", Provider: provider, DirectoryURL: a.LegacyDirectoryURL,
			Email: a.LegacyEmail, RootCA: a.LegacyRootCA, EABKeyID: a.LegacyEABKeyID,
			EABHMACKey: a.LegacyEABHMACKey, AcceptTOS: a.LegacyAcceptTOS,
		}.Normalize()}
	}
	a.LegacyEnabled, a.LegacyDirectoryURL, a.LegacyEmail, a.LegacyRootCA = false, "", "", ""
	a.LegacyEABKeyID, a.LegacyEABHMACKey, a.LegacyAcceptTOS = "", "", false
	for i := range a.Orders {
		if a.Orders[i].Authority == "" && len(a.Authorities) > 0 {
			a.Orders[i].Authority = a.Authorities[0].ID
		}
	}
	return s
}

// ACMEOrder is a certificate asked of the authority: the names, and the doors
// it is served on. It sits in the pool beside the certificates made or
// imported by hand, and is placed the same way; what differs is where the
// material comes from - the authority, fetched on the first visit and renewed
// before it ends - and where it lives, the ACME cache.
type ACMEOrder struct {
	ID string `json:"id"`
	// Authority is the ID of the authority it is asked of.
	Authority string   `json:"authority"`
	Names     []string `json:"names"`
	Console   bool     `json:"console,omitempty"`
	App       bool     `json:"app,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
}

// OrderID names an order by its authority and names: the same request, made
// twice, is one order - which is also what keeps an order folded from older
// settings the same order on every read.
func OrderID(authority string, names []string) string {
	h := sha256.Sum256([]byte(authority + " " + strings.Join(lower(names), " ")))
	return "acme-" + hex.EncodeToString(h[:6])
}

// MaxHSTS is the longest HSTS promise the console lets an installation make:
// two years, what the browsers' preload lists ask for. Past it a typo becomes
// a decade.
const MaxHSTS = 2 * 365 * 24 * 3600

// DefaultHSTS is the promise made when nobody chose one: a day. A browser
// keeps it for its whole length even if the certificates go away, so the
// first setting is the one that is cheap to be wrong about.
const DefaultHSTS = 24 * 3600

// HSTS is the lifetime actually sent: none unless the plain port is forced
// over to HTTPS, the default when no length was chosen.
func (s Settings) HSTS(redirecting bool) int {
	if !redirecting {
		return 0
	}
	if s.HSTSMaxAge > 0 {
		return s.HSTSMaxAge
	}
	return DefaultHSTS
}

// ACMESettings are the authorities and what is asked of them.
type ACMESettings struct {
	// Authorities are the ACME accounts set up, in the order they were.
	Authorities []Authority `json:"authorities,omitempty"`
	// Orders are the certificates asked of an authority, each placed on the
	// doors it serves. A door only ever asks for what is placed on it.
	Orders []ACMEOrder `json:"orders,omitempty"`

	// The one account of the settings before authorities (v73), read once
	// and folded into an authority by Normalized.
	LegacyEnabled      bool   `json:"enabled,omitempty"`
	LegacyDirectoryURL string `json:"directoryUrl,omitempty"`
	LegacyEmail        string `json:"email,omitempty"`
	LegacyRootCA       string `json:"rootCa,omitempty"`
	LegacyEABKeyID     string `json:"eabKeyId,omitempty"`
	LegacyEABHMACKey   string `json:"eabHmacKey,omitempty"`
	LegacyAcceptTOS    bool   `json:"acceptTos,omitempty"`
	// The names per door of the settings before orders, read once and folded
	// by Normalized.
	LegacyDomains        []string `json:"domains,omitempty"`
	LegacyConsoleDomains []string `json:"consoleDomains,omitempty"`
}

// DomainsOn is every name the authority may be asked for on one door: the
// names of the orders placed there.
func (a ACMESettings) DomainsOn(plane string) []string {
	var out []string
	for _, o := range a.Orders {
		if (plane == PlaneConsole && o.Console) || (plane == PlaneApp && o.App) {
			for _, n := range lower(o.Names) {
				if !slices.Contains(out, n) {
					out = append(out, n)
				}
			}
		}
	}
	return out
}

// placed is every name an order of one authority placed on a door holds:
// that authority's closed list.
func (a ACMESettings) placed(authority string) []string {
	var out []string
	for _, o := range a.Orders {
		if o.Authority != authority || (!o.Console && !o.App) {
			continue
		}
		for _, n := range lower(o.Names) {
			if !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
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

	// issues is what happened to each name asked of an authority (request.go).
	issues issueBook
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

	// One client per authority, each with its own account and its own corner
	// of the cache; each door maps a name to the client that asks for it. An
	// authority with nothing placed is not armed at all.
	consoleBy, appBy := map[string]*autocert.Manager{}, map[string]*autocert.Manager{}
	clients := map[string]*autocert.Manager{}
	authorities := cfg.ACME.Authorities
	// ACME is Enterprise (SSL-05). On the community image, what a
	// configuration brought over is kept and not asked - and said, since a
	// certificate nobody renews is one that ends.
	if !edition.Enterprise {
		placed := 0
		for _, a := range authorities {
			placed += len(cfg.ACME.placed(a.ID))
		}
		if placed > 0 {
			problems = append(problems, fmt.Sprintf(
				"ACME is part of the Enterprise edition: %d name(s) placed on a door are kept, and not asked of any authority", placed))
		}
		authorities = nil
	}
	for _, auth := range authorities {
		hosts := cfg.ACME.placed(auth.ID)
		if len(hosts) == 0 {
			continue
		}
		am, aerr := NewACME(ACMEOptions{
			DirectoryURL: auth.Directory(),
			Email:        auth.Email,
			Hosts:        hosts,
			RootCA:       auth.RootCA,
			EABKeyID:     auth.EABKeyID,
			EABHMACKey:   auth.EABHMACKey,
			AcceptTOS:    auth.AcceptTOS,
			Cache:        PrefixedCache{C: s.cache, Prefix: auth.CachePrefix},
		})
		if aerr != nil {
			// A broken authority must not take the installed certificates down
			// with it: what was serving yesterday keeps serving, and the
			// problem is named on screen.
			problems = append(problems, fmt.Sprintf("authority %s: %v", orName(auth), aerr))
			continue
		}
		clients[auth.ID] = am
	}
	for _, o := range cfg.ACME.Orders {
		am := clients[o.Authority]
		if am == nil {
			continue
		}
		for _, n := range lower(o.Names) {
			if o.Console {
				consoleBy[n] = am
			}
			if o.App {
				appBy[n] = am
			}
		}
	}
	s.Console.SetACME(consoleBy)
	s.App.SetACME(appBy)
	armed := len(clients) > 0
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
	s.redirect.SetServes(s.App.Answers)
	// HSTS stands with the redirect, down included: when every certificate
	// has expired and the redirect retreats, telling browsers to insist on
	// HTTPS would be the one thing left locking them out.
	s.redirect.SetHSTS(cfg.HSTS(redirect))

	s.mu.Lock()
	s.state.Console, s.state.App = s.adminLn.Running(), s.appLn.Running()
	s.state.ACME = armed
	s.state.Redirecting = redirect
	s.state.Problems = problems
	s.mu.Unlock()
	for _, p := range problems {
		slog.Warn("tls", "problem", p)
	}
	s.request(cfg)
	return failed
}

func orName(a Authority) string {
	if a.Name != "" {
		return a.Name
	}
	return a.ID
}

// Stop closes both HTTPS doors, for a graceful shutdown.
func (s *Supervisor) Stop(ctx context.Context) error {
	err := s.appLn.Stop(ctx)
	if aerr := s.adminLn.Stop(ctx); err == nil {
		err = aerr
	}
	return err
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
