package logging

import (
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

// The access log: one line per request that crossed the front door (OBS-03).
//
// WHAT IT IS FOR is not debugging, it is the PROOF OF ACCESS - the half of an
// audit only a gateway can produce. The service behind knows what an operation
// meant; it does not know, and cannot prove, who was on the other end, because
// all it has is what we put in a header. We authenticated, so we owe the
// record. And what a service can never write at all is the call it never saw:
// a refusal. Those are often the most interesting lines here.
//
// What it does NOT carry is the body. `POST /orders/12/validate` is recorded;
// "exceptional 40% discount" is the service's to write, under the same
// trace_id, in its own audit. Reading bodies to guess at meaning would cost
// throughput and store other people's data for no gain.
//
// It goes to standard OUTPUT while everything else goes to standard error, and
// every line carries `"type":"access"` besides, so a collector can route it
// whichever of the two ways it prefers.

// accessLogger is nil until Setup turns it on, and swapped rather than locked:
// a hot path must not take a mutex to find out whether anybody is listening.
var accessLogger atomic.Pointer[slog.Logger]

// EnableAccessLog starts writing access lines, in the format already chosen.
// Off by default, and that is deliberate: at four hundred requests a second
// this is thirty-five million lines a day, and an operator reading `docker
// logs` for a startup problem should not have to opt OUT of that.
func EnableAccessLog(format Format) { EnableAccessLogTo(os.Stdout, format) }

// EnableAccessLogTo is EnableAccessLog onto another output: what a test reads
// the lines from.
func EnableAccessLogTo(w io.Writer, format Format) {
	if otelOn.Load() {
		format = FormatOTel
	}
	accessLogger.Store(newAccessLogger(w, format))
}

// DisableAccessLog stops, live. Turning it on for an hour to catch something
// is the ordinary use, so turning it off again has to be as cheap.
func DisableAccessLog() { accessLogger.Store(nil) }

// AccessLogEnabled answers the console and the API.
func AccessLogEnabled() bool { return accessLogger.Load() != nil }

func newAccessLogger(w io.Writer, format Format) *slog.Logger {
	// Its OWN level, fixed: an access log is a record, not a severity. An
	// operator who quiets the gateway to Error must not silently lose the
	// audit trail as a side effect.
	return slog.New(withPush(handlerFor(w, format, &slog.HandlerOptions{Level: slog.LevelInfo}), slog.LevelInfo))
}

// Access is one crossing of the front door.
type Access struct {
	// TraceID is the join: the same string the service writes in its own
	// audit, so the two halves are one story. Present on every request,
	// whether or not anybody is tracing (OBS-04).
	TraceID string
	Method  string
	// Path as it was asked for. It carries identifiers - `/patients/42` - and
	// that is the point: an audit of data access is an audit of WHICH datum.
	Path string
	// Endpoint is the template the path matched (`/patients/{id}`), when the
	// route declares a spec. It is what makes these lines countable without
	// the identifiers.
	Endpoint string
	Route    string
	Status   int
	Bytes    int64
	Duration time.Duration
	// User and Tenant as THIS gateway authenticated them - not as a header
	// claimed them. Empty for an anonymous caller.
	User   string
	Tenant string
	// Token names the credential when a machine called, beside the account.
	Token string
	// IP is the caller as the gateway resolved it, trusted proxies included.
	IP string
	// Outcome says in one word what happened, so the interesting lines are
	// greppable without knowing the status codes by heart: ok, refused,
	// failed, upstream-down.
	Outcome string
}

// Write records one crossing. It costs nothing when the log is off, which is
// the state it is in on almost every installation.
func Write(a Access) {
	lg := accessLogger.Load()
	if lg == nil {
		return
	}
	attrs := []any{
		"type", "access",
		"trace_id", a.TraceID,
		"method", a.Method,
		"path", a.Path,
		"status", a.Status,
		"ms", a.Duration.Milliseconds(),
		"outcome", a.Outcome,
	}
	// Absent fields are LEFT OUT rather than written empty: a line that says
	// `"user":""` invites somebody to read it as an anonymous caller when it
	// may mean a route that never asked.
	for _, kv := range []struct {
		k string
		v string
	}{
		{"endpoint", a.Endpoint},
		{"route", a.Route},
		{"user", a.User},
		{"tenant", a.Tenant},
		{"token", a.Token},
		{"ip", a.IP},
	} {
		if kv.v != "" {
			attrs = append(attrs, kv.k, kv.v)
		}
	}
	if a.Bytes > 0 {
		attrs = append(attrs, "bytes", a.Bytes)
	}
	lg.Info("access", attrs...)
}
