package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/store"
)

// Who may call the scheduled calls API on the internal plane (SCHED-01).
//
// It used to be one answer: a token of perimeter schedules, minted by an
// administrator and handed to every service. That is a burden exactly where it
// buys least - a cluster where every pod trusts every other one - so the
// answer is now a setting, in two parts (store/internalaccess.go): how a
// caller is recognised, and which of them are let in.
//
//   - none: nothing is asked. A Meerkat token is still read when one is sent,
//     a ServiceAccount token is checked with the API server, and otherwise the
//     address is followed to the workload or container holding it.
//   - token: a Meerkat token of perimeter schedules, as before.
//   - cluster: the pod's own ServiceAccount token, which the API server
//     vouches for. Kubernetes only.
//
// A credential that is SENT is a credential that is checked, in every mode: a
// revoked token does not become an anonymous caller by being revoked, and a
// bearer nobody vouches for is refused rather than ignored.
//
// The address is the TCP peer, never a forwarded header. Nothing stands in
// front of this plane - it is a ClusterIP - and the private ranges a pod lives
// in are the ones the forwarding rule trusts by default, so a header here
// would be a pod choosing whose name it calls under.
//
// The JWKS and the discovery document are outside all of this: public keys,
// open in every mode. And the control plane's compatibility door for the
// services that have not moved (schedules.go) keeps answering their token and
// nothing else, whatever the mode here.

// CallerFinder is what the internal plane asks the runtime about a caller
// (discovery.Callers): a test hands it a fake rather than reach a cluster.
type CallerFinder interface {
	Kubernetes() bool
	Review(ctx context.Context, token string) (string, error)
	ByAddress(ctx context.Context, ip string) (identity, via string)
	ServiceAccounts(ctx context.Context) ([]string, error)
	Workloads(ctx context.Context) ([]string, error)
}

// internalCaller is who wrote a request on the internal plane, as far as the
// mode could tell: the identity in its mode's form, how it was known, and the
// address it came from.
type internalCaller struct {
	Identity string
	Via      string
	Addr     string
}

// words says who it was, for the audit and for a refusal.
func (c internalCaller) words() string {
	switch c.Via {
	case store.CallerToken:
		return "token " + c.Identity
	case store.CallerServiceAccount:
		return "ServiceAccount " + c.Identity
	case store.CallerWorkload, store.CallerContainer:
		return c.Identity + ", deduced from its address " + c.Addr
	}
	return "an unidentified caller at " + c.Addr
}

// actor is the account a request without a token acts as: nobody, named by
// what is known of it - which is what the trail then shows, as its actor id.
// A space in it on purpose: no account id has one, so the trail can never
// join it to a person - a container called "root" stays a container.
func (c internalCaller) actor() store.User {
	var label string
	switch c.Via {
	case store.CallerServiceAccount:
		label = "ServiceAccount " + c.Identity
	case store.CallerWorkload:
		label = "workload " + c.Identity
	case store.CallerContainer:
		label = "container " + c.Identity
	default:
		label = "unidentified caller " + c.Addr
	}
	return store.User{ID: label, Username: label}
}

type callerKey struct{}
type accessKey struct{}

// callerOf is who wrote this request. On the internal plane the funnel below
// says; on the control plane's compatibility door it is the token, the only
// caller that door answers.
func callerOf(r *http.Request) internalCaller {
	if c, ok := r.Context().Value(callerKey{}).(internalCaller); ok {
		return c
	}
	if name := actorToken(r.Context()); name != "" {
		return internalCaller{Identity: name, Via: store.CallerToken, Addr: filters.PeerIP(r)}
	}
	return internalCaller{Via: store.CallerUnidentified, Addr: filters.PeerIP(r)}
}

// door is one API of the internal plane and what guards it: the token
// perimeter that opens it, the setting that says who may call it, and the
// screen an operator changes that setting on. The scheduler was the first;
// the notifications (NOTIF-05) are the second, with their own setting - the
// services that notify are not the ones that schedule.
type door struct {
	scope  string
	where  string
	access func(context.Context) store.InternalAccess
}

