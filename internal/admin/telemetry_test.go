package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Asking the collector whether it answers, before saving the address: the
// mistake this catches is the one that otherwise shows up three days later in
// an empty Jaeger.
func TestTheCollectorProbeSaysWhatAnswered(t *testing.T) {
	var got struct {
		path        string
		contentType string
		auth        string
		body        string
	}
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.path, got.contentType, got.auth, got.body = r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("Authorization"), string(b)
		// As a collector answers: the format it was sent.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
	}))
	t.Cleanup(collector.Close)

	f := setup(t)
	code, out := f.call(t, "POST", "/api/settings/telemetry/test",
		`{"endpoint":"`+collector.URL+`","headers":{"Authorization":"Bearer plain-token"}}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("probe: %d %s", code, out)
	}
	var answer struct {
		Status int   `json:"status"`
		Ms     int64 `json:"ms"`
	}
	if err := json.Unmarshal([]byte(out), &answer); err != nil {
		t.Fatalf("answer: %v (%s)", err, out)
	}
	if answer.Status != http.StatusOK {
		t.Errorf("the answer does not carry what the collector said: %s", out)
	}

	// The path the exporter writes, not the one somebody typed.
	if got.path != "/v1/traces" {
		t.Errorf("the probe called %q, want /v1/traces", got.path)
	}
	// An EMPTY batch: the whole path is exercised and no span is recorded in
	// anybody's backend. A probe that wrote a fake trace is a probe people
	// stop running.
	if strings.Contains(got.body, "spanId") || !strings.Contains(got.body, `"resourceSpans":[]`) {
		t.Errorf("the probe posted something other than an empty batch: %s", got.body)
	}
	if got.contentType != "application/json" {
		t.Errorf("content type %q", got.contentType)
	}
	if got.auth != "Bearer plain-token" {
		t.Errorf("the auth header did not travel: %q", got.auth)
	}
}

// A collector that refuses is reported as a refusal, and the two mistakes that
// actually happen are named rather than left as a number.
func TestTheCollectorProbeNamesTheLikelyCause(t *testing.T) {
	ui := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "404 page not found", http.StatusNotFound)
	}))
	t.Cleanup(ui.Close)

	f := setup(t)
	code, out := f.call(t, "POST", "/api/settings/telemetry/test",
		`{"endpoint":"`+ui.URL+`"}`, f.rootC)
	if code != http.StatusBadGateway {
		t.Fatalf("a 404 from the collector answered %d: %s", code, out)
	}
	if !strings.Contains(out, "4318") {
		t.Errorf("the refusal does not point at the OTLP port: %s", out)
	}
	// And the collector's own body is NOT handed back: this endpoint makes the
	// gateway fetch an address somebody gave it, and returning what came back
	// would make it a reader of whatever the gateway can reach.
	if strings.Contains(out, "page not found") {
		t.Errorf("the collector's body was returned to the caller: %s", out)
	}
}

// A 200 is not an answer by itself: pointed at a collector's UI, the probe
// lands on a single-page application that serves its index for any path. This
// is the mistake measured against a real cluster - Jaeger answered 200 on both
// 4318 and 16686 - and blessing the second one would send an installation off
// to export nothing.
func TestTheCollectorProbeRefusesAWebPage(t *testing.T) {
	ui := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><body>Jaeger UI</body></html>"))
	}))
	t.Cleanup(ui.Close)

	f := setup(t)
	code, out := f.call(t, "POST", "/api/settings/telemetry/test", `{"endpoint":"`+ui.URL+`"}`, f.rootC)
	if code != http.StatusBadGateway {
		t.Fatalf("a page answering 200 was taken for a collector: %d %s", code, out)
	}
	if !strings.Contains(out, "16686") {
		t.Errorf("the refusal does not name the mistake: %s", out)
	}
	if strings.Contains(out, "Jaeger UI") {
		t.Errorf("the page's body was returned to the caller: %s", out)
	}
}

// And a collector that answers 200 with nothing at all is accepted: some do.
func TestTheCollectorProbeAcceptsAnEmptyAnswer(t *testing.T) {
	quiet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(quiet.Close)

	f := setup(t)
	if code, out := f.call(t, "POST", "/api/settings/telemetry/test", `{"endpoint":"`+quiet.URL+`"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("a bare 200 was refused: %d %s", code, out)
	}
}

