package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/cron"
)

// Scheduled calls (SCHED-01).
//
// A schedule is three answers: WHAT to call, WHEN, and AS WHOM.
//
// What is a ROUTE and a path, never a URL. The route is the indirection that
// makes this worth having: a service that moves keeps its schedules, and a
// caller cannot name an address it could not reach through the door anyway.
//
// As whom is the containment, and it is the whole security model. There is NO
// account behind a scheduled call: it is made as "meerkat" carrying the ROLES
// the schedule asks for, and everything in front of the service reads them
// like anybody else's - the route's rule, the per-endpoint rules, the
// organisation's hours, all of it. Nothing is bypassed at four in the morning.
//
// Why roles rather than an account: an account would have to be created for
// every service, and kept in step with the endpoints it calls, for ever. What
// bounds a schedule instead is the TOKEN that wrote it - an administrator's
// credential, minted by root, traced in the audit, revocable in a click - and
// two services that must not reach the same things get two tokens.
//
// What this is NOT, and the line matters because somebody will ask: it is not
// a queue. No fan-out to several consumers, no acknowledgement protocol, no
// dead letters. One schedule, one call, at least once. A service that needs
// the other thing needs a broker, and saying so is what keeps the promise of
// zero dependencies honest for everyone else.

// Schedule is one scheduled call.
type Schedule struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Roles the CALL carries. A scheduled call has no account behind it: it is
	// made as "meerkat" with these roles, and the route's rule, the endpoint
	// rules and the identity forwarded to the service all read them exactly as
	// they read a person's.
	//
	// Asking for a role is not being granted one: what bounds this is the
	// token that wrote the schedule (an administrator's credential, minted by
	// root, traced in the audit). Two services that must not reach the same
	// things get two tokens.
	Roles []string `json:"roles,omitempty"`
	// Role is the same field said in the singular, because one role is the
	// ordinary case and `"roles": ["station_fetch"]` is a plural nobody meant.
	// Folded into Roles on the way in and never written back: there is ONE
	// field in the store, and a reader is never handed two spellings of it.
	Role      string `json:"role,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	TenantID  string `json:"tenantId,omitempty"`
	RouteID   string `json:"routeId"`
	Route     string `json:"route,omitempty"` // route name, filled for reading

	// Metadata is the service's OWN filing system: whatever it needs to find
	// its schedules again - a station, a customer, a batch - without Meerkat
	// having to know what any of it means.
	//
	// Text values, deliberately. What is done with metadata is matching, and
	// matching - equality or an expression - is a question about text. A
	// number written as one reads as one.
	Metadata map[string]string `json:"metadata,omitempty"`

	Method string `json:"method"`
	Path   string `json:"path"`
	// Body is JSON as it was written: an object or an array goes out as it
	// stands (application/json unless ContentType says otherwise), and a JSON
	// STRING goes out as its content - which is how a form, an XML document or
	// anything else that is not JSON is sent.
	Body        json.RawMessage `json:"body,omitempty"`
	ContentType string          `json:"contentType,omitempty"`
	// Headers the call carries beyond the ones the gateway sets. Values may be
	// vault references (`$name`), resolved at the moment of the call - which is
	// how a service's own key reaches it without ever being stored, returned
	// by this API, or drawn in the console.
	//
	// Authorization is NOT among them: on this plane that header is the
	// caller's identity, and the gateway writes it. A service that needs a key
	// of its own reads it from a header of its own.
	Headers map[string]string `json:"headers,omitempty"`

	// WHEN, and there are three ways to say it because they say three
	// different things.
	//
	// Every is an ISO 8601 duration between two runs (PT6H, P1D): a CADENCE,
	// counted from the end of the last run. Cron is a five-field expression:
	// a CALENDAR - "every Monday at three", "the first of the month" - which a
	// duration cannot express and which does not drift, because the next turn
	// is the next occurrence rather than now plus something. At is ONE DATE,
	// RFC 3339 with its offset: the call goes out then, once, and the schedule
	// is over - the delayed action a service reaches for when the trigger is
	// something that happened rather than something on a calendar.
	//
	// One of the three, never two: a schedule that carried two answers to
	// "when" would have to pick one, and whichever it picked would surprise
	// somebody.
	Every   string `json:"every"`
	Cron    string `json:"cron,omitempty"`
	At      string `json:"at,omitempty"`
	StartAt int64  `json:"startAt,omitempty"`

	// Timezone is where the calendar is read - "three in the morning" is a
	// question about WHERE. Empty means UTC - a scheduled call has nobody whose
	// zone it could borrow - and it is kept empty for a cadence and for a
	// single date, where it would mean nothing: a duration is the same length
	// everywhere, and an RFC 3339 date carries its own offset.
	Timezone string `json:"timezone,omitempty"`

	// Overlap says what to do when the previous run has not reported back:
	// skip this turn, or call anyway.
	Overlap string `json:"overlap"`
	// CatchUp is how late a missed run may still go out, in seconds. A
	// gateway that was down all night should not fire twelve hourly runs the
	// moment it wakes up; it should fire the last one, or none.
	CatchUp int64 `json:"catchUp"`
	// Timeout bounds one call (ISO duration), MaxScheduleTimeout at most. The
	// call STARTS the work: a service with a long job answers at once and
	// reports its progress on the run, so the timeout covers the answer, not
	// the job.
	Timeout string `json:"timeout,omitempty"`
	Paused  bool   `json:"paused"`

	// NextAt is the turn owed. While a run is in flight it is the turn that
	// run is for, and nothing more is owed: the next turn is armed when the
	// run closes, counted from then.
	NextAt int64 `json:"nextAt"`

	// The current run and its lease.
	RunID      string `json:"runId,omitempty"`
	RunStarted int64  `json:"runStarted,omitempty"`
	ClaimedBy  string `json:"claimedBy,omitempty"`
	ClaimedAt  int64  `json:"claimedAt,omitempty"`
	Progress   int    `json:"progress"`
	// RunState is where the run in flight stands - RunCalling or RunAccepted
	// - and it is what a lapsed lease is read against. See LapsedRuns.
	RunState string `json:"runState,omitempty"`
	// Attempts is how many times the run has been sent. The last run's count
	// stays once it closes: "done, on the second attempt" is worth reading.
	Attempts int `json:"attempts,omitempty"`
	// RunCause says why this turn goes out - a turn of its own, or a replay
	// somebody asked for - and RunOf what it continues. Posed when the turn
	// is armed, copied into the history when the run ends.
	RunCause string `json:"runCause,omitempty"`
	RunOf    string `json:"runOf,omitempty"`
	// TurnTry is how many attempts the turn now armed has already had. The
	// claim turns it into Attempts, so "attempt 2 of 3" survives the run that
	// ended: a retry is a new run of the SAME turn.
	TurnTry int `json:"turnTry,omitempty"`

	// How the last FINISHED run ended. A run in flight does not touch it.
	LastState  string `json:"lastState,omitempty"`
	LastDetail string `json:"lastDetail,omitempty"`
	LastAt     int64  `json:"lastAt,omitempty"`

	CreatedAt int64 `json:"createdAt,omitempty"`
	UpdatedAt int64 `json:"updatedAt,omitempty"`
}

// RunCredentialID is the id of the token minted for one run. One shape, known
// to the scheduler that mints it and to the store that ends it.
func RunCredentialID(runID string) string { return runCredentialPrefix + runID }

const runCredentialPrefix = "run-"

// IsRunCredential says whether a token is a scheduled call's own, which the
// gateway minted seconds ago and will delete when the run closes.
//
// It exists for one decision: the switch that turns PERSONAL API tokens off
// must not stop the scheduler. That setting is about what people may mint for
// themselves; a run's credential is the gateway's own plumbing, and a gateway
// whose night jobs silently 401 because somebody tightened a policy about
// something else is a gateway nobody can debug. Everything else still applies
// to it - the account's own state, the expiry, the plane.
//
// Safe as a prefix because ids are generated here, never chosen by a caller.
func IsRunCredential(id string) bool { return strings.HasPrefix(id, runCredentialPrefix) }

// Where a run in flight stands (run_state). The difference is the whole
// at-least-once contract: it says what a lapsed lease means.
const (
	// RunCalling is a call sent and not answered yet. A lease lapsing here
	// means the node making it stopped before the answer: nobody knows
	// whether the service got it, so it is sent again, same run identifier.
	RunCalling = "calling"
	// RunAccepted is work the service took: it answered 202, or it has
	// reported on the run since. A lease lapsing here means the SERVICE went
	// quiet, and sending again would only be ignored - the run is lost.
	RunAccepted = "accepted"
)

// How a run ended (last_state), and RunRunning, which is the word a service
// reports progress with.
const (
	RunRunning = "running"
	RunDone    = "done"
	RunFailed  = "failed"
	// RunLost is a run whose lease lapsed with nothing left to try: the
	// service took the work and stopped reporting, or every attempt was cut
	// short. Told apart from a failure on purpose - nobody refused anything.
	RunLost = "lost"
	// RunDropped is a turn that never went out: it came round later than the
	// schedule's catch-up allows, so making the call would have been worse
	// than not making it. Nobody failed here either.
	RunDropped = "dropped"
)

// ScheduleRetentionChoices are how long a finished single date is kept, in
// days: a week to a year. It is housekeeping, not compliance - the row is
// kept so somebody can see that the delayed action went out, and swept when
// nobody is looking at it any more.
var ScheduleRetentionChoices = []int{7, 30, 90, 365}

// DefaultScheduleRetention is a month.
const DefaultScheduleRetention = 30

// SettingScheduleRetention is how many days a finished single date stays
// (SCHED-02).
const SettingScheduleRetention = "scheduleRetention"

// Why a turn went out. A turn of its own is the ordinary case and is stored
// as the empty string, so nothing has to be written for it; the others say
// that somebody, or something, asked for this one.
const (
	// CauseTurn is its own turn coming round - a cadence, a calendar, a date.
	CauseTurn = "turn"
	// CauseManual is "run now": an operator, or a service, brought it forward.
	CauseManual = "manual"
	// CauseReplay is a turn asked for AGAIN, naming the run it replays -
	// typically one that was dropped for being late, or that failed.
	CauseReplay = "replay"
	// CauseRetry is the gateway trying the same turn again by itself, after a
	// failure worth trying again. See the scheduler's Retryable.
	CauseRetry = "retry"
	// CauseAsked is the SERVICE asking to be called again later, because it
	// knows something the gateway cannot - a source not published yet, a
	// dependency missing. It names the run that asked.
	CauseAsked = "asked"
)

// ScheduleRun is one ended turn, as the history keeps it (SCHED-03).
// Attempt says which attempt ended it: 2 means a gateway stopped before the
// answer and another sent the call again.
type ScheduleRun struct {
	ID         string `json:"id"`
	ScheduleID string `json:"scheduleId"`
	// RunID is what the service was told to dedupe on. Empty for a turn that
	// never went out.
	RunID   string `json:"runId,omitempty"`
	Attempt int    `json:"attempt,omitempty"`
	Node    string `json:"node,omitempty"`
	// Cause and OfRun are the chain: why this one went out, and which run it
	// continues.
	Cause     string `json:"cause,omitempty"`
	OfRun     string `json:"ofRun,omitempty"`
	StartedAt int64  `json:"startedAt,omitempty"`
	EndedAt   int64  `json:"endedAt"`
	State     string `json:"state"`
	Status    int    `json:"status,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// RunNext says what to arm once a run is closed: when the next turn is owed,
