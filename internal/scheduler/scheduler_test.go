package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"github.com/softwarity/meerkat/internal/vault"
)

// plane stands in for the data plane: it records what arrived and answers
// what the test asked it to.
type plane struct {
	status  int32
	calls   atomic.Int32
	lastRun atomic.Value // string
	lastCtx atomic.Value // *http.Request
}

func (p *plane) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.calls.Add(1)
	p.lastRun.Store(r.Header.Get(RunHeader))
	p.lastCtx.Store(r)
	w.WriteHeader(int(atomic.LoadInt32(&p.status)))
}

func setup(t *testing.T, p http.Handler) (*Scheduler, *store.Store, context.Context) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "svc", Enabled: true}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	s := New(Deps{Store: st, Plane: p, Node: "node-a", Lease: time.Minute})
	return s, st, ctx
}

// pass is one round, and the calls it started: the loop never waits for
// them, a test has to.
func pass(ctx context.Context, t *testing.T, s *Scheduler) {
	t.Helper()
	if err := s.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	s.settle()
}

func due(id string) store.Schedule {
	return store.Schedule{
		ID: id, Name: "nightly close", Roles: []string{"ops"}, RouteID: "r1",
		Method: "POST", Path: "/jobs/close", Every: "PT1H", Overlap: store.OverlapSkip,
		NextAt: time.Now().Add(-time.Minute).Unix(),
	}
}

