package tracing

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
)

// What the gateway has to say about a request, and the seam the Enterprise
// exporter plugs into (OBS-04).
//
// The model is here and the exporter is in ee/telemetry, for the same reason
// the counters are in internal/metrics and their OTLP push is in ee/telemetry:
// the trunk describes, the Enterprise half ships it somewhere.
// The community binary does not link the exporter, so Emit finds nobody home
// and costs an atomic load.
//
// TWO SPANS PER REQUEST BY DEFAULT. A span per internal step - the route
// lookup, the access check - would be thousands per request and a collector
// bill to match. What those steps are worth saying is said as ATTRIBUTES and
// EVENTS on the two spans that already exist, which cost bytes rather than
// objects.
//
// Measured, that was not enough: on a real installation the events landed at
// 0.02 ms and the gateway's own 1.6 ms happened after them, where nothing was
// said. So an installation can ask for the gateway's detail - a handful of
// steps per crossing, under it - and it is off unless somebody does (step.go).

// Kind is the halves of a proxy's work, and the standard's numbers for them:
// what we answered, what we called, and - when asked - the steps in between.
type Kind int

const (
	// KindInternal is a step of the gateway's own work, inside the crossing:
	// the identity handed to an upstream, a query to the store. Emitted only
	// when the installation asks for the gateway's detail (see Step).
	KindInternal Kind = 1
	// KindServer covers the whole crossing: from the request arriving to the
	// response being written. NOT to the upstream answering - a gateway keeps
	// working after that, rewriting HTML and injecting what it injects.
	KindServer Kind = 2
	// KindClient is the call out, nested inside the server span. The gap
	// between the two is the gateway's OWN time, which is the number nobody
	// else can produce.
	KindClient Kind = 3
)

// Status is what the standard says about how it ended. Unset is the honest
// default: a 404 is not an error, it is an answer.
type Status int

// The standard's three: nothing claimed, it went well, it did not.
const (
	StatusUnset Status = 0
	StatusOK    Status = 1
	StatusError Status = 2
)

// Attr is one thing worth knowing about a span. A string or an integer, which
// covers everything a gateway has to say; the standard allows more, and the
// day we need a third kind is the day to add it.
type Attr struct {
	Key string
	Str string
	Int int64
	// IsInt picks the branch, because the zero value of Int is a real number
	// somebody may want to record.
	IsInt bool
	// Strs is a list of texts - the roles a caller holds - and IsList picks
	// that branch, for the same reason: an empty list is still an answer.
	Strs   []string
	IsList bool
}

// String makes a text attribute.
func String(key, value string) Attr { return Attr{Key: key, Str: value} }

// Int64 makes a numeric one.
func Int64(key string, value int64) Attr { return Attr{Key: key, Int: value, IsInt: true} }

// Strings makes a list of texts.
func Strings(key string, values []string) Attr { return Attr{Key: key, Strs: values, IsList: true} }

// Event is a named INSTANT inside a span - "the route was chosen", "access was
// granted". The cheap way to record a step: a few bytes on a span that already
// exists, rather than a span of its own.
type Event struct {
	Name string
	At   time.Time
	Attr []Attr
}

// Span is one unit of work, with a beginning and an end. Not an instant: a
// duration is a span, and two spans are never subtracted to find one.
type Span struct {
	TraceID string
	SpanID  string
	// ParentSpanID is empty at the root, which is the case whenever this
	// gateway opened the journey itself.
	ParentSpanID string
	Name         string
	Kind         Kind
	Start        time.Time
	End          time.Time
	Attrs        []Attr
	Events       []Event
	Status       Status
	// StatusMessage says what went wrong, and only then.
	StatusMessage string
}

// Duration is what the span measures.
func (s Span) Duration() time.Duration { return s.End.Sub(s.Start) }

// NewSpanID names a step. Exported because the router needs one for the
// context it hands to the next hop.
func NewSpanID() string { return randomHex(8) }

// exporter is where finished spans go, nil in the community binary. A pointer
// swapped atomically rather than a mutex: this is read on every request.
var exporter atomic.Pointer[func(Span)]

