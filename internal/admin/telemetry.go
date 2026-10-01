package admin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/tracing"
	"github.com/softwarity/meerkat/internal/vault"
	"github.com/softwarity/meerkat/internal/version"
)

// The tracing setting (OBS-04), on the same shape as the Prometheus switch.
//
// Infra's, and Enterprise to turn on: exporting is what is sold, the trace
// context travelling is not. The community image answers WHY rather than
// pretending the switch does nothing - a screen whose toggle does not stick is
// worse than one that says it cannot.
//
// WHAT COMES BACK IS RAW: `$otlp-token` as written, never the value behind it.
// The exporter resolves at the moment of the call, so the credential is not in
// this response, not in the audit trail and not on the screen (VAULT-05).

type telemetrySetting struct {
	store.TelemetryConfig
	// Enterprise says whether this build can export at all, so the console
	// draws the switch as locked rather than as off. Read-only: an edition is
	// decided by which image runs, never by a request.
	Enterprise bool `json:"enterprise"`
	// Exporting says whether spans are actually leaving right now, which is
	// not the same question as whether somebody ticked the box: a collector
	// address that was refused leaves the setting on and the pipe shut.
	Exporting bool `json:"exporting"`
	// PushingMetrics is the same question for the counters.
	PushingMetrics bool `json:"pushingMetrics"`
	// HeaderSet says a LITERAL header value is stored: something authenticates
	// to the collector that this payload does not carry. A reference travels,
	// so it does not raise this - the console tells the two states apart by
	// it, exactly as the mail relay's password does (VAULT-05).
	HeaderSet bool `json:"headerSet"`
}

// hideLiterals is the rule this screen had written in a tooltip and nowhere
// else: a reference is public and travels, a literal never leaves the gateway.
// It comes back blank, with HeaderSet raised, and the console then offers to
// move it into the vault - the same offer every other secret field makes.
func hideLiterals(cfg store.TelemetryConfig) (store.TelemetryConfig, bool) {
	held := false
	if len(cfg.Headers) == 0 {
		return cfg, false
	}
	safe := make(map[string]string, len(cfg.Headers))
	for name, value := range cfg.Headers {
		if value != "" && !vault.IsRef(value) {
			held = true
			safe[name] = ""
			continue
		}
		safe[name] = value
	}
	cfg.Headers = safe
	return cfg, held
}

// WHOSE SETTING THIS IS, and it was wrong: root's, for no reason that survives
// being asked. Where a gateway sends its own spans is infrastructure - the same
// domain as TLS, the mail relay, the proxy ceilings and the maintenance switch,
// all of which an infra admin already holds. It crosses no partition: it names
// no account, carries no identity, and its credential is a vault reference.
//
// What stays root's is what CROSSES a partition or creates one - a
// control-plane token, an agent, an export of the whole installation - not an
// address a routing plane sends its own timings to. Reading it is gateway
// scope, writing it infra, exactly like a route.
func (a *API) registerTelemetry(mux Mux) {
	mux.Handle("GET /api/settings/telemetry", a.gw(a.getTelemetry))
	mux.Handle("PUT /api/settings/telemetry", a.infraAdmin(a.putTelemetry))
	mux.Handle("POST /api/settings/telemetry/test", a.infraAdmin(a.testTelemetry))
}

// telemetryProbe is the address and the auth header AS THEY STAND ON SCREEN:
// somebody types an address and asks whether anything answers, before saving
// it. Same gesture as the mail relay's test, for the same reason - a setting
// that only reports its mistake three days later, in an empty Jaeger, is a
// setting nobody trusts.
type telemetryProbe struct {
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers,omitempty"`
	// Metrics asks the collector's metrics path too. Traces are always
	// probed: the address is judged on them first.
	Metrics bool `json:"metrics,omitempty"`
}

// telemetryProbeResult is deliberately thin: the status and how long it took.
//
// NOT THE BODY, and that is the whole care this endpoint needs. It makes this
// gateway fetch an address somebody hands it, which is infra's and no more
// than configuring the export already allows - but returning what came back
// would turn it into a reader of whatever the gateway can reach. The status
// answers the question ("does the collector answer?"); anything more answers
// somebody else's.
type telemetryProbeResult struct {
	Status int    `json:"status"`
	Ms     int64  `json:"ms"`
	Note   string `json:"note,omitempty"`
}

// probeTimeout: a collector that has not answered in five seconds has not
// answered. Short on purpose - this runs while somebody waits on a screen.
const probeTimeout = 5 * time.Second

