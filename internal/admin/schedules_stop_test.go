package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// aRunInFlight creates a schedule on the internal plane and opens a run the
// service accepted, as the scheduler leaves it after a 202.
func aRunInFlight(t *testing.T, f fixture, body string) (store.Schedule, string) {
	t.Helper()
	srv := internalPlane(t, f)
	code, out := on(t, srv, "POST", "/api/schedules", body, "", nil)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, out)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(out), &made); err != nil {
		t.Fatal(err)
	}
	return made, openRun(t, f, made.ID)
}

func openRun(t *testing.T, f fixture, id string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := f.api.st.ArmSchedule(ctx, id, time.Now().Add(-time.Second), "", ""); err != nil {
		t.Fatal(err)
	}
	run := "run-" + id + "-" + time.Now().Format("150405.000000000")
	if ok, err := f.api.st.ClaimSchedule(ctx, id, run, "node-a", time.Now()); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if err := f.api.st.AcceptRun(ctx, id, run, time.Now()); err != nil {
		t.Fatal(err)
	}
	return run
}

// stopSays reads a 410: the run is over, and why in one word.
func stopSays(t *testing.T, code int, body, want string) {
	t.Helper()
	var said struct{ Stop string }
	_ = json.Unmarshal([]byte(body), &said)
	if code != http.StatusGone || said.Stop != want {
		t.Fatalf("want 410 stop %q, got %d %s", want, code, body)
	}
}

// The report is also the gateway's word to a long job: 204 carry on, 410
// stop. A progress report keeps what the service said, for the screens.
func TestAReportKeepsWhatTheServiceSays(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	srv := internalPlane(t, f)
	made, run := aRunInFlight(t, f, stationSchedule)
	code, out := on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"`+run+`","progress":73,"detail":"7 233 / 9 882 records"}`, "", nil)
	if code != http.StatusNoContent {
		t.Fatalf("a progress report: %d %s", code, out)
	}
	got, _ := f.api.st.GetSchedule(context.Background(), made.ID)
	if got.Progress != 73 || got.RunDetail != "7 233 / 9 882 records" {
		t.Fatalf("the report was not kept: progress %d, detail %q", got.Progress, got.RunDetail)
	}
	// The run closes: the detail of a run in flight goes with it.
	if code, out := on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"`+run+`","state":"done","detail":"9 882 records"}`, "", nil); code != http.StatusNoContent {
		t.Fatalf("the final report: %d %s", code, out)
	}
	got, _ = f.api.st.GetSchedule(context.Background(), made.ID)
	if got.RunDetail != "" || got.LastState != store.RunDone {
		t.Fatalf("the close left the run's detail behind: %+v", got)
	}
}

