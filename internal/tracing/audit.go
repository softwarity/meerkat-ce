package tracing

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// The audit trail, sent to the collector as OpenTelemetry LOGS (AUD-03).
//
// A copy: the trail stays in the database and on the console's screen. Every
// event the gateway records - a change made in the console, a sign-in, a
// refusal, a new factor - also leaves, as a log record whose resource says
// meerkat.stream=audit, which is what lets the Collector and the backend keep
// it apart from ordinary logs: its own query, its own retention, its own
// readers. It carries the trace id of the request that caused it, the join
// with the trace and the access log.
//
// NEVER SAMPLED, and NOT QUIETLY LOST. Recording an event never waits on the
// network: it goes into a bounded queue, a worker sends it in batches and
// retries while the collector does not answer. What a full queue has to drop
// is counted and said in the log - an audit trail with a silent hole is worse
// than one that admits it.
//
// The SENDING is Enterprise, like the rest of the export: the community binary
// registers no sender, and ShipAudit is then a no-op.

// AuditRecord is one event, as it leaves.
type AuditRecord struct {
	Time    time.Time
	TraceID string
	// Body is what happened, in one line: the action ("route.update").
	Body  string
	Attrs []Attr
	// Console is an event of the console - a change made there, a sign-in to
	// it - rather than of the data plane. The two are sent on two switches.
	Console bool
	// Severity, for a log line (logship.go); an audit event is INFO.
	SeverityText   string
	SeverityNumber int
	SpanID         string
}

const (
	auditQueueSize = 10000
	auditBatchSize = 200
	auditFlush     = 2 * time.Second
	auditRetryMax  = 30 * time.Second
)

var (
	auditStarter func(Config) (func([]byte) error, error)

	auditMu     sync.Mutex
	auditSend   func([]byte) error // nil: not shipping
	auditConfig Config
	auditQueue  = make(chan AuditRecord, auditQueueSize)
	auditOnce   sync.Once
	auditLost   atomic.Int64
	// Which audits leave: the data plane's (account sign-ins, audited
	// endpoints) and the console's, each on its own switch.
	auditData    atomic.Bool
	auditConsole atomic.Bool
	auditWarned  atomic.Int64 // unix second of the last "dropped" warning
)

// RegisterAuditStarter is called from ee/telemetry's init(): given the
// collector's address, it returns what posts a batch to /v1/logs.
func RegisterAuditStarter(f func(Config) (func([]byte) error, error)) { auditStarter = f }

// ApplyAudit points the audit at the collector, or stops it. Idempotent, like
// Apply: called at startup and whenever the setting changes.
func ApplyAudit(cfg Config, on bool) error {
	auditMu.Lock()
	defer auditMu.Unlock()
	auditSend = nil
	if !on {
		return nil
	}
	if auditStarter == nil {
		return fmt.Errorf("sending the audit to an OpenTelemetry collector is part of the Enterprise edition")
	}
	send, err := auditStarter(cfg)
	if err != nil {
		return err
	}
	auditSend, auditConfig = send, cfg
	auditOnce.Do(func() { go auditWorker() })
	return nil
}

// SetAuditScope says which audits leave: the data plane's, the console's.
func SetAuditScope(data, console bool) {
	auditData.Store(data)
	auditConsole.Store(console)
}

// ShippingData says whether the data plane's audit leaves - what an audited
// endpoint asks before it reads anything.
func ShippingData() bool { return auditData.Load() && Shipping() }

// Shipping says whether the audit leaves.
func Shipping() bool {
	auditMu.Lock()
	defer auditMu.Unlock()
	return auditSend != nil
}

// AuditLost is how many events a full queue had to drop since the start.
func AuditLost() int64 { return auditLost.Load() }

// ShipAudit queues one event. Never blocks the request that recorded it.
func ShipAudit(r AuditRecord) {
	if (r.Console && !auditConsole.Load()) || (!r.Console && !auditData.Load()) || !Shipping() {
		return
	}
	select {
	case auditQueue <- r:
	default:
		n := auditLost.Add(1)
		now := time.Now().Unix()
		if last := auditWarned.Load(); now-last >= 60 && auditWarned.CompareAndSwap(last, now) {
			slog.Warn("audit events dropped: the collector is not keeping up", "lost", n)
		}
	}
}

