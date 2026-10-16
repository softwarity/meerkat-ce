package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// The OIDC provider's half of the store (SAUTH-05): the applications behind
// the data plane that sign their people in against this gateway, the codes in
// flight and the refresh tokens they hold.
//
// Separate tables from the agents' OAuth (oauth.go) on purpose, and that is
// the whole security argument: an agent client reaches the CONTROL plane, an
// application client the DATA plane, and a row shared between them would be a
// key that opens both. What is shared is code - the PKCE check, the
// single-use take - never rows.
//
// The clients are DECLARED by an administrator rather than registered by
// themselves: there is no consent screen, because the person who declared the
// application already decided it may know who signs in to it. Who may sign in
// is the route's business - its access rule - not the client's.

// OIDCClient is one application this gateway is the OpenID provider of.
type OIDCClient struct {
	// ID is the client_id, chosen by the administrator and immutable: it is
	// written in the application's own configuration.
	ID string `json:"id"`
	// Confidential says the application keeps a secret (a server). A public
	// one (code running in a browser) cannot, and proves itself by PKCE alone.
	Confidential bool `json:"confidential"`
	// SecretHash is the sha256 of the secret, hex. Never serialised: the
	// secret is shown once, when it is drawn, and never again.
	SecretHash string `json:"-"`
	// SecretPrefix is the start of the secret, to recognise which one an
	// application holds without being able to use it.
	SecretPrefix string `json:"secretPrefix"`
	// No redirect uri is declared: an application is served behind its
	// routes, and a code goes back to an address under the routes bound to
	// the client (gateway.OIDCReturns), nowhere else. The redirect_uris
	// column of oidc_clients is left in place, written and read by nothing.
	// Audience is the "aud" of the access tokens. Empty means the client id.
	Audience string `json:"audience"`
	// TokenTTL is how long the id and access tokens live, an ISO-8601
	// duration. Empty means OIDCDefaultTokenTTL.
	TokenTTL  string `json:"tokenTtl"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// The bounds of a token's life. Short by default: an access token is a bearer
// credential, and a sign-out at the gateway reaches it only when it expires.
// A day at most, because past that the refresh token is what should be doing
// the work.
const (
	OIDCDefaultTokenTTL = 5 * time.Minute
	OIDCMinTokenTTL     = time.Minute
	OIDCMaxTokenTTL     = 24 * time.Hour
)

// OIDCCodeLifetime is how long an authorisation code may wait to be spent: the
// application is already waiting for it, so a minute is generous.
const OIDCCodeLifetime = 60 * time.Second

// OIDCSecretPrefix starts every client secret, so one found in a log or a
// repository says what it is.
const OIDCSecretPrefix = "mko_"

// oidcClientIDOK is what a client id may be: it travels in URLs and in the
// application's configuration, so nothing that needs escaping.
var oidcClientIDOK = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// ErrNoOIDCClient is what an unknown client id answers.
var ErrNoOIDCClient = errors.New("store: unknown oidc client")

// ErrOIDCGrant marks a code or refresh token that cannot be spent: unknown,
// already used, or expired. The provider answers invalid_grant for all three,
// and the wrapped sentence says which one it was.
var ErrOIDCGrant = errors.New("invalid grant")

// TTL is the lifetime of the client's tokens, its default when unset.
func (c OIDCClient) TTL() time.Duration {
	if c.TokenTTL == "" {
		return OIDCDefaultTokenTTL
	}
	d, err := ParseISODuration(c.TokenTTL)
	if err != nil {
		return OIDCDefaultTokenTTL
	}
	return d
}

// AudienceOrID is the "aud" its access tokens carry.
func (c OIDCClient) AudienceOrID() string {
	if c.Audience != "" {
		return c.Audience
	}
	return c.ID
}

// SecretMatches says whether secret is this client's, in constant time. A
// public client has none, and nothing matches it.
func (c OIDCClient) SecretMatches(secret string) bool {
	if !c.Confidential || c.SecretHash == "" || secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(HashOIDCSecret(secret)), []byte(c.SecretHash)) == 1
}

// HashOIDCSecret is how a client secret is kept: sha256, hex, like the API
// tokens. The secret is 32 random bytes, so a slow hash would buy nothing.
func HashOIDCSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// NewOIDCSecret draws a client secret: its clear value (shown once), its hash
// (kept) and its prefix (shown always).
func NewOIDCSecret() (secret, hash, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("store: draw an oidc client secret: %w", err)
	}
	secret = OIDCSecretPrefix + base64.RawURLEncoding.EncodeToString(b)
	return secret, HashOIDCSecret(secret), secret[:12], nil
}

// SanitizeOIDCClient checks a client and normalises what it can. Refusals
// name what is allowed: they are read by the person typing the form.
func SanitizeOIDCClient(c *OIDCClient) error {
	c.ID = strings.TrimSpace(c.ID)
	if !oidcClientIDOK.MatchString(c.ID) {
		return invalidf(fmt.Errorf("client id %q is not allowed: 1 to 63 lower-case letters, digits, "+
			"and . _ - after the first character, which is a letter or a digit", c.ID))
	}
	c.Audience = strings.TrimSpace(c.Audience)
	if len(c.Audience) > 255 || strings.ContainsFunc(c.Audience, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) {
		return invalidf(fmt.Errorf("audience %q is not allowed: up to 255 characters, no spaces", c.Audience))
	}
	c.TokenTTL = strings.TrimSpace(c.TokenTTL)
	if c.TokenTTL != "" {
		d, err := ParseISODuration(c.TokenTTL)
		if err != nil {
			return invalidf(fmt.Errorf("token lifetime %q is not an ISO-8601 duration: %w", c.TokenTTL, err))
		}
		if d < OIDCMinTokenTTL || d > OIDCMaxTokenTTL {
			return invalidf(fmt.Errorf("token lifetime %q is not allowed: from PT1M to PT24H", c.TokenTTL))
		}
	}
	if !c.Confidential {
		c.SecretHash, c.SecretPrefix = "", ""
	}
	return nil
}

const oidcClientCols = `id, confidential, secret_hash, secret_prefix, audience, token_ttl, created_at, updated_at`

func scanOIDCClient(row rowScanner) (OIDCClient, error) {
	var c OIDCClient
	err := row.Scan(&c.ID, &c.Confidential, &c.SecretHash, &c.SecretPrefix,
		&c.Audience, &c.TokenTTL, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// CreateOIDCClient records a new client, refusing an id already taken. The
// secret, if any, is set apart (SetOIDCClientSecret): drawing one is an act
// of its own, and the hash passed here is kept as it is.
func (s *Store) CreateOIDCClient(ctx context.Context, c OIDCClient) (OIDCClient, error) {
	if err := SanitizeOIDCClient(&c); err != nil {
		return c, err
	}
	now := time.Now().Unix()
	c.CreatedAt, c.UpdatedAt = now, now
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO oidc_clients (`+oidcClientCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO NOTHING`,
		c.ID, c.Confidential, c.SecretHash, c.SecretPrefix,
		c.Audience, c.TokenTTL, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return c, fmt.Errorf("store: create oidc client %q: %w", c.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return c, invalidf(fmt.Errorf("an OIDC client is already called %q: a client id is unique", c.ID))
	}
	return c, nil
}

// UpdateOIDCClient changes what a client says about itself. The id, the
// creation date and the secret are not its to change: a secret is drawn by
// SetOIDCClientSecret only - and dropped when the client becomes public, since
// a client that keeps no secret must not be accepted with one.
func (s *Store) UpdateOIDCClient(ctx context.Context, c OIDCClient) (OIDCClient, error) {
	old, err := s.GetOIDCClient(ctx, c.ID)
	if err != nil {
		return c, err
	}
	c.SecretHash, c.SecretPrefix, c.CreatedAt = old.SecretHash, old.SecretPrefix, old.CreatedAt
	if err := SanitizeOIDCClient(&c); err != nil {
		return c, err
	}
	c.UpdatedAt = time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`UPDATE oidc_clients SET confidential = ?, secret_hash = ?, secret_prefix = ?,
		   audience = ?, token_ttl = ?, updated_at = ? WHERE id = ?`,
		c.Confidential, c.SecretHash, c.SecretPrefix,
		c.Audience, c.TokenTTL, c.UpdatedAt, c.ID)
	if err != nil {
		return c, fmt.Errorf("store: update oidc client %q: %w", c.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return c, ErrNoOIDCClient
	}
	return c, nil
}

// SetOIDCClientSecret replaces a confidential client's secret: the old one
// stops working at once.
func (s *Store) SetOIDCClientSecret(ctx context.Context, id, hash, prefix string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE oidc_clients SET secret_hash = ?, secret_prefix = ?, updated_at = ? WHERE id = ? AND confidential = ?`,
		hash, prefix, time.Now().Unix(), id, true)
	if err != nil {
		return fmt.Errorf("store: set the secret of oidc client %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetOIDCClient(ctx, id); err != nil {
			return err
		}
		return invalidf(fmt.Errorf("client %q is public: it keeps no secret, and proves itself with PKCE", id))
	}
	return nil
}