// schedulesDoor is the scheduled calls API (SCHED-01).
func (a *API) schedulesDoor() door {
	return door{scope: store.ScopeSchedules, where: "Scheduler, Access", access: a.st.InternalAccess}
}

// notificationsDoor is the notifications API (NOTIF-05).
func (a *API) notificationsDoor() door {
	return door{scope: store.ScopeNotifications, where: "Notifications, Access",
		access: func(ctx context.Context) store.InternalAccess { return a.st.NotifySettings(ctx).Access }}
}

// internalSchedules is the funnel of the scheduler API on the internal plane.
func (a *API) internalSchedules(next userHandler) http.Handler {
	return a.internalDoor(a.schedulesDoor(), next)
}

type doorKey struct{}

// throughDoor says which door of the internal plane a request came in by, ""
// on the control plane: what lets a notifications token through authed here
// and nowhere else.
func throughDoor(ctx context.Context) string {
	s, _ := ctx.Value(doorKey{}).(string)
	return s
}

// internalDoor is the funnel of one API on the internal plane: the mode
// decides what a caller must present, the list who gets through.
func (a *API) internalDoor(d door, next userHandler) http.Handler {
	byToken := a.perimeter(d.scope, a.tokenCaller(d, next))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A browser has no business here in any mode: its cookie is dropped,
		// so no console session ever reaches the scheduler through a side
		// door - it would be an unidentified caller at best.
		r.Header.Del("Cookie")
		access := d.access(r.Context())
		ctx := context.WithValue(r.Context(), accessKey{}, access)
		r = r.WithContext(context.WithValue(ctx, doorKey{}, d.scope))
		bearer := bearerOf(r)
		meerkat := strings.HasPrefix(bearer, "mk_")
		switch {
		case access.Mode == store.InternalAccessCluster:
			a.clusterCaller(w, r, d, access, bearer, next)
		case meerkat:
			byToken.ServeHTTP(w, r)
		case access.Mode == store.InternalAccessToken:
			writeErr(w, http.StatusUnauthorized,
				"the internal plane asks for a Meerkat token: send one of perimeter "+d.scope+
					" (Authorization: Bearer mk_...), minted on the console under Infra, Access tokens, Internal")
		default:
			a.noneCaller(w, r, d, access, bearer, next)
		}
	})
}

// perimeter admits an application administrator, and a token whose perimeter
// is exactly this door's.
func (a *API) perimeter(scope string, next userHandler) http.Handler {
	if scope == store.ScopeSchedules {
		return a.schedules(next)
	}
	return a.authed(func(w http.ResponseWriter, r *http.Request, actor store.User) {
		if actor.Root || actor.AppAdmin || tokenScope(r.Context()) == scope {
			next(w, r, actor)
			return
		}
		writeErr(w, http.StatusForbidden,
			"this API answers an application administrator, or a token whose perimeter is "+scope)
	})
}

// tokenCaller follows the door's guard when a Meerkat token was sent: the
// token is the identity, and its name is what the list names - or, in None
// mode where the list names services, the workload at its address.
func (a *API) tokenCaller(d door, next userHandler) userHandler {
	return func(w http.ResponseWriter, r *http.Request, actor store.User) {
		access, _ := r.Context().Value(accessKey{}).(store.InternalAccess)
		c := internalCaller{Identity: actorToken(r.Context()), Via: store.CallerToken, Addr: filters.PeerIP(r)}
		if !access.Admits(c.Identity) && !a.admittedByAddress(r.Context(), access, c.Addr) {
			refuseCaller(w, d, access, c)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, c)), actor)
	}
}

// clusterCaller admits a ServiceAccount token the API server vouches for, and
// nothing else.
func (a *API) clusterCaller(w http.ResponseWriter, r *http.Request, d door, access store.InternalAccess, bearer string, next userHandler) {
	if a.Callers == nil || !a.Callers.Kubernetes() {
		writeErr(w, http.StatusServiceUnavailable,
			"the internal plane asks for a cluster token, which the Kubernetes API checks, and this gateway "+
				"does not run on Kubernetes: an infrastructure administrator chooses none or token "+
				"("+d.where+")")
		return
	}
	if bearer == "" || strings.HasPrefix(bearer, "mk_") {
		writeErr(w, http.StatusUnauthorized,
			"the internal plane asks for the calling pod's ServiceAccount token (Authorization: Bearer "+
				"followed by the content of /var/run/secrets/kubernetes.io/serviceaccount/token); "+
				"a Meerkat token is not accepted in this mode")
		return
	}
	c, ok := a.reviewed(w, r, bearer)
	if !ok {
		return
	}
	if !access.Admits(c.Identity) {
		refuseCaller(w, d, access, c)
		return
	}
	next(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, c)), c.actor())
}

