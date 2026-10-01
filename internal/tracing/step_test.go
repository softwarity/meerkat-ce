package tracing

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type caught struct {
	mu    sync.Mutex
	spans []Span
}

func (c *caught) add(s Span) { c.mu.Lock(); c.spans = append(c.spans, s); c.mu.Unlock() }

func catch(t *testing.T) *caught {
	t.Helper()
	c := &caught{}
	RegisterExporter(c.add)
	before := Detail()
	t.Cleanup(func() { RegisterExporter(nil); SetDetail(before) })
	return c
}

// Off, a step is free: nothing is emitted and the context comes back as it
// was, so a call site never has to ask first.
func TestAStepIsNothingWhileTheDetailIsOff(t *testing.T) {
	c := catch(t)
	SetDetail(false)
	ctx := WithCurrent(context.Background(), "trace", "server")
	got, end := Step(ctx, "identity forward")
	end(nil)
	if got != ctx {
		t.Error("the context changed although the detail is off")
	}
	if len(c.spans) != 0 {
		t.Errorf("%d spans emitted with the detail off", len(c.spans))
	}
}

// On, a step hangs under the crossing, and a step inside it under the step -
// which is what lets the store's queries land under the handover that caused
// them rather than beside it.
func TestStepsNestUnderTheCrossing(t *testing.T) {
	c := catch(t)
	SetDetail(true)
	ctx := WithCurrent(context.Background(), "trace", "server")

	outer, endOuter := Step(ctx, "identity forward")
	_, endInner := Step(outer, "SELECT users")
	endInner(errors.New("no such row"))
	endOuter(nil)

	if len(c.spans) != 2 {
		t.Fatalf("%d spans, want the two steps", len(c.spans))
	}
	inner, outerSpan := c.spans[0], c.spans[1]
	if outerSpan.ParentSpanID != "server" || outerSpan.TraceID != "trace" {
		t.Errorf("the step is not under the crossing: %+v", outerSpan)
	}
	if inner.ParentSpanID != outerSpan.SpanID {
		t.Errorf("the inner step's parent = %q, want the outer step %q", inner.ParentSpanID, outerSpan.SpanID)
	}
	if outerSpan.Kind != KindInternal {
		t.Errorf("a step is of kind %d, want internal", outerSpan.Kind)
	}
	if inner.Status != StatusError || inner.StatusMessage != "no such row" {
		t.Errorf("a failed step does not say so: %+v", inner)
	}
}

// Nothing recorded, nothing to hang under: a request the sampler turned down
// costs its steps nothing either.
func TestAStepNeedsARecordedCrossing(t *testing.T) {
	c := catch(t)
	SetDetail(true)
	_, end := Step(context.Background(), "identity forward")
	end(nil)
	if len(c.spans) != 0 {
		t.Errorf("%d spans emitted with no crossing recorded", len(c.spans))
	}
}

// A route found untraced after the fact switches off every step under it,
// including the ones already open - no half trace for a route somebody left
// out of the telemetry.
func TestAnUntracedRouteSilencesItsSteps(t *testing.T) {
	c := catch(t)
	SetDetail(true)
	ctx := WithCurrent(context.Background(), "trace", "server")
	_, end := Step(ctx, "identity forward")
	DropCurrent(ctx)
	end(nil)
	if len(c.spans) != 0 {
		t.Errorf("%d spans emitted for a route that is not traced", len(c.spans))
	}
}
