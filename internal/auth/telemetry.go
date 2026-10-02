package auth

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/softwarity/meerkat/internal/tracing"
)

// The browser half of the tracing, on the DATA plane (OBS-04).
//
// Two paths, and each exists for a reason a CDN could not serve.
//
// THE SCRIPT is ours because a page served by this gateway must not go and
// fetch code somewhere else: an air-gapped installation has no internet, a
// Content-Security-Policy that allows a CDN allows everything on it, and a
// third party would see every one of your users for free. It is built into the
// Enterprise binary (`make telemetry`) and absent from the community one, so
// this path 404s there rather than being a switch that does nothing.
//
// THE RELAY exists so the collector stays PRIVATE. The alternative is an
// endpoint reachable from every user's browser, which means exposing a
// collector on the internet, with CORS, accepting unauthenticated bodies from
// anybody. Here the page posts to the origin it is already on, through the
// door that is already there, and the gateway forwards with the credential no
// page ever sees.

// telemetryBodyLimit bounds one batch from a page. A browser's exporter is
// capped at 512 spans and a span is a few hundred bytes; a megabyte is
// generous for that and small enough that a thousand tabs cannot fill memory.
const telemetryBodyLimit = 1 << 20

// telemetryCache is how long a browser may keep the script. Long, because it
// is immutable for the life of a build: a new version arrives with a new
// binary, and the page that loads it is served by that binary.
const telemetryCache = "public, max-age=3600"

func (h *Handler) registerTelemetry(mux *http.ServeMux) {
	mux.HandleFunc("GET /meerkat/telemetry.js", h.telemetryJS)
	mux.HandleFunc("POST /meerkat/telemetry", h.telemetryRelay)
}

// telemetryJS serves the bundle, or says it is not here.
func (h *Handler) telemetryJS(w http.ResponseWriter, r *http.Request) {
	js, ok := tracing.Bundle()
	if !ok {
		// Not an error page: whatever asked for this is a script tag, and the
		// honest answer to "is this here" is no.
		http.NotFound(w, r)
		return
	}
	head := w.Header()
	head.Set("Content-Type", "text/javascript; charset=utf-8")
	head.Set("Cache-Control", telemetryCache)
	head.Set("Content-Length", strconv.Itoa(len(js)))
	_, _ = w.Write(js)
}

// telemetryRelay forwards one batch of a page's spans to the collector.
//
// WHAT IT DOES NOT DO is read them. The body is OTLP the page's own exporter
// wrote, and parsing it here to write it again would be a second
// implementation of a wire format whose mistakes are silent. What the gateway
// adds is what a browser must not hold: the collector's real address and its
// credential.
//
// It answers 204 on success and 502 when the collector refused, rather than
// pretending: a page that is told "kept" when nothing was kept has no way to
// know its telemetry is going nowhere.
func (h *Handler) telemetryRelay(w http.ResponseWriter, r *http.Request) {
	if !tracing.Relaying() {
		// The export is off, or this is the community image. Either way the
		// path does not exist as far as a page is concerned.
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, telemetryBodyLimit))
	if err != nil {
		http.Error(w, "batch too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(body) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Who is reading the page, said by the gateway rather than by the page
	// (tracing/person.go). A batch the stamp cannot read is relayed as it came:
	// the person is worth adding, not worth losing the spans over.
	if attrs := tracing.PersonAttrs(h.relayPerson(r)); len(attrs) > 0 {
		if stamped, err := tracing.StampSpans(body, attrs); err == nil {
			body = stamped
		}
	}
	start := time.Now()
	if err := tracing.RelaySpans(body); err != nil {
		// Logged at DEBUG: a collector in trouble already says so once through
		// the exporter's own warning, and a line per page would be one line
		// per user per five seconds.
		slog.Debug("telemetry: a page's spans were not relayed", "err", err, "bytes", len(body))
		http.Error(w, "the collector did not take it", http.StatusBadGateway)
		return
	}
	slog.Debug("telemetry: relayed a page's spans", "bytes", len(body), "took", time.Since(start))
	w.WriteHeader(http.StatusNoContent)
}

// relayPerson is the person holding this page's session. Nobody signed in, or
// a sign-in still owing a step, is nobody.
func (h *Handler) relayPerson(r *http.Request) tracing.Who {
	if !tracing.Caller() || h.sm == nil {
		return tracing.Who{}
	}
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		return tracing.Who{}
	}
	who := tracing.Who{UserID: sess.UserID, TenantID: sess.TenantID}
	if u, err := h.st.GetUserByID(r.Context(), sess.UserID); err == nil {
		who.Username = u.Username
	}
	if sess.TenantID != "" {
		if t, err := h.st.GetTenant(r.Context(), sess.TenantID); err == nil {
			who.Tenant = t.Name
		}
	}
	if sess.GroupID != "" {
		if g, err := h.st.GetGroup(r.Context(), sess.GroupID); err == nil {
			who.Group = g.Name
		}
	}
	// The roles the access rules are judged on, as the user button reads them.
	if names, err := h.st.SessionRoleNames(r.Context(), sess.UserID, sess.TenantID, sess.GroupID); err == nil {
		who.Roles = names
	}
	return who
}
