package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	filtering "github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// UI test mode (DEV-10): a developer browses a UI route AS a simulated
// identity, driven by the user-button's developer bar. The simulation is
// attached to the developer's REAL session and scoped to ONE route, with a
// TTL: every request that session sends through that route carries the
// simulated identity (simKey), so access gates, endpoint security, page
// stamps, identity forwarding and the X-Meerkat-Test upstream markers all
// behave as if that user browsed - the whole point is SEEING what a role
// sees (role-CSS).
//
// The running test rides a COOKIE, signed with the simulation key every node
// reads from the database, bound to the developer's session and to the route.
// It used to sit in each process's memory, and with several replicas the
// Apply landed on one node and the page it reloaded on another, which knew
// nothing of it: the bar came back closed and the page unchanged. Whichever
// node answers now reads the same thing. The TTL ends a forgotten test; so
// does signing out, since the cookie names a session that no longer exists.

// uiSimTTL bounds a forgotten test: past it the route serves normally again.
const uiSimTTL = time.Hour

type uiSimEntry struct {
	user    string
	roles   []string
	by      string // the real developer, for the X-Meerkat-Test-By marker
	expires time.Time
}

// uiSimClaims is what the cookie carries, signed.
type uiSimClaims struct {
	Session string   `json:"s"` // the dev session's token hash
	Route   string   `json:"rt"`
	User    string   `json:"u"`
	Roles   []string `json:"r"`
	By      string   `json:"b"`
	Exp     int64    `json:"e"`
}

// uiSimCookie is the cookie of one route's test, on the scheme r arrived
// over. One per route, named after a hash of its id: an id is not bound to
// the characters a cookie name allows.
func uiSimCookie(r *http.Request, route string) string {
	sum := sha256.Sum256([]byte(route))
	return session.ForScheme(session.Suffixed("MEERKAT_UISIM_"+hex.EncodeToString(sum[:])[:12]), r)
}