// The whole point, end to end: a turn is owed, the call is made through the
// plane, and the run closes with what came back.
func TestAnOwedTurnIsCalled(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	if p.calls.Load() != 1 {
		t.Fatalf("the call was not made: %d", p.calls.Load())
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != store.RunDone {
		t.Fatalf("the run did not close as done: %+v", got)
	}
	if got.RunID != "" {
		t.Fatalf("the run stayed open on a 200: %+v", got)
	}
	if got.NextAt <= time.Now().Unix() {
		t.Fatalf("the next turn was not armed: %d", got.NextAt)
	}
	// The at-least-once contract: the service is given something to dedupe on.
	if run, _ := p.lastRun.Load().(string); run == "" {
		t.Fatalf("the call carried no run identifier")
	}
}

// A scheduled call carries no credential at all, and that is the design: the
// identity is posed in the CONTEXT of an in-process request, so it never
// travels on a wire and nothing has to be minted, stored or deleted. What the
// service sees is a caller named meerkat with the roles the schedule asked for
// - which this test cannot observe through a fake plane, and which
// internal/gateway proves on the real one.
func TestTheCallMintsNothing(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	r, _ := p.lastCtx.Load().(*http.Request)
	if r == nil {
		t.Fatal("no call was made")
	}
	if got := r.Header.Get("Authorization"); got != "" {
		t.Fatalf("a scheduled call carried a credential: %q", got)
	}
	tokens, err := st.ListAPITokens(ctx, "u1", store.PlaneData)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 0 {
		t.Fatalf("a call minted a token: %+v", tokens)
	}
}

// A failure is recorded as one, with enough to recognise it.
func TestAFailedCallIsRecorded(t *testing.T) {
	p := &plane{status: http.StatusForbidden}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != store.RunFailed {
		t.Fatalf("a 403 was not recorded as a failure: %+v", got)
	}
	if got.NextAt <= time.Now().Unix() {
		t.Fatalf("a failure must still arm the next turn: %d", got.NextAt)
	}
}

// 202 is the long-job contract: the run stays open and the service reports.
func TestAnAcceptedCallKeepsTheRunOpen(t *testing.T) {
	p := &plane{status: http.StatusAccepted}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID == "" {
		t.Fatalf("202 closed the run: %+v", got)
	}
	if got.RunState != store.RunAccepted {
		t.Fatalf("the run is not marked accepted: %+v", got)
	}
	// A run in flight does not overwrite how the LAST one ended: the screen
	// shows both, and a red "running" in the last-run column was the result.
	if got.LastState != "" {
		t.Fatalf("the run in flight wrote over the last result: %+v", got)
	}
}

// While a run is in flight nothing more is owed, however many passes go by:
// the next turn is armed when it closes, counted from then - which is what
// keeps a slow job from finding its next turn waiting in the past.
func TestNothingIsOwedWhileARunIsInFlight(t *testing.T) {
	p := &plane{status: http.StatusAccepted}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s) // leaves the run open
	pass(ctx, t, s)
	if p.calls.Load() != 1 {
		t.Fatalf("a second call went out while the first was in flight: %d calls", p.calls.Load())
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if err := st.FinishSchedule(ctx, "s1", got.RunID, store.RunEnd{State: store.RunDone, Detail: "done", Status: 200}, store.RunNext{At: s.next(got, time.Now())}, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetSchedule(ctx, "s1")
	if got.NextAt <= time.Now().Unix() {
		t.Fatalf("the close did not arm the next turn from its own end: %d", got.NextAt)
	}
}

// A job the service took (202) and then went quiet on is LOST, not sent
// again: the service has it, and a second call would only be ignored.
func TestAnAcceptedRunThatGoesQuietIsLost(t *testing.T) {
	p := &plane{status: http.StatusAccepted}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	// Nothing renews the lease: the service never reports.
	s.d.Now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID != "" || got.LastState != store.RunLost {
		t.Fatalf("the quiet run was not closed as lost: %+v", got)
	}
	if !strings.Contains(got.LastDetail, "202") {
		t.Fatalf("the detail does not say the service took it: %q", got.LastDetail)
	}
	if p.calls.Load() != 1 {
		t.Fatalf("an accepted run was sent again: %d calls", p.calls.Load())
	}
}

// claimedElsewhere is a call another node made and never saw answered: the
// row as that node left it when it stopped.
func claimedElsewhere(ctx context.Context, t *testing.T, st *store.Store, sc store.Schedule, runID string, at time.Time) {
	t.Helper()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.ClaimSchedule(ctx, sc.ID, runID, "node-b", at); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
}

// The at-least-once contract, the half that needs a cluster: a node stopped
// in the middle of a call, and another sends it AGAIN - same run identifier,
// so a service that did get the first one ignores the second.
func TestACallCutByAStoppedNodeIsSentAgainAsTheSameRun(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	sc := due("s1")
	sc.NextAt = time.Now().Add(-5 * time.Minute).Unix()
	claimedElsewhere(ctx, t, st, sc, "run-from-b", time.Now().Add(-2*time.Minute)) // lease: a minute
	pass(ctx, t, s)
	if p.calls.Load() != 1 {
		t.Fatalf("the cut call was not sent again: %d calls", p.calls.Load())
	}
	r, _ := p.lastCtx.Load().(*http.Request)
	if got := r.Header.Get(RunHeader); got != "run-from-b" {
		t.Fatalf("sent again under another run id: %q", got)
	}
	if got := r.Header.Get(AttemptHeader); got != "2" {
		t.Fatalf("the service is not told this is a second attempt: %q", got)
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID != "" || got.LastState != store.RunDone || got.Attempts != 2 {
		t.Fatalf("the second attempt did not close the run: %+v", got)
	}
	if !strings.Contains(got.LastDetail, "attempt 2") {
		t.Fatalf("the result does not say which attempt made it: %q", got.LastDetail)
	}
}

// A first call says so too: a service can log the header without guessing.
func TestAFirstCallIsTheFirstAttempt(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	r, _ := p.lastCtx.Load().(*http.Request)
	if got := r.Header.Get(AttemptHeader); got != "1" {
		t.Fatalf("attempt header on a first call: %q", got)
	}
}

// A call that brings a node down would bring every node down in turn. So a
// run is given up after MaxAttempts, not sent a fourth time.
func TestACallIsGivenUpAfterItsLastAttempt(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	sc := due("s1")
	sc.NextAt = time.Now().Add(-time.Hour).Unix()
	start := time.Now().Add(-50 * time.Minute)
	claimedElsewhere(ctx, t, st, sc, "run-x", start)
	for i := 1; i < MaxAttempts; i++ { // each node that took it back stopped as well
		at := start.Add(time.Duration(i) * 10 * time.Minute)
		if ok, err := st.RetakeRun(ctx, "s1", "run-x", "node-c", at, time.Minute); err != nil || !ok {
			t.Fatalf("attempt %d: %v %v", i+1, ok, err)
		}
	}
	pass(ctx, t, s)
	if p.calls.Load() != 0 {
		t.Fatalf("a run was sent past its last attempt: %d calls", p.calls.Load())
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID != "" || got.LastState != store.RunLost {
		t.Fatalf("the run was not given up: %+v", got)
	}
	if got.NextAt <= time.Now().Unix() {
		t.Fatalf("giving up must still arm the next turn: %d", got.NextAt)
	}
}

// Sent again only within the catch-up: a call that is only worth making on
// time is not made twenty minutes late because a node died.
func TestACutCallLaterThanItsCatchUpIsLost(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	sc := due("s1")
	sc.NextAt, sc.CatchUp = time.Now().Add(-20*time.Minute).Unix(), 60
	claimedElsewhere(ctx, t, st, sc, "run-x", time.Now().Add(-20*time.Minute))
	pass(ctx, t, s)
	if p.calls.Load() != 0 {
		t.Fatalf("a late call was sent again: %d calls", p.calls.Load())
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != store.RunLost || !strings.Contains(got.LastDetail, "catch-up") {
		t.Fatalf("not closed for being late: %+v", got)
	}
}

// A node that stops cleanly hands its call back at once, and another node
// sends it again at once - not in five minutes, when the lease would have.
func TestAStoppingNodeHandsItsCallBack(t *testing.T) {
	release := make(chan struct{})
	var firstRun atomic.Value
	hang := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstRun.Store(r.Header.Get(RunHeader))
		select {
		case <-r.Context().Done():
		case <-release:
		}
		w.WriteHeader(http.StatusBadGateway)
	})
	s, st, ctx := setup(t, hang)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Pass(ctx); err != nil { // the call is now in flight, and stays
		t.Fatal(err)
	}
	var announced atomic.Int32
	s.d.Announce = func(context.Context) { announced.Add(1) }
	drainCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	s.Drain(drainCtx)
	close(release)

	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID == "" || got.ClaimedAt != 0 || got.RunState != store.RunCalling {
		t.Fatalf("the cut call was not handed back as the same run: %+v", got)
	}
	if announced.Load() != 1 {
		t.Fatalf("the other nodes were not told: %d announcements", announced.Load())
	}
	// Another node, straight away.
	p := &plane{status: http.StatusOK}
	other := New(Deps{Store: st, Plane: p, Node: "node-b", Lease: time.Minute})
	pass(ctx, t, other)
	if p.calls.Load() != 1 {
		t.Fatalf("the handed-back call was not sent again at once: %d calls", p.calls.Load())
	}
	was, _ := firstRun.Load().(string)
	if r, _ := p.lastCtx.Load().(*http.Request); r.Header.Get(RunHeader) != was {
		t.Fatalf("sent again as another run: %q, first was %q", r.Header.Get(RunHeader), was)
	}
}

// The calls are made outside any lock, several at a time: a service that
// takes its time holds its own call and nobody else's.
func TestASlowCallHoldsNobodyElse(t *testing.T) {
	release := make(chan struct{})
	var fast atomic.Int32
	s, st, ctx := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			<-release
		} else {
			fast.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	slow, quick := due("slow"), due("quick")
	slow.Path, quick.Path = "/slow", "/quick"
	slow.NextAt = time.Now().Add(-2 * time.Minute).Unix() // listed first
	for _, sc := range []store.Schedule{slow, quick} {
		if err := st.CreateSchedule(ctx, sc); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, _ := st.GetSchedule(ctx, "quick")
		if got.LastState == store.RunDone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the quick call waited for the slow one: %+v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	s.settle()
}

// A node with every slot busy leaves the rest OWED rather than queueing it:
// another node takes it, or this one when a call comes back.
func TestAFullNodeLeavesTheRestOwed(t *testing.T) {
	release := make(chan struct{})
	s, st, ctx := setup(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	s.slots = make(chan struct{}, 1)
	for _, id := range []string{"a", "b"} {
		if err := st.CreateSchedule(ctx, due(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	owed, err := st.DueSchedules(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(owed) != 1 {
		t.Fatalf("one slot, two turns: one should be left owed, got %d", len(owed))
	}
	close(release)
	s.settle()
}

// The loop sleeps until the next thing owed, within its bounds.
func TestItSleepsUntilTheNextThingOwed(t *testing.T) {
	s, st, ctx := setup(t, &plane{status: http.StatusOK})
	if got := s.sleep(ctx); got != s.d.Backstop {
		t.Fatalf("nothing scheduled should sleep the backstop: %s", got)
	}
	sc := due("s1")
	sc.NextAt = time.Now().Add(30 * time.Second).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	if got := s.sleep(ctx); got < 28*time.Second || got > 31*time.Second {
		t.Fatalf("a turn in thirty seconds should sleep about that: %s", got)
	}
	past := due("s2")
	if err := st.CreateSchedule(ctx, past); err != nil {
		t.Fatal(err)
	}
	if got := s.sleep(ctx); got != minPause {
		t.Fatalf("a turn already owed should wait no more than the floor: %s", got)
	}
}

// The lease has to outlast the longest wait a call is allowed, or a call
// still waiting for its answer would be taken for a dead node's and sent
// twice.
func TestTheLeaseOutlastsTheLongestCall(t *testing.T) {
	if DefaultLease <= store.MaxScheduleTimeout {
		t.Fatalf("a lease of %s is shorter than a call may wait (%s)", DefaultLease, store.MaxScheduleTimeout)
	}
}

// A paused schedule is not called, whatever its turn says.
func TestAPausedScheduleIsNotCalled(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	sc := due("s1")
	sc.Paused = true
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	if p.calls.Load() != 0 {
		t.Fatalf("a paused schedule was called")
	}
}

// A service that is down. The gateway answers its own error PAGE, and what
// gets recorded must be the sentence a human reads on it, not its doctype.
func TestADeadServiceRecordsWhatThePageSays(t *testing.T) {
	page := `<!doctype html><html><head><style>b{color:red}</style></head>` +
		"<body><h1>Unavailable</h1>\n<p>This application is not responding</p></body></html>"
	s, st, ctx := setup(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(page))
	}))
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != store.RunFailed {
		t.Fatalf("a dead service is a failed run: %+v", got)
	}
	if strings.ContainsAny(got.LastDetail, "<>") || strings.Contains(got.LastDetail, "color:red") {
		t.Fatalf("the markup was kept: %q", got.LastDetail)
	}
	for _, want := range []string{"502", "Unavailable", "This application is not responding"} {
		if !strings.Contains(got.LastDetail, want) {
			t.Fatalf("the detail does not say %q: %q", want, got.LastDetail)
		}
	}
	if got.NextAt <= time.Now().Unix() {
		t.Fatalf("a dead service must not stop the schedule: next turn %d", got.NextAt)
	}
	if got.RunID != "" {
		t.Fatalf("the run stayed open on a dead service: %+v", got)
	}
}

// A call that outlasts the time the schedule allows. The plane turns the
// cancelled request into a 502 like any other; the run must say WE stopped
// waiting rather than blame the service's page for it.
func TestACallThatOutlastsItsTimeoutSaysSo(t *testing.T) {
	s, st, ctx := setup(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusBadGateway)
	}))
	sc := due("s1")
	sc.Timeout = "PT1S"
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != store.RunFailed {
		t.Fatalf("a call that never answered is a failure: %+v", got)
	}
	if !strings.Contains(got.LastDetail, "no answer within") {
		t.Fatalf("the detail does not name the timeout: %q", got.LastDetail)
	}
}

// A service's OWN key reaches it through a header, and the row never holds it:
// the value is a vault reference, resolved at the moment of the call. This is
// the answer to "my endpoint is protected by something that is not the
// gateway" - the secret lives in the vault, not in the schedule.
func TestAHeaderCarriesAVaultSecretWithoutStoringIt(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	if err := st.SaveVaultEntry(ctx, vault.Entry{
		Name: "stations-key", Kind: vault.KindSecret, Scope: vault.ScopeApp, Value: "s3cr3t-value",
	}); err != nil {
		t.Fatalf("writing a vault entry: %v", err)
	}
	sc := due("s1")
	sc.Headers = map[string]string{"X-Api-Key": "${stations-key}", "X-Action": "fetch"}
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	r, _ := p.lastCtx.Load().(*http.Request)
	if r == nil {
		t.Fatal("no call was made")
	}
	if got := r.Header.Get("X-Api-Key"); got != "s3cr3t-value" {
		t.Fatalf("the vault reference was not resolved on the wire: %q", got)
	}
	if got := r.Header.Get("X-Action"); got != "fetch" {
		t.Fatalf("a plain header did not survive: %q", got)
	}
	// And what is STORED is still the reference, not the secret.
	back, err := st.GetSchedule(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if back.Headers["X-Api-Key"] != "${stations-key}" {
		t.Fatalf("the secret was written into the schedule: %q", back.Headers["X-Api-Key"])
	}
}

// A body written as JSON goes out as JSON; a body written as a string goes out
// as its content, which is how anything that is not JSON is expressed.
func TestTheBodyGoesOutAsItWasWritten(t *testing.T) {
	for _, c := range []struct{ name, body, want, kind string }{
		{"an object", `{"station":"42"}`, `{"station":"42"}`, "application/json"},
		{"a string", `"station=42"`, `station=42`, "text/plain; charset=utf-8"},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := due("s1")
			sc.Body = []byte(c.body)
			got, kind := requestBody(sc)
			if string(got) != c.want {
				t.Errorf("on the wire: %q, want %q", got, c.want)
			}
			if kind != c.kind {
				t.Errorf("content type: %q, want %q", kind, c.kind)
			}
		})
	}
}

// Two nodes on one table, looking at the same instant, with no lock between
// them: every turn is called exactly once, and the calls spread over both.
func TestTwoNodesShareTheTurnsWithoutALock(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	by := map[string]int{}
	record := func(node string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(20 * time.Millisecond) // long enough for the other node to be looking too
			mu.Lock()
			seen[r.Header.Get(JobHeader)]++
			by[node]++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		})
	}
	a, st, ctx := setup(t, record("a"))
	b := New(Deps{Store: st, Plane: record("b"), Node: "node-b", Lease: time.Minute})
	const turns = 12
	for i := range turns {
		if err := st.CreateSchedule(ctx, due(fmt.Sprintf("s%02d", i))); err != nil {
			t.Fatal(err)
		}
	}
	// Two slots each, so neither node can take everything in one pass: the
	// rest stays owed for whoever looks next, as it would in production.
	a.slots, b.slots = make(chan struct{}, 2), make(chan struct{}, 2)
	var wg sync.WaitGroup
	for _, s := range []*Scheduler{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range turns {
				if err := s.Pass(ctx); err != nil {
					t.Error(err)
				}
				s.settle()
			}
		}()
	}
	wg.Wait()
	if len(seen) != turns {
		t.Fatalf("%d turns called out of %d", len(seen), turns)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("%s was called %d times", id, n)
		}
	}
	if by["a"] == 0 || by["b"] == 0 {
		t.Errorf("the calls did not spread over the nodes: %v", by)
	}
}

