package admin

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/oidcprovider"
	"github.com/softwarity/meerkat/internal/signing"
	"github.com/softwarity/meerkat/internal/store"
)

// The internal plane (:7070): what the gateway serves to the backends of its
// own cluster, and to nobody else.
//
// Three ports, three audiences. The application port serves people and the
// programs they run; the control plane serves the operators and their agents;
// this one serves the services behind the gateway. It used to be the control
// plane that did both of the last two, and that tied the scheduler's API to
// wherever the console was published: an operator who opened the console on
// the company network opened the API services write their schedules with to
// the company network too. Here, the port is a ClusterIP no ingress points
// at, and a NetworkPolicy can keep the console away from the backends without
// keeping the backends away from what they need.
//
// "Internal" is a place, not a credential. Every pod of every namespace
// reaches a ClusterIP unless something says otherwise, so the rule here is
// the same as everywhere else: nothing on this plane gives any power without
// a token. The JWKS needs none because it gives none - public keys, which a
// service must be able to read before it holds anything at all.
//
// It answers TOKENS only. A browser has no business here, and refusing the
// cookie outright means no session minted for the console ever reaches the
// scheduler through a side door.

// RegisterInternal mounts the internal plane on mux: the JWKS, and the whole
// scheduler API behind tokensOnly.
func (a *API) RegisterInternal(mux Mux) {
	mux.HandleFunc("GET "+gateway.JWKSPath, a.router.ServeJWKS)
	// The discovery document (SAUTH-04): the issuer, and where its keys are.
	// What a library that starts from the issuer reads first - which is why
	// it is worth setting the issuer to this plane's own URL.
	mux.HandleFunc("GET "+DiscoveryPath, a.serveDiscovery)
	// The OpenID provider's server-to-server endpoints (SAUTH-05), when this
	// binary carries one: an application's server exchanges its codes here,
	// inside the cluster. Not behind tokensOnly - a client proves itself by
	// its secret or its PKCE verifier, which is the provider's to check.
	if a.OIDC != nil {
		a.OIDC.MountInternal(mux)
	}
	for _, e := range []struct {
		pattern string
		h       userHandler
	}{
		{"GET /api/schedules", a.listSchedules},
		{"POST /api/schedules", a.createSchedule},
		{"GET /api/schedules/{id}", a.getSchedule},
		{"PUT /api/schedules/{id}", a.putSchedule},
		{"DELETE /api/schedules/{id}", a.deleteSchedule},
		{"POST /api/schedules/{id}/pause", a.pauseSchedule},
		{"POST /api/schedules/{id}/resume", a.resumeSchedule},
		{"POST /api/schedules/{id}/run", a.runSchedule},
		{"GET /api/schedules/{id}/runs", a.listScheduleRuns},
		{"PATCH /api/schedules/{id}/run", a.reportRun},
	} {
		mux.Handle(e.pattern, tokensOnly(a.schedules(e.h)))
	}
}

// tokensOnly refuses a request that carries no bearer token, and drops the
// cookies of one that does: on this plane the token is the whole identity.
func tokensOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			writeErr(w, http.StatusUnauthorized,
				"the internal plane answers a token (Authorization: Bearer ...), never a browser session")
			return
		}
		r.Header.Del("Cookie")
		next.ServeHTTP(w, r)
	})
}

// consoleSide marks the scheduler endpoints the control plane keeps: the
// console READS the schedules and acts on them - pause, resume, run now,
// replay a turn, delete - and writes none. A service token still arriving
// here is a backend configured before the internal plane: it is served, and
// told once where its door is now.
func (a *API) consoleSide(next userHandler) userHandler {
	return func(w http.ResponseWriter, r *http.Request, actor store.User) {
		a.warnMoved(r)
		next(w, r, actor)
	}
}

// serviceTokens is the guard of what only a service writes, on the control
// plane: the schedules guard, then serviceSide.
func (a *API) serviceTokens(next userHandler) http.Handler {
	return a.schedules(a.serviceSide(next))
}