// noneCaller asks nothing and works out who is calling: a ServiceAccount
// token when one is sent, the address otherwise.
func (a *API) noneCaller(w http.ResponseWriter, r *http.Request, d door, access store.InternalAccess, bearer string, next userHandler) {
	addr := filters.PeerIP(r)
	var c internalCaller
	if bearer != "" {
		if a.Callers == nil || !a.Callers.Kubernetes() {
			writeErr(w, http.StatusUnauthorized,
				"this bearer is not a Meerkat token (mk_...), and off Kubernetes no other is checked here: "+
					"send a Meerkat token, or none")
			return
		}
		var ok bool
		if c, ok = a.reviewed(w, r, bearer); !ok {
			return
		}
	} else {
		c = internalCaller{Via: store.CallerUnidentified, Addr: addr}
		if a.Callers != nil {
			if id, via := a.Callers.ByAddress(r.Context(), addr); id != "" {
				c.Identity, c.Via = id, via
			}
		}
	}
	admitted := access.Admits(c.Identity)
	if !admitted && c.Via == store.CallerServiceAccount {
		admitted = a.admittedByAddress(r.Context(), access, addr)
	}
	if !admitted {
		refuseCaller(w, d, access, c)
		return
	}
	next(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, c)), c.actor())
}

// reviewed asks the API server about a bearer. A refusal is written here.
func (a *API) reviewed(w http.ResponseWriter, r *http.Request, bearer string) (internalCaller, bool) {
	id, err := a.Callers.Review(r.Context(), bearer)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "the cluster could not check this token: "+err.Error())
		return internalCaller{}, false
	}
	if id == "" {
		writeErr(w, http.StatusUnauthorized,
			"the cluster does not vouch for this bearer as a ServiceAccount token, and it is not a Meerkat "+
				"token (mk_...): send a valid one of either")
		return internalCaller{}, false
	}
	return internalCaller{Identity: id, Via: store.CallerServiceAccount, Addr: filters.PeerIP(r)}, true
}

// admittedByAddress is the second chance None mode gives: its list names
// services, so a caller known by a token or a ServiceAccount is also matched
// as the workload holding its address. Asked only when the first name failed.
func (a *API) admittedByAddress(ctx context.Context, access store.InternalAccess, addr string) bool {
	if access.Mode != store.InternalAccessNone || len(access.Callers) == 0 || a.Callers == nil {
		return false
	}
	id, _ := a.Callers.ByAddress(ctx, addr)
	return id != "" && access.Admits(id)
}

// refuseCaller says who was refused, and who would not be.
func refuseCaller(w http.ResponseWriter, d door, access store.InternalAccess, c internalCaller) {
	allowed := strings.Join(access.Callers, ", ")
	if c.Identity == "" {
		writeErr(w, http.StatusForbidden,
			"this caller could not be identified (address "+c.Addr+": no token, and no workload or "+
				"container holds it), and the internal plane admits only these callers: "+allowed)
		return
	}
	writeErr(w, http.StatusForbidden,
		c.words()+" is not among the callers the internal plane admits ("+allowed+
			"); an infrastructure administrator adds it on the "+d.where)
}

// bearerOf is the bearer sent, "" when none.
func bearerOf(r *http.Request) string {
	b, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(b)
}

// ---- the setting, on the control plane ----

