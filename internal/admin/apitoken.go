package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// Control-plane API tokens (foundation for headless management - a CLI or an
// MCP server driving Meerkat, PLANNED). A control-plane token authenticates its
// owner on the ADMIN port only (the plane isolation lives in session.Resolve),
// so it carries no tenant/group context. Minting is root-only: the token acts
// with the owner's capabilities, and only root should hand out control-plane
// access. Only the token HASH is stored; the clear value is shown once.
func (a *API) registerAdminTokens(mux Mux) {
	mux.Handle("GET /api/admin-tokens", a.rootOnly(a.listAdminTokens))
	mux.Handle("POST /api/admin-tokens", a.rootOnly(a.createAdminToken))
	mux.Handle("DELETE /api/admin-tokens/{id}", a.rootOnly(a.revokeAdminToken))
	mux.Handle("PUT /api/admin-tokens/{id}", a.rootOnly(a.updateAdminToken))
	mux.Handle("POST /api/admin-tokens/{id}/renew", a.rootOnly(a.renewAdminToken))
	mux.Handle("POST /api/admin-tokens/{id}/toggle", a.rootOnly(a.toggleAdminToken))
}

func (a *API) listAdminTokens(w http.ResponseWriter, r *http.Request, actor store.User) {
	tokens, err := a.st.ListAPITokens(r.Context(), actor.ID, store.PlaneAdmin)
	if err != nil {
		a.internal(w, err)
		return
	}
	if tokens == nil {
		tokens = []store.APIToken{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (a *API) createAdminToken(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body struct {
		Name   string `json:"name"`
		Days   int    `json:"days"`   // 0 = never expires
		Scope  string `json:"scope"`  // full | readonly, empty = the safe one
		Domain string `json:"domain"` // gateway | app, empty = the whole plane
		From   string `json:"from"`   // CIDR ranges, empty = anywhere
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "token name is required")
		return
	}
	if len(name) > 60 {
		name = name[:60]
	}
	scope, err := store.SanitizeTokenScope(body.Scope)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	domain, err := store.SanitizeTokenDomain(body.Domain)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	from, err := store.SanitizeTokenCIDRs(body.From)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	var expiresAt int64
	if body.Days > 0 {
		expiresAt = time.Now().Add(time.Duration(body.Days) * 24 * time.Hour).Unix()
	}
	secret, hash, prefix, err := mintToken()
	if err != nil {
		a.internal(w, err)
		return
	}
	id := newID()
	if err := a.st.AddAPIToken(r.Context(), store.NewToken{
		ID: id, UserID: actor.ID, Name: name, TokenHash: hash, Prefix: prefix,
		Plane: store.PlaneAdmin, Scope: scope, Domain: domain, FromCIDRs: from,
		ExpiresAt: expiresAt,
	}); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "token.create", "token", id, name, "",
		"control-plane token, "+perimeterWords(scope, domain, from))
	// The clear value travels exactly once, here.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": id, "name": name, "prefix": prefix, "token": secret,
		"expiresAt": expiresAt, "scope": scope, "domain": domain, "fromCidrs": from,
	})
}

// renewAdminToken gives a token a new secret and keeps everything else.
//
// A ROTATION, and what it keeps is why it is not simply minting a second one:
// the name, the perimeter, the domain, the addresses and the expiry stay, so
// the audit keeps one history for one credential and the only thing to change
// anywhere is the value in one configuration file.
//
// The old secret stops working the moment this answers - that is what a
// rotation is, and it is also why the console asks first. The new one travels
// exactly once, here, like a freshly minted one.
func (a *API) renewAdminToken(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	before, ok := a.adminToken(r, actor.ID, id)
	if !ok {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	secret, hash, prefix, err := mintToken()
	if err != nil {
		a.internal(w, err)
		return
	}
	// The same call the OAuth refresh uses (MCP-07): replace the secret, keep
	// the identity. It is not scoped by user, and does not need to be - the
	// lookup above already refused a token that is not this account's, and a
	// second function differing only by that would be a second function to
	// keep in step. The expiry is passed back unchanged: a rotation is a new
	// key, not a new deadline.
	if err := a.st.RenewAPIToken(r.Context(), id, hash, prefix, before.ExpiresAt); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "token.renew", "token", id, before.Name, "",
		"a new secret; the previous one stops working")
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "name": before.Name, "prefix": prefix, "token": secret,
		"expiresAt": before.ExpiresAt, "scope": before.Scope, "domain": before.Domain,
		"fromCidrs": before.FromCIDRs,
	})
}

