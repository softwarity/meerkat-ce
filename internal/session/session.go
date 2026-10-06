// Package session implements Meerkat's web sessions, as decided in the
// requirements (Q6): an opaque httpOnly cookie whose state lives in the
// store - revocation is immediate - fronted by a small in-memory cache so
// the hot path does not pay a database read on every request. JWTs are for
// the API path and upstream propagation, never for the browser.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/store"
)

// Cookie names, one per plane: cookies are NOT port-scoped, so on a same-host
// deployment the two ports would otherwise share the browser's session.
//
// And one set per INSTALLATION, for the same reason one level up: two
// gateways a browser reaches under one host name overwrote each other's
// sessions. SetCookieSuffix appends the installation's identifier at startup,
// before any request; until then (and in tests) the names are the bare ones.
var (
	CookieName      = "MEERKAT_SESSION"       // data plane
	AdminCookieName = "MEERKAT_ADMIN_SESSION" // control plane
)

// SetCookieSuffix makes this installation's cookie names its own. Called once
// by main, before the handlers serve anything.
func SetCookieSuffix(suffix string) {
	cookieSuffix = suffix
	CookieName = Suffixed("MEERKAT_SESSION")
	AdminCookieName = Suffixed("MEERKAT_ADMIN_SESSION")
	UntilCookieName = Suffixed("MEERKAT_UNTIL")
	AdminUntilCookieName = Suffixed("MEERKAT_ADMIN_UNTIL")
}

// SecurePrefix names a cookie set over HTTPS. Browsers forbid a page in the
// clear to overwrite a Secure cookie of the same name ("leave secure cookies
// alone"), so a session opened over HTTPS made signing in over plain HTTP
// silently impossible: the password was right, the cookie was thrown away,
// and the login page came back. The plain door of the console exists for the
// day a certificate is broken - exactly when somebody who used HTTPS before
// needs it. Two names, one per scheme, never meet; and __Host- makes the
// browser guarantee the HTTPS one was set over HTTPS, by this host, for the
// whole site.
const SecurePrefix = "__Host-"

// ForScheme is the name a cookie carries on the scheme r arrived over.
func ForScheme(name string, r *http.Request) string {
	if filters.Secure(r) {
		return SecurePrefix + name
	}
	return name
}

var cookieSuffix string

// Suffixed is base with this installation's suffix, for a cookie that holds
// state of THIS installation (a session, a trusted browser). A preference -
// the language, the colour scheme - is the person's and stays shared.
func Suffixed(base string) string {
	if cookieSuffix == "" {
		return base
	}
	return base + "_" + cookieSuffix
}

// Planes stamped on every stored session - Resolve refuses a token from the
// other plane even if someone copies the cookie across.
const (
	DataPlane  = "data"
	AdminPlane = "admin"
)

// ErrNoSession is returned when the request carries no valid session.
var ErrNoSession = errors.New("session: none")

// Manager issues, resolves and revokes sessions for ONE plane.
type Manager struct {
	st         *store.Store
	ttl        time.Duration // session lifetime
	cacheTTL   time.Duration // how long a store read may be served from memory
	now        func() time.Time
	cookieName string
	plane      string

	mu    sync.Mutex
	cache map[string]cacheEntry
	// tokens is the same memory for API tokens, by token hash (resolveToken).
	tokens map[string]tokenEntry
	// forgotten counts the drops from tokens. A read notes it before asking
	// the database and stores its answer only if it has not moved: a revoke
	// landing while the read was in flight must not see the old answer put
	// back after it.
	forgotten uint64
	// notify tells the other gateways to drop what this node just changed
	// (invalidate.go). Nil on a single one.
	notify func(topic, arg string)
}

type cacheEntry struct {
	sess    store.Session
	readAt  time.Time
	invalid bool // negative cache: known-absent token
}