// GetOIDCClient returns one client.
func (s *Store) GetOIDCClient(ctx context.Context, id string) (OIDCClient, error) {
	c, err := scanOIDCClient(s.db.QueryRowContext(ctx,
		`SELECT `+oidcClientCols+` FROM oidc_clients WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return OIDCClient{}, ErrNoOIDCClient
	}
	if err != nil {
		return OIDCClient{}, fmt.Errorf("store: oidc client %q: %w", id, err)
	}
	return c, nil
}

// ListOIDCClients returns every client, by id. Never nil.
func (s *Store) ListOIDCClients(ctx context.Context) ([]OIDCClient, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+oidcClientCols+` FROM oidc_clients ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list oidc clients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []OIDCClient{}
	for rows.Next() {
		c, err := scanOIDCClient(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan oidc client: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteOIDCClient removes a client, and with it the codes and refresh tokens
// it was handed: a token of a client that no longer exists must not be
// spendable on a client created later under the same id.
//
// Whether a route still names it is the caller's question (RoutesForOIDCClient):
// the store keeps no reference from a route's JSON to this table.
func (s *Store) DeleteOIDCClient(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM oidc_clients WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete oidc client %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoOIDCClient
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oidc_codes WHERE client_id = ?`, id); err != nil {
		return fmt.Errorf("store: delete the codes of oidc client %q: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oidc_refresh WHERE client_id = ?`, id); err != nil {
		return fmt.Errorf("store: delete the refresh tokens of oidc client %q: %w", id, err)
	}
	return nil
}

// RoutesForOIDCClient returns the routes that sign people into a client, in
// routing order - which is the order the provider tries them in: the first
// whose access rule grants the person decides what its tokens say. Disabled
// routes are included; the caller decides whether they count.
func (s *Store) RoutesForOIDCClient(ctx context.Context, clientID string) ([]Route, error) {
	routes, err := s.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	out := []Route{}
	for _, r := range routes {
		if r.Identity != nil && r.Identity.Mechanism == IdentityOIDC && r.Identity.Client == clientID {
			out = append(out, r)
		}
	}
	return out, nil
}

// OIDCCode is an authorisation code in flight: the person has been granted by
// a route, the application has not yet exchanged the code. Everything the
// exchange needs is captured here, because the browser is gone by then.
type OIDCCode struct {
	Hash     string
	ClientID string
	UserID   string
	TenantID string
	GroupID  string
	// SessionHash is the gateway session the person signed in with: what
	// ties the tokens to a sign-out.
	SessionHash string
	// RouteID is the route whose access rule granted the person, and whose
	// claims the tokens carry.
	RouteID     string
	RedirectURI string
	// Challenge is the PKCE S256 challenge.
	Challenge string
	Nonce     string
	Scope     string
	ExpiresAt int64
}

// SaveOIDCCode stores a freshly issued code. ExpiresAt left at zero is
// OIDCCodeLifetime from now.
func (s *Store) SaveOIDCCode(ctx context.Context, c OIDCCode) error {
	if c.ExpiresAt == 0 {
		c.ExpiresAt = time.Now().Add(OIDCCodeLifetime).Unix()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oidc_codes (code_hash, client_id, user_id, tenant_id, group_id, session_hash, route_id,
		   redirect_uri, challenge, nonce, scope, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Hash, c.ClientID, c.UserID, c.TenantID, c.GroupID, c.SessionHash, c.RouteID,
		c.RedirectURI, c.Challenge, c.Nonce, c.Scope, c.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store: save oidc code: %w", err)
	}
	return nil
}

// TakeOIDCCode returns a code and DELETES it in the same statement, like
// TakeOAuthCode: good once, and "once" has to survive two requests racing.
func (s *Store) TakeOIDCCode(ctx context.Context, hash string) (OIDCCode, error) {
	var c OIDCCode
	err := s.db.QueryRowContext(ctx,
		`DELETE FROM oidc_codes WHERE code_hash = ?
		 RETURNING code_hash, client_id, user_id, tenant_id, group_id, session_hash, route_id,
		   redirect_uri, challenge, nonce, scope, expires_at`, hash).
		Scan(&c.Hash, &c.ClientID, &c.UserID, &c.TenantID, &c.GroupID, &c.SessionHash, &c.RouteID,
			&c.RedirectURI, &c.Challenge, &c.Nonce, &c.Scope, &c.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, fmt.Errorf("this authorisation code is unknown or already used: %w", ErrOIDCGrant)
	}
	if err != nil {
		return c, fmt.Errorf("store: take oidc code: %w", err)
	}
	if time.Now().Unix() >= c.ExpiresAt {
		return c, fmt.Errorf("this authorisation code has expired: %w", ErrOIDCGrant)
	}
	return c, nil
}

// OIDCRefresh is a refresh token an application holds.
type OIDCRefresh struct {
	Hash        string
	ClientID    string
	SessionHash string
	UserID      string
	TenantID    string
	GroupID     string
	Scope       string
	ExpiresAt   int64
}

// SaveOIDCRefresh records a refresh token. It must say when it expires: a
// refresh token that never does is a session nobody can end.
func (s *Store) SaveOIDCRefresh(ctx context.Context, r OIDCRefresh) error {
	if r.ExpiresAt <= 0 {
		return errors.New("store: an oidc refresh token needs an expiry")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oidc_refresh (token_hash, client_id, session_hash, user_id, tenant_id, group_id, scope, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Hash, r.ClientID, r.SessionHash, r.UserID, r.TenantID, r.GroupID, r.Scope, r.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store: save oidc refresh token: %w", err)
	}
	return nil
}

// TakeOIDCRefresh spends a refresh token. Rotating, and deleted on use: the
// caller issues the next one, and a token presented twice means one of the two
// presenters stole it.
func (s *Store) TakeOIDCRefresh(ctx context.Context, hash string) (OIDCRefresh, error) {
	var r OIDCRefresh
	err := s.db.QueryRowContext(ctx,
		`DELETE FROM oidc_refresh WHERE token_hash = ?
		 RETURNING token_hash, client_id, session_hash, user_id, tenant_id, group_id, scope, expires_at`, hash).
		Scan(&r.Hash, &r.ClientID, &r.SessionHash, &r.UserID, &r.TenantID, &r.GroupID, &r.Scope, &r.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, fmt.Errorf("this refresh token is unknown or already used: %w", ErrOIDCGrant)
	}
	if err != nil {
		return r, fmt.Errorf("store: take oidc refresh token: %w", err)
	}
	if time.Now().Unix() >= r.ExpiresAt {
		return r, fmt.Errorf("this refresh token has expired: %w", ErrOIDCGrant)
	}
	return r, nil
}

// RevokeOIDCSession drops what a gateway session handed to applications - its
// codes in flight and its refresh tokens - so a sign-out at the gateway ends
// them too. The access tokens already out live until their short expiry.
func (s *Store) RevokeOIDCSession(ctx context.Context, sessionHash string) (int64, error) {
	if sessionHash == "" {
		return 0, nil
	}
	var total int64
	for _, q := range []string{
		`DELETE FROM oidc_codes WHERE session_hash = ?`,
		`DELETE FROM oidc_refresh WHERE session_hash = ?`,
	} {
		res, err := s.db.ExecContext(ctx, q, sessionHash)
		if err != nil {
			return total, fmt.Errorf("store: revoke oidc session: %w", err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// PurgeOIDCLeftovers drops the codes and refresh tokens past their expiry:
// rows nobody can spend any more. Run by the periodic upkeep.
func (s *Store) PurgeOIDCLeftovers(ctx context.Context, now int64) (int64, error) {
	var total int64
	for _, q := range []string{
		`DELETE FROM oidc_codes WHERE expires_at < ?`,
		`DELETE FROM oidc_refresh WHERE expires_at < ?`,
	} {
		res, err := s.db.ExecContext(ctx, q, now)
		if err != nil {
			return total, fmt.Errorf("store: purge oidc leftovers: %w", err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}
