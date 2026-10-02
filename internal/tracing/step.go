package tracing

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// The gateway's own detail (OBS-04): steps INSIDE a crossing, off by default.
//
// WHY IT EXISTS. The two spans a crossing emits give the gateway's own time as a
// single number - the gap between the server span and the client span - and on
// a real installation that number was 1.6 ms of a 9 ms crossing, with the two
// events that were meant to explain it (route chosen, access granted) both
// landing at 0.02 ms. Everything that cost anything happened after them, where
// nothing was said. Events mark instants; they cannot divide a stretch of time.
//
// WHY IT IS OFF. The original reasoning still holds for most installations: a
// span per step multiplies what a collector stores, and somebody operating
// their own services does not need the inside of the gateway in every trace.
// So it is a decision - "detail the gateway's own work" - taken on the
// telemetry screen, for as long as somebody is looking.
//
// WHAT IT COSTS WHEN OFF: one atomic load per step, and the context comes back
// untouched. And it never adds a trace: it only deepens the ones already
// sampled, so the rate and the ceiling mean what they meant.

var detail atomic.Bool

// SetDetail turns the gateway's steps on or off, live.
func SetDetail(on bool) { detail.Store(on) }

// Detail is what is set right now.
func Detail() bool { return detail.Load() }

// current is the span a step hangs under. A pointer carried in the context, so
// a package that knows nothing of the router - the store - can hang its own
// steps under the crossing that caused them, and so a route that is not traced
// can switch every step off after the fact.
type current struct {
	traceID string
	spanID  string
	// crossing is shared by every step under it, however deep.
	crossing *crossing
}

// crossing holds the steps of one request until its server span is settled.
// A step that ends before the router has chosen the route - the session's
// queries - cannot know yet whether that route is traced: sent at once, it
// would be a child of a span that is then never reported.
type crossing struct {
	off  atomic.Bool
	mu   sync.Mutex
	held []Span
	sent bool // settled and reported: a step ending later leaves at once
}

func (c *crossing) emit(s Span) {
	c.mu.Lock()
	if !c.sent {
		c.held = append(c.held, s)
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	if !c.off.Load() {
		Emit(s)
	}
}

type currentKey struct{}

// WithCurrent makes a recorded span the parent of the steps taken under ctx.
// Called by the router for its server span, and by Step for its own.
func WithCurrent(ctx context.Context, traceID, spanID string) context.Context {
	return context.WithValue(ctx, currentKey{}, &current{traceID: traceID, spanID: spanID, crossing: &crossing{}})
}

// SendCurrent reports the steps held under ctx, once its server span is
// reported: they leave together, or not at all.
func SendCurrent(ctx context.Context) {
	c, ok := ctx.Value(currentKey{}).(*current)
	if !ok || c == nil {
		return
	}
	c.crossing.mu.Lock()
	held := c.crossing.held
	c.crossing.held, c.crossing.sent = nil, true
	c.crossing.mu.Unlock()
	if c.crossing.off.Load() {
		return
	}
	for _, s := range held {
		Emit(s)
	}
}

// DropCurrent says the span under ctx is not reported after all - a route that
// is not traced, known only once the router chose it. Every step under it then
// emits nothing, including the ones already opened.
func DropCurrent(ctx context.Context) {
	if c, ok := ctx.Value(currentKey{}).(*current); ok && c != nil {
		c.crossing.off.Store(true)
		c.crossing.mu.Lock()
		c.crossing.held = nil
		c.crossing.mu.Unlock()
	}
}

// Step opens a step of the gateway's own work under the current span, and
// returns the context its own steps hang under and the function that ends it.
//
// Free when nothing asked for it: the detail is off, nothing is recorded, or
// the route is not traced. The context then comes back as it was, and the end
// function does nothing - so a call site never has to ask first.
func Step(ctx context.Context, name string, attrs ...Attr) (context.Context, func(err error, more ...Attr)) {
	if !detail.Load() {
		return ctx, noEnd
	}
	parent, ok := ctx.Value(currentKey{}).(*current)
	if !ok || parent == nil || parent.crossing.off.Load() {
		return ctx, noEnd
	}
	id := NewSpanID()
	child := &current{traceID: parent.traceID, spanID: id, crossing: parent.crossing}
	start := time.Now()
	return context.WithValue(ctx, currentKey{}, child), func(err error, more ...Attr) {
		// Checked again at the end: a route found untraced after the step
		// began must not leave half a trace behind.
		if parent.crossing.off.Load() {
			return
		}
		span := Span{
			TraceID: parent.traceID, SpanID: id, ParentSpanID: parent.spanID,
			Name: name, Kind: KindInternal,
			Start: start, End: time.Now(),
			Attrs: append(attrs, more...),
		}
		if err != nil {
			span.Status, span.StatusMessage = StatusError, err.Error()
		}
		parent.crossing.emit(span)
	}
}

func noEnd(error, ...Attr) {}
