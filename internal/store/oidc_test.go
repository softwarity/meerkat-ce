package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
)

func oidcClient(id string) OIDCClient {
	return OIDCClient{ID: id, Confidential: true}
}

func TestOIDCClientRoundTrip(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()

	secret, hash, prefix, err := NewOIDCSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, OIDCSecretPrefix) || !strings.HasPrefix(secret, prefix) || hash == secret {
		t.Fatalf("secret %q, prefix %q, hash %q: not what a secret looks like", secret, prefix, hash)
	}
	c := oidcClient("billing")
	c.SecretHash, c.SecretPrefix = hash, prefix
	c.Audience = "billing-api"
	c.TokenTTL = "PT10M"
	saved, err := s.CreateOIDCClient(ctx, c)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if saved.CreatedAt == 0 || saved.UpdatedAt == 0 {
		t.Errorf("timestamps not stamped: %+v", saved)
	}
	got, err := s.GetOIDCClient(ctx, "billing")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.Confidential || got.Audience != "billing-api" || got.TokenTTL != "PT10M" || got.SecretPrefix != prefix {
		t.Errorf("read back wrong: %+v", got)
	}
	if !got.SecretMatches(secret) || got.SecretMatches(secret+"x") || got.SecretMatches("") {
		t.Error("the secret does not match itself, or matches something else")
	}
	if got.TTL() != 10*time.Minute || got.AudienceOrID() != "billing-api" {
		t.Errorf("ttl %v audience %q", got.TTL(), got.AudienceOrID())
	}

	// A second client of the same id is refused, by name.
	if _, err := s.CreateOIDCClient(ctx, oidcClient("billing")); err == nil || !errors.Is(err, ErrInvalid) ||
		!strings.Contains(err.Error(), `"billing"`) {
		t.Errorf("duplicate id: %v", err)
	}

	// An update keeps the secret and the creation date.
	got.Audience = ""
	got.SecretHash, got.SecretPrefix = "forged", "forged"
	upd, err := s.UpdateOIDCClient(ctx, got)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	back, _ := s.GetOIDCClient(ctx, "billing")
	if back.SecretHash != hash || back.SecretPrefix != prefix || back.CreatedAt != saved.CreatedAt || back.Audience != "" {
		t.Errorf("update touched what it must not: %+v", back)
	}
	if upd.AudienceOrID() != "billing" {
		t.Errorf("empty audience is the client id, got %q", upd.AudienceOrID())
	}

	// A new secret replaces the old one.
	secret2, hash2, prefix2, _ := NewOIDCSecret()
	if err := s.SetOIDCClientSecret(ctx, "billing", hash2, prefix2); err != nil {
		t.Fatal(err)
	}
	back, _ = s.GetOIDCClient(ctx, "billing")
	if back.SecretMatches(secret) || !back.SecretMatches(secret2) {
		t.Error("the old secret still works, or the new one does not")
	}

	// Made public, it drops its secret, and no secret can be set on it.
	back.Confidential = false
	if _, err := s.UpdateOIDCClient(ctx, back); err != nil {
		t.Fatal(err)
	}
	back, _ = s.GetOIDCClient(ctx, "billing")
	if back.SecretHash != "" || back.SecretPrefix != "" || back.SecretMatches(secret2) {
		t.Errorf("a public client kept a secret: %+v", back)
	}
	if err := s.SetOIDCClientSecret(ctx, "billing", hash2, prefix2); err == nil || !errors.Is(err, ErrInvalid) {
		t.Errorf("a secret was set on a public client: %v", err)
	}
	if err := s.SetOIDCClientSecret(ctx, "nobody", hash2, prefix2); !errors.Is(err, ErrNoOIDCClient) {
		t.Errorf("secret on an unknown client: %v", err)
	}

	list, err := s.ListOIDCClients(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	if _, err := s.UpdateOIDCClient(ctx, oidcClient("nobody")); !errors.Is(err, ErrNoOIDCClient) {
		t.Errorf("update of an unknown client: %v", err)
	}
	if err := s.DeleteOIDCClient(ctx, "billing"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteOIDCClient(ctx, "billing"); !errors.Is(err, ErrNoOIDCClient) {
		t.Errorf("second delete: %v", err)
	}
	if list, _ := s.ListOIDCClients(ctx); list == nil || len(list) != 0 {
		t.Errorf("an empty list must be empty, not nil: %#v", list)
	}
}

func TestOIDCClientRefusalsNameWhatIsAllowed(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(*OIDCClient)
		want string
	}{
		{"id upper case", func(c *OIDCClient) { c.ID = "Billing" }, "lower-case"},
		{"id empty", func(c *OIDCClient) { c.ID = "" }, "lower-case"},
		{"ttl short", func(c *OIDCClient) { c.TokenTTL = "PT30S" }, "from PT1M to PT24H"},
		{"ttl long", func(c *OIDCClient) { c.TokenTTL = "P2D" }, "from PT1M to PT24H"},
		{"ttl junk", func(c *OIDCClient) { c.TokenTTL = "5m" }, "ISO-8601"},
		{"audience control", func(c *OIDCClient) { c.Audience = "my\tapp" }, "no control character"},
		{"audience long", func(c *OIDCClient) { c.Audience = strings.Repeat("a", 256) }, "up to 255 characters"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cl := oidcClient("billing")
			c.edit(&cl)
			err := SanitizeOIDCClient(&cl)
			if err == nil || !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want a refusal saying %q", err, c.want)
			}
		})
	}
}

