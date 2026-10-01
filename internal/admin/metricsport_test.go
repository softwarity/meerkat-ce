package admin

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/metrics"
)

// freePort is a port nothing listens on, as of now.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// scrape reads the metrics port the way a Prometheus does.
func scrape(t *testing.T, port int, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(port)+"/metrics", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res.StatusCode, string(b)
}

// The metrics port is chosen in the console with the switch, opened at once,
// open to a scraper with no credential - the network is the lock, as for
// PostgreSQL's and RabbitMQ's exporters - and closed with the switch.
func TestTheMetricsPortOpensWhereTheConsoleSays(t *testing.T) {
	f := setup(t)
	if exposition == nil {
		exposition = func(w io.Writer, _ *metrics.Registry) { _, _ = io.WriteString(w, "meerkat_up 1\n") }
		t.Cleanup(func() { exposition = nil })
	}
	f.api.ServeMetricsPort()
	f.api.metricsDoor.host = "127.0.0.1"
	t.Cleanup(func() { _ = f.api.StopMetricsPort(context.Background()) })

	port := freePort(t)
	code, out := f.call(t, "PUT", "/api/settings/metrics",
		`{"enabled":true,"metricsPort":`+strconv.Itoa(port)+`}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("switching it on: %d %s", code, out)
	}
	var got metricsSetting
	_ = json.Unmarshal([]byte(out), &got)
	if got.MetricsPort != port || got.RequireToken {
		t.Errorf("the answer does not say what was decided: %s", out)
	}
	if code, body := scrape(t, port, ""); code != http.StatusOK || !strings.Contains(body, "meerkat_up") {
		t.Fatalf("a scrape with no token was not answered: %d %s", code, body)
	}
	// Only the counters: this port is worth nothing to somebody after the
	// console or the API.
	res, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("the metrics port answered the API: %d", res.StatusCode)
	}

	// Asking for a token closes it to a bare scrape, and a metrics token
	// opens it - checked by the control plane's own funnel.
	if code, out := f.call(t, "PUT", "/api/settings/metrics",
		`{"enabled":true,"requireToken":true,"metricsPort":`+strconv.Itoa(port)+`}`, f.rootC); code != http.StatusOK {
		t.Fatalf("asking for a token: %d %s", code, out)
	}
	if code, _ := scrape(t, port, ""); code != http.StatusUnauthorized {
		t.Errorf("a token is required and a bare scrape got %d", code)
	}
	code, body := f.call(t, "POST", "/api/admin-tokens", `{"name":"prometheus","scope":"metrics"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("minting a metrics token: %d %s", code, body)
	}
	var minted struct{ Token string }
	_ = json.Unmarshal([]byte(body), &minted)
	if code, body := scrape(t, port, minted.Token); code != http.StatusOK {
		t.Errorf("the metrics token was refused: %d %s", code, body)
	}

	// Moving it moves it.
	moved := freePort(t)
	if code, out := f.call(t, "PUT", "/api/settings/metrics",
		`{"enabled":true,"metricsPort":`+strconv.Itoa(moved)+`}`, f.rootC); code != http.StatusOK {
		t.Fatalf("moving it: %d %s", code, out)
	}
	if code, _ := scrape(t, moved, ""); code != http.StatusOK {
		t.Errorf("nothing answers on the new port: %d", code)
	}
	if code, _ := scrape(t, port, ""); code != 0 {
		t.Errorf("the old port still answers: %d", code)
	}

	// And the switch off closes it: no switch, no listener.
	if code, out := f.call(t, "PUT", "/api/settings/metrics",
		`{"enabled":false,"metricsPort":`+strconv.Itoa(moved)+`}`, f.rootC); code != http.StatusOK {
		t.Fatalf("switching it off: %d %s", code, out)
	}
	if code, _ := scrape(t, moved, ""); code != 0 {
		t.Errorf("the port is still open with the exposition off: %d", code)
	}
}

// A port this gateway cannot open is refused with the reason, and nothing is
// saved: a setting the gateway cannot carry out would read as working.
func TestATakenMetricsPortIsRefused(t *testing.T) {
	f := setup(t)
	f.api.ServeMetricsPort()
	f.api.metricsDoor.host = "127.0.0.1"
	t.Cleanup(func() { _ = f.api.StopMetricsPort(context.Background()) })

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = busy.Close() })
	port := busy.Addr().(*net.TCPAddr).Port

	code, out := f.call(t, "PUT", "/api/settings/metrics",
		`{"enabled":true,"metricsPort":`+strconv.Itoa(port)+`}`, f.rootC)
	if code != http.StatusConflict || !strings.Contains(out, strconv.Itoa(port)) {
		t.Fatalf("a taken port answered %d: %s", code, out)
	}
	if f.api.metricsEnabled(context.Background()) {
		t.Error("the refused decision was kept")
	}
}

// The two ports this gateway already serves are not a metrics port, and the
// sentence says which one it collided with.
func TestTheMetricsPortIsNotAnotherPlane(t *testing.T) {
	f := setup(t)
	f.api.DataAddr = ":8080"
	for body, want := range map[string]string{
		`{"enabled":false,"metricsPort":9090}`:  "control plane",
		`{"enabled":false,"metricsPort":8080}`:  "applications",
		`{"enabled":false,"metricsPort":70000}`: "1 to 65535",
	} {
		code, out := f.call(t, "PUT", "/api/settings/metrics", body, f.rootC)
		if code != http.StatusUnprocessableEntity || !strings.Contains(out, want) {
			t.Errorf("%s answered %d: %s", body, code, out)
		}
	}
}