// and whether it CONTINUES this one - the gateway trying again (CauseRetry),
// or the service asking to be called back later (CauseAsked).
//
// Either way it is a new run - a service that refused with a 403, or asked to
// be called at ten, has not acted on the call, so sending it the same run
// identifier would have it deduplicated away - but the same TURN, which is
// why the attempts travel with it and the history links the two.
type RunNext struct {
	At time.Time
	// Cause is empty for an ordinary next turn.
	Cause string
	// Tries is how many attempts that turn has already had.
	Tries int
}

// RunEnd is how a turn ended: the word, what answered, and the sentence a
// human reads. One argument rather than three, because these three always
// travel together and a positional status among strings is a bug waiting.
type RunEnd struct {
	State  string
	Detail string
	Status int
}

// What a service may ask for when it answers "not now" (SCHED-05), whether
// on the call itself (424 with a Retry-After) or on the report of a job it
// accepted (retryIn).
const (
	// MinAsk is the soonest it may ask to be called back. Below it, a
	// schedule is a client polling, and should call as one.
	MinAsk = time.Minute
	// MaxAsk is the furthest. Past a day, what is being described is a
	// cadence, and the schedule already has one.
	MaxAsk = 24 * time.Hour
	// MaxAsks is how many times in a row a service may push its own turn
	// back. After that the turn is let go and the schedule's own cadence
	// takes over: a service that has asked five times is not waiting for a
	// moment, it is broken, and a screen has to be able to say so.
	MaxAsks = 5
)

// MaxScheduleTimeout is the longest a call may wait for its answer. It is
// below the scheduler's lease on purpose: a call still waiting when its lease
// lapsed would be taken for a dead node's and sent a second time. Work longer
// than this is what the 202 is for.
const MaxScheduleTimeout = 2 * time.Minute

// What to do when a turn arrives and the previous one has not reported.
const (
	OverlapSkip = "skip"
	OverlapRun  = "run"
)

