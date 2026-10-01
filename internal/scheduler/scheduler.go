// Package scheduler runs the calls a service asked to have made for it
// (SCHED-01).
//
// THE SHAPE, and why it is this one.
//
// The gateway is the only component that can already reach every service,
// authenticate as a caller, and apply the rules in front of them. So a
// scheduled task is not a message on a bus somebody has to run: it is an
// ordinary request that enters by the gateway's own front door, at an hour
// nobody had to be awake for. A service exposes an endpoint - it already does
// - and needs no library, no protocol of ours, and no broker.
//
// IT RUNS AS "meerkat", CARRYING THE ROLES THE SCHEDULE ASKS FOR. There is no
// account behind it and nothing is minted: the identity is posed in the
// CONTEXT of the in-process request, never in a header, so nothing about it
// travels on a wire and nothing about it can be forged from outside. The
// route's rule and the per-endpoint rules then read those roles exactly as
// they read a person's - a schedule is not a way around them at four in the
// morning.
//
// THE CLOCK IS HERE, AND ONLY HERE. There is no tick: the loop sleeps until
// the next thing owed - a turn coming due, a lease running out - and is rung
// awake when a schedule changes, on this node or, through the change bus, on
// another. A slow backstop covers a lost notification and a row edited in
// SQL. The same code runs on the embedded database and on PostgreSQL:
// PostgreSQL brings the doorbell between nodes, never the clock - it has
// none, and an extension that gave it one would still need a gateway up to
// make the call, since the call goes through the gateway.
//
// NO LOCK. Every node that wakes tries to take what is owed; the claim is an
// UPDATE carrying its own condition, so each turn has one winner. The calls
// therefore spread over the nodes, and they are made OUTSIDE any lock, a few
// at a time per node: a slow service holds its own call and nobody else's.
//
// AT LEAST ONCE. A call can succeed at the service and fail on the way back,
// and a node can stop in the middle of one. Every call carries a run
// identifier, stable for the whole run, and a service that sees the same one
// twice must ignore the second. A call whose node stopped before the answer
// is SENT AGAIN, same identifier, by whichever node sees its lease lapse -
// within the schedule's catch-up, and MaxAttempts times at most, so a call
// that brings a node down does not bring them all down in turn. A node that
// stops cleanly hands its calls back at once rather than letting their lease
// run out. Exactly-once would need a transaction shared between the gateway
// and the service, which means a shared database, which is the dependency
// this product exists without.
//
// LONG WORK ANSWERS 202. A call starts the work; it does not wait for it. A
// service that answers 202 keeps the run open and reports its own progress,
// which is what makes a three-hour job expressible without a three-hour HTTP
// request. Anything else closes the run there and then. A failure is not
// retried: the next turn is the retry, at the schedule's own cadence.
package scheduler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

const (
	// DefaultBackstop is the longest the loop sleeps, whatever the table
	// says. It is not what makes a schedule punctual - the wake-up is - it is
	// what makes a lost notification cost a minute rather than a turn.
	DefaultBackstop = time.Minute
	// DefaultLease is how long a run may go without news before it is
	// settled: sent again when nobody answered it, lost when the service took
	// it and went quiet. Longer than store.MaxScheduleTimeout on purpose - a
	// call still waiting for its answer must never look like a dead node's.
	DefaultLease = 5 * time.Minute
	// DefaultTimeout is how long a call waits for its answer when the
	// schedule does not say: store.MaxScheduleTimeout at most either way.
	DefaultTimeout = 30 * time.Second
	// DefaultConcurrency is how many calls one node makes at once. Past it,
	// what is owed stays owed: another node takes it, or this one when a
	// call comes back.
	DefaultConcurrency = 16
	// MaxAttempts is how many attempts one TURN gets - a call sent again
	// because the gateway making it stopped, and a call tried again after a
	// failure worth trying again, both count against it.
	MaxAttempts = 3
	// minPause is the shortest sleep between two passes. A row that cannot
	// be settled - a database refusing a write - costs a pass a second, not a
	// core.
	minPause = time.Second

	// RunHeader carries the run identifier, and it is the service's half of
	// the at-least-once contract.
	RunHeader = "Meerkat-Job-Run"
	// JobHeader names the schedule, so a service can tell two of them apart
	// without keeping a map of run ids.
	JobHeader = "Meerkat-Job"
	// AttemptHeader counts the attempts of this turn: 2 or more says the first
	// one did not land, or did not land well.
	AttemptHeader = "Meerkat-Job-Attempt"
)

