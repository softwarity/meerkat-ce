package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/softwarity/meerkat/internal/tracing"
)

// OpenTelemetry's JSON log format, on the same outputs (OBS-03).
//
// One object per line, with the field names of OpenTelemetry's log data model:
//
//	{"timestamp": "...", "severity_text": "INFO", "severity_number": 9,
//	 "body": "...", "trace_id": "...", "span_id": "...",
//	 "attributes": {...}, "resource": {"service.name": "meerkat", ...}}
//
// The gateway does not PUSH its logs: a Collector running as a DaemonSet reads
// what every container writes and decodes these fields one by one, so a log
// line lands in the backend with its severity, its trace and its attributes as
// such - and the trace id is the join with the trace and with the audit. A
// switch on the telemetry screen turns it on, live, on every node.

// FormatOTel is that format.
const FormatOTel Format = "otel"

var (
	// base is the format chosen at startup (json or text): what the switch
	// returns to when it is turned off.
	base atomic.Value // Format
	// otelOn is the switch.
	otelOn atomic.Bool
	// pushOn sends every line to the collector too.
	pushOn atomic.Bool
	// resource is what every OTel line says about who wrote it.
	resource atomic.Value // map[string]string
)

// SetResource names the writer on every OTel line: the service and its
// version, at least.
func SetResource(attrs map[string]string) { resource.Store(attrs) }

// SetOTel turns OpenTelemetry's JSON format on or off, for the operational
// log and for the access log. Off, both return to the format chosen at
// startup.
func SetOTel(on bool) {
	if otelOn.Swap(on) == on {
		return
	}
	install()
}

// SetPush pushes every line to the collector as well, over OTLP
// (tracing/logship.go), while the outputs keep their format. The other way
// out, for a node with no agent to read them.
func SetPush(on bool) {
	if pushOn.Swap(on) == on {
		return
	}
	install()
}

// Pushing says whether the lines are pushed.
func Pushing() bool { return pushOn.Load() }

// install puts the handlers the switches ask for in place, for the
// operational log and for the access log.
func install() {
	slog.SetDefault(slog.New(withPush(handlerFor(os.Stderr, current(), &slog.HandlerOptions{Level: level}), level)))
	if AccessLogEnabled() {
		accessLogger.Store(newAccessLogger(os.Stdout, current()))
	}
}

// withPush tees h to the collector when the push is on.
func withPush(h slog.Handler, lv slog.Leveler) slog.Handler {
	if !pushOn.Load() {
		return h
	}
	return teeHandler{h, &pushHandler{newOTelHandler(io.Discard, &slog.HandlerOptions{Level: lv})}}
}

// OTel says whether the switch is on.
func OTel() bool { return otelOn.Load() }

// current is the format in force: OTel when switched on, the startup one
// otherwise.
func current() Format {
	if otelOn.Load() {
		return FormatOTel
	}
	if f, ok := base.Load().(Format); ok {
		return f
	}
	return FormatText
}

func handlerFor(w io.Writer, f Format, opts *slog.HandlerOptions) slog.Handler {
	switch f {
	case FormatOTel:
		return newOTelHandler(w, opts)
	case FormatJSON:
		return slog.NewJSONHandler(w, opts)
	default:
		return slog.NewTextHandler(w, opts)
	}
}

// otelHandler writes one OTel JSON object per record.
type otelHandler struct {
	w      io.Writer
	mu     *sync.Mutex
	level  slog.Leveler
	attrs  []slog.Attr // from WithAttrs, already qualified by their groups
	groups []string
}

func newOTelHandler(w io.Writer, opts *slog.HandlerOptions) *otelHandler {
	var lv slog.Leveler = slog.LevelInfo
	if opts != nil && opts.Level != nil {
		lv = opts.Level
	}
	return &otelHandler{w: w, mu: &sync.Mutex{}, level: lv}
}

func (h *otelHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.attrs = append(append([]slog.Attr{}, h.attrs...), qualify(h.groups, attrs)...)
	return &c
}

func (h *otelHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	c.groups = append(append([]string{}, h.groups...), name)
	return &c
}

func (h *otelHandler) Handle(ctx context.Context, r slog.Record) error {
	b, err := json.Marshal(h.build(ctx, r))
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err = h.w.Write(append(b, '\n'))
	return err
}