// Paused while it runs: the next progress report is told to stop, the run is
// closed as cancelled and says why, and a report after that hears the same.
// A FINAL report on a paused schedule is still taken: the work is done.
func TestAPausedScheduleStopsItsRunAtTheNextReport(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	srv := internalPlane(t, f)
	made, run := aRunInFlight(t, f, stationSchedule)
	if code, out := on(t, srv, "POST", "/api/schedules/"+made.ID+"/pause", "", "", nil); code != http.StatusOK {
		t.Fatalf("pause: %d %s", code, out)
	}
	report := `{"run":"` + run + `","progress":10}`
	code, out := on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run", report, "", nil)
	stopSays(t, code, out, "paused")
	got, _ := f.api.st.GetSchedule(context.Background(), made.ID)
	if got.RunID != "" || got.LastState != store.RunCancelled {
		t.Fatalf("the paused run was not closed as cancelled: %+v", got)
	}
	code, out = on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run", report, "", nil)
	stopSays(t, code, out, "paused")

	// Resumed, run again, paused - and the job finishes before it reports.
	if code, out := on(t, srv, "POST", "/api/schedules/"+made.ID+"/resume", "", "", nil); code != http.StatusOK {
		t.Fatalf("resume: %d %s", code, out)
	}
	run = openRun(t, f, made.ID)
	if err := f.api.st.PauseSchedule(context.Background(), made.ID, true); err != nil {
		t.Fatal(err)
	}
	if code, out := on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"`+run+`","state":"done"}`, "", nil); code != http.StatusNoContent {
		t.Fatalf("a final report on a paused schedule was refused: %d %s", code, out)
	}
	got, _ = f.api.st.GetSchedule(context.Background(), made.ID)
	if got.LastState != store.RunDone {
		t.Fatalf("the finished work was thrown away: %+v", got)
	}
}

// Cancelling closes the run at once, in the history too; the service hears
// it at its next report; and a schedule with no run has nothing to cancel.
// Deleting the schedule is heard the same way.
func TestARunIsCancelledAndTheServiceIsToldToStop(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	srv := internalPlane(t, f)
	made, run := aRunInFlight(t, f, stationSchedule)

	// From the console.
	code, out := f.call(t, "DELETE", "/api/schedules/"+made.ID+"/run", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("cancel: %d %s", code, out)
	}
	runs, _ := f.api.st.ListScheduleRuns(context.Background(), made.ID, 0)
	if len(runs) != 1 || runs[0].State != store.RunCancelled || runs[0].RunID != run ||
		!strings.Contains(runs[0].Detail, "cancelled by root") {
		t.Fatalf("the cancelled run is not in the history as such: %+v", runs)
	}
	got, _ := f.api.st.GetSchedule(context.Background(), made.ID)
	if got.RunID != "" || got.NextAt <= time.Now().Unix() {
		t.Fatalf("the cancel did not close the run and arm the next turn: %+v", got)
	}
	code, out = on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"`+run+`","progress":50}`, "", nil)
	stopSays(t, code, out, "cancelled")
	// Even a final report: the run was cancelled, its result goes nowhere.
	code, out = on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"`+run+`","state":"done"}`, "", nil)
	stopSays(t, code, out, "cancelled")
	if code, out := f.call(t, "DELETE", "/api/schedules/"+made.ID+"/run", "", f.rootC); code != http.StatusConflict {
		t.Fatalf("cancelling with no run in flight: %d %s", code, out)
	}
	// A run id nobody was given is still a 409: wrong, not over.
	if code, out := on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"never-given","progress":1}`, "", nil); code != http.StatusConflict {
		t.Fatalf("an unknown run: %d %s", code, out)
	}

	// From the service itself, on the internal plane.
	openRun(t, f, made.ID)
	if code, out := on(t, srv, "DELETE", "/api/schedules/"+made.ID+"/run", "", "", nil); code != http.StatusOK {
		t.Fatalf("cancel on the internal plane: %d %s", code, out)
	}

	// Deleted while it runs.
	run = openRun(t, f, made.ID)
	if code, out := on(t, srv, "DELETE", "/api/schedules/"+made.ID, "", "", nil); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, out)
	}
	code, out = on(t, srv, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"`+run+`","progress":50}`, "", nil)
	stopSays(t, code, out, "deleted")
}

// "Pause every import": one call, the list's own filter, the schedules it
// matched as they now stand - and a call that names nothing is refused.
func TestSchedulesArePausedByTheirMetadata(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	srv := internalPlane(t, f)
	for _, body := range []string{
		`{"name":"import fr","routeId":"r1","every":"PT1H","metadata":{"svc":"stations","job":"oscar-import"}}`,
		`{"name":"import de","routeId":"r1","every":"PT1H","metadata":{"svc":"stations","job":"oscar-import"}}`,
		`{"name":"updates","routeId":"r1","every":"PT1H","metadata":{"svc":"stations","job":"oscar-updates"}}`,
	} {
		if code, out := on(t, srv, "POST", "/api/schedules", body, "", nil); code != http.StatusCreated {
			t.Fatalf("create: %d %s", code, out)
		}
	}
	code, out := on(t, srv, "POST", "/api/schedules/pause?meta.svc=stations&meta.job=oscar-import", "", "", nil)
	if code != http.StatusOK {
		t.Fatalf("pause by metadata: %d %s", code, out)
	}
	var touched []store.Schedule
	if err := json.Unmarshal([]byte(out), &touched); err != nil {
		t.Fatal(err)
	}
	if len(touched) != 2 || !touched[0].Paused || !touched[1].Paused {
		t.Fatalf("pause by metadata answered %s", out)
	}
	all, _ := f.api.st.ListSchedules(context.Background(), store.ScheduleFilter{})
	for _, sc := range all {
		if sc.Paused != (sc.Metadata["job"] == "oscar-import") {
			t.Fatalf("%s paused=%v", sc.Name, sc.Paused)
		}
	}
	if code, out := on(t, srv, "POST", "/api/schedules/resume?meta.job=~^oscar-", "", "", nil); code != http.StatusOK {
		t.Fatalf("resume by expression: %d %s", code, out)
	}
	all, _ = f.api.st.ListSchedules(context.Background(), store.ScheduleFilter{})
	for _, sc := range all {
		if sc.Paused {
			t.Fatalf("%s is still paused", sc.Name)
		}
	}
	if code, out := on(t, srv, "POST", "/api/schedules/pause", "", "", nil); code != http.StatusUnprocessableEntity {
		t.Fatalf("a pause naming nothing: %d %s", code, out)
	}
}

// overlap "run" was accepted and never acted on: refused now, by name.
func TestASecondRunAlongsideTheFirstIsRefused(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	srv := internalPlane(t, f)
	code, out := on(t, srv, "POST", "/api/schedules",
		`{"name":"x","routeId":"r1","every":"PT1H","overlap":"run"}`, "", nil)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "skip") {
		t.Fatalf("overlap run: %d %s", code, out)
	}
}