// The service's own answer to "not now": 424 Failed Dependency with a
// Retry-After. It is not 503 on purpose - 503 says "I am down", which is
// already covered by trying again - and 424 says "I am fine, what I needed
// is not", which is the case where the SERVICE knows when to come back and
// the gateway cannot guess.
// askedAgain reads the service's answer to "not now": 424 with a Retry-After,
// in seconds or as an HTTP date. Anything else is an ordinary failure.
func askedAgain(res *http.Response, now time.Time) (time.Time, bool) {
	if res.StatusCode != http.StatusFailedDependency {
		return time.Time{}, false
	}
	after := strings.TrimSpace(res.Header.Get("Retry-After"))
	if after == "" {
		return time.Time{}, false
	}
	// Seconds, as a service writes them nine times out of ten; an HTTP date
	// otherwise, which RFC 9110 allows and some frameworks send.
	at, err := asSeconds(after, now)
	if err != nil {
		date, dateErr := http.ParseTime(after)
		if dateErr != nil {
			return time.Time{}, false
		}
		at = date
	}
	// Bounded at both ends, and clamped rather than refused: a service that
	// says "in five seconds" gets a minute, and one that says "in a fortnight"
	// gets a day. Refusing would turn a reasonable answer into a lost turn.
	return within(at, now.Add(store.MinAsk), now.Add(store.MaxAsk)), true
}

func asSeconds(after string, now time.Time) (time.Time, error) {
	secs, err := strconv.Atoi(after)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(time.Duration(secs) * time.Second), nil
}

func within(at, floor, ceiling time.Time) time.Time {
	if at.Before(floor) {
		return floor
	}
	if at.After(ceiling) {
		return ceiling
	}
	return at
}

// backoff is how long the gateway waits before trying a turn again. It grows,
// because the two things worth trying again - a service restarting, a version
// rolling out - take tens of seconds, not milliseconds, and hammering one
// that is coming back up is how a retry becomes the outage.
var backoff = []time.Duration{30 * time.Second, 2 * time.Minute}

