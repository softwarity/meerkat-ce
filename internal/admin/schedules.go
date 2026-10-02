package admin

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/store"
)

// The scheduled calls (SCHED-01), and this is the WHOLE API for them.
//
// They live on the control plane, and that placement is the design. A schedule
// is a service the gateway provides - like the agent endpoint - not
// something an application exposes to its users: a browser never talks to it.
// A backend does, with a token, and hands its own users whatever it decides to
// show them.
//
// WHO THE CALLS RUN AS is in the schedule, and it is not an account: a
// scheduled call is made as "meerkat" carrying the ROLES the schedule asks
// for. No service account to create, none to keep in step with the endpoints
// it calls - the schedule names the roles its call needs, and the route's rule
// reads them like anybody else's.
//
// Asking for a role is not being granted one: what bounds it is holding this
// token at all. It is an administrator's credential, minted by root, traced in
// the audit, revocable in a click - and two services that must not reach the
// same things get two tokens.
//
// THE IDS ARE OURS. A service that named its own would collide with the next
// service that likes the same word, and the second write would take the first
// one's schedule, roles included. POST hands back an id; a service that would
// rather not keep it files the schedule under its own metadata and asks for it
// back that way.
//
// The token's perimeter (MCP-02, ScopeSchedules) is what keeps that safe to
// hand out: it opens this API and nothing else on this port.
//
// The CONSOLE reads the same endpoints and creates nothing: pause it, run it
// now, remove it - the three things wanted at two in the morning.
const schedulesPath = "/api/schedules"

func (a *API) registerSchedules(mux Mux) {
	mux.Handle("GET /api/schedules", a.schedules(a.listSchedules))
	mux.Handle("POST /api/schedules", a.schedules(a.createSchedule))
	mux.Handle("GET /api/schedules/{id}", a.schedules(a.getSchedule))
	mux.Handle("PUT /api/schedules/{id}", a.schedules(a.putSchedule))
	mux.Handle("POST /api/schedules/{id}/pause", a.schedules(a.pauseSchedule))
	mux.Handle("POST /api/schedules/{id}/resume", a.schedules(a.resumeSchedule))
	mux.Handle("POST /api/schedules/{id}/run", a.schedules(a.runSchedule))
	// What each turn did (SCHED-03): the row keeps the last result, this
	// keeps them all - including the turns that never went out.
	mux.Handle("GET /api/schedules/{id}/runs", a.schedules(a.listScheduleRuns))
	// What a service reports on a run it answered 202 to: how far it has got,
	// and that it is done. Same door as the rest, same token.
	mux.Handle("PATCH /api/schedules/{id}/run", a.schedules(a.reportRun))
	mux.Handle("DELETE /api/schedules/{id}", a.schedules(a.deleteSchedule))
	// How long a finished delayed action is kept before the sweep (SCHED-02).
	// Beside the trail's own retention, root's alone for the same reason: it
	// decides how long the gateway remembers what it did.
	mux.Handle("GET /api/settings/schedules", a.rootOnly(a.getScheduleSettings))
	mux.Handle("PUT /api/settings/schedules", a.rootOnly(a.putScheduleSettings))
}

// scheduleSettings is the housekeeping the scheduler has, and what may be
// chosen. A single date that has been and gone is kept this long so somebody
// can see that it went out, then swept.
type scheduleSettings struct {
	RetentionDays int   `json:"retentionDays"`
	Choices       []int `json:"choices"`
}

func (a *API) getScheduleSettings(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, scheduleSettings{
		RetentionDays: a.st.ScheduleRetentionDays(r.Context()), Choices: store.ScheduleRetentionChoices,
	})
}

func (a *API) putScheduleSettings(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p scheduleSettings
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed schedule settings: "+err.Error())
		return
	}
	before := a.st.ScheduleRetentionDays(r.Context())
	if err := a.st.SetScheduleRetentionDays(r.Context(), p.RetentionDays); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.auditUpdate(r.Context(), actor, "schedule.retention", "settings", "", "", "",
		map[string]int{"retentionDays": before}, map[string]int{"retentionDays": p.RetentionDays})
	a.getScheduleSettings(w, r, actor)
}

// schedules admits the two callers this API has: an application
// administrator, and a token whose perimeter is exactly this.
func (a *API) schedules(next userHandler) http.Handler {
	return a.authed(func(w http.ResponseWriter, r *http.Request, actor store.User) {
		if actor.Root || actor.AppAdmin || tokenScope(r.Context()) == store.ScopeSchedules {
			next(w, r, actor)
			return
		}
		writeErr(w, http.StatusForbidden,
			"the scheduled calls answer an application administrator, or a token whose perimeter is "+
				store.ScopeSchedules)
	})
}

