// Command meerkat is the Meerkat app-gateway.
//
// Walking skeleton: routes live in the embedded store, are matched and
// proxied by the gateway router, and HTML responses can carry gateway
// injections. SIGHUP reloads the routes.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // IANA zones for business-access windows, even on distroless

	"github.com/softwarity/meerkat"
	"github.com/softwarity/meerkat/internal/admin"
	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/cluster"
	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/devtunnel"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/evalmark"
	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/expiry"
	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/idp"
	"github.com/softwarity/meerkat/internal/live"
	"github.com/softwarity/meerkat/internal/logging"
	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/notifier"
	"github.com/softwarity/meerkat/internal/oidcprovider"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/scheduler"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/tracing"
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
	// The database moves (db.go): a dump, a file, a copy into a server. Not a
	// gateway either - no ports - and it exits when the copy is done.
	if len(os.Args) > 1 && os.Args[1] == "db" {
		os.Exit(runDB(os.Args[2:]))
	}
	// The probe a container runtime runs inside the container (probe.go): the
	// image has no shell and no wget to run one with.
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(runHealth(os.Args[2:], os.Stdout, os.Stderr))
	}

	showVersion := flag.Bool("version", false, "print version and exit")
	addr := flag.String("addr", envOr("MEERKAT_ADDR", ":8080"), "application (data plane) listen address")
	adminAddr := flag.String("admin-addr", envOr("MEERKAT_ADMIN_ADDR", ":9090"), "administration (control plane) listen address")
	// The third audience: the services behind the gateway (the JWKS, the
	// scheduler's API). Plain HTTP on purpose - it is reached by a name inside
	// the cluster that no certificate carries, which is exactly what broke the
	// JWKS fetch the day HTTPS was forced on the application port.
	internalAddr := flag.String("internal-addr", envOr("MEERKAT_INTERNAL_ADDR", ":7070"),
		"internal plane listen address: what the services behind the gateway call (JWKS, scheduler API); keep it inside the cluster")
	internalTLSAddr := flag.String("internal-tls-addr", envOr("MEERKAT_INTERNAL_TLS_ADDR", ":7443"),
		"internal plane HTTPS listen address, opened when a certificate is placed on the internal plane")
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
	licenseFile := flag.String("license", envOr("MEERKAT_LICENSE_FILE", ""),
		"license file Softwarity issued, mounted from a secret; the console then shows it and takes no other")
	// Two ports per plane, and they have different jobs: one serves, the other
	// tells the caller where to go (SSL-02). A port whose protocol cannot be
	// named is a port nobody can write a firewall rule or a runbook against.
	tlsAddr := flag.String("tls-addr", envOr("MEERKAT_TLS_ADDR", ":8443"),
		"application (data plane) HTTPS listen address, opened when TLS is switched on")
	adminTLSAddr := flag.String("admin-tls-addr", envOr("MEERKAT_ADMIN_TLS_ADDR", ":9443"),
		"administration (control plane) HTTPS listen address, opened when TLS is switched on")
	// The developer tunnel's SSH port (DEV-11). Always has one - turning the
	// tunnel off is done by its switch (Infra, Plug) or by closing the
	// developer surface, not by withholding a port. Above 1024 because binding a privileged port would want root or
	// CAP_NET_BIND_SERVICE, against the grain of the rest of the image; which
	// port is PUBLISHED outside the cluster is a deployment decision anyway.
	plugAddr := flag.String("plug-addr", envOr("MEERKAT_PLUG_ADDR", devtunnel.DefaultAddr),
		"developer tunnel (plug) listen address, Enterprise only; it opens only once switched on (console: Infra, Plug) and with an orchestrator reachable")
	// What this gateway's own output looks like (OBS-03). Until these existed
	// Go's default stood - text, at Info, unchangeable. The level sits behind a
	// slog.LevelVar so it can be turned at run time; nothing does yet.
	logLevel := flag.String("log-level", envOr("MEERKAT_LOG_LEVEL", "info"),
		"how much the gateway says about itself: debug, info, warn, error")
	logFormat := flag.String("log-format", envOr("MEERKAT_LOG_FORMAT", ""),
		"json or text, text when empty: what is written before the settings are read; JSON for an agent is the console's switch (Infra, OpenTelemetry)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("meerkat %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
		return
	}

	// Before anything else says anything: a startup line written under Go's
	// default handler and then a switch to JSON would give a collector two
	// shapes for the same stream.
	// Who writes, on every line in OpenTelemetry's format (OBS-03).
	logging.SetResource(map[string]string{"service.name": "meerkat", "service.version": version.Version})
	format := logging.Setup(logging.Options{
		Format: logging.Format(*logFormat), Level: *logLevel,
	})
	// The access log and the trace export are switches of the console now
	// (Infra, OpenTelemetry), stored and applied on every node. The variables
	// that used to set them are read only to say so: ignored in silence, they
	// would look like they still worked.
	for _, v := range []string{"MEERKAT_ACCESS_LOG", "MEERKAT_OTLP_ENDPOINT", "MEERKAT_OTLP_SAMPLE"} {
		if os.Getenv(v) != "" {
			slog.Warn(v+" is no longer read: the access log and the trace export are set in the console (Infra, OpenTelemetry)",
				"ignored", os.Getenv(v))
		}
	}
	// Not "level": slog writes the RECORD's level under that name, and a JSON
	// object with the same key twice is a line a collector reads at random.
	slog.Info("logs", "threshold", *logLevel, "format", string(format))

	if err := run(options{
		addr: *addr, adminAddr: *adminAddr, internalAddr: *internalAddr, internalTLSAddr: *internalTLSAddr,
		tlsAddr: *tlsAddr, adminTLSAddr: *adminTLSAddr,
		consoleURL: *consoleURL, dataDir: *dataDir, configFile: *configFile,
		vaultFile: *vaultFile, tenancy: *tenancy, plugAddr: *plugAddr, databaseURL: *databaseURL,
		licenseFile: *licenseFile,
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
	internalAddr          string
	internalTLSAddr       string
	tlsAddr, adminTLSAddr string
	consoleURL, dataDir   string
	configFile, vaultFile string
	tenancy               string
	plugAddr              string
	databaseURL           string
	licenseFile           string
}

func run(o options) error {
	addr, adminAddr, consoleURL := o.addr, o.adminAddr, o.consoleURL
	dataDir, configFile, vaultFile, tenancy := o.dataDir, o.configFile, o.vaultFile, o.tenancy
	// MEERKAT_PRODUCTION used to close the developer surface whatever the
	// console said. The dev capability decides now, everywhere; the variable
	// is read only to tell whoever still sets it, since ignored in
	// silence it would look like it still worked.
	if v := os.Getenv("MEERKAT_PRODUCTION"); v != "" {
		slog.Warn("MEERKAT_PRODUCTION is no longer read: developers are the accounts with the dev capability (Application, Users, where Remove developer mode takes it from all but root) and the tunnel is switched on Infra, Plug",
			"ignored", v)
	}
	// Whose forwarded headers are believed is said on the Domains & TLS
	// screen now, plane by plane (SSL-14). The variable that used to say it
	// is read only to tell whoever still sets it where it went: ignored in
	// silence, it would look like it still worked.
	if v := os.Getenv("MEERKAT_TRUSTED_PROXIES"); v != "" {
		slog.Warn("MEERKAT_TRUSTED_PROXIES is no longer read: declare the proxy in front, and its addresses, on the Domains & TLS screen (Infra)",
			"ignored", v)
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
	// This installation's cookies are its own (session.SetCookieSuffix): two
	// gateways a browser reaches under one host name no longer overwrite each
	// other's sessions. Before any handler exists.
	installID, err := st.InstallationID(context.Background())
	if err != nil {
		return err
	}
	session.SetCookieSuffix(installID)

	ctx := context.Background()
	// Which product this binary IS, before it answers a request - a sentence
	// rather than the word "edition" three times. It is the line somebody reads
	// when an Enterprise capability is missing, and the first thing it has to
	// settle is whether they are running the Enterprise image at all: the tag
	// is a COMPILE-time constant, so a build that forgot it says so here and
	// nowhere else until a feature refuses.
	//
	// The machine-readable `edition` attribute stays on the data plane's own
	// line, where a collector looks for it.
	// The evaluation image says so here, because a journal is where someone
	// checks what is actually running after a deployment.
	switch {
	case evalmark.StartupNote != "":
		slog.Info("Meerkat Enterprise edition, "+evalmark.StartupNote+" build",
			"version", version.Version, "note", "not licensed for production use")
	case edition.Enterprise:
		slog.Info("Meerkat Enterprise edition", "version", version.Version)
	default:
		slog.Info("Meerkat Community edition", "version", version.Version)
	}
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
	// ignored by a configured one. Without one the gateway starts EMPTY: what
	// it serves is the operator's to say, and demonstration routes pointing at
	// somebody else's website were ours, not theirs.
	if _, err := config.Seed(ctx, st, configFile, time.Now().Unix()); err != nil {
		return err
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
	defer tracing.Stop()

	// The wordings this installation corrected or added (I18N), pushed into
	// the data plane before a single page is served. Missed, every built-in
	// page would speak the strings we ship until somebody opened the editor
	// and saved - which is the kind of bug that only shows after a restart.
	if layer, err := st.LocaleOverrides(ctx); err != nil {
		slog.Warn("locale corrections not loaded, the built-in pages speak the shipped wordings", "err", err)
	} else if len(layer) > 0 {
		auth.SetLocaleOverrides(layer)
		slog.Info("locale corrections", "languages", len(layer))
	}

	router := gateway.New(st, sessions)
	router.AdminAddr = adminAddr         // CORS for the admin console's Try it out
	router.DataAddr = addr               // the issuer's port under a loopback public name
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
	// The OpenID provider (SAUTH-05): the applications behind the routes sign
	// their people in through this gateway. Enterprise code; in the community
	// image the provider mounts nothing and wraps nothing, and its paths fall
	// through to the routes like any other.
	oidc := oidcprovider.New(oidcprovider.Deps{Store: st, Sessions: sessions, Router: router,
		Refuse: authHandler.ServeAppRefused})
	mux.Handle("/", oidc.WrapData(router))

	// Control plane (:9090): admin API and the console. Keep this port off
	// the public load balancer. Login/logout are mounted here too, so the
	// console origin is self-sufficient for authentication.
	adminMux := http.NewServeMux()
	adminAuth := auth.NewAdmin(st, adminSessions)
	adminAuth.Mailer = mailer
	adminAPI := admin.New(st, adminSessions, router)
	adminAPI.OIDC = oidc
	adminAPI.Mailer = mailer
	adminAPI.DataAddr = addr
	adminAPI.InternalAddr = o.internalAddr
	if abs, err := filepath.Abs(dataDir); err == nil {
		adminAPI.DataDir = abs
	}
	adminAPI.InternalTLSAddr = o.internalTLSAddr
	// Who calls the internal plane: a ServiceAccount token checked with the
	// API server, an address followed to its workload or container.
	adminAPI.Callers = discovery.NewCallers()
	adminAPI.ReleaseNotes = meerkat.ReleaseNotes
	adminAPI.LicenseFile = o.licenseFile
	adminAPI.LogLicense(ctx)
	// The Enterprise rows of the contract, for the License screen (CONSOLE-14).
	for _, f := range meerkat.EnterpriseFeatures() {
		adminAPI.EnterpriseFeatures = append(adminAPI.EnterpriseFeatures,
			admin.EditionFeature{ID: f.ID, Status: f.Status, Edition: f.Edition})
	}
	// The change bus (STORE-03): what this node reloads after a write, the
	// others reload too. Registered here rather than inside each package
	// because this is the only file that knows there are exactly two things a
	// node keeps in memory. On the embedded database Run returns at once.
	bus := cluster.New(st)
	bus.Register(store.TopicRouting, router.Reload)
	adminAPI.Bus = bus
	adminAPI.Served = served

	// The session caches, both planes. A session changed on one gateway - the
	// organisation just chosen, the login step just completed, the account
	// just revoked - is dropped from the memory of all of them, or the next
	// request served by another node answers with the previous state for the
	// few seconds the cache holds.
	//
	// Both managers forget on every signal: a token hash belongs to exactly
	// one of the two planes, so telling the other one costs a map lookup and
	// saves this wiring from having to know which.
	//
	// And both forget LOCALLY first, before the message leaves. A change made
	// through one plane can concern the other: an account disabled on the
	// control plane owns data-plane API tokens, which the data manager keeps
	// in its own memory - and a single gateway has no bus to carry that across.
	planes := []*session.Manager{sessions, adminSessions}
	forget := func(topic, arg string) {
		for _, sm := range planes {
			switch topic {
			case store.TopicSession:
				sm.Forget(arg)
			case store.TopicSessionUser:
				sm.ForgetUser(arg)
			case store.TopicAPIToken:
				sm.ForgetToken(arg)
			}
		}
	}
	tell := func(topic, arg string) {
		forget(topic, arg)
		bus.Signal(context.Background(), topic, arg)
	}
	for _, sm := range planes {
		sm.Notify(tell)
	}
	for _, topic := range []string{store.TopicSession, store.TopicSessionUser, store.TopicAPIToken} {
		bus.OnSignal(topic, func(arg string) { forget(topic, arg) })
	}

	// Who a session IS - the account, its organisation, its roles - is kept by
	// the router between writes (gateway/identitycache.go), and forgotten as a
	// whole after any write served outside the proxied traffic: the admin API,
	// the agent endpoint, the sign-in and profile pages. See afterWrites.
	identityChanged := func() {
		router.ForgetIdentities()
		bus.Signal(context.Background(), store.TopicIdentity, "*")
	}
	bus.OnSignal(store.TopicIdentity, func(string) { router.ForgetIdentities() })

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
	// The notifications (NOTIF-05): what the services behind the routes tell
	// their users. Enterprise code; in the community image nothing is
	// mounted, no badge is drawn, and the internal plane has no such API. One
	// signal per notification crosses the nodes, whatever its audience: each
	// node reads the recipients from the rows and tells the pages it holds.
	// A reader's acknowledgement calls the sender's service through the data
	// plane's own handler, built further down: reached through this.
	var notifyPlane http.Handler
	notify := notifier.New(notifier.Deps{Store: st, Sessions: sessions, Router: router, Hub: authHandler.Events(),
		Pages: authHandler, Signal: func(ctx context.Context, arg string) { bus.Signal(ctx, store.TopicNotification, arg) },
		Plane: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if notifyPlane == nil {
				http.Error(w, "the data plane is not up yet", http.StatusServiceUnavailable)
				return
			}
			notifyPlane.ServeHTTP(w, r)
		}),
		Audit: adminAPI.Record})
	// Everything of the data plane's own is known now: mount it.
	mountData(mux, readyz(st, router), authHandler, router, oidc, notify)
	adminAPI.Notify = notify
	authHandler.NotifyBadge = func(ctx context.Context, sess store.Session) (auth.NotifyBadge, bool) {
		b, ok := notify.Badge(ctx, sess)
		return auth.NotifyBadge{Total: b.Total, Page: b.Page, InBar: b.InBar}, ok
	}
	bus.OnSignalFrom(store.TopicNotification, func(from, arg string) {
		if from == bus.Node() {
			return // already told here, before it left
		}
		notify.Receive(arg)
	})
	bus.OnSignal(store.TopicEvent, func(arg string) {
		topic, payload, ok := strings.Cut(arg, " ")
		if !ok {
			return
		}
		for _, hub := range hubs {
			hub.Deliver(topic, []byte(payload))
		}
	})
	// What passes through, counted here and pooled across the cluster (OBS-01).
	//
	// Every node broadcasts its running TOTALS and every node aggregates, so
	// the console's curves are the cluster's rather than whichever node the
	// load balancer handed the screen. Nothing is written to the database for
	// this: at a five-second resolution that would be forty writes a second on
	// the store that also serves sessions, forever, to hold a chart.
	//
	// A single-node installation takes exactly the same path with one member.
	fleet := metrics.NewFleet()
	window := metrics.NewWindow(metrics.DefaultWindow)
	var lastSent map[string]metrics.RouteSnapshot
	go metrics.Run(ctx, router.Metrics(), fleet, window, bus.Node(), metrics.DefaultInterval,
		func(s metrics.Snapshot) {
			// Only the routes whose totals moved: a table of a hundred and
			// fifty is mostly idle in any given five seconds, and a
			// notification may carry 7000 bytes.
			var moved metrics.Snapshot
			moved, lastSent = metrics.Moved(s, lastSent)
			if len(moved.Routes) == 0 {
				return
			}
			for _, msg := range metrics.Encode(moved, metrics.Chunk) {
				bus.Signal(ctx, store.TopicMetrics, msg)
			}
		})
	// What a developer's machine answers for, told across the cluster (DEV-11).
	// The traffic already crosses on its own - plug's signpost is a service on
	// the overlay - so what travels here is only the SAYING, and losing that is
	// how somebody reads a page of a colleague's work-in-progress without one
	// word telling them. Same shape as the metrics fleet: a node publishes its
	// whole list, the others keep one entry per node, silence expires.
	served.Relay(func(names []devtunnel.Served) {
		msg, err := json.Marshal(names)
		if err != nil {
			return
		}
		bus.Signal(ctx, store.TopicServed, string(msg))
	})
	bus.OnSignalFrom(store.TopicServed, func(from, arg string) {
		if from == bus.Node() {
			return // its own list, already the truth here
		}
		var names []devtunnel.Served
		if err := json.Unmarshal([]byte(arg), &names); err != nil {
			slog.Debug("a served-names message could not be read", "from", from, "err", err)
			return
		}
		served.Report(from, names, time.Now())
	})
	// The plug runs, the same way (DEV-13, DEV-14): each node says which it holds a
	// part of, the trail is written once through the store, and ending one is
	// asked of every node.
	served.Wire(devtunnel.Wiring{
		Store: st, Node: bus.Node(), Audit: adminAPI.Record,
		Close: func(kind, target, reason string) {
			bus.Signal(ctx, store.TopicPlugClose, kind+" "+target+" "+reason)
		},
	})
	served.RelaySessions(func(sessions []devtunnel.Session) {
		msg, err := json.Marshal(sessions)
		if err != nil {
			return
		}
		bus.Signal(ctx, store.TopicPlugSessions, string(msg))
	})
	bus.OnSignalFrom(store.TopicPlugSessions, func(from, arg string) {
		if from == bus.Node() {
			return
		}
		var sessions []devtunnel.Session
		if err := json.Unmarshal([]byte(arg), &sessions); err != nil {
			slog.Debug("a plug-sessions message could not be read", "from", from, "err", err)
			return
		}
		served.ReportSessions(from, sessions, time.Now())
	})
	bus.OnSignalFrom(store.TopicPlugClose, func(from, arg string) {
		if from == bus.Node() {
			return // already applied by the node that asked
		}
		kind, rest, _ := strings.Cut(arg, " ")
		target, reason, _ := strings.Cut(rest, " ")
		served.ApplyClose(kind, target, reason)
	})
	go served.Tell(ctx, metrics.DefaultInterval)

	bus.OnSignalFrom(store.TopicMetrics, func(from, arg string) {
		s, err := metrics.Decode(arg)
		if err != nil {
			// A node speaking another dialect during a rolling update, or a
			// truncated payload. Logged once at debug and dropped: the next
			// message carries totals and repairs whatever this one lost.
			slog.Debug("could not read a node's counters", "from", from, "err", err)
			return
		}
		fleet.Report(from, s, time.Now())
	})

	adminAPI.Metrics = window
	// And WHICH endpoint of a route a request was. Resolved here rather than
	// in the router: the common case is a spec the service publishes itself,
	// and fetching it while compiling routes would make a reload wait on a
	// service. See (*admin.API).RefreshOperations.
	go adminAPI.RefreshOperations(ctx)
	// One socket for the whole console, with a source per screen - and a
	// REGISTRY per perimeter, because what a caller may watch is decided at the
	// upgrade and cannot be checked inside a source (see internal/live).
	//
	// The journal of administrative writes is served to every administrator: it
	// is what tells a screen that what it is showing moved (CONSOLE-13). The
	// traffic curves answer the routing plane, the scheduled calls the
	// application's - the same perimeters their own APIs answer.
	changes := live.NewChanges()
	traffic := live.NewTraffic(window)
	schedules := live.NewSchedules(st)
	// Whether each route's target is there at all (SVC-04): checked in the
	// background every gateway.TargetInterval, per node like the breaker, and
	// a flip is pushed to the routes screen. Stops with ctx.
	routeHealth := live.NewRouteHealth()
	logs := live.NewLogs()
	// Who is signed in (CONSOLE-07): every session written or removed moves the
	// counter, and the Users and Members screens re-read GET /api/sessions. A
	// move on this node is told to the others over the bus, so a console held
	// by another gateway hears of a sign-in it did not serve.
	// The SAML assertions already spent (AUTH-19): the store's table, so a
	// replay is refused on every node.
	idp.SetAssertionLedger(st)

	presence := live.NewPresence()
	st.OnSessionsMoved(func() {
		presence.Moved()
		bus.Signal(context.Background(), store.TopicPresence, "")
	})
	bus.OnSignal(store.TopicPresence, func(string) { presence.Moved() })
	// An ACME request answered (SSL-05): the TLS screen re-reads the pool
	// instead of asking every few seconds while one is out. Told by the TLS
	// supervisor, wired where it is built.
	issues := live.NewCounter(live.CertificatesTopic)
	bus.OnSignal(store.TopicIssue, func(string) { issues.Moved() })
	// Which of the console's doors is called, and by whom (SSL-15): what says
	// a proxy was pointed at HTTPS, and what keeps the port it still calls
	// from being switched off. Pooled across the cluster like the served
	// names; a clock lets a door that stopped receiving turn idle.
	doorTraffic := certs.NewTraffic()
	doorMoved := live.NewCounter(live.DoorTrafficTopic)
	doorTraffic.Moved = doorMoved.Moved
	doorTraffic.Relay = func(list []certs.Seen) {
		if msg, err := json.Marshal(list); err == nil {
			bus.Signal(context.Background(), store.TopicDoorTraffic, string(msg))
		}
	}
	bus.OnSignalFrom(store.TopicDoorTraffic, func(from, arg string) {
		if from == bus.Node() {
			return
		}
		var list []certs.Seen
		if err := json.Unmarshal([]byte(arg), &list); err != nil {
			slog.Debug("a door-traffic message could not be read", "from", from, "err", err)
			return
		}
		doorTraffic.Report(from, list)
	})
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				doorTraffic.Check()
			}
		}
	}()
	adminAPI.DoorTraffic = doorTraffic

	// The pause (store/pause.go): every node stops writing at once, and every
	// console hears it. Told by the API on the node that took it, relayed here
	// to the others.
	paused := live.NewCounter(live.PauseTopic)
	adminAPI.PauseMoved = paused.Moved
	bus.OnSignal(store.TopicPause, func(arg string) {
		st.Pause(arg == "on")
		paused.Moved()
	})
	// The runtime, watched (discovery.Watcher): its events, not a timer, are
	// what say a target lost a replica or changed image.
	runtime := discovery.NewWatcher()
	go runtime.Run(ctx)
	go router.WatchTargets(ctx, gateway.DefaultTargetCheck(runtime, routeHealth.Flipped))
	liveServer := live.New(func(p live.Perimeter) live.Sources {
		sources := live.Sources{
			live.ChangesTopic: changes.For(p.Named, p.Quiet),
			// A counter, no person: what it answers is read through
			// GET /api/sessions, which scopes it.
			live.PresenceTopic:     presence,
			live.CertificatesTopic: issues,
			live.DoorTrafficTopic:  doorMoved,
			live.PauseTopic:        paused,
		}
		if p.Traffic {
			sources[live.TrafficTopic] = traffic
			// The routing plane's readers are who GET /api/routes/health
			// answers, so they are who hear that a target flipped.
			sources[live.RouteHealthTopic] = routeHealth
			// And that this node wrote a line, for the Logs screen (OBS-03),
			// read through GET /api/logs, which answers the same readers.
			sources[live.LogsTopic] = logs
		}
		if p.Schedules {
			// The scheduler screen watches a run start, advance and finish
			// (SCHED-01).
			sources[live.SchedulesTopic] = schedules
		}
		return sources
	})
	adminAPI.Live = func(p admin.LivePerimeter, w http.ResponseWriter, r *http.Request) {
		liveServer.ServeFor(live.Perimeter{
			Key: p.Key, Named: p.Named, Quiet: p.Quiet,
			Traffic: p.RoutingPlane, Schedules: p.ApplicationPlane,
		}, w, r)
	}
	// Every administrative write passes through the audit funnel, which is what
	// makes this complete without an emission at forty call sites.
	adminAPI.Changes = changes
	// And what the store raises by itself - a schedule starting or stopping
	// to fail (SCHED-06) - goes through the same funnel, so the trail on
	// screen shows it without a reload.
	st.OnOwnAudit(adminAPI.Record)
	// The log level turned on another node's console (OBS-03).
	bus.OnSignal(store.TopicLogLevel, admin.ApplyLogLevelSignal)
	// And a write taken by ANOTHER node reaches the consoles this one holds
	// open. A signal rather than a topic: there is no table to re-read behind
	// it - the screens ask the API - so losing one costs a late screen and
	// never a wrong write, which the row's revision refuses anyway.
	bus.OnSignal(store.TopicChanged, func(arg string) {
		if ev, ok := admin.DecodeChange(arg); ok {
			changes.Record(ev)
		}
	})
	mountAdmin(adminMux, readyz(st, router), adminAuth, adminAPI)
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
			// Paused for a database move: the upkeep deletes, and nothing is
			// written now. It catches up on the minute after the pause.
			if st.Paused() {
				continue
			}
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
	everything := func(*http.Request) bool { return true }
	dataPlane := whilePaused(st, everything, router.ServePaused, sessions.Sliding(afterWrites(mux, "/", identityChanged,
		// The OpenID provider's writes are its own codes and refresh tokens,
		// never an identity - and an application refreshing its tokens is
		// traffic, not an administrator's write.
		oidcprovider.PathPrefix)))
	adminAPI.DataPlane = dataPlane
	notifyPlane = dataPlane
	srv := &http.Server{
		Addr:              addr,
		Handler:           dataPlane,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Scheduled calls (SCHED-01). The scheduler is given the data plane's own
	// handler, not an address: a scheduled call enters by the front door, in
	// process, so it meets the predicates, the filters, the route's rule and
	// the per-endpoint rules exactly as a person does - and no port, no
	// certificate and no redirect sit between the gateway and itself.
	sched := scheduler.New(scheduler.Deps{
		Store: st,
		Plane: dataPlane,
		// Which node claimed a run, for the console and for a log line. The
		// hostname is what an operator recognises in a cluster; a pod's name
		// IS its hostname.
		Node: hostname(),
		// A node that stops hands its calls in flight back, and says so, so
		// that another sends them again now rather than when their lease ends.
		Announce: func(ctx context.Context) { bus.Announce(ctx, store.TopicSchedules) },
	})
	// Written from the control plane - that is where the API lives - so it is
	// the admin side that rings this node's own scheduler.
	adminAPI.Scheduler = sched
	// The doorbell: a schedule written on another node is a reason to look
	// now. The table is still the truth - see store/changes.go.
	bus.Register(store.TopicSchedules, func(context.Context) error {
		sched.Wake()
		return nil
	})
	schedCtx, stopSchedules := context.WithCancel(context.Background())
	defer stopSchedules()
	go sched.Run(schedCtx)
	adminHandler := whilePaused(st, refusesWhilePaused, refusePaused, adminSessions.Sliding(afterWrites(adminMux, "", identityChanged)))
	// The agent calls the control plane through the very chain a request
	// meets (MCP-09): the same pause, the same reload after a write.
	adminAPI.Self = adminHandler
	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Internal plane (:7070): the services behind the gateway, by token. No
	// session slides here and no write changes an identity, so neither
	// wrapper the control plane carries has anything to do.
	internalMux := http.NewServeMux()
	mountInternal(internalMux, adminAPI)
	internalPlane := whilePaused(st, refusesWhilePaused, refusePaused, internalMux)
	adminAPI.InternalPlane = internalPlane
	internalSrv := &http.Server{
		Addr:              o.internalAddr,
		Handler:           internalPlane,
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
	// Each plane names its requests, for the forwarded headers to be judged
	// by what that plane's door says (SSL-14) - outermost, so the redirect
	// reads it too.
	srv.Handler = filters.OnPlane(certs.PlaneApp, dataRedirect.Wrap(srv.Handler))
	adminSrv.Handler = filters.OnPlane(certs.PlaneConsole, doorTraffic.Wrap(certs.PlaneConsole, adminSrv.Handler))
	internalSrv.Handler = filters.OnPlane(certs.PlaneInternal, internalSrv.Handler)
	// Where the runtime publishes the HTTPS door, so the plain port sends
	// callers where they can go (8444) rather than where the container
	// listens (8443). Read now and again every few minutes: a Service edited
	// by hand changes it with no restart.
	go func() {
		for {
			ports := discovery.Self(ctx).Ports
			dataRedirect.SetPublished(func(inside int) int { return ports[inside] })
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
		}
	}()
	tlsSup := certs.NewSupervisor(st,
		func(err error) bool { return errors.Is(err, store.ErrNoRows) },
		certs.NewListener("data", o.tlsAddr, srv.Handler, nil),
		certs.NewListener("admin", o.adminTLSAddr, certs.ForgetHSTS(adminSrv.Handler), nil),
		dataRedirect,
		// The internal plane's HTTPS door (SSL-12): no redirect and no HSTS,
		// its callers are programs reaching it by a name inside the cluster.
		certs.NewListener("internal", o.internalTLSAddr, internalSrv.Handler, nil),
	)
	tlsSup.Ports(addr, adminAddr, o.internalAddr)
	// The plain doors too, so a door switched off listens on the loopback
	// only (SSL-15). A plain door that stops serving for any other reason
	// stops the gateway, as ListenAndServe did.
	errc := make(chan error, 3)
	plains := map[string]*certs.Plain{
		certs.PlaneApp:      certs.NewPlain("data", addr, srv.Handler),
		certs.PlaneConsole:  certs.NewPlain("admin", adminAddr, adminSrv.Handler),
		certs.PlaneInternal: certs.NewPlain("internal", o.internalAddr, internalSrv.Handler),
	}
	for _, p := range plains {
		p.OnError = func(err error) { errc <- err }
	}
	tlsSup.Plain = plains
	// What each door says becomes what its plane believes (SSL-14), and a
	// proxied door serves the certificate carrying its runtime names.
	tlsSup.Trust = filters.SetPlane
	tlsSup.RuntimeNames = adminAPI.PlaneNames
	// A mailed link may point only at a name a certificate placed on the
	// plane covers (P0-01): anything else mails a code. The sign-in pages of
	// each plane ask their own door.
	authHandler.Covers = tlsSup.App.Answers
	adminAuth.Covers = tlsSup.Console.Answers
	// Or a name the operator declared reached through a proxy that
	// terminates TLS in front of that door: the request arrives in clear,
	// and the name is still ours.
	authHandler.Proxied = tlsSup.Proxied(certs.PlaneApp)
	adminAuth.Proxied = tlsSup.Proxied(certs.PlaneConsole)
	// One gateway orders a certificate, the others wait and find it in the
	// shared cache (PERF-03). A no-op on the embedded database.
	tlsSup.SerialiseIssuance(st.WithLock)
	adminAPI.TLS = tlsSup
	tlsSup.OnIssueMoved(func() {
		issues.Moved()
		bus.Signal(context.Background(), store.TopicIssue, "")
	})
	bus.Register(store.TopicCertificates, tlsSup.Reload)
	// The certificates the file system provides (SSL-11): a renewal by the
	// cluster rewrites the files and nobody saves anything, so each node
	// watches its own mounts and reloads its doors when what they hold moved.
	go func() {
		last := st.FileCertificatesSum(ctx)
		for range time.Tick(30 * time.Second) {
			sum := st.FileCertificatesSum(ctx)
			if sum == last {
				continue
			}
			last = sum
			if _, err := st.RefreshFileCertificates(ctx); err != nil {
				slog.Error("certificates from the file system not refreshed", "err", err)
			}
			if err := tlsSup.Reload(ctx); err != nil {
				slog.Error("tls reload after a file changed", "err", err)
			}
			slog.Info("a certificate from the file system changed, its doors were reloaded")
		}
	}()
	// The internal plane's certificate asked of cert-manager (SSL-13): each
	// node watches the Secret through the API and holds the pair in memory, so
	// a renewal - cert-manager rewriting the Secret - reaches every door when
	// it happens, the key never entering the database.
	if cm, why := discovery.NewCertManager(); cm != nil {
		// One Certificate per plane: the internal plane's, and the console's
		// and the application's when a proxy in front re-encrypts to them
		// (SSL-14). Each node follows the three Secrets.
		managers := map[string]*discovery.CertManager{}
		for _, plane := range store.CertPlanes {
			managers[plane] = cm.For(plane)
		}
		adminAPI.CertManager = cm
		adminAPI.CertManagers = map[string]admin.CertManager{
			store.PlaneConsole: managers[store.PlaneConsole],
			store.PlaneApp:     managers[store.PlaneApp],
		}
		st.OnIssuedPair(func(secret string) (string, string, bool) {
			for _, m := range managers {
				if m.SecretName() == secret {
					return m.Pair()
				}
			}
			return "", "", false
		})
		for _, m := range managers {
			go m.Watch(ctx, func() {
				c, k, ok := m.Pair()
				if ok {
					if _, err := st.RefreshIssuedCertificate(ctx, m.SecretName(), c, k); err != nil {
						slog.Error("the certificate cert-manager issued is not stored", "err", err)
					}
				}
				if err := tlsSup.Reload(ctx); err != nil {
					slog.Error("tls reload after cert-manager's Secret moved", "err", err)
				}
				if ok {
					slog.Info("cert-manager wrote a certificate's Secret, its doors were reloaded", "secret", m.SecretName())
				} else {
					slog.Info("a certificate's Secret from cert-manager is gone, its doors were reloaded", "secret", m.SecretName())
				}
				issues.Moved()
			}, issues.Moved)
		}
	} else {
		adminAPI.CertManagerWhy = why
	}
	// Where the spans, the pushed counters and the logs go (OBS-04, OBS-06),
	// and whether the access log is written: the STORED setting, applied at
	// every start - an operator who switched something on in the console
	// finds it on again after a restart - and on every node when it changes.
	adminAPI.ApplyTelemetry(ctx)
	bus.Register(store.TopicTelemetry, func(ctx context.Context) error {
		adminAPI.ApplyTelemetry(ctx)
		return nil
	})
	defer metrics.StopPush()
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

	// The daily notice about accounts whose window is closing (MODEL-02). It
	// shares the same lifetime: a loop that only ever reads a setting and,
	// once a day, sends one message. In a cluster every node runs it and one
	// of them sends - the advisory lock and the recorded day settle which.
	digestCtx, stopDigest := context.WithCancel(context.Background())
	defer stopDigest()
	go expiry.New(st, mailer).Run(digestCtx)

	// The developer tunnel follows the developer-mode switch: mode off, no
	// agent, no port, nothing listening. The community image linked no agent
	// in, so this returns at once and says nothing.
	tunnelCtx, stopTunnel := context.WithCancel(context.Background())
	defer stopTunnel()
	go devtunnel.Supervise(tunnelCtx, devtunnel.Deps{Store: st, Registry: served, Addr: o.plugAddr})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	// The plain doors, as the stored setting says: opened here, last, once
	// everything behind them is wired; reloads switch them afterwards.
	tlsCfg := st.RawTLS(ctx)
	for plane, p := range plains {
		if err := p.Open(!tlsCfg.Opens(plane, "http")); err != nil {
			return err
		}
	}
	// The address, and nothing else: which product this is and which version
	// it runs were said thirteen milliseconds earlier, and a line that
	// repeats the line above it teaches people to stop reading the log.
	slog.Info("meerkat data plane listening", "addr", addr, "loopbackOnly", plains[certs.PlaneApp].Loopback())
	slog.Info("meerkat control plane listening", "addr", adminAddr, "loopbackOnly", plains[certs.PlaneConsole].Loopback())
	slog.Info("meerkat internal plane listening", "addr", o.internalAddr, "loopbackOnly", plains[certs.PlaneInternal].Loopback())

	select {
	case err := <-errc:
		return err
	case <-stop:
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// The scheduler first stops TAKING, then gets a few seconds for the
		// calls it has made - through the data plane, in process, so closing
		// the listeners does not cut them. What is still waiting after that
		// is handed back, for a node that is staying to send it again.
		stopSchedules()
		drained := make(chan struct{})
		go func() {
			defer close(drained)
			drainCtx, stopDrain := context.WithTimeout(shutdownCtx, 5*time.Second)
			defer stopDrain()
			sched.Drain(drainCtx)
		}()
		defer func() { <-drained }()
		err := plains[certs.PlaneApp].Shutdown(shutdownCtx)
		if aerr := plains[certs.PlaneConsole].Shutdown(shutdownCtx); err == nil {
			err = aerr
		}
		if ierr := plains[certs.PlaneInternal].Shutdown(shutdownCtx); err == nil {
			err = ierr
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

// healthz is LIVENESS: is this process still running and serving?
//
// It answers UP unconditionally, and that is the correct answer rather than a
// lazy one. Liveness decides whether to KILL the process. A liveness probe
// that failed because the database was unreachable would turn a database blip
// into a restart of every node at once - each one killed for a fault none of
// them has and none of them can fix by dying. What belongs to a dependency
// belongs to readiness, which decides whether to send traffic.
// afterWrites calls changed once a request that may have written has been
// served - by anything on mux except the pattern named proxied.
//
// It is how the identity the router remembers learns it is stale, and it sits
// HERE rather than in the handlers on purpose: accounts, organisations,
// memberships, groups and roles are written from dozens of them, today's and
// tomorrow's, and a cache dropped one handler at a time is a stale role the day
// somebody forgets one. The cost is precision - any write forgets everything -
// and writes are what a gateway sees least.
//
// The router's own pattern is left out: a POST proxied to an application is
// that application's business, and the one kind of write that arrives by the
// thousand. So are the paths under each of quiet: writes that touch no
// identity, made by programs at their own pace.
func afterWrites(mux *http.ServeMux, proxied string, changed func(), quiet ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return
		}
		if proxied != "" {
			if _, pattern := mux.Handler(r); pattern == proxied {
				return
			}
		}
		for _, prefix := range quiet {
			if strings.HasPrefix(r.URL.Path, prefix) {
				return
			}
		}
		changed()
	})
}

// whilePaused answers a request with refuse while the gateway is paused for a
// database move (store/pause.go). The probes go through: a paused node is
// alive and ready, and an orchestrator that restarted it would end the pause on
// the very database it is leaving.
//
// refuses says which requests it turns away: on the data plane, all of them.
func whilePaused(st *store.Store, refuses func(*http.Request) bool, refuse http.HandlerFunc, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if st.Paused() && r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && refuses(r) {
			refuse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// refusesWhilePaused says what the control plane turns away during a pause:
// every write but the ones the move is made of - resuming, testing the target
// (a POST that reads only), copying the database out, and dismissing the copy
// it remembers (which the store writes past the pause, store/lastcopy.go). Reading goes on: the console stays open
// on what it shows, and root keeps the screen that ends the pause.
func refusesWhilePaused(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	switch r.URL.Path {
	case "/api/backup/pause", "/api/backup/check", "/api/backup/copy", "/api/backup/last-copy":
		return false
	}
	return true
}

// refusePaused is the control plane's answer to a write during a pause.
func refusePaused(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "60")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": store.ErrPaused.Error()})
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":"UP","version":%q}`, version.Version)
}

// readyz is READINESS: can this node take a request right now?
//
// Two questions, and both were answered "yes" by a handler that looked at
// nothing. A node whose store is gone still holds its compiled table and still
// answers - so it declared itself ready while it could no longer resolve a
// session, read a setting or reload a route. And a node that has accepted
// connections but not yet compiled its first table answers 404 to everything,
// which a rolling update reads as a healthy instance and feeds live traffic.
//
// 503 with the reason, because an operator reading a probe failure needs to
// know which of the two it is.
func readyz(st *store.Store, router *gateway.Router) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Short, and shorter than any sane probe period: a readiness check
		// that hangs is a readiness check that says nothing, and the
		// orchestrator's own timeout then decides on no information at all.
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		if err := st.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"DOWN","reason":"the store is not answering","version":%q}`, version.Version)
			return
		}
		if router != nil && !router.Ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"DOWN","reason":"the routing table has not been compiled yet","version":%q}`, version.Version)
			return
		}
		_, _ = fmt.Fprintf(w, `{"status":"UP","version":%q}`, version.Version)
	}
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