// Retryable says whether a failed call is worth making again, and it is a
// SHORT list on purpose.
//
// Behind a scheduled call there is an internal service, so the answers below
// are usually a moment rather than a verdict: a gateway that has just
// restarted does not know the account yet (401), the roles are not loaded
// (403), the route of a version still rolling out is not there (404), and
// nobody answered at all (502, 503, 504, or our own timeout, which arrives
// here as a zero).
//
// Everything else is the service SAYING something, and a 500 most of all:
// that is a bug at the far end, and the far end is where it is handled. The
// next turn is the retry, at the schedule's own cadence.
func Retryable(status int) bool {
	switch status {
	case 0, // no answer: our timeout
		http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// Deps is what the scheduler needs from the gateway that hosts it.
type Deps struct {
	Store *store.Store
	// Plane is the DATA plane's handler. The call goes through it rather than
	// straight to the upstream, and that is the containment: predicates,
	// filters, the route's rule, the per-endpoint rules and the organisation's
	// hours all apply to a scheduled call exactly as they apply to a person.
	Plane http.Handler
	// Host is what the synthetic request presents, for routes chosen by host.
	// Empty means localhost.
	Host string
	Node string
	// Announce tells the other nodes the schedules moved. Called when this
	// node hands its calls back as it stops, so that another sends them again
	// now rather than at its next look. Nil on a single gateway.
	Announce    func(context.Context)
	Now         func() time.Time
	Backstop    time.Duration
	Lease       time.Duration
	Concurrency int
}

// Scheduler is the loop, and the calls it has in flight.
type Scheduler struct {
	d    Deps
	wake chan struct{}
	// slots holds one token per call in flight on this node.
	slots chan struct{}
	calls sync.WaitGroup
	// callsCtx is what every call runs under, apart from the loop's own
	// context: stopping the loop stops the TAKING, and Drain decides what
	// happens to the calls already made.
	callsCtx context.Context
	cut      context.CancelFunc
	// loopDone is closed when Run returns, so Drain never counts calls while
	// a pass could still be starting one.
	started  atomic.Bool
	loopDone chan struct{}
	// handedBack counts the calls cut at shutdown and handed back.
	handedBack atomic.Int32
}

// New builds a scheduler, filling the defaults.
func New(d Deps) *Scheduler {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Backstop <= 0 {
		d.Backstop = DefaultBackstop
	}
	if d.Lease <= 0 {
		d.Lease = DefaultLease
	}
	if d.Concurrency <= 0 {
		d.Concurrency = DefaultConcurrency
	}
	if d.Node == "" {
		d.Node = "node"
	}
	callsCtx, cut := context.WithCancel(context.Background())
	return &Scheduler{
		d:        d,
		wake:     make(chan struct{}, 1),
		slots:    make(chan struct{}, d.Concurrency),
		callsCtx: callsCtx,
		cut:      cut,
		loopDone: make(chan struct{}),
	}
}

// Wake asks for a pass now rather than at the next wake-up. Called when a
// schedule is written here, when a call comes back, and by the change bus
// when a schedule is written on another node - the notification is a
// doorbell, never the truth.
func (s *Scheduler) Wake() {
	select {
	case s.wake <- struct{}{}:
	default: // one pending wake-up is as good as ten
	}
}

// Run serves until ctx ends. It looks at once - turns owed while the gateway
// was down are owed now - then sleeps until the next thing is.
func (s *Scheduler) Run(ctx context.Context) {
	s.started.Store(true)
	defer close(s.loopDone)
	for {
		if err := s.Pass(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("scheduler pass", "err", err)
		}
		timer := time.NewTimer(s.sleep(ctx))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		case <-s.wake:
			timer.Stop()
		}
	}
}

// sleep is how long until anything is owed, between minPause and the
// backstop. A node with no slot free waits for a call to come back instead -
// the call rings Wake as it ends - since what is owed is not its to take.
func (s *Scheduler) sleep(ctx context.Context) time.Duration {
	if len(s.slots) == cap(s.slots) {
		return s.d.Backstop
	}
	at, ok, err := s.d.Store.NextWake(ctx, s.d.Lease)
	if err != nil || !ok {
		return s.d.Backstop
	}
	return min(max(at.Sub(s.d.Now()), minPause), s.d.Backstop)
}

// Drain is this node leaving. Call it once Run's context has ended: it waits
// for the calls in flight until ctx ends, then cuts the rest and HANDS THEM
// BACK, so another node sends them again at once, same run identifier,
// rather than when their lease runs out. A rolling deployment is the common
// case, not a crash.
func (s *Scheduler) Drain(ctx context.Context) {
	if s.started.Load() {
		select {
		case <-s.loopDone:
		case <-ctx.Done():
			return // the loop is stuck on the database; the leases will do
		}
	}
	done := make(chan struct{})
	go func() {
		s.calls.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
	}
	s.cut()
	// A cut call returns as soon as the plane lets go of it. Bounded anyway:
	// the process is leaving, and a lease settles whatever was not handed back.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	if n := s.handedBack.Load(); n > 0 {
		slog.Info("scheduler: calls in flight handed back to the other nodes", "calls", n)
		if s.d.Announce != nil {
			actx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			s.d.Announce(actx)
			cancel()
		}
	}
}

// Pass does one round: settle the runs whose lease lapsed, then take what is
// owed. Exported because it is the whole behaviour - the loop above only
// decides when. The calls it starts run on their own; it does not wait.
func (s *Scheduler) Pass(ctx context.Context) error {
	now := s.d.Now()
	s.reap(ctx, now)
	return s.fireDue(ctx, now)
}

// reap settles the runs nobody is saying anything about. A row left running
// for ever is a schedule that silently stopped.
func (s *Scheduler) reap(ctx context.Context, now time.Time) {
	lapsed, err := s.d.Store.LapsedRuns(ctx, now, s.d.Lease)
	if err != nil {
		slog.Warn("scheduler: lapsed runs", "err", err)
		return
	}
	for _, sc := range lapsed {
		switch {
		case sc.RunState == store.RunAccepted:
			s.abandon(ctx, sc, now, fmt.Sprintf(
				"the service took the call (202) and has not reported for %s", s.d.Lease))
		case sc.Paused:
			s.abandon(ctx, sc, now,
				"the node making the call stopped before the answer, and the schedule is paused")
		case sc.Attempts >= MaxAttempts:
			s.abandon(ctx, sc, now, fmt.Sprintf(
				"no answer after %d attempts: each time, the node making the call stopped first", sc.Attempts))
		case sc.TooLate(now):
			s.abandon(ctx, sc, now,
				"the node making the call stopped before the answer, and sending it again would be later than this schedule's catch-up allows")
		default:
			if !s.takeSlot() {
				return // no room here: another node, or this one's next pass
			}
			retaken, err := s.d.Store.RetakeRun(ctx, sc.ID, sc.RunID, s.d.Node, now, s.d.Lease)
			if err != nil || !retaken {
				s.freeSlot()
				if err != nil {
					slog.Warn("scheduler: taking a call back", "schedule", sc.ID, "err", err)
				}
				continue
			}
			sc.Attempts++
			slog.Warn("scheduled call sent again: the node making it stopped before the answer",
				"schedule", sc.Name, "run", sc.RunID, "attempt", sc.Attempts, "was_claimed_by", sc.ClaimedBy)
			s.launch(sc)
		}
	}
}

