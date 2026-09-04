package cluster_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/cluster"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// An operator chooses their organisation on one gateway and is served the
// previous one by the other for the next five seconds. That is a wrong answer,
// not a late one, and it is what an unshared session cache does.
func TestChoosingAnOrganisationReachesTheOtherGateway(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sa.CreateUser(ctx, store.User{ID: "u1", Username: "alice", Enabled: true}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Node B caches; node A writes. Both are wired the way main wires them.
	smA := session.NewManager(sa)
	// A cache that will NOT lapse during the test: the five-second default
	// would let node B converge on its own, and the test would pass with the
	// signal disconnected - which is exactly what it must never do.
	smB := session.NewManager(sb, session.WithCacheTTL(time.Minute))
	busA, busB := cluster.New(sa), cluster.New(sb)
	smA.Notify(func(topic, arg string) { busA.Signal(ctx, topic, arg) })
	busB.OnSignal(store.TopicSession, smB.Forget)
	busB.OnSignal(store.TopicSessionUser, smB.ForgetUser)
	go busB.Run(ctx)
	time.Sleep(500 * time.Millisecond)

	// A session, issued on A and carried by a request each node can read.
	rec := httptest.NewRecorder()
	token, err := smA.Issue(ctx, rec, httptest.NewRequest(http.MethodGet, "/", nil), "u1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	carry := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range rec.Result().Cookies() {
			r.AddCookie(c)
		}
		return r
	}
	_ = token

	// B reads it once, which is what puts it in B's memory.
	if _, err := smB.Resolve(ctx, carry()); err != nil {
		t.Fatalf("node B could not read the session: %v", err)
	}

	// A changes it.
	if err := smA.SetTenant(ctx, carry(), "acme"); err != nil {
		t.Fatalf("set tenant on node A: %v", err)
	}

	// B must see the new organisation without waiting for its cache to lapse.
	deadline := time.Now().Add(5 * time.Second)
	for {
		sess, err := smB.Resolve(ctx, carry())
		if err != nil {
			t.Fatalf("node B: %v", err)
		}
		if sess.TenantID == "acme" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("node B still serves the previous organisation: the caches are not shared")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// A password reset deletes the rows on one node; the other one is still
// serving them from memory, which is the version of this bug that matters.
func TestRevokingAUserReachesTheOtherGateway(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sa.CreateUser(ctx, store.User{ID: "u1", Username: "alice", Enabled: true}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	smA := session.NewManager(sa)
	smB := session.NewManager(sb, session.WithCacheTTL(time.Minute)) // see the note above
	busA, busB := cluster.New(sa), cluster.New(sb)
	smA.Notify(func(topic, arg string) { busA.Signal(ctx, topic, arg) })
	busB.OnSignal(store.TopicSession, smB.Forget)
	busB.OnSignal(store.TopicSessionUser, smB.ForgetUser)
	go busB.Run(ctx)
	time.Sleep(500 * time.Millisecond)

	rec := httptest.NewRecorder()
	if _, err := smA.Issue(ctx, rec, httptest.NewRequest(http.MethodGet, "/", nil), "u1"); err != nil {
		t.Fatalf("issue: %v", err)
	}
	carry := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range rec.Result().Cookies() {
			r.AddCookie(c)
		}
		return r
	}
	if _, err := smB.Resolve(ctx, carry()); err != nil {
		t.Fatalf("node B could not read the session: %v", err)
	}

	if _, err := sa.DeleteSessionsForUser(ctx, "u1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	smA.Revoked("u1")

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := smB.Resolve(ctx, carry()); err != nil {
			return // gone, which is the point
		}
		if time.Now().After(deadline) {
			t.Fatal("node B still serves a revoked session: whoever was signed in keeps their page")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
