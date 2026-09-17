package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

func bearer(token string) *http.Request {
	req := httptest.NewRequest("GET", "/api/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// mintToken stores a token whose CLEAR value is "mk_<secret>" and returns the
// secret value - the hash matches what the resolver computes (sha256 hex).
func mintToken(t *testing.T, st *store.Store, userID, tenantID, groupID string, expiresAt int64) string {
	t.Helper()
	secret := "mk_" + "secret-" + userID + "-" + groupID + tenantID
	if err := st.AddAPIToken(context.Background(), store.NewToken{
		ID: "tok-" + userID + groupID + tenantID, UserID: userID, Name: "test",
		TokenHash: hashToken(secret), Prefix: secret[:10], Plane: store.PlaneData,
		Scope: store.ScopeFull, TenantID: tenantID, GroupID: groupID, ExpiresAt: expiresAt,
	}); err != nil {
		t.Fatalf("AddAPIToken: %v", err)
	}
	return secret
}

func enabledUser(t *testing.T, st *store.Store, id string) {
	t.Helper()
	if err := st.CreateUser(context.Background(), store.User{ID: id, Username: id, PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
}

func TestBearerTokenResolves(t *testing.T) {
	m, st := setup(t)
	enabledUser(t, st, "alice")
	secret := mintToken(t, st, "alice", "t1", "g1", 0)

	sess, err := m.Resolve(context.Background(), bearer(secret))
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	if sess.UserID != "alice" || sess.TenantID != "t1" || sess.GroupID != "g1" {
		t.Fatalf("token session = %+v, want alice/t1/g1", sess)
	}
}

func TestBearerTokenRevokedAndDisabled(t *testing.T) {
	m, st := setup(t)
	enabledUser(t, st, "alice")
	secret := mintToken(t, st, "alice", "t1", "", 0)
	ctx := context.Background()

	// Disabled -> refused; re-enabled -> works again. (id = tok-alice + "" + t1)
	const id = "tok-alicet1"
	// Every write is followed by TokenChanged, as the handlers do: the token
	// was resolved (and remembered) before each change.
	if _, err := m.Resolve(ctx, bearer(secret)); err != nil {
		t.Fatalf("live token should resolve: %v", err)
	}
	if _, err := st.SetAPITokenEnabled(ctx, "alice", id, false); err != nil {
		t.Fatal(err)
	}
	m.TokenChanged(id)
	if _, err := m.Resolve(ctx, bearer(secret)); err == nil {
		t.Fatalf("a disabled token must not resolve")
	}
	if _, err := st.SetAPITokenEnabled(ctx, "alice", id, true); err != nil {
		t.Fatal(err)
	}
	m.TokenChanged(id)
	if _, err := m.Resolve(ctx, bearer(secret)); err != nil {
		t.Fatalf("a re-enabled token must resolve: %v", err)
	}
	// Revoked -> gone.
	if _, err := st.RevokeAPIToken(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	m.TokenChanged(id)
	if _, err := m.Resolve(ctx, bearer(secret)); err == nil {
		t.Fatalf("a revoked token must not resolve")
	}
}

// A token authenticates ONLY on its own plane: a data token never opens the
// admin port, an admin (control-plane) token never opens the data port.
func TestBearerTokenPlaneIsolation(t *testing.T) {
	dataM, st := setup(t)
	adminM := NewManager(st, ForAdminPlane()) // same store, admin plane
	ctx := context.Background()
	enabledUser(t, st, "root")

	// A DATA token resolves on the data plane but not the admin plane.
	dataClear := mintToken(t, st, "root", "", "", 0) // minted with PlaneData
	if _, err := dataM.Resolve(ctx, bearer(dataClear)); err != nil {
		t.Fatalf("data token on the data plane must resolve: %v", err)
	}
	if _, err := adminM.Resolve(ctx, bearer(dataClear)); err == nil {
		t.Fatalf("a data token must NOT open the admin plane")
	}

	// An ADMIN token resolves on the admin plane but not the data plane.
	adminClear := "mk_admin-secret-value"
	if err := st.AddAPIToken(ctx, store.NewToken{
		ID: "atok", UserID: "root", Name: "cli", TokenHash: hashToken(adminClear),
		Prefix: adminClear[:10], Plane: store.PlaneAdmin, Scope: store.ScopeFull,
	}); err != nil {
		t.Fatalf("AddAPIToken(admin): %v", err)
	}
	if _, err := adminM.Resolve(ctx, bearer(adminClear)); err != nil {
		t.Fatalf("admin token on the admin plane must resolve: %v", err)
	}
	if _, err := dataM.Resolve(ctx, bearer(adminClear)); err == nil {
		t.Fatalf("an admin token must NOT open the data plane")
	}
}

func TestBearerTokenExpiredDisabledUserAndPolicy(t *testing.T) {
	m, st := setup(t)
	enabledUser(t, st, "alice")
	ctx := context.Background()

	// Expired.
	expired := mintToken(t, st, "alice", "t1", "", time.Now().Add(-time.Hour).Unix())
	if _, err := m.Resolve(ctx, bearer(expired)); err == nil {
		t.Fatalf("an expired token must not resolve")
	}

	// A live token, then the OWNER is disabled -> tokens stop at once.
	live := mintToken(t, st, "alice", "t2", "", 0) // different group => different id
	if _, err := m.Resolve(ctx, bearer(live)); err != nil {
		t.Fatalf("live token should resolve: %v", err)
	}
	if err := st.UpdateUser(ctx, store.User{ID: "alice", Username: "alice", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	m.UserChanged("alice")
	if _, err := m.Resolve(ctx, bearer(live)); err == nil {
		t.Fatalf("a disabled account's token must not resolve")
	}

	// Policy off -> no token authenticates, even a live one.
	if err := st.UpdateUser(ctx, store.User{ID: "alice", Username: "alice", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	m.UserChanged("alice")
	if _, err := m.Resolve(ctx, bearer(live)); err != nil {
		t.Fatalf("re-enabled account's token should resolve: %v", err)
	}
	if err := st.SetSetting(ctx, store.SettingAPITokens, false); err != nil {
		t.Fatal(err)
	}
	m.TokenChanged("*")
	if _, err := m.Resolve(ctx, bearer(live)); err == nil {
		t.Fatalf("policy off must refuse every token")
	}
}

func TestBearerRejectedOnAdminPlane(t *testing.T) {
	m, st := setup(t, ForAdminPlane())
	enabledUser(t, st, "alice")
	secret := mintToken(t, st, "alice", "t1", "", 0)
	if _, err := m.Resolve(context.Background(), bearer(secret)); err == nil {
		t.Fatalf("the admin plane must never accept a personal API token")
	}
}

// A resolved token is served from memory for the cache window, like a cookie
// session: that is what keeps an API route from paying three queries per
// request. What makes a revocation immediate is therefore TokenChanged, called
// by every write - a write that forgot it would wait out the window, and this
// test is the one that says so.
func TestBearerTokenIsServedFromMemory(t *testing.T) {
	now := time.Now()
	clock := &now
	m, st := setup(t, WithCacheTTL(5*time.Second), WithClock(func() time.Time { return *clock }))
	enabledUser(t, st, "alice")
	secret := mintToken(t, st, "alice", "t1", "", 0)
	ctx := context.Background()

	if _, err := m.Resolve(ctx, bearer(secret)); err != nil {
		t.Fatalf("live token should resolve: %v", err)
	}
	if _, err := st.RevokeAPIToken(ctx, "alice", "tok-alicet1"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Resolve(ctx, bearer(secret)); err != nil {
		t.Fatalf("within the window, without TokenChanged, the remembered token answers: %v", err)
	}
	later := now.Add(6 * time.Second)
	clock = &later
	if _, err := m.Resolve(ctx, bearer(secret)); err == nil {
		t.Fatalf("past the window the database is asked again, and the token is gone")
	}
}

// What depends on the clock and on the request is judged EVERY time, cache or
// not: an expiry passing inside the window, a caller outside the token's
// ranges.
func TestBearerTokenClockAndAddressBeatTheCache(t *testing.T) {
	now := time.Now()
	clock := &now
	m, st := setup(t, WithCacheTTL(time.Hour), WithClock(func() time.Time { return *clock }))
	enabledUser(t, st, "alice")
	ctx := context.Background()

	expiring := mintToken(t, st, "alice", "t1", "", now.Add(time.Minute).Unix())
	if _, err := m.Resolve(ctx, bearer(expiring)); err != nil {
		t.Fatalf("unexpired token should resolve: %v", err)
	}
	later := now.Add(2 * time.Minute)
	clock = &later
	if _, err := m.Resolve(ctx, bearer(expiring)); err == nil {
		t.Fatalf("an expiry reached inside the cache window must still refuse")
	}

	ranged := "mk_ranged-secret-value"
	if err := st.AddAPIToken(ctx, store.NewToken{
		ID: "ranged", UserID: "alice", Name: "ci", TokenHash: hashToken(ranged), Prefix: ranged[:10],
		Plane: store.PlaneData, Scope: store.ScopeFull, FromCIDRs: "192.0.2.0/24",
	}); err != nil {
		t.Fatal(err)
	}
	inside := bearer(ranged) // httptest's peer is 192.0.2.1
	if _, err := m.Resolve(ctx, inside); err != nil {
		t.Fatalf("a caller inside the range should resolve: %v", err)
	}
	outside := bearer(ranged)
	outside.RemoteAddr = "198.51.100.7:4000"
	if _, err := m.Resolve(ctx, outside); err == nil {
		t.Fatalf("a remembered token must still refuse a caller outside its ranges")
	}
}