func TestOIDCCodesAreSpentOnce(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	code := OIDCCode{Hash: "h1", ClientID: "billing", UserID: "u1", TenantID: "t1", GroupID: "g1",
		SessionHash: "sess", RouteID: "r1", RedirectURI: "https://app.example.com/callback",
		Challenge: "chal", Nonce: "n", Scope: "openid profile"}
	if err := s.SaveOIDCCode(ctx, code); err != nil {
		t.Fatal(err)
	}
	got, err := s.TakeOIDCCode(ctx, "h1")
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	code.ExpiresAt = got.ExpiresAt
	if got != code {
		t.Errorf("read back %+v, want %+v", got, code)
	}
	if left := got.ExpiresAt - time.Now().Unix(); left <= 0 || left > int64(OIDCCodeLifetime/time.Second) {
		t.Errorf("a code lives %ds", left)
	}
	if _, err := s.TakeOIDCCode(ctx, "h1"); !errors.Is(err, ErrOIDCGrant) || !strings.Contains(err.Error(), "already used") {
		t.Errorf("a code was spent twice: %v", err)
	}
	if err := s.SaveOIDCCode(ctx, OIDCCode{Hash: "old", ExpiresAt: time.Now().Add(-time.Second).Unix()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TakeOIDCCode(ctx, "old"); !errors.Is(err, ErrOIDCGrant) || !strings.Contains(err.Error(), "expired") {
		t.Errorf("an expired code: %v", err)
	}
}

func TestOIDCRefreshRotates(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.SaveOIDCRefresh(ctx, OIDCRefresh{Hash: "r1"}); err == nil {
		t.Error("a refresh token without an expiry was accepted")
	}
	r := OIDCRefresh{Hash: "r1", ClientID: "billing", SessionHash: "sess", UserID: "u1", TenantID: "t1",
		GroupID: "g1", Scope: "openid", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	if err := s.SaveOIDCRefresh(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err := s.TakeOIDCRefresh(ctx, "r1")
	if err != nil || got != r {
		t.Fatalf("take: %+v %v", got, err)
	}
	if _, err := s.TakeOIDCRefresh(ctx, "r1"); !errors.Is(err, ErrOIDCGrant) {
		t.Errorf("a refresh token was spent twice: %v", err)
	}
	if err := s.SaveOIDCRefresh(ctx, OIDCRefresh{Hash: "r2", ExpiresAt: time.Now().Add(-time.Second).Unix()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TakeOIDCRefresh(ctx, "r2"); !errors.Is(err, ErrOIDCGrant) || !strings.Contains(err.Error(), "expired") {
		t.Errorf("an expired refresh token: %v", err)
	}
}

func TestOIDCLeftoversGo(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	now := time.Now()
	past, future := now.Add(-time.Hour).Unix(), now.Add(time.Hour).Unix()
	for _, c := range []OIDCCode{
		{Hash: "c-old", ClientID: "a", SessionHash: "s1", ExpiresAt: past},
		{Hash: "c-live", ClientID: "a", SessionHash: "s2", ExpiresAt: future},
		{Hash: "c-gone-client", ClientID: "b", SessionHash: "s3", ExpiresAt: future},
	} {
		if err := s.SaveOIDCCode(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []OIDCRefresh{
		{Hash: "r-old", ClientID: "a", SessionHash: "s1", ExpiresAt: past},
		{Hash: "r-live", ClientID: "a", SessionHash: "s2", ExpiresAt: future},
		{Hash: "r-signed-out", ClientID: "a", SessionHash: "s4", ExpiresAt: future},
		{Hash: "r-gone-client", ClientID: "b", SessionHash: "s3", ExpiresAt: future},
	} {
		if err := s.SaveOIDCRefresh(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.PurgeOIDCLeftovers(ctx, now.Unix())
	if err != nil || n != 2 {
		t.Errorf("purged %d (%v), want the 2 expired rows", n, err)
	}
	// A sign-out drops what its session handed out.
	if n, err := s.RevokeOIDCSession(ctx, "s4"); err != nil || n != 1 {
		t.Errorf("revoked %d (%v), want 1", n, err)
	}
	// A deleted client takes its codes and tokens with it.
	if _, err := s.CreateOIDCClient(ctx, oidcClient("b")); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteOIDCClient(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"c-gone-client"} {
		if _, err := s.TakeOIDCCode(ctx, h); err == nil {
			t.Errorf("code %s outlived its client", h)
		}
	}
	for _, h := range []string{"r-gone-client", "r-signed-out"} {
		if _, err := s.TakeOIDCRefresh(ctx, h); err == nil {
			t.Errorf("refresh token %s should be gone", h)
		}
	}
	if _, err := s.TakeOIDCCode(ctx, "c-live"); err != nil {
		t.Errorf("a live code went: %v", err)
	}
	if _, err := s.TakeOIDCRefresh(ctx, "r-live"); err != nil {
		t.Errorf("a live refresh token went: %v", err)
	}
}

func TestRoutesForOIDCClientInRoutingOrder(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	preds := []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}}
	for _, r := range []Route{
		{ID: "admins", Name: "app-admins", Order: 2, Upstream: "http://a", Predicates: preds,
			Identity: &IdentityForward{Mechanism: IdentityOIDC, Client: "app"}},
		{ID: "users", Name: "app-users", Order: 5, Upstream: "http://a", Predicates: preds,
			Identity: &IdentityForward{Mechanism: IdentityOIDC, Client: "app"}},
		{ID: "other", Name: "other", Order: 1, Upstream: "http://a", Predicates: preds,
			Identity: &IdentityForward{Mechanism: IdentityOIDC, Client: "other"}},
		{ID: "jwt", Name: "jwt", Order: 0, Upstream: "http://a", Predicates: preds,
			Identity: &IdentityForward{Mechanism: "jwt"}},
	} {
		if err := s.SaveRoute(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.RoutesForOIDCClient(ctx, "app")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "admins" || got[1].ID != "users" {
		t.Errorf("routes for app: %+v", got)
	}
	if got[0].Identity.Client != "app" {
		t.Error("the client did not survive the route's round trip")
	}
	none, err := s.RoutesForOIDCClient(ctx, "nobody")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("no routes must be an empty list: %#v %v", none, err)
	}
}

// One rule for every aud the gateway writes: a client's audience takes a
// space inside, as a route's does (its default, the route's name, may hold one).
func TestOIDCClientAudienceTakesASpaceInside(t *testing.T) {
	cl := oidcClient("billing")
	cl.Audience = "  Billing API  "
	if err := SanitizeOIDCClient(&cl); err != nil {
		t.Fatalf("audience with a space inside refused: %v", err)
	}
	if cl.Audience != "Billing API" {
		t.Fatalf("audience = %q, want it trimmed", cl.Audience)
	}
}