// RegisterExporter is called from ee/telemetry's init(), and again whenever
// the operator changes where spans go. A nil function turns it back off.
func RegisterExporter(f func(Span)) {
	if f == nil {
		exporter.Store(nil)
		return
	}
	exporter.Store(&f)
}

// Exporting says whether anybody is listening, so a caller can skip building
// what nobody will read.
func Exporting() bool { return exporter.Load() != nil }

// Emit hands a finished span over. It never blocks and never fails: an
// exporter that cannot keep up drops, because a gateway that waits on a
// collector is a gateway a collector can take down.
func Emit(s Span) {
	if f := exporter.Load(); f != nil {
		(*f)(s)
	}
}

// sampleRate is the share of journeys WE open that get recorded, times a
// thousand so the decision is integer arithmetic on the hot path.
var sampleRate atomic.Int64

// SetSampleRate takes a share between 0 and 1. Out-of-range values are pulled
// back into it rather than refused: a rate is a dial, and a dial that throws
// is a dial nobody turns.
func SetSampleRate(share float64) {
	switch {
	case share <= 0:
		sampleRate.Store(0)
	case share >= 1:
		sampleRate.Store(1000)
	default:
		sampleRate.Store(int64(share * 1000))
	}
}

// SampleRate is what is set right now.
func SampleRate() float64 { return float64(sampleRate.Load()) / 1000 }

// ceiling is the absolute bound: at most this many journeys recorded per
// second, whoever decided. Zero means no ceiling.
var ceiling atomic.Int64

// The current second and what has been spent in it. Not a token bucket - a
// counter reset on the second, which is allocation-free and close enough for
// a spending limit. The reset races benignly: the worst case is a slightly
// generous burst, never a blocked request.
var (
	windowSecond atomic.Int64
	windowSpent  atomic.Int64
)

// SetMaxPerSecond bounds how many journeys are recorded per second. Zero
// removes the bound.
func SetMaxPerSecond(n int64) {
	if n < 0 {
		n = 0
	}
	ceiling.Store(n)
}

// MaxPerSecond is what is set right now.
func MaxPerSecond() int64 { return ceiling.Load() }

// affordable takes one from this second's allowance.
func affordable() bool {
	allowed := ceiling.Load()
	if allowed <= 0 {
		return true
	}
	now := time.Now().Unix()
	if windowSecond.Swap(now) != now {
		windowSpent.Store(0)
	}
	return windowSpent.Add(1) <= allowed
}

// ShouldRecord decides whether this journey is worth exporting.
//
// THE DECISION IS MADE ONCE, AT THE HEAD, and everybody downstream reads the
// flag rather than rolling the dice again. Get this wrong and a browser
// sampling at 100% produces spans whose parent - decided at 10% here - was
// never exported: traces that look broken in the backend, for a bug nobody
// finds by reading either side alone.
//
// So a context that came from a caller carries a decision, and we respect it.
// Only a journey we opened ourselves is ours to decide.
//
// WHICH LEAVES THE ABUSE, and it is real: the flag is a byte anybody can send,
// and somebody sending 01 on every request makes every service downstream
// export - which is not their bill. The obvious answer, believing the flag
// only from inside the cluster, does not survive contact with a deployment:
// behind an ingress or a load balancer the peer address is the ingress, so a
// private-address test calls the entire internet trusted. And a list of
// trusted proxies is the setting this codebase refuses elsewhere for the same
// reason (internal/filters/scheme.go).
//
// So the leverage is BOUNDED instead of the caller being judged: a ceiling on
// journeys recorded per second, which caps the cost absolutely whoever decided
// and needs nobody to be identified. The question an operator can always
// answer is how much they are willing to pay, never who deserves belief.
func ShouldRecord(sc SpanContext) bool {
	if sc.Inbound {
		return sc.Sampled && affordable()
	}
	rate := sampleRate.Load()
	if rate <= 0 {
		return false
	}
	if rate < 1000 && rand.Int64N(1000) >= rate {
		return false
	}
	return affordable()
}

