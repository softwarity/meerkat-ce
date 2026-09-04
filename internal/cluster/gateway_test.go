package cluster_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/cluster"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The whole point, end to end: a route saved on one gateway is SERVED by the
// other one, without a restart and without anybody touching it.
//
// The tests above prove the bus delivers; this proves the thing it delivers is
// the product. It goes through the real Router.Reload, over a real HTTP
// listener, and asks the second node the only question an operator ever asks:
// does this URL work.
func TestARouteSavedOnOneGatewayIsServedByTheOther(t *testing.T) {
	sa, sb := twoNodes(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// What the routes point at. Nothing clever: it answers, and that is how we
	// know the request went all the way through.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("served"))
	}))
	defer upstream.Close()

	// Node B: a gateway that has loaded its plan and is serving. It never
	// takes the write.
	routerB := gateway.New(sb, session.NewManager(sb))
	if err := routerB.Reload(ctx); err != nil {
		t.Fatalf("node B initial load: %v", err)
	}
	nodeB := httptest.NewServer(routerB)
	defer nodeB.Close()

	busB := cluster.New(sb)
	busB.Register(store.TopicRouting, routerB.Reload)
	go busB.Run(ctx)

	// Node A takes the write, reloads itself, and announces - which is exactly
	// what internal/admin's reloadRouting does on a saved route.
	busA := cluster.New(sa)
	route := store.Route{
		ID: "r1", Name: "probe", Order: 1, Enabled: true, Upstream: upstream.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": "/probe"}}},
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := sa.SaveRoute(ctx, route); err != nil {
			t.Fatalf("save on node A: %v", err)
		}
		busA.Announce(ctx, store.TopicRouting)

		res, err := http.Get(nodeB.URL + "/probe")
		if err != nil {
			t.Fatalf("asking node B: %v", err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode == http.StatusOK && string(body) == "served" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("node B still answers %d %q for a route node A saved: the second gateway "+
				"is serving the previous plan, which is the bug this package exists to close",
				res.StatusCode, body)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