// ScheduleMethods are the verbs a schedule may use. GET and HEAD are absent
// deliberately: a scheduled call is an ACTION, and a schedule that only reads
// is a schedule whose answer nobody looks at.
var ScheduleMethods = []string{"POST", "PUT", "PATCH", "DELETE"}

// SanitizeSchedule fills the defaults and refuses what cannot work, naming
// what is allowed. routeExists answers whether a route id is one this gateway
// serves - the caller passes it because the store must not reach into routing.
func SanitizeSchedule(s *Schedule, routeExists func(string) bool) error {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return fmt.Errorf("a schedule needs a name: it is what the console lists and what a log line says")
	}
	if err := sanitizeRoles(s); err != nil {
		return err
	}
	s.RouteID = strings.TrimSpace(s.RouteID)
	if s.RouteID == "" {
		return fmt.Errorf("a schedule needs a route to call: name the service, not a URL")
	}
	if routeExists != nil && !routeExists(s.RouteID) {
		return fmt.Errorf("no route %q on this gateway: a schedule names a route, so that the service can move without taking its schedules with it", s.RouteID)
	}
	if err := sanitizeMetadata(s.Metadata); err != nil {
		return err
	}
	if err := sanitizeHeaders(s.Headers); err != nil {
		return err
	}
	if len(s.Body) > 0 && !json.Valid(s.Body) {
		return fmt.Errorf("body must be JSON: an object or an array to send as it stands, or a string to send as it is")
	}
	s.Method = strings.ToUpper(strings.TrimSpace(s.Method))
	if s.Method == "" {
		s.Method = "POST"
	}
	if !contains(ScheduleMethods, s.Method) {
		return fmt.Errorf("method %q cannot be scheduled (allowed: %s): a scheduled call is an action, not a read",
			s.Method, strings.Join(ScheduleMethods, ", "))
	}
	s.Path = strings.TrimSpace(s.Path)
	if s.Path == "" {
		s.Path = "/"
	}
	if !strings.HasPrefix(s.Path, "/") {
		return fmt.Errorf("the path must start with /, got %q", s.Path)
	}
	// WHEN: a cadence, a calendar or a single date - one of the three.
	s.Cron = strings.TrimSpace(s.Cron)
	s.Every = strings.TrimSpace(s.Every)
	s.At = strings.TrimSpace(s.At)
	s.Timezone = strings.TrimSpace(s.Timezone)
	if said := saidWhen(s); len(said) > 1 {
		return fmt.Errorf("a schedule says WHEN once, and this one says it %d times (%s): every is a cadence, "+
			"cron a calendar, at a single date - keep the one you meant", len(said), strings.Join(said, ", "))
	}
	switch {
	case s.At != "":
		if _, err := time.Parse(time.RFC3339, s.At); err != nil {
			return fmt.Errorf("at must be one date, RFC 3339 with its offset (2026-10-03T04:00:00Z, "+
				"2026-10-03T06:00:00+02:00), got %q", s.At)
		}
		// A start is the first turn of something that repeats. A single date
		// IS its own first turn, so the two together are two answers again.
		if s.StartAt > 0 {
			return fmt.Errorf("at %q already says when this goes out: startAt places the first turn of a schedule that repeats", s.At)
		}
		// The offset is in the date. A zone beside it is a second answer to
		// the same question, and the two disagree the night the clocks move.
		s.Timezone = ""
	case s.Cron != "":
		if _, err := cron.Parse(s.Cron); err != nil {
			return fmt.Errorf("cron %q: %w", s.Cron, err)
		}
		// A calendar is read somewhere. An unknown zone is refused here rather
		// than silently becoming UTC, which would move every run by an hour
		// and tell nobody.
		if s.Timezone != "" {
			if _, err := time.LoadLocation(s.Timezone); err != nil {
				return fmt.Errorf("timezone %q is not a zone this gateway knows: UTC, or an IANA name such as Europe/Paris", s.Timezone)
			}
		}
	default:
		d, err := ParseISODuration(s.Every)
		if err != nil || d <= 0 {
			return fmt.Errorf("say when: every must be an ISO 8601 duration above zero (PT30M, PT6H, P1D), "+
				"cron a five-field expression (0 3 * * MON), or at a single RFC 3339 date, got every %q", s.Every)
		}
		if d < time.Minute {
			return fmt.Errorf("the shortest cadence is one minute, asked for %s: below that, a schedule is a client and should call as one", s.Every)
		}
		// A zone means nothing to a duration - it is the same length
		// everywhere - and a field that is stored but never read is a field
		// somebody will eventually believe in.
		s.Timezone = ""
	}
	if s.Timeout != "" {
		t, err := ParseISODuration(s.Timeout)
		if err != nil || t <= 0 {
			return fmt.Errorf("timeout must be an ISO 8601 duration above zero, got %q", s.Timeout)
		}
		if t > MaxScheduleTimeout {
			return fmt.Errorf("timeout %s is longer than the %s a call may wait: it bounds the ANSWER, not the work - "+
				"a long job answers 202 at once and reports on its run", s.Timeout, MaxScheduleTimeout)
		}
	}
	if s.Overlap == "" {
		s.Overlap = OverlapSkip
	}
	if s.Overlap != OverlapSkip && s.Overlap != OverlapRun {
		return fmt.Errorf("overlap must be %q or %q, got %q", OverlapSkip, OverlapRun, s.Overlap)
	}
	if s.CatchUp < 0 {
		return fmt.Errorf("catchUp is a number of seconds and cannot be negative")
	}
	if s.Progress < 0 || s.Progress > 100 {
		return fmt.Errorf("progress is a percentage, 0 to 100, got %d", s.Progress)
	}
	return nil
}

// The bounds on metadata. Not a schema - what a service files its schedules
// under is its business - but a size: this is stored on every row, returned in
// every listing, and matched against on every filtered read.
const (
	maxMetaKeys      = 20
	maxMetaKeyLen    = 40
	maxMetaValueLen  = 200
	metaKeyCharsHelp = "letters, digits, and - _ . :"
)

var metaKeyOK = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

func sanitizeMetadata(m map[string]string) error {
	if len(m) > maxMetaKeys {
		return fmt.Errorf("metadata carries %d keys, %d at most: it is how a service finds its schedules again, not where it keeps its data", len(m), maxMetaKeys)
	}
	for k, v := range m {
		switch {
		case k == "":
			return fmt.Errorf("a metadata key cannot be empty (%s)", metaKeyCharsHelp)
		case len(k) > maxMetaKeyLen:
			return fmt.Errorf("metadata key %q is longer than %d characters", k, maxMetaKeyLen)
		case !metaKeyOK.MatchString(k):
			return fmt.Errorf("metadata key %q: %s", k, metaKeyCharsHelp)
		case len(v) > maxMetaValueLen:
			return fmt.Errorf("metadata value for %q is longer than %d characters", k, maxMetaValueLen)
		}
	}
	return nil
}

