package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// A typo in a log level must not stop a gateway from starting, and Info is the
// level that will tell them about the typo.
func TestParseLevelFallsBackToInfo(t *testing.T) {
	for _, c := range []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{" warn ", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"verbose", slog.LevelInfo},
	} {
		if got := ParseLevel(c.in); got != c.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// Whatever an operator types, they get the same word back - so the console and
// the API agree with each other.
func TestLevelNameRoundTrip(t *testing.T) {
	for _, name := range []string{"debug", "info", "warn", "error"} {
		if got := LevelName(ParseLevel(name)); got != name {
			t.Errorf("%q came back as %q", name, got)
		}
	}
}

// Turning it up and back down without restarting anything is the whole reason
// the level lives behind a pointer.
func TestSetLevelIsLive(t *testing.T) {
	before := Level()
	t.Cleanup(func() { level.Set(before) })

	SetLevel("debug")
	if Level() != slog.LevelDebug {
		t.Fatalf("level = %v after asking for debug", Level())
	}
	SetLevel("warn")
	if Level() != slog.LevelWarn {
		t.Fatalf("level = %v after asking for warn", Level())
	}
}

// Off is the default, and writing while off must cost nothing and produce
// nothing - an installation that never asked for an access log must not find
// thirty-five million lines a day in its collector.
func TestAccessLogIsOffByDefault(t *testing.T) {
	t.Cleanup(DisableAccessLog)
	DisableAccessLog()
	if AccessLogEnabled() {
		t.Fatal("the access log reports itself on before anybody asked")
	}
	Write(Access{TraceID: "x", Method: "GET", Path: "/"}) // must not panic
}

// What one line carries, and what it deliberately leaves out.
func TestAccessLineShape(t *testing.T) {
	var buf bytes.Buffer
	accessLogger.Store(newAccessLogger(&buf, FormatJSON))
	t.Cleanup(DisableAccessLog)

	Write(Access{
		TraceID:  "4bf92f3577b34da6a3ce929d0e0e4736",
		Method:   "GET",
		Path:     "/patients/42",
		Endpoint: "/patients/{id}",
		Route:    "dmp-api",
		Status:   200,
		Duration: 45 * time.Millisecond,
		User:     "dr.martin",
		IP:       "10.0.0.7",
		Outcome:  "ok",
	})

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("the access log is not JSON: %v\n%s", err, buf.String())
	}
	// The discriminator is what lets one stream carry two logs.
	if line["type"] != "access" {
		t.Errorf(`type = %v, want "access"`, line["type"])
	}
	// The join key, without which this line answers "somebody did something".
	if line["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace_id = %v", line["trace_id"])
	}
	// The identifier IS the audit: a path without it audits nothing.
	if line["path"] != "/patients/42" {
		t.Errorf("path = %v", line["path"])
	}
	if line["endpoint"] != "/patients/{id}" {
		t.Errorf("endpoint = %v", line["endpoint"])
	}
	if line["ms"] != float64(45) {
		t.Errorf("ms = %v", line["ms"])
	}
	// An absent field is absent, not written empty: `"tenant":""` reads as an
	// answer when it is the absence of one.
	for _, absent := range []string{"tenant", "token", "bytes"} {
		if _, there := line[absent]; there {
			t.Errorf("%q was written although it was never set", absent)
		}
	}
}

// A refusal is a line, and it is the one a service can never write: it never
// saw the call.
func TestAccessLineRecordsARefusal(t *testing.T) {
	var buf bytes.Buffer
	accessLogger.Store(newAccessLogger(&buf, FormatJSON))
	t.Cleanup(DisableAccessLog)

	Write(Access{
		TraceID: "abc", Method: "GET", Path: "/patients/42",
		Status: 403, Outcome: "refused", User: "intern",
	})
	out := buf.String()
	for _, want := range []string{`"status":403`, `"outcome":"refused"`, `"user":"intern"`} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal line does not carry %s:\n%s", want, out)
		}
	}
}

// Quieting the gateway must not silently take the audit trail with it.
func TestAccessLogIgnoresTheOperationalLevel(t *testing.T) {
	before := Level()
	t.Cleanup(func() { level.Set(before) })
	t.Cleanup(DisableAccessLog)

	var buf bytes.Buffer
	accessLogger.Store(newAccessLogger(&buf, FormatJSON))
	SetLevel("error")

	Write(Access{TraceID: "abc", Method: "GET", Path: "/", Status: 200, Outcome: "ok"})
	if buf.Len() == 0 {
		t.Fatal("setting the gateway to error silenced the access log")
	}
}