// abandon closes a lapsed run as lost, if nobody took it back meanwhile.
func (s *Scheduler) abandon(ctx context.Context, sc store.Schedule, now time.Time, why string) {
	closed, err := s.d.Store.AbandonRun(ctx, sc.ID, sc.RunID, why, s.next(sc, now), now, s.d.Lease)
	if err != nil {
		slog.Warn("scheduler: closing an abandoned run", "schedule", sc.ID, "err", err)
		return
	}
	if closed {
		slog.Warn("scheduled call lost", "schedule", sc.Name, "run", sc.RunID, "why", why)
	}
}

// fireDue takes what is owed, as long as there is room to make the calls.
func (s *Scheduler) fireDue(ctx context.Context, now time.Time) error {
	due, err := s.d.Store.DueSchedules(ctx, now)
	if err != nil {
		return err
	}
	for _, sc := range due {
		// Missed for longer than the schedule accepts. A gateway that was down
		// all night should not fire twelve hourly runs on waking.
		if sc.TooLate(now) {
			late := now.Sub(time.Unix(sc.NextAt, 0)).Round(time.Second)
			why := fmt.Sprintf("the turn owed at %s came round %s late, past the %ds this schedule allows",
				time.Unix(sc.NextAt, 0).UTC().Format(time.RFC3339), late, sc.CatchUp)
			dropped, err := s.d.Store.DropTurn(ctx, sc.ID, time.Unix(sc.NextAt, 0), s.next(sc, now), now, why)
			if err != nil {
				slog.Warn("scheduler: dropping a late turn", "schedule", sc.ID, "err", err)
			} else if dropped {
				slog.Info("scheduled call dropped: later than its catch-up allows",
					"schedule", sc.Name, "due", sc.NextAt)
			}
			continue
		}
		if !s.takeSlot() {
			// What is left stays owed. In a cluster another node takes it; here,
			// the call that comes back first rings the next pass.
			slog.Debug("scheduler: every slot is busy, leaving the rest owed", "left", sc.Name)
			return nil
		}
		runID, err := newRunID()
		if err != nil {
			s.freeSlot()
			return err
		}
		claimed, err := s.d.Store.ClaimSchedule(ctx, sc.ID, runID, s.d.Node, now)
		if err != nil || !claimed {
			// Not claimed is the normal answer in a cluster: another node took
			// this turn between the listing and here.
			s.freeSlot()
			if err != nil {
				slog.Warn("scheduler: claim", "schedule", sc.ID, "err", err)
			}
			continue
		}
		// Which attempt of the TURN this is: the claim wrote turn_try + 1, and
		// what follows - the header, the history, the retry decision - all
		// read it from here.
		sc.RunID, sc.RunState, sc.Attempts = runID, store.RunCalling, sc.TurnTry+1
		s.launch(sc)
	}
	return nil
}

func (s *Scheduler) takeSlot() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Scheduler) freeSlot() { <-s.slots }

// launch makes the call on its own goroutine, holding the slot taken for it.
func (s *Scheduler) launch(sc store.Schedule) {
	s.calls.Add(1)
	go func() {
		defer s.calls.Done()
		defer s.Wake() // a slot came free, and a run closed: time to look again
		defer s.freeSlot()
		s.call(sc)
	}()
}

// settle waits for the calls in flight. Tests only: the loop never waits.
func (s *Scheduler) settle() { s.calls.Wait() }

