package live

import (
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// The version of a row is what tells the client "this changed". It used to be
// updated_at alone, and updated_at is a SECOND: a call to a service that
// answers in milliseconds is claimed and closed inside the same one, so the
// two states carried the same version and the end had nothing to announce. A
// screen that caught the run in between kept "running" until somebody pressed
// reload - which is exactly how this was found.
func TestARunClaimedAndClosedInOneSecondIsStillTwoVersions(t *testing.T) {
	const second = 1789999188
	running := store.Schedule{
		ID: "s1", UpdatedAt: second,
		RunID: "GvbDqbNG", ClaimedBy: "node-a", RunState: store.RunCalling, Attempts: 1,
	}
	done := store.Schedule{
		ID: "s1", UpdatedAt: second,
		LastState: store.RunDone, LastAt: second,
	}
	if rowVersion(running) == rowVersion(done) {
		t.Fatalf("a run that started and ended in the same second looks unchanged: %q", rowVersion(running))
	}
}

// And progress moves within one run, on a job long enough to report: the same
// second must not swallow that either.
func TestProgressMovesTheVersion(t *testing.T) {
	at := store.Schedule{ID: "s1", UpdatedAt: 1789999188, RunID: "GvbDqbNG", Progress: 10}
	on := at
	on.Progress = 60
	if rowVersion(at) == rowVersion(on) {
		t.Fatal("progress moved and the row says nothing changed")
	}
}

// A row nobody touched keeps its version: that is what stops a screen watching
// fifty schedules from receiving fifty rows every tick. Read twice, from two
// copies, because a listing builds a fresh struct on every pass.
func TestAnUntouchedRowKeepsItsVersion(t *testing.T) {
	sc := store.Schedule{ID: "s1", UpdatedAt: 1789999188, LastState: store.RunDone, LastAt: 1789999188}
	again := sc
	if rowVersion(sc) != rowVersion(again) {
		t.Fatalf("the same row has two versions: %q then %q", rowVersion(sc), rowVersion(again))
	}
}

// The 202 lands in the same second as the claim more often than not: the
// screen still has to learn the service took the work. Same for a call sent
// again, which is the one an operator most wants to see.
func TestAcceptanceAndAnotherAttemptMoveTheVersion(t *testing.T) {
	calling := store.Schedule{ID: "s1", UpdatedAt: 1789999188, RunID: "GvbDqbNG",
		RunState: store.RunCalling, Attempts: 1}
	accepted := calling
	accepted.RunState = store.RunAccepted
	if rowVersion(calling) == rowVersion(accepted) {
		t.Fatal("the service took the work and the row says nothing changed")
	}
	again := calling
	again.Attempts = 2
	if rowVersion(calling) == rowVersion(again) {
		t.Fatal("the call was sent again and the row says nothing changed")
	}
}
