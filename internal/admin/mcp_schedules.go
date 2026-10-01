package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
)

// The agent's view of the scheduled calls (SCHED-01).
//
// Three tools, and the line between what is here and what is not is the same
// one the console draws. READING the schedules answers the question an
// operator actually brings to an agent - "what is hammering that service
// every minute, and did last night's close run?" - and PAUSING one is the
// answer to it, reversible in a word. RUNNING one is the other half of the
// same night: a turn that was missed, asked for now rather than at its hour.
//
// CREATING one is deliberately absent, and not out of caution about tools: a
// schedule declares the ROLES its call will carry, so writing one is writing a
// standing call that reaches whatever those roles reach, every night, until
// somebody removes it. What bounds that today is holding a schedules-perimeter
// token - a deliberate act by root - and a tool would hand the same reach to
// whatever an agent was asked to do. The service that wants a schedule asks
// for it on the control plane, with its own token.
func (a *API) scheduleTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_schedules", Allow: administersIdentity, Title: "List the scheduled calls", ReadOnly: true,
			Description: "The calls this gateway makes on a timer (SCHED-01): what each one calls, how " +
				"often - a cadence (every PT6H), a calendar (cron, in its own timezone) or a single date " +
				"(once at ..., a delayed action that reads 'finished' when it has gone) - which roles " +
				"its call carries, " +
				"when it is next owed, and how the last run ended. Use it to answer " +
				"'what runs at night here', 'did it run', and 'why is that service being called every " +
				"minute'. A run in flight shows where it stands - calling (no answer yet) or accepted " +
				"(the service answered 202 and reports on it) - its progress, the node that claimed it, " +
				"and the attempt when a node stopped before the answer and it was sent again.",
			Schema: noArgs(),
			Call:   a.toolListSchedules,
		},
		{
			Name: "pause_schedule", Allow: administersIdentity, Title: "Pause or resume a scheduled call",
			Description: "Stop a schedule from firing, or let it fire again. Pausing keeps its next turn, " +
				"so resuming does not lose where it was in its cadence - and it changes nothing about the " +
				"run in flight, which finishes or lapses on its own. Take the id from list_schedules.",
			Schema: object(map[string]any{
				"id": str("The schedule's id, as given by list_schedules."),
				"paused": map[string]any{"type": "boolean",
					"description": "true stops it firing, false lets it fire again."},
			}, "id", "paused"),
			Call: a.toolPauseSchedule,
		},
		{
			Name: "run_schedule", Allow: administersIdentity, Title: "Run a scheduled call now",
			Description: "Bring a schedule's next turn forward to now, for a turn that was missed or a " +
				"job somebody wants rerun. It makes no call itself: the scheduler does, within a " +
				"second, and the run then shows in list_schedules like any other. It does NOT " +
				"skip a turn: the cadence continues from this run. Refused for a paused schedule - " +
				"resume it first - and while a run is in flight: its close arms the next turn.",
			Schema: object(map[string]any{
				"id": str("The schedule's id, as given by list_schedules."),
				"replayOf": str("Optional: the id of a past run, as list_schedule_runs gives it. The turn " +
					"goes out again as a NEW run, and the history says which one it replays."),
			}, "id"),
			Call: a.toolRunSchedule,
		},
		{
			Name: "list_schedule_runs", Allow: administersIdentity, Title: "The history of a scheduled call",
			ReadOnly: true,
			Description: "What each turn of one schedule did (SCHED-03), newest first: when it ended, how " +
				"it ended (done, failed, lost, or dropped for a turn that never went out because it came " +
				"round too late), what answered, which node made the call, which attempt it was, and what " +
				"it continues - its own turn, or a replay. Use it for 'since when is this failing', 'did " +
				"last night's close run' and 'what did it answer': list_schedules keeps only the LAST " +
				"result, which is why this exists. Take the id from list_schedules.",
			Schema: object(map[string]any{
				"id": str("The schedule's id, as given by list_schedules."),
				"limit": map[string]any{"type": "integer",
					"description": "How many runs at most, newest first (100 by default)."},
			}, "id"),
			Call: a.toolScheduleRuns,
		},
	}
}