// requestBody is what goes on the wire, and which Content-Type says so.
//
// An object or an array is sent as it stands - JSON in, JSON out. A JSON
// STRING is sent as its content, which is how a form body, an XML document or
// a line of text is expressed in a field that is otherwise JSON. An explicit
// contentType always wins: it is the one the service asked for.
func requestBody(sc store.Schedule) ([]byte, string) {
	if len(sc.Body) == 0 {
		return nil, sc.ContentType
	}
	var text string
	if err := json.Unmarshal(sc.Body, &text); err == nil {
		kind := sc.ContentType
		if kind == "" {
			kind = "text/plain; charset=utf-8"
		}
		return []byte(text), kind
	}
	kind := sc.ContentType
	if kind == "" {
		kind = "application/json"
	}
	return sc.Body, kind
}

// secrets resolves the vault references a header value carries. A name the
// vault does not hold is left VERBATIM - `$typo` arrives as `$typo`, and the
// service says no - rather than silently becoming an empty header, which
// fails in a far more confusing way.
func (s *Scheduler) secrets(ctx context.Context, sc store.Schedule, value string) string {
	if !strings.Contains(value, "$") {
		return value
	}
	scope := vault.ScopeApp
	if sc.TenantID != "" {
		scope = vault.TenantScope(sc.TenantID)
	}
	values, err := s.d.Store.VaultValues(ctx, scope)
	if err != nil {
		slog.Warn("scheduler: reading the vault", "schedule", sc.ID, "err", err)
		return value
	}
	out, missing := vault.Expand(value, func(name string) (string, bool) {
		v, ok := values[name]
		return v, ok
	})
	if len(missing) > 0 {
		slog.Warn("scheduled call: a header names a vault entry that is not there",
			"schedule", sc.ID, "missing", missing)
	}
	return out
}

// close ends a run and arms what comes next: another attempt at the same turn
// when the failure is worth one, and otherwise the next turn, counted from NOW
// - the end of the run, which is what a cadence promises. What it says goes on
// the row AND in the history (SCHED-03).
func (s *Scheduler) close(ctx context.Context, sc store.Schedule, end store.RunEnd) {
	now := s.d.Now()
	next := store.RunNext{At: s.next(sc, now)}
	if at, ok := s.retryAt(sc, end, now); ok {
		next = store.RunNext{At: at, Cause: store.CauseRetry, Tries: sc.Attempts}
		slog.Info("scheduled call will be tried again", "schedule", sc.Name, "run", sc.RunID,
			"status", end.Status, "attempt", sc.Attempts, "again_at", at.UTC().Format(time.RFC3339))
	}
	if err := s.d.Store.FinishSchedule(ctx, sc.ID, sc.RunID, end, next, now); err != nil {
		slog.Warn("scheduler: closing a run", "schedule", sc.ID, "err", err)
	}
}

// closeAsked ends a run the service answered "not now" to, and arms the turn
// again when IT said. The reason it gave stays on this run - that is what the
// chain is read from the morning after - and the turn that follows points
// back at it.
func (s *Scheduler) closeAsked(ctx context.Context, sc store.Schedule, end store.RunEnd, at time.Time) {
	now := s.d.Now()
	if sc.TurnTry >= store.MaxAsks {
		// Asked five times in a row: this is not a moment any more. The turn
		// is let go, and the schedule's own cadence takes over.
		end.Detail += fmt.Sprintf(" (asked to be called back %d times in a row: letting the turn go)", sc.TurnTry)
		slog.Warn("scheduled call: the service keeps asking to be called back",
			"schedule", sc.Name, "asks", sc.TurnTry)
		s.close(ctx, sc, end)
		return
	}
	slog.Info("scheduled call put off by the service", "schedule", sc.Name, "run", sc.RunID,
		"again_at", at.UTC().Format(time.RFC3339), "asks", sc.TurnTry+1)
	next := store.RunNext{At: at, Cause: store.CauseAsked, Tries: sc.TurnTry + 1}
	if err := s.d.Store.FinishSchedule(ctx, sc.ID, sc.RunID, end, next, now); err != nil {
		slog.Warn("scheduler: closing a run the service put off", "schedule", sc.ID, "err", err)
	}
}

