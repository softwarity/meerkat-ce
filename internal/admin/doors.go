package admin

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/store"
)

// The planes as the runtime names them, and what the operator says about the
// console's and the application's (SSL-14, certs.Door).
//
// Each plane is reached inside the runtime by the name of what fronts its
// port - the chart's Service <release>, <release>-admin, <release>-internal;
// one Swarm or compose service for every port. Those names are what a
// certificate re-encrypting the hop from a proxy carries, and what the screen
// lists under each plane.

// PlaneNames are the names the runtime resolves to one plane's port, the
// advertised one first; none when it cannot tell. The supervisor asks it to
// know which certificate a proxied door serves the hop with.
func (a *API) PlaneNames(ctx context.Context, plane string) []string {
	return a.planeRuntime(ctx, plane).Names
}

// planeRuntime is the runtime's whole answer for one plane.
func (a *API) planeRuntime(ctx context.Context, plane string) discovery.Internal {
	if plane == store.PlaneInternal {
		return a.internalOf(ctx)
	}
	var addr string
	if a.TLS != nil {
		st := a.TLS.State()
		addr = st.AppPlainAddr
		if plane == store.PlaneConsole {
			addr = st.ConsolePlainAddr
		}
	}
	port, err := strconv.Atoi(portOf(addr))
	if err != nil || port == 0 {
		return discovery.Internal{Why: "this plane's port is not known here"}
	}
	if c, ok := a.planeOrigins.Load(port); ok {
		if at := c.(*internalOriginAt); time.Since(at.at) < time.Minute {
			return at.Internal
		}
	}
	lookup := a.LookupInternal
	if lookup == nil {
		lookup = discovery.InternalOrigin
	}
	in := lookup(ctx, port)
	a.planeOrigins.Store(port, &internalOriginAt{Internal: in, port: port, at: time.Now()})
	return in
}

// doorView is one plane as the Domains & TLS screen shows it.
type doorView struct {
	// Proxied, Names and ProxyAddresses are what the operator said (none for
	// the internal plane).
	Proxied        bool     `json:"proxied"`
	Names          []string `json:"names"`
	ProxyAddresses []string `json:"proxyAddresses"`
	// Runtime is how the runtime names the plane: its names, which runtime
	// said so, or why none could.
	Runtime discovery.Internal `json:"runtime"`
	// Uncovered are the declared names no certificate served on the plane
	// covers - only when the gateway serves them itself. Saving refuses
	// them; one can still be left by a certificate taken off since.
	Uncovered []string `json:"uncovered"`
}

// doorsView is the three planes.
type doorsView struct {
	Console  doorView `json:"console"`
	App      doorView `json:"app"`
	Internal doorView `json:"internal"`
}

func (a *API) doorsView(ctx context.Context, cfg certs.Settings) doorsView {
	rows, _ := a.st.ListCertificates(ctx)
	view := func(plane string) doorView {
		d := cfg.DoorOn(plane)
		rt := a.planeRuntime(ctx, plane)
		rt.Names = orEmpty(rt.Names)
		v := doorView{Proxied: d.Proxied, Names: orEmpty(d.Names), ProxyAddresses: orEmpty(d.ProxyAddresses),
			Runtime: rt, Uncovered: []string{}}
		if !d.Proxied {
			for _, n := range d.Names {
				if !coveredOn(rows, cfg, plane, n) {
					v.Uncovered = append(v.Uncovered, n)
				}
			}
		}
		return v
	}
	return doorsView{Console: view(store.PlaneConsole), App: view(store.PlaneApp), Internal: view(store.PlaneInternal)}
}

// coveredOn says whether a certificate placed on plane - a row, or an order
// to an authority - covers the name.
func coveredOn(rows []store.Certificate, cfg certs.Settings, plane, name string) bool {
	for _, c := range rows {
		if c.On(plane) && !c.Pending() && certs.Covers(c.Names(), name) {
			return true
		}
	}
	for _, o := range cfg.ACME.Orders {
		if ((plane == store.PlaneConsole && o.Console) || (plane == store.PlaneApp && o.App)) && certs.Covers(o.Names, name) {
			return true
		}
	}
	return false
}