// serviceSide marks what only a service writes - the schedule itself, and the
// report on the run it is working on. On the control plane it is left only to
// the service tokens that wrote there before the internal plane existed, for
// the time it takes their backends to move. Nobody else writes a schedule
// here: the console never did, and an operator's token is not a service's.
func (a *API) serviceSide(next userHandler) userHandler {
	return func(w http.ResponseWriter, r *http.Request, actor store.User) {
		if tokenScope(r.Context()) != store.ScopeSchedules {
			writeErr(w, http.StatusForbidden,
				"a schedule is written by the service it calls, on the internal plane (port "+
					portOf(a.InternalAddr)+", "+schedulesPath+"), with a token whose perimeter is "+
					store.ScopeSchedules+"; the console reads schedules and acts on them, it writes none")
			return
		}
		a.warnMoved(r)
		next(w, r, actor)
	}
}

// warnMoved says it once per token and per process: a backend calling every
// minute must not turn the journal into a list of the same sentence.
func (a *API) warnMoved(r *http.Request) {
	if tokenScope(r.Context()) != store.ScopeSchedules {
		return
	}
	name := actorToken(r.Context())
	if _, seen := a.movedWarned.LoadOrStore(name, true); seen {
		return
	}
	slog.Warn("a service calls the scheduler on the control plane; its address is now the internal plane",
		"token", name, "path", r.URL.Path, "use", "http://<gateway>:"+portOf(a.InternalAddr)+schedulesPath)
}

// DiscoveryPath is where the internal plane publishes its discovery document.
const DiscoveryPath = "/.well-known/openid-configuration"

// discoveryDocument is what a token's verifier needs, and nothing it would
// take for a promise. Without the OpenID provider there is no authorization,
// token or userinfo endpoint: the gateway then signs one short token per
// request it forwards and is no OAuth server. The fields are the ones a
// library that starts from the issuer reads (Spring's issuer-uri, go-oidc):
// the issuer to compare, the keys to verify with, the algorithms they sign.
//
// With the provider active (SAUTH-05, Enterprise) the document also names its
// endpoints, because an application's SERVER reads it here, inside the
// cluster: the authorize endpoint is the public one - it is the person's
// browser that goes there - and the token, userinfo and keys endpoints are on
// this plane, at the address the server used, so a code is exchanged without
// going out through the public name and back.
type discoveryDocument struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string   `json:"token_endpoint,omitempty"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint,omitempty"`
	JWKSURI               string   `json:"jwks_uri"`
	EndSessionEndpoint    string   `json:"end_session_endpoint,omitempty"`
	ResponseTypes         []string `json:"response_types_supported,omitempty"`
	GrantTypes            []string `json:"grant_types_supported,omitempty"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported,omitempty"`
	TokenAuthMethods      []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	Scopes                []string `json:"scopes_supported,omitempty"`
	Algorithms            []string `json:"id_token_signing_alg_values_supported"`
	Subjects              []string `json:"subject_types_supported"`
}