// hostname names this node for the schedules it claims. Unknown rather than
// empty when the OS will not say: a run claimed by "" reads as a bug.
func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
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
	// The retention is root's to choose (AUD-02), read at every pass: shortened
	// today, the older events go at the next one.
	retention := time.Duration(st.AuditRetentionDays(ctx)) * 24 * time.Hour
	if n, err := st.PurgeAuditEventsBefore(ctx, time.Now().Add(-retention).Unix()); err != nil {
		slog.Error("audit purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged old audit events", "count", n)
	}
	// The SAML assertions spent (AUTH-19), once they could not be posted
	// again anyway.
	if _, err := st.PurgeSpentAssertions(ctx, time.Now().Unix()); err != nil {
		slog.Error("spent assertion purge failed", "err", err)
	}
	// The one-shot challenges (passkey ceremonies, the sign-up captcha), the
	// trusted browsers past their trust, the agents' OAuth codes and refresh
	// tokens, and the OIDC provider's: all were written with an expiry and,
	// until now, swept by nobody - spent ones go when taken, and the ones
	// abandoned halfway stayed for ever.
	if _, err := st.PurgeExpiredChallenges(ctx, time.Now().Unix()); err != nil {
		slog.Error("challenge purge failed", "err", err)
	}
	if _, err := st.PurgeExpiredTrustedBrowsers(ctx, time.Now().Unix()); err != nil {
		slog.Error("trusted browser purge failed", "err", err)
	}
	if err := st.PurgeOAuthLeftovers(ctx); err != nil {
		slog.Error("oauth leftover purge failed", "err", err)
	}
	if _, err := st.PurgeOIDCLeftovers(ctx, time.Now().Unix()); err != nil {
		slog.Error("oidc leftover purge failed", "err", err)
	}
	// The notifications past the life their sender gave them (NOTIF-05), with
	// each recipient's state: personal data is not kept beyond it.
	if n, err := st.PurgeNotifications(ctx, time.Now().Unix()); err != nil {
		slog.Error("notification purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged expired notifications", "count", n)
	}
	// The audited calls of the data plane (AUD-04), on their own, shorter
	// lifetime: the one kind of the trail whose volume follows the traffic.
	calls := time.Duration(st.EndpointCallRetentionDays(ctx)) * 24 * time.Hour
	if n, err := st.PurgeEndpointCallsBefore(ctx, time.Now().Add(-calls).Unix()); err != nil {
		slog.Error("audited call purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged old audited calls", "count", n)
	}
	// The delayed actions that have been and gone (SCHED-02). Kept a while so
	// an operator can see that they went out, swept when nobody is looking at
	// them any more - the retention is root's, like the trail's.
	kept := time.Duration(st.ScheduleRetentionDays(ctx)) * 24 * time.Hour
	if n, err := st.PurgeFinishedSchedules(ctx, time.Now().Add(-kept).Unix()); err != nil {
		slog.Error("finished schedule purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged finished schedules", "count", n)
	}
	// And the history of what each turn did (SCHED-03), on the same
	// retention: it is the same question - how long does this gateway
	// remember what it did.
	if n, err := st.PurgeScheduleRuns(ctx, time.Now().Add(-kept).Unix()); err != nil {
		slog.Error("schedule history purge failed", "err", err)
	} else if n > 0 {
		slog.Debug("purged old schedule runs", "count", n)
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