// The headers a schedule may add, and the ones it may not.
//
// Authorization is the refusal that matters: on this plane it carries the
// CALLER, and the gateway writes it from the account the schedule runs as. A
// schedule that could overwrite it would be a schedule that calls as somebody
// else - the one thing the whole design refuses. Host is the route's business,
// and the gateway's own Meerkat-* headers are how a service tells one turn
// from another.
var reservedHeaders = map[string]string{
	"authorization":       "the gateway writes it: it is the account this schedule runs as",
	"host":                "the route decides it",
	"meerkat-job":         "the gateway writes it",
	"meerkat-job-run":     "the gateway writes it",
	"meerkat-job-attempt": "the gateway writes it",
}

func sanitizeHeaders(h map[string]string) error {
	if len(h) > 10 {
		return fmt.Errorf("a schedule carries %d headers, 10 at most", len(h))
	}
	for name, v := range h {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return fmt.Errorf("a header needs a name")
		}
		if why, taken := reservedHeaders[key]; taken {
			return fmt.Errorf("header %q cannot be set on a schedule: %s", name, why)
		}
		if strings.ContainsAny(name, " \t\r\n:") {
			return fmt.Errorf("header name %q: letters, digits and dashes", name)
		}
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("header %q: a value cannot carry a line break", name)
		}
		if len(v) > 1000 {
			return fmt.Errorf("header %q is longer than 1000 characters", name)
		}
	}
	return nil
}

// saidWhen lists the ways this schedule answers "when". More than one is a
// refusal; the names are what the message quotes back.
func saidWhen(s *Schedule) []string {
	var said []string
	for _, one := range []struct{ name, value string }{
		{"every", s.Every}, {"cron", s.Cron}, {"at", s.At},
	} {
		if one.value != "" {
			said = append(said, one.name+" "+one.value)
		}
	}
	return said
}

// Once reports whether this schedule is a single date: it goes out once and
// is then over.
func (s Schedule) Once() bool { return s.At != "" }

// Moment is the instant a single date names. Sanitize has already refused
// what does not parse, so a zero here is a row written before the rule.
func (s Schedule) Moment() (time.Time, bool) {
	if s.At == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s.At)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Finished reports whether a single date has been and gone: it owes nothing
// more and nothing is in flight. A schedule that repeats is never finished.
func (s Schedule) Finished() bool {
	return s.Once() && s.NextAt == 0 && s.RunID == ""
}

// Interval is the cadence as a duration. Sanitize has already refused what
// does not parse, so a zero here means a row written before the rule existed.
func (s Schedule) Interval() time.Duration {
	d, err := ParseISODuration(s.Every)
	if err != nil {
		return 0
	}
	return d
}

// FirstTurn is when a new schedule is first owed: StartAt when it names one,
// otherwise now.
//
// A CADENCE starts at once - "every six hours" from now is six hours of
// waiting nobody asked for. A CALENDAR owes its next OCCURRENCE instead: a
// nightly job written at noon must not fire at noon, once, before settling
// into the calendar it was given. A SINGLE DATE is its own first turn, even
// when it is already past: the catch-up decides whether it still goes out,
// exactly as it decides for a turn missed while the gateway was down.
func (s Schedule) FirstTurn(now time.Time) int64 {
	if at, ok := s.Moment(); ok {
		return at.Unix()
	}
	from := now
	if s.StartAt > 0 {
		from = time.Unix(s.StartAt, 0)
	}
	if s.Cron == "" {
		return from.Unix()
	}
	// A named start that IS an occurrence counts as one - Next answers
	// strictly after, so it is asked a minute earlier. Not done for "now":
	// there, landing on the current minute would fire a nightly job the
	// instant it was written, which is the surprise FirstTurn exists to avoid.
	if s.StartAt > 0 {
		from = from.Add(-time.Minute)
	}
	next, err := s.NextRun(from)
	if err != nil {
		return from.Unix()
	}
	return next.Unix()
}