func (a *API) listSchedules(w http.ResponseWriter, r *http.Request, _ store.User) {
	q := r.URL.Query()
	f := store.ScheduleFilter{
		TenantID: strings.TrimSpace(q.Get("tenant")),
		RouteID:  strings.TrimSpace(q.Get("route")),
	}
	// meta.<key>=<value>, or meta.<key>=~<expression>. This is how one token
	// serves a fleet: the service files its schedules under whatever it knows
	// them by, and asks for them back the same way.
	for key, values := range q {
		name, ok := strings.CutPrefix(key, "meta.")
		if !ok {
			continue
		}
		for _, v := range values {
			m, err := store.ParseMetaMatch(name, v)
			if err != nil {
				writeErr(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			f.Meta = append(f.Meta, m)
		}
	}
	list, err := a.st.ListSchedules(r.Context(), f)
	if err != nil {
		a.internal(w, err)
		return
	}
	if list == nil {
		list = []store.Schedule{}
	}
	writeJSON(w, http.StatusOK, a.nameSchedules(r, list))
}

func (a *API) getSchedule(w http.ResponseWriter, r *http.Request, _ store.User) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, a.nameSchedules(r, []store.Schedule{sc})[0])
}

// createSchedule stores a new one and hands back its id.
//
// The id is OURS, not the caller's: an id a service chooses is an id two
// services choose, and the second write would silently take the first one's
// schedule - roles included. A service that needs to find this schedule again
// without keeping the id files it under its own metadata and asks for it back
// that way, which is what the metadata is for.
func (a *API) createSchedule(w http.ResponseWriter, r *http.Request, actor store.User) {
	in, ok := a.readSchedule(w, r, actor, store.Schedule{})
	if !ok {
		return
	}
	in.ID = newScheduleID()
	in.CreatedAt = 0
	in.NextAt = in.FirstTurn(time.Now())
	if err := a.st.CreateSchedule(r.Context(), in); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "schedule.create", "schedule", in.ID, in.Name, "", rolesWords(in.Roles))
	a.schedulesMoved(r.Context())
	writeJSON(w, http.StatusCreated, in)
}

// putSchedule replaces one that exists. An unknown id is a 404 rather than a
// creation: the ids are ours, so one that was never handed out is one nobody
// should be writing to.
func (a *API) putSchedule(w http.ResponseWriter, r *http.Request, actor store.User) {
	stored, ok := a.schedule(w, r)
	if !ok {
		return
	}
	in, ok := a.readSchedule(w, r, actor, stored)
	if !ok {
		return
	}
	in.ID, in.CreatedAt = stored.ID, stored.CreatedAt
	// WHEN changed, so the turn owed under the old answer is not owed under
	// the new one. A single date moved this way is how a delayed action is
	// pushed back - and how a finished one is given another date.
	if in.Every != stored.Every || in.Cron != stored.Cron || in.At != stored.At || in.Timezone != stored.Timezone {
		in.NextAt = in.FirstTurn(time.Now())
	} else {
		in.NextAt = stored.NextAt
	}
	if err := a.st.UpdateSchedule(r.Context(), in); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "schedule.update", "schedule", in.ID, in.Name, "", rolesWords(in.Roles))
	a.schedulesMoved(r.Context())
	writeJSON(w, http.StatusOK, in)
}

// readSchedule decodes what was sent and refuses what cannot work - the half
// the two writers have in common.
func (a *API) readSchedule(w http.ResponseWriter, r *http.Request, actor store.User, stored store.Schedule) (store.Schedule, bool) {
	var in store.Schedule
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return in, false
	}
	in.CreatedBy = actor.Username
	// The run belongs to whoever is running it, never to the writer.
	in.RunID, in.ClaimedBy, in.ClaimedAt, in.Progress = stored.RunID, stored.ClaimedBy, stored.ClaimedAt, stored.Progress
	in.RunStarted, in.RunState, in.Attempts = stored.RunStarted, stored.RunState, stored.Attempts
	in.LastState, in.LastDetail, in.LastAt = stored.LastState, stored.LastDetail, stored.LastAt
	if err := store.SanitizeSchedule(&in, a.routeExists(r)); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return in, false
	}
	return in, true
}

// newScheduleID: ours, opaque, and recognisable in a log line.
func newScheduleID() string { return "sch_" + newID() }

// rolesWords is what the audit records about a schedule's reach: the roles its
// calls will carry, or the fact that they carry none.
func rolesWords(roles []string) string {
	if len(roles) == 0 {
		return "calls as " + gateway.ScheduledUser + ", no role"
	}
	return "calls as " + gateway.ScheduledUser + " with " + strings.Join(roles, ", ")
}