// tokenEntry is what the database said about one API token, and about its
// owner, at readAt. Only a token that EXISTS on this plane is remembered: a
// refused one is read again next time, so re-enabling a token never waits for
// an entry nobody could have named by its id.
type tokenEntry struct {
	tok store.ResolvedToken
	// allowed is the gateway-wide personal-token policy (AUTH-16) as it read.
	allowed bool
	// owner is the account reduced to what a request is still judged against
	// (enabled, validity window): the whole row carries an avatar, and there is
	// one entry per token.
	owner  store.User
	readAt time.Time
}

// maxCachedTokens bounds the token memory: past it, entries older than the
// cache window are swept before a new one is added.
const maxCachedTokens = 10000

// Option tweaks a Manager (tests mostly).
type Option func(*Manager)

// WithTTL sets the session lifetime (default 30m - the V1 default).
func WithTTL(d time.Duration) Option { return func(m *Manager) { m.ttl = d } }

// WithCacheTTL sets the memory-cache window (default 5s).
func WithCacheTTL(d time.Duration) Option { return func(m *Manager) { m.cacheTTL = d } }

// WithClock overrides time.Now (tests).
func WithClock(now func() time.Time) Option { return func(m *Manager) { m.now = now } }

// ForAdminPlane scopes the manager to the control plane: its own cookie name
// and its own session plane - the two ports never share a browser session.
func ForAdminPlane() Option {
	return func(m *Manager) { m.cookieName, m.plane = AdminCookieName, AdminPlane }
}

