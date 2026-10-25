package admin

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/store"
)

// The scheduled calls (SCHED-01), and this is the WHOLE API for them.
//
// They live on the internal plane (internalplane.go), and that placement is
// the design. A schedule is a service the gateway provides to the backends
// behind it, not something an application exposes to its users: a browser
// never talks to it. A backend does, with a token, and hands its own users
// whatever it decides to show them. The control plane keeps the half the
// console needs - read, pause, resume, run now, delete - and writes none.
//
// WHO THE CALLS RUN AS is in the schedule, and it is not an account: a
// scheduled call is made as "meerkat" carrying the ROLES the schedule asks
// for. No service account to create, none to keep in step with the endpoints
// it calls - the schedule names the roles its call needs, and the route's rule
// reads them like anybody else's.
//
// Asking for a role is not being granted one: what bounds it is being let in
// at all. With a token that is an administrator's credential, minted by root,
// traced in the audit, revocable in a click - and two services that must not
// reach the same things get two tokens. Without one (the internal plane's None
// mode, internalaccess.go), it is the cluster's own trust and the list of
// callers, and the console says so where the mode is chosen.
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
// The CONSOLE reads the same rows and creates nothing: pause it, run it now,
// remove it - the three things wanted at two in the morning. It writes one
// field, the description, for the schedules whose service never said what
// they are for.
const schedulesPath = "/api/schedules"

func (a *API) registerSchedules(mux Mux) {
	// What the console does: read, and act on what exists. The service's own
	// door is the internal plane (internalplane.go), which carries all of it.
	mux.Handle("GET /api/schedules", a.schedules(a.consoleSide(a.listSchedules)))
	mux.Handle("GET /api/schedules/{id}", a.schedules(a.consoleSide(a.getSchedule)))
	mux.Handle("POST /api/schedules/{id}/pause", a.schedules(a.consoleSide(a.pauseSchedule)))
	mux.Handle("POST /api/schedules/{id}/resume", a.schedules(a.consoleSide(a.resumeSchedule)))
	mux.Handle("POST /api/schedules/{id}/run", a.schedules(a.consoleSide(a.runSchedule)))
	// What each turn did (SCHED-03): the row keeps the last result, this
	// keeps them all - including the turns that never went out.
	mux.Handle("GET /api/schedules/{id}/runs", a.schedules(a.consoleSide(a.listScheduleRuns)))
	mux.Handle("DELETE /api/schedules/{id}", a.schedules(a.consoleSide(a.deleteSchedule)))
	// The one thing the console writes: what a schedule is for, when the
	// service that asked for it did not say.
	mux.Handle("PUT /api/schedules/{id}/description", a.schedules(a.consoleSide(a.describeSchedule)))
	// Stopping the run in flight: closed at once, and the service hears it
	// at its next report.
	mux.Handle("DELETE /api/schedules/{id}/run", a.schedules(a.consoleSide(a.cancelRun)))
	// What only a service writes: the schedule, and the report on a run it
	// answered 202 to. Kept here for the service tokens of a backend that
	// has not moved to the internal plane yet, and for nobody else.
	mux.Handle("POST /api/schedules", a.serviceTokens(a.createSchedule))
	mux.Handle("PUT /api/schedules/{id}", a.serviceTokens(a.putSchedule))
	mux.Handle("PATCH /api/schedules/{id}/run", a.serviceTokens(a.reportRun))
	// How long a finished delayed action is kept before the sweep (SCHED-02).
	// Beside the trail's own retention, root's alone for the same reason: it
	// decides how long the gateway remembers what it did.
	mux.Handle("GET /api/settings/schedules", a.rootOnly(a.getScheduleSettings))
	mux.Handle("PUT /api/settings/schedules", a.rootOnly(a.putScheduleSettings))
	// Who may call the API on the internal plane, and how they are known.
	a.registerInternalAccess(mux)
	a.registerNotifications(mux)
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
	f, ok := scheduleFilter(w, r)
	if !ok {
		return
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

// scheduleFilter reads what a caller narrows the schedules by: the
// organisation, the route, and conditions on the service's own metadata.
func scheduleFilter(w http.ResponseWriter, r *http.Request) (store.ScheduleFilter, bool) {
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
				return f, false
			}
			f.Meta = append(f.Meta, m)
		}
	}
	return f, true
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
	// Who wrote it, as the gateway knows it - never what the body claims.
	c := callerOf(r)
	in.Caller, in.CallerVia, in.CallerAddr = c.Identity, c.Via, c.Addr
	if err := a.st.CreateSchedule(r.Context(), in); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "schedule.create", "schedule", in.ID, in.Name, "",
		rolesWords(in.Roles)+"; written by "+c.words())
	a.schedulesMoved(r.Context())
	a.answerSchedule(w, r, http.StatusCreated, in.ID)
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
	// A service that says nothing about it keeps the description there: an
	// operator may have written it, and a service that rewrites its schedule
	// at every start would wipe it each time.
	if in.Description == "" {
		in.Description = stored.Description
	}
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
	a.answerSchedule(w, r, http.StatusOK, in.ID)
}