// Location is where this schedule's calendar is read. An unknown name falls
// back to UTC rather than stopping the schedule: sanitize refuses one at the
// door, so a bad name here means a zone that was renamed out of the database
// since, and a job that runs an hour off beats a job that stops.
func (s Schedule) Location() *time.Location {
	if s.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// NextRun is when this schedule is owed again after t.
//
// The three shapes answer differently, and the difference is the point. A
// CADENCE counts from the moment given - so from the END of the last run,
// which is what keeps a slow job from firing again the instant it returns. A
// CALENDAR answers the next occurrence, which does not drift and does not care
// how long the last run took. A SINGLE DATE answers NOTHING - a zero time,
// with no error: there is no next turn, and the run that closes ends the
// schedule rather than arming one.
func (s Schedule) NextRun(after time.Time) (time.Time, error) {
	if s.Once() {
		return time.Time{}, nil
	}
	if s.Cron != "" {
		expr, err := cron.Parse(s.Cron)
		if err != nil {
			return time.Time{}, err
		}
		next, ok := expr.Next(after.In(s.Location()))
		if !ok {
			return time.Time{}, fmt.Errorf("%q has no occurrence in the next five years", s.Cron)
		}
		return next, nil
	}
	d := s.Interval()
	if d <= 0 {
		return time.Time{}, fmt.Errorf("neither a cadence nor a calendar: every is %q", s.Every)
	}
	return after.Add(d), nil
}

// Due reports whether this schedule owes a run at t, and whether it is late
// beyond what its catch-up allows.
func (s Schedule) Due(t time.Time) bool {
	return !s.Paused && s.NextAt > 0 && s.NextAt <= t.Unix()
}

// TooLate reports whether the turn owed at NextAt has been missed for longer
// than the schedule accepts. Zero catch-up means "run it whenever you can".
func (s Schedule) TooLate(t time.Time) bool {
	return s.CatchUp > 0 && t.Unix()-s.NextAt > s.CatchUp
}

const scheduleColumns = `id, name, roles, created_by, tenant_id, route_id, method, path, body,
	content_type, headers, metadata, every, cron, at, timezone, start_at, overlap, catch_up, timeout, paused, next_at, run_id, run_started,
	claimed_by, claimed_at, progress, run_state, attempts, run_cause, run_of, turn_try, last_state, last_detail, last_at, created_at, updated_at`

func scanSchedule(row interface{ Scan(...any) error }) (Schedule, error) {
	var s Schedule
	var meta, headers, body, roles string
	err := row.Scan(&s.ID, &s.Name, &roles, &s.CreatedBy, &s.TenantID, &s.RouteID, &s.Method,
		&s.Path, &body, &s.ContentType, &headers, &meta, &s.Every, &s.Cron, &s.At, &s.Timezone, &s.StartAt, &s.Overlap,
		&s.CatchUp, &s.Timeout,
		&s.Paused, &s.NextAt, &s.RunID, &s.RunStarted, &s.ClaimedBy, &s.ClaimedAt, &s.Progress,
		&s.RunState, &s.Attempts, &s.RunCause, &s.RunOf, &s.TurnTry,
		&s.LastState, &s.LastDetail, &s.LastAt, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, err
	}
	if body != "" {
		s.Body = json.RawMessage(body)
	}
	if roles != "" && roles != "[]" {
		if err := json.Unmarshal([]byte(roles), &s.Roles); err != nil {
			return s, fmt.Errorf("store: schedule %q roles: %w", s.ID, err)
		}
	}
	if meta != "" && meta != "{}" {
		if err := json.Unmarshal([]byte(meta), &s.Metadata); err != nil {
			return s, fmt.Errorf("store: schedule %q metadata: %w", s.ID, err)
		}
	}
	if headers != "" && headers != "{}" {
		if err := json.Unmarshal([]byte(headers), &s.Headers); err != nil {
			return s, fmt.Errorf("store: schedule %q headers: %w", s.ID, err)
		}
	}
	return s, nil
}

// rolesJSON is what goes in the column: "" for none, so a schedule asking for
// nothing stores nothing rather than an empty list to special-case.
func rolesJSON(roles []string) string {
	if len(roles) == 0 {
		return ""
	}
	b, err := json.Marshal(roles)
	if err != nil {
		return ""
	}
	return string(b)
}

// metaJSON is what goes in the column: "" for nothing, so a schedule without
// metadata stores no JSON at all rather than an empty object everybody has to
// special-case on the way out.
func metaJSON(m map[string]string) (string, error) {
	if len(m) == 0 {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("store: schedule metadata: %w", err)
	}
	return string(b), nil
}

// CreateSchedule stores a new schedule. The first turn is StartAt, or now.
func (s *Store) CreateSchedule(ctx context.Context, sc Schedule) error {
	now := time.Now().Unix()
	if sc.CreatedAt == 0 {
		sc.CreatedAt = now
	}
	sc.UpdatedAt = now
	if sc.NextAt == 0 {
		sc.NextAt = sc.FirstTurn(time.Unix(now, 0))
	}
	meta, err := metaJSON(sc.Metadata)
	if err != nil {
		return err
	}
	headers, err := metaJSON(sc.Headers)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO schedules (`+scheduleColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.ID, sc.Name, rolesJSON(sc.Roles), sc.CreatedBy, sc.TenantID, sc.RouteID, sc.Method, sc.Path, string(sc.Body),
		sc.ContentType, headers, meta, sc.Every, sc.Cron, sc.At, sc.Timezone, sc.StartAt, sc.Overlap, sc.CatchUp, sc.Timeout,
		sc.Paused, sc.NextAt,
		sc.RunID, sc.RunStarted, sc.ClaimedBy, sc.ClaimedAt, sc.Progress, sc.RunState, sc.Attempts,
		sc.RunCause, sc.RunOf, sc.TurnTry,
		sc.LastState, sc.LastDetail, sc.LastAt, sc.CreatedAt, sc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create schedule %q: %w", sc.ID, err)
	}
	return nil
}

// GetSchedule reads one, sql.ErrNoRows when there is none.
func (s *Store) GetSchedule(ctx context.Context, id string) (Schedule, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+scheduleColumns+` FROM schedules WHERE id = ?`, id)
	sc, err := scanSchedule(row)
	if err != nil {
		return Schedule{}, err
	}
	return sc, nil
}

// ScheduleFilter narrows a listing. Empty fields do not narrow.
type ScheduleFilter struct {
	TenantID string
	RouteID  string
	// Meta narrows on the service's own filing system. All of them must
	// match: two conditions are an AND, which is what somebody asking two
	// questions at once means.
	Meta []MetaMatch
}

// MetaMatch is one condition on one metadata key: equality, or an expression.
type MetaMatch struct {
	Key   string
	Value string
	// Expr is set when the condition was written as an expression. RE2, like
	// every regular expression in Go - linear time, no backtracking - so a
	// filter a service writes cannot become a way to hang the gateway.
	Expr *regexp.Regexp
}

func (m MetaMatch) matches(meta map[string]string) bool {
	v, ok := meta[m.Key]
	if !ok {
		return false
	}
	if m.Expr != nil {
		return m.Expr.MatchString(v)
	}
	return v == m.Value
}

// ParseMetaMatch reads one condition as it arrives in a query string:
// `station=42` is equality, `station=~^st-\d+$` is an expression.
//
// Compiled HERE, so a bad expression is a refusal naming the problem rather
// than a listing that quietly matches nothing.
func ParseMetaMatch(key, value string) (MetaMatch, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return MetaMatch{}, fmt.Errorf("a metadata filter needs a key: meta.<key>=<value>, or meta.<key>=~<expression>")
	}
	if expr, ok := strings.CutPrefix(value, "~"); ok {
		re, err := regexp.Compile(expr)
		if err != nil {
			return MetaMatch{}, fmt.Errorf("metadata filter %s=~%s is not a valid expression: %w", key, expr, err)
		}
		return MetaMatch{Key: key, Expr: re}, nil
	}
	return MetaMatch{Key: key, Value: value}, nil
}

// ListSchedules returns the schedules matching the filter, soonest first.
func (s *Store) ListSchedules(ctx context.Context, f ScheduleFilter) ([]Schedule, error) {
	q := `SELECT ` + scheduleColumns + ` FROM schedules WHERE 1 = 1`
	var args []any
	if f.TenantID != "" {
		q += ` AND tenant_id = ?`
		args = append(args, f.TenantID)
	}
	if f.RouteID != "" {
		q += ` AND route_id = ?`
		args = append(args, f.RouteID)
	}
	q += ` ORDER BY next_at, name`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list schedules: %w", err)
		}
		if !matchesMeta(sc.Metadata, f.Meta) {
			continue
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func matchesMeta(meta map[string]string, conds []MetaMatch) bool {
	for _, c := range conds {
		if !c.matches(meta) {
			return false
		}
	}
	return true
}

// UpdateSchedule rewrites what a writer may change. The run columns are NOT
// among them: they belong to whoever is running the thing.
func (s *Store) UpdateSchedule(ctx context.Context, sc Schedule) error {
	meta, err := metaJSON(sc.Metadata)
	if err != nil {
		return err
	}
	headers, err := metaJSON(sc.Headers)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE schedules SET name = ?, roles = ?, tenant_id = ?, route_id = ?,
		method = ?, path = ?, body = ?, content_type = ?, headers = ?, metadata = ?, every = ?, cron = ?, at = ?,
		timezone = ?, start_at = ?, overlap = ?, catch_up = ?, timeout = ?, paused = ?, next_at = ?,
		updated_at = ? WHERE id = ?`,
		sc.Name, rolesJSON(sc.Roles), sc.TenantID, sc.RouteID, sc.Method, sc.Path, string(sc.Body), sc.ContentType, headers, meta, sc.Every,
		sc.Cron, sc.At, sc.Timezone, sc.StartAt, sc.Overlap, sc.CatchUp, sc.Timeout, sc.Paused, sc.NextAt,
		time.Now().Unix(), sc.ID)
	if err != nil {
		return fmt.Errorf("store: update schedule %q: %w", sc.ID, err)
	}
	return nil
}

// PauseSchedule stops and restarts the turns. A paused schedule keeps its
// next turn: resuming it does not lose where it was in its cadence.
func (s *Store) PauseSchedule(ctx context.Context, id string, paused bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE schedules SET paused = ?, updated_at = ? WHERE id = ?`, paused, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: pause schedule %q: %w", id, err)
	}
	return nil
}

// DeleteSchedule removes one.
func (s *Store) DeleteSchedule(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = ?`, id); err != nil {
		return fmt.Errorf("store: delete schedule %q: %w", id, err)
	}
	// Its history goes with it: rows about a schedule nobody can name any
	// more are rows nobody will ever read.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM schedule_runs WHERE schedule_id = ?`, id); err != nil {
		return fmt.Errorf("store: delete the history of schedule %q: %w", id, err)
	}
	return nil
}

