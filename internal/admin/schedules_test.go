package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// bearerJSON is the headless path with a body: what a backend service does.
func (f fixture) bearerJSON(t *testing.T, method, path, body, token string) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.adminSrv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res.StatusCode, string(b)
}

// serviceToken mints the credential a backend holds: a control-plane token
// whose perimeter is the schedules and nothing else.
func serviceToken(t *testing.T, f fixture, name string) string {
	t.Helper()
	code, body := f.call(t, "POST", "/api/admin-tokens",
		`{"name":"`+name+`","scope":"schedules"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("minting a service token: %d %s", code, body)
	}
	var made struct{ Token string }
	if err := json.Unmarshal([]byte(body), &made); err != nil {
		t.Fatal(err)
	}
	return made.Token
}

func aRoute(t *testing.T, f fixture) {
	t.Helper()
	if err := f.api.st.SaveRoute(context.Background(), store.Route{
		ID: "r1", Name: "orders-api", Enabled: true, Upstream: "http://orders:8080",
	}); err != nil {
		t.Fatal(err)
	}
}

// The whole shape of SCHED-01's API, from the side that uses it: a service
// names its own schedules, finds them again by its own metadata, and sees
// nobody else's.
func TestAServiceManagesItsOwnSchedules(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "stations")

	// The perimeter is the point: this token opens the schedules and nothing
	// else on this port.
	if code, body := f.bearer(t, "GET", "/api/me", token); code != http.StatusForbidden {
		t.Fatalf("a schedules token must not open the rest of the control plane: %d %s", code, body)
	}

	// The identity it runs as is in the payload; the id comes back from us.
	const payload = `{"name":"station 42","roles":["station_fetch"],"routeId":"r1","method":"POST",` +
		`"path":"/jobs/poll","every":"PT1H","metadata":{"station":"42","kind":"poll"}}`
	code, body := f.bearerJSON(t, "POST", "/api/schedules", payload, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(body), &made); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(made.ID, "sch_") {
		t.Fatalf("the gateway must hand back an id of its own, got %q", made.ID)
	}
	if len(made.Roles) != 1 || made.Roles[0] != "station_fetch" {
		t.Fatalf("the call's roles are not what the payload asked for: %+v", made.Roles)
	}
	if made.Metadata["station"] != "42" {
		t.Fatalf("the metadata did not survive: %+v", made.Metadata)
	}
	// Changing it is a PUT on the id we were given, and an id nobody was given
	// is a 404 rather than a back door to creation.
	if code, body := f.bearerJSON(t, "PUT", "/api/schedules/"+made.ID, payload, token); code != http.StatusOK {
		t.Fatalf("update: %d %s", code, body)
	}
	if code, _ := f.bearerJSON(t, "PUT", "/api/schedules/sch_nobodygaveyouthis", payload, token); code != http.StatusNotFound {
		t.Fatalf("a PUT on an unknown id answered %d, want 404", code)
	}
	if n := countSchedules(t, f, token, ""); n != 1 {
		t.Fatalf("updating left %d schedules", n)
	}

	// Finding them again is what the metadata is for.
	if n := countSchedules(t, f, token, "?meta.station=42"); n != 1 {
		t.Fatalf("meta.station=42 found %d", n)
	}
	if n := countSchedules(t, f, token, "?meta.station=99"); n != 0 {
		t.Fatalf("meta.station=99 found %d", n)
	}
	if n := countSchedules(t, f, token, "?meta.station=~^4"); n != 1 {
		t.Fatalf("an expression found %d", n)
	}
	// Two conditions are an AND.
	if n := countSchedules(t, f, token, "?meta.station=42&meta.kind=push"); n != 0 {
		t.Fatalf("two conditions behaved as an OR: %d", n)
	}
	// And an expression that does not compile is an answer, not an empty list.
	if code, body := f.bearer(t, "GET", "/api/schedules?meta.station=~^(", token); code != http.StatusUnprocessableEntity {
		t.Fatalf("a broken expression must be refused by name: %d %s", code, body)
	}
}

// One token serves a FLEET: several schedules, several sets of roles, told
// apart by what the service filed them under. Minting one token per service
// would be one secret per service to rotate.
func TestOneTokenSchedulesForSeveralJobs(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "backend")

	for _, one := range []struct{ name, roles, meta string }{
		{"station 42", `["station_fetch"]`, `{"kind":"poll"}`},
		{"invoice acme", `["billing_run"]`, `{"kind":"invoice"}`},
	} {
		body := `{"name":"` + one.name + `","roles":` + one.roles + `,"routeId":"r1",` +
			`"path":"/jobs/x","every":"PT1H","metadata":` + one.meta + `}`
		if code, got := f.bearerJSON(t, "POST", "/api/schedules", body, token); code != http.StatusCreated {
			t.Fatalf("%s: %d %s", one.name, code, got)
		}
	}
	if n := countSchedules(t, f, token, "?meta.kind=invoice"); n != 1 {
		t.Fatalf("the invoices filter found %d", n)
	}
	if n := countSchedules(t, f, token, ""); n != 2 {
		t.Fatalf("the token sees %d schedules", n)
	}
}

// An operator reads every schedule on the gateway, whoever owns it, and keeps
// the three actions wanted at two in the morning.
func TestAnOperatorSeesEverySchedule(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "backend")
	code, body := f.bearerJSON(t, "POST", "/api/schedules",
		`{"name":"station 42","roles":["station_fetch"],"routeId":"r1","path":"/jobs/poll","every":"PT1H"}`,
		token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(body), &made); err != nil {
		t.Fatal(err)
	}

	code, body = f.call(t, "GET", "/api/schedules", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(body, made.ID) {
		t.Fatalf("an operator must see every schedule: %d %s", code, body)
	}
	if code, _ := f.call(t, "POST", "/api/schedules/"+made.ID+"/pause", "", f.rootC); code != http.StatusOK {
		t.Fatalf("pause: %d", code)
	}
	if code, _ := f.call(t, "POST", "/api/schedules/"+made.ID+"/resume", "", f.rootC); code != http.StatusOK {
		t.Fatalf("resume: %d", code)
	}
	if code, _ := f.call(t, "DELETE", "/api/schedules/"+made.ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
}

func countSchedules(t *testing.T, f fixture, token, query string) int {
	t.Helper()
	code, body := f.bearer(t, "GET", "/api/schedules"+query, token)
	if code != http.StatusOK {
		t.Fatalf("list%s: %d %s", query, code, body)
	}
	var list []store.Schedule
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("list%s: %v (%s)", query, err, body)
	}
	return len(list)
}

// bell records what a write told the cluster and this node's scheduler.
type bell struct {
	mu        sync.Mutex
	announced []string
	woken     int
}

func (b *bell) Register(string, func(context.Context) error) {}
func (b *bell) OnSignal(string, func(string))                {}
func (b *bell) OnSignalFrom(string, func(string, string))    {}
func (b *bell) Node() string                                 { return "test" }
func (b *bell) Signal(context.Context, string, string)       {}
func (b *bell) Run(context.Context)                          {}
func (b *bell) Announce(_ context.Context, topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.announced = append(b.announced, topic)
}
func (b *bell) Wake() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.woken++
}

// rang says whether the last write reached both: the other nodes through the
// bus, and this node's own scheduler - which sleeps until the next thing owed
// and would otherwise sleep through the change. Resets for the next write.
func (b *bell) rang(t *testing.T, what string) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.announced) != 1 || b.announced[0] != store.TopicSchedules {
		t.Errorf("%s: the other nodes were told %v, want [%s]", what, b.announced, store.TopicSchedules)
	}
	if b.woken != 1 {
		t.Errorf("%s: this node's scheduler was woken %d times, want 1", what, b.woken)
	}
	b.announced, b.woken = nil, 0
}

// Every write that moves what is owed rings the bell - on the bus, so the
// other nodes react, and on this node. The first version sent a bare NOTIFY
// that bumped no version: the other nodes heard it and found nothing to do.
func TestEveryScheduleWriteRingsTheBell(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	b := &bell{}
	f.api.Bus, f.api.Scheduler = b, b
	token := serviceToken(t, f, "backend")

	code, body := f.bearerJSON(t, "POST", "/api/schedules",
		`{"name":"poll","routeId":"r1","path":"/jobs/poll","every":"PT1H"}`, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	b.rang(t, "create")
	var made store.Schedule
	if err := json.Unmarshal([]byte(body), &made); err != nil {
		t.Fatal(err)
	}
	id := made.ID
	if code, body := f.bearerJSON(t, "PUT", "/api/schedules/"+id,
		`{"name":"poll","routeId":"r1","path":"/jobs/poll","every":"PT2H"}`, token); code != http.StatusOK {
		t.Fatalf("put: %d %s", code, body)
	}
	b.rang(t, "put")
	for _, action := range []string{"pause", "resume", "run"} {
		if code, body := f.bearerJSON(t, "POST", "/api/schedules/"+id+"/"+action, "", token); code >= 300 {
			t.Fatalf("%s: %d %s", action, code, body)
		}
		b.rang(t, action)
	}

	// The agent's two writes ring it the same way.
	ctx := mcpCtx(rootUser(t, f))
	if _, err := f.api.toolPauseSchedule(ctx, json.RawMessage(`{"id":"`+id+`","paused":false}`)); err != nil {
		t.Fatal(err)
	}
	b.rang(t, "pause_schedule")
	if _, err := f.api.toolRunSchedule(ctx, json.RawMessage(`{"id":"`+id+`"}`)); err != nil {
		t.Fatal(err)
	}
	b.rang(t, "run_schedule")

	// A service closing its run arms the next turn: that moves what is owed.
	// Reporting progress does not, and stays off the bus.
	now := time.Now()
	if ok, err := f.api.st.ClaimSchedule(context.Background(), id, "run-a", "node-a", now); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if code, body := f.bearerJSON(t, "PATCH", "/api/schedules/"+id+"/run",
		`{"run":"run-a","state":"running","progress":40}`, token); code != http.StatusNoContent {
		t.Fatalf("progress: %d %s", code, body)
	}
	b.mu.Lock()
	quiet := len(b.announced) == 0 && b.woken == 0
	b.mu.Unlock()
	if !quiet {
		t.Errorf("progress rang the bell: %v, %d", b.announced, b.woken)
	}
	if code, body := f.bearerJSON(t, "PATCH", "/api/schedules/"+id+"/run",
		`{"run":"run-a","state":"done"}`, token); code != http.StatusNoContent {
		t.Fatalf("done: %d %s", code, body)
	}
	b.rang(t, "report done")

	if code, _ := f.bearerJSON(t, "DELETE", "/api/schedules/"+id, "", token); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	b.rang(t, "delete")
}

// "Run now" under a run in flight is refused, by the API and by the agent:
// that turn is the run's own until it closes, and the close arms the next.
func TestRunNowWaitsForTheRunInFlight(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "backend")
	code, body := f.bearerJSON(t, "POST", "/api/schedules",
		`{"name":"poll","routeId":"r1","path":"/jobs/poll","every":"PT1H"}`, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(body), &made); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.api.st.ClaimSchedule(context.Background(), made.ID, "run-a", "node-a", time.Now()); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	code, body = f.bearerJSON(t, "POST", "/api/schedules/"+made.ID+"/run", "", token)
	if code != http.StatusConflict || !strings.Contains(body, "in flight") {
		t.Fatalf("run now under a run in flight: %d %s", code, body)
	}
	_, err := f.api.toolRunSchedule(mcpCtx(rootUser(t, f)), json.RawMessage(`{"id":"`+made.ID+`"}`))
	if err == nil || !strings.Contains(err.Error(), "in flight") {
		t.Fatalf("the agent was not refused the same way: %v", err)
	}
}

// A delayed action (SCHED-02): a service asks to be called once, at a date,
// and the gateway owes exactly that turn - no cadence to invent.
func TestAServiceAsksToBeCalledOnceAtADate(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "stations")

	at := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	body := `{"name":"cart reminder","routeId":"r1","path":"/jobs/remind","at":"` +
		at.Format(time.RFC3339) + `","metadata":{"cart":"42"}}`
	code, out := f.bearerJSON(t, "POST", "/api/schedules", body, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, out)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(out), &made); err != nil {
		t.Fatal(err)
	}
	if made.At == "" || made.Every != "" {
		t.Fatalf("the date was not kept as the answer to when: %+v", made)
	}
	if made.NextAt != at.Unix() {
		t.Fatalf("the turn owed is %d, want %d", made.NextAt, at.Unix())
	}

	// Pushed back: the service says another date, and the turn moves with it.
	later := at.Add(time.Hour)
	code, out = f.bearerJSON(t, "PUT", "/api/schedules/"+made.ID,
		`{"name":"cart reminder","routeId":"r1","path":"/jobs/remind","at":"`+
			later.Format(time.RFC3339)+`"}`, token)
	if code != http.StatusOK {
		t.Fatalf("put: %d %s", code, out)
	}
	var moved store.Schedule
	if err := json.Unmarshal([]byte(out), &moved); err != nil {
		t.Fatal(err)
	}
	if moved.NextAt != later.Unix() {
		t.Fatalf("the delayed action was not pushed back: %d, want %d", moved.NextAt, later.Unix())
	}

	// Two answers to "when" are refused by name, not silently preferred.
	code, out = f.bearerJSON(t, "POST", "/api/schedules",
		`{"name":"both","routeId":"r1","path":"/jobs/remind","every":"PT1H","at":"`+
			at.Format(time.RFC3339)+`"}`, token)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "says WHEN once") {
		t.Fatalf("a schedule said when twice and was taken: %d %s", code, out)
	}
}

