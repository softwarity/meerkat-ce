package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// fakeCallers is the runtime as a test wants it: which bearers the cluster
// vouches for, and what holds which address. Every request of these tests
// comes from 127.0.0.1.
type fakeCallers struct {
	kube    bool
	reviews map[string]string
	addrs   map[string]string
	broken  bool
	asked   int
}

func (f *fakeCallers) Kubernetes() bool { return f.kube }

func (f *fakeCallers) Review(_ context.Context, token string) (string, error) {
	f.asked++
	if f.broken {
		return "", errors.New("the API server did not answer")
	}
	return f.reviews[token], nil
}

func (f *fakeCallers) ByAddress(_ context.Context, ip string) (string, string) {
	id := f.addrs[ip]
	if id == "" {
		return "", ""
	}
	if f.kube {
		return id, store.CallerWorkload
	}
	return id, store.CallerContainer
}

func (f *fakeCallers) ServiceAccounts(context.Context) ([]string, error) {
	return []string{"canopy/default", "canopy/reference-data"}, nil
}

func (f *fakeCallers) Workloads(context.Context) ([]string, error) {
	return []string{"canopy/Deployment/reference-data"}, nil
}

func setAccess(t *testing.T, f fixture, mode string, callers ...string) {
	t.Helper()
	if callers == nil {
		callers = []string{}
	}
	if err := f.api.st.SetInternalAccess(context.Background(), store.InternalAccess{Mode: mode, Callers: callers}); err != nil {
		t.Fatal(err)
	}
}

// tokenMode is today's internal plane: a Meerkat token, and nothing else.
func tokenMode(t *testing.T, f fixture) { setAccess(t, f, store.InternalAccessToken) }

// created reads back who the gateway recorded as a schedule's creator.
func created(t *testing.T, body string) store.Schedule {
	t.Helper()
	var sc store.Schedule
	if err := json.Unmarshal([]byte(body), &sc); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return sc
}