// uiSimSign seals the claims with the simulation key. The label keeps a test
// cookie and a swagger test token apart: neither verifies as the other.
func (rt *Router) uiSimSign(c uiSimClaims) string {
	payload, _ := json.Marshal(c)
	mac := hmac.New(sha256.New, rt.simKey())
	mac.Write([]byte("ui-test\x00"))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// uiSimOpen verifies a cookie value; false for anything forged or expired.
func (rt *Router) uiSimOpen(v string) (uiSimClaims, bool) {
	p64, m64, ok := strings.Cut(v, ".")
	if !ok {
		return uiSimClaims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(p64)
	if err != nil {
		return uiSimClaims{}, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(m64)
	if err != nil {
		return uiSimClaims{}, false
	}
	mac := hmac.New(sha256.New, rt.simKey())
	mac.Write([]byte("ui-test\x00"))
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return uiSimClaims{}, false
	}
	var c uiSimClaims
	if err := json.Unmarshal(payload, &c); err != nil || time.Now().Unix() > c.Exp {
		return uiSimClaims{}, false
	}
	return c, true
}

func setUISimCookie(w http.ResponseWriter, r *http.Request, route, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     uiSimCookie(r, route),
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   filtering.Secure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// RegisterUISim mounts the developer-bar endpoints on the data-plane mux,
// next to the developer docs. Same gate as those: the dev capability, and
// nothing else.
func (rt *Router) RegisterUISim(mux *http.ServeMux) {
	mux.HandleFunc("GET /meerkat/dev-sim", rt.uiSimState)
	mux.HandleFunc("POST /meerkat/dev-sim", rt.uiSimSet)
	mux.HandleFunc("DELETE /meerkat/dev-sim", rt.uiSimClear)
}

// uiSimGate resolves the calling developer, or answers the refusal: a non-dev
// session gets an explicit 401.
func (rt *Router) uiSimGate(w http.ResponseWriter, r *http.Request) (sessionKey string, dev store.User, ok bool) {
	sess, err := rt.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		uiSimErr(w, http.StatusUnauthorized, "sign in with a developer account to use the UI test mode")
		return "", store.User{}, false
	}
	u, err := rt.st.GetUserByID(r.Context(), sess.UserID)
	if err != nil || !rt.st.DevAllowed(r.Context(), u) {
		uiSimErr(w, http.StatusUnauthorized, "the UI test mode requires the dev capability: ask an application administrator to grant it on Users")
		return "", store.User{}, false
	}
	return sess.TokenHash, u, true
}

func uiSimErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type uiSimView struct {
	Active bool     `json:"active"`
	User   string   `json:"user,omitempty"`
	Roles  []string `json:"roles,omitempty"`
}

func writeUISim(w http.ResponseWriter, v uiSimView) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// uiSimState answers the developer bar's load question: is THIS route
// simulated for MY session, and as whom?
func (rt *Router) uiSimState(w http.ResponseWriter, r *http.Request) {
	key, _, ok := rt.uiSimGate(w, r)
	if !ok {
		return
	}
	e, active := rt.uiSimFor(r, key, r.URL.Query().Get("route"))
	if !active {
		writeUISim(w, uiSimView{})
		return
	}
	writeUISim(w, uiSimView{Active: true, User: e.user, Roles: e.roles})
}

type uiSimRequest struct {
	Route string   `json:"route"`
	User  string   `json:"user"`
	Roles []string `json:"roles"`
}

// uiSimSet starts (or edits) the simulation for the caller's session on one
// route. The identity is validated the same way the swagger headers are.
func (rt *Router) uiSimSet(w http.ResponseWriter, r *http.Request) {
	key, dev, ok := rt.uiSimGate(w, r)
	if !ok {
		return
	}
	by := dev.Username
	var body uiSimRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		uiSimErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	body.Route = strings.TrimSpace(body.Route)
	body.User = strings.TrimSpace(body.User)
	if body.Route == "" || !rt.routeLive(body.Route) {
		uiSimErr(w, http.StatusUnprocessableEntity, "route: name a route of the live snapshot")
		return
	}
	if body.User == "" {
		uiSimErr(w, http.StatusUnprocessableEntity, "user: a simulated username is required")
		return
	}
	e := uiSimEntry{user: body.User, by: by, expires: time.Now().Add(uiSimTTL)}
	for _, role := range body.Roles {
		if role = strings.TrimSpace(role); role != "" && schemeTokenOK.MatchString(role) {
			e.roles = append(e.roles, role)
		}
	}
	// Expanded AT SET TIME so the developer bar shows the implication: check
	// a parent role, reload, and its descendants come back checked.
	e.roles = rt.expandSimRoles(r.Context(), e.roles)
	setUISimCookie(w, r, body.Route, rt.uiSimSign(uiSimClaims{
		Session: key, Route: body.Route, User: e.user, Roles: e.roles, By: by, Exp: e.expires.Unix(),
	}), int(uiSimTTL.Seconds()))
	slog.Info("ui test mode on", "by", by, "route", body.Route, "as", e.user, "roles", e.roles)
	detail := "as " + e.user
	if len(e.roles) > 0 {
		detail += ", roles " + strings.Join(e.roles, ", ")
	}
	rt.uiSimAudit(r, uiSimStart, dev, body.Route, detail)
	writeUISim(w, uiSimView{Active: true, User: e.user, Roles: e.roles})
}

// uiSimClear ends the simulation for the caller's session on one route.
func (rt *Router) uiSimClear(w http.ResponseWriter, r *http.Request) {
	key, dev, ok := rt.uiSimGate(w, r)
	if !ok {
		return
	}
	by := dev.Username
	route := r.URL.Query().Get("route")
	// Written only when a test WAS running: the bar's Cancel on an idle
	// route also lands here, and is no event.
	if e, running := rt.uiSimFor(r, key, route); running {
		rt.uiSimAudit(r, uiSimStop, dev, route, "as "+e.user)
	}
	setUISimCookie(w, r, route, "", -1)
	slog.Info("ui test mode off", "by", by, "route", route)
	writeUISim(w, uiSimView{})
}

// The trail's actions for the UI test mode: browsing under another identity
// is exactly what an audit is for, so starting and ending a test are written
// to the trail, beside the developer's other actions on their account
// (devkey.add). What the simulated identity then does reaches the upstream
// marked X-Meerkat-Test, and an audited operation names it.
const (
	uiSimStart = "uitest.start"
	uiSimStop  = "uitest.stop"
)

// uiSimAudit writes one UI test event about the developer's account, naming
// the route. Best-effort, like every security line: a test never fails
// because its line could not be written.
func (rt *Router) uiSimAudit(r *http.Request, action string, dev store.User, routeID, detail string) {
	name := routeID
	rt.mu.RLock()
	for i := range rt.routes {
		if rt.routes[i].id == routeID && rt.routes[i].name != "" {
			name = rt.routes[i].name
		}
	}
	rt.mu.RUnlock()
	ip, peer := filtering.ClientAndPeer(r)
	ev := store.AuditEvent{
		ActorID: dev.ID, ActorName: dev.Username, Action: action, Target: store.AuditTargetAccount,
		TargetID: dev.ID, TargetName: dev.Username, IP: ip, Peer: peer,
		Detail: "route " + name + ": " + detail,
	}
	if err := rt.st.AddAuditEvent(r.Context(), ev); err != nil {
		slog.Error("ui test trail write failed", "action", action, "by", dev.Username, "err", err)
	}
}

// routeLive reports whether the id is in the live snapshot (a disabled route
// is not - simulating it would silently do nothing).
func (rt *Router) routeLive(id string) bool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	for i := range rt.routes {
		if rt.routes[i].id == id {
			return true
		}
	}
	return false
}

// uiSimFor reads the test the request's cookie carries for this route: only
// when it verifies, has not expired, and names THIS session and THIS route -
// a cookie copied to another browser, or to another route, does nothing.
func (rt *Router) uiSimFor(r *http.Request, sessionKey, routeID string) (uiSimEntry, bool) {
	ck, err := r.Cookie(uiSimCookie(r, routeID))
	if err != nil {
		return uiSimEntry{}, false
	}
	c, ok := rt.uiSimOpen(ck.Value)
	if !ok || c.Session != sessionKey || c.Route != routeID {
		return uiSimEntry{}, false
	}
	return uiSimEntry{user: c.User, roles: c.Roles, by: c.By, expires: time.Unix(c.Exp, 0)}, true
}

// applyUISim stamps the simulated identity on a matched request when the
// caller's session runs a UI test on this route. A cookie lookup while no
// test runs; a header/token simulation (swagger) always wins over it.
func (rt *Router) applyUISim(req *http.Request, routeID string) *http.Request {
	if _, err := req.Cookie(uiSimCookie(req, routeID)); err != nil {
		return req
	}
	if _, ok := simulatedIdentity(req.Context()); ok {
		return req
	}
	sess, err := rt.sm.Resolve(req.Context(), req)
	if err != nil || sess.Pending != "" {
		return req
	}
	e, ok := rt.uiSimFor(req, sess.TokenHash, routeID)
	if !ok {
		return req
	}
	// The capability is read again on every request, not only at Apply: a
	// developer whose dev capability is taken (Remove developer mode) loses
	// a running test at once, not when its cookie expires. Read only while a
	// test cookie is presented, so nobody else pays for it.
	if u, err := rt.st.GetUserByID(req.Context(), sess.UserID); err != nil || !rt.st.DevAllowed(req.Context(), u) {
		return req
	}
	d := identityData{UserID: "simulated", Username: e.user, Fullname: "Simulated identity", Roles: e.roles}
	ctx := req.Context()
	ctx = withSimMeta(ctx, simMeta{By: e.by, Via: "ui-test", Route: routeID})
	return req.WithContext(withSimulatedIdentity(ctx, d))
}

// uiSimRefusalPage keeps the developer out of a lockout: when the SIMULATED
// identity is refused by an access gate, a bare 403 would also take the
// developer bar away - and with it the way out of the test. This small page
// says what happened and offers the exit.
func uiSimRefusalPage(w http.ResponseWriter, req *http.Request) {
	m, _ := simulationMeta(req.Context())
	d, _ := simulatedIdentity(req.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>UI test - refused</title>` +
		`<body style="font-family:system-ui;background:#1c1c22;color:#e8e8ee;display:grid;place-items:center;min-height:100vh;margin:0">` +
		`<div style="max-width:34em;padding:2em;border:3px solid #f59e0b;border-radius:12px">` +
		`<h1 style="font-size:1.2em;margin:0 0 .6em">Simulated identity refused</h1>` +
		`<p style="margin:0 0 1.2em;line-height:1.5">This route refuses "` + htmlEscape(d.Username) +
		`" (roles: ` + htmlEscape(strings.Join(d.Roles, ", ")) + `). That IS what such a user would ` +
		`experience here. Adjust the roles from the developer bar after exiting, or end the test.</p>` +
		`<button style="font:inherit;padding:.5em 1.2em;border-radius:8px;border:0;background:#f59e0b;color:#1c1c22;cursor:pointer" ` +
		`onclick="fetch('/meerkat/dev-sim?route=` + htmlEscape(m.Route) + `',{method:'DELETE'}).then(()=>location.reload())">` +
		`Exit UI test mode</button></div>`))
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