// How long a finished delayed action is kept is housekeeping for this
// installation: root's alone, like the trail's own retention beside it.
func TestTheScheduleRetentionIsRootsAlone(t *testing.T) {
	f := setupBare(t)
	token := serviceToken(t, f, "backend")

	if code, _ := f.bearer(t, "GET", "/api/settings/schedules", token); code != http.StatusForbidden {
		t.Fatalf("a schedules token read the retention: %d", code)
	}
	code, body := f.call(t, "GET", "/api/settings/schedules", "", f.rootC)
	if code != http.StatusOK || !strings.Contains(body, "retentionDays") {
		t.Fatalf("root cannot read the retention: %d %s", code, body)
	}
	code, body = f.call(t, "PUT", "/api/settings/schedules", `{"retentionDays":90,"choices":[]}`, f.rootC)
	if code != http.StatusOK || !strings.Contains(body, "90") {
		t.Fatalf("root cannot set the retention: %d %s", code, body)
	}
	if code, body := f.call(t, "PUT", "/api/settings/schedules",
		`{"retentionDays":3,"choices":[]}`, f.rootC); code != http.StatusUnprocessableEntity {
		t.Fatalf("a lifetime nobody offers was taken: %d %s", code, body)
	}
}

// The history over the API (SCHED-03), and the replay it exists for: a turn
// that never went out is kept, and can be asked for again.
func TestAServiceReadsItsHistoryAndReplaysATurn(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "stations")
	code, out := f.bearerJSON(t, "POST", "/api/schedules",
		`{"name":"nightly close","routeId":"r1","path":"/jobs/close","every":"PT1H","catchUp":60}`, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, out)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(out), &made); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// A turn nobody was up for: owed two hours ago, dropped for being late.
	due := time.Now().Add(-2 * time.Hour)
	if _, err := f.api.st.ArmSchedule(ctx, made.ID, due, "", ""); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.api.st.DropTurn(ctx, made.ID, due, time.Now().Add(time.Hour), time.Now(),
		"two hours late, past the 60s this schedule allows"); err != nil || !ok {
		t.Fatalf("drop: %v %v", ok, err)
	}

	code, out = f.bearer(t, "GET", "/api/schedules/"+made.ID+"/runs", token)
	if code != http.StatusOK {
		t.Fatalf("history: %d %s", code, out)
	}
	var runs []store.ScheduleRun
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].State != store.RunDropped {
		t.Fatalf("the turn that never went out is not in the history: %s", out)
	}

	// Asked for again, by name: a new run that says what it replays.
	code, out = f.bearerJSON(t, "POST", "/api/schedules/"+made.ID+"/run",
		`{"replayOf":"`+runs[0].ID+`"}`, token)
	if code != http.StatusAccepted {
		t.Fatalf("replay: %d %s", code, out)
	}
	back, err := f.api.st.GetSchedule(ctx, made.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.RunCause != store.CauseReplay || back.RunOf != runs[0].ID {
		t.Fatalf("the replay did not carry what it continues: %+v", back)
	}
	if back.NextAt > time.Now().Unix()+2 {
		t.Fatalf("the replayed turn is not owed now: %d", back.NextAt)
	}

	// A run that is not this schedule's is a 404, not a turn fired blindly.
	if code, out := f.bearerJSON(t, "POST", "/api/schedules/"+made.ID+"/run",
		`{"replayOf":"run-nope"}`, token); code != http.StatusNotFound {
		t.Fatalf("an unknown run was replayed: %d %s", code, out)
	}
}

