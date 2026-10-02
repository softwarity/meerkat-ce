package store

import (
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/routing"
)

// Endpoint audit (AUD-04): the calls of an operation become audit events sent
// to the collector (AUD-03). The gateway does not store them - their volume is
// the data plane's, not the console's.
//
// What an event carries without being asked: the caller (account, name,
// organisation, active group, roles), the operation, the path as requested,
// the status and the trace id. What it carries on request: Fields, read from
// the call, and the JSON Body.

// The places a field is read from.
const (
	AuditFromPath   = "path"   // a {variable} of the operation's path
	AuditFromQuery  = "query"  // a query parameter
	AuditFromHeader = "header" // a request header
	AuditFromBody   = "body"   // a JSON pointer into the request body (/order/id)
)

// AuditBodyMax is the largest body an event carries, and reads: a call with a
// bigger one is audited without it, and says so.
const AuditBodyMax = 64 << 10

// DefaultAuditMask is what a body never carries in clear when the operation
// names no mask of its own: the field names that hold secrets, matched without
// regard to case, at any depth.
var DefaultAuditMask = []string{"password", "passwd", "secret", "token", "apiKey", "api_key", "authorization", "cookie"}

// EndpointAudit is one audited operation.
type EndpointAudit struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	// Description says what the call means in the trail - pre-filled from the
	// OpenAPI summary, and the body of the event.
	Description string `json:"description,omitempty"`
	// Fields are read from the call and carried as audit.field.<name>.
	Fields []AuditField `json:"fields,omitempty"`
	// Body carries the request body, JSON only, AuditBodyMax at most, with the
	// Mask fields replaced.
	Body bool `json:"body,omitempty"`
	// Mask names the fields a body never carries in clear; empty means
	// DefaultAuditMask.
	Mask []string `json:"mask,omitempty"`
}

// AuditField is one piece of information taken from the call.
type AuditField struct {
	Name string `json:"name"`
	From string `json:"from"`
	Key  string `json:"key"`
}

// ValidateAudit checks a route's audited operations, and names what is wrong.
func ValidateAudit(audits []EndpointAudit) error {
	seen := map[string]bool{}
	for i := range audits {
		a := &audits[i]
		a.Method = strings.ToUpper(strings.TrimSpace(a.Method))
		if a.Method != "*" && !validHTTPMethod(a.Method) {
			return fmt.Errorf("audited operation %d: invalid method %q", i, a.Method)
		}
		if _, err := routing.CompilePath(a.Path); err != nil {
			return fmt.Errorf("audited operation %s: %w", a.Method, err)
		}
		k := a.Method + " " + a.Path
		if seen[k] {
			return fmt.Errorf("the operation %s is audited twice", k)
		}
		seen[k] = true
		names := map[string]bool{}
		for _, f := range a.Fields {
			name := strings.TrimSpace(f.Name)
			if name == "" {
				return fmt.Errorf("%s %s: a field needs a name", a.Method, a.Path)
			}
			if names[name] {
				return fmt.Errorf("%s %s: the field %q is named twice", a.Method, a.Path, name)
			}
			names[name] = true
			switch f.From {
			case AuditFromPath, AuditFromQuery, AuditFromHeader:
				if strings.TrimSpace(f.Key) == "" {
					return fmt.Errorf("%s %s: the field %q says where to read from (%s) but not what", a.Method, a.Path, name, f.From)
				}
			case AuditFromBody:
				if !strings.HasPrefix(f.Key, "/") {
					return fmt.Errorf("%s %s: the field %q reads the body with a JSON pointer, such as /order/id, got %q", a.Method, a.Path, name, f.Key)
				}
			default:
				return fmt.Errorf("%s %s: the field %q reads from %q; allowed: path, query, header, body", a.Method, a.Path, name, f.From)
			}
		}
	}
	return nil
}
