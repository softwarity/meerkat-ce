package tracing

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// What a caller sends is taken as it stands, because the identifier is the
// whole point: changing it would break the join it exists to make.
func TestParseTraceparentKept(t *testing.T) {
	const in = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	tc, ok := Parse(in)
	if !ok {
		t.Fatalf("%q was refused", in)
	}
	if tc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace id = %q", tc.TraceID)
	}
	if tc.SpanID != "00f067aa0ba902b7" {
		t.Errorf("span id = %q", tc.SpanID)
	}
	if !tc.Sampled {
		t.Error("the sampling bit was read as off, and it is 01")
	}
	if !tc.Inbound {
		t.Error("a context that came from the caller is not marked inbound")
	}
}

// A future version carries more fields and stays readable: that is what the
// version number is for, and refusing it would strand every caller the day
// the standard grows.
func TestParseTraceparentFutureVersion(t *testing.T) {
	const in = "cc-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-what-comes-next"
	tc, ok := Parse(in)
	if !ok {
		t.Fatalf("a later version was refused: %q", in)
	}
	if tc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace id = %q", tc.TraceID)
	}
}

// Nearly right is worse than absent: it makes a service believe it belongs to
// a journey that does not exist. Each of these is replaced by one of ours.
func TestParseTraceparentRefused(t *testing.T) {
	for _, c := range []struct{ why, in string }{
		{"empty", ""},
		{"too short", "00-4bf92f35-00f067aa0ba902b7-01"},
		{"trace id all zeros", "00-00000000000000000000000000000000-00f067aa0ba902b7-01"},
		{"span id all zeros", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01"},
		{"uppercase", "00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01"},
		{"version ff is reserved invalid", "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
		{"not hex", "00-zzf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
		{"no separator after the known fields", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01x"},
	} {
		if _, ok := Parse(c.in); ok {
			t.Errorf("%s: %q was accepted", c.why, c.in)
		}
	}
}

// Nothing came in, so we open the journey - and we do NOT claim anybody should
// report it. The zero flag is what keeps a generated identifier honest.
func TestTraceOpenedIsNotSampled(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost/x", nil)
	tc := FromRequest(req)
	if tc.Inbound {
		t.Error("a journey we opened is marked as coming from the caller")
	}
	if tc.Sampled {
		t.Fatal("a generated trace asks to be exported: that is a dangling parent")
	}
	if len(tc.TraceID) != 32 || isZeros(tc.TraceID) {
		t.Fatalf("trace id = %q", tc.TraceID)
	}
	if !strings.HasSuffix(tc.Header(), "-00") {
		t.Errorf("header does not carry the zero flag: %q", tc.Header())
	}
	// And it is readable by everybody else, which is the only reason to use
	// the standard rather than a header of our own.
	back, ok := Parse(tc.Header())
	if !ok {
		t.Fatalf("we wrote something we cannot read: %q", tc.Header())
	}
	if back.TraceID != tc.TraceID {
		t.Errorf("round trip changed the journey: %q then %q", tc.TraceID, back.TraceID)
	}
}

// Two requests are two journeys. Obvious, and exactly the kind of thing that
// breaks silently the day somebody caches the wrong value.
func TestTraceOpenedIsUnique(t *testing.T) {
	seen := make(map[string]bool, 500)
	for i := 0; i < 500; i++ {
		id := newSpanContext().TraceID
		if seen[id] {
			t.Fatalf("the same journey was opened twice: %q", id)
		}
		seen[id] = true
	}
}

// A malformed header does not travel: it is replaced by a journey of ours,
// rather than passed on for a service to believe in.
func TestTraceReplacesRubbish(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost/x", nil)
	req.Header.Set(Header, "not-a-traceparent")
	tc := FromRequest(req)
	if tc.Inbound {
		t.Fatal("rubbish was taken as a caller's context")
	}
	if tc.Sampled {
		t.Error("and it came back sampled")
	}
}

// The context carries it for everything that has to write it down - a log
// line, an error page, an audit row.
func TestTraceIDFromContext(t *testing.T) {
	if got := ID(t.Context()); got != "" {
		t.Errorf("a request that never came through the door reported %q, want empty", got)
	}
	tc := newSpanContext()
	if got := ID(With(t.Context(), tc)); got != tc.TraceID {
		t.Errorf("TraceID = %q, want %q", got, tc.TraceID)
	}
}