// TestTheModeDecidesWhoGetsIn is the whole table: each mode, a caller that is
// identified or not, a list or none. The list is compared with the identity
// the mode gives; an unidentified caller passes only when there is no list.
func TestTheModeDecidesWhoGetsIn(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "stations")
	runtime := &fakeCallers{reviews: map[string]string{"sa-token": "canopy/reference-data"}, addrs: map[string]string{}}
	f.api.Callers = runtime
	srv := internalPlane(t, f)

	cases := []struct {
		name    string
		kube    bool
		at      string // what holds 127.0.0.1, "" for nothing
		mode    string
		callers []string
		bearer  string
		code    int
		who     string // the caller recorded, on a 201
		via     string
		says    string // a word the refusal must carry
	}{
		// None: nothing asked; the caller is whatever can be worked out.
		{name: "none, nobody known, no list", mode: "none", code: 201, via: store.CallerUnidentified},
		{name: "none, nobody known, a list", mode: "none", callers: []string{"canopy/Deployment/reference-data"},
			code: 403, says: "could not be identified"},
		{name: "none, address known, no list", kube: true, at: "canopy/Deployment/reference-data", mode: "none",
			code: 201, who: "canopy/Deployment/reference-data", via: store.CallerWorkload},
		{name: "none, address known, listed", kube: true, at: "canopy/Deployment/reference-data", mode: "none",
			callers: []string{"canopy/Deployment/reference-data"}, code: 201, who: "canopy/Deployment/reference-data",
			via: store.CallerWorkload},
		{name: "none, address known, not listed", kube: true, at: "canopy/Deployment/other", mode: "none",
			callers: []string{"canopy/Deployment/reference-data"}, code: 403, says: "canopy/Deployment/reference-data"},
		{name: "none, a container on Docker", at: "orders", mode: "none", callers: []string{"orders"},
			code: 201, who: "orders", via: store.CallerContainer},
		{name: "none, a Meerkat token", mode: "none", bearer: token, code: 201, who: "stations", via: store.CallerToken},
		{name: "none, a token whose workload is listed", kube: true, at: "canopy/Deployment/reference-data", mode: "none",
			callers: []string{"canopy/Deployment/reference-data"}, bearer: token, code: 201, who: "stations",
			via: store.CallerToken},
		{name: "none, a ServiceAccount token", kube: true, mode: "none", bearer: "sa-token", code: 201,
			who: "canopy/reference-data", via: store.CallerServiceAccount},
		{name: "none, a bearer nobody vouches for", kube: true, mode: "none", bearer: "forged", code: 401,
			says: "does not vouch"},
		{name: "none, a foreign bearer off Kubernetes", mode: "none", bearer: "forged", code: 401, says: "mk_"},
		{name: "none, a revoked Meerkat token", mode: "none", bearer: "mk_revoked", code: 401},
		// Token: today's internal plane.
		{name: "token, none sent", mode: "token", code: 401, says: "Meerkat token"},
		{name: "token, sent, no list", mode: "token", bearer: token, code: 201, who: "stations", via: store.CallerToken},
		{name: "token, sent, listed", mode: "token", callers: []string{"stations"}, bearer: token, code: 201,
			who: "stations", via: store.CallerToken},
		{name: "token, sent, not listed", mode: "token", callers: []string{"billing"}, bearer: token, code: 403,
			says: "billing"},
		{name: "token, a ServiceAccount's instead", kube: true, mode: "token", bearer: "sa-token", code: 401},
		// Cluster: the pod's own ServiceAccount token, Kubernetes only.
		{name: "cluster, off Kubernetes", mode: "cluster", bearer: "sa-token", code: 503, says: "does not run on Kubernetes"},
		{name: "cluster, none sent", kube: true, mode: "cluster", code: 401, says: "ServiceAccount token"},
		{name: "cluster, a Meerkat token", kube: true, mode: "cluster", bearer: token, code: 401, says: "not accepted"},
		{name: "cluster, vouched for, no list", kube: true, mode: "cluster", bearer: "sa-token", code: 201,
			who: "canopy/reference-data", via: store.CallerServiceAccount},
		{name: "cluster, vouched for, listed", kube: true, mode: "cluster", callers: []string{"canopy/reference-data"},
			bearer: "sa-token", code: 201, who: "canopy/reference-data", via: store.CallerServiceAccount},
		{name: "cluster, vouched for, not listed", kube: true, mode: "cluster", callers: []string{"canopy/billing"},
			bearer: "sa-token", code: 403, says: "canopy/billing"},
		{name: "cluster, not vouched for", kube: true, mode: "cluster", bearer: "forged", code: 401},
	}
	for _, c := range cases {
		runtime.kube = c.kube
		runtime.addrs["127.0.0.1"] = c.at
		setAccess(t, f, c.mode, c.callers...)
		code, body := on(t, srv, "POST", "/api/schedules", stationSchedule, c.bearer, nil)
		if code != c.code {
			t.Errorf("%s: %d, want %d: %s", c.name, code, c.code, body)
			continue
		}
		if c.says != "" && !strings.Contains(body, c.says) {
			t.Errorf("%s: the refusal does not say %q: %s", c.name, c.says, body)
		}
		if code != http.StatusCreated {
			continue
		}
		sc := created(t, body)
		if sc.Caller != c.who || sc.CallerVia != c.via || sc.CallerAddr != "127.0.0.1" {
			t.Errorf("%s: recorded %q by %q from %q, want %q by %q", c.name, sc.Caller, sc.CallerVia, sc.CallerAddr, c.who, c.via)
		}
	}
}