// build is one record as OpenTelemetry's log data model names it.
func (h *otelHandler) build(ctx context.Context, r slog.Record) map[string]any {
	line := map[string]any{
		"timestamp":       r.Time.UTC().Format(time.RFC3339Nano),
		"severity_text":   severityText(r.Level),
		"severity_number": severityNumber(r.Level),
		"body":            r.Message,
	}
	attrs := map[string]any{}
	for _, a := range h.attrs {
		flatten(attrs, "", a)
	}
	var own []slog.Attr
	r.Attrs(func(a slog.Attr) bool { own = append(own, a); return true })
	for _, a := range qualify(h.groups, own) {
		flatten(attrs, "", a)
	}
	// The trace this line belongs to: named by the record (the access log
	// carries it), or by the context it was written in.
	if id, ok := attrs["trace_id"].(string); ok && id != "" {
		line["trace_id"] = id
		delete(attrs, "trace_id")
	} else if id := tracing.ID(ctx); id != "" {
		line["trace_id"] = id
	}
	if id, ok := attrs["span_id"].(string); ok && id != "" {
		line["span_id"] = id
		delete(attrs, "span_id")
	}
	if len(attrs) > 0 {
		line["attributes"] = attrs
	}
	if res, ok := resource.Load().(map[string]string); ok && len(res) > 0 {
		line["resource"] = res
	}
	return line
}

// pushHandler sends each record to the collector's queue, with the same
// fields the OTel format writes.
type pushHandler struct{ *otelHandler }

func (h *pushHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return &pushHandler{h.otelHandler.WithAttrs(a).(*otelHandler)}
}

func (h *pushHandler) WithGroup(n string) slog.Handler {
	return &pushHandler{h.otelHandler.WithGroup(n).(*otelHandler)}
}

func (h *pushHandler) Handle(ctx context.Context, r slog.Record) error {
	line := h.build(ctx, r)
	rec := tracing.AuditRecord{
		Time:           r.Time,
		Body:           r.Message,
		SeverityText:   severityText(r.Level),
		SeverityNumber: severityNumber(r.Level),
	}
	rec.TraceID, _ = line["trace_id"].(string)
	rec.SpanID, _ = line["span_id"].(string)
	if attrs, ok := line["attributes"].(map[string]any); ok {
		for k, v := range attrs {
			switch t := v.(type) {
			case string:
				rec.Attrs = append(rec.Attrs, tracing.String(k, t))
			case int64:
				rec.Attrs = append(rec.Attrs, tracing.Int64(k, t))
			case bool:
				rec.Attrs = append(rec.Attrs, tracing.String(k, strconv.FormatBool(t)))
			default:
				rec.Attrs = append(rec.Attrs, tracing.String(k, fmt.Sprint(t)))
			}
		}
	}
	tracing.ShipLog(rec)
	return nil
}

// teeHandler writes a record to two handlers: the output, and the push.
type teeHandler struct{ a, b slog.Handler }

func (t teeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return t.a.Enabled(ctx, l) || t.b.Enabled(ctx, l)
}

func (t teeHandler) Handle(ctx context.Context, r slog.Record) error {
	var err error
	if t.a.Enabled(ctx, r.Level) {
		err = t.a.Handle(ctx, r.Clone())
	}
	if t.b.Enabled(ctx, r.Level) {
		_ = t.b.Handle(ctx, r)
	}
	return err
}

func (t teeHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return teeHandler{t.a.WithAttrs(a), t.b.WithAttrs(a)}
}

func (t teeHandler) WithGroup(n string) slog.Handler {
	return teeHandler{t.a.WithGroup(n), t.b.WithGroup(n)}
}

// qualify prefixes attributes with the open groups, as slog would nest them.
func qualify(groups []string, attrs []slog.Attr) []slog.Attr {
	if len(groups) == 0 {
		return attrs
	}
	return []slog.Attr{{Key: strings.Join(groups, "."), Value: slog.GroupValue(attrs...)}}
}

// flatten writes a into m with dotted keys: OTel attributes are flat, and a
// backend indexes "upstream.host" better than a nested object.
func flatten(m map[string]any, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, g := range a.Value.Group() {
			flatten(m, key, g)
		}
		return
	}
	if key == "" {
		return
	}
	switch a.Value.Kind() {
	case slog.KindString:
		m[key] = a.Value.String()
	case slog.KindInt64:
		m[key] = a.Value.Int64()
	case slog.KindUint64:
		m[key] = a.Value.Uint64()
	case slog.KindFloat64:
		m[key] = a.Value.Float64()
	case slog.KindBool:
		m[key] = a.Value.Bool()
	case slog.KindDuration:
		m[key] = a.Value.Duration().Milliseconds()
	case slog.KindTime:
		m[key] = a.Value.Time().UTC().Format(time.RFC3339Nano)
	default:
		if err, ok := a.Value.Any().(error); ok {
			m[key] = err.Error()
			return
		}
		m[key] = a.Value.String()
	}
}

// The severity, in OpenTelemetry's words and numbers.
func severityText(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

func severityNumber(l slog.Level) int {
	switch {
	case l >= slog.LevelError:
		return 17
	case l >= slog.LevelWarn:
		return 13
	case l >= slog.LevelInfo:
		return 9
	default:
		return 5
	}
}