// A long job that cannot run yet says so on its own report: the run closes
// with the reason, and the turn comes back when the SERVICE said (SCHED-05).
func TestALongJobPutsItsOwnTurnOff(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "stations")
	code, out := f.bearerJSON(t, "POST", "/api/schedules",
		`{"name":"daily extract","routeId":"r1","path":"/jobs/extract","every":"PT6H"}`, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, out)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(out), &made); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if ok, err := f.api.st.ClaimSchedule(ctx, made.ID, "run-a", "node-a", time.Now()); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}

	code, out = f.bearerJSON(t, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"run-a","state":"failed","detail":"the extract is not published yet","retryIn":"PT10M"}`, token)
	if code != http.StatusNoContent {
		t.Fatalf("report: %d %s", code, out)
	}
	back, err := f.api.st.GetSchedule(ctx, made.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.RunCause != store.CauseAsked || back.TurnTry != 1 {
		t.Fatalf("the turn was not put off by the service: %+v", back)
	}
	if d := back.NextAt - time.Now().Unix(); d < 9*60 || d > 11*60 {
		t.Fatalf("the turn comes back in %ds, want about ten minutes", d)
	}
	runs, err := f.api.st.ListScheduleRuns(ctx, made.ID, 0)
	if err != nil || len(runs) != 1 {
		t.Fatalf("history: %+v %v", runs, err)
	}
	if !strings.Contains(runs[0].Detail, "not published yet") {
		t.Fatalf("the reason was lost: %q", runs[0].Detail)
	}

	// The refusals, each naming what is allowed.
	if ok, _ := f.api.st.ClaimSchedule(ctx, made.ID, "run-b", "node-a", time.Now().Add(11*time.Minute)); !ok {
		t.Fatal("claim the turn the service asked for")
	}
	if code, body := f.bearerJSON(t, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"run-b","state":"failed","retryIn":"soon"}`, token); code != http.StatusUnprocessableEntity ||
		!strings.Contains(body, "ISO 8601") {
		t.Fatalf("a delay that is not one was taken: %d %s", code, body)
	}
	if code, body := f.bearerJSON(t, "PATCH", "/api/schedules/"+made.ID+"/run",
		`{"run":"run-b","state":"done","retryIn":"PT10M"}`, token); code != http.StatusUnprocessableEntity ||
		!strings.Contains(body, "needs no second call") {
		t.Fatalf("a done job asked to be called back: %d %s", code, body)
	}
}