// THE LIFE OF A RUN, and every statement below carries its own condition.
//
// That is what lets several gateways share the table with no lock: two nodes
// asking at the same moment produce one winner and one row untouched, and the
// loser simply finds nothing to do. It is also what lets a call spread over
// the nodes - whoever wakes first takes what is owed - and keeps a slow call
// from holding anybody but itself.
//
//	owed ---claim---> calling ---202 or a report---> accepted
//	                     |                               |
//	                     +------ an answer, or a report saying so ------> closed
//	lease lapsed on calling:  sent again, same run id (RetakeRun), or lost
//	lease lapsed on accepted: lost - the service took it and went quiet
//
// While a run is in flight NOTHING more is owed: the next turn is armed when
// it closes, counted from then. That is what a cadence promises - counted
// from the end of the last run - and it is why a slow job never finds its
// next turn waiting in the past when it returns.

// DueSchedules are the turns owed at t: not paused, due, and not already in
// flight - a schedule owes nothing while its previous run is open.
func (s *Store) DueSchedules(ctx context.Context, t time.Time) ([]Schedule, error) {
	return s.schedulesWhere(ctx, "due schedules",
		`paused = ? AND next_at > 0 AND next_at <= ? AND run_id = '' ORDER BY next_at`, false, t.Unix())
}

// ClaimSchedule takes a turn: the run is opened, calling, first attempt.
// false is the normal answer in a cluster - another node took it first.
func (s *Store) ClaimSchedule(ctx context.Context, id, runID, node string, t time.Time) (bool, error) {
	return s.affected(ctx, "claim schedule "+id, `UPDATE schedules
		SET run_id = ?, run_started = ?, claimed_by = ?, claimed_at = ?, progress = 0,
		    run_state = ?, attempts = turn_try + 1, updated_at = ?
		WHERE id = ? AND paused = ? AND next_at > 0 AND next_at <= ? AND run_id = ''`,
		runID, t.Unix(), node, t.Unix(), RunCalling, t.Unix(),
		id, false, t.Unix())
}

// AcceptRun records that the service took the work: it answered 202. The
// lease starts again from here - the service now has a lease's worth of time
// to say something - and a lapse is no longer a reason to send it again.
func (s *Store) AcceptRun(ctx context.Context, id, runID string, t time.Time) error {
	_, err := s.affected(ctx, "accept run of "+id, `UPDATE schedules
		SET run_state = ?, claimed_at = ?, updated_at = ?
		WHERE id = ? AND run_id = ?`,
		RunAccepted, t.Unix(), t.Unix(), id, runID)
	return err
}

// TouchSchedule renews a lease and records progress. runID is checked so a
// late report from a previous run cannot move the current one. A service
// reporting on a run has it, by definition: the run is accepted from then on.
func (s *Store) TouchSchedule(ctx context.Context, id, runID string, progress int, t time.Time) error {
	_, err := s.affected(ctx, "touch schedule "+id, `UPDATE schedules
		SET claimed_at = ?, progress = ?, run_state = ?, updated_at = ?
		WHERE id = ? AND run_id = ?`, t.Unix(), progress, RunAccepted, t.Unix(), id, runID)
	return err
}

// turnAt is what goes in next_at: the moment, or ZERO when nothing more is
// owed - a single date that has been and gone. Unix() on a zero time is a
// number from the year minus 44, which would read as a turn owed since for
// ever.
func turnAt(next time.Time) int64 {
	if next.IsZero() {
		return 0
	}
	return next.Unix()
}

// FinishSchedule closes a run, writes what it did into the history and arms
// the next turn - or ends the schedule, when it was a single date and there
// is no next. Same runID guard.
func (s *Store) FinishSchedule(ctx context.Context, id, runID string, end RunEnd, next RunNext, t time.Time) error {
	// Read before the update, which wipes what the history needs: which
	// attempt this was, which node held it, and what it continued.
	was, readErr := s.GetSchedule(ctx, id)
	closed, err := s.affected(ctx, "finish schedule "+id, `UPDATE schedules
		SET run_id = '', run_state = '', claimed_by = '', claimed_at = 0, run_cause = '', run_of = '',
		    turn_try = 0, last_state = ?, last_detail = ?, last_at = ?, next_at = ?, updated_at = ?
		WHERE id = ? AND run_id = ?`,
		end.State, clip(end.Detail), t.Unix(), turnAt(next.At), t.Unix(), id, runID)
	if err != nil {
		return err
	}
	if closed && readErr == nil {
		row := s.recordRun(ctx, was, end, t)
		// A turn that continues this one: it carries the attempts already
		// made, and the history says which run it follows.
		if next.Cause != "" {
			if _, err := s.affected(ctx, "arm what follows the run of "+id,
				`UPDATE schedules SET run_cause = ?, run_of = ?, turn_try = ?, updated_at = ?
				 WHERE id = ? AND run_id = ''`,
				next.Cause, row, next.Tries, t.Unix(), id); err != nil {
				slog.Warn("store: arming what follows a run", "schedule", id, "err", err)
			}
		}
	}
	// A closed run has no credential. Here rather than in the caller, because
	// a run closes from three places - the answer, the service's own report,
	// and a lapsed lease - and a token outliving its run in any of them is a
	// secret nobody is watching any more.
	_ = s.DeleteAPIToken(ctx, RunCredentialID(runID))
	return nil
}

