package gateway

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/logging"
	"github.com/softwarity/meerkat/internal/tracing"
)

// Who crossed the front door, written once the answer is out (OBS-03).
//
// The line is assembled in TWO places because that is where the two halves are
// known: the identity, while the request is being let through or turned away,
// and everything else once it has been answered. A note travels between them
// in the context - and only when somebody asked for an access log, so an
// installation that never turned it on allocates nothing and looks nothing up.
//
// WHY THE IDENTITY IS STASHED rather than read again at the end: resolving a
// caller can mean a session lookup, and doing it a second time on every
// request to write it down would make the log cost more than the request.

// accessNote is what the let-through path learns and the answering path writes.
// A pointer in the context, mutated in place: there is one request per note,
// and it is filled before it is read.
type accessNote struct {
	user   string
	tenant string
	token  string
}

type accessKey struct{}

// withAccess attaches a note when the access log is on, and hands back the
// request untouched when it is off - which is the state of almost every
// installation, and the one that has to cost nothing.
func withAccess(req *http.Request) *http.Request {
	if !logging.AccessLogEnabled() {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), accessKey{}, &accessNote{}))
}

// noteOf reads the note back, nil when there is nothing to write.
func noteOf(ctx context.Context) *accessNote {
	n, _ := ctx.Value(accessKey{}).(*accessNote)
	return n
}

// noteIdentity records who this turned out to be. Called where the router has
// just resolved it anyway, for the cost of three assignments.
func noteIdentity(ctx context.Context, d identityData) {
	n := noteOf(ctx)
	if n == nil {
		return
	}
	n.user, n.tenant = d.Username, d.Tenant
	if n.tenant == "" {
		n.tenant = d.TenantID
	}
}

// writeAccess puts down one crossing. route and endpoint are empty for a
// request that matched nothing, which is itself worth recording: a rising
// number of them is somebody calling a path this gateway does not serve.
func writeAccess(req *http.Request, routeName, endpoint string, status int, bytes int64, took time.Duration) {
	if !logging.AccessLogEnabled() {
		return
	}
	a := logging.Access{
		TraceID:  tracing.ID(req.Context()),
		Method:   req.Method,
		Path:     req.URL.Path,
		Endpoint: endpoint,
		Route:    routeName,
		Status:   status,
		Bytes:    bytes,
		Duration: took,
		IP:       callerIP(req),
		Outcome:  outcomeOf(status),
	}
	if n := noteOf(req.Context()); n != nil {
		a.User, a.Tenant, a.Token = n.user, n.tenant, n.token
	}
	logging.Write(a)
}

// outcomeOf says in ONE word what happened, so the lines worth looking at are
// greppable by somebody who does not know the status codes by heart.
//
// The distinction that earns its keep is refused against failed: a 403 is the
// gateway doing its job and an auditor's most interesting line, a 502 is a
// service that is down. Lumping both under "error" would bury the first in the
// second on the morning an upstream falls over.
func outcomeOf(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "refused"
	case status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout:
		return "upstream-down"
	case status >= 500:
		return "failed"
	case status >= 400:
		return "rejected"
	default:
		return "ok"
	}
}

// callerIP is the address this gateway resolved, not the one a header claimed.
// X-Forwarded-For is deliberately NOT read here: an audit line saying who
// called must not be a line the caller wrote themselves.
func callerIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}
