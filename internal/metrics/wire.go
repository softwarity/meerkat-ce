package metrics

import (
	"errors"
	"strconv"
	"strings"
)

// What one node tells the others, on the wire.
//
// Text, and deliberately: this travels as a PostgreSQL notification payload,
// which is what a person reads when they go looking at why a curve is wrong.
// A compressed blob would be a third of the size and would turn every
// investigation into a decoding exercise.
//
// The shape, one message:
//
//	m1 <inFlight> <logins> <refused> <unmatched> <route> <route> ...
//	route = <id>:<class0>.<..>.<class5>:<bucket0>.<..>:<fail0>.<..>:<sumMicros>
//
// Numbers are TOTALS since that node started, never intervals - see fleet.go
// for why that is what makes a lossy bus usable.
//
// The name is not sent. Every node reads the same routing table from the same
// database, so it already knows what a route is called; sending it would cost
// bytes on every message and would need escaping for a field a person types.
const wireVersion = "m1"

// Chunk is the largest a message may be. Below MaxSignalBytes in
// internal/store, with room for the topic and node id the bus puts in front.
const Chunk = 6500

// Encode turns totals into one or more messages, none longer than limit.
//
// Several rather than one truncated: a receiver merges what arrives into what
// it already had, so a message is a partial update by construction and
// splitting one costs nothing. Dropping the tail instead would leave the last
// routes of the table permanently unreported - and they would be the same
// routes every time, since the order is stable.
func Encode(s Snapshot, limit int) []string {
	if limit <= 0 {
		limit = Chunk
	}
	head := wireVersion + " " +
		strconv.FormatInt(s.InFlight, 10) + " " +
		strconv.FormatUint(s.Logins, 10) + " " +
		strconv.FormatUint(s.Refused, 10) + " " +
		strconv.FormatUint(s.Unmatched, 10)

	var out []string
	var b strings.Builder
	b.WriteString(head)
	for _, r := range s.Routes {
		// A separator inside an id would split a record in two and produce a
		// number where a name was expected. Skipped rather than escaped: an id
		// like that does not exist in this product, and a skipped route is a
		// missing curve while a corrupt stream is a wrong one.
		if strings.ContainsAny(r.ID, " :.") {
			continue
		}
		rec := encodeRoute(r)
		if b.Len()+1+len(rec) > limit && b.Len() > len(head) {
			out = append(out, b.String())
			b.Reset()
			b.WriteString(head)
		}
		b.WriteByte(' ')
		b.WriteString(rec)
	}
	return append(out, b.String())
}

func encodeRoute(r RouteSnapshot) string {
	var b strings.Builder
	b.WriteString(r.ID)
	b.WriteByte(':')
	for i, n := range r.ByClass {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(strconv.FormatUint(n, 10))
	}
	b.WriteByte(':')
	for i, n := range r.Buckets {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(strconv.FormatUint(n, 10))
	}
	b.WriteByte(':')
	for i, n := range r.Failures {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(strconv.FormatUint(n, 10))
	}
	b.WriteByte(':')
	b.WriteString(strconv.FormatUint(uint64(r.SumSecs*1e6), 10))
	return b.String()
}

// ErrWireVersion says the message came from a build that speaks another
// dialect. During a rolling update the two versions are live at once, and a
// node that cannot read a message must ignore it rather than guess.
var ErrWireVersion = errors.New("metrics: a node is speaking another version of the wire format")

// Decode reads one message.
func Decode(msg string) (Snapshot, error) {
	fields := strings.Split(msg, " ")
	if len(fields) < 5 {
		return Snapshot{}, errors.New("metrics: truncated message")
	}
	if fields[0] != wireVersion {
		return Snapshot{}, ErrWireVersion
	}
	var s Snapshot
	inFlight, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return Snapshot{}, err
	}
	s.InFlight = inFlight
	for i, into := range []*uint64{&s.Logins, &s.Refused, &s.Unmatched} {
		n, err := strconv.ParseUint(fields[2+i], 10, 64)
		if err != nil {
			return Snapshot{}, err
		}
		*into = n
	}
	for _, rec := range fields[5:] {
		r, err := decodeRoute(rec)
		if err != nil {
			return Snapshot{}, err
		}
		s.Routes = append(s.Routes, r)
	}
	return s, nil
}

func decodeRoute(rec string) (RouteSnapshot, error) {
	parts := strings.Split(rec, ":")
	if len(parts) != 5 {
		return RouteSnapshot{}, errors.New("metrics: a route record is not five fields")
	}
	out := RouteSnapshot{ID: parts[0]}
	classes, err := numbers(parts[1])
	if err != nil {
		return out, err
	}
	for i := range out.ByClass {
		if i < len(classes) {
			out.ByClass[i] = classes[i]
		}
	}
	if out.Buckets, err = numbers(parts[2]); err != nil {
		return out, err
	}
	fails, err := numbers(parts[3])
	if err != nil {
		return out, err
	}
	for i := range out.Failures {
		if i < len(fails) {
			out.Failures[i] = fails[i]
		}
	}
	micros, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		return out, err
	}
	out.SumSecs = float64(micros) / 1e6
	return out, nil
}

func numbers(field string) ([]uint64, error) {
	raw := strings.Split(field, ".")
	out := make([]uint64, 0, len(raw))
	for _, s := range raw {
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// Moved returns the routes whose totals differ from what was last sent, and
// the new "last sent" to keep. Sending only these is what keeps a message
// under the payload limit on a table of a hundred and fifty routes, most of
// which see no traffic in any given five seconds.
func Moved(now Snapshot, sent map[string]RouteSnapshot) (Snapshot, map[string]RouteSnapshot) {
	out := Snapshot{At: now.At, InFlight: now.InFlight, Logins: now.Logins,
		Refused: now.Refused, Unmatched: now.Unmatched}
	keep := make(map[string]RouteSnapshot, len(now.Routes))
	for _, r := range now.Routes {
		keep[r.ID] = r
		if before, ok := sent[r.ID]; ok && before.Total() == r.Total() && before.SumSecs == r.SumSecs {
			continue
		}
		out.Routes = append(out.Routes, r)
	}
	return out, keep
}