// An address that cannot work is refused before anything is dialled, with the
// same sentence the setting itself would give.
func TestTheCollectorProbeRefusesAnImpossibleAddress(t *testing.T) {
	f := setup(t)
	code, out := f.call(t, "POST", "/api/settings/telemetry/test", `{"endpoint":"collector:4318"}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("an address with no scheme answered %d: %s", code, out)
	}
	if !strings.Contains(out, "http://") {
		t.Errorf("the refusal does not say what is allowed: %s", out)
	}
}

// Where a gateway sends its own spans is INFRASTRUCTURE, like TLS or the mail
// relay: whoever edits routes configures it, and the editor that draws a
// route's two switches reads it to know whether they do anything yet.
func TestTheTelemetrySettingBelongsToWhoeverEditsRoutes(t *testing.T) {
	f := setup(t)
	if err := f.api.st.CreateUser(context.Background(), store.User{
		ID: "infra", Username: "infra", PasswordHash: "x", InfraAdmin: true, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	infraC := issue(t, f.api.sm, "infra")

	code, out := f.call(t, "GET", "/api/settings/telemetry", "", infraC)
	if code != http.StatusOK {
		t.Fatalf("an infra admin was refused the setting: %d %s", code, out)
	}
	if !strings.Contains(out, `"enabled"`) {
		t.Errorf("the setting does not say whether the export is on: %s", out)
	}
	// And they can write it: a setting one may read but not change is a screen
	// that greys itself out for no reason anybody can act on.
	if code, out := f.call(t, "PUT", "/api/settings/telemetry",
		`{"enabled":false,"endpoint":"","sample":0.1,"maxPerSecond":200}`, infraC); code != http.StatusOK {
		t.Fatalf("an infra admin was refused the write: %d %s", code, out)
	}

	// Somebody who administers nothing has no business here.
	if code, _ := f.call(t, "GET", "/api/settings/telemetry", "", f.plainC); code == http.StatusOK {
		t.Error("a plain user read the telemetry setting")
	}
}

// A backend that takes traces and nothing else - Jaeger - is named as such
// when the metrics are asked about too: otherwise every push is refused while
// the traces arrive and the screen looks fine.
func TestTheCollectorProbeSaysWhenMetricsAreNotTaken(t *testing.T) {
	tracesOnly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(tracesOnly.Close)

	f := setup(t)
	if code, out := f.call(t, "POST", "/api/settings/telemetry/test", `{"endpoint":"`+tracesOnly.URL+`"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("traces alone were refused: %d %s", code, out)
	}
	code, out := f.call(t, "POST", "/api/settings/telemetry/test", `{"endpoint":"`+tracesOnly.URL+`","metrics":true}`, f.rootC)
	if code != http.StatusBadGateway || !strings.Contains(out, "not metrics") || !strings.Contains(out, "Metrics tab off") {
		t.Fatalf("a traces-only backend passed for metrics: %d %s", code, out)
	}
}

// Switched on with nothing to send, the export is refused with the choice.
func TestAnExportThatSendsNothingIsRefused(t *testing.T) {
	f := setup(t)
	code, out := f.call(t, "PUT", "/api/settings/telemetry",
		`{"enabled":true,"traces":false,"metrics":false,"endpoint":"http://collector:4318","sample":0.1,"maxPerSecond":200}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "traces, metrics or the audit") {
		t.Fatalf("an export sending nothing answered %d: %s", code, out)
	}
}
