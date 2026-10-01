// Package tracing is the request's place in a journey, and the model of what
// the gateway has to say about it (OBS-04).
//
// It sits apart from the router for the same reason internal/metrics does: the
// Enterprise exporter has to see this model, and the trunk must not have to
// see the exporter. The router produces; ee/telemetry consumes; neither knows
// the other exists.
package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// W3C Trace Context at the front door (OBS-04).
//
// WHAT THIS IS FOR, and it is not tracing. The identifier a request carries is
// what JOINS a line of ours to somebody else's: our access log says who called
// what, the service's own audit says what the call meant, and the two are one
// story because they share this string. That join has to exist on 100% of
// requests and live as long as the logs - which a trace never does, being
// sampled and kept for days.
//
// So the identifier is generated here, always, and the sampling bit is set to
// ZERO when nothing asked otherwise. A traceparent whose flags say "00" means
// "here is the name of this journey, nobody is expected to report it": it
// costs sixteen random bytes, needs no collector and no SDK, and leaves no
// dangling parent - the trap that catches anybody who invents an identifier
// and then reports nothing under it.
//
// The day the Enterprise export is switched on, the same identifiers become
// traces somebody can open, and the old log lines still point at them.
//
// WHAT COMES IN IS NOT TRUSTED BLINDLY, and only a front door can make that
// call. The last byte is a sampling decision: whoever sends "01" on every
// request makes every service downstream export, which is somebody else's
// bill. A caller can also pick a trace id and write themselves into a journey
// that is not theirs. So a malformed header is replaced rather than passed on.
// Deciding WHOSE sampling flag to believe belongs with the export that acts on
// it, and lands with the Enterprise half.

// Header is the name the whole world agreed on for trace context.
const Header = "traceparent"

// SpanContext is one request's place in a journey.
type SpanContext struct {
	// TraceID names the journey: 32 lowercase hex, stable end to end.
	TraceID string
	// SpanID names the STEP THAT CALLED US, which is our parent. Empty when
	// we opened the journey ourselves, because then there is nobody above.
	SpanID string
	// Sampled is the flag as it stands: whether anybody is expected to report
	// this journey. False is the community edition's answer to everything.
	Sampled bool
	// Inbound says the caller brought a usable context rather than us opening
	// one. It is what tells a later sampling decision whether there is already
	// a decision to respect.
	Inbound bool
}

type traceKey struct{}

// With puts a request's place in a journey in its context, which is where
// everything that has to write it down goes looking.
func With(ctx context.Context, tc SpanContext) context.Context {
	return context.WithValue(ctx, traceKey{}, tc)
}

// From reads it back.
func From(ctx context.Context) (SpanContext, bool) {
	tc, ok := ctx.Value(traceKey{}).(SpanContext)
	return tc, ok
}

// ID is what a log line, an error page or an audit row writes down. Empty
// when the request never went through the front door, which is the honest
// answer rather than a fabricated identifier that joins nothing.
func ID(ctx context.Context) string {
	tc, ok := From(ctx)
	if !ok {
		return ""
	}
	return tc.TraceID
}

// FromRequest is the one call a handler makes: read what came in, or open a
// journey. It never fails and never returns an empty identifier - a join key
// that is sometimes missing is a join key nobody can rely on.
func FromRequest(r *http.Request) SpanContext {
	if tc, ok := Parse(r.Header.Get(Header)); ok {
		return tc
	}
	return newSpanContext()
}

// Header writes the context back out in the shape the standard defines.
//
// The span id we hand to the next hop is OURS, not the one we received: that
// is what makes a tree rather than a flat list. In the community edition we
// emit no span, so there is nothing of ours to name - and rather than invent a
// parent nobody will report, we pass the journey on with the flags we have and
// let the caller's own span stand as the parent.
func (tc SpanContext) Header() string {
	parent := tc.SpanID
	if parent == "" {
		// We opened this journey, so the next hop's parent is a step of ours.
		// With the flags at 00 nothing is expected to be reported under it,
		// which is what keeps this honest.
		parent = randomHex(8)
	}
	flags := "00"
	if tc.Sampled {
		flags = "01"
	}
	return "00-" + tc.TraceID + "-" + parent + "-" + flags
}

// newSpanContext opens a journey nobody named: a fresh identifier, and NOT sampled.
func newSpanContext() SpanContext {
	return SpanContext{TraceID: randomHex(16)}
}

// Parse reads the header, strictly.
//
// Strictly, because a header that is nearly right is worse than one that is
// absent: it makes a service believe it is part of a journey that does not
// exist. The rules are the standard's - the version, the two identifiers in
// lowercase hex, neither of them all zeros - and anything else is replaced by
// a journey of our own rather than passed on.
func Parse(v string) (SpanContext, bool) {
	v = strings.TrimSpace(v)
	// version(2) - trace(32) - span(16) - flags(2), separated by three dashes.
	const want = 2 + 1 + 32 + 1 + 16 + 1 + 2
	if len(v) < want {
		return SpanContext{}, false
	}
	// A future version may carry more fields, and the standard says to read
	// the ones we know rather than refuse. Only the rest must be separated.
	if len(v) > want && v[want] != '-' {
		return SpanContext{}, false
	}
	parts := strings.SplitN(v[:want], "-", 4)
	if len(parts) != 4 {
		return SpanContext{}, false
	}
	version, traceID, spanID, flags := parts[0], parts[1], parts[2], parts[3]
	// "ff" is reserved as invalid by the standard, and an unknown version is
	// still readable in this shape - that is the point of the version field.
	if !isLowerHex(version, 2) || version == "ff" {
		return SpanContext{}, false
	}
	if !isLowerHex(traceID, 32) || isZeros(traceID) {
		return SpanContext{}, false
	}
	if !isLowerHex(spanID, 16) || isZeros(spanID) {
		return SpanContext{}, false
	}
	if !isLowerHex(flags, 2) {
		return SpanContext{}, false
	}
	// The sampling bit is the low one; the rest are reserved and ignored.
	b, err := hex.DecodeString(flags)
	if err != nil {
		return SpanContext{}, false
	}
	return SpanContext{
		TraceID: traceID,
		SpanID:  spanID,
		Sampled: b[0]&0x01 != 0,
		Inbound: true,
	}, true
}

// isLowerHex is the standard's alphabet: lowercase only, so that comparing two
// identifiers is comparing two strings.
func isLowerHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// isZeros catches the two identifiers the standard forbids: all-zero means
// "no such thing", and a service that receives one has been told nothing.
func isZeros(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

// randomHex is n bytes of randomness, written the way the standard wants them.
// crypto/rand does not fail on the platforms this runs on, and if it ever did
// the process has larger problems than a trace identifier.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HeaderOut is the name of this request, given back to whoever made it.
// Not `traceparent`: that header is a CONTEXT, and answering with one would
// invite a client to treat us as their parent. This is a plain identifier, for
// a person to read off a page and paste into a search.
const HeaderOut = "Meerkat-Trace-Id"

// Placeholder is where the identifier goes in a page built once and
// written many times.
const Placeholder = "@@meerkat-trace@@"
