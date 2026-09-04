package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

func setup(t *testing.T, opts ...Option) (*Manager, *store.Store) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.CreateUser(context.Background(), store.User{ID: "u1", Username: "admin", PasswordHash: "x", Root: true}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return NewManager(st, opts...), st
}

func issue(t *testing.T, m *Manager) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", nil)
	if _, err := m.Issue(context.Background(), rec, req, "u1"); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// Two cookies now: the opaque session, and the readable deadline a page
	// needs to know when it is over without asking.
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != CookieName || cookies[1].Name != UntilCookieName {
		t.Fatalf("expected the session cookie and its deadline, got %+v", cookies)
	}
	if cookies[1].HttpOnly {
		t.Fatalf("the deadline must be readable by the page: %+v", cookies[1])
	}
	return cookies[0]
}

func requestWith(c *http.Cookie) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	if c != nil {
		req.AddCookie(c)
	}
	return req
}

func TestIssueResolveRoundTrip(t *testing.T) {
	m, _ := setup(t)
	c := issue(t, m)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags wrong: %+v", c)
	}
	sess, err := m.Resolve(context.Background(), requestWith(c))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if sess.UserID != "u1" {
		t.Fatalf("UserID = %q", sess.UserID)
	}
}

func TestResolveWithoutCookie(t *testing.T) {
	m, _ := setup(t)
	if _, err := m.Resolve(context.Background(), requestWith(nil)); !errors.Is(err, ErrNoSession) {
		t.Fatalf("want ErrNoSession, got %v", err)
	}
}

func TestResolveGarbageToken(t *testing.T) {
	m, _ := setup(t)
	c := &http.Cookie{Name: CookieName, Value: "forged"}
	if _, err := m.Resolve(context.Background(), requestWith(c)); !errors.Is(err, ErrNoSession) {
		t.Fatalf("want ErrNoSession, got %v", err)
	}
}

func TestExpiryBeatsCache(t *testing.T) {
	now := time.Now()
	clock := &now
	m, _ := setup(t,
		WithTTL(10*time.Minute),
		WithCacheTTL(time.Hour), // cache would happily serve stale - expiry must win
		WithClock(func() time.Time { return *clock }),
	)
	c := issue(t, m)
	if _, err := m.Resolve(context.Background(), requestWith(c)); err != nil {
		t.Fatalf("fresh session rejected: %v", err)
	}
	later := now.Add(11 * time.Minute)
	clock = &later
	if _, err := m.Resolve(context.Background(), requestWith(c)); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expired session served: %v", err)
	}
}

func TestDestroyRevokesImmediately(t *testing.T) {
	m, _ := setup(t, WithCacheTTL(time.Hour))
	c := issue(t, m)
	if _, err := m.Resolve(context.Background(), requestWith(c)); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	rec := httptest.NewRecorder()
	if err := m.Destroy(context.Background(), rec, requestWith(c)); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	// Despite the 1h cache window, the eviction makes revocation immediate.
	if _, err := m.Resolve(context.Background(), requestWith(c)); !errors.Is(err, ErrNoSession) {
		t.Fatalf("revoked session still served: %v", err)
	}
	// And BOTH cookies are cleared - a deadline left behind would tell every
	// open page that a session it no longer has is good for another half hour.
	cleared := rec.Result().Cookies()
	if len(cleared) != 2 {
		t.Fatalf("expected both cookies cleared: %+v", cleared)
	}
	for _, c := range cleared {
		if c.MaxAge != -1 {
			t.Fatalf("cookie not cleared: %+v", c)
		}
	}
}

// The lifetime measures INACTIVITY: a request pushes the deadline, so someone
// who works is never signed out, and someone who walks away is. Written in the
// second half only - a write per request would take the store's write lock on
// the hot path to gain a few seconds.
func TestSessionSlidesOnUse(t *testing.T) {
	now := time.Now()
	clock := &now
	m, st := setup(t, WithTTL(30*time.Minute), WithCacheTTL(0),
		WithClock(func() time.Time { return *clock }))
	c := issue(t, m)

	call := func() []*http.Cookie {
		rec := httptest.NewRecorder()
		m.Sliding(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
			ServeHTTP(rec, requestWith(c))
		return rec.Result().Cookies()
	}
	sess, err := m.Resolve(context.Background(), requestWith(c))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	issued := sess.ExpiresAt

	// First half: nothing is written and nothing is said.
	at10 := now.Add(10 * time.Minute)
	clock = &at10
	if got := call(); len(got) != 0 {
		t.Fatalf("the first half wrote cookies: %+v", got)
	}
	if s, _ := st.GetSession(context.Background(), sess.TokenHash); s.ExpiresAt != issued {
		t.Fatalf("the first half moved the deadline: %d", s.ExpiresAt)
	}

	// Past halfway, the deadline moves and both cookies follow the row.
	at20 := now.Add(20 * time.Minute)
	clock = &at20
	got := call()
	if len(got) != 2 {
		t.Fatalf("the extension did not reach the browser: %+v", got)
	}
	want := at20.Add(30 * time.Minute).Unix()
	if s, _ := st.GetSession(context.Background(), sess.TokenHash); s.ExpiresAt != want {
		t.Fatalf("deadline = %d, want %d", s.ExpiresAt, want)
	}

	// And it really is an idle timeout: 40 minutes after signing in, a session
	// used all along is still live where a fixed window would have ended it.
	at40 := now.Add(40 * time.Minute)
	clock = &at40
	if _, err := m.Resolve(context.Background(), requestWith(c)); err != nil {
		t.Fatalf("a session in continuous use was ended: %v", err)
	}

	// Left alone past its lifetime, it ends.
	at90 := now.Add(90 * time.Minute)
	clock = &at90
	if _, err := m.Resolve(context.Background(), requestWith(c)); !errors.Is(err, ErrNoSession) {
		t.Fatalf("an idle session survived: %v", err)
	}
}

func TestPurgeExpired(t *testing.T) {
	now := time.Now()
	clock := &now
	m, st := setup(t, WithTTL(time.Minute), WithClock(func() time.Time { return *clock }))
	_ = issue(t, m)
	later := now.Add(2 * time.Minute)
	clock = &later
	n, err := m.PurgeExpired(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("PurgeExpired = %d, %v", n, err)
	}
	if n, _ := st.PurgeExpiredSessions(context.Background(), later.Unix()); n != 0 {
		t.Fatalf("second purge removed %d", n)
	}
}
