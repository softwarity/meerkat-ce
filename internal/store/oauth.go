package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// The OAuth half of the agent endpoint (MCP-07): what lets an agent connect
// WITHOUT anybody copying a secret.
//
// The shape is the one Jira and Linear present to a client: the agent asks the
// gateway who may authorise it, registers itself, sends its user to a consent
// page, and comes back with a token nobody typed. What the person approves on
// that page is the PERIMETER - which is why the perimeter did not disappear
// with the minting dialog, it moved to the moment somebody actually connects
// an agent.
//
// The access token that comes out is an ordinary control-plane token
// (api_tokens, with client_id set), so everything already built keeps working
// unchanged: the plane isolation, the perimeter, the audit naming the token,
// and revoking it from one place.

// OAuthClient is an agent that registered itself (RFC 7591). There is no
// secret: these are command-line and desktop programs, which cannot keep one.
// What stands in for it is PKCE plus a redirect back to the loopback address.
type OAuthClient struct {
	ID           string   `json:"clientId"`
	Name         string   `json:"clientName"`
	RedirectURIs []string `json:"redirectUris"`
	CreatedAt    int64    `json:"createdAt"`
}

// maxRedirectURIs bounds a registration. A client needs one or two; a hundred
// is somebody filling the table.
const maxRedirectURIs = 8

// SanitizeRedirectURI refuses what a public client may not come back on.
//
// Loopback (RFC 8252) and https, and nothing else: a redirect to another host
// is how an authorisation code leaves for somewhere its owner never intended.
// The PORT of a loopback address is deliberately not checked at registration -
// a CLI takes whatever port the operating system gives it - so the comparison
// at authorise time ignores it too.
func SanitizeRedirectURI(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" && u.Scheme != "" && u.Opaque == "" {
		return "", fmt.Errorf("redirect uri %q: not a url", raw)
	}
	host := u.Hostname()
	switch {
	case u.Scheme == "http" && (host == "127.0.0.1" || host == "::1" || host == "localhost"):
		return u.String(), nil
	case u.Scheme == "https":
		return u.String(), nil
	default:
		return "", fmt.Errorf(
			"redirect uri %q: only https and loopback http (127.0.0.1, ::1, localhost) are allowed", raw)
	}
}

// SameRedirect compares a redirect against a registered one, ignoring the port
// of a loopback address: a command-line client is handed a free port by the
// system every time it runs, and pinning it would break the second run.
func SameRedirect(registered, asked string) bool {
	if registered == asked {
		return true
	}
	a, errA := url.Parse(registered)
	b, errB := url.Parse(asked)
	if errA != nil || errB != nil || a.Scheme != "http" || b.Scheme != "http" {
		return false
	}
	if !isLoopback(a.Hostname()) || !isLoopback(b.Hostname()) {
		return false
	}
	return a.Hostname() == b.Hostname() && a.Path == b.Path
}

