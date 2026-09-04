package store

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// API tokens (AUTH-16): a personal access token authenticates its owner on
// the DATA plane's API routes without a browser session. It CAPTURES the
// session context it was born in - the active tenant and (in exclusive mode)
// the active group - so an API call needs no interactive tenant/group choice.
// Only the token HASH is stored; the clear value is shown once at creation.

// A token's PERIMETER (MCP-02). A control-plane token acts with its owner's
// capabilities, which for the tokens minted today means root: the perimeter is
// what makes handing one to an agent survivable. It can only ever TAKE AWAY -
// it never grants what the owner does not have - so the whole rights model
// stays the accounts' one, with the token rogner-ing it.
//
// Enforced on the CONTROL plane, where it is checked once for every endpoint
// (admin.authed) and therefore covers the REST API and the agent endpoint
// alike. Data-plane tokens are minted ScopeFull: the data plane proxies an
// application's own traffic, and read-only there would be a different feature.
const (
	// ScopeFull: everything the owner may do.
	ScopeFull = "full"
	// ScopeReadOnly: reads only. What counts as a read is decided by the
	// endpoint, not by the HTTP verb - see admin.readsOnly.
	ScopeReadOnly = "readonly"
)

// TokenScopes are the allowed perimeters, in the order a form should offer
// them (the safe one first).
var TokenScopes = []string{ScopeReadOnly, ScopeFull}

// The token's DOMAIN, the second axis of the perimeter (MCP-02). Scope says
// how far a token may go; domain says over what.
//
// It composes with the capabilities rather than duplicating them: a domain
// MASKS what its owner holds, so a token is at most its owner and often less.
// A gateway token minted by root administers the routing plane and nothing
// else - not the accounts, not the organisations, and not the root-only
// screens - which is what makes handing one to a deployment agent a smaller
// decision than handing over root.
const (
	// DomainAll: everything the owner may do.
	DomainAll = ""
	// DomainGateway: the routing plane only (the gateway-admin capability).
	DomainGateway = "gateway"
	// DomainApp: the application's identity only (the app-admin capability,
	// and the organisations its owner administers).
	DomainApp = "app"
)

// TokenDomains are the allowed domains, widest last: a form offers the
// narrowest first for the same reason the scope does.
var TokenDomains = []string{DomainGateway, DomainApp, DomainAll}

// SanitizeTokenDomain normalizes a domain and names what is allowed. Unlike
// the scope, an empty value is the WIDE one: a domain is a restriction someone
// adds, and "the whole control plane" is what a token without one has always
// meant.
func SanitizeTokenDomain(domain string) (string, error) {
	switch d := strings.ToLower(strings.TrimSpace(domain)); d {
	case DomainAll, "all":
		return DomainAll, nil
	case DomainGateway, DomainApp:
		return d, nil
	default:
		return "", fmt.Errorf("token domain %q: allowed are %s, %s, or none for the whole control plane",
			domain, DomainGateway, DomainApp)
	}
}

// SanitizeTokenCIDRs normalizes the addresses a token may be used from
// (MCP-02), and refuses what does not parse - a typo here would silently allow
// nobody, and the owner would blame the token.
//
// Judged on the TCP PEER and never on X-Forwarded-For, which is a header
// anybody can write. The consequence is worth saying out loud where an
// administrator will read it: a control plane sitting behind a reverse proxy
// sees the proxy's address for every caller, so this restriction is only
// meaningful when agents reach the port directly.
func SanitizeTokenCIDRs(raw string) (string, error) {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// A bare address is the obvious thing to type, and refusing it would
		// teach nothing: it means that address alone.
		if !strings.Contains(part, "/") {
			if ip, err := netip.ParseAddr(part); err == nil {
				part = ip.String() + "/" + strconv.Itoa(ip.BitLen())
			}
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return "", fmt.Errorf("token address %q: expected an address or a CIDR range, like 10.0.0.7 or 10.0.0.0/24", part)
		}
		out = append(out, prefix.Masked().String())
	}
	return strings.Join(out, ","), nil
}

