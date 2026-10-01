package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

func scheduleStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, User{ID: "u1", Username: "svc", Enabled: true}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return st, ctx
}

func aSchedule(id string) Schedule {
	return Schedule{
		ID: id, Name: "nightly close", Roles: []string{"ops"}, RouteID: "r1",
		Method: "POST", Path: "/jobs/close", Every: "PT1H",
	}
}

// The sanitiser names what is allowed rather than saying no: a refusal a
// service cannot act on is a support ticket.
func TestSanitizeScheduleNamesWhatIsAllowed(t *testing.T) {
	known := func(id string) bool { return id == "r1" }
	cases := []struct {
		name  string
		edit  func(*Schedule)
		wants string
	}{
		{"no name", func(s *Schedule) { s.Name = "" }, "needs a name"},
		{"no route", func(s *Schedule) { s.RouteID = "" }, "route to call"},
		{"unknown route", func(s *Schedule) { s.RouteID = "nope" }, "no route"},
		{"a read", func(s *Schedule) { s.Method = "GET" }, "not a read"},
		{"a relative path", func(s *Schedule) { s.Path = "jobs" }, "must start with /"},
		{"no cadence", func(s *Schedule) { s.Every = "" }, "ISO 8601 duration"},
		{"too fast", func(s *Schedule) { s.Every = "PT10S" }, "shortest cadence"},
		{"an unknown overlap", func(s *Schedule) { s.Overlap = "queue" }, "overlap must be"},
		{"a cadence and a date", func(s *Schedule) { s.At = "2026-10-03T04:00:00Z" }, "says WHEN once"},
		{"a date that is not one", func(s *Schedule) { s.Every, s.At = "", "3 octobre" }, "RFC 3339"},
		{"a date without its offset", func(s *Schedule) { s.Every, s.At = "", "2026-10-03T04:00:00" }, "RFC 3339"},
		{"a date with a start", func(s *Schedule) {
			s.Every, s.At, s.StartAt = "", "2026-10-03T04:00:00Z", 1790000000
		}, "already says when"},
		{"a wait longer than a lease", func(s *Schedule) { s.Timeout = "PT3M" }, "answers 202"},
		{"the attempt header", func(s *Schedule) {
			s.Headers = map[string]string{"Meerkat-Job-Attempt": "1"}
		}, "the gateway writes it"},
		{"both answers to when", func(s *Schedule) { s.Cron = "0 3 * * *" }, "says WHEN once"},
		{"a cron that is not one", func(s *Schedule) { s.Every, s.Cron = "", "0 3 *" }, "five fields"},
		{"an unknown zone", func(s *Schedule) {
			s.Every, s.Cron, s.Timezone = "", "0 3 * * *", "Europe/Atlantis"
		}, "IANA name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sc := aSchedule("s1")
			c.edit(&sc)
			err := SanitizeSchedule(&sc, known)
			if err == nil {
				t.Fatalf("accepted %s", c.name)
			}
			if !strings.Contains(err.Error(), c.wants) {
				t.Fatalf("the refusal does not say what is allowed: %v", err)
			}
		})
	}
	sc := aSchedule("s1")
	if err := SanitizeSchedule(&sc, known); err != nil {
		t.Fatalf("a good schedule was refused: %v", err)
	}
	if sc.Overlap != OverlapSkip {
		t.Fatalf("overlap defaults to %q, got %q", OverlapSkip, sc.Overlap)
	}
	// A zone means nothing to a duration, so it is not kept: a stored field
	// nobody reads is a field somebody will eventually believe in.
	sc = aSchedule("s1")
	sc.Timezone = "Europe/Paris"
	if err := SanitizeSchedule(&sc, known); err != nil {
		t.Fatal(err)
	}
	if sc.Timezone != "" {
		t.Fatalf("a cadence kept a timezone: %q", sc.Timezone)
	}
	// And a calendar keeps the one it was given.
	sc = aSchedule("s1")
	sc.Every, sc.Cron, sc.Timezone = "", "0 3 * * MON", "Europe/Paris"
	if err := SanitizeSchedule(&sc, known); err != nil {
		t.Fatalf("a calendar was refused: %v", err)
	}
	if sc.Timezone != "Europe/Paris" {
		t.Fatalf("the calendar lost its zone: %q", sc.Timezone)
	}
}

