package gateway

import (
	"net/http"
	"strconv"
	"strings"
)

// What a gRPC call actually answered (ROUTE-20).
//
// A gRPC service answers 200 to every call and puts the verdict in the
// grpc-status TRAILER - or, when there is no body at all, in a header of the
// same name. Counted on the HTTP status alone, every failed call was a 2xx:
// a gRPC route read zero failures on the traffic screen, in the pushed
// metrics, in the access log and on the span, while its callers were getting
// UNAVAILABLE. So once the answer is written, a gRPC response is counted by
// what grpc-status says, translated to the HTTP status that means the same
// thing - the mapping gRPC publishes for its own gateways. What the CALLER
// receives is not touched: it still gets its 200 and its trailer.

// grpcAware returns the status to count for this answer: the HTTP one, unless
// the response is gRPC and its grpc-status says otherwise.
func grpcAware(h http.Header, status int) int {
	if status != http.StatusOK || !strings.HasPrefix(h.Get("Content-Type"), "application/grpc") {
		return status
	}
	// Read after the proxy finished: an announced trailer is in the header map
	// under its own name, an unannounced one under the trailer prefix, and a
	// trailers-only answer carried it as a plain header from the start.
	raw := h.Get("Grpc-Status")
	if raw == "" {
		raw = h.Get(http.TrailerPrefix + "Grpc-Status")
	}
	if raw == "" {
		return status
	}
	code, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return http.StatusInternalServerError
	}
	return grpcHTTPStatus(code)
}

// grpcHTTPStatus is gRPC's own table from a status code to an HTTP status
// (google.rpc.Code), so a dashboard reading the classes reads them as an HTTP
// API's would: the caller's fault in 4xx, the service's in 5xx.
func grpcHTTPStatus(code int) int {
	switch code {
	case 0: // OK
		return http.StatusOK
	case 1: // CANCELLED, by the caller
		return 499
	case 3, 9, 11: // INVALID_ARGUMENT, FAILED_PRECONDITION, OUT_OF_RANGE
		return http.StatusBadRequest
	case 4: // DEADLINE_EXCEEDED
		return http.StatusGatewayTimeout
	case 5: // NOT_FOUND
		return http.StatusNotFound
	case 6, 10: // ALREADY_EXISTS, ABORTED
		return http.StatusConflict
	case 7: // PERMISSION_DENIED
		return http.StatusForbidden
	case 8: // RESOURCE_EXHAUSTED
		return http.StatusTooManyRequests
	case 12: // UNIMPLEMENTED
		return http.StatusNotImplemented
	case 14: // UNAVAILABLE
		return http.StatusServiceUnavailable
	case 16: // UNAUTHENTICATED
		return http.StatusUnauthorized
	default: // UNKNOWN, INTERNAL, DATA_LOSS, and any code from the future
		return http.StatusInternalServerError
	}
}