func (a *API) testTelemetry(w http.ResponseWriter, r *http.Request, _ store.User) {
	var body telemetryProbe
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	// The same cleaning the setting gets, so a test and a save judge the same
	// address: the scheme is checked, a pasted /v1/traces is taken off, and
	// the path this exporter writes is added below.
	cfg := store.TelemetryConfig{Enabled: true, Traces: true, Endpoint: body.Endpoint, Headers: body.Headers}
	if err := store.SanitizeTelemetry(&cfg); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	stored := a.st.RawTelemetry(r.Context())
	headers := make(map[string]string, len(cfg.Headers))
	for name, value := range cfg.Headers {
		// Blank where a literal is stored: the console never received it, so
		// the probe uses what the exporter would use. Otherwise the test would
		// answer 401 on a collector the export reaches perfectly well.
		if value == "" {
			value = stored.Headers[name]
		}
		// The header may be a vault reference, as the rule says it should be:
		// resolved here, at the moment of the call, exactly as the exporter
		// does it.
		headers[name] = a.st.ExpandInfra(r.Context(), value)
	}
	// EMPTY batches: they exercise the whole path - name resolution, the
	// network, TLS, the credential, the collector's own answer - and record
	// nothing anywhere. A probe that wrote a fake trace into somebody's
	// backend would be a probe people stop running.
	status, took, code, refusal := probeCollector(r.Context(), cfg.Endpoint+"/v1/traces", `{"resourceSpans":[]}`, headers)
	if refusal != "" {
		writeErr(w, code, refusal)
		return
	}
	if body.Metrics {
		// The address takes traces; whether it takes METRICS is its own
		// question, and the mistake it catches is a real one: Jaeger receives
		// OTLP traces and nothing else, so the counters would be refused at
		// every push while the traces arrive and everything looks fine.
		_, _, code, refusal := probeCollector(r.Context(), cfg.Endpoint+"/v1/metrics", `{"resourceMetrics":[]}`, headers)
		if refusal != "" {
			writeErr(w, code, "traces are accepted, but not metrics - "+refusal+
				". A backend that receives traces only (Jaeger does) needs an OpenTelemetry Collector in front of it for the metrics, or leave them to the metrics endpoint")
			return
		}
	}
	writeJSON(w, http.StatusOK, telemetryProbeResult{Status: status, Ms: took})
}

// probeCollector posts one empty batch to path and says what answered: the
// status and the time, or the status to answer with and a sentence naming the
// likely mistake.
func probeCollector(ctx context.Context, path, batch string, headers map[string]string) (int, int64, int, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(batch))
	if err != nil {
		return 0, 0, http.StatusUnprocessableEntity, "that address cannot be called: " + err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	client := &http.Client{Timeout: probeTimeout}
	start := time.Now()
	res, err := client.Do(req)
	took := time.Since(start).Milliseconds()
	if err != nil {
		return 0, 0, http.StatusBadGateway, "nothing answered: " + err.Error()
	}
	defer func() { _ = res.Body.Close() }()
	// Read a little of it, and ONLY to tell what kind of thing answered. None
	// of it is handed back.
	peek, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	if res.StatusCode >= 400 {
		return 0, 0, http.StatusBadGateway, probeRefusal(res.StatusCode, path)
	}
	// A 200 IS NOT AN ANSWER BY ITSELF, and this is the mistake the probe
	// exists to catch. Pointed at a collector's UI - Jaeger's 16686 rather
	// than its 4318 - the request lands on a single-page application, which
	// serves its index for any path and answers 200. Without this, the probe
	// blesses exactly the address that will export nothing.
	//
	// An OTLP endpoint answers what it was sent, so JSON; some answer 200 with
	// nothing at all, which is accepted too. Anything that looks like a page
	// is not a collector.
	if ct := res.Header.Get("Content-Type"); !otlpAnswer(ct, peek) {
		return 0, 0, http.StatusBadGateway, fmt.Sprintf(
			"something answered %d, but not as a collector does (%s): this is likely a web interface rather than an OTLP endpoint - Jaeger listens for traces on 4318 and serves its UI on 16686",
			res.StatusCode, contentKind(ct))
	}
	return res.StatusCode, took, 0, ""
}

// otlpAnswer reports whether what came back is what a collector sends: JSON, or
// nothing. Judged on the content type first, and on the body when the type is
// missing - some servers send none, and a page still starts with a tag.
func otlpAnswer(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	if contentType == "" && len(trimmed) == 0 {
		return true
	}
	return false
}

// contentKind names what answered, in one word, for the refusal above. The
// content type as sent would do, but it is a header a caller controls: a word
// this code chose cannot carry anything back from the address probed.
func contentKind(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "html"):
		return "it answered with a web page"
	case ct == "":
		return "it answered with a body and no content type"
	default:
		return "it did not answer with JSON"
	}
}

