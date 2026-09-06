package gateway

import (
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/routing"
)

// Naming an endpoint nobody declared.
//
// A route with an OpenAPI spec, or with per-endpoint rules, gets exact
// templates - somebody wrote them. Most routes have neither, and for those the
// screen said nothing at all, which is a screen that answers "which endpoint is
// slow" with silence.
//
// So the shape is deduced from the path, and the deduction is FENCED by two
// things, because counting raw paths is not a shortcut - it is a denial of
// service. One series per order is 350 bytes kept for the life of the process,
// chosen by whoever sends the requests: `for i in 1..10000000; curl /orders/$i`
// is a gateway brought down by its own instrumentation.
//
//  1. The path is FOLDED before it is counted: a segment that looks like an
//     identifier becomes {id}, so /orders/1042 and /orders/1043 are one line
//     and not two - and so is /orders/<uuid>, since a marker per shape would
//     split one endpoint back into three.
//  2. What survives the fold is CAPPED per route. Past the budget, everything
//     lands in one bucket, so the worst a hostile path space can cost is the
//     budget - and it never grows after that.
//
// It is also, unlike a spec, sometimes WRONG: the 2024 in /files/2024/report is
// a year, not an identifier. Which is why these lines are marked deduced
// wherever they are shown, and never mixed with what a service declares.

// maxDeduced is how many templates one route may invent before it stops
// inventing. Two hundred is more operations than most services expose and far
// less than an attacker needs to hurt anyone: at ~350 bytes a series that is
// 70 KB per route, spent once.
const maxDeduced = 200

// otherTemplate is where everything past the budget goes. A bucket rather than
// a silence: "there is traffic here I could not name" is worth one line.
const otherTemplate = "(other)"

// deduceTemplate folds the segments of a path that look like identifiers.
//
// Three shapes, deliberately few. Every rule added catches more real ids and
// mistakes more real words for ids, and a rule that is wrong half the time
// makes the whole list untrustworthy - the failure this is fenced against is
// cardinality, not imprecision.
//
// Returns the path itself when nothing folds, which is the common case for a
// small service and costs no allocation.
func deduceTemplate(path string) string {
	if !needsFolding(path) {
		return path
	}
	var b strings.Builder
	b.Grow(len(path) + 8)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i < len(path) && path[i] != '/' {
			continue
		}
		b.WriteString(fold(path[start:i]))
		if i < len(path) {
			b.WriteByte('/')
		}
		start = i + 1
	}
	return b.String()
}

// needsFolding is the same walk without the building, so a path that folds to
// itself is returned as itself - no builder, no allocation, and that is most
// requests on most services.
func needsFolding(path string) bool {
	start := 0
	for i := 0; i <= len(path); i++ {
		if i < len(path) && path[i] != '/' {
			continue
		}
		if seg := path[start:i]; fold(seg) != seg {
			return true
		}
		start = i + 1
	}
	return false
}

// idPlaceholder is what every recognised shape folds to - ONE marker, not one
// per shape. /anything/1042 and /anything/<uuid> are the same endpoint served
// two ways, and giving them separate lines splits the answer to the question
// the screen exists for. Which KIND of identifier it was is not something an
// operator acts on.
const idPlaceholder = "{id}"

func fold(seg string) string {
	switch {
	case seg == "":
		return seg
	case allDigits(seg):
		// The known lie: a year, a version, a page number. Marked deduced for
		// exactly this reason.
		return idPlaceholder
	case len(seg) >= 16 && isHexish(seg):
		// A UUID, a digest, an object id, a request id - whatever dialect of
		// hex-and-dashes somebody chose. Sixteen because below that, hex is as
		// likely to be a word: "deadbeef", "facade", "added".
		//
		// Written as one rule rather than one per shape because the shapes are
		// endless: a strict 8-4-4-4-12 test let a 12-4-4-4-12 identifier
		// through, and each one that gets through is a series of its own - in
		// this gateway AND in whatever scrapes it.
		return idPlaceholder
	default:
		return seg
	}
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isHexish is hex digits and dashes, with at least one hex digit: this NAMES a
// series, it does not validate an identifier, so the shape is what matters and
// not which dialect produced it.
func isHexish(s string) bool {
	hex := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '-':
		case (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'):
			hex = true
		default:
			return false
		}
	}
	return hex
}

// deduce names a request no declared template covers, or returns nil once the
// route has spent its budget and even the (other) bucket cannot be reached -
// which cannot happen, but the shape says so rather than assuming it.
//
// Runs AFTER the answer has been written, like everything else on this path.
// The map is nested by method so the lookup key is the template alone: a
// concatenated "GET /x" key would be one allocation per request, spent on
// nothing.
func (s *opsSlot) deduce(r *http.Request, strip int) *metrics.Endpoint {
	if s.reg == nil {
		return nil
	}
	tpl := deduceTemplate(routing.StripSegments(r.URL.Path, strip))

	s.dedMu.RLock()
	e := s.deduced[r.Method][tpl]
	full := s.dedFull
	s.dedMu.RUnlock()
	if e != nil {
		return e
	}
	if full {
		return s.otherBucket()
	}

	s.dedMu.Lock()
	defer s.dedMu.Unlock()
	if e := s.deduced[r.Method][tpl]; e != nil {
		return e
	}
	if s.dedCount >= maxDeduced {
		s.dedFull = true
		return s.other()
	}
	if s.deduced == nil {
		s.deduced = map[string]map[string]*metrics.Endpoint{}
	}
	if s.deduced[r.Method] == nil {
		s.deduced[r.Method] = map[string]*metrics.Endpoint{}
	}
	e = s.reg.Deduced(s.routeID, r.Method, tpl)
	s.deduced[r.Method][tpl] = e
	s.dedCount++
	return e
}

func (s *opsSlot) otherBucket() *metrics.Endpoint {
	s.dedMu.Lock()
	defer s.dedMu.Unlock()
	return s.other()
}

// other is the overflow bucket, created once. Called under dedMu.
func (s *opsSlot) other() *metrics.Endpoint {
	if s.dedOther == nil {
		s.dedOther = s.reg.Deduced(s.routeID, "*", otherTemplate)
	}
	return s.dedOther
}
