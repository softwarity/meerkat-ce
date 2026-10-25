package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// PAUSED IS THE OPERATOR'S. A service that rewrites its schedule at every
// start sends what it knows - the call and when - and a pause somebody put on
// it at two in the morning is not among that: a PUT without `paused` keeps the
// one in place, as it keeps the description. A service that does say it is
// obeyed, both ways.
func TestAServiceRewriteKeepsTheOperatorsPause(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "backend")
	const payload = `{"name":"poll","routeId":"r1","path":"/jobs/poll","every":"PT1H"%s}`
	code, body := f.bearerJSON(t, "POST", "/api/schedules", strings.Replace(payload, "%s", "", 1), token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(body), &made); err != nil {
		t.Fatal(err)
	}
	if code, body := f.call(t, "POST", "/api/schedules/"+made.ID+"/pause", "", f.rootC); code != http.StatusOK {
		t.Fatalf("pause: %d %s", code, body)
	}
	paused := func() bool {
		t.Helper()
		got, err := f.api.st.GetSchedule(context.Background(), made.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.Paused
	}
	// The service restarts and writes its schedule again, saying nothing of it.
	code, body = f.bearerJSON(t, "PUT", "/api/schedules/"+made.ID, strings.Replace(payload, "%s", "", 1), token)
	if code != http.StatusOK {
		t.Fatalf("update: %d %s", code, body)
	}
	if !paused() || !strings.Contains(body, `"paused":true`) {
		t.Fatalf("a PUT without paused resumed the operator's pause: %s", body)
	}
	// Said, it is the service's word.
	if code, body := f.bearerJSON(t, "PUT", "/api/schedules/"+made.ID,
		strings.Replace(payload, "%s", `,"paused":false`, 1), token); code != http.StatusOK || paused() {
		t.Fatalf("an explicit paused:false did not resume: %d %s", code, body)
	}
	if code, body := f.bearerJSON(t, "PUT", "/api/schedules/"+made.ID,
		strings.Replace(payload, "%s", `,"paused":true`, 1), token); code != http.StatusOK || !paused() {
		t.Fatalf("an explicit paused:true did not pause: %d %s", code, body)
	}
}

// What the gateway keeps about a run - why the turn goes out, what it
// continues, how many tries it had, since when it fails, what the service last
// said - is the gateway's: a body that carries it writes none of it, on a
// creation as on a rewrite, and the answer says what is stored rather than
// what was sent.
func TestAServiceWritesNoneOfTheGatewaysState(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "backend")
	const forged = `,"runCause":"replay","runOf":"run-x","turnTry":2,"failingSince":1234,` +
		`"runDetail":"half done","route":"somewhere else"`
	payload := `{"name":"poll","routeId":"r1","path":"/jobs/poll","every":"PT1H"` + forged + `}`
	code, body := f.bearerJSON(t, "POST", "/api/schedules", payload, token)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	var made store.Schedule
	if err := json.Unmarshal([]byte(body), &made); err != nil {
		t.Fatal(err)
	}
	clean := func(what string, sc store.Schedule) {
		t.Helper()
		if sc.RunCause != "" || sc.RunOf != "" || sc.TurnTry != 0 || sc.FailingSince != 0 || sc.RunDetail != "" {
			t.Fatalf("%s: the gateway's state was taken from the body: %+v", what, sc)
		}
		if sc.Route != "orders-api" {
			t.Fatalf("%s: the route's name is %q, want the gateway's word for it", what, sc.Route)
		}
	}
	clean("create answer", made)
	stored, err := f.api.st.GetSchedule(context.Background(), made.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored.Route = "orders-api" // a row carries the id; the name is filled for reading
	clean("create stored", stored)

	code, body = f.bearerJSON(t, "PUT", "/api/schedules/"+made.ID, payload, token)
	if code != http.StatusOK {
		t.Fatalf("update: %d %s", code, body)
	}
	var back store.Schedule
	if err := json.Unmarshal([]byte(body), &back); err != nil {
		t.Fatal(err)
	}
	clean("update answer", back)
}

// Every answer carrying a schedule names its route, as the list and the get
// do: a screen or a service reading the pause's answer must not see the row
// lose a field.
func TestEveryScheduleAnswerNamesItsRoute(t *testing.T) {
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
	for _, step := range []string{"/pause", "/resume", "/run"} {
		code, body := f.call(t, "POST", "/api/schedules/"+made.ID+step, "", f.rootC)
		if code/100 != 2 || !strings.Contains(body, `"route":"orders-api"`) {
			t.Fatalf("POST %s: %d %s, want the route named", step, code, body)
		}
	}
}
