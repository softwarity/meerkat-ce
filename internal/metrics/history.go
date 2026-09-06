package metrics

import "time"

// How an endpoint answers for a PERIOD rather than for the life of the process.
//
// The screen shows a route over the window it draws - seventeen minutes, an
// hour - and opening that route has to answer over the SAME period. Two clocks
// on one table is a table whose lines do not add up to the row above them, and
// a reader who notices that stops trusting the whole screen. The one thing
// worse than a caption explaining it is no caption.
//
// But the endpoints cannot join the sampled window: one entry per endpoint per
// five seconds is what every node would hold and what the ring would carry, and
// a service declaring two hundred operations makes that two hundred times the
// routing table. So they get their own history, and it is COARSE where the
// window is fine:
//
//   - one point a minute, not one every five seconds;
//   - three numbers, not a status class breakdown and a twelve-bucket
//     histogram - requests, errors and time spent are what a ranking is read
//     on, and the finer analysis is what a Prometheus is for;
//   - written only when something HAPPENED, so an endpoint nobody calls costs
//     nothing at all. That is not an optimisation, it is what makes the answer
//     correct with gaps: a gap means no traffic, so differencing against the
//     last point before the period still yields exactly the period's traffic.
//
// The resolution costs up to a minute of error at the edge of the period, which
// is invisible in seventeen and would be the wrong thing to spend memory on.

// historyPoints is how many minutes one endpoint remembers. More than the
// window is an hour, so a screen asking for the whole window is always answered
// from inside the ring rather than from its oldest edge.
const historyPoints = 70

// point is an endpoint's running totals at a minute boundary.
type point struct {
	at       int64 // unix seconds
	requests uint64
	errors   uint64
	micros   uint64
}

// totals is what the ring stores, read from the live counters.
func (e *Endpoint) totals() point {
	var requests uint64
	for i := range e.byClass {
		requests += e.byClass[i].Load()
	}
	return point{
		requests: requests,
		errors:   e.byClass[4].Load() + e.byClass[5].Load(),
		micros:   e.sumMicros.Load(),
	}
}

// roll appends a point when this endpoint moved since its last one. Called
// under the registry's write lock, once a minute.
func (e *Endpoint) roll(now time.Time) {
	t := e.totals()
	if n := len(e.hist); n > 0 {
		last := e.hist[n-1]
		if last.requests == t.requests && last.micros == t.micros {
			return // nothing happened; the last point still answers for now
		}
	} else if t.requests == 0 {
		return // never called, and it costs nothing to say so
	}
	t.at = now.Unix()
	if len(e.hist) == historyPoints {
		copy(e.hist, e.hist[1:])
		e.hist = e.hist[:historyPoints-1]
	}
	e.hist = append(e.hist, t)
}

// since is this endpoint's traffic over the period starting at from, and the
// instant the answer actually starts at - which is the nearest minute boundary
// the ring holds, or the start of the process when the ring reaches no further
// back than that.
//
// A zero from asks for everything since the process started.
func (e *Endpoint) since(from time.Time) (requests, errors uint64, secs float64, at time.Time) {
	now := e.totals()
	var base point
	if !from.IsZero() {
		want := from.Unix()
		for i := range e.hist {
			if e.hist[i].at <= want {
				base = e.hist[i]
				continue
			}
			break
		}
	}
	if base.at != 0 {
		at = time.Unix(base.at, 0)
	}
	return now.requests - base.requests,
		now.errors - base.errors,
		float64(now.micros-base.micros) / 1e6,
		at
}

// RollEndpoints advances every endpoint's history when a minute has passed.
// Called by the sampler on its own tick, so nothing else has to know it exists.
func (reg *Registry) RollEndpoints(now time.Time) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if now.Sub(reg.rolledAt) < time.Minute {
		return
	}
	reg.rolledAt = now
	for _, e := range reg.endpoints {
		e.roll(now)
	}
}