func (a *API) toolScheduleRuns(ctx context.Context, raw json.RawMessage) (any, error) {
	var in struct {
		ID    string `json:"id"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("which schedule: pass the id list_schedules gives")
	}
	sc, err := a.st.GetSchedule(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("no schedule %q on this gateway", in.ID)
	}
	runs, err := a.st.ListScheduleRuns(ctx, sc.ID, in.Limit)
	if err != nil {
		return nil, err
	}
	type line struct {
		ID      string `json:"id"`
		Ended   string `json:"ended"`
		State   string `json:"state"`
		Took    string `json:"took,omitempty"`
		Status  int    `json:"status,omitempty"`
		Attempt int    `json:"attempt,omitempty"`
		Node    string `json:"node,omitempty"`
		Because string `json:"because,omitempty"`
		Detail  string `json:"detail,omitempty"`
	}
	out := make([]line, 0, len(runs))
	for _, r := range runs {
		l := line{
			ID: r.ID, Ended: time.Unix(r.EndedAt, 0).UTC().Format(time.RFC3339),
			State: r.State, Status: r.Status, Attempt: r.Attempt, Node: r.Node, Detail: r.Detail,
		}
		if r.StartedAt > 0 && r.EndedAt >= r.StartedAt {
			l.Took = (time.Duration(r.EndedAt-r.StartedAt) * time.Second).String()
		}
		if r.Cause != "" && r.Cause != store.CauseTurn {
			l.Because = r.Cause
			if r.OfRun != "" {
				l.Because += " of " + r.OfRun
			}
		}
		out = append(out, l)
	}
	return map[string]any{"schedule": sc.Name, "runs": out}, nil
}

func (a *API) toolListSchedules(ctx context.Context, _ json.RawMessage) (any, error) {
	list, err := a.st.ListSchedules(ctx, store.ScheduleFilter{})
	if err != nil {
		return nil, err
	}
	type line struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Roles    []string `json:"roles,omitempty"`
		Tenant   string   `json:"tenant,omitempty"`
		Calls    string   `json:"calls"`
		Every    string   `json:"every"`
		Timezone string   `json:"timezone,omitempty"`
		Paused   bool     `json:"paused"`
		NextAt   string   `json:"nextAt,omitempty"`
		Running  string   `json:"running,omitempty"`
		Progress int      `json:"progress,omitempty"`
		Last     string   `json:"last,omitempty"`
	}
	out := make([]line, 0, len(list))
	for _, sc := range list {
		l := line{
			ID: sc.ID, Name: sc.Name, Tenant: sc.TenantID, Roles: sc.Roles,
			Calls:  sc.Method + " " + sc.Path + " on route " + sc.RouteID,
			Every:  sc.Every,
			Paused: sc.Paused,
		}
		// A calendar and a single date answer the same question as a cadence,
		// so they go in the same field rather than in two more an agent has
		// to know about.
		switch {
		case sc.Cron != "":
			l.Every = "cron " + sc.Cron
			l.Timezone = sc.Timezone
		case sc.Once():
			l.Every = "once at " + sc.At
		}
		if rt, err := a.st.GetRoute(ctx, sc.RouteID); err == nil && rt.Name != "" {
			l.Calls = sc.Method + " " + sc.Path + " on " + rt.Name
		}
		switch {
		case sc.RunID != "":
			// Nothing more is owed while a run is open: the next turn is armed
			// when it closes.
			l.NextAt = "after the run in flight"
		case sc.Finished():
			// A single date that has been and gone owes nothing ever again.
			l.NextAt = "finished"
		case sc.NextAt > 0 && !sc.Paused:
			l.NextAt = time.Unix(sc.NextAt, 0).UTC().Format(time.RFC3339)
		}
		if sc.RunID != "" {
			l.Running = runWords(sc)
			l.Progress = sc.Progress
		}
		if sc.LastState != "" {
			l.Last = sc.LastState
			if sc.LastAt > 0 {
				l.Last += " at " + time.Unix(sc.LastAt, 0).UTC().Format(time.RFC3339)
			}
			if sc.LastDetail != "" {
				l.Last += " (" + sc.LastDetail + ")"
			}
		}
		out = append(out, l)
	}
	return out, nil
}

func (a *API) toolPauseSchedule(ctx context.Context, raw json.RawMessage) (any, error) {
	var in struct {
		ID     string `json:"id"`
		Paused *bool  `json:"paused"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("which schedule: pass the id list_schedules gives")
	}
	if in.Paused == nil {
		return nil, fmt.Errorf("say whether to pause: paused true stops it firing, false lets it fire again")
	}
	sc, err := a.st.GetSchedule(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("no schedule %q on this gateway", in.ID)
	}
	if err := a.st.PauseSchedule(ctx, sc.ID, *in.Paused); err != nil {
		return nil, err
	}
	verb := "schedule.resume"
	if *in.Paused {
		verb = "schedule.pause"
	}
	a.auditEvent(ctx, mcpActor(ctx), verb, "schedule", sc.ID, sc.Name, "", "")
	a.schedulesMoved(ctx)
	said := "resumed"
	if *in.Paused {
		said = "paused"
	}
	return map[string]any{"id": sc.ID, "name": sc.Name, "paused": *in.Paused,
		"said": sc.Name + " is " + said}, nil
}

func (a *API) toolRunSchedule(ctx context.Context, raw json.RawMessage) (any, error) {
	var in struct {
		ID       string `json:"id"`
		ReplayOf string `json:"replayOf"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, fmt.Errorf("which schedule: pass the id list_schedules gives")
	}
	sc, err := a.st.GetSchedule(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("no schedule %q on this gateway", in.ID)
	}
	if sc.Paused {
		return nil, fmt.Errorf("%q is paused: resume it before asking for a run", sc.Name)
	}
	now := time.Now()
	cause, of := store.CauseManual, ""
	if in.ReplayOf != "" {
		past, err := a.st.GetScheduleRun(ctx, sc.ID, in.ReplayOf)
		if err != nil {
			return nil, fmt.Errorf("no run %q in the history of %q: replay names one of its own runs, "+
				"as list_schedule_runs gives them", in.ReplayOf, sc.Name)
		}
		cause, of = store.CauseReplay, past.ID
	}
	armed, err := a.st.ArmSchedule(ctx, sc.ID, now, cause, of)
	if err != nil {
		return nil, err
	}
	if !armed {
		return nil, fmt.Errorf("%q: %s", sc.Name, runInFlight)
	}
	a.auditEvent(ctx, mcpActor(ctx), "schedule.run", "schedule", sc.ID, sc.Name, "", "")
	a.schedulesMoved(ctx)
	return map[string]any{"id": sc.ID, "name": sc.Name,
		"owedAt": now.UTC().Format(time.RFC3339),
		"said":   sc.Name + " is owed now; the call goes out within a second, and list_schedules shows how it ended"}, nil
}

// runWords is a run in flight, said in one line.
func runWords(sc store.Schedule) string {
	out := sc.RunState + " since " + time.Unix(sc.RunStarted, 0).UTC().Format(time.RFC3339)
	if sc.ClaimedBy != "" {
		out += ", claimed by " + sc.ClaimedBy
	}
	if sc.Attempts > 1 {
		out += fmt.Sprintf(", attempt %d", sc.Attempts)
	}
	return out
}