// NewManager builds a Manager over the store.
func NewManager(st *store.Store, opts ...Option) *Manager {
	m := &Manager{
		st:         st,
		ttl:        30 * time.Minute,
		cacheTTL:   5 * time.Second,
		now:        time.Now,
		cookieName: CookieName,
		plane:      DataPlane,
		cache:      map[string]cacheEntry{},
		tokens:     map[string]tokenEntry{},
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Issue creates a session for userID with the manager's default TTL and no
// active tenant, and sets the cookie on w.
func (m *Manager) Issue(ctx context.Context, w http.ResponseWriter, r *http.Request, userID string) (string, error) {
	return m.IssueWith(ctx, w, r, userID, "", "", m.ttl, "", "", "")
}

// IssueWith creates a session with an explicit active tenant and lifetime -
// the login flow passes the RESOLVED TTL (membership -> tenant -> global,
// TENANT-05). The returned token is the raw cookie value (only its hash is
// persisted).
func (m *Manager) IssueWith(ctx context.Context, w http.ResponseWriter, r *http.Request, userID, tenantID, groupID string, ttl time.Duration, pending, next, method string) (string, error) {
	if ttl <= 0 {
		ttl = m.ttl
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sess := store.Session{
		TokenHash: hashToken(token),
		UserID:    userID,
		TenantID:  tenantID,
		GroupID:   groupID,
		Pending:   pending,
		Next:      next,
		Method:    method,
		ExpiresAt: m.now().Add(ttl).Unix(),
		TTL:       int64(ttl.Seconds()),
		Plane:     m.plane,
		// What a list of sessions shows (AUTH-14): when, from where, with what.
		CreatedAt: m.now().Unix(),
		IP:        filters.ClientIP(r),
		Agent:     clipAgent(r.UserAgent()),
	}
	if err := m.st.CreateSession(ctx, sess); err != nil {
		return "", err
	}
	m.setCookies(w, r, token, ttl, sess.ExpiresAt)
	return token, nil
}

// UntilCookie carries the session's deadline where a PAGE can read it: the
// session cookie is httpOnly (it must be), so nothing in the browser could
// otherwise tell whether a signed-in tab left open for an hour is still
// signed in. A unix timestamp discloses nothing; the page agent reads it and
// goes to the login page on its own instead of waiting for someone to click
// and discover it is over. One cookie per plane, same reason as the session's.
var (
	UntilCookieName      = "MEERKAT_UNTIL"
	AdminUntilCookieName = "MEERKAT_ADMIN_UNTIL"
)

// untilCookieName is the readable deadline's name for this manager's plane.
func (m *Manager) untilCookieName() string {
	if m.plane == AdminPlane {
		return AdminUntilCookieName
	}
	return UntilCookieName
}

// setCookies writes the session cookie and its readable deadline together -
// they must never disagree: a browser that drops the session cookie while the
// row still lives signs someone out mid-work, and a deadline that outlives the
// cookie makes a page believe in a session it no longer carries.
func (m *Manager) setCookies(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration, expiresAt int64) {
	secure := filters.Secure(r)
	age := int(ttl.Seconds())
	http.SetCookie(w, &http.Cookie{
		Name:     ForScheme(m.cookieName, r),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   age,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     ForScheme(m.untilCookieName(), r),
		Value:    strconv.FormatInt(expiresAt, 10),
		Path:     "/",
		HttpOnly: false, // the whole point: a page reads it
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   age,
	})
}

// Sliding is the idle timeout, and it wraps a whole plane: any request
// carrying a live session pushes its deadline to now + the lifetime the login
// resolved. Someone who works is never signed out; someone who walks away is,
// which is what the TTL was always meant to say.
//
// Written to the store only in the SECOND half of the lifetime. A write per
// request would take SQLite's write lock on the hot path for nothing - the
// deadline gained would be seconds - and this way a 30-minute session writes
// at most every 15 minutes.
//
// The cookies follow the row, always: the session cookie's MaxAge was the
// lifetime, so without this the browser would drop it while the row still
// lived. That is a sign-out with no server-side cause, and it would be blamed
// on everything but the cookie.
func (m *Manager) Sliding(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.slide(w, r)
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) slide(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(ForScheme(m.cookieName, r))
	if err != nil || c.Value == "" {
		return // no browser session: an API token has no deadline to push
	}
	sess, err := m.Resolve(r.Context(), r)
	if err != nil {
		return
	}
	ttl := time.Duration(sess.TTL) * time.Second
	if ttl <= 0 {
		ttl = m.ttl
	}
	now := m.now()
	if now.Unix() < sess.ExpiresAt-int64(ttl.Seconds())/2 {
		return // still in the first half: nothing to write, nothing to say
	}
	until := now.Add(ttl).Unix()
	if err := m.st.ExtendSession(r.Context(), sess.TokenHash, sess.ExpiresAt, until); err != nil {
		// Paused (store/pause.go): the session keeps its deadline, which is
		// still hours away, and says nothing about it.
		if !errors.Is(err, store.ErrPaused) {
			slog.Error("session extension failed", "err", err)
		}
		return
	}
	sess.ExpiresAt = until
	m.remember(sess.TokenHash, cacheEntry{sess: sess, readAt: now})
	m.setCookies(w, r, c.Value, ttl, until)
}

// ClearPending marks the request's session as done with its current
// login-flow step (AUTH-05) and refreshes the cache.
func (m *Manager) ClearPending(ctx context.Context, r *http.Request) error {
	return m.SetPending(ctx, r, "")
}

// SetPending advances the request's session to the next login-flow step
// (AUTH-05) - e.g. from the password step to the MFA step - without issuing a
// new session. "" clears the step (flow complete). Refreshes the cache.
func (m *Manager) SetPending(ctx context.Context, r *http.Request, step string) error {
	c, err := r.Cookie(ForScheme(m.cookieName, r))
	if err != nil || c.Value == "" {
		return ErrNoSession
	}
	th := hashToken(c.Value)
	if err := m.st.SetSessionPending(ctx, th, step); err != nil {
		return err
	}
	m.dropped(th)
	return nil
}

// SetTenant records the active tenant on the request's session (the
// select-tenant step - TENANT-03) and refreshes the cache.
func (m *Manager) SetTenant(ctx context.Context, r *http.Request, tenantID string) error {
	c, err := r.Cookie(ForScheme(m.cookieName, r))
	if err != nil || c.Value == "" {
		return ErrNoSession
	}
	th := hashToken(c.Value)
	if err := m.st.SetSessionTenant(ctx, th, tenantID); err != nil {
		return err
	}
	m.dropped(th)
	return nil
}

// SetGroup records the ACTIVE group on the request's session (the
// select-group step, exclusive mode - RBAC-03) and refreshes the cache.
func (m *Manager) SetGroup(ctx context.Context, r *http.Request, groupID string) error {
	c, err := r.Cookie(ForScheme(m.cookieName, r))
	if err != nil || c.Value == "" {
		return ErrNoSession
	}
	th := hashToken(c.Value)
	if err := m.st.SetSessionGroup(ctx, th, groupID); err != nil {
		return err
	}
	m.dropped(th)
	return nil
}

// Resolve returns the session carried by the request, or ErrNoSession. Reads
// are served from the memory cache within cacheTTL; expiry is always checked
// against the wall clock, so a cached session never outlives its TTL.
func (m *Manager) Resolve(ctx context.Context, r *http.Request) (store.Session, error) {
	c, err := r.Cookie(ForScheme(m.cookieName, r))
	if err != nil || c.Value == "" {
		// No browser session: an API token may authenticate (AUTH-16), but only
		// one scoped to THIS plane - a data token never opens the admin port and
		// an admin (control-plane) token never opens the data port.
		if sess, ok := m.resolveToken(ctx, r); ok {
			return sess, nil
		}
		return store.Session{}, ErrNoSession
	}
	th := hashToken(c.Value)
	now := m.now()

	m.mu.Lock()
	entry, hit := m.cache[th]
	m.mu.Unlock()
	if hit && now.Sub(entry.readAt) < m.cacheTTL {
		if entry.invalid || now.Unix() >= entry.sess.ExpiresAt {
			return store.Session{}, ErrNoSession
		}
		return entry.sess, nil
	}

	sess, err := m.st.GetSession(ctx, th)
	if err != nil || sess.Plane != m.plane {
		// Unknown token OR a token from the other plane (a copied cookie):
		// both answer exactly "no session".
		m.remember(th, cacheEntry{invalid: true, readAt: now})
		return store.Session{}, ErrNoSession
	}
	m.remember(th, cacheEntry{sess: sess, readAt: now})
	if now.Unix() >= sess.ExpiresAt {
		return store.Session{}, ErrNoSession
	}
	return sess, nil
}

// apiTokenPrefix marks Meerkat personal access tokens - greppable in logs,
// detectable by secret scanners, distinct from a session cookie value.
const apiTokenPrefix = "mk_"

// resolveToken authenticates an "Authorization: Bearer mk_..." request against
// a live API token, synthesizing the session context the token captured
// (tenant + group).
//
// What the database says about a token - that it exists and is enabled, on
// which plane, for whom, under which policy, and whether its owner is enabled -
// is read at most once per cache window, like a cookie session. It used to be
// read on EVERY request: three queries (the token, the policy, the account)
// that held an API route to 7,500 req/s on one core where Kong's key-auth reads
// 36,000 (tools/bench). Revocation stays immediate the way it is for sessions:
// every write that changes a token or its owner drops the entry here and tells
// the other gateways (invalidate.go), so the window only ever covers a lost
// message.
//
// What depends on the REQUEST or on the CLOCK is still judged every time: the
// caller's address against the token's ranges, the token's expiry, the
// owner's validity window.
func (m *Manager) resolveToken(ctx context.Context, r *http.Request) (store.Session, bool) {
	auth := r.Header.Get("Authorization")
	const bearer = "Bearer "
	if !strings.HasPrefix(auth, bearer) {
		return store.Session{}, false
	}
	raw := strings.TrimSpace(auth[len(bearer):])
	if !strings.HasPrefix(raw, apiTokenPrefix) {
		return store.Session{}, false
	}
	th := hashToken(raw)
	now := m.now()
	e, ok := m.cachedToken(th, now)
	if !ok {
		if e, ok = m.readToken(ctx, th, now); !ok {
			return store.Session{}, false
		}
	}
	tok := e.tok
	if !e.allowed {
		return store.Session{}, false
	}
	if tok.ExpiresAt != 0 && now.Unix() >= tok.ExpiresAt {
		return store.Session{}, false
	}
	// Where the token may be used from (MCP-02), judged on the TCP PEER: a
	// forwarding header is written by whoever sends it. A refusal here is a
	// plain "no session" - the caller learns nothing about which token exists
	// - so it is logged, or an administrator would have nothing to go on.
	if !store.AllowsAddress(tok.FromCIDRs, r.RemoteAddr) {
		slog.Warn("api token refused: address outside its allowed range",
			"token", tok.Name, "from", r.RemoteAddr, "allowed", tok.FromCIDRs)
		return store.Session{}, false
	}
	// A token outlives nothing its owner does not: an account past its window
	// stops answering, machine-to-machine included.
	if !e.owner.Enabled || !e.owner.ValidAt(now) {
		return store.Session{}, false
	}
	m.touchToken(ctx, th, tok.ID, now)
	return store.Session{
		UserID: tok.UserID, TenantID: tok.TenantID, GroupID: tok.GroupID,
		Plane: m.plane, ExpiresAt: now.Add(m.ttl).Unix(),
		// What a cookie session can never carry: this caller is a token. The
		// guard reads the perimeter (MCP-02) and the audit reads the name
		// (MCP-03) - both would be unanswerable from the account alone.
		TokenID: tok.ID, TokenName: tok.Name, TokenScope: tok.Scope,
	}, true
}

// cachedToken returns what was read about a token within the cache window.
func (m *Manager) cachedToken(tokenHash string, now time.Time) (tokenEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, hit := m.tokens[tokenHash]
	if !hit || now.Sub(e.readAt) >= m.cacheTTL {
		return tokenEntry{}, false
	}
	return e, true
}

// readToken asks the database, and remembers the answer only for a token that
// exists on this plane. Anything else - absent, disabled, expired, the other
// plane's - drops whatever was remembered and is asked again next time.
func (m *Manager) readToken(ctx context.Context, tokenHash string, now time.Time) (tokenEntry, bool) {
	m.mu.Lock()
	before := m.forgotten
	m.mu.Unlock()
	tok, err := m.st.ResolveAPIToken(ctx, tokenHash, now.Unix())
	if err != nil {
		m.forgetTokenHash(tokenHash)
		return tokenEntry{}, false
	}
	// A token authenticates ONLY on its own plane - this is the isolation
	// between the data port and the admin port.
	if tok.Plane != m.plane {
		return tokenEntry{}, false
	}
	u, err := m.st.GetUserByID(ctx, tok.UserID)
	if err != nil {
		m.forgetTokenHash(tokenHash)
		return tokenEntry{}, false
	}
	e := tokenEntry{
		tok: tok,
		// The gateway-wide personal-token policy (AUTH-16) gates DATA tokens
		// only; admin (control-plane) tokens are a root capability, not that
		// policy's. A scheduled run's own credential is not a personal token
		// either - it is the gateway calling a service on its owner's behalf,
		// and turning personal tokens off must not stop the night's jobs.
		allowed: m.plane != DataPlane || store.IsRunCredential(tok.ID) || m.st.APITokensAllowed(ctx),
		owner:   store.User{ID: u.ID, Enabled: u.Enabled, ValidFrom: u.ValidFrom, ValidUntil: u.ValidUntil},
		readAt:  now,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forgotten != before {
		// Something was dropped meanwhile: answer this request, remember nothing.
		return e, true
	}
	if len(m.tokens) >= maxCachedTokens {
		for th, old := range m.tokens {
			if now.Sub(old.readAt) >= m.cacheTTL {
				delete(m.tokens, th)
			}
		}
	}
	m.tokens[tokenHash] = e
	return e, true
}

func (m *Manager) forgetTokenHash(tokenHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, tokenHash)
	m.forgotten++
}

// touchToken stamps a token's last use, at most once a minute: a write per
// request would put back exactly the database round-trip the cache removed.
// Best-effort - a failure never blocks the call. The stamp is recorded in the
// cached entry BEFORE the write, so concurrent requests do not all decide the
// minute is up.
func (m *Manager) touchToken(ctx context.Context, tokenHash, id string, now time.Time) {
	m.mu.Lock()
	e, hit := m.tokens[tokenHash]
	due := hit && now.Unix()-e.tok.LastUsedAt >= 60
	if due {
		e.tok.LastUsedAt = now.Unix()
		m.tokens[tokenHash] = e
	}
	m.mu.Unlock()
	if due {
		_ = m.st.TouchAPIToken(ctx, id, now.Unix())
	}
}

// Destroy revokes the request's session (if any), evicts it from the cache
// and clears the cookie. Revocation is immediate on this node; other nodes
// converge within cacheTTL (LISTEN/NOTIFY-style invalidation comes with the
// cluster backend).
func (m *Manager) Destroy(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie(ForScheme(m.cookieName, r))
	if err == nil && c.Value != "" {
		th := hashToken(c.Value)
		if err := m.st.DeleteSession(ctx, th); err != nil {
			return err
		}
		m.dropped(th)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     ForScheme(m.cookieName, r),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   filters.Secure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	// The deadline goes with it: left behind, it would tell every open page
	// that a session it no longer has is good for another half hour.
	http.SetCookie(w, &http.Cookie{
		Name:     ForScheme(m.untilCookieName(), r),
		Value:    "",
		Path:     "/",
		Secure:   filters.Secure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	return nil
}

// PurgeExpired removes expired sessions from the store (periodic upkeep).
func (m *Manager) PurgeExpired(ctx context.Context) (int64, error) {
	return m.st.PurgeExpiredSessions(ctx, m.now().Unix())
}

func (m *Manager) remember(th string, e cacheEntry) {
	m.mu.Lock()
	m.cache[th] = e
	m.mu.Unlock()
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// clipAgent bounds a User-Agent before it is stored: the header is the
// caller's to write, and a row is not the place for whatever they pasted.
func clipAgent(ua string) string {
	if len(ua) > 300 {
		return ua[:300]
	}
	return ua
}

// CurrentID is the public id of the session the request carries, "" without
// one - what a list of sessions marks as "this browser".
func (m *Manager) CurrentID(r *http.Request) string {
	c, err := r.Cookie(ForScheme(m.cookieName, r))
	if err != nil || c.Value == "" {
		return ""
	}
	return store.SessionID(hashToken(c.Value))
}

// RevokeByIDOf is RevokeByID for a session that must belong to owner: a
// person closing their own sessions cannot close anyone else's by sending
// another id. "" when it is not theirs, and nothing is closed.
func (m *Manager) RevokeByIDOf(ctx context.Context, id, owner string) (string, string, error) {
	_, userID, _, err := m.st.SessionHashByID(ctx, id)
	if err != nil || userID != owner {
		return "", "", err
	}
	return m.RevokeByID(ctx, id)
}

// RevokeByID ends one session by its public id (AUTH-14, SEC-07), on every
// node: the row goes, and so do the caches that would have answered for it
// for a few more seconds. It says whose session it was, for the trail.
func (m *Manager) RevokeByID(ctx context.Context, id string) (userID, plane string, err error) {
	hash, userID, plane, err := m.st.SessionHashByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	if err := m.st.DeleteSession(ctx, hash); err != nil {
		return "", "", err
	}
	m.dropped(hash)
	return userID, plane, nil
}