// putDoor says, for the console or the application, who serves its public
// names and which they are (SSL-14). Saving is applying: the door's
// certificates, its trust in forwarded headers and its https names follow on
// every node.
func (a *API) putDoor(w http.ResponseWriter, r *http.Request, actor store.User) {
	plane := r.PathValue("plane")
	if plane != store.PlaneConsole && plane != store.PlaneApp {
		writeErr(w, http.StatusNotFound, fmt.Sprintf(
			"plane %q: expected %s or %s - the internal plane is reached by its runtime names only",
			plane, store.PlaneConsole, store.PlaneApp))
		return
	}
	var p struct {
		Proxied        bool     `json:"proxied"`
		Names          []string `json:"names"`
		ProxyAddresses []string `json:"proxyAddresses"`
	}
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed door: "+err.Error())
		return
	}
	names, err := proxiedNames(p.Names)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	_, addresses, err := certs.ProxyAddresses(p.ProxyAddresses)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "proxy addresses: "+err.Error())
		return
	}
	cfg := a.st.RawTLS(r.Context())
	other := store.PlaneConsole
	if plane == store.PlaneConsole {
		other = store.PlaneApp
	}
	for _, n := range names {
		if slices.Contains(cfg.DoorOn(other).Names, n) {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
				"%s is already the %s's: a name reaches one plane", n, planeWord(other)))
			return
		}
	}
	// Served by this gateway, a name ADDED has a certificate to present. The
	// names already there stay through a switch of mode - from a proxy back
	// to this gateway, say - and are shown uncovered until one is placed:
	// refusing them would make the switch the one step nobody can take.
	if !p.Proxied {
		rows, err := a.st.ListCertificates(r.Context())
		if err != nil {
			a.internal(w, err)
			return
		}
		had := cfg.DoorOn(plane).Names
		var bare []string
		for _, n := range names {
			if !slices.Contains(had, n) && !coveredOn(rows, cfg, plane, n) {
				bare = append(bare, n)
			}
		}
		if len(bare) > 0 {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
				"no certificate placed on the %s covers %s: place one that carries it, or let a proxy in front serve it",
				planeWord(plane), strings.Join(bare, ", ")))
			return
		}
	}
	next := certs.Door{Proxied: p.Proxied, Names: names, ProxyAddresses: addresses}
	// The console's door is the one this request came in by. A proxy in front
	// told Meerkat the browser's hop is HTTPS: a door that stops believing it
	// sees this very session as plain http - its cookie no longer comes back,
	// a passkey's origin no longer matches - and the operator is signed out
	// with no way back in but the database (2026-10-15, on the eval).
	if plane == store.PlaneConsole && filters.Secure(r) {
		var https []string
		if next.Proxied {
			https = next.Names
		}
		proxies, _ := next.Trusted()
		if !filters.SecureUnder(r, proxies, https) {
			writeErr(w, http.StatusConflict, fmt.Sprintf(
				"you reach the console through a proxy in front of it (%s, over HTTPS): with this door Meerkat would no longer believe it, "+
					"see your own session as plain http and sign you out. Keep A proxy in front, or open the console on its HTTPS port first",
				filters.PeerIP(r)))
			return
		}
	}
	before := doorAudited(cfg.DoorOn(plane))
	cfg.SetDoor(plane, next)
	if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
		a.internal(w, err)
		return
	}
	// Through the supervisor, which every node reloads on the change bus.
	a.applyTLS(r.Context())
	a.auditUpdate(r.Context(), actor, "tls.door", "settings", plane, planeWord(plane), "",
		before, doorAudited(cfg.DoorOn(plane)))
	writeJSON(w, http.StatusOK, a.tlsView(r.Context()))
}

// doorAudited is a door as the trail compares it, its lists never null.
type doorAudit struct {
	Proxied        bool     `json:"proxied"`
	Names          []string `json:"names"`
	ProxyAddresses []string `json:"proxyAddresses"`
}

func doorAudited(d certs.Door) doorAudit {
	return doorAudit{Proxied: d.Proxied, Names: orEmpty(d.Names), ProxyAddresses: orEmpty(d.ProxyAddresses)}
}

