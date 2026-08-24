package gateway

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/routing"
)

// gateChain wraps a handler with the route's gates - the bricks that accept or
// refuse (ROUTE-04): body and header size today, rate limiting and circuit
// breaking when they land.
//
// They run in declared order, outermost, before every other thing the route
// does. A gate that refuses has already written its answer, so the chain stops
// there and nothing downstream is reached: an oversized upload is turned away
// before the upstream is dialled, and before a single byte of the body is read.
//
// A route with no gate gets its handler back untouched and pays nothing per
// request.
func gateChain(gates []routing.Gate, next http.Handler) http.Handler {
	if len(gates) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, gate := range gates {
			if !gate(w, r) {
				return // the gate answered the caller itself
			}
		}
		next.ServeHTTP(w, r)
	})
}