func isLoopback(host string) bool {
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// SaveOAuthClient records a registration.
func (s *Store) SaveOAuthClient(ctx context.Context, c OAuthClient) error {
	if len(c.RedirectURIs) == 0 {
		return errors.New("a client registration needs at least one redirect uri")
	}
	if len(c.RedirectURIs) > maxRedirectURIs {
		return fmt.Errorf("a client registration is limited to %d redirect uris", maxRedirectURIs)
	}
	clean := make([]string, 0, len(c.RedirectURIs))
	for _, raw := range c.RedirectURIs {
		uri, err := SanitizeRedirectURI(raw)
		if err != nil {
			return err
		}
		clean = append(clean, uri)
	}
	name := strings.TrimSpace(c.Name)
	if name == "" {
		name = "an agent"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oauth_clients (id, name, redirect_uris, created_at) VALUES (?, ?, ?, ?)`,
		c.ID, name, strings.Join(clean, "\n"), time.Now().Unix())
	if err != nil {
		return fmt.Errorf("store: register oauth client: %w", err)
	}
	return nil
}

// ErrNoOAuthClient is what an unknown client_id answers.
var ErrNoOAuthClient = errors.New("store: unknown oauth client")

// GetOAuthClient returns a registration.
func (s *Store) GetOAuthClient(ctx context.Context, id string) (OAuthClient, error) {
	var c OAuthClient
	var uris string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, redirect_uris, created_at FROM oauth_clients WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &uris, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNoOAuthClient
	}
	if err != nil {
		return c, fmt.Errorf("store: oauth client %q: %w", id, err)
	}
	c.RedirectURIs = strings.Split(uris, "\n")
	return c, nil
}

// OAuthCode is an authorisation code in flight: the person has approved, the
// agent has not yet exchanged it. Everything the exchange needs is captured
// here, because the browser that approved is gone by then.
type OAuthCode struct {
	Hash        string
	ClientID    string
	UserID      string
	RedirectURI string
	// Challenge is the PKCE S256 challenge. Mandatory: a public client has no
	// secret, and this is what stops a code stolen from a redirect being spent
	// by somebody else.
	Challenge string
	Scope     string // the perimeter approved: readonly | full
	Domain    string // and over what: "" | gateway | app
	Resource  string // the audience the token is for (RFC 8707)
	ExpiresAt int64
}

// codeLifetime is deliberately short: a code is spent within seconds of being
// issued, by a program that is already waiting for it.
const codeLifetime = 5 * time.Minute

// SaveOAuthCode stores a freshly approved code.
func (s *Store) SaveOAuthCode(ctx context.Context, c OAuthCode) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, challenge, scope, domain, resource, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Hash, c.ClientID, c.UserID, c.RedirectURI, c.Challenge, c.Scope, c.Domain, c.Resource,
		time.Now().Add(codeLifetime).Unix())
	if err != nil {
		return fmt.Errorf("store: save oauth code: %w", err)
	}
	return nil
}

// TakeOAuthCode returns a code and DELETES it in the same breath: a code is
// good once, and "once" has to survive two clients racing for it.
func (s *Store) TakeOAuthCode(ctx context.Context, hash string) (OAuthCode, error) {
	var c OAuthCode
	err := s.db.QueryRowContext(ctx,
		`DELETE FROM oauth_codes WHERE code_hash = ?
		 RETURNING code_hash, client_id, user_id, redirect_uri, challenge, scope, domain, resource, expires_at`, hash).
		Scan(&c.Hash, &c.ClientID, &c.UserID, &c.RedirectURI, &c.Challenge, &c.Scope, &c.Domain, &c.Resource, &c.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, errors.New("this authorisation code is unknown or already used")
	}
	if err != nil {
		return c, fmt.Errorf("store: take oauth code: %w", err)
	}
	if time.Now().Unix() >= c.ExpiresAt {
		return c, errors.New("this authorisation code has expired")
	}
	return c, nil
}

// SaveRefreshToken records the refresh token paired with an access token.
func (s *Store) SaveRefreshToken(ctx context.Context, hash, tokenID string, expiresAt int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oauth_refresh (token_hash, token_id, expires_at) VALUES (?, ?, ?)`,
		hash, tokenID, expiresAt)
	if err != nil {
		return fmt.Errorf("store: save refresh token: %w", err)
	}
	return nil
}

// TakeRefreshToken spends a refresh token, returning the access token it
// renews. Rotating, and deleted on use: a refresh token presented twice means
// one of the two presenters stole it, and neither gets to go on.
func (s *Store) TakeRefreshToken(ctx context.Context, hash string) (tokenID string, err error) {
	var expiresAt int64
	err = s.db.QueryRowContext(ctx,
		`DELETE FROM oauth_refresh WHERE token_hash = ? RETURNING token_id, expires_at`, hash).
		Scan(&tokenID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("this refresh token is unknown or already used")
	}
	if err != nil {
		return "", fmt.Errorf("store: take refresh token: %w", err)
	}
	if time.Now().Unix() >= expiresAt {
		return "", errors.New("this refresh token has expired")
	}
	return tokenID, nil
}

// PurgeOAuthLeftovers drops expired codes and refresh tokens. Called by the
// same ticker that purges the audit trail: rows nobody can use any more are
// rows nobody should have to think about.
func (s *Store) PurgeOAuthLeftovers(ctx context.Context) error {
	now := time.Now().Unix()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oauth_codes WHERE expires_at < ?`, now); err != nil {
		return fmt.Errorf("store: purge oauth codes: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oauth_refresh WHERE expires_at < ?`, now); err != nil {
		return fmt.Errorf("store: purge refresh tokens: %w", err)
	}
	return nil
}

// RenewAPIToken replaces the secret of an existing token, keeping its identity
// (its name, its perimeter, the client it belongs to) and its place in the
// list. This is what a refresh does: the same connection, a new key.
func (s *Store) RenewAPIToken(ctx context.Context, id, tokenHash, prefix string, expiresAt int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET token_hash = ?, prefix = ?, expires_at = ? WHERE id = ?`,
		tokenHash, prefix, expiresAt, id)
	if err != nil {
		return fmt.Errorf("store: renew api token: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("this connection has been revoked")
	}
	return nil
}
