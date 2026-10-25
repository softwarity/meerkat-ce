package apidocs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// The three specifications are written by hand, and what keeps them true is a
// test per plane (cmd/meerkat/apidocs_test.go): every pattern a plane mounts
// is documented or excluded with a reason, and every documented operation is
// mounted. The comparison is the one below, so that "GET /api/routes/{id}" on
// the mux and "/api/routes/{routeId}" in the document are the same endpoint.

// Op is one documented operation.
type Op struct {
	// Enterprise says the operation is mounted by Enterprise code only
	// (x-meerkat-edition: enterprise): a community binary is not expected to
	// serve it.
	Enterprise bool
}

var paramRe = regexp.MustCompile(`\{[^}]*\}`)

// Key is how a mounted pattern and a documented operation are compared: the
// method in capitals, a space, the path with every parameter written {}. The
// mux's {$} (this exact path) and {name...} (the rest of it) read as the path
// and as one parameter. A pattern without a method serves them all, and is
// keyed "* /path".
func Key(pattern string) string {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		method, path = "*", pattern
	}
	path = strings.TrimSuffix(path, "{$}")
	return strings.ToUpper(method) + " " + paramRe.ReplaceAllString(path, "{}")
}

// Operations reads a specification's operations, by Key.
func Operations(spec []byte) (map[string]Op, error) {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, fmt.Errorf("apidocs: the specification is not JSON: %w", err)
	}
	out := map[string]Op{}
	for path, ops := range doc.Paths {
		for method, raw := range ops {
			switch method {
			case "get", "put", "post", "delete", "patch", "head", "options":
			default:
				continue // parameters, summary, description: not an operation
			}
			var op struct {
				Edition string `json:"x-meerkat-edition"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("apidocs: %s %s: %w", method, path, err)
			}
			out[Key(strings.ToUpper(method)+" "+path)] = Op{Enterprise: op.Edition == "enterprise"}
		}
	}
	return out, nil
}

// Server is one address a specification's operations are reached at.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// Served rewrites a specification for the page: the version this binary is,
// and the addresses its operations are reached at (none leaves the
// document's own servers).
func Served(spec []byte, version string, servers ...Server) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, fmt.Errorf("apidocs: the specification is not JSON: %w", err)
	}
	if info, ok := doc["info"].(map[string]any); ok {
		info["version"] = version
	}
	if len(servers) > 0 {
		doc["servers"] = servers
	}
	return json.MarshalIndent(doc, "", "  ")
}
