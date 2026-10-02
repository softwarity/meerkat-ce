package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/tracing"
)

// What a DaemonSet Collector decodes field by field (OBS-03): the names are
// OpenTelemetry's, the trace id is lifted out of the attributes, groups are
// flattened with dots, and the writer is named in the resource.
func TestALineInOTelFormatCarriesTheLogDataModel(t *testing.T) {
	SetResource(map[string]string{"service.name": "meerkat", "service.version": "test"})
	defer SetResource(nil)
	var buf bytes.Buffer
	lg := slog.New(newOTelHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	lg.With("node", "n1").WithGroup("upstream").Warn("the upstream is down",
		"host", "billing:8080", "took", 1500*time.Millisecond, "err", errors.New("refused"),
		slog.String("trace_id", "4bf92f3577b34da6a3ce929d0e0e4736"))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("not one JSON object: %q: %v", buf.String(), err)
	}
	if line["severity_text"] != "WARN" || line["severity_number"] != float64(13) {
		t.Errorf("severity: %v %v", line["severity_text"], line["severity_number"])
	}
	if line["body"] != "the upstream is down" {
		t.Errorf("body: %v", line["body"])
	}
	if _, err := time.Parse(time.RFC3339Nano, line["timestamp"].(string)); err != nil {
		t.Errorf("timestamp %v: %v", line["timestamp"], err)
	}
	attrs, _ := line["attributes"].(map[string]any)
	for k, want := range map[string]any{
		"node": "n1", "upstream.host": "billing:8080", "upstream.took": float64(1500), "upstream.err": "refused",
	} {
		if attrs[k] != want {
			t.Errorf("attribute %s = %v, want %v (all: %v)", k, attrs[k], want, attrs)
		}
	}
	// A trace id written inside a group is still an attribute; one written at
	// the top level is the line's own.
	res, _ := line["resource"].(map[string]any)
	if res["service.name"] != "meerkat" {
		t.Errorf("resource: %v", res)
	}
}

func TestATraceIdAtTheTopLevelIsTheLinesOwn(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(newOTelHandler(&buf, nil))
	lg.Info("access", "trace_id", "4bf92f3577b34da6a3ce929d0e0e4736", "status", 200)
	var line map[string]any
	_ = json.Unmarshal(buf.Bytes(), &line)
	if line["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace_id not lifted: %v", line)
	}
	if attrs, _ := line["attributes"].(map[string]any); attrs["trace_id"] != nil || attrs["status"] != float64(200) {
		t.Errorf("attributes: %v", attrs)
	}
}

func TestTheLevelStillFilters(t *testing.T) {
	var buf bytes.Buffer
	h := newOTelHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("an info line passes a warn level")
	}
}

// The switch, both ways: on writes OTel, off returns to what was chosen at
// startup.
func TestTheSwitchReturnsToTheStartupFormat(t *testing.T) {
	base.Store(FormatJSON)
	defer func() { SetOTel(false); base.Store(Format("")) }()
	SetOTel(true)
	if current() != FormatOTel {
		t.Fatalf("on: %s", current())
	}
	SetOTel(false)
	if current() != FormatJSON {
		t.Fatalf("off: %s, want the startup json", current())
	}
}

// Pushed: the line leaves for the collector as an OTLP log marked
// meerkat.stream=logs, with its severity and attributes - and the output keeps
// writing it too.
func TestAPushedLineLeavesAndStillIsWritten(t *testing.T) {
	sent := make(chan string, 4)
	tracing.RegisterAuditStarter(func(tracing.Config) (func([]byte) error, error) {
		return func(b []byte) error { sent <- string(b); return nil }, nil
	})
	if err := tracing.ApplyLogs(tracing.Config{Endpoint: "http://collector"}, true); err != nil {
		t.Fatal(err)
	}
	defer func() {
		SetPush(false)
		_ = tracing.ApplyLogs(tracing.Config{}, false)
		tracing.RegisterAuditStarter(nil)
	}()
	base.Store(FormatJSON)
	SetPush(true)
	slog.Warn("the upstream is down", "route", "billing")
	select {
	case b := <-sent:
		for _, want := range []string{`"meerkat.stream"`, `"logs"`, `"the upstream is down"`, `"WARN"`, `"route"`} {
			if !strings.Contains(b, want) {
				t.Errorf("missing %s in %s", want, b)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was pushed")
	}
}