// routeExists answers the sanitiser's one question about the outside world.
func (a *API) routeExists(r *http.Request) func(string) bool {
	return func(id string) bool {
		_, err := a.st.GetRoute(r.Context(), id)
		return err == nil
	}
}

// reportRun is the other half of a 202: the service says how far it has got,
// and eventually that it is done. The run identifier is checked, so a late
// report from a previous turn cannot close the current one.
func (a *API) reportRun(w http.ResponseWriter, r *http.Request, _ store.User) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	var in struct {
		Run      string `json:"run"`
		State    string `json:"state"`
		Progress int    `json:"progress"`
		Detail   string `json:"detail"`
		// "Not now, call me back then" for a job that was accepted: the same
		// answer a synchronous call gives with 424 and a Retry-After.
		RetryIn string `json:"retryIn"`
	}
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	if in.Run == "" {
		in.Run = r.Header.Get("Meerkat-Job-Run")
	}
	if sc.RunID == "" || in.Run != sc.RunID {
		writeErr(w, http.StatusConflict,
			"this run is not the one in flight: a report names the run it was given")
		return
	}
	if in.Progress < 0 || in.Progress > 100 {
		writeErr(w, http.StatusUnprocessableEntity, "progress is a percentage, 0 to 100")
		return
	}
	now := time.Now()
	switch in.State {
	case "", store.RunRunning:
		// Progress renews the lease and nothing else: the other nodes need
		// not hear of it - a lease pushed later only means one of them wakes
		// up for nothing, finds nothing lapsed, and sleeps again.
		if err := a.st.TouchSchedule(r.Context(), sc.ID, in.Run, in.Progress, now); err != nil {
			a.internal(w, err)
			return
		}
	case store.RunDone, store.RunFailed:
		next := now.Add(time.Hour)
		if at, e := sc.NextRun(now); e == nil {
			next = at
		}
		plan := store.RunNext{At: next}
		end := store.RunEnd{State: in.State, Detail: in.Detail}
		if in.RetryIn != "" {
			if in.State != store.RunFailed {
				writeErr(w, http.StatusUnprocessableEntity,
					"retryIn goes with a failed run: a job that is done needs no second call")
				return
			}
			d, err := store.ParseISODuration(in.RetryIn)
			if err != nil || d <= 0 {
				writeErr(w, http.StatusUnprocessableEntity,
					"retryIn must be an ISO 8601 duration above zero (PT10M), got "+in.RetryIn)
				return
			}
			if sc.TurnTry >= store.MaxAsks {
				// Asked over and over: the turn is let go, and the cadence
				// takes over. Said on the run, so a screen can show it.
				end.Detail += fmt.Sprintf(" (asked to be called back %d times in a row: letting the turn go)", sc.TurnTry)
			} else {
				plan = store.RunNext{
					At:    askedAt(now, d),
					Cause: store.CauseAsked,
					Tries: sc.TurnTry + 1,
				}
			}
		}
		if err := a.st.FinishSchedule(r.Context(), sc.ID, in.Run, end, plan, now); err != nil {
			a.internal(w, err)
			return
		}
		// A closed run arms the next turn: whoever sleeps until then has to
		// know it moved.
		a.schedulesMoved(r.Context())
	default:
		writeErr(w, http.StatusUnprocessableEntity,
			"state must be empty, "+store.RunRunning+", "+store.RunDone+" or "+store.RunFailed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) pauseSchedule(w http.ResponseWriter, r *http.Request, actor store.User) {
	a.setSchedulePaused(w, r, actor, true)
}

func (a *API) resumeSchedule(w http.ResponseWriter, r *http.Request, actor store.User) {
	a.setSchedulePaused(w, r, actor, false)
}

func (a *API) setSchedulePaused(w http.ResponseWriter, r *http.Request, actor store.User, paused bool) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	if err := a.st.PauseSchedule(r.Context(), sc.ID, paused); err != nil {
		a.internal(w, err)
		return
	}
	verb := "schedule.resume"
	if paused {
		verb = "schedule.pause"
	}
	a.auditEvent(r.Context(), actor, verb, "schedule", sc.ID, sc.Name, "", "")
	// Resuming one whose turn came round while it was paused fires it now.
	a.schedulesMoved(r.Context())
	sc.Paused = paused
	writeJSON(w, http.StatusOK, sc)
}

