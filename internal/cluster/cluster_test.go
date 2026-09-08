package cluster_test

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/cluster"
)

// One gateway must not need a cluster to work: the trunk calls the same
// methods either way, so the quiet bus has to answer all of them and do
// nothing. A nil bus would mean a nil check at every call site, which is how
// one of them ends up missing.
func TestWithNoLoopEverythingStillAnswers(t *testing.T) {
	// No loop is registered in THIS test binary, which is what makes it the
	// right place to check the other side of the seam: the shape a gateway
	// with nobody to tell falls back to.
	if cluster.Available() {
		t.Fatal("a loop registered itself into the trunk's own tests")
	}
	b := cluster.New(nil)
	ctx := context.Background()
	b.Register("routing", func(context.Context) error { return nil })
	b.OnSignal("session", func(string) {})
	b.OnSignalFrom("metrics", func(string, string) {})
	b.Signal(ctx, "session", "hash")
	b.Announce(ctx, "routing")
	b.Run(ctx) // returns at once: there is nobody to listen to

	// It still has an identity. The code that keeps one entry per node reports
	// the local one through the same door, so there is no privileged member
	// and one gateway takes the same path as five.
	if b.Node() == "" {
		t.Error("the quiet bus has no node id")
	}
}