// serveDiscovery answers with the issuer the gateway signs with, and the JWKS
// at the address the caller used to get here: inside a cluster that is the
// gateway's name there, which is the one a service can reach.
func (a *API) serveDiscovery(w http.ResponseWriter, r *http.Request) {
	base := filters.Scheme(r) + "://" + r.Host
	doc := discoveryDocument{
		Issuer:     a.router.Issuer(),
		JWKSURI:    base + gateway.JWKSPath,
		Algorithms: signing.Algorithms,
		Subjects:   []string{"public"},
	}
	if a.OIDC != nil {
		if e, ok := a.OIDC.Endpoints(r.Context(), base); ok {
			doc.AuthorizationEndpoint = e.Authorization
			doc.TokenEndpoint = e.Token
			doc.UserinfoEndpoint = e.Userinfo
			doc.EndSessionEndpoint = e.EndSession
			doc.ResponseTypes = oidcprovider.ResponseTypes
			doc.GrantTypes = oidcprovider.GrantTypes
			doc.CodeChallengeMethods = oidcprovider.CodeChallengeMethods
			doc.TokenAuthMethods = oidcprovider.TokenAuthMethods
			doc.Scopes = oidcprovider.Scopes
		}
	}
	// Not cached by anything in between while the provider can answer: the
	// document then depends on the provider's state, and on the host asked.
	if doc.TokenEndpoint != "" {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	writeJSON(w, http.StatusOK, doc)
}

// internalOriginAt is the runtime's answer, for which port, and when.
type internalOriginAt struct {
	discovery.Internal
	port int
	at   time.Time
}

// internalOriginOf is the internal plane as the runtime names it, "" when it
// cannot tell. Asked when a screen needs it rather than on a timer: the name
// only moves when somebody renames the Service, and the console only shows it
// in two drawers. Kept a minute, so a burst of screens is one question to the
// runtime and not one each.
//
// Once a certificate is placed on the internal plane (SSL-12), it is the HTTPS
// door that is named: https, and the port the runtime sends to 7443.
func (a *API) internalOriginOf(ctx context.Context) string {
	return a.internalOf(ctx).Origin
}

// internalOf is the runtime's whole answer: the address, every name it is
// reached by, and which runtime gave it - what the console's how-to picks its
// platform by.
//
// Over HTTPS the address names what the certificate carries: of the runtime's
// names, the first one the internal plane's certificate covers. A certificate
// written for the short name of a Swarm stack advertises that one, and the
// services told it are told a name their handshake accepts.
func (a *API) internalOf(ctx context.Context) discovery.Internal {
	addr, secure := a.InternalAddr, false
	if a.TLS != nil && a.TLS.State().Internal && a.InternalTLSAddr != "" {
		addr, secure = a.InternalTLSAddr, true
	}
	port, _ := strconv.Atoi(portOf(addr))
	var in discovery.Internal
	if c := a.internalOrigin.Load(); c != nil && c.port == port && time.Since(c.at) < time.Minute {
		in = c.Internal
	} else {
		lookup := a.LookupInternal
		if lookup == nil {
			lookup = discovery.InternalOrigin
		}
		in = lookup(ctx, port)
		a.internalOrigin.Store(&internalOriginAt{Internal: in, port: port, at: time.Now()})
	}
	if secure && strings.HasPrefix(in.Origin, "http://") {
		in.Origin = "https://" + strings.TrimPrefix(in.Origin, "http://")
		if n := a.presentedName(ctx, in.Names); n != "" && n != in.Names[0] {
			in.Origin = strings.Replace(in.Origin, "://"+in.Names[0]+":", "://"+n+":", 1)
		}
	}
	return in
}

// presentedName is the first of names the internal plane's certificate
// covers, "" when none is placed or none covers one.
func (a *API) presentedName(ctx context.Context, names []string) string {
	rows, err := a.st.ListCertificates(ctx)
	if err != nil {
		return ""
	}
	for _, n := range names {
		for _, c := range rows {
			if c.Internal && !c.Pending() && certs.Covers(c.Names(), n) {
				return n
			}
		}
	}
	return ""
}

// internalNames are the names the runtime resolves to the internal plane, the
// advertised one first; none when it cannot tell.
func (a *API) internalNames(ctx context.Context) []string {
	return a.internalOf(ctx).Names
}

// onlyInternal reports whether every name a certificate carries is one of the
// internal plane's: what no visitor of the console or the application uses.
func onlyInternal(names, internal []string) bool {
	if len(names) == 0 {
		return false
	}
	for _, n := range names {
		if !slices.ContainsFunc(internal, func(i string) bool { return strings.EqualFold(i, n) }) {
			return false
		}
	}
	return true
}

// coversOne reports whether a certificate carrying names answers for at least
// one of hosts.
func coversOne(names, hosts []string) bool {
	for _, h := range hosts {
		if certs.Covers(names, h) {
			return true
		}
	}
	return false
}