// listScheduleRuns is the history of one schedule, newest first (SCHED-03).
// The row itself only keeps the LAST result, which answers "how did it go"
// but never "since when is it failing" or "did last night's close run".
func (a *API) listScheduleRuns(w http.ResponseWriter, r *http.Request, _ store.User) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := a.st.ListScheduleRuns(r.Context(), sc.ID, limit)
	if err != nil {
		a.internal(w, err)
		return
	}
	if runs == nil {
		runs = []store.ScheduleRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

// runSchedule brings the next turn forward, on its own or as the REPLAY of
// one the history kept - a turn dropped for being late, a call that failed.
// It calls nothing itself: the scheduler does, which is the only place that
// knows about leases and slots - and whichever node wakes first makes it.
//
// A replay is a new run with a new identifier: the schedule's payload is what
// goes out, as it stands today, and what it replays is written in the history
// so the two read as one chain.
func (a *API) runSchedule(w http.ResponseWriter, r *http.Request, actor store.User) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	var in struct {
		ReplayOf string `json:"replayOf"`
	}
	// The body is optional: "run now" says nothing, a replay names a run.
	if r.ContentLength > 0 {
		if err := decodeStrict(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
			return
		}
	}
	if sc.Paused {
		writeErr(w, http.StatusConflict, "this schedule is paused: resume it before asking for a run")
		return
	}
	cause, of := store.CauseManual, ""
	if in.ReplayOf != "" {
		past, err := a.st.GetScheduleRun(r.Context(), sc.ID, in.ReplayOf)
		if err != nil {
			writeErr(w, http.StatusNotFound,
				"no run "+in.ReplayOf+" in this schedule's history: replay names one of its own runs")
			return
		}
		cause, of = store.CauseReplay, past.ID
	}
	armed, err := a.st.ArmSchedule(r.Context(), sc.ID, time.Now(), cause, of)
	if err != nil {
		a.internal(w, err)
		return
	}
	if !armed {
		writeErr(w, http.StatusConflict, runInFlight)
		return
	}
	what := ""
	if in.ReplayOf != "" {
		what = "replay of " + in.ReplayOf
	}
	a.auditEvent(r.Context(), actor, "schedule.run", "schedule", sc.ID, sc.Name, "", what)
	// "Now" means now: this node wakes, and the others hear it.
	a.schedulesMoved(r.Context())
	sc.NextAt = time.Now().Unix()
	writeJSON(w, http.StatusAccepted, sc)
}

func (a *API) deleteSchedule(w http.ResponseWriter, r *http.Request, actor store.User) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	if err := a.st.DeleteSchedule(r.Context(), sc.ID); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "schedule.delete", "schedule", sc.ID, sc.Name, "", "")
	a.schedulesMoved(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) schedule(w http.ResponseWriter, r *http.Request) (store.Schedule, bool) {
	sc, err := a.st.GetSchedule(r.Context(), r.PathValue("id"))
	if err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusNotFound, "no schedule "+r.PathValue("id"))
		} else {
			a.internal(w, err)
		}
		return store.Schedule{}, false
	}
	return sc, true
}

// nameSchedules fills the route's NAME, which a row carries as an id and a
// screen shows as a word.
func (a *API) nameSchedules(r *http.Request, list []store.Schedule) []store.Schedule {
	routes := map[string]string{}
	for i, sc := range list {
		if name, ok := routes[sc.RouteID]; ok {
			list[i].Route = name
		} else if rt, err := a.st.GetRoute(r.Context(), sc.RouteID); err == nil {
			routes[sc.RouteID] = rt.Name
			list[i].Route = rt.Name
		}
	}
	return list
}

// schedulesMoved is what every schedule write does once it has landed, and
// the ONE place it is done: this node's scheduler looks now, and the other
// nodes hear it through the change bus.
//
// Through the BUS, never a bare notification: a node reacts to the version
// the bus bumps in the table, so a NOTIFY with no version behind it rings a
// bell nobody answers - which is what this did, from every write, until
// reload_test.go started refusing it. Best effort either way: the table is
// the truth, and a node nobody told finds the change at its backstop.
//
// A nil Scheduler is a gateway built without one: nobody to wake.
func (a *API) schedulesMoved(ctx context.Context) {
	a.announce(ctx, store.TopicSchedules)
	if a.Scheduler != nil {
		a.Scheduler.Wake()
	}
}

// askedAt bounds what a service asks for at both ends, and CLAMPS rather than
// refuses: one that says "in five seconds" gets a minute, one that says "in a
// fortnight" gets a day. Refusing would turn a reasonable answer into a lost
// turn.
func askedAt(now time.Time, in time.Duration) time.Time {
	switch {
	case in < store.MinAsk:
		return now.Add(store.MinAsk)
	case in > store.MaxAsk:
		return now.Add(store.MaxAsk)
	default:
		return now.Add(in)
	}
}

// runInFlight is the refusal to bring a turn forward while a run is open.
// That turn is the run's own until it closes, and the close arms the next.
const runInFlight = "a run of this schedule is in flight: nothing more is owed until it closes, " +
	"and its close arms the next turn - wait for it, or pause the schedule"
