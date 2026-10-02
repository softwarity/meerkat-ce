package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/tracing"
)

// Endpoint audit (AUD-04): an audited operation's calls become audit events,
// sent to the collector like the rest of the trail (AUD-03).
//
// OUTSIDE the endpoint guard, and that is the point: auditing observes and
// never decides. Wrapped around the guard, it sees the answer the caller got -
// a refusal included, which is an event worth having - and nothing it does can
// open or close a door.
//
// FREE when the audit is not sent: one atomic read, and the request goes on
// untouched. The body is read only for an operation that asks for it.

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
		if !tracing.ShippingData() {
			next.ServeHTTP(w, req)
			return
		}
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
		tracing.ShipAudit(rt.auditRecordOf(req, routeName, op, specPath, status, body, bodyTooBig))
	}), nil
}

func isJSON(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mt == "application/json" || strings.HasSuffix(mt, "+json"))
}

// auditRecordOf builds the event: the caller as the gateway authenticated
// them, the operation, what was asked for, the answer, and what the operation
// said to take from the call.
func (rt *Router) auditRecordOf(req *http.Request, routeName string, op *auditedOp, specPath string, status int, body []byte, bodyTooBig bool) tracing.AuditRecord {
	what := strings.TrimSpace(op.spec.Description)
	if what == "" {
		what = req.Method + " " + op.tmpl
	}
	attrs := []tracing.Attr{
		tracing.String("audit.action", "endpoint.call"),
		tracing.String("audit.target", "endpoint"),
		tracing.String("audit.description", what),
		tracing.String("meerkat.route", routeName),
		tracing.String("http.request.method", req.Method),
		tracing.String("http.route", op.tmpl),
		tracing.String("url.path", req.URL.Path),
		tracing.Int64("http.response.status_code", int64(status)),
		tracing.String("client.address", callerIP(req)),
	}
	// The caller, always - the switch on the spans (OBS-04) is about traces;
	// an audit event without its actor is not one.
	if d, ok := rt.sessionIdentity(req); ok {
		attrs = append(attrs,
			tracing.String("user.id", d.UserID), tracing.String("user.name", d.Username),
			tracing.String("meerkat.tenant.id", d.TenantID), tracing.String("meerkat.tenant.name", d.Tenant),
			tracing.String("meerkat.group", d.Group))
		if len(d.Roles) > 0 {
			attrs = append(attrs, tracing.Strings("meerkat.roles", d.Roles))
		}
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
		attrs = append(attrs, tracing.String("audit.field."+f.Name, v))
	}
	if op.spec.Body {
		switch {
		case bodyTooBig:
			attrs = append(attrs, tracing.String("audit.body.omitted", "larger than "+strconv.Itoa(store.AuditBodyMax)+" bytes"))
		case parsed != nil:
			if b, err := json.Marshal(masked(parsed, op.mask)); err == nil {
				attrs = append(attrs, tracing.String("audit.body", string(b)))
			}
		}
	}
	return tracing.AuditRecord{Time: time.Now(), TraceID: tracing.ID(req.Context()), Body: what, Attrs: attrs}
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