// LapsedRuns are the runs nobody is saying anything about any more: the
// lease ran out, or the node holding the run handed it back as it stopped
// (claimed_at 0). What happens to each depends on its state - see RetakeRun
// and AbandonRun - so they are returned rather than settled in one statement.
func (s *Store) LapsedRuns(ctx context.Context, t time.Time, lease time.Duration) ([]Schedule, error) {
	return s.schedulesWhere(ctx, "lapsed runs",
		`run_id <> '' AND claimed_at <= ?`, t.Add(-lease).Unix())
}

// RetakeRun is the at-least-once half: a call whose node stopped before the
// answer is taken by this one, to be sent AGAIN under the same run id - the
// identifier a service dedupes on. Only a call still waiting for its answer
// qualifies, and only while its lease is still lapsed, so two nodes seeing the
// same row produce one attempt.
func (s *Store) RetakeRun(ctx context.Context, id, runID, node string, t time.Time, lease time.Duration) (bool, error) {
	return s.affected(ctx, "retake run of "+id, `UPDATE schedules
		SET claimed_by = ?, claimed_at = ?, attempts = attempts + 1, updated_at = ?
		WHERE id = ? AND run_id = ? AND run_state = ? AND claimed_at <= ?`,
		node, t.Unix(), t.Unix(), id, runID, RunCalling, t.Add(-lease).Unix())
}

// AbandonRun closes a lapsed run as lost and arms the next turn. The lapse is
// part of the condition: a run another node has just taken again is not one
// to close under it.
func (s *Store) AbandonRun(ctx context.Context, id, runID, detail string, next, t time.Time, lease time.Duration) (bool, error) {
	was, readErr := s.GetSchedule(ctx, id)
	closed, err := s.affected(ctx, "abandon run of "+id, `UPDATE schedules
		SET run_id = '', run_state = '', claimed_by = '', claimed_at = 0, run_cause = '', run_of = '',
		    turn_try = 0, last_state = ?, last_detail = ?, last_at = ?, next_at = ?, updated_at = ?
		WHERE id = ? AND run_id = ? AND claimed_at <= ?`,
		RunLost, clip(detail), t.Unix(), turnAt(next), t.Unix(), id, runID, t.Add(-lease).Unix())
	if closed && err == nil && readErr == nil {
		s.recordRun(ctx, was, RunEnd{State: RunLost, Detail: detail}, t)
	}
	return closed, err
}

// ReleaseRun hands a call back without waiting for its lease: the node making
// it is stopping, and would rather another one sent it again NOW than in five
// minutes. The run keeps its id and its attempts - it is the same run.
func (s *Store) ReleaseRun(ctx context.Context, id, runID string, t time.Time) error {
	_, err := s.affected(ctx, "release run of "+id, `UPDATE schedules
		SET claimed_by = '', claimed_at = 0, updated_at = ?
		WHERE id = ? AND run_id = ? AND run_state = ?`,
		t.Unix(), id, runID, RunCalling)
	return err
}

// ArmSchedule moves the next turn - "run now", a replay, or a turn later than
// its catch-up allows. Refused while a run is in flight (false): that turn is
// the run's own until it closes, and the close arms the next one anyway.
//
// cause and ofRun ride along to the run that follows, so the history can say
// why it went out and what it continues. Empty is an ordinary turn.
func (s *Store) ArmSchedule(ctx context.Context, id string, next time.Time, cause, ofRun string) (bool, error) {
	return s.affected(ctx, "arm schedule "+id,
		`UPDATE schedules SET next_at = ?, run_cause = ?, run_of = ?, turn_try = 0, updated_at = ?
		 WHERE id = ? AND run_id = ''`,
		turnAt(next), cause, ofRun, time.Now().Unix(), id)
}

// DropTurn gives up a turn that is later than the schedule's catch-up allows
// and arms the next one - or ends a single date, which has no next. The turn
// itself is part of the condition: a "run now" or a claim that landed between
// the listing and here is not the turn this was about.
//
// It is recorded on the row rather than only in a log line: a turn that never
// went out is exactly what an operator comes looking for the morning after.
func (s *Store) DropTurn(ctx context.Context, id string, due, next, t time.Time, detail string) (bool, error) {
	was, readErr := s.GetSchedule(ctx, id)
	dropped, err := s.affected(ctx, "drop turn of "+id, `UPDATE schedules
		SET next_at = ?, last_state = ?, last_detail = ?, last_at = ?, run_cause = '', run_of = '',
		    turn_try = 0, updated_at = ?
		WHERE id = ? AND run_id = '' AND next_at = ?`,
		turnAt(next), RunDropped, clip(detail), t.Unix(), t.Unix(), id, due.Unix())
	if dropped && err == nil && readErr == nil {
		// A turn that never went out has no run identifier and no node: what
		// it has is the moment it was owed, which is what a replay needs.
		was.RunID, was.ClaimedBy, was.Attempts, was.RunStarted = "", "", 0, due.Unix()
		s.recordRun(ctx, was, RunEnd{State: RunDropped, Detail: detail}, t)
	}
	return dropped, err
}

// NextWake is the next moment anything is owed: a turn coming due, or a
// lease running out. It is what lets the scheduler sleep until then instead
// of looking every few seconds. ok is false when nothing is scheduled at all.
func (s *Store) NextWake(ctx context.Context, lease time.Duration) (time.Time, bool, error) {
	var turn, lapse sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(next_at) FROM schedules
		WHERE paused = ? AND next_at > 0 AND run_id = ''`, false).Scan(&turn); err != nil {
		return time.Time{}, false, fmt.Errorf("store: next turn: %w", err)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT MIN(claimed_at) FROM schedules WHERE run_id <> ''`).Scan(&lapse); err != nil {
		return time.Time{}, false, fmt.Errorf("store: next lease: %w", err)
	}
	var at time.Time
	if turn.Valid {
		at = time.Unix(turn.Int64, 0)
	}
	if lapse.Valid {
		if end := time.Unix(lapse.Int64, 0).Add(lease); at.IsZero() || end.Before(at) {
			at = end
		}
	}
	return at, !at.IsZero(), nil
}