// A delayed action (SCHED-02): the call goes out at its date, once, and the
// schedule is then finished rather than armed again.
func TestADelayedActionGoesOutOnceAndIsFinished(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	sc := due("s1")
	sc.Every, sc.At = "", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	sc.NextAt = sc.FirstTurn(time.Now())
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	if p.calls.Load() != 1 {
		t.Fatalf("the delayed action was not called: %d", p.calls.Load())
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != store.RunDone {
		t.Fatalf("the run did not close: %+v", got)
	}
	if got.NextAt != 0 || !got.Finished() {
		t.Fatalf("a single date armed another turn: %+v", got)
	}
	// And it stays finished: the passes that follow owe nothing.
	pass(ctx, t, s)
	if p.calls.Load() != 1 {
		t.Fatalf("a finished schedule was called again: %d", p.calls.Load())
	}
}

// A delayed action nobody was up for is dropped rather than fired hours late,
// and the row says so - it is what an operator looks for the next morning,
// and what they can then replay.
func TestADelayedActionPastItsCatchUpIsDroppedAndSaysSo(t *testing.T) {
	p := &plane{status: http.StatusOK}
	s, st, ctx := setup(t, p)
	sc := due("s1")
	sc.Every, sc.At = "", time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339)
	sc.CatchUp = 60
	sc.NextAt = sc.FirstTurn(time.Now())
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	if p.calls.Load() != 0 {
		t.Fatalf("a call went out two hours late: %d", p.calls.Load())
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != store.RunDropped || !strings.Contains(got.LastDetail, "late") {
		t.Fatalf("the dropped turn is not recorded: %+v", got)
	}
	if got.NextAt != 0 || !got.Finished() {
		t.Fatalf("a dropped single date is still owed: %+v", got)
	}
}

// What the scheduler does lands in the history (SCHED-03), with what
// answered - and a call sent again after a gateway stopped says which
// attempt ended the turn.
func TestWhatTheSchedulerDoesIsKept(t *testing.T) {
	p := &plane{status: http.StatusForbidden}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	runs, err := st.ListScheduleRuns(ctx, "s1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != http.StatusForbidden || runs[0].State != store.RunFailed {
		t.Fatalf("the failure was not kept as one: %+v", runs)
	}
	if runs[0].Node != "node-a" || runs[0].Attempt != 1 {
		t.Fatalf("the history lost who made the call: %+v", runs[0])
	}

	// A call another gateway was making when it stopped: this one sends it
	// again, and the turn ends on its second attempt.
	atomic.StoreInt32(&p.status, http.StatusOK)
	sc := due("s2")
	sc.NextAt = time.Now().Add(-5 * time.Minute).Unix()
	claimedElsewhere(ctx, t, st, sc, "run-from-b", time.Now().Add(-2*time.Minute))
	pass(ctx, t, s)
	runs, _ = st.ListScheduleRuns(ctx, "s2", 0)
	if len(runs) != 1 || runs[0].Attempt != 2 || runs[0].RunID != "run-from-b" {
		t.Fatalf("the second attempt is not in the history: %+v", runs)
	}
}

// A replay is a new run of the same turn, and the history says which one it
// continues - which is what makes the chain readable on a screen.
func TestAReplayNamesTheRunItContinues(t *testing.T) {
	p := &plane{status: http.StatusBadGateway}
	s, st, ctx := setup(t, p)
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	first, _ := st.ListScheduleRuns(ctx, "s1", 0)
	if len(first) != 1 {
		t.Fatalf("no first run: %+v", first)
	}
	atomic.StoreInt32(&p.status, http.StatusOK)
	if ok, err := st.ArmSchedule(ctx, "s1", time.Now(), store.CauseReplay, first[0].ID); err != nil || !ok {
		t.Fatalf("arm the replay: %v %v", ok, err)
	}
	pass(ctx, t, s)
	runs, _ := st.ListScheduleRuns(ctx, "s1", 0)
	if len(runs) != 2 {
		t.Fatalf("the replay did not run: %+v", runs)
	}
	if runs[0].Cause != store.CauseReplay || runs[0].OfRun != first[0].ID {
		t.Fatalf("the replay does not name what it continues: %+v", runs[0])
	}
	if runs[0].RunID == first[0].RunID {
		t.Fatal("a replay reused the run identifier of what it replays")
	}
	if runs[0].State != store.RunDone {
		t.Fatalf("the replay did not close: %+v", runs[0])
	}
}

// clock is a time a test moves by hand: the retries wait, and a test that
// waited with them would take minutes.
type clock struct{ at atomic.Int64 }

func (c *clock) now() time.Time      { return time.Unix(c.at.Load(), 0) }
func (c *clock) set(t time.Time)     { c.at.Store(t.Unix()) }
func (c *clock) add(d time.Duration) { c.at.Store(c.at.Load() + int64(d/time.Second)) }

// A 403 behind a schedule is usually a moment, not a verdict: the service
// restarted and has not loaded its roles yet. So the turn is tried again, as
// a NEW run of the same turn - a service that deduplicates would throw away a
// call it never acted on.
func TestAFailureWorthTryingAgainIsTriedAgain(t *testing.T) {
	p := &plane{status: http.StatusForbidden}
	s, st, ctx := setup(t, p)
	c := &clock{}
	c.set(time.Now())
	s.d.Now = c.now
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunCause != store.CauseRetry || got.TurnTry != 1 {
		t.Fatalf("the turn was not armed for another attempt: %+v", got)
	}
	if got.NextAt != c.now().Add(30*time.Second).Unix() {
		t.Fatalf("the next attempt is at %d, want %d", got.NextAt, c.now().Add(30*time.Second).Unix())
	}

	// Nothing goes out before then.
	c.add(10 * time.Second)
	pass(ctx, t, s)
	if p.calls.Load() != 1 {
		t.Fatalf("the retry did not wait: %d calls", p.calls.Load())
	}

	// And when it does, it is attempt two of the same turn, under its own run
	// identifier, and the history links it to the one it follows.
	atomic.StoreInt32(&p.status, http.StatusOK)
	c.add(25 * time.Second)
	pass(ctx, t, s)
	if p.calls.Load() != 2 {
		t.Fatalf("the retry was not made: %d calls", p.calls.Load())
	}
	runs, _ := st.ListScheduleRuns(ctx, "s1", 0)
	if len(runs) != 2 {
		t.Fatalf("the history is wrong: %+v", runs)
	}
	if runs[0].Attempt != 2 || runs[0].Cause != store.CauseRetry || runs[0].OfRun != runs[1].ID {
		t.Fatalf("the second attempt does not follow the first: %+v", runs[0])
	}
	if runs[0].RunID == runs[1].RunID {
		t.Fatal("a retry reused the run identifier of a call the service answered")
	}
	after, _ := st.GetSchedule(ctx, "s1")
	if after.RunCause != "" || after.TurnTry != 0 || after.LastState != store.RunDone {
		t.Fatalf("the turn did not settle: %+v", after)
	}
}

// A 500 is the service SAYING something: a bug at the far end, handled at the
// far end. The next turn is the retry, at the schedule's own cadence.
func TestAFailureTheServiceOwnsWaitsForTheNextTurn(t *testing.T) {
	p := &plane{status: http.StatusInternalServerError}
	s, st, ctx := setup(t, p)
	c := &clock{}
	c.set(time.Now())
	s.d.Now = c.now
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunCause == store.CauseRetry {
		t.Fatalf("a 500 was tried again: %+v", got)
	}
	if got.NextAt != c.now().Add(time.Hour).Unix() {
		t.Fatalf("the cadence was not kept: %d", got.NextAt)
	}
}

// Three attempts, and then the turn is let go: a service answering 403 for
// ever is called three times, not for ever.
func TestATurnGetsThreeAttemptsAndNoMore(t *testing.T) {
	p := &plane{status: http.StatusNotFound}
	s, st, ctx := setup(t, p)
	c := &clock{}
	c.set(time.Now())
	s.d.Now = c.now
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		pass(ctx, t, s)
		c.add(3 * time.Minute)
	}
	if p.calls.Load() != int32(MaxAttempts) {
		t.Fatalf("the turn was attempted %d times, want %d", p.calls.Load(), MaxAttempts)
	}
	runs, _ := st.ListScheduleRuns(ctx, "s1", 0)
	if len(runs) != MaxAttempts || runs[0].Attempt != MaxAttempts {
		t.Fatalf("the attempts are not in the history: %+v", runs)
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunCause == store.CauseRetry {
		t.Fatalf("a fourth attempt was armed: %+v", got)
	}
}

// A call only worth making on time is not made again twenty minutes later:
// the catch-up bounds the retries as it bounds everything else.
func TestARetryPastTheCatchUpIsNotMade(t *testing.T) {
	p := &plane{status: http.StatusBadGateway}
	s, st, ctx := setup(t, p)
	c := &clock{}
	c.set(time.Now())
	s.d.Now = c.now
	sc := due("s1")
	// Owed now, with ten seconds of catch-up: the turn goes out, and the
	// attempt that would follow is thirty seconds later - too late.
	sc.NextAt, sc.CatchUp = c.now().Unix(), 10
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunCause == store.CauseRetry {
		t.Fatalf("a retry was armed past the catch-up: %+v", got)
	}
	if p.calls.Load() != 1 {
		t.Fatalf("called %d times", p.calls.Load())
	}
}

// "Not now, come back at ten": the service knows why - a source not published
// yet - so it says when, and the gateway does not guess (SCHED-05).
func TestTheServicePutsItsOwnTurnOff(t *testing.T) {
	putOff := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(http.StatusFailedDependency)
		_, _ = w.Write([]byte("the daily extract is not published yet"))
	})
	s, st, ctx := setup(t, putOff)
	c := &clock{}
	c.set(time.Now())
	s.d.Now = c.now
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	pass(ctx, t, s)
	got, _ := st.GetSchedule(ctx, "s1")
	if got.NextAt != c.now().Add(10*time.Minute).Unix() {
		t.Fatalf("the turn was not put off to where the service asked: %d", got.NextAt)
	}
	if got.RunCause != store.CauseAsked || got.TurnTry != 1 {
		t.Fatalf("what follows does not say the service asked for it: %+v", got)
	}
	runs, _ := st.ListScheduleRuns(ctx, "s1", 0)
	if len(runs) != 1 || runs[0].State != store.RunFailed {
		t.Fatalf("the run that asked is not in the history: %+v", runs)
	}
	// The REASON stays on the run that gave it: that is what the chain is
	// read from the morning after.
	if !strings.Contains(runs[0].Detail, "not published yet") {
		t.Fatalf("the reason was lost: %q", runs[0].Detail)
	}
}