// Config is where spans go, filled by whoever configures the gateway and read
// by the Enterprise exporter. It lives here rather than in ee/telemetry so the
// trunk can carry the operator's answer without linking the exporter.
type Config struct {
	Endpoint string
	Headers  map[string]string
	Service  string
	Version  string
	// Sample is the share of journeys WE open that get recorded. A journey a
	// caller already decided about is not ours to decide again.
	Sample float64
	// MaxPerSecond is the ceiling on journeys recorded per second, whoever
	// decided - the budget, and the only thing that bounds a caller who sets
	// the sampling flag on every request. Zero means no ceiling.
	MaxPerSecond int64
	// Detail adds the gateway's own steps to the recorded traces (step.go).
	Detail bool
	// Caller names the signed-in caller on the spans (person.go).
	Caller bool
}

// starter is what ee/telemetry registers. Nil in the community binary, which
// is what makes tracing an Enterprise feature: absent code refuses by itself,
// and this only has to say so.
var starter func(Config) (stop func(), err error)

// RegisterStarter is called from ee/telemetry's init().
func RegisterStarter(f func(Config) (func(), error)) { starter = f }

// Start points the gateway at a collector. It answers WHY rather than failing
// quietly when this binary is the community one: an operator who set an
// endpoint and sees no traces deserves to be told which of the two reasons it
// is.
func Start(cfg Config) (stop func(), err error) {
	if starter == nil {
		return nil, errNotEnterprise
	}
	SetSampleRate(cfg.Sample)
	return starter(cfg)
}

// errNotEnterprise names the edition rather than the missing symbol.
var errNotEnterprise = notEnterprise{}

type notEnterprise struct{}

func (notEnterprise) Error() string {
	return "exporting traces is an Enterprise feature: this is the community image, which links no exporter. " +
		"The trace context still travels (traceparent is generated and forwarded) and the identifier is in the access log."
}

// running is the exporter currently installed, so that changing where traces
// go is one call rather than a dance the caller has to get right.
var (
	runningMu   sync.Mutex
	runningStop func()
)

// The relay opens for the pages that start their own journeys, and closed is
// the default: an installation exporting its own spans but injecting nothing
// into anybody's pages has no reason to hold a door open.
//
// TWO ANSWERS DECIDE IT, and they come from two places, which is why neither
// can set it alone: the setting says traces leave this gateway at all, and the
// ROUTES say whether any page was given the bundle. The routes are read on
// every reload, the setting whenever an operator changes it, so each posts its
// half and the relay follows.
var (
	exporting     atomic.Bool
	browserWanted atomic.Bool
)

// SetBrowserWanted is the routes' half: at least one of them starts its
// journey in the page. Called by the router on every reload, so adding the
// first such route opens the path and removing the last one closes it.
func SetBrowserWanted(v bool) { browserWanted.Store(v) }

// BrowserWanted is what the routes last said.
func BrowserWanted() bool { return browserWanted.Load() }

// relayBudget is how many batches a second the relay takes when a page is
// starting journeys. Derived rather than configured: a page flushes every five
// seconds, so this is roughly a thousand concurrent tabs - far past any real
// audience, and far short of what a flood would like to spend.
const relayBudget = 200

// Apply makes the gateway's tracing match cfg, whatever it was doing before.
// Called at startup and again whenever an operator changes the setting, which
// is why it is idempotent and why turning it off is Apply with Enabled false
// rather than a second entry point.
func Apply(cfg Config, enabled bool) error {
	runningMu.Lock()
	defer runningMu.Unlock()

	// Stop first, always. Two exporters pointed at two collectors would each
	// get half the spans, which is worse than either alone.
	if runningStop != nil {
		runningStop()
		runningStop = nil
	}
	SetSampleRate(cfg.Sample)
	SetMaxPerSecond(cfg.MaxPerSecond)
	SetRelayMaxPerSecond(relayBudget)
	SetDetail(cfg.Detail)
	SetCaller(cfg.Caller)
	exporting.Store(enabled)
	if !enabled {
		return nil
	}
	stop, err := Start(cfg)
	if err != nil {
		return err
	}
	runningStop = stop
	return nil
}