// A calendar's FIRST turn is its next occurrence, not the moment it was
// written: a nightly job created at noon must not fire at noon, once, before
// settling into the calendar it was given.
func TestACalendarOwesItsNextOccurrence(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.Every, sc.Cron, sc.Timezone = "", "0 3 * * *", "UTC"
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSchedule(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	next := time.Unix(got.NextAt, 0).UTC()
	if next.Hour() != 3 || next.Minute() != 0 {
		t.Fatalf("the first turn is not three in the morning: %s", next)
	}
	if !next.After(time.Now()) {
		t.Fatalf("the first turn is in the past: %s", next)
	}
	if time.Until(next) > 24*time.Hour {
		t.Fatalf("the first turn is more than a day out: %s", next)
	}
	// And the turn after that is a day later, with no drift: a cadence would
	// have counted from the end of the run instead.
	after, err := got.NextRun(next)
	if err != nil {
		t.Fatal(err)
	}
	if after.Sub(next) != 24*time.Hour {
		t.Fatalf("a daily calendar drifted: %s then %s", next, after)
	}
}

// The claim is the whole cluster story: two nodes asking at the same moment
// produce one winner, and nothing above it needs a lock.
func TestOnlyOneNodeClaimsATurn(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.NextAt = time.Now().Add(-time.Minute).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	first, err := st.ClaimSchedule(ctx, "s1", "run-a", "node-a", now)
	if err != nil || !first {
		t.Fatalf("the first node did not claim the turn: %v %v", first, err)
	}
	second, err := st.ClaimSchedule(ctx, "s1", "run-b", "node-b", now)
	if err != nil {
		t.Fatal(err)
	}
	if second {
		t.Fatalf("two nodes claimed the same turn")
	}
	got, err := st.GetSchedule(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != "run-a" || got.ClaimedBy != "node-a" {
		t.Fatalf("the loser overwrote the winner: %+v", got)
	}
}

// A node can die mid-call. The lease is what finds the run again, and the
// run is taken back AS ITSELF - same identifier, one more attempt - by exactly
// one node, never claimed afresh as if it had been a new turn.
func TestALapsedCallIsTakenBackByOneNode(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	// Owed before the moment the first node claims it, below: the claim
	// carries the same condition the due list does.
	sc.NextAt = time.Now().Add(-2 * time.Hour).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	long, lease := time.Now().Add(-time.Hour), 5*time.Minute
	if ok, err := st.ClaimSchedule(ctx, "s1", "run-a", "node-a", long); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	lapsed, err := st.LapsedRuns(ctx, time.Now(), lease)
	if err != nil {
		t.Fatal(err)
	}
	if len(lapsed) != 1 || lapsed[0].RunID != "run-a" || lapsed[0].RunState != RunCalling {
		t.Fatalf("the abandoned run was not found: %+v", lapsed)
	}
	// A run in flight is not a turn owed: nobody claims it afresh.
	if ok, _ := st.ClaimSchedule(ctx, "s1", "run-b", "node-b", time.Now()); ok {
		t.Fatal("a run in flight was claimed as a new turn")
	}
	now := time.Now()
	first, err := st.RetakeRun(ctx, "s1", "run-a", "node-b", now, lease)
	if err != nil || !first {
		t.Fatalf("the lapsed call was not taken back: %v %v", first, err)
	}
	second, _ := st.RetakeRun(ctx, "s1", "run-a", "node-c", now, lease)
	if second {
		t.Fatal("two nodes took the same call back")
	}
	// Nor can a node that saw it lapsed close it under the one that took it.
	if closed, _ := st.AbandonRun(ctx, "s1", "run-a", "lost", now.Add(time.Hour), now, lease); closed {
		t.Fatal("a run just taken back was closed as lost")
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID != "run-a" || got.ClaimedBy != "node-b" || got.Attempts != 2 {
		t.Fatalf("taken back as something else: %+v", got)
	}
}

// A job the service TOOK is not sent again when its lease lapses: the service
// went quiet, and a second call would only be ignored. Only a call nobody
// answered qualifies.
func TestAnAcceptedRunIsNeverTakenBack(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.NextAt = time.Now().Add(-2 * time.Hour).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	long, lease := time.Now().Add(-time.Hour), 5*time.Minute
	if ok, _ := st.ClaimSchedule(ctx, "s1", "run-a", "node-a", long); !ok {
		t.Fatal("claim")
	}
	if err := st.AcceptRun(ctx, "s1", "run-a", long); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.RetakeRun(ctx, "s1", "run-a", "node-b", time.Now(), lease); ok {
		t.Fatal("a run the service accepted was taken back to be sent again")
	}
	now := time.Now()
	closed, err := st.AbandonRun(ctx, "s1", "run-a", "went quiet", now.Add(time.Hour), now, lease)
	if err != nil || !closed {
		t.Fatalf("the quiet run was not closed: %v %v", closed, err)
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID != "" || got.LastState != RunLost || got.NextAt <= now.Unix() {
		t.Fatalf("closed wrong: %+v", got)
	}
}

// A node that stops hands its call back at once: lapsed from that moment,
// whatever the lease says, and still the same run.
func TestAHandedBackCallIsLapsedAtOnce(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.NextAt = time.Now().Add(-time.Minute).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if ok, _ := st.ClaimSchedule(ctx, "s1", "run-a", "node-a", now); !ok {
		t.Fatal("claim")
	}
	if err := st.ReleaseRun(ctx, "s1", "run-a", now); err != nil {
		t.Fatal(err)
	}
	lapsed, err := st.LapsedRuns(ctx, now, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(lapsed) != 1 || lapsed[0].RunID != "run-a" {
		t.Fatalf("a handed-back call waits for its lease: %+v", lapsed)
	}
}

// The scheduler sleeps until the next thing owed, and this is the question it
// asks: the soonest turn, or the soonest lease to run out.
func TestTheNextWakeIsTheSoonestThingOwed(t *testing.T) {
	st, ctx := scheduleStore(t)
	lease := 5 * time.Minute
	if _, ok, err := st.NextWake(ctx, lease); err != nil || ok {
		t.Fatalf("nothing scheduled should mean nothing to wake for: %v %v", ok, err)
	}
	now := time.Now().Truncate(time.Second)
	later := aSchedule("later")
	later.NextAt = now.Add(time.Hour).Unix()
	paused := aSchedule("paused")
	paused.NextAt, paused.Paused = now.Add(time.Minute).Unix(), true
	for _, sc := range []Schedule{later, paused} {
		if err := st.CreateSchedule(ctx, sc); err != nil {
			t.Fatal(err)
		}
	}
	at, ok, err := st.NextWake(ctx, lease)
	if err != nil || !ok || !at.Equal(now.Add(time.Hour)) {
		t.Fatalf("a paused schedule is nothing to wake for: %s %v %v", at, ok, err)
	}
	// A run in flight: its turn is not owed again, its lease is what matters.
	running := aSchedule("running")
	running.NextAt = now.Add(-time.Minute).Unix()
	if err := st.CreateSchedule(ctx, running); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.ClaimSchedule(ctx, "running", "run-a", "node-a", now); !ok {
		t.Fatal("claim")
	}
	at, _, _ = st.NextWake(ctx, lease)
	if !at.Equal(now.Add(lease)) {
		t.Fatalf("the lease was not the next thing owed: %s, want %s", at, now.Add(lease))
	}
}

// "Run now" is refused while a run is in flight: that turn is the run's own
// until it closes, and the close arms the next one.
func TestATurnIsNotBroughtForwardUnderARun(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.NextAt = time.Now().Add(-time.Minute).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.ClaimSchedule(ctx, "s1", "run-a", "node-a", time.Now()); !ok {
		t.Fatal("claim")
	}
	armed, err := st.ArmSchedule(ctx, "s1", time.Now(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if armed {
		t.Fatal("a turn was brought forward under a run in flight")
	}
}

// A report from a previous run must not close the current one.
func TestAReportNamesItsOwnRun(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.NextAt = time.Now().Add(-time.Minute).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if ok, _ := st.ClaimSchedule(ctx, "s1", "run-a", "node-a", now); !ok {
		t.Fatal("claim")
	}
	if err := st.FinishSchedule(ctx, "s1", "run-stale", RunEnd{State: RunDone, Detail: "from a previous turn"}, RunNext{At: now.Add(time.Hour)}, now); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.RunID != "run-a" {
		t.Fatalf("a stale report closed the run in flight: %+v", got)
	}
	if err := st.FinishSchedule(ctx, "s1", "run-a", RunEnd{State: RunDone, Detail: "200 OK", Status: 200}, RunNext{At: now.Add(time.Hour)}, now); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetSchedule(ctx, "s1")
	if got.RunID != "" || got.LastState != RunDone {
		t.Fatalf("the run did not close: %+v", got)
	}
	if got.NextAt <= now.Unix() {
		t.Fatalf("the next turn was not armed: %d", got.NextAt)
	}
}

// Due, paused, and the catch-up bound.
func TestWhatIsOwedAndWhatIsTooLate(t *testing.T) {
	st, ctx := scheduleStore(t)
	now := time.Now()
	due := aSchedule("due")
	due.NextAt = now.Add(-time.Minute).Unix()
	paused := aSchedule("paused")
	paused.NextAt, paused.Paused = now.Add(-time.Minute).Unix(), true
	later := aSchedule("later")
	later.NextAt = now.Add(time.Hour).Unix()
	for _, sc := range []Schedule{due, paused, later} {
		if err := st.CreateSchedule(ctx, sc); err != nil {
			t.Fatal(err)
		}
	}
	list, err := st.DueSchedules(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "due" {
		t.Fatalf("what is owed is wrong: %+v", list)
	}
	late := aSchedule("late")
	late.NextAt, late.CatchUp = now.Add(-2*time.Hour).Unix(), 60
	if !late.TooLate(now) {
		t.Fatal("two hours late with a minute of catch-up should be too late")
	}
	late.CatchUp = 0
	if late.TooLate(now) {
		t.Fatal("no catch-up bound means run it whenever you can")
	}
}

// One role is said in the singular, and it lands in the same place as a list
// of one - a caller should not have to write a plural it does not mean.
func TestSanitizeScheduleRoleSingular(t *testing.T) {
	s := Schedule{Name: "n", RouteID: "r", Every: "PT1H", Role: " station_fetch "}
	if err := SanitizeSchedule(&s, nil); err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if len(s.Roles) != 1 || s.Roles[0] != "station_fetch" {
		t.Fatalf("roles = %v, want [station_fetch]", s.Roles)
	}
	// And it is not handed back as a second spelling of the same thing.
	if s.Role != "" {
		t.Fatalf("role kept as %q: the store holds one field", s.Role)
	}
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte(`"role"`)) {
		t.Fatalf("the singular is written back: %s", out)
	}
}

// Both spellings at once is a refusal that names them, not a silent merge.
func TestSanitizeScheduleRoleAndRoles(t *testing.T) {
	s := Schedule{Name: "n", RouteID: "r", Every: "PT1H", Role: "a", Roles: []string{"b"}}
	err := SanitizeSchedule(&s, nil)
	if err == nil {
		t.Fatal("role and roles together were accepted")
	}
	for _, want := range []string{"role", "roles"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error does not name %q: %v", want, err)
		}
	}
}

// The list is cleaned rather than stored as typed: blanks and repeats are
// noise, and the order somebody wrote is the order they read back.
func TestSanitizeScheduleRolesCleaned(t *testing.T) {
	s := Schedule{Name: "n", RouteID: "r", Every: "PT1H",
		Roles: []string{" ops ", "", "ops", "billing_run"}}
	if err := SanitizeSchedule(&s, nil); err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if len(s.Roles) != 2 || s.Roles[0] != "ops" || s.Roles[1] != "billing_run" {
		t.Fatalf("roles = %v, want [ops billing_run]", s.Roles)
	}

	many := make([]string, maxScheduleRoles+1)
	for i := range many {
		many[i] = fmt.Sprintf("r%d", i)
	}
	s = Schedule{Name: "n", RouteID: "r", Every: "PT1H", Roles: many}
	if err := SanitizeSchedule(&s, nil); err == nil {
		t.Fatalf("%d roles were accepted", len(many))
	}
}

// A single date is the third way to say when (SCHED-02): the call goes out
// then, once, and the schedule is over rather than armed again.
func TestASingleDateIsOwedOnceAndThenNothing(t *testing.T) {
	sc := aSchedule("s1")
	sc.Every, sc.At = "", "2026-10-03T06:00:00+02:00"
	if err := SanitizeSchedule(&sc, func(string) bool { return true }); err != nil {
		t.Fatalf("a single date was refused: %v", err)
	}
	if !sc.Once() {
		t.Fatal("a schedule with a date does not read as a single one")
	}
	// The offset is in the date: a zone beside it would be a second answer.
	sc.Timezone = "Europe/Paris"
	if err := SanitizeSchedule(&sc, func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if sc.Timezone != "" {
		t.Fatalf("a single date kept a timezone: %q", sc.Timezone)
	}
	want := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)
	if got := sc.FirstTurn(time.Now()); got != want.Unix() {
		t.Fatalf("the first turn is %s, want %s", time.Unix(got, 0).UTC(), want)
	}
	// And there is no next: the run that closes ends the schedule.
	next, err := sc.NextRun(want)
	if err != nil || !next.IsZero() {
		t.Fatalf("a single date owes a next turn: %s %v", next, err)
	}
}

// A single date whose run has closed is finished: nothing owed, nothing in
// flight, and the sweep may take it once the retention is past.
func TestAFinishedSingleDateIsSweptAfterItsRetention(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("once")
	sc.Every, sc.At = "", "2026-10-03T04:00:00Z"
	sc.NextAt = time.Now().Add(-time.Minute).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	keeps := aSchedule("hourly")
	keeps.NextAt = time.Now().Add(time.Hour).Unix()
	if err := st.CreateSchedule(ctx, keeps); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if ok, _ := st.ClaimSchedule(ctx, "once", "run-a", "node-a", now); !ok {
		t.Fatal("claim")
	}
	// The close arms nothing: a zero moment is not a turn owed since 1970.
	if err := st.FinishSchedule(ctx, "once", "run-a", RunEnd{State: RunDone, Detail: "200 OK", Status: 200}, RunNext{At: time.Time{}}, now); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetSchedule(ctx, "once")
	if got.NextAt != 0 || !got.Finished() {
		t.Fatalf("the single date was armed again: %+v", got)
	}
	// Still there for a while - somebody may want to see that it went out.
	if n, err := st.PurgeFinishedSchedules(ctx, now.Add(-24*time.Hour).Unix()); err != nil || n != 0 {
		t.Fatalf("swept too early: %d %v", n, err)
	}
	if n, err := st.PurgeFinishedSchedules(ctx, now.Add(time.Second).Unix()); err != nil || n != 1 {
		t.Fatalf("the finished date was not swept: %d %v", n, err)
	}
	if _, err := st.GetSchedule(ctx, "hourly"); err != nil {
		t.Fatalf("the sweep took a schedule that repeats: %v", err)
	}
}

// A turn given up for being late is written on the row, not only in a log:
// the morning after, that is exactly what somebody comes looking for.
func TestADroppedTurnSaysSoOnTheRow(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.CatchUp = 60
	due := time.Now().Add(-2 * time.Hour)
	sc.NextAt = due.Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	next := now.Add(time.Hour)
	dropped, err := st.DropTurn(ctx, "s1", due, next, now, "two hours late")
	if err != nil || !dropped {
		t.Fatalf("the late turn was not dropped: %v %v", dropped, err)
	}
	got, _ := st.GetSchedule(ctx, "s1")
	if got.LastState != RunDropped || got.NextAt != next.Unix() {
		t.Fatalf("dropped wrong: %+v", got)
	}
	// And a turn that moved since is not the turn this was about.
	again, _ := st.DropTurn(ctx, "s1", due, next, now, "two hours late")
	if again {
		t.Fatal("a turn was dropped twice")
	}
}

// The history (SCHED-03): the row keeps the last result, this keeps them all
// - what ended, how, what answered, and what it continues.
func TestEveryEndedTurnLandsInTheHistory(t *testing.T) {
	st, ctx := scheduleStore(t)
	sc := aSchedule("s1")
	sc.NextAt = time.Now().Add(-time.Minute).Unix()
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if ok, _ := st.ClaimSchedule(ctx, "s1", "run-a", "node-a", now); !ok {
		t.Fatal("claim")
	}
	if err := st.FinishSchedule(ctx, "s1", "run-a",
		RunEnd{State: RunDone, Detail: "200 OK", Status: 200}, RunNext{At: now.Add(time.Hour)}, now); err != nil {
		t.Fatal(err)
	}
	runs, err := st.ListScheduleRuns(ctx, "s1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("the ended turn is not in the history: %+v", runs)
	}
	one := runs[0]
	if one.State != RunDone || one.Status != 200 || one.RunID != "run-a" || one.Node != "node-a" {
		t.Fatalf("the history lost what happened: %+v", one)
	}
	if one.Cause != CauseTurn {
		t.Fatalf("an ordinary turn reads %q", one.Cause)
	}

	// A replay: the turn goes out again, as its own run, and the history
	// says which one it continues.
	if ok, err := st.ArmSchedule(ctx, "s1", now, CauseReplay, one.ID); err != nil || !ok {
		t.Fatalf("arm a replay: %v %v", ok, err)
	}
	if ok, _ := st.ClaimSchedule(ctx, "s1", "run-b", "node-b", now); !ok {
		t.Fatal("claim the replay")
	}
	if err := st.FinishSchedule(ctx, "s1", "run-b",
		RunEnd{State: RunFailed, Detail: "403 Forbidden", Status: 403}, RunNext{At: now.Add(time.Hour)}, now); err != nil {
		t.Fatal(err)
	}
	runs, _ = st.ListScheduleRuns(ctx, "s1", 0)
	if len(runs) != 2 {
		t.Fatalf("the replay is not in the history: %+v", runs)
	}
	replay := runs[0]
	if replay.Cause != CauseReplay || replay.OfRun != one.ID {
		t.Fatalf("the chain is broken: %+v", replay)
	}
	// And the schedule is back to owing ordinary turns.
	back, _ := st.GetSchedule(ctx, "s1")
	if back.RunCause != "" || back.RunOf != "" {
		t.Fatalf("the replay stuck to the schedule: %+v", back)
	}

	// A turn that never went out is history too - it is what somebody comes
	// looking for, and what they then replay.
	due := time.Unix(back.NextAt, 0)
	if ok, err := st.DropTurn(ctx, "s1", due, due.Add(time.Hour), now, "too late"); err != nil || !ok {
		t.Fatalf("drop: %v %v", ok, err)
	}
	runs, _ = st.ListScheduleRuns(ctx, "s1", 0)
	if len(runs) != 3 || runs[0].State != RunDropped || runs[0].RunID != "" {
		t.Fatalf("a dropped turn is not in the history: %+v", runs)
	}

	// The sweep, and the deletion, take it with them.
	if n, err := st.PurgeScheduleRuns(ctx, now.Add(-24*time.Hour).Unix()); err != nil || n != 0 {
		t.Fatalf("swept too early: %d %v", n, err)
	}
	if err := st.DeleteSchedule(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	runs, _ = st.ListScheduleRuns(ctx, "s1", 0)
	if len(runs) != 0 {
		t.Fatalf("a deleted schedule kept its history: %+v", runs)
	}
}
