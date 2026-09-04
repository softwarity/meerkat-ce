package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A test token minted on one gateway has to be readable by the next one.
//
// It was not: the key was drawn at every boot, so behind a load balancer the
// simulator - and the agent's test_routing - worked or failed depending on
// which node answered. The same bug made every restart invalidate a token an
// operator had just copied, which is the version of it a single-node
// installation meets.
func TestATestTokenMintedOnOneGatewayWorksOnTheOther(t *testing.T) {
	url := dbtest.URL(t)
	dir := t.TempDir()

	// Two routers over ONE database. On the embedded one that is two routers
	// over one file, which is exactly the restart case; on PostgreSQL it is
	// two nodes.
	open := func() *Router {
		st, err := store.OpenAt(dir, url)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })
		rt := New(st, session.NewManager(st))
		if err := rt.Reload(context.Background()); err != nil {
			t.Fatalf("reload: %v", err)
		}
		return rt
	}
	a, b := open(), open()

	token, _ := a.MintSimulationToken("alice", []string{"ops"}, time.Minute)
	id, ok := b.verifySimulationToken(token)
	if !ok {
		t.Fatal("the second gateway rejected a token the first one minted: " +
			"behind a load balancer the simulator would work one request in two")
	}
	if id.Username != "alice" || len(id.Roles) != 1 || id.Roles[0] != "ops" {
		t.Fatalf("the identity did not survive the trip: %+v", id)
	}
}

// And a router that never reloaded still signs with something of its own,
// rather than with nothing.
func TestARouterWithNoStoredKeyStillSigns(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rt := New(st, session.NewManager(st))

	token, _ := rt.MintSimulationToken("bob", nil, time.Minute)
	if _, ok := rt.verifySimulationToken(token); !ok {
		t.Fatal("a router that has not reloaded could not read its own token")
	}
}
