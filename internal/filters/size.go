package filters

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Request size, shared by the gate bricks that refuse on it (ROUTE-04).
//
// A gate is not a modifier: a modifier transforms a request and its signature
// carries neither an error nor a response, so it has no way to say no. What
// lives here is everything a refusal needs - how a size is written by a human,
// how a header block is weighed, and how the refusal is phrased.

// ParseSize reads a size as someone writes it: 2MB, 512KB, 1048576. Bytes when
// no unit is given.
//
// A written unit rather than a raw count of bytes, because this ends up in an
// exported YAML that another person reads: "2MB" says what it is, "2097152"
// has to be divided twice before it says anything.
func ParseSize(s string) (int64, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, errors.New("a size is required, e.g. 2MB, 512KB or a number of bytes")
	}
	upper := strings.ToUpper(strings.ReplaceAll(raw, " ", ""))
	mult := int64(1)
	switch {
	case strings.HasSuffix(upper, "KB"), strings.HasSuffix(upper, "K"):
		mult, upper = 1<<10, strings.TrimSuffix(strings.TrimSuffix(upper, "KB"), "K")
	case strings.HasSuffix(upper, "MB"), strings.HasSuffix(upper, "M"):
		mult, upper = 1<<20, strings.TrimSuffix(strings.TrimSuffix(upper, "MB"), "M")
	case strings.HasSuffix(upper, "GB"), strings.HasSuffix(upper, "G"):
		mult, upper = 1<<30, strings.TrimSuffix(strings.TrimSuffix(upper, "GB"), "G")
	case strings.HasSuffix(upper, "B"):
		upper = strings.TrimSuffix(upper, "B")
	}
	n, err := strconv.ParseFloat(upper, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a size: write a number with an optional unit, e.g. 2MB, 512KB, 1048576", s)
	}
	if n <= 0 {
		// Zero would read as "refuse everything", which takes a route off the
		// air without a word. Removing the brick is how one removes a limit.
		return 0, fmt.Errorf("%q is not a usable limit: a size must be greater than zero (remove the gate to lift the limit)", s)
	}
	return int64(n * float64(mult)), nil
}

// FormatBytes renders a size the way it was probably typed - 2 MB, 512 KB -
// and falls back to bytes below a kilobyte.
func FormatBytes(n int64) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return strconv.FormatInt(n>>20, 10) + " MB"
	case n >= 1<<10 && n%(1<<10) == 0:
		return strconv.FormatInt(n>>10, 10) + " KB"
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return strconv.FormatInt(n, 10) + " bytes"
	}
}

// HeaderBytes weighs the header block the way Go's own server weighs it for
// Server.MaxHeaderBytes: the request line, then each field as
// "Name: value\r\n". Counting the parsed map rather than the bytes on the wire
// is off by whatever whitespace the client used, and deliberately so - the
// number an admin sets is about what the upstream will have to parse, not
// about the client's formatting.
func HeaderBytes(r *http.Request) int64 {
	target := r.RequestURI
	if target == "" { // synthesised requests (tests, internal calls)
		target = r.URL.RequestURI()
	}
	n := int64(len(r.Method) + len(target) + len(r.Proto) + 4)
	for name, values := range r.Header {
		for _, v := range values {
			n += int64(len(name) + len(v) + 4) // ": " and CRLF
		}
	}
	// The Host is a header on the wire and not in the map.
	if r.Host != "" {
		n += int64(len("Host") + len(r.Host) + 4)
	}
	return n
}

// HasBody says whether there is anything to weigh. The Upgrade case is the one
// that matters: a WebSocket has no body, it has a CONVERSATION, and the bytes
// of that conversation flow through the same reader. Capping it would cut the
// socket the moment a chat window passed the limit - which is exactly the trap
// RewriteBody refuses for streams, on the other side.
func HasBody(r *http.Request) bool {
	if r.Body == nil || r.Body == http.NoBody {
		return false
	}
	return !IsUpgrade(r)
}

// IsUpgrade reports a request asking to switch protocol.
func IsUpgrade(r *http.Request) bool {
	for _, v := range r.Header.Values("Connection") {
		for _, tok := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), "upgrade") {
				return true
			}
		}
	}
	return false
}

// RefuseSize answers with the status AND the arithmetic. "Too large" alone is
// the kind of error that costs an afternoon: it says neither by how much nor
// what the limit is, and the limit lives somewhere neither the caller nor the
// application developer can see. A negative got means the length was never
// declared, so only the cap is worth reporting.
func RefuseSize(w http.ResponseWriter, r *http.Request, status int, what string, got, limit int64) {
	msg := fmt.Sprintf("%s: %s received, this route accepts %s", what, FormatBytes(got), FormatBytes(limit))
	if got < 0 {
		msg = fmt.Sprintf("%s: this route accepts %s", what, FormatBytes(limit))
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Nothing was consumed, so nothing is cached: the next request with a
	// smaller body must not be answered from this.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r != nil && r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(msg + "\n"))
}

// OversizeBody reports whether an error from the proxy is a caller who blew
// through the body cap rather than an upstream that failed. Without it the
// refusal reaches the client as "502 upstream unavailable" - a lie that sends
// everyone to look at a service which was never called.
func OversizeBody(err error) (*http.MaxBytesError, bool) {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return mbe, true
	}
	return nil, false
}