func (a *API) revokeAdminToken(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	name := adminTokenName(a, r, actor.ID, id) // capture before deletion
	existed, err := a.st.RevokeAPIToken(r.Context(), actor.ID, id)
	if err != nil {
		a.internal(w, err)
		return
	}
	if !existed {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	a.auditEvent(r.Context(), actor, "token.revoke", "token", id, name, "", "")
	w.WriteHeader(http.StatusNoContent)
}

// updateAdminToken changes what a token may do, without touching the token.
//
// EVERYTHING EXCEPT THE SECRET. The secret is a hash in a column and encodes
// none of this - not the perimeter, not the domain, not the addresses, not the
// expiry, not the name - so none of them has to be reissued to be changed, and
// whoever holds the key keeps holding the same key.
//
// Which is what makes NARROWING cheap, and that is the reason this endpoint
// exists. A read-only token sitting in a monitoring stack's scrape config
// should become a metrics one without minting a second and editing another
// team's repository: that friction is precisely why the safer move does not
// get made. Widening is possible too, and the audit records the before and
// after of every field, so it is a change somebody can see rather than one
// that happened.
func (a *API) updateAdminToken(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	var body struct {
		Name   string `json:"name"`
		Days   int    `json:"days"`   // 0 = never expires
		Scope  string `json:"scope"`  // metrics | readonly | full
		Domain string `json:"domain"` // gateway | app, empty = the whole plane
		From   string `json:"from"`   // CIDR ranges, empty = anywhere
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "token name is required")
		return
	}
	if len(name) > 60 {
		name = name[:60]
	}
	// Read BEFORE, so the audit says what changed rather than what it became.
	before, ok := a.adminToken(r, actor.ID, id)
	if !ok {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	var expiresAt int64
	if body.Days > 0 {
		expiresAt = time.Now().Add(time.Duration(body.Days) * 24 * time.Hour).Unix()
	}
	edit := store.TokenEdit{Name: name, Scope: body.Scope, Domain: body.Domain,
		FromCIDRs: body.From, ExpiresAt: expiresAt}
	existed, err := a.st.UpdateAPIToken(r.Context(), actor.ID, id, edit)
	if err != nil {
		// Sanitize refuses here, naming what is allowed: a perimeter nobody
		// can spell is a caller's mistake, not this gateway's.
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if !existed {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	after, _ := a.adminToken(r, actor.ID, id)
	a.auditUpdate(r.Context(), actor, "token.update", "token", id, before.Name, "", before, after)
	writeJSON(w, http.StatusOK, after)
}

// adminToken is one of the actor's control-plane tokens, as the list shows it.
func (a *API) adminToken(r *http.Request, userID, id string) (store.APIToken, bool) {
	tokens, err := a.st.ListAPITokens(r.Context(), userID, store.PlaneAdmin)
	if err != nil {
		return store.APIToken{}, false
	}
	for _, t := range tokens {
		if t.ID == id {
			return t, true
		}
	}
	return store.APIToken{}, false
}

func (a *API) toggleAdminToken(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	existed, err := a.st.SetAPITokenEnabled(r.Context(), actor.ID, id, body.Enabled)
	if err != nil {
		a.internal(w, err)
		return
	}
	if !existed {
		writeErr(w, http.StatusNotFound, "token not found")
		return
	}
	verb := "token.disable"
	if body.Enabled {
		verb = "token.enable"
	}
	a.auditEvent(r.Context(), actor, verb, "token", id, adminTokenName(a, r, actor.ID, id), "", "")
	w.WriteHeader(http.StatusNoContent)
}

// adminTokenName resolves a token's display name (best-effort) for the audit.
func adminTokenName(a *API, r *http.Request, userID, id string) string {
	tokens, err := a.st.ListAPITokens(r.Context(), userID, store.PlaneAdmin)
	if err != nil {
		return ""
	}
	for _, t := range tokens {
		if t.ID == id {
			return t.Name
		}
	}
	return ""
}

// mintToken generates a fresh "mk_..." secret and returns its clear value, its
// hash (sha256 hex, matching session.hashToken), and a display prefix.
func mintToken() (secret, hash, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	secret = "mk_" + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(secret))
	hash = hex.EncodeToString(sum[:])
	prefix = secret[:12]
	return secret, hash, prefix, nil
}

// perimeterWords is the perimeter in the audit's own words. A trail saying
// "control-plane token" for a read-only gateway token and for a root one alike
// would be recording the least interesting half of the event.
func perimeterWords(scope, domain, from string) string {
	words := scope
	switch domain {
	case store.DomainGateway:
		words += ", routing plane only"
	case store.DomainApp:
		words += ", application identity only"
	}
	if from != "" {
		words += ", from " + from
	}
	return words
}
