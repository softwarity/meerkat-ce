// Command meerkat is the Meerkat app-gateway.
//
// Walking skeleton: routes live in the embedded store, are matched and
// proxied by the gateway router, and HTML responses can carry gateway
// injections. SIGHUP reloads the routes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // IANA zones for business-access windows, even on distroless

	"github.com/softwarity/meerkat/internal/admin"
	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/cluster"
	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/devtunnel"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/version"
)

func main() {
	// Before anything else: a process the developer tunnel started to answer
	// ONE verb. It is not a gateway - no store, no ports, no flags - and it
	// exits when the answer is out. The gateway re-execs itself rather than
	// running a verb in a goroutine because the verbs call os.Exit, which is
	// right for a process whose whole job is one answer and would take the
	// gateway down with it. The community image has no verbs to answer.
	if len(os.Args) > 1 && os.Args[1] == devtunnel.VerbArg {
		devtunnel.RunVerb()
		return
	}
	// The anonymous account a developer reaches before they have a client:
	// `ssh get@gateway install | sh`. This gateway has no client to hand out,
	// and says so in the one place the person is looking.
	if len(os.Args) > 1 && os.Args[1] == devtunnel.DownloadArg {
		devtunnel.PrintDownloadNotice()
		return
	}

	showVersion := flag.Bool("version", false, "print version and exit")
	addr := flag.String("addr", envOr("MEERKAT_ADDR", ":8080"), "application (data plane) listen address")
	adminAddr := flag.String("admin-addr", envOr("MEERKAT_ADMIN_ADDR", ":9090"), "administration (control plane) listen address")
	consoleURL := flag.String("console-url", envOr("MEERKAT_CONSOLE_URL", ""), "dev only: proxy the console UI to a front dev server (e.g. http://localhost:4200)")
	dataDir := flag.String("data", envOr("MEERKAT_DATA", "data"), "data directory (embedded storage)")
	// An external database, for several gateways serving one installation.
	// Empty is the embedded one, which is the default and the promise: a
	// `docker run` with nothing else keeps working exactly as it does.
	//
	// PostgreSQL is the one option, and it was chosen for a reason that is not
	// storage: it carries LISTEN/NOTIFY, which is how one instance tells the
	// others to re-read what changed, and advisory locks. No second moving
	// part to run.
	databaseURL := flag.String("database-url", envOr("MEERKAT_DATABASE_URL", ""),
		"PostgreSQL URL for a multi-instance deployment; empty uses the embedded storage in -data")
	configFile := flag.String("config", envOr("MEERKAT_CONFIG_FILE", ""),
		"configuration file (YAML or JSON) seeding an EMPTY gateway on first start; never overwrites a configured one")
	tenancy := flag.String("tenancy", envOr("MEERKAT_TENANCY", store.TenancySingle),
		"single (one implicit organisation) or multi (Enterprise); chosen once, at the first start")
	vaultFile := flag.String("vault", envOr("MEERKAT_VAULT_FILE", ""),
		"encrypted vault file to ingest once (passphrase in MEERKAT_VAULT_PASSPHRASE or _FILE)")
	// Two ports per plane, and they have different jobs: one serves, the other
	// tells the caller where to go (SSL-02). A port whose protocol cannot be
	// named is a port nobody can write a firewall rule or a runbook against.
	tlsAddr := flag.String("tls-addr", envOr("MEERKAT_TLS_ADDR", ":8443"),
		"application (data plane) HTTPS listen address, opened when TLS is switched on")
	adminTLSAddr := flag.String("admin-tls-addr", envOr("MEERKAT_ADMIN_TLS_ADDR", ":9443"),
		"administration (control plane) HTTPS listen address, opened when TLS is switched on")
	// This gateway is a PRODUCTION one: the whole developer surface is closed
	// here, whatever the database says. An environment decision rather than a
	// stored one, because the stored switch travels - a configuration export,
	// a restored backup, a database copied from staging can each carry it ON
	// into production, and nobody notices until there is a tunnel port in
	// front of customers.
	production := flag.Bool("production", envOr("MEERKAT_PRODUCTION", "") != "",
		"declare this gateway production: the developer surface (tooling, API docs, tunnel) stays closed whatever the settings say")
	// The developer tunnel's SSH port (DEV-11). Always has one - turning the
	// tunnel off is done by closing the developer surface, not by withholding
	// a port. Above 1024 because binding a privileged port would want root or
	// CAP_NET_BIND_SERVICE, against the grain of the rest of the image; which
	// port is PUBLISHED outside the cluster is a deployment decision anyway.
	plugAddr := flag.String("plug-addr", envOr("MEERKAT_PLUG_ADDR", devtunnel.DefaultAddr),
		"developer tunnel (plug) listen address, Enterprise only; it opens only where developer mode is on and a cluster is reachable")
	flag.Parse()

	if *showVersion {
		fmt.Printf("meerkat %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
		return
	}

	if err := run(options{
		addr: *addr, adminAddr: *adminAddr, tlsAddr: *tlsAddr, adminTLSAddr: *adminTLSAddr,
		consoleURL: *consoleURL, dataDir: *dataDir, configFile: *configFile,
		vaultFile: *vaultFile, tenancy: *tenancy, plugAddr: *plugAddr, databaseURL: *databaseURL,
		production: *production,
	}); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// options is what the command line and the environment settled on. A struct
// rather than nine positional strings: the two HTTPS addresses were the ones
// that made the argument list unreadable.
type options struct {
	addr, adminAddr       string
	tlsAddr, adminTLSAddr string
	consoleURL, dataDir   string
	configFile, vaultFile string
	tenancy               string
	plugAddr              string
	databaseURL           string
	production            bool
}

func run(o options) error {
	addr, adminAddr, consoleURL := o.addr, o.adminAddr, o.consoleURL
	dataDir, configFile, vaultFile, tenancy := o.dataDir, o.configFile, o.vaultFile, o.tenancy
	// Before the store opens, so the very first read of the developer switch
	// already has the environment's answer. Said out loud once: an operator
	// looking for tooling that is not there deserves to find the reason in the
	// startup lines rather than in the source.
	store.SetProduction(o.production)
	if o.production {
		slog.Info("production gateway: the developer surface is closed here", "why", "MEERKAT_PRODUCTION")
	}
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}
	st, err := store.OpenAt(dataDir, o.databaseURL)
	if err != nil {
		return err
	}
	if o.databaseURL != "" {
		// Said out loud, because it changes where everything this gateway
		// knows actually lives - and because a URL in an environment is easy
		// to set by accident and hard to notice.
		slog.Info("storage: external database", "kind", "postgres")
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	// What this installation IS, before it answers a request. The edition is a
	// COMPILE-time constant now - the image is the licence - so nothing is read
	// here beyond the mode.
	slog.Info("edition", "edition", edition.Name, "enterprise", edition.Enterprise)
	if err := settleTenancy(ctx, st, tenancy); err != nil {
		return err
	}
	// The VAULT first (VAULT-03): a configuration references $names, and a
	// route saved before its reference resolves comes up inert. Ingested once,
	// then never replayed.
	passphrase, err := config.PassphraseFrom(
		os.Getenv("MEERKAT_VAULT_PASSPHRASE"), os.Getenv("MEERKAT_VAULT_PASSPHRASE_FILE"))
	if err != nil {
		return err
	}
	if _, err := config.SeedVault(ctx, st, vaultFile, passphrase, time.Now().Unix()); err != nil {
		return err
	}
	// Then the configuration file (CFG-03): it seeds an empty gateway and is
	// ignored by a configured one. When it does seed, the demo routes stay
	// away - the operator has said what this gateway serves.
	seeded, err := config.Seed(ctx, st, configFile, time.Now().Unix())
	if err != nil {
		return err
	}
	if !seeded {
		if err := seedDemoRoute(ctx, st); err != nil {
			return err
		}
	}
	if err := auth.SeedAdmin(ctx, st); err != nil {
		return err
	}
	// The tape's first point (CFG-06), taken before anyone can change anything.
	// Without it the earliest state a gateway can go back to is the one AFTER
	// its first change - and "put it back the way it was" would be the one
	// thing the history cannot do. Written only when the configuration differs
	// from the last point, so an ordinary restart adds nothing; a restart that
	// seeded from a file, or one whose database was touched meanwhile, does.
	if written, err := config.Record(ctx, st, "", "gateway started"); err != nil {
		slog.Error("startup restore point not written", "err", err)
	} else if written {
		slog.Info("restore point taken", "reason", "startup")
	}

	// One session manager PER PLANE: distinct cookie names and a plane stamp
	// on every stored session - the two ports never share a browser session.
	sessions := session.NewManager(st)
	adminSessions := session.NewManager(st, session.ForAdminPlane())
	router := gateway.New(st, sessions)
	router.AdminAddr = adminAddr         // CORS for the admin console's Try it out
	router.AdminSessions = adminSessions // authorizes identity simulation (Try it out)
	if err := router.Reload(ctx); err != nil {
		return err
	}

	// Data plane (:8080): application routes + the user-flow pages. The
	// console is NEVER reachable here (CONSOLE-11).
	// Outbound e-mail: the config is read from the store AT SEND TIME, so a
	// console change applies to the very next message.
	mailer := func(ctx context.Context, msg mail.Message) error {
		return mail.Send(ctx, st.GetSMTP(ctx), msg)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	authHandler := auth.New(st, sessions)
	// What a developer's machine is serving right now, and who by. The
	// registry lives in the trunk because trunk code READS it; only the
	// Enterprise agent ever writes to it, so in the community image it stays
	// empty and every screen built on it shows nothing, without a guard.
	// The unavailable page (LIFE-05) is a page the gateway SERVES, so it goes
	// through the same chrome as the sign-in page: theme, layout, mark, and
	// the visitor's language. The router cannot reach that package on its own
	// - internal/auth already imports internal/routing - so the wire is made
	// here, where both are already known. The per-route maintenance FILTER
	// gets the same one: two ways to turn maintenance on, one page.
	router.Pages = authHandler.ServeMaintenance
	router.Stripe = authHandler.MaintenanceStripe
	routing.SetMaintenanceHandler(authHandler.ServeMaintenance)

	served := devtunnel.NewRegistry(authHandler.Events())
	// The pages read it, the tunnel writes it. In the community image nothing
	// writes, so what every page reads is an empty list.
	authHandler.WatchServed(served)
	authHandler.Mailer = mailer
	authHandler.Register(mux)
	router.RegisterDevDocs(mux) // /meerkat/apidocs - developer docs (dev capability)
	router.RegisterUISim(mux)   // /meerkat/dev-sim - UI test mode (dev capability)
	mux.Handle("/", router)

	// Control plane (:9090): admin API and the console. Keep this port off
	// the public load balancer. Login/logout are mounted here too, so the
	// console origin is self-sufficient for authentication.
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /healthz", healthz)
	adminAuth := auth.NewAdmin(st, adminSessions)
	adminAuth.Mailer = mailer
	adminAuth.Register(adminMux)
	adminAPI := admin.New(st, adminSessions, router)
	adminAPI.Mailer = mailer
	adminAPI.DataAddr = addr
	// The change bus (STORE-03): what this node reloads after a write, the
	// others reload too. Registered here rather than inside each package
	// because this is the only file that knows there are exactly two things a
	// node keeps in memory. On the embedded database Run returns at once.
	bus := cluster.New(st)
	bus.Register(store.TopicRouting, router.Reload)
	adminAPI.Bus = bus

	// The session caches, both planes. A session changed on one gateway - the
	// organisation just chosen, the login step just completed, the account
	// just revoked - is dropped from the memory of all of them, or the next
	// request served by another node answers with the previous state for the
	// few seconds the cache holds.
	//
	// Both managers forget on every signal: a token hash belongs to exactly
	// one of the two planes, so telling the other one costs a map lookup and
	// saves this wiring from having to know which.
	planes := []*session.Manager{sessions, adminSessions}
	tell := func(topic, arg string) { bus.Signal(context.Background(), topic, arg) }
	for _, sm := range planes {
		sm.Notify(tell)
	}
	bus.OnSignal(store.TopicSession, func(tokenHash string) {
		for _, sm := range planes {
			sm.Forget(tokenHash)
		}
	})
	bus.OnSignal(store.TopicSessionUser, func(userID string) {
		for _, sm := range planes {
			sm.ForgetUser(userID)
		}
	})

	// The live channel. A page is held open by whichever gateway the load
	// balancer gave it, so a message published on one node reaches a fraction
	// of the audience: a developer taking over a service is announced to the
	// people whose socket happens to be on the right node, and the others see
	// nothing. Relayed, it reaches all of them.
	//
	// The channel is never REQUIRED - every page has its state from a fetch at
	// load - so a message too big to travel, or a database that will not carry
	// it, costs the freshness and nothing else.
	hubs := []*events.Hub{authHandler.Events(), adminAuth.Events()}
	for _, hub := range hubs {
		hub.Relay(func(topic string, payload []byte) {
			bus.Signal(context.Background(), store.TopicEvent, topic+" "+string(payload))
		})
	}
	bus.OnSignal(store.TopicEvent, func(arg string) {
		topic, payload, ok := strings.Cut(arg, " ")
		if !ok {
			return
		}
		for _, hub := range hubs {
			hub.Deliver(topic, []byte(payload))
		}
	})
	adminAPI.Register(adminMux)
	if err := admin.RegisterConsole(adminMux, consoleURL, st, adminSessions); err != nil {
		return err
	}

	// Periodic TTL upkeep: expired sessions, lapsed e-mail tokens, and
	// self-registrations abandoned before confirming (7 days).
	//
	// One node at a time (STORE-03). Nothing here is unsafe to repeat - they
	// are DELETEs on a predicate - but N gateways scanning the same tables
	// every minute is N times the work for the same one row, and the audit
	// table is the biggest in the product. A node that finds the lock taken
	// skips this minute rather than queueing: the work comes round again in
	// sixty seconds, so waiting for a turn would only mean doing it twice.
	go func() {
		for range time.Tick(time.Minute) {
			ctx := context.Background()
			ran, err := st.TryLock(ctx, "upkeep", func(ctx context.Context) error {
				purge(ctx, sessions, st)
				return nil
			})
			if err != nil {
				slog.Error("upkeep lock failed", "err", err)
			} else if !ran {
				slog.Debug("upkeep skipped: another node is doing it")
			}
		}
	}()

	// The idle timeout wraps each plane whole (TENANT-05): any request
	// carrying a live session pushes its deadline, so the lifetime measures
	// inactivity rather than time since sign-in. Outermost on purpose - the
	// healthz handler and the flow pages are requests like the others, and a
	// deadline that only moved on SOME paths would end sessions at random.
	srv := &http.Server{
		Addr:              addr,
		Handler:           sessions.Sliding(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminSessions.Sliding(adminMux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// TLS (SSL-01/02): one HTTPS door per plane, opened and closed while the
	// gateway runs. The certificate a handshake receives comes from a
	// callback, so replacing one never moves a port and never drops a
	// connection - which is what the previous generation had to do.
	//
	// Only the APPLICATION plane's plain port redirects. The console's stays a
	// plain port for good: it is what a broken certificate gets repaired from.
	dataRedirect := certs.NewRedirect()
	srv.Handler = dataRedirect.Wrap(srv.Handler)
	tlsSup := certs.NewSupervisor(st,
		func(err error) bool { return errors.Is(err, store.ErrNoRows) },
		certs.NewListener("data", o.tlsAddr, srv.Handler, nil),
		certs.NewListener("admin", o.adminTLSAddr, adminSrv.Handler, nil),
		dataRedirect,
	)
	tlsSup.Ports(addr, adminAddr)
	// One gateway orders a certificate, the others wait and find it in the
	// shared cache (PERF-03). A no-op on the embedded database.
	tlsSup.SerialiseIssuance(st.WithLock)
	adminAPI.TLS = tlsSup
	bus.Register(store.TopicCertificates, tlsSup.Reload)
	if err := tlsSup.Reload(ctx); err != nil {
		// Not fatal: a taken HTTPS port must not keep the gateway from serving
		// the plain one, or a typo in an address would take the whole
		// installation down instead of one door.
		slog.Error("tls", "err", err)
	}

	// SIGHUP -> hot reload of the routes; SIGINT/SIGTERM -> graceful stop.
	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGHUP)
	go func() {
		for range reload {
			if err := router.Reload(context.Background()); err != nil {
				slog.Error("reload failed", "err", err)
			}
		}
	}()

	// The change bus, listening for what the OTHER nodes write. It shares the
	// tunnel's lifetime for the same reason: both are background loops that
	// must stop before the process does, and neither has anything to say on a
	// single-node installation.
	busCtx, stopBus := context.WithCancel(context.Background())
	defer stopBus()
	go bus.Run(busCtx)

	// The developer tunnel follows the developer-mode switch: mode off, no
	// agent, no port, nothing listening. The community image linked no agent
	// in, so this returns at once and says nothing.
	tunnelCtx, stopTunnel := context.WithCancel(context.Background())
	defer stopTunnel()
	go devtunnel.Supervise(tunnelCtx, devtunnel.Deps{Store: st, Registry: served, Addr: o.plugAddr})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	errc := make(chan error, 2)
	go func() {
		slog.Info("meerkat data plane listening", "addr", addr, "version", version.Version, "edition", edition.Name)
		errc <- srv.ListenAndServe()
	}()
	go func() {
		slog.Info("meerkat control plane listening", "addr", adminAddr)
		errc <- adminSrv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-stop:
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := srv.Shutdown(shutdownCtx)
		if aerr := adminSrv.Shutdown(shutdownCtx); err == nil {
			err = aerr
		}
		if terr := tlsSup.Stop(shutdownCtx); err == nil {
			err = terr
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return nil
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":"UP","version":%q}`, version.Version)
}

// seedDemoRoute gives a fresh instance one visible route, so `docker run` +
// one curl shows the whole chain (matching, strip, proxy, head injection).
// It only ever runs on an empty routes table.
func seedDemoRoute(ctx context.Context, st *store.Store) error {
	n, err := st.CountRoutes(ctx)
	if err != nil || n > 0 {
		return err
	}
	slog.Info("first start: seeding demo routes", "public", "/demo", "authenticated", "/secure", "trap", "/**")
	if err := st.SaveRoute(ctx, store.Route{
		ID:       "demo",
		Name:     "demo",
		Order:    100,
		Enabled:  true,
		IsUI:     true,
		Upstream: demoUpstream(),
		Predicates: []routing.Spec{
			{Type: "path", Args: map[string]any{"patterns": []any{"/demo/**"}}},
		},
		Filters: []routing.Spec{
			{Type: "strip-prefix", Args: map[string]any{"parts": 1}},
		},
		// Link names it in the user's applications menu (UIF-03): a UI route
		// without one is reachable but unlisted, which is not what a demo is for.
		UI: &store.RouteUI{
			Link:     "Demo",
			CustomJS: `console.log("injected by meerkat, the sentinel is watching")`,
		},
	}); err != nil {
		return err
	}
	if err := st.SaveRoute(ctx, store.Route{
		ID:       "demo-secure",
		Name:     "demo-secure",
		Order:    101,
		Enabled:  true,
		Access:   store.Access{Level: store.AccessAuth},
		IsUI:     true,
		Upstream: demoUpstream(),
		Predicates: []routing.Spec{
			{Type: "path", Args: map[string]any{"patterns": []any{"/secure/**"}}},
		},
		Filters: []routing.Spec{
			{Type: "strip-prefix", Args: map[string]any{"parts": 1}},
		},
		UI: &store.RouteUI{
			Link:     "Demo (secure)",
			CustomJS: `console.log("authenticated, meerkat let you in")`,
		},
	}); err != nil {
		return err
	}
	// The TRAP (ROUTE-10) is an ordinary route: a "/**" catch-all ordered LAST,
	// so whatever the routes above did not match - "/" included - lands there.
	return st.SaveRoute(ctx, store.Route{
		ID:       "trap",
		Name:     "trap",
		Order:    900,
		Enabled:  true,
		Upstream: demoUpstream(),
		Predicates: []routing.Spec{
			{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}},
		},
	})
}

// demoUpstream is what the seeded demo routes point at. httpbin.org by
// default, because a first start should show something working without asking
// anyone to run a container - and an ADDRESS in env, because a test bench must
// not depend on somebody else's website being up. The e2e suite runs its own
// httpbin and names it here; without that, three tests failed on i/o timeouts
// that said nothing about this product.
func demoUpstream() string {
	if u := os.Getenv("MEERKAT_DEMO_UPSTREAM"); u != "" {
		return u
	}
	return "https://httpbin.org"
}

// settleTenancy seeds the mode on a first start and gets out of the way after.
//
// The flag is for BOOTSTRAP - a first boot, a seeded install, GitOps - and the
// console owns the mode from then on: it is a setting read per request, not a
// value captured here, so switching it takes effect immediately and reverses
// just as fast. That is also why nothing below refuses to start. A gateway
// that will not boot because a flag disagrees with its database turns a
// configuration mistake into an outage, at the worst possible hour; the only
// refusal left is a mode that does not exist, where guessing would be worse.
func settleTenancy(ctx context.Context, st *store.Store, asked string) error {
	switch asked {
	case store.TenancySingle, store.TenancyMulti:
	default:
		return fmt.Errorf("tenancy %q is not allowed: use %q or %q",
			asked, store.TenancySingle, store.TenancyMulti)
	}
	recorded := st.Tenancy(ctx)
	seeded, err := st.TenancyWasChosen(ctx)
	if err != nil {
		return err
	}
	if seeded {
		// The console decides from here. Saying so beats silently ignoring a
		// flag someone put in a compose file and believes in.
		if asked != recorded {
			slog.Info("-tenancy ignored: the mode is set in the console",
				"asked", asked, "running", recorded)
		}
		reportHiddenTenants(ctx, st, recorded)
		return nil
	}
	mode := asked
	if mode == store.TenancyMulti && !edition.Enterprise {
		slog.Warn("-tenancy multi ignored: no license covers it, starting single-tenant",
			"edition", edition.Name)
		mode = store.TenancySingle
	}
	// SetTenancy, not a bare setting write: it also MARKS the mode as chosen.
	// Without that mark this branch runs again on the next boot, and a gateway
	// switched to multi in the console would come back single after a restart -
	// its organisations still there, none of them named anywhere.
	if err := st.SetTenancy(ctx, mode); err != nil {
		return err
	}
	slog.Info("tenancy", "mode", mode)
	return nil
}

// reportHiddenTenants says out loud what single-tenant mode is holding back.
// Nothing is deleted and the mode reverses in one click, but the people in
// those organisations lose access to whatever asks for one - and finding that
// out through complaints rather than through a startup line is the failure
// mode this exists to prevent.
func reportHiddenTenants(ctx context.Context, st *store.Store, mode string) {
	if mode != store.TenancySingle {
		return
	}
	n, err := st.CountTenants(ctx)
	if err != nil || n <= 1 {
		return
	}
	slog.Warn("single-tenant mode is serving one organisation and holding back the others",
		"hidden", n-1, "nothing_deleted", true)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// purge is one round of TTL upkeep. Extracted from the loop above so the lock
// wraps a call rather than a block: what runs under a lock should be readable
// as one thing.
//
// Every failure is logged and the next one is still attempted. They are
// independent tables, and giving up on the audit trail because an e-mail token
// would not delete is how a minute of upkeep becomes none.
func purge(ctx context.Context, sessions *session.Manager, st *store.Store) {
	if n, err := sessions.PurgeExpired(ctx); err != nil {
		slog.Error("session purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged expired sessions", "count", n)
	}
	if _, err := st.PurgeExpiredEmailTokens(ctx, time.Now().Unix()); err != nil {
		slog.Error("email token purge failed", "err", err)
	}
	if n, err := st.PurgeUnconfirmedSelfRegistrations(ctx, time.Now().Add(-7*24*time.Hour).Unix()); err != nil {
		slog.Error("unconfirmed sign-up purge failed", "err", err)
	} else if n > 0 {
		slog.Info("purged unconfirmed sign-ups", "count", n)
	}
	if _, err := st.PurgeExpiredAPITokens(ctx, time.Now().Unix()); err != nil {
		slog.Error("api token purge failed", "err", err)
	}
	if n, err := st.PurgeAuditEventsBefore(ctx, time.Now().Add(-admin.AuditRetention).Unix()); err != nil {
		slog.Error("audit purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged old audit events", "count", n)
	}
	// The brute-force rows, past the longest window any policy can name. A day
	// rather than the configured window: the policy is editable, and a purge
	// that trims to the CURRENT setting would erase the evidence the moment
	// someone widened it.
	if n, err := st.PurgeAttemptsBefore(ctx, time.Now().Add(-24*time.Hour)); err != nil {
		slog.Error("attempt purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged old sign-in attempts", "count", n)
	}
}
