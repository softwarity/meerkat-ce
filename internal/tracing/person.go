package tracing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
)

// Who made the call, on the spans (OBS-04).
//
// A trace without the person answers "what was slow" and never "for whom",
// which is the first question a support ticket asks. Switched on (the
// telemetry screen), every span the gateway is responsible for names the
// signed-in caller: the account and the organisation, by id and by name. OFF
// by default, because a person in a trace is personal data leaving for a
// collector somebody else may run, and that is a decision to take, not to
// inherit. The attribute names are OpenTelemetry's own
// (user.id, user.name), so a backend that knows them shows them where it shows
// a user; the organisation has no standard name and carries ours.
//
// THE BROWSER'S SPANS ARE STAMPED BY THE GATEWAY, at the relay, rather than by
// the page: the page never has to be told who is reading it, and its spans
// say the same thing the gateway's do.

var caller atomic.Bool

// SetCaller turns the caller on the spans on or off, live.
func SetCaller(on bool) { caller.Store(on) }

// Caller is what is set right now.
func Caller() bool { return caller.Load() }

// ONE LAYER CARRIES THE PERSON, never two. A journey the browser bundle
// opened says so in tracestate (meerkat=b, written by its sampler on the root
// span): its spans are stamped at the relay, so the gateway's own span on that
// journey stays silent. Every other journey - a backend, a script, a page
// without the bundle - gets the person on the gateway's span.

// FromPage says whether a tracestate header names a journey our bundle opened.
func FromPage(tracestate string) bool {
	for _, entry := range strings.Split(tracestate, ",") {
		if strings.TrimSpace(entry) == "meerkat=b" {
			return true
		}
	}
	return false
}

// Who is the person behind a crossing, as the gateway knows them.
type Who struct {
	UserID   string
	Username string
	TenantID string
	Tenant   string
	// Group is the session's active group, when the organisation works in
	// exclusive mode and one is chosen; Roles are the roles the access rules
	// were judged on - what a refusal is explained by.
	Group string
	Roles []string
}

// PersonAttrs is what the spans say about w: nothing while the switch is off,
// nothing for an anonymous call.
func PersonAttrs(w Who) []Attr {
	if !Caller() || w.UserID == "" {
		return nil
	}
	attrs := []Attr{String("user.id", w.UserID)}
	if w.Username != "" {
		attrs = append(attrs, String("user.name", w.Username))
	}
	if w.TenantID != "" {
		attrs = append(attrs, String("meerkat.tenant.id", w.TenantID))
	}
	if w.Tenant != "" {
		attrs = append(attrs, String("meerkat.tenant.name", w.Tenant))
	}
	if w.Group != "" {
		attrs = append(attrs, String("meerkat.group", w.Group))
	}
	if len(w.Roles) > 0 {
		attrs = append(attrs, Strings("meerkat.roles", w.Roles))
	}
	return attrs
}

// StampSpans adds attrs to every span of an OTLP/JSON batch - a page's, on its
// way through the relay. Every other byte of the batch travels as it came:
// numbers are kept as written (a timestamp in nanoseconds does not survive a
// float), and fields this code does not know are carried untouched.
//
// An attribute the page already set under the same key is REPLACED: the
// gateway is the one that knows who holds the session, and a page claiming to
// be somebody else is precisely what this must not believe.
func StampSpans(body []byte, attrs []Attr) ([]byte, error) {
	if len(attrs) == 0 {
		return body, nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var batch map[string]any
	if err := dec.Decode(&batch); err != nil {
		return nil, fmt.Errorf("the batch is not OTLP/JSON: %w", err)
	}
	stamp := make([]any, len(attrs))
	keys := make(map[string]bool, len(attrs))
	for i, a := range attrs {
		keys[a.Key] = true
		value := map[string]any{"stringValue": a.Str}
		switch {
		case a.IsInt:
			value = map[string]any{"intValue": json.Number(fmt.Sprint(a.Int))}
		case a.IsList:
			values := make([]any, len(a.Strs))
			for i, v := range a.Strs {
				values[i] = map[string]any{"stringValue": v}
			}
			value = map[string]any{"arrayValue": map[string]any{"values": values}}
		}
		stamp[i] = map[string]any{"key": a.Key, "value": value}
	}
	for _, rs := range list(batch["resourceSpans"]) {
		for _, ss := range list(field(rs, "scopeSpans")) {
			for _, sp := range list(field(ss, "spans")) {
				span, ok := sp.(map[string]any)
				if !ok {
					continue
				}
				kept := make([]any, 0, len(list(span["attributes"]))+len(stamp))
				for _, a := range list(span["attributes"]) {
					if k, _ := field(a, "key").(string); keys[k] {
						continue
					}
					kept = append(kept, a)
				}
				span["attributes"] = append(kept, stamp...)
			}
		}
	}
	return json.Marshal(batch)
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func field(v any, key string) any {
	m, _ := v.(map[string]any)
	return m[key]
}