// Stop tears down whatever is running. For shutdown, and for a test that has
// to leave the process as it found it.
func Stop() {
	runningMu.Lock()
	defer runningMu.Unlock()
	if runningStop != nil {
		runningStop()
		runningStop = nil
	}
}

// The browser half's two seams (OBS-04).
//
// The bundle and the relay both live in ee/telemetry - the script is 80 KB the
// community binary does not carry, and relaying to a collector is the same
// Enterprise decision as exporting to one. The data plane asks here and gets
// nothing when this is the community image, which is what makes /meerkat/
// telemetry.js a 404 there rather than a switch that does nothing.

var (
	bundle atomic.Pointer[[]byte]
	relay  atomic.Pointer[func([]byte) error]
)

// RegisterBundle hands over the OpenTelemetry script, from ee/telemetry's
// init(). Absent when the Enterprise binary was built without `make telemetry`.
func RegisterBundle(js []byte) {
	if len(js) == 0 {
		bundle.Store(nil)
		return
	}
	bundle.Store(&js)
}

// Bundle is the script the data plane serves, and whether there is one.
func Bundle() ([]byte, bool) {
	if p := bundle.Load(); p != nil {
		return *p, true
	}
	return nil, false
}

// RegisterRelay installs the forwarder a page's spans go through. Nil turns it
// off - which is what happens when the export is switched off, because a relay
// with nowhere to forward to is an open door to nothing.
func RegisterRelay(f func([]byte) error) {
	if f == nil {
		relay.Store(nil)
		return
	}
	relay.Store(&f)
}

// Relaying says whether the path is open, so the data plane can answer 404
// rather than accept a body it will drop. Open when the export is running AND
// some route starts its journeys in the page: a gateway that injects into
// nobody's pages holds no door for anybody's batches.
func Relaying() bool { return relay.Load() != nil && browserWanted.Load() }

// The relay's own budget, in batches per second. Separate from the span
// ceiling because it bounds a different thing: that one is what WE decide to
// record, this one is what strangers ask us to forward.
var (
	relayCeiling atomic.Int64
	relaySecond  atomic.Int64
	relaySpent   atomic.Int64
)

// SetRelayMaxPerSecond bounds how many batches a page may push through. Zero
// removes the bound.
func SetRelayMaxPerSecond(n int64) {
	if n < 0 {
		n = 0
	}
	relayCeiling.Store(n)
}

// ErrRelayBusy is what a caller past the budget gets, and it is deliberately
// not an error about them: there is no way to tell a legitimate page from a
// flood, so the answer is the same and it is temporary.
var ErrRelayBusy = relayBusy{}

type relayBusy struct{}

func (relayBusy) Error() string { return "too many span batches this second" }

// RelaySpans forwards one batch from a page to the collector.
//
// WHY THROUGH US AT ALL: the alternative is a collector reachable from every
// user's browser, which means exposing it on the internet with CORS and
// accepting unauthenticated bodies from anybody. Relaying keeps the collector
// private and puts the page's spans behind the door that is already there.
//
// WHICH LEAVES US HOLDING THE DOOR. The bundle runs on public pages too, so
// requiring a session would silence exactly the anonymous visits worth seeing
// - and that makes this path one an anonymous caller may post to. So it is
// BOUNDED rather than judged, the same reckoning as the sampling flag: a
// budget in batches per second, which caps what a flood can spend of somebody
// else's collector and needs nobody to be identified.
func RelaySpans(body []byte) error {
	f := relay.Load()
	if f == nil || !browserWanted.Load() {
		return errNotRelaying
	}
	if !relayAffordable() {
		return ErrRelayBusy
	}
	return (*f)(body)
}

func relayAffordable() bool {
	allowed := relayCeiling.Load()
	if allowed <= 0 {
		return true
	}
	now := time.Now().Unix()
	if relaySecond.Swap(now) != now {
		relaySpent.Store(0)
	}
	return relaySpent.Add(1) <= allowed
}

var errNotRelaying = notRelaying{}

type notRelaying struct{}

func (notRelaying) Error() string {
	return "this gateway is not relaying browser spans: the export is off, or this is the community image"
}