// registerInternalAccess mounts the setting beside the scheduler's other one.
// Read by whoever administers a domain - the Scheduler's readers see it,
// read-only - and changed by the infrastructure's administrators: it decides
// who may reach a port of the gateway, which is the routing plane's business.
func (a *API) registerInternalAccess(mux Mux) {
	mux.Handle("GET /api/settings/internal-access", a.domainAdmin(a.getInternalAccess))
	mux.Handle("PUT /api/settings/internal-access", a.infraAdmin(a.putInternalAccess))
	// What the drawer's list offers in each mode: the workloads, the tokens,
	// the ServiceAccounts.
	mux.Handle("GET /api/settings/internal-access/candidates", a.domainAdmin(a.internalAccessCandidates))
}

// internalAccessView is the setting, and what the screen needs around it.
type internalAccessView struct {
	store.InternalAccess
	// Modes are the ones this platform offers: cluster only on Kubernetes.
	Modes []string `json:"modes"`
	// Platform is what the gateway runs on, as discovery names it.
	Platform string `json:"platform"`
	// Editable says whether this reader may change it.
	Editable bool `json:"editable"`
}

func (a *API) getInternalAccess(w http.ResponseWriter, r *http.Request, actor store.User) {
	modes := []string{store.InternalAccessNone, store.InternalAccessToken}
	if a.onKubernetes() {
		modes = append(modes, store.InternalAccessCluster)
	}
	lookup := a.LookupPlatform
	if lookup == nil {
		lookup = discovery.Runtime
	}
	writeJSON(w, http.StatusOK, internalAccessView{
		InternalAccess: a.st.InternalAccess(r.Context()),
		Modes:          modes,
		Platform:       lookup(r.Context()).Name,
		Editable:       actor.Root || actor.InfraAdmin,
	})
}

func (a *API) putInternalAccess(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in store.InternalAccess
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed internal access: "+err.Error())
		return
	}
	if err := store.SanitizeInternalAccess(&in); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if in.Mode == store.InternalAccessCluster && !a.onKubernetes() {
		writeErr(w, http.StatusUnprocessableEntity,
			"mode cluster checks a ServiceAccount token with the Kubernetes API, and this gateway does not "+
				"run on Kubernetes: allowed here are "+store.InternalAccessNone+" and "+store.InternalAccessToken)
		return
	}
	before := a.st.InternalAccess(r.Context())
	if err := a.st.SetInternalAccess(r.Context(), in); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "settings.internal-access", "settings", "", "", "", before, in)
	a.getInternalAccess(w, r, actor)
}

// internalAccessCandidates lists what fits a mode's list: the workloads and
// containers a caller is deduced as in mode none, the tokens' names in mode
// token, the ServiceAccounts in mode cluster. What cannot be listed is said,
// and the list stays free to type.
func (a *API) internalAccessCandidates(w http.ResponseWriter, r *http.Request, _ store.User) {
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	if mode == "" {
		mode = a.st.InternalAccess(r.Context()).Mode
	}
	out := struct {
		Mode        string   `json:"mode"`
		Candidates  []string `json:"candidates"`
		Unavailable string   `json:"unavailable,omitempty"`
	}{Mode: mode, Candidates: []string{}}
	var list []string
	var err error
	switch mode {
	case store.InternalAccessToken:
		scope := store.ScopeSchedules
		if r.URL.Query().Get("for") == "notifications" {
			scope = store.ScopeNotifications
		}
		list, err = a.st.TokenNamesOfScope(r.Context(), scope)
	case store.InternalAccessNone, store.InternalAccessCluster:
		if a.Callers == nil {
			out.Unavailable = "no runtime to ask on this gateway: type the names"
			break
		}
		if mode == store.InternalAccessNone {
			list, err = a.Callers.Workloads(r.Context())
		} else {
			list, err = a.Callers.ServiceAccounts(r.Context())
		}
	default:
		writeErr(w, http.StatusUnprocessableEntity,
			"mode "+mode+": allowed are "+strings.Join(store.InternalAccessModes, ", "))
		return
	}
	if err != nil {
		out.Unavailable = err.Error()
	}
	if list != nil {
		out.Candidates = list
	}
	writeJSON(w, http.StatusOK, out)
}

// onKubernetes says whether a ServiceAccount token can be checked here.
func (a *API) onKubernetes() bool { return a.Callers != nil && a.Callers.Kubernetes() }
