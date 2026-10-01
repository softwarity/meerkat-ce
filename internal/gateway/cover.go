package gateway

import (
	"bufio"
	"errors"
	"net"
	"net/http"
)

// What happens when a caller a rule turned away is then answered by a route
// that has nothing for them either.
//
// A refusal does not go back immediately: two routes may match the same paths
// for two audiences, and the one turned away from the first has to be able to
// land on the second. So the refusal is kept aside and the search continues -
// which is right, until the route that answers is a CATCH-ALL with nothing at
// that address. The caller then gets its 404, an answer about a page that does
// not exist, when the true answer was "you may not come in here". The refusal
// is lost, the operator sees a missing page, and the rule they just wrote looks
// like it did nothing.
//
// So the fallback's answer is held until its STATUS is known. Under 400 it is a
// real answer and everything streams through as before. At 400 and above nobody
// had anything for this caller, and the refusal kept aside is the truthful
// answer - it is written instead, in the shape the caller can read (a page for
// a browser, a status for an API client), which the catch-all's own error page
// was not.
//
// Only the status is held, never the body: a successful response streams the
// moment its header is written, so nothing here buffers a download.
type coveringWriter struct {
	http.ResponseWriter
	decided bool
	// covered says the fallback answered with an error and was swallowed.
	covered bool
}

func (c *coveringWriter) WriteHeader(code int) {
	if c.decided {
		return
	}
	c.decided = true
	if code >= http.StatusBadRequest {
		c.covered = true
		return
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *coveringWriter) Write(b []byte) (int, error) {
	if !c.decided {
		c.WriteHeader(http.StatusOK)
	}
	if c.covered {
		// Swallowed, and reported as written: a proxy that sees a short write
		// treats it as a broken connection and logs an error about nothing.
		return len(b), nil
	}
	return c.ResponseWriter.Write(b)
}

func (c *coveringWriter) Flush() {
	if c.covered {
		return
	}
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack passes through: an upgrade is a success by definition, and there is
// nothing left to arbitrate once the connection stops being an exchange of
// responses.
func (c *coveringWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := c.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("gateway: this connection cannot be hijacked")
	}
	c.decided = true
	return h.Hijack()
}

func (c *coveringWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