// readSchedule decodes what was sent and refuses what cannot work - the half
// the two writers have in common.
func (a *API) readSchedule(w http.ResponseWriter, r *http.Request, actor store.User, stored store.Schedule) (store.Schedule, bool) {
	var in store.Schedule
	raw, err := io.ReadAll(r.Body)
	if err == nil {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		err = dec.Decode(&in)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return in, false
	}
	in.CreatedBy = actor.Username
	// The run belongs to whoever is running it, never to the writer: on a
	// creation that is nothing at all, whatever the body carries - a service
	// that sent a cause, a try count or a failing date would otherwise have
	// written the gateway's account of its own runs.
	in.RunID, in.ClaimedBy, in.ClaimedAt, in.Progress = stored.RunID, stored.ClaimedBy, stored.ClaimedAt, stored.Progress
	in.RunStarted, in.RunState, in.Attempts = stored.RunStarted, stored.RunState, stored.Attempts
	in.RunCause, in.RunOf, in.TurnTry, in.RunDetail = stored.RunCause, stored.RunOf, stored.TurnTry, stored.RunDetail
	in.LastState, in.LastDetail, in.LastAt = stored.LastState, stored.LastDetail, stored.LastAt
	in.FailingSince, in.UpdatedAt, in.Route = stored.FailingSince, stored.UpdatedAt, ""
	// PAUSED IS THE OPERATOR'S unless the service says otherwise. A service
	// that rewrites its schedule at every start sends the call and when, and
	// a pause put on it from the console is not among what it knows: leaving
	// the field out keeps the one in place, as for the description. Sent, it
	// is obeyed - a service may create a schedule paused, or resume its own.
	if stored.ID != "" {
		var said struct {
			Paused *bool `json:"paused"`
		}
		if json.Unmarshal(raw, &said) == nil && said.Paused == nil {
			in.Paused = stored.Paused
		}
	}
	// Who created it stays who created it: a later writer is in the audit.
	in.Caller, in.CallerVia, in.CallerAddr = stored.Caller, stored.CallerVia, stored.CallerAddr
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
//
// THE ANSWER IS ALSO THE GATEWAY'S WORD TO THE SERVICE. 204: carry on (or,
// for a final report, it is recorded). 410 Gone: STOP - the run is over on
// this side, and the body says why in one word (stopWhy). The run is already
// closed when the service hears it: it stops its work and reports nothing
// more. That is how a pause, a cancel or a deletion reaches a job that is
// three hours into its work: at its next report, which a long job sends
// anyway to keep its lease.
func (a *API) reportRun(w http.ResponseWriter, r *http.Request, _ store.User) {
	sc, err := a.st.GetSchedule(r.Context(), r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) {
		stopRun(w, stopDeleted, "this schedule no longer exists: stop the work, nothing is waiting for its report")
		return
	}
	if err != nil {
		a.internal(w, err)
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
	if in.Run == "" {
		writeErr(w, http.StatusUnprocessableEntity,
			"name the run: the run field, or the Meerkat-Job-Run header the call carried")
		return
	}
	if sc.RunID == "" || subtle.ConstantTimeCompare([]byte(in.Run), []byte(sc.RunID)) != 1 {
		// A run this schedule HAD: closed meanwhile - cancelled, paused,
		// lost - so the service is told to stop rather than that it is wrong.
		if ended, err := a.st.EndedRun(r.Context(), sc.ID, in.Run); err == nil {
			why := stopWhy(ended)
			stopRun(w, why, "this run is over: "+ended.State+" ("+ended.Detail+"); stop the work")
			return
		}
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
		// Paused while it ran: the run stops here, closed as cancelled, and
		// the turn comes round again after a resume. A FINAL report on a
		// paused schedule is still taken below - the work is done, and
		// throwing its result away would help nobody.
		if sc.Paused {
			next := now.Add(time.Hour)
			if at, e := sc.NextRun(now); e == nil {
				next = at
			}
			end := store.RunEnd{State: store.RunCancelled, Detail: pausedWhileRunning}
			if err := a.st.FinishSchedule(r.Context(), sc.ID, in.Run, end, store.RunNext{At: next}, now); err != nil {
				a.internal(w, err)
				return
			}
			a.schedulesMoved(r.Context())
			stopRun(w, stopPaused, "the schedule is paused: stop the work, the run is closed and the next turn "+
				"comes after it is resumed")
			return
		}
		// Progress renews the lease and nothing else: the other nodes need
		// not hear of it - a lease pushed later only means one of them wakes
		// up for nothing, finds nothing lapsed, and sleeps again. The screens
		// read the table on their own beat, detail included.
		if err := a.st.TouchSchedule(r.Context(), sc.ID, in.Run, in.Progress, in.Detail, now); err != nil {
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

// Why a service is told to stop (the "stop" of a 410 on its report).
const (
	stopPaused    = "paused"
	stopCancelled = "cancelled"
	stopDeleted   = "deleted"
	stopLost      = "lost"
	stopClosed    = "closed"
)

// pausedWhileRunning is what the history says about a run closed because its
// schedule was paused: how a later report on it is told "paused" rather than
// "cancelled".
const pausedWhileRunning = "paused while it ran: stopped at the service's report"

// stopWhy is the word a service hears about a run that is over.
func stopWhy(ended store.ScheduleRun) string {
	switch ended.State {
	case store.RunCancelled:
		if ended.Detail == pausedWhileRunning {
			return stopPaused
		}
		return stopCancelled
	case store.RunLost:
		return stopLost
	}
	return stopClosed
}

// stopRun is the 410: the run is over on this side, and the service stops.
func stopRun(w http.ResponseWriter, why, msg string) {
	writeJSON(w, http.StatusGone, map[string]string{"error": msg, "stop": why})
}

// cancelRun stops the run in flight (DELETE /api/schedules/{id}/run). It is
// closed at once, as cancelled, and the next turn is armed at the schedule's
// own cadence; the service hears it at its next report, which is answered
// 410 with "stop": "cancelled". A call still waiting for its answer is let
// go the same way: whatever comes back for it no longer matches the run.
func (a *API) cancelRun(w http.ResponseWriter, r *http.Request, actor store.User) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	if sc.RunID == "" {
		writeErr(w, http.StatusConflict, "no run in flight: there is nothing to cancel")
		return
	}
	who := actor.Username
	if who == "" {
		who = callerOf(r).Identity
	}
	if who == "" {
		who = "the API"
	}
	if err := a.cancel(r.Context(), sc, "cancelled by "+who); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "schedule.cancel", "schedule", sc.ID, sc.Name, "", "run "+sc.RunID)
	a.schedulesMoved(r.Context())
	now, err := a.st.GetSchedule(r.Context(), sc.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.nameSchedules(r, []store.Schedule{now})[0])
}

// cancel closes the run in flight as cancelled and arms the next turn.
func (a *API) cancel(ctx context.Context, sc store.Schedule, detail string) error {
	now := time.Now()
	next := now.Add(time.Hour)
	if at, e := sc.NextRun(now); e == nil {
		next = at
	}
	return a.st.FinishSchedule(ctx, sc.ID, sc.RunID,
		store.RunEnd{State: store.RunCancelled, Detail: detail}, store.RunNext{At: next}, now)
}

// pauseMatching and resumeMatching act on every schedule a filter matches -
// the list's own (tenant, route, meta.<key>=...) - which is how a service
// pauses "every import" in one call. A run in flight on a schedule paused
// this way stops at its next report, like any pause.
func (a *API) pauseMatching(w http.ResponseWriter, r *http.Request, actor store.User) {
	a.setPausedMatching(w, r, actor, true)
}

func (a *API) resumeMatching(w http.ResponseWriter, r *http.Request, actor store.User) {
	a.setPausedMatching(w, r, actor, false)
}

func (a *API) setPausedMatching(w http.ResponseWriter, r *http.Request, actor store.User, paused bool) {
	f, ok := scheduleFilter(w, r)
	if !ok {
		return
	}
	if f.TenantID == "" && f.RouteID == "" && len(f.Meta) == 0 {
		writeErr(w, http.StatusUnprocessableEntity,
			"say which schedules: meta.<key>=<value> (or =~<expression>), tenant or route - "+
				"every schedule of the gateway at once is not one call's to decide")
		return
	}
	list, err := a.st.ListSchedules(r.Context(), f)
	if err != nil {
		a.internal(w, err)
		return
	}
	verb := "schedule.resume"
	if paused {
		verb = "schedule.pause"
	}
	moved := false
	for i, sc := range list {
		if sc.Paused == paused {
			continue
		}
		if err := a.st.PauseSchedule(r.Context(), sc.ID, paused); err != nil {
			a.internal(w, err)
			return
		}
		a.auditEvent(r.Context(), actor, verb, "schedule", sc.ID, sc.Name, "", "by filter")
		list[i].Paused, moved = paused, true
	}
	if moved {
		a.schedulesMoved(r.Context())
	}
	if list == nil {
		list = []store.Schedule{}
	}
	writeJSON(w, http.StatusOK, a.nameSchedules(r, list))
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
	a.answerSchedule(w, r, http.StatusOK, sc.ID)
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
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound,
				"no run "+in.ReplayOf+" in this schedule's history: replay names one of its own runs")
			return
		}
		if err != nil {
			a.internal(w, err)
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
	a.answerSchedule(w, r, http.StatusAccepted, sc.ID)
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

// describeSchedule writes what a schedule is for, and nothing else. Empty
// removes it.
func (a *API) describeSchedule(w http.ResponseWriter, r *http.Request, actor store.User) {
	sc, ok := a.schedule(w, r)
	if !ok {
		return
	}
	var in struct {
		Description string `json:"description"`
	}
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	if err := store.SanitizeScheduleDescription(&in.Description); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.DescribeSchedule(r.Context(), sc.ID, in.Description); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "schedule.describe", "schedule", sc.ID, sc.Name, "",
		map[string]string{"description": sc.Description}, map[string]string{"description": in.Description})
	a.schedulesMoved(r.Context())
	sc.Description = in.Description
	writeJSON(w, http.StatusOK, a.nameSchedules(r, []store.Schedule{sc})[0])
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

// answerSchedule answers a write with the row as it now stands, route named:
// what the gateway stored, never what the body sent, and the same shape as a
// get - a pause's answer that lost the route was a row that changed under the
// screen reading it.
func (a *API) answerSchedule(w http.ResponseWriter, r *http.Request, code int, id string) {
	sc, err := a.st.GetSchedule(r.Context(), id)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, code, a.nameSchedules(r, []store.Schedule{sc})[0])
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