// What a service asks for is bounded at both ends, and clamped rather than
// refused: "in five seconds" is a minute, "in a fortnight" is a day.
func TestWhatAServiceAsksForIsBounded(t *testing.T) {
	for _, c := range []struct {
		name, after string
		want        time.Duration
	}{
		{"too soon", "5", store.MinAsk},
		{"reasonable", "600", 10 * time.Minute},
		{"a fortnight", "1209600", store.MaxAsk},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := &http.Response{
				StatusCode: http.StatusFailedDependency,
				Header:     http.Header{"Retry-After": []string{c.after}},
			}
			now := time.Now()
			at, ok := askedAgain(res, now)
			if !ok {
				t.Fatal("the service was not heard")
			}
			if at.Sub(now) != c.want {
				t.Fatalf("asked for %s, got %s, want %s", c.after, at.Sub(now), c.want)
			}
		})
	}
	// Without a Retry-After there is nothing to honour: an ordinary failure.
	if _, ok := askedAgain(&http.Response{StatusCode: http.StatusFailedDependency,
		Header: http.Header{}}, time.Now()); ok {
		t.Fatal("a 424 with nothing to say moved a turn")
	}
	// And 503 is not this answer: it says "I am down", which is what trying
	// again already covers.
	if _, ok := askedAgain(&http.Response{StatusCode: http.StatusServiceUnavailable,
		Header: http.Header{"Retry-After": []string{"600"}}}, time.Now()); ok {
		t.Fatal("a 503 was taken for the service naming its own moment")
	}
}

// A service that asks for ever is broken, not waiting: after five in a row
// the turn is let go and the schedule's own cadence takes over.
func TestAServiceCannotPutATurnOffForEver(t *testing.T) {
	putOff := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusFailedDependency)
	})
	s, st, ctx := setup(t, putOff)
	c := &clock{}
	c.set(time.Now())
	s.d.Now = c.now
	if err := st.CreateSchedule(ctx, due("s1")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < store.MaxAsks+3; i++ {
		pass(ctx, t, s)
		c.add(2 * time.Minute)
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunCause == store.CauseAsked {
		t.Fatalf("the turn is still being put off: %+v", got)
	}
	if !strings.Contains(got.LastDetail, "letting the turn go") {
		t.Fatalf("nothing says why it stopped: %q", got.LastDetail)
	}
	// The cadence took over: the next turn is an hour after the run that gave
	// up, so nothing goes out in the minutes that follow.
	if got.NextAt <= c.now().Unix()-int64(time.Hour/time.Second) {
		t.Fatalf("the cadence did not take over: %d", got.NextAt)
	}
}