// putPorts switches a plane's plain-HTTP door on or off (SSL-15). HTTPS has
// no switch: it opens with a certificate and closes with the last one. HTTP
// goes off only while HTTPS serves, or the plane would close without a word.
//
// On the console the door in use is not switched off at all - the one this
// request came through, or one called within the traffic window: the
// operator reaches the console through HTTPS first, then switches HTTP off.
// Application and internal: free, as long as HTTPS serves.
func (a *API) putPorts(w http.ResponseWriter, r *http.Request, actor store.User) {
	plane := r.PathValue("plane")
	if !slices.Contains(store.CertPlanes, plane) {
		writeErr(w, http.StatusNotFound, fmt.Sprintf("plane %q: expected %s", plane, strings.Join(store.CertPlanes, ", ")))
		return
	}
	var p struct {
		HTTP bool `json:"http"`
	}
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed ports: "+err.Error())
		return
	}
	cfg := a.st.RawTLS(r.Context())
	if !p.HTTP && cfg.Opens(plane, "http") {
		if !a.httpsServes(plane) {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
				"no certificate serves HTTPS on the %s: switching HTTP off would close it - add one first", planeWord(plane)))
			return
		}
		if plane == store.PlaneConsole {
			if why := a.consoleInUse(r, "http"); why != "" {
				writeErr(w, http.StatusConflict, why)
				return
			}
		}
	}
	before := orEmpty(cfg.Closed[plane])
	cfg.SetOpen(plane, "http", p.HTTP)
	if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditUpdate(r.Context(), actor, "tls.ports", "settings", plane, planeWord(plane), "",
		map[string]any{"closed": before}, map[string]any{"closed": orEmpty(cfg.Closed[plane])})
	writeJSON(w, http.StatusOK, a.tlsView(r.Context()))
}

// consolePort is one of the console's doors as an operator reads it.
func (a *API) consolePort(scheme string) string {
	if a.TLS == nil {
		return strings.ToUpper(scheme)
	}
	st := a.TLS.State()
	addr := st.ConsolePlainAddr
	if scheme == "https" {
		addr = st.ConsoleAddr
	}
	return strings.ToUpper(scheme) + ":" + portOf(addr)
}

// consoleInUse says why the console's door of scheme is in use - this very
// request came through it, or something called it within the traffic window
// - and what to do first; "" when it is not.
func (a *API) consoleInUse(r *http.Request, scheme string) string {
	other := "https"
	if scheme == "https" {
		other = "http"
	}
	here := "http"
	if r.TLS != nil {
		here = "https"
	}
	first := fmt.Sprintf("access the console through %s first", a.consolePort(other))
	if here == scheme {
		return fmt.Sprintf("you reach the console through %s: %s", a.consolePort(scheme), first)
	}
	if a.DoorTraffic != nil {
		if peer, ok := a.DoorTraffic.Receiving(store.PlaneConsole, scheme); ok {
			return fmt.Sprintf("%s still receives requests (from %s, in the last %d s): %s - and point your proxy at it",
				a.consolePort(scheme), peer, int(certs.TrafficWindow/time.Second), first)
		}
	}
	return ""
}

// lastCertificateRefusal says why taking row off the planes it leaves would
// close a door that must stay: its plane's last HTTPS certificate, on a plane
// whose HTTP is off (nothing would answer), or on the console while its HTTPS
// is in use. "" when nothing stands in the way.
func (a *API) lastCertificateRefusal(r *http.Request, row store.Certificate, leaving []string) string {
	if len(leaving) == 0 || a.TLS == nil {
		return ""
	}
	ctx := r.Context()
	cfg := a.st.RawTLS(ctx)
	rows, err := a.st.ListCertificates(ctx)
	if err != nil {
		return ""
	}
	for _, plane := range leaving {
		if !a.httpsServes(plane) {
			continue
		}
		door, runtime := cfg.DoorOn(plane), a.PlaneNames(ctx, plane)
		kept := false
		for _, c := range rows {
			if c.ID != row.ID && c.On(plane) && !c.Pending() && door.Active(c.Names(), runtime) {
				kept = true
				break
			}
		}
		if kept {
			continue
		}
		if !cfg.Opens(plane, "http") {
			return fmt.Sprintf("this is the %s's last HTTPS certificate and its HTTP is off: switch HTTP on first, or nothing would answer",
				planeWord(plane))
		}
		if plane == store.PlaneConsole {
			if why := a.consoleInUse(r, "https"); why != "" {
				return "this is the console's last HTTPS certificate and " + why
			}
		}
	}
	return ""
}

// httpsServes says whether a plane's HTTPS door is open now.
func (a *API) httpsServes(plane string) bool {
	if a.TLS == nil {
		return false
	}
	st := a.TLS.State()
	switch plane {
	case store.PlaneConsole:
		return st.Console
	case store.PlaneApp:
		return st.App
	}
	return st.Internal
}
