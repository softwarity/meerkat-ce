package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/tracing"
)

// Endpoint audit (AUD-04): an audited operation's calls become audit events,
// kept in the trail and sent to the collector like the rest of it (AUD-03).
//
// OUTSIDE the endpoint guard, and that is the point: auditing observes and
// never decides. Wrapped around the guard, it sees the answer the caller got -
// a refusal included, which is an event worth having - and nothing it does can
// open or close a door.
//
// Free for a call no operation audits: a path match, and the request goes on
// untouched. The body is read only for an operation that asks for it, and the
// event is handed to a writer, never written in the request.

type auditedOp struct {
	method string
	path   routing.CompiledPath
	tmpl   string
	spec   store.EndpointAudit
	mask   map[string]bool
	body   bool // reads the body: for the body itself or for a body field
}

func (rt *Router) endpointAuditor(routeName string, audits []store.EndpointAudit, filters []routing.Spec, next http.Handler) (http.Handler, error) {
	ops := make([]auditedOp, 0, len(audits))
	for _, a := range audits {
		cp, err := routing.CompilePath(a.Path)
		if err != nil {
			return nil, err
		}
		mask := a.Mask
		if len(mask) == 0 {
			mask = store.DefaultAuditMask
		}
		m := make(map[string]bool, len(mask))
		for _, k := range mask {
			m[strings.ToLower(k)] = true
		}
		needsBody := a.Body
		for _, f := range a.Fields {
			if f.From == store.AuditFromBody {
				needsBody = true
			}
		}
		ops = append(ops, auditedOp{method: strings.ToUpper(a.Method), path: cp, tmpl: a.Path, spec: a, mask: m, body: needsBody})
	}
	strip := stripPrefixCount(filters)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		specPath := routing.StripSegments(req.URL.Path, strip)
		var op *auditedOp
		for i := range ops {
			if (ops[i].method == "*" || ops[i].method == req.Method) && ops[i].path.Match(specPath) {
				op = &ops[i]
				break
			}
		}
		if op == nil {
			next.ServeHTTP(w, req)
			return
		}
		var body []byte
		bodyTooBig := false
		if op.body && isJSON(req.Header.Get("Content-Type")) && req.Body != nil {
			b, err := io.ReadAll(io.LimitReader(req.Body, store.AuditBodyMax+1))
			// What was read goes back in front of what was not: the upstream
			// gets the whole body whatever the audit kept.
			req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(b), req.Body))
			if err == nil {
				if len(b) > store.AuditBodyMax {
					bodyTooBig = true
				} else {
					body = b
				}
			}
		}
		ww, _ := record(w)
		next.ServeHTTP(ww, req)
		status := ww.status
		if status == 0 {
			status = http.StatusOK
		}
		now := time.Now()
		ev := rt.auditCallOf(req, routeName, op, specPath, status, body, bodyTooBig)
		ev.ID, ev.At = store.NewEventID(), now.Unix()
		rt.st.RecordEndpointCall(ev)
		if tracing.ShippingData() {
			tracing.ShipAudit(auditRecordOf(ev, now))
		}
	}), nil
}

func isJSON(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mt == "application/json" || strings.HasSuffix(mt, "+json"))
}