// TestWhoCreatedItIsKept: the creator is the gateway's to write, at creation -
// not what the body claims, and not changed by a later write.
func TestWhoCreatedItIsKept(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	f.api.Callers = &fakeCallers{kube: true, addrs: map[string]string{"127.0.0.1": "canopy/Deployment/reference-data"}}
	srv := internalPlane(t, f)
	forged := strings.Replace(stationSchedule, `"name"`, `"caller":"root","callerVia":"token","name"`, 1)
	code, body := on(t, srv, "POST", "/api/schedules", forged, "", nil)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	sc := created(t, body)
	if sc.Caller != "canopy/Deployment/reference-data" || sc.CallerVia != store.CallerWorkload {
		t.Fatalf("the body chose its creator: %+v", sc)
	}
	token := serviceToken(t, f, "stations")
	if code, body := on(t, srv, "PUT", "/api/schedules/"+sc.ID, stationSchedule, token, nil); code != http.StatusOK {
		t.Fatalf("update: %d %s", code, body)
	}
	got, err := f.api.st.GetSchedule(context.Background(), sc.ID)
	if err != nil || got.Caller != "canopy/Deployment/reference-data" || got.CallerVia != store.CallerWorkload {
		t.Fatalf("a later writer replaced the creator: %+v %v", got, err)
	}
	// And the trail says who wrote it, and how that was known.
	events, _ := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{TargetID: sc.ID})
	found := false
	for _, e := range events {
		if e.Action == "schedule.create" && e.ActorID == "workload canopy/Deployment/reference-data" &&
			strings.Contains(e.Detail, "deduced from its address 127.0.0.1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the audit does not say who created it: %+v", events)
	}
}

// TestTheJWKSStaysOpenInEveryMode: public keys, whatever the scheduler asks.
func TestTheJWKSStaysOpenInEveryMode(t *testing.T) {
	f := setupBare(t)
	srv := internalPlane(t, f)
	for _, mode := range store.InternalAccessModes {
		setAccess(t, f, mode, "somebody")
		for _, path := range []string{"/.well-known/jwks.json", DiscoveryPath} {
			if code, body := on(t, srv, "GET", path, "", "", nil); code != http.StatusOK {
				t.Errorf("mode %s, %s: %d %s", mode, path, code, body)
			}
		}
	}
}

// TestTheControlPlaneDoorStaysTokenOnly: the compatibility path for services
// that have not moved answers their token and nothing else, whatever the
// internal plane's mode - None opens the internal plane, not the console's.
func TestTheControlPlaneDoorStaysTokenOnly(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	setAccess(t, f, store.InternalAccessNone)
	req, _ := http.NewRequest("POST", f.adminSrv.URL+"/api/schedules", strings.NewReader(stationSchedule))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("nothing sent on the control plane: %d", res.StatusCode)
	}
	token := serviceToken(t, f, "stations")
	code, body := f.bearerJSON(t, "POST", "/api/schedules", stationSchedule, token)
	if code != http.StatusCreated {
		t.Fatalf("a service token on the old address: %d %s", code, body)
	}
	if sc := created(t, body); sc.Caller != "stations" || sc.CallerVia != store.CallerToken {
		t.Fatalf("the old door recorded %+v", sc)
	}
}

// TestAnUnreachableClusterIsSaid: a token nobody could check is neither let
// in nor called forged - the refusal says the question could not be asked.
func TestAnUnreachableClusterIsSaid(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	f.api.Callers = &fakeCallers{kube: true, broken: true}
	srv := internalPlane(t, f)
	setAccess(t, f, store.InternalAccessCluster)
	code, body := on(t, srv, "POST", "/api/schedules", stationSchedule, "sa-token", nil)
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "could not check") {
		t.Fatalf("an API server that does not answer: %d %s", code, body)
	}
}