// AllowsAddress reports whether a token restricted to these ranges may be used
// from addr (a host, or a host:port as net/http reports it). No ranges at all
// means anywhere, which is what every token has always been.
func AllowsAddress(cidrs, addr string) bool {
	if strings.TrimSpace(cidrs) == "" {
		return true
	}
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return false
	}
	// An IPv4 address arriving on a dual-stack listener wears an IPv6 coat
	// (::ffff:10.0.0.7): unwrap it, or a v4 range would never match.
	ip = ip.Unmap()
	for _, raw := range strings.Split(cidrs, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// SanitizeTokenScope normalizes a perimeter and names what is allowed when it
// is not one of them. An empty value is the SAFE one: a token whose perimeter
// nobody stated is not a token that may rewrite the gateway.
func SanitizeTokenScope(scope string) (string, error) {
	switch s := strings.ToLower(strings.TrimSpace(scope)); s {
	case "":
		return ScopeReadOnly, nil
	case ScopeFull, ScopeReadOnly:
		return s, nil
	default:
		return "", fmt.Errorf("token perimeter %q: allowed are %s and %s", scope, ScopeReadOnly, ScopeFull)
	}
}

// APITokensAllowed reads the gateway-wide policy (SettingAPITokens): whether
// users may mint and use API tokens. Missing key = allowed (ships on).
func (s *Store) APITokensAllowed(ctx context.Context) bool {
	var allowed bool
	if err := s.GetSetting(ctx, SettingAPITokens, &allowed); err != nil {
		return true
	}
	return allowed
}

// APIToken is one token prepared for display (never carries the hash out).
type APIToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"` // first clear chars, to recognise it
	Plane     string `json:"plane"`  // data | admin (control plane)
	Scope     string `json:"scope"`  // full | readonly (MCP-02)
	Domain    string `json:"domain"` // "" | gateway | app (MCP-02)
	FromCIDRs string `json:"fromCidrs,omitempty"`
	// ClientID names the registered agent this token was issued to (MCP-07),
	// empty for one minted by hand. A token with one is a CONNECTION, not a
	// key somebody carries around.
	ClientID   string `json:"clientId,omitempty"`
	ClientName string `json:"clientName,omitempty"`
	TenantID   string `json:"tenantId"`
	TenantName string `json:"tenantName"`
	GroupID    string `json:"groupId"`
	GroupName  string `json:"groupName"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"` // 0 = never
	LastUsedAt int64  `json:"lastUsedAt"`
}

// ResolvedToken is the context a valid token authenticates as.
type ResolvedToken struct {
	ID string
	// Name is carried so the audit can say WHICH token acted, not merely whose
	// account it was minted on (MCP-03).
	Name       string
	UserID     string
	Plane      string // the plane this token is valid on (data | admin)
	Scope      string // full | readonly (MCP-02)
	Domain     string // "" | gateway | app (MCP-02)
	FromCIDRs  string // comma-separated ranges, empty = anywhere
	TenantID   string
	GroupID    string
	LastUsedAt int64
}

// NewToken is a token about to be recorded. A struct rather than a dozen
// positional strings: the two call sites (a personal data token, a
// control-plane token) already disagree on half the fields, and a caller that
// swaps two of them would mint something nobody notices until it authenticates.
type NewToken struct {
	ID        string
	UserID    string
	Name      string
	TokenHash string
	Prefix    string
	Plane     string // data | admin, defaults to data
	Scope     string // full | readonly, defaults to the safe one
	Domain    string // "" | gateway | app
	FromCIDRs string // comma-separated ranges, empty = anywhere
	ClientID  string // the registered agent, empty when minted by hand
	TenantID  string // captured session context, empty on the control plane
	GroupID   string
	ExpiresAt int64 // 0 = never
}

// AddAPIToken records a freshly minted token by its hash.
func (s *Store) AddAPIToken(ctx context.Context, t NewToken) error {
	if t.Plane == "" {
		t.Plane = PlaneData
	}
	scope, err := SanitizeTokenScope(t.Scope)
	if err != nil {
		return err
	}
	domain, err := SanitizeTokenDomain(t.Domain)
	if err != nil {
		return err
	}
	cidrs, err := SanitizeTokenCIDRs(t.FromCIDRs)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (id, user_id, name, token_hash, prefix, plane, scope, domain, from_cidrs, client_id, tenant_id, group_id, enabled, created_at, expires_at, last_used_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		t.ID, t.UserID, t.Name, t.TokenHash, t.Prefix, t.Plane, scope, domain, cidrs, t.ClientID,
		t.TenantID, t.GroupID, true, time.Now().Unix(), t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store: add api token for %q: %w", t.UserID, err)
	}
	return nil
}