// retryAt says whether this failure gets another attempt, and when.
//
// Three bounds, and each one exists for a reason somebody met: MaxAttempts,
// so a service that answers 403 for ever is called three times and not for
// ever; the schedule's CATCH-UP, because a call that is only worth making on
// time is not worth making twenty minutes later; and the growing backoff,
// because what is being waited for is a service coming back up.
func (s *Scheduler) retryAt(sc store.Schedule, end store.RunEnd, now time.Time) (time.Time, bool) {
	if end.State != store.RunFailed || !Retryable(end.Status) {
		return time.Time{}, false
	}
	tries := max(sc.Attempts, 1)
	if tries >= MaxAttempts {
		return time.Time{}, false
	}
	at := now.Add(backoff[min(tries-1, len(backoff)-1)])
	// The catch-up is measured from the turn this run is for, which is the
	// one still written on the row while the run is open.
	if sc.CatchUp > 0 && at.Unix()-sc.NextAt > sc.CatchUp {
		slog.Info("scheduled call not tried again: the next attempt would be past its catch-up",
			"schedule", sc.Name, "catch_up", sc.CatchUp)
		return time.Time{}, false
	}
	return at, true
}

// call makes one scheduled request and records what came back.
func (s *Scheduler) call(sc store.Schedule) {
	timeout := DefaultTimeout
	if d, err := store.ParseISODuration(sc.Timeout); err == nil && d > 0 {
		// Clamped as well as refused at the door: a call still waiting when
		// its lease lapses would be sent a second time by another node.
		timeout = min(d, store.MaxScheduleTimeout)
	}
	callCtx, cancel := context.WithTimeout(s.callsCtx, timeout)
	defer cancel()
	// What is written about the run is written whatever became of the call:
	// a call cut at shutdown still has to be handed back.
	ctx := context.WithoutCancel(callCtx)

	// WHO THE CALL IS. No account, no credential to mint and delete: the
	// identity is posed in the context of a request this process hands
	// straight to the data plane, so it never travels on a wire and cannot be
	// forged from outside. The route's rule, the endpoint rules and whatever
	// the route forwards to the service all read it as they read a person's.
	callCtx = gateway.WithScheduledCaller(callCtx, sc.Roles, sc.TenantID)

	host := s.host(callCtx, sc)
	payload, contentType := requestBody(sc)
	var body io.Reader
	if len(payload) > 0 {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(callCtx, sc.Method, "http://"+host+sc.Path, body)
	if err != nil {
		s.close(ctx, sc, store.RunEnd{State: store.RunFailed,
			Detail: "the schedule does not describe a request: " + err.Error()})
		return
	}
	req.Host = host
	req.Header.Set(RunHeader, sc.RunID)
	req.Header.Set(JobHeader, sc.ID)
	req.Header.Set(AttemptHeader, strconv.Itoa(max(sc.Attempts, 1)))
	req.Header.Set("User-Agent", "meerkat-scheduler")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	// What the SERVICE asked to be told, beyond what the gateway says by
	// itself. Vault references are resolved HERE, at the moment of the call:
	// the row holds `$stations-key`, the wire holds the key, and nothing in
	// between - not this API's answers, not the console - ever saw it.
	for name, value := range sc.Headers {
		req.Header.Set(name, s.secrets(callCtx, sc, value))
	}

	rec := httptest.NewRecorder()
	s.d.Plane.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if callCtx.Err() != nil {
		// Cut by Drain: this node is leaving before the answer came. Nobody
		// knows whether the service got it, so it goes back to be sent again
		// - by a node that is staying.
		if s.callsCtx.Err() != nil {
			if err := s.d.Store.ReleaseRun(ctx, sc.ID, sc.RunID, s.d.Now()); err != nil {
				slog.Warn("scheduler: handing a call back", "schedule", sc.ID, "err", err)
				return
			}
			s.handedBack.Add(1)
			return
		}
		// The call passed its own deadline. The plane turns that into a 502,
		// like any other upstream that went quiet, so recording the answer
		// verbatim would blame the service for a wait WE stopped. Say which it
		// was.
		s.close(ctx, sc, store.RunEnd{State: store.RunFailed,
			Detail: fmt.Sprintf("no answer within the %s this schedule allows", timeout)})
		return
	}

	// 202 means the service took the work and will say when it is done. The
	// run stays OPEN, and its lease is what protects against a service that
	// never comes back - a lapse now means the SERVICE went quiet, so nothing
	// is sent again.
	if res.StatusCode == http.StatusAccepted {
		if err := s.d.Store.AcceptRun(ctx, sc.ID, sc.RunID, s.d.Now()); err != nil {
			slog.Warn("scheduler: recording a 202", "schedule", sc.ID, "err", err)
		}
		slog.Info("scheduled call accepted", "schedule", sc.Name, "run", sc.RunID)
		return
	}
	state, detail := store.RunDone, fmt.Sprintf("%d %s", res.StatusCode, http.StatusText(res.StatusCode))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		state = store.RunFailed
		if extra := summarize(res.Header.Get("Content-Type"), rec.Body.String()); extra != "" {
			detail += ": " + extra
		}
	}
	if sc.Attempts > 1 {
		detail += fmt.Sprintf(" (attempt %d)", sc.Attempts)
	}
	slog.Info("scheduled call made", "schedule", sc.Name, "run", sc.RunID,
		"route", sc.RouteID, "roles", sc.Roles, "status", res.StatusCode, "attempt", sc.Attempts)
	end := store.RunEnd{State: state, Detail: detail, Status: res.StatusCode}
	// "Not now, come back then": the service knows why, so it decides when.
	if at, asked := askedAgain(res, s.d.Now()); asked {
		s.closeAsked(ctx, sc, end, at)
		return
	}
	s.close(ctx, sc, end)
}