// recordRun writes one ended attempt into the history. Called from the three
// places a turn ends - an answer, the service's own report, and a lapse or a
// drop - and from INSIDE them, so no call site can forget one.
//
// Best effort, deliberately: a history that failed to write must not fail the
// close it describes, or a gateway with a full disk would start leaving runs
// open for ever.
func (s *Store) recordRun(ctx context.Context, was Schedule, end RunEnd, t time.Time) string {
	cause := was.RunCause
	if cause == "" {
		cause = CauseTurn
	}
	row := newRunRowID()
	_, err := s.db.ExecContext(ctx, `INSERT INTO schedule_runs
		(id, schedule_id, run_id, attempt, node, cause, of_run, started_at, ended_at, state, status, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row, was.ID, was.RunID, was.Attempts, was.ClaimedBy, cause, was.RunOf,
		was.RunStarted, t.Unix(), end.State, end.Status, clip(end.Detail))
	if err != nil {
		slog.Warn("store: a run was not recorded in the history", "schedule", was.ID, "err", err)
		return ""
	}
	return row
}

// newRunRowID names one history row, and it SORTS: the moment first, in a
// fixed width, then randomness. ended_at is a second, and two runs of the
// same schedule inside one second are ordinary - a call that answers in
// milliseconds, a replay asked for straight away - so the id is what puts
// them in the order they happened.
func newRunRowID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("run-%016x-%s", uint64(time.Now().UnixNano()), hex.EncodeToString(b[:]))
}

// ListScheduleRuns is the history of one schedule, newest first.
func (s *Store) ListScheduleRuns(ctx context.Context, scheduleID string, limit int) ([]ScheduleRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, schedule_id, run_id, attempt, node, cause, of_run,
		started_at, ended_at, state, status, detail FROM schedule_runs
		WHERE schedule_id = ? ORDER BY ended_at DESC, id DESC LIMIT ?`, scheduleID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: schedule runs of %q: %w", scheduleID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []ScheduleRun
	for rows.Next() {
		var r ScheduleRun
		if err := rows.Scan(&r.ID, &r.ScheduleID, &r.RunID, &r.Attempt, &r.Node, &r.Cause, &r.OfRun,
			&r.StartedAt, &r.EndedAt, &r.State, &r.Status, &r.Detail); err != nil {
			return nil, fmt.Errorf("store: schedule runs of %q: %w", scheduleID, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetScheduleRun reads one attempt of one schedule, by the identifier the
// history gave it. sql.ErrNoRows when there is none.
func (s *Store) GetScheduleRun(ctx context.Context, scheduleID, id string) (ScheduleRun, error) {
	var r ScheduleRun
	err := s.db.QueryRowContext(ctx, `SELECT id, schedule_id, run_id, attempt, node, cause, of_run,
		started_at, ended_at, state, status, detail FROM schedule_runs
		WHERE schedule_id = ? AND id = ?`, scheduleID, id).
		Scan(&r.ID, &r.ScheduleID, &r.RunID, &r.Attempt, &r.Node, &r.Cause, &r.OfRun,
			&r.StartedAt, &r.EndedAt, &r.State, &r.Status, &r.Detail)
	if err != nil {
		return ScheduleRun{}, err
	}
	return r, nil
}

// PurgeScheduleRuns sweeps the history past the retention.
func (s *Store) PurgeScheduleRuns(ctx context.Context, before int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM schedule_runs WHERE ended_at <= ?`, before)
	if err != nil {
		return 0, fmt.Errorf("store: purge schedule runs: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) schedulesWhere(ctx context.Context, what, where string, args ...any) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+scheduleColumns+` FROM schedules WHERE `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("store: %s: %w", what, err)
	}
	defer func() { _ = rows.Close() }()
	var out []Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", what, err)
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// affected runs a conditional UPDATE and says whether it took.
func (s *Store) affected(ctx context.Context, what, query string, args ...any) (bool, error) {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("store: %s: %w", what, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: %s: %w", what, err)
	}
	return n > 0, nil
}

// clip keeps a run's detail to what a row and a screen can hold.
func clip(detail string) string {
	if len(detail) > 500 {
		return detail[:500]
	}
	return detail
}

// SchedulesForRoute answers the one question the route editor asks: is
// anything scheduled against this service? Deleting a route with schedules on
// it is how a schedule becomes a call to nowhere.
func (s *Store) SchedulesForRoute(ctx context.Context, routeID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schedules WHERE route_id = ?`, routeID).Scan(&n)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("store: schedules for route %q: %w", routeID, err)
	}
	return n, nil
}

// maxScheduleRoles bounds what one call may ask to carry. A call needs one
// role, sometimes two; a list of fifty is somebody pasting a catalogue.
const maxScheduleRoles = 10

// sanitizeRoles folds the singular into the plural and leaves ONE clean list:
// trimmed, without blanks, without repeats, in the order it was written.
//
// `role` and `roles` together is refused rather than merged: they are two
// answers to the same question, and picking one would surprise whoever wrote
// the other.
func sanitizeRoles(s *Schedule) error {
	s.Role = strings.TrimSpace(s.Role)
	if s.Role != "" {
		if len(s.Roles) > 0 {
			return fmt.Errorf("say the roles once: role %q and roles %v are the same field - keep the one you meant", s.Role, s.Roles)
		}
		s.Roles = []string{s.Role}
		s.Role = ""
	}
	if len(s.Roles) == 0 {
		s.Roles = nil
		return nil
	}
	seen := make(map[string]bool, len(s.Roles))
	clean := make([]string, 0, len(s.Roles))
	for _, r := range s.Roles {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		clean = append(clean, r)
	}
	if len(clean) > maxScheduleRoles {
		return fmt.Errorf("a scheduled call carries %d roles, %d at most: it asks for what one endpoint needs, it does not carry a catalogue", len(clean), maxScheduleRoles)
	}
	if len(clean) == 0 {
		clean = nil
	}
	s.Roles = clean
	return nil
}

// ScheduleRetentionDays reads the retention, the default when unset or
// invalid.
func (s *Store) ScheduleRetentionDays(ctx context.Context) int {
	var d int
	if err := s.GetSetting(ctx, SettingScheduleRetention, &d); err != nil || !slices.Contains(ScheduleRetentionChoices, d) {
		return DefaultScheduleRetention
	}
	return d
}

// SetScheduleRetentionDays stores it, refusing a lifetime that is not offered.
func (s *Store) SetScheduleRetentionDays(ctx context.Context, days int) error {
	if !slices.Contains(ScheduleRetentionChoices, days) {
		return fmt.Errorf("a finished schedule is kept for one of %v days, asked for %d", ScheduleRetentionChoices, days)
	}
	return s.SetSetting(ctx, SettingScheduleRetention, days)
}

// PurgeFinishedSchedules sweeps the single dates that have been and gone,
// and the history they leave behind. Only those: a schedule that repeats is
// never finished, and one still owing a turn or holding a run is not either.
func (s *Store) PurgeFinishedSchedules(ctx context.Context, before int64) (int64, error) {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM schedule_runs WHERE schedule_id IN (
		SELECT id FROM schedules WHERE at <> '' AND next_at = 0 AND run_id = '' AND updated_at <= ?)`,
		before); err != nil {
		return 0, fmt.Errorf("store: purge the history of finished schedules: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM schedules
		WHERE at <> '' AND next_at = 0 AND run_id = '' AND updated_at <= ?`, before)
	if err != nil {
		return 0, fmt.Errorf("store: purge finished schedules: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