// auditCallOf builds the event: the caller as the gateway authenticated them,
// the operation, what was asked for, the answer, and what the operation said
// to take from the call.
func (rt *Router) auditCallOf(req *http.Request, routeName string, op *auditedOp, specPath string, status int, body []byte, bodyTooBig bool) store.AuditEvent {
	what := strings.TrimSpace(op.spec.Description)
	if what == "" {
		what = req.Method + " " + op.tmpl
	}
	call := &store.EndpointCall{
		Route: routeName, Method: req.Method, Operation: op.tmpl, Path: req.URL.Path,
		Status: status, TraceID: tracing.ID(req.Context()),
	}
	ev := store.AuditEvent{
		Action: "endpoint.call", Target: store.AuditTargetEndpoint,
		TargetID: routeName, TargetName: what, IP: callerIP(req), Data: call,
	}
	// The caller, always - the switch on the spans (OBS-04) is about traces;
	// an audit event without its actor is not one.
	if d, ok := rt.sessionIdentity(req); ok {
		ev.ActorID, ev.ActorName = d.UserID, d.Username
		ev.TenantID, ev.TenantName = d.TenantID, d.Tenant
		call.Group, call.Roles = d.Group, d.Roles
	}
	var parsed any
	if body != nil {
		_ = json.Unmarshal(body, &parsed)
	}
	vars := op.path.Vars(specPath)
	for _, f := range op.spec.Fields {
		var v string
		switch f.From {
		case store.AuditFromPath:
			v = vars[f.Key]
		case store.AuditFromQuery:
			v = req.URL.Query().Get(f.Key)
		case store.AuditFromHeader:
			v = req.Header.Get(f.Key)
		case store.AuditFromBody:
			v = pointer(parsed, f.Key)
		}
		if call.Fields == nil {
			call.Fields = map[string]string{}
		}
		call.Fields[f.Name] = v
	}
	if op.spec.Body {
		switch {
		case bodyTooBig:
			call.BodyOmitted = "larger than " + strconv.Itoa(store.AuditBodyMax) + " bytes"
		case parsed != nil:
			if b, err := json.Marshal(masked(parsed, op.mask)); err == nil {
				call.Body = string(b)
			}
		}
	}
	return ev
}

// auditRecordOf is the event as it leaves for the collector, under the names
// a collector already knows (OpenTelemetry's http.*, user.*, client.address).
// At the instant, not the row's second: two calls in one second are two lines
// to a collector that keeps one per timestamp.
func auditRecordOf(ev store.AuditEvent, at time.Time) tracing.AuditRecord {
	c := ev.Data
	attrs := []tracing.Attr{
		tracing.String("audit.id", ev.ID),
		tracing.String("audit.action", ev.Action),
		tracing.String("audit.target", ev.Target),
		tracing.String("audit.description", ev.TargetName),
		tracing.String("meerkat.route", c.Route),
		tracing.String("http.request.method", c.Method),
		tracing.String("http.route", c.Operation),
		tracing.String("url.path", c.Path),
		tracing.Int64("http.response.status_code", int64(c.Status)),
		tracing.String("client.address", ev.IP),
	}
	if ev.ActorID != "" {
		attrs = append(attrs,
			tracing.String("user.id", ev.ActorID), tracing.String("user.name", ev.ActorName),
			tracing.String("meerkat.tenant.id", ev.TenantID), tracing.String("meerkat.tenant.name", ev.TenantName),
			tracing.String("meerkat.group", c.Group))
		if len(c.Roles) > 0 {
			attrs = append(attrs, tracing.Strings("meerkat.roles", c.Roles))
		}
	}
	names := make([]string, 0, len(c.Fields))
	for n := range c.Fields {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		attrs = append(attrs, tracing.String("audit.field."+n, c.Fields[n]))
	}
	switch {
	case c.BodyOmitted != "":
		attrs = append(attrs, tracing.String("audit.body.omitted", c.BodyOmitted))
	case c.Body != "":
		attrs = append(attrs, tracing.String("audit.body", c.Body))
	}
	return tracing.AuditRecord{Time: at, TraceID: c.TraceID, Body: ev.TargetName, Attrs: attrs}
}

// pointer resolves a JSON pointer (RFC 6901) in v, as text.
func pointer(v any, ptr string) string {
	if v == nil || !strings.HasPrefix(ptr, "/") {
		return ""
	}
	for _, tok := range strings.Split(ptr[1:], "/") {
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		switch c := v.(type) {
		case map[string]any:
			v = c[tok]
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(c) {
				return ""
			}
			v = c[i]
		default:
			return ""
		}
	}
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// masked replaces, at any depth, the value of every key the mask names.
func masked(v any, mask map[string]bool) any {
	switch c := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(c))
		for k, x := range c {
			if mask[strings.ToLower(k)] {
				out[k] = "***"
				continue
			}
			out[k] = masked(x, mask)
		}
		return out
	case []any:
		out := make([]any, len(c))
		for i, x := range c {
			out[i] = masked(x, mask)
		}
		return out
	default:
		return v
	}
}