// ResolveAPIToken maps a token hash to its live context (AUTH-16): present,
// enabled, unexpired. The caller still checks the user is enabled. Returns
// sql.ErrNoRows when absent/disabled/expired - all "no token".
func (s *Store) ResolveAPIToken(ctx context.Context, tokenHash string, now int64) (ResolvedToken, error) {
	var t ResolvedToken
	var enabled bool
	var expires int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, user_id, plane, scope, domain, from_cidrs, tenant_id, group_id, enabled, expires_at, last_used_at FROM api_tokens WHERE token_hash = ?`,
		tokenHash).Scan(&t.ID, &t.Name, &t.UserID, &t.Plane, &t.Scope, &t.Domain, &t.FromCIDRs,
		&t.TenantID, &t.GroupID, &enabled, &expires, &t.LastUsedAt)
	if err != nil {
		return ResolvedToken{}, err // sql.ErrNoRows is the "no token" signal
	}
	if !enabled || (expires != 0 && now >= expires) {
		return ResolvedToken{}, sql.ErrNoRows
	}
	return t, nil
}

// TouchAPIToken stamps a token's last use (best-effort, throttled by caller).
func (s *Store) TouchAPIToken(ctx context.Context, id string, now int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, now, id)
	return err
}

// ListAPITokens returns a user's tokens ON THE GIVEN PLANE (with tenant names),
// newest first. The two planes are listed separately (the profile page shows
// data tokens, the Gateway page shows admin tokens).
func (s *Store) ListAPITokens(ctx context.Context, userID, plane string) ([]APIToken, error) {
	if plane == "" {
		plane = PlaneData
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.name, a.prefix, a.plane, a.scope, a.domain, a.from_cidrs,
		        a.client_id, COALESCE(c.name, ''),
		        a.tenant_id, COALESCE(t.name, ''), a.group_id, COALESCE(g.name, ''),
		        a.enabled, a.created_at, a.expires_at, a.last_used_at
		 FROM api_tokens a
		 LEFT JOIN tenants t ON t.id = a.tenant_id
		 LEFT JOIN groups g ON g.id = a.group_id
		 LEFT JOIN oauth_clients c ON c.id = a.client_id
		 WHERE a.user_id = ? AND a.plane = ? ORDER BY a.created_at DESC`, userID, plane)
	if err != nil {
		return nil, fmt.Errorf("store: list api tokens for %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []APIToken
	for rows.Next() {
		var a APIToken
		if err := rows.Scan(&a.ID, &a.Name, &a.Prefix, &a.Plane, &a.Scope, &a.Domain, &a.FromCIDRs,
			&a.ClientID, &a.ClientName,
			&a.TenantID, &a.TenantName, &a.GroupID, &a.GroupName,
			&a.Enabled, &a.CreatedAt, &a.ExpiresAt, &a.LastUsedAt); err != nil {
			return nil, fmt.Errorf("store: scan api token: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetAPITokenEnabled toggles a token on/off (scoped by user), reporting
// whether it existed.
func (s *Store) SetAPITokenEnabled(ctx context.Context, userID, id string, enabled bool) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET enabled = ? WHERE id = ? AND user_id = ?`, enabled, id, userID)
	if err != nil {
		return false, fmt.Errorf("store: set api token %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RevokeAPIToken deletes one of the user's tokens (scoped by user), reporting
// whether it existed.
func (s *Store) RevokeAPIToken(ctx context.Context, userID, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return false, fmt.Errorf("store: revoke api token %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteAPIToken drops one token by id, whoever owns it. Used by the OAuth
// revocation endpoint, where the caller proved possession of the token rather
// than of the account.
func (s *Store) DeleteAPIToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete api token %q: %w", id, err)
	}
	return nil
}

// PurgeExpiredAPITokens drops lapsed tokens (periodic upkeep).
func (s *Store) PurgeExpiredAPITokens(ctx context.Context, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM api_tokens WHERE expires_at != 0 AND expires_at <= ?`, now)
	if err != nil {
		return 0, fmt.Errorf("store: purge api tokens: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
