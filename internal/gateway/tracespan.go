package gateway

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/softwarity/meerkat/internal/tracing"
)

// The gateway's own two spans (OBS-04).
//
// WHAT THEY BUY, and it is the one number nobody else in the cluster can
// produce: the SERVER span covers the whole crossing, the CLIENT span covers
// the call out, and the gap between them is the gateway's own work - choosing
// a route, checking access, running the filters, rewriting the answer.
// Without them that time is a hole in the trace between the caller's span and
// the service's, and a hole is where an argument between two teams starts.
//
// THE SERVER SPAN ENDS WHEN THE ANSWER IS WRITTEN, not when the upstream
// replies: a gateway keeps working after that. Which is exactly why the CLIENT
// span nests inside it rather than sitting beside it.
//
// NOTHING IS BUILT WHEN NOBODY IS EXPORTING. The community binary links no
// exporter, so Exporting() is false, no note is allocated, no clock is read.

// spanNote is the server span in flight. A pointer in the context because the
// two ends of it are a long way apart: the request arriving, and whichever of
// half a dozen exits answers it.
type spanNote struct {
	ctx   tracing.SpanContext
	id    string
	start time.Time
	// dropped is a route saying it is not traced. The note is already in the
	// context by then - the journey is named before anything can refuse it -
	// so this is what makes the two spans never leave, rather than a note that
	// was never opened.
	dropped bool
	// events are the steps worth naming inside the span. Cheap - a name and an
	// instant - which is the whole reason they are not spans of their own.
	events []tracing.Event
}

type spanKey struct{}

// withSpan starts the server span when somebody is listening, and hands back
// the request untouched when nobody is.
func withSpan(req *http.Request, sc tracing.SpanContext) *http.Request {
	if !tracing.Exporting() || !tracing.ShouldRecord(sc) {
		return req
	}
	n := &spanNote{ctx: sc, id: tracing.NewSpanID(), start: time.Now()}
	ctx := context.WithValue(req.Context(), spanKey{}, n)
	// The steps of the gateway's own work, when the installation asked for
	// them, hang under this crossing - the store's included, which knows
	// nothing of the router and finds its parent here.
	ctx = tracing.WithCurrent(ctx, sc.TraceID, n.id)
	return req.WithContext(ctx)
}

// spanOf reads the note back, nil when this request is not being recorded -
// which is the answer for almost all of them, at almost every sample rate.
func spanOf(ctx context.Context) *spanNote {
	n, _ := ctx.Value(spanKey{}).(*spanNote)
	return n
}

// dropSpan is a route that is not traced, said once the router has chosen it.
//
// WHY HERE AND NOT AT THE DOOR. Which route answers is not known when the
// request arrives - that is the router's whole job - so the span is opened for
// everything and abandoned for the routes an operator left out. Nothing is
// emitted, and the outgoing trace context is not rewritten either: what the
// caller sent travels on untouched, which is the difference between "this
// gateway does not report on this route" and "this journey does not exist".
func dropSpan(ctx context.Context) {
	if n := spanOf(ctx); n != nil {
		n.dropped = true
	}
	// And every step under it, opened or not yet.
	tracing.DropCurrent(ctx)
}

// dropTracing is that decision posed on the way IN, as the outermost wrapper of
// a route's handler: the client span is emitted from inside the upstream call,
// so saying it on the way out would come too late.
func dropTracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dropSpan(r.Context())
		next.ServeHTTP(w, r)
	})
}

// spanEvent names an instant inside the span: the route was chosen, access was
// granted. Costs a name and a timestamp, against a whole object for a span.
func spanEvent(ctx context.Context, name string, attrs ...tracing.Attr) {
	if n := spanOf(ctx); n != nil {
		n.events = append(n.events, tracing.Event{Name: name, At: time.Now(), Attr: attrs})
	}
}

// finishSpan closes the server span and hands it over. Called from the same
// exits that count the request, refusals included: a 403 is a span worth
// having, and it is one the service behind never sees.
func finishSpan(req *http.Request, routeName, endpoint string, status int) {
	n := spanOf(req.Context())
	if n == nil || n.dropped {
		return
	}
	name := req.Method
	if endpoint != "" {
		name += " " + endpoint
	} else if routeName != "" {
		name += " " + routeName
	}
	attrs := []tracing.Attr{
		tracing.String("http.request.method", req.Method),
		tracing.Int64("http.response.status_code", int64(status)),
		// The TEMPLATE, not the path: an attribute carrying identifiers is a
		// cardinality bomb in every backend that indexes attributes. The path
		// itself belongs in the access log, which is where an audit reads it.
		tracing.String("url.path", endpoint),
	}
	if routeName != "" {
		attrs = append(attrs, tracing.String("meerkat.route", routeName))
	}
	tracing.Emit(tracing.Span{
		TraceID:      n.ctx.TraceID,
		SpanID:       n.id,
		ParentSpanID: n.ctx.SpanID,
		Name:         name,
		Kind:         tracing.KindServer,
		Start:        n.start,
		End:          time.Now(),
		Attrs:        attrs,
		Events:       n.events,
		Status:       spanStatus(status),
		// The message only when there IS one: a status object on a 404 invites
		// a backend to paint an answer red.
		StatusMessage: spanMessage(status),
	})
}

// spanStatus follows the standard's reading rather than ours: on a SERVER
// span, only 5xx is an error. A 403 is the gateway working correctly, and a
// backend that paints every refusal red is a backend nobody looks at.
func spanStatus(status int) tracing.Status {
	if status >= 500 {
		return tracing.StatusError
	}
	return tracing.StatusUnset
}

func spanMessage(status int) string {
	if status >= 500 {
		return "HTTP " + strconv.Itoa(status)
	}
	return ""
}

// tracedTransport is the CLIENT span, and the ONE place the outgoing trace
// context is written when we are recording.
//
// Both at once, deliberately: the span id we put in the header has to be the
// id of a span we actually emit. Writing one without the other is how a trace
// ends up with a parent nobody ever reported - the failure this whole design
// is arranged to avoid.
type tracedTransport struct{ base http.RoundTripper }

func (t tracedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	n := spanOf(req.Context())
	if n == nil || n.dropped {
		return t.base.RoundTrip(req)
	}
	id := tracing.NewSpanID()
	// The next hop's parent is this call, not the crossing that contains it.
	out := n.ctx
	out.SpanID, out.Sampled = id, true
	req.Header.Set(tracing.Header, out.Header())

	start := time.Now()
	res, err := t.base.RoundTrip(req)

	attrs := []tracing.Attr{
		tracing.String("http.request.method", req.Method),
		tracing.String("server.address", req.URL.Host),
	}
	status := tracing.StatusUnset
	message := ""
	switch {
	case err != nil:
		// The service was never asked, or never answered. Recorded as OUR
		// failure to reach it rather than as its failure to answer.
		status, message = tracing.StatusError, err.Error()
	default:
		attrs = append(attrs, tracing.Int64("http.response.status_code", int64(res.StatusCode)))
		if res.StatusCode >= 500 {
			status = tracing.StatusError
		}
	}
	tracing.Emit(tracing.Span{
		TraceID: n.ctx.TraceID, SpanID: id, ParentSpanID: n.id,
		// The VERB, as OpenTelemetry names an HTTP client span: the host is an
		// attribute (server.address, above), and a backend that groups by name
		// then puts every call out together instead of one row per upstream.
		Name: req.Method, Kind: tracing.KindClient,
		Start: start, End: time.Now(),
		Attrs: attrs, Status: status, StatusMessage: message,
	})
	return res, err
}