// host is what the synthetic request presents. A route chosen by NAME rather
// than by path only matches a request carrying that name, and a scheduled call
// is a request like any other: so the route's own first host is used when it
// declares one, and the gateway's otherwise.
func (s *Scheduler) host(ctx context.Context, sc store.Schedule) string {
	rt, err := s.d.Store.GetRoute(ctx, sc.RouteID)
	if err == nil {
		for _, p := range rt.Predicates {
			if p.Type != "host" {
				continue
			}
			switch v := p.Args["hosts"].(type) {
			case string:
				if h := firstHost(v); h != "" {
					return h
				}
			case []any:
				for _, one := range v {
					if str, ok := one.(string); ok {
						if h := firstHost(str); h != "" {
							return h
						}
					}
				}
			case []string:
				for _, one := range v {
					if h := firstHost(one); h != "" {
						return h
					}
				}
			}
		}
	}
	if s.d.Host != "" {
		return s.d.Host
	}
	return "localhost"
}

// firstHost takes one name out of a list, skipping the wildcards: "*.acme.io"
// is a shape, not an address, and presenting it verbatim matches nothing.
func firstHost(list string) string {
	for _, one := range strings.Split(list, ",") {
		one = strings.TrimSpace(one)
		if one == "" || strings.HasPrefix(one, "*") {
			continue
		}
		return one
	}
	return ""
}

// next is the moment this schedule is owed again, and a ZERO time when it is
// owed nothing more - a single date, which ends with the run it asked for.
//
// For a CADENCE it is one interval from now, never from the turn it missed,
// so a gateway coming back after a long night does not immediately owe every
// turn it slept through. For a CALENDAR it is the next occurrence, which
// answers the same question by itself: a night spent down is a run missed, not
// a queue to work through.
func (s *Scheduler) next(sc store.Schedule, now time.Time) time.Time {
	at, err := sc.NextRun(now)
	if err != nil {
		// Sanitize refuses both shapes of nonsense at the door, so this is a
		// row written before a rule existed. An hour is a cadence that keeps
		// the schedule visible instead of parking it for ever.
		slog.Warn("scheduler: this schedule no longer says when", "schedule", sc.ID, "err", err)
		return now.Add(time.Hour)
	}
	return at
}

func newRunID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// summarize is what a failure says beyond its status: enough to recognise the
// refusal, never a page. A page is the common case here - the gateway answers
// one itself when a service is down - so the markup goes and what a human
// would read stays, rather than two hundred characters of doctype.
func summarize(contentType, body string) string {
	body = strings.TrimSpace(body)
	if strings.Contains(strings.ToLower(contentType), "html") || strings.HasPrefix(body, "<") {
		body = stripTags(body)
	}
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > 200 {
		body = strings.TrimSpace(body[:200])
	}
	return body
}

var (
	// Script and style blocks go whole: what they contain is not what the
	// page says.
	dropBlocks = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`),
		regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`),
	}
	dropTags = regexp.MustCompile(`(?s)<[^>]*>`)
)

func stripTags(s string) string {
	for _, re := range dropBlocks {
		s = re.ReplaceAllString(s, " ")
	}
	s = dropTags.ReplaceAllString(s, " ")
	return html.UnescapeString(s)
}
