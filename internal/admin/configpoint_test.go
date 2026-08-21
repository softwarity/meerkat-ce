package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// The tape (CFG-06). What is pinned here is the CONDITION - the fingerprint
// moved - because that is what makes the feature affordable to trust: an
// endpoint nobody thought about is recorded anyway, and everything that is not
// configuration is ignored without a list of exceptions to maintain.
func TestTheTapeRecordsWhatMovesTheConfiguration(t *testing.T) {
	f := setup(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(upstream.Close)

	points := func() []store.ConfigPoint {
		t.Helper()
		code, body := f.call(t, "GET", "/api/config/history", "", f.rootC)
		if code != http.StatusOK {
			t.Fatalf("history: %d %s", code, body)
		}
		var out []store.ConfigPoint
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	// A route: configuration, so the tape moves.
	route := strings.Replace(validRoute, "%s", upstream.URL, 1)
	if code, out := f.call(t, "PUT", "/api/routes/r1", route, f.rootC); code != http.StatusOK {
		t.Fatalf("put route: %d %s", code, out)
	}
	first := points()
	if len(first) == 0 {
		t.Fatal("a route was saved and the tape recorded nothing")
	}
	if first[0].Label == "" {
		t.Fatalf("a point must carry the words the audit trail used: %+v", first[0])
	}

	// An ACCOUNT: not configuration. Nothing moves, so nothing is recorded -
	// and no list of exceptions had to say so.
	if code, out := f.call(t, "POST", "/api/users",
		`{"username":"tape","fullname":"Tape T","enabled":true}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("create user: %d %s", code, out)
	}
	if after := points(); len(after) != len(first) {
		t.Fatalf("creating an account wrote to the configuration tape: %d then %d", len(first), len(after))
	}

	// The same route saved again, unchanged: a write that changes nothing.
	if code, _ := f.call(t, "PUT", "/api/routes/r1", route, f.rootC); code != http.StatusOK {
		t.Fatal("re-put route")
	}
	if after := points(); len(after) != len(first) {
		t.Fatalf("a no-op write took a point: %d then %d", len(first), len(after))
	}

	// The state as the tape itself would store it, kept aside to go back to.
	// Read through the API rather than hand-written: a point holds the
	// gateway's OWN rendering of a state, and a document that only looks like
	// it would come back with a different fingerprint.
	code, doc1 := f.call(t, "GET", "/api/config/document", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("document: %d %s", code, doc1)
	}

	// Folding: a second change by the same person, moments later, REPLACES the
	// point rather than adding one. Twenty points for one afternoon of colour
	// picking is a tape nobody reads; the price, stated, is that one cannot go
	// back into the middle of a gesture.
	second := strings.Replace(route, `"name": "api"`, `"name": "second"`, 1)
	second = strings.Replace(second, "/api/**", "/second/**", 1)
	if code, out := f.call(t, "PUT", "/api/routes/r2", second, f.rootC); code != http.StatusOK {
		t.Fatalf("put second route: %d %s", code, out)
	}
	folded := points()
	if len(folded) != len(first) {
		t.Fatalf("a change moments later must fold into the last point: %d then %d", len(first), len(folded))
	}
	if code, body := f.call(t, "GET", "/api/config/history/"+folded[0].ID, "", f.rootC); code != http.StatusOK ||
		!strings.Contains(body, "second") {
		t.Fatalf("the folded point must hold the LATEST state: %d %s", code, body)
	}

	// Going back to the moment before that second route. The point is placed
	// through the store with a timestamp of its own: what the API is being
	// asked here is the restore, and staging it through the console would mean
	// waiting out the folding window in a unit test.
	if err := f.api.st.AddConfigPoint(context.Background(), store.ConfigPoint{
		ID: "before", At: time.Now().Add(-time.Hour).Unix(), ActorID: "root",
		Label: "before the second route", Digest: store.DigestOf(doc1), Document: doc1,
	}); err != nil {
		t.Fatal(err)
	}
	code, plan := f.call(t, "GET", "/api/config/history/before/plan", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("plan: %d %s", code, plan)
	}
	if !strings.Contains(plan, `"action":"remove"`) {
		t.Fatalf("the plan must say what going back removes: %s", plan)
	}
	if code, out := f.call(t, "POST", "/api/config/history/before/restore", "", f.rootC); code != http.StatusOK {
		t.Fatalf("restore: %d %s", code, out)
	}
	if code, body := f.call(t, "GET", "/api/routes/r2", "", f.rootC); code != http.StatusNotFound {
		t.Fatalf("the restore did not take: %d %s", code, body)
	}

	// Going back must never eat the point it undoes - which folding did, and
	// which is what made a change vanish and leave two identical lines where it
	// had been. A return lands on a state the tape already knows, and a known
	// state is never folded, however fast it comes.
	back := points()
	if len(back) <= len(folded) {
		t.Fatalf("the restore folded into the change it undid: %d then %d", len(folded), len(back))
	}
	if back[0].Digest != store.DigestOf(doc1) {
		t.Fatal("the newest point must hold the state that was restored")
	}
	// And the screen is told the two are the same state, so a repeated line
	// reads as "back to then" rather than as a bug.
	var views []struct {
		SameAs  int64 `json:"sameAs"`
		Live    bool  `json:"live"`
		Current bool  `json:"current"`
	}
	_, body := f.call(t, "GET", "/api/config/history", "", f.rootC)
	if err := json.Unmarshal([]byte(body), &views); err != nil {
		t.Fatal(err)
	}
	if views[0].SameAs == 0 {
		t.Fatalf("a point repeating an earlier state must say when it was first seen: %s", body)
	}
	// Both points hold what is being served, and the screen needs to know it
	// for both: ONE is where the gateway is (current), BOTH are states it is
	// already in (live). Without the second, the older row offers a Restore
	// that reports success and changes nothing - which is what it looked like
	// from the outside.
	live := 0
	for _, v := range views {
		if v.Live {
			live++
		}
	}
	if live != 2 {
		t.Fatalf("both points holding the served state must say so: %s", body)
	}
	if !views[0].Current || views[len(views)-1].Current {
		t.Fatalf("only the newest of them is where the gateway is: %s", body)
	}

	// And a moment can be named, which is the one way it crosses to the shelf.
	if code, body := f.call(t, "POST", "/api/config/history/before/save?name=From%20the%20tape",
		"", f.rootC); code != http.StatusCreated {
		t.Fatalf("save a point: %d %s", code, body)
	}
	if _, list := f.call(t, "GET", "/api/configurations", "", f.rootC); !strings.Contains(list, "From the tape") {
		t.Fatalf("the point did not reach the shelf: %s", list)
	}
}