// TestTheSettingIsReadByAdminsAndChangedByInfra: the Scheduler's readers see
// it, the infrastructure's administrators change it, and the change is in the
// trail. Cluster is refused where it cannot be checked.
func TestTheSettingIsReadByAdminsAndChangedByInfra(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()
	for _, u := range []store.User{
		{ID: "infra", Username: "infra", PasswordHash: "x", InfraAdmin: true, Enabled: true},
		{ID: "app", Username: "app", PasswordHash: "x", AppAdmin: true, Enabled: true},
	} {
		if err := f.api.st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	infra, app := issue(t, f.api.sm, "infra"), issue(t, f.api.sm, "app")
	const path = "/api/settings/internal-access"

	code, body := f.call(t, "GET", path, "", app)
	if code != http.StatusOK || !strings.Contains(body, `"mode":"none"`) || !strings.Contains(body, `"editable":false`) {
		t.Fatalf("a fresh installation, read by an app admin: %d %s", code, body)
	}
	if code, _ := f.call(t, "GET", path, "", f.plainC); code != http.StatusForbidden {
		t.Fatalf("a person administering nothing read it: %d", code)
	}
	if code, _ := f.call(t, "PUT", path, `{"mode":"token","callers":[]}`, app); code != http.StatusForbidden {
		t.Fatalf("an app admin changed it: %d", code)
	}
	code, body = f.call(t, "PUT", path, `{"mode":"token","callers":["stations"," stations ",""]}`, infra)
	if code != http.StatusOK || !strings.Contains(body, `"callers":["stations"]`) || !strings.Contains(body, `"editable":true`) {
		t.Fatalf("an infra admin sets it: %d %s", code, body)
	}
	if a := f.api.st.InternalAccess(ctx); a.Mode != store.InternalAccessToken {
		t.Fatalf("stored %+v", a)
	}
	code, body = f.call(t, "PUT", path, `{"mode":"cluster","callers":[]}`, infra)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "none and token") {
		t.Fatalf("cluster off Kubernetes: %d %s", code, body)
	}
	if code, body := f.call(t, "PUT", path, `{"mode":"open","callers":[]}`, infra); code != http.StatusUnprocessableEntity ||
		!strings.Contains(body, "none, token, cluster") {
		t.Fatalf("a mode that does not exist: %d %s", code, body)
	}
	f.api.Callers = &fakeCallers{kube: true}
	if code, body := f.call(t, "PUT", path, `{"mode":"cluster","callers":["canopy/default"]}`, infra); code != http.StatusOK ||
		!strings.Contains(body, `"cluster"]`) {
		t.Fatalf("cluster on Kubernetes: %d %s", code, body)
	}
	events, _ := f.api.st.ListAuditEvents(ctx, store.AuditFilter{Target: "settings"})
	n := 0
	for _, e := range events {
		if e.Action == "settings.internal-access" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d audit lines for two changes: %+v", n, events)
	}

	// What the list offers, per mode.
	serviceToken(t, f, "stations")
	for mode, want := range map[string]string{
		"token":   `"stations"`,
		"cluster": `"canopy/default"`,
		"none":    `"canopy/Deployment/reference-data"`,
	} {
		code, body := f.call(t, "GET", path+"/candidates?mode="+mode, "", app)
		if code != http.StatusOK || !strings.Contains(body, want) {
			t.Errorf("candidates for %s: %d %s", mode, code, body)
		}
	}
}

// TestARunIsReportedOnItsIdentifierInEveryMode: a report answers a call the
// gateway made, so the run identifier that call carried is the proof - not the
// access mode's credential. A service holding a Meerkat token still closes its
// run after the mode moved to the cluster's ServiceAccount, and a run nobody
// was handed is refused.
func TestARunIsReportedOnItsIdentifierInEveryMode(t *testing.T) {
	f := setupBare(t)
	aRoute(t, f)
	token := serviceToken(t, f, "stations")
	f.api.Callers = &fakeCallers{kube: true, addrs: map[string]string{}}
	srv := internalPlane(t, f)
	setAccess(t, f, "token")
	code, body := on(t, srv, "POST", "/api/schedules", stationSchedule, token, nil)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	sc := created(t, body)
	ctx := context.Background()
	for i, mode := range []string{"cluster", "token", "none"} {
		setAccess(t, f, mode)
		run := fmt.Sprintf("run-%d", i)
		if ok, err := f.api.st.ClaimSchedule(ctx, sc.ID, run, "node-a", time.Now().Add(time.Duration(i)*time.Hour)); err != nil || !ok {
			t.Fatalf("%s: claim: %v %v", mode, ok, err)
		}
		if code, body := on(t, srv, "PATCH", "/api/schedules/"+sc.ID+"/run",
			`{"run":"not-this-one","state":"done"}`, token, nil); code != http.StatusConflict {
			t.Errorf("%s: a run nobody was handed was taken: %d %s", mode, code, body)
		}
		if code, body := on(t, srv, "PATCH", "/api/schedules/"+sc.ID+"/run",
			`{"run":"`+run+`","state":"done"}`, token, nil); code != http.StatusNoContent {
			t.Errorf("%s: the run it was handed was refused: %d %s", mode, code, body)
		}
	}
}