// probeRefusal turns a status into the sentence that names the likely cause.
// The two that actually happen are worth naming: the OTLP port is not the one
// a collector's UI listens on, and a collector behind an authenticating proxy
// refuses before it ever reads a batch.
func probeRefusal(status int, path string) string {
	msg := fmt.Sprintf("the collector answered %d", status)
	switch status {
	case http.StatusNotFound:
		if strings.HasSuffix(path, "/v1/metrics") {
			return msg + " on /v1/metrics: it has no metrics receiver"
		}
		return msg + " on /v1/traces: this is likely the address of its UI rather than its OTLP port (Jaeger listens on 4318, its UI on 16686)"
	case http.StatusUnauthorized, http.StatusForbidden:
		return msg + ": it wants a credential, or the one in the auth header was refused"
	case http.StatusUnsupportedMediaType:
		return msg + ": it does not accept OTLP over HTTP with a JSON body, which is what this gateway speaks"
	}
	return msg
}

func (a *API) getTelemetry(w http.ResponseWriter, r *http.Request) {
	cfg, held := hideLiterals(a.st.RawTelemetry(r.Context()))
	writeJSON(w, http.StatusOK, telemetrySetting{
		TelemetryConfig: cfg,
		Enterprise:      edition.Enterprise,
		Exporting:       tracing.Exporting(),
		PushingMetrics:  metrics.Pushing(),
		HeaderSet:       held,
	})
}

func (a *API) putTelemetry(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body telemetrySetting
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	cfg := body.TelemetryConfig
	if err := store.SanitizeTelemetry(&cfg); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if cfg.Enabled {
		if err := edition.Require("exporting to an OpenTelemetry collector"); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
	}
	before := a.st.RawTelemetry(r.Context())
	// A blank value where a LITERAL is stored means "leave it alone": the
	// console never received that literal, so it cannot send it back, and
	// taking the blank at face value would silently unauthenticate the export
	// the first time somebody edited the rate. The same rule the mail relay's
	// password follows.
	for name, value := range cfg.Headers {
		if value != "" {
			continue
		}
		if kept, ok := before.Headers[name]; ok && kept != "" {
			cfg.Headers[name] = kept
		}
	}
	if err := a.st.SetSetting(r.Context(), store.SettingTelemetry, cfg); err != nil {
		a.internal(w, err)
		return
	}
	// Applied NOW, not at the next restart. An operator who turns tracing on
	// to look at something that is happening cannot wait for a deployment.
	a.ApplyTelemetry(r.Context())
	// And on every other node: each runs its own exporters, and a setting
	// applied on the node that saved it alone would leave the rest exporting
	// where it used to until their next restart.
	a.announce(r.Context(), store.TopicTelemetry)
	// The injected fragment is BAKED INTO the routes at compile time, like the
	// portal bar and the branding, so turning the browser half on has to
	// recompile them. Without this the switch sticks in the database and
	// nothing appears in a page until the next restart.
	if err := a.reloadRouting(r.Context()); err != nil {
		slog.Warn("routes not reloaded after the tracing setting changed", "err", err)
	}
	a.auditUpdate(r.Context(), actor, "telemetry.configure", "settings", "", "", "", before, cfg)
	safe, held := hideLiterals(cfg)
	writeJSON(w, http.StatusOK, telemetrySetting{
		TelemetryConfig: safe, Enterprise: edition.Enterprise, Exporting: tracing.Exporting(),
		PushingMetrics: metrics.Pushing(), HeaderSet: held,
	})
}

// ApplyTelemetry points the exporters wherever the setting now says - the
// spans and the pushed counters, the two signals of one collector - resolving
// the vault references on the way, which is the only place they are resolved
// and the moment they are used. Main calls it at startup and hangs it on the
// change bus; saving the setting calls it here.
func (a *API) ApplyTelemetry(ctx context.Context) {
	cfg := a.st.ResolvedTelemetry(ctx)
	err := tracing.Apply(tracing.Config{
		Endpoint:     cfg.Endpoint,
		Headers:      cfg.Headers,
		Service:      "meerkat",
		Version:      version.Version,
		Sample:       cfg.Sample,
		MaxPerSecond: cfg.MaxPerSecond,
		Detail:       cfg.GatewayDetail,
	}, cfg.ExportsTraces())
	if err != nil {
		// Said out loud rather than swallowed: the setting stuck, the pipe did
		// not, and `exporting` in the answer is what tells the screen so.
		slog.Warn("traces are not being exported", "endpoint", cfg.Endpoint, "why", err)
	}
	host, _ := os.Hostname()
	err = metrics.ApplyPush(metrics.PushConfig{
		Endpoint: cfg.Endpoint,
		Headers:  cfg.Headers,
		Service:  "meerkat",
		Version:  version.Version,
		Instance: host,
	}, a.registry(), cfg.ExportsMetrics())
	if err != nil {
		slog.Warn("metrics are not being pushed", "endpoint", cfg.Endpoint, "why", err)
	}
}