// auditWorker sends the queue in batches, and holds a batch - retrying with a
// growing pause - while the collector refuses it. Meanwhile the queue fills;
// once full, ShipAudit drops and counts.
func auditWorker() {
	batch := make([]AuditRecord, 0, auditBatchSize)
	tick := time.NewTicker(auditFlush)
	defer tick.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		body, err := auditPayload(batch)
		if err != nil {
			slog.Warn("audit batch not encoded", "err", err)
			batch = batch[:0]
			return
		}
		pause := time.Second
		for {
			auditMu.Lock()
			send := auditSend
			auditMu.Unlock()
			if send == nil {
				// Switched off while waiting: what is held stays in the
				// database, which is the trail; only the copy is abandoned.
				batch = batch[:0]
				return
			}
			err := send(body)
			if err == nil {
				batch = batch[:0]
				return
			}
			slog.Warn("audit batch not taken by the collector, retrying", "events", len(batch), "in", pause, "err", err)
			time.Sleep(pause)
			if pause *= 2; pause > auditRetryMax {
				pause = auditRetryMax
			}
		}
	}
	for {
		select {
		case r := <-auditQueue:
			batch = append(batch, r)
			if len(batch) >= auditBatchSize {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}

// auditPayload is an OTLP/JSON logs export request, marked audit.
func auditPayload(batch []AuditRecord) ([]byte, error) {
	auditMu.Lock()
	cfg := auditConfig
	auditMu.Unlock()
	return logsPayload(cfg, "audit", "meerkat.audit", batch)
}

// logsPayload is an OTLP/JSON logs export request: the records of one batch,
// under a resource that names the writer and the stream - "audit" or "logs" -
// which is what the Collector and the backend keep apart.
func logsPayload(cfg Config, stream, scope string, batch []AuditRecord) ([]byte, error) {
	records := make([]map[string]any, len(batch))
	for i, r := range batch {
		text, num := r.SeverityText, r.SeverityNumber
		if text == "" {
			text, num = "INFO", 9
		}
		rec := map[string]any{
			"timeUnixNano":   strconv.FormatInt(r.Time.UnixNano(), 10),
			"severityNumber": num,
			"severityText":   text,
			"body":           map[string]any{"stringValue": r.Body},
			"attributes":     otlpAttrs(r.Attrs),
		}
		if r.TraceID != "" {
			rec["traceId"] = r.TraceID
		}
		if r.SpanID != "" {
			rec["spanId"] = r.SpanID
		}
		records[i] = rec
	}
	service := cfg.Service
	if service == "" {
		service = "meerkat"
	}
	resource := []Attr{String("service.name", service), String("meerkat.stream", stream)}
	if cfg.Version != "" {
		resource = append(resource, String("service.version", cfg.Version))
	}
	return json.Marshal(map[string]any{
		"resourceLogs": []any{map[string]any{
			"resource": map[string]any{"attributes": otlpAttrs(resource)},
			"scopeLogs": []any{map[string]any{
				"scope":      map[string]any{"name": scope},
				"logRecords": records,
			}},
		}},
	})
}

func otlpAttrs(attrs []Attr) []any {
	out := make([]any, 0, len(attrs))
	for _, a := range attrs {
		var v map[string]any
		switch {
		case a.IsInt:
			v = map[string]any{"intValue": strconv.FormatInt(a.Int, 10)}
		case a.IsList:
			vals := make([]any, len(a.Strs))
			for i, s := range a.Strs {
				vals[i] = map[string]any{"stringValue": s}
			}
			v = map[string]any{"arrayValue": map[string]any{"values": vals}}
		default:
			if a.Str == "" {
				continue
			}
			v = map[string]any{"stringValue": a.Str}
		}
		out = append(out, map[string]any{"key": a.Key, "value": v})
	}
	return out
}
