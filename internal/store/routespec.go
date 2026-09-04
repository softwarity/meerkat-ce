package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
)

// A route's OpenAPI specification (SVC-06), and where it comes from.
//
// Two sources, one at a time, and the difference is not "url or file" - it is
// LIVING or FROZEN. An upstream spec is re-read on every screen that needs it,
// so the documentation follows the service. A deposited file is a snapshot: it
// moves when somebody deposits a new one, and never on its own. That is what
// the console has to say out loud, or a per-endpoint policy ends up posed on
// operations the service dropped months ago.
//
// What does NOT change between the two: the spec always has a url ON THE
// ROUTE - served by the upstream, or served by Meerkat from what was
// deposited. So one curl hands the contract to Postman, to a client generator
// and to the built-in swagger alike, and the served docs have one read path
// instead of two.
const (
	// SpecUpstream: Path is where the SERVICE publishes its spec, relative to
	// the upstream (resolved the way the route's own traffic is) or absolute
	// for a third party.
	SpecUpstream = "upstream"
	// SpecFile: the spec was deposited here. Path is the single segment the
	// gateway serves it under, inside the route's own prefix, and Filename is
	// where it came from - the name it takes inside an exported package.
	SpecFile = "file"
)

// RouteSpec names a route's OpenAPI spec. The CONTENT of a deposited file is
// deliberately absent: it lives in the route_specs table, because this object
// is stored in the routes' api column, which every ListRoutes reads in full -
// the console's table, every reload of the router. Megabytes re-read to draw a
// list of names is the one thing this feature must not cost.
type RouteSpec struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Filename string `json:"filename,omitempty"`
}

// SpecKindOpenAPI is the genre of blob route_specs holds today. The key is
// (route, genre) so a route that one day carries a second attachment does not
// need a table of its own.
const SpecKindOpenAPI = "openapi"

// SanitizeRouteSpec normalizes and validates a route's spec declaration.
// A wholly empty one is not an error: it means the route declares no spec, and
// the caller drops it.
func SanitizeRouteSpec(s *RouteSpec) error {
	s.Type = strings.ToLower(strings.TrimSpace(s.Type))
	s.Path = strings.TrimSpace(s.Path)
	s.Filename = strings.TrimSpace(s.Filename)
	switch s.Type {
	case SpecUpstream:
		if s.Path == "" {
			return errors.New("openapi spec: an upstream spec needs the path or url where the service publishes it")
		}
		// A deposited file's name says nothing once the spec is fetched live.
		s.Filename = ""
	case SpecFile:
		if s.Filename == "" {
			return errors.New("openapi spec: a deposited spec needs the name of the file it came from")
		}
		if err := checkBaseName("filename", s.Filename); err != nil {
			return err
		}
		if s.Path == "" {
			s.Path = "openapi.json"
		}
		s.Path = strings.TrimLeft(s.Path, "/")
		if err := checkBaseName("path", s.Path); err != nil {
			return err
		}
		// Served as JSON whatever was deposited (a YAML spec is converted on
		// the way out), so the name says JSON. Normalized rather than refused:
		// the extension is not a decision anyone makes, and the console shows
		// the resulting url.
		if ext := path.Ext(s.Path); ext != ".json" {
			s.Path = strings.TrimSuffix(s.Path, ext) + ".json"
		}
	default:
		return fmt.Errorf("openapi spec type %q: allowed are %s, %s", s.Type, SpecUpstream, SpecFile)
	}
	return nil
}

// checkBaseName refuses anything but one plain file name: a spec served by the
// gateway sits inside the route's prefix, and a path that can climb out of it
// is a path that can answer for another route.
func checkBaseName(field, v string) error {
	if strings.ContainsAny(v, `/\`) || v == "." || v == ".." || strings.Contains(v, "..") {
		return fmt.Errorf("openapi spec %s %q: one plain file name, with no directory and no \"..\"", field, v)
	}
	return nil
}

// Empty reports a declaration that names nothing.
func (s RouteSpec) Empty() bool { return s.Type == "" && s.Path == "" && s.Filename == "" }

// Spec returns the route's spec declaration, or the zero value.
func (r Route) Spec() RouteSpec {
	if r.API == nil || r.API.Spec == nil {
		return RouteSpec{}
	}
	return *r.API.Spec
}

// maxRouteSpecBytes caps a deposited spec. The number comes from the EXPORT,
// not from the parser: this file travels inside a configuration package, and
// what nobody can carry around is not a configuration any more. Real specs are
// far below it - the biggest public ones (Kubernetes, Stripe) sit in single
// megabytes.
const maxRouteSpecBytes = 4 << 20

// SetRouteSpec stores the file deposited for a route, replacing whatever was
// there. The content is kept AS DEPOSITED - the conversion to JSON happens on
// the way out, once, for upstream specs and deposited ones alike.
func (s *Store) SetRouteSpec(ctx context.Context, routeID string, content []byte) error {
	if len(content) == 0 {
		return errors.New("store: route spec: the file is empty")
	}
	if len(content) > maxRouteSpecBytes {
		return fmt.Errorf("store: route spec: %d bytes, the limit is %d", len(content), maxRouteSpecBytes)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO route_specs (route_id, kind, content) VALUES (?, ?, ?)
		 ON CONFLICT(route_id, kind) DO UPDATE SET content = excluded.content`,
		routeID, SpecKindOpenAPI, string(content))
	if err != nil {
		return fmt.Errorf("store: save route spec %q: %w", routeID, err)
	}
	return nil
}

// RouteSpecContent returns the file deposited for a route, or false when there
// is none.
func (s *Store) RouteSpecContent(ctx context.Context, routeID string) ([]byte, bool, error) {
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT content FROM route_specs WHERE route_id = ? AND kind = ?`, routeID, SpecKindOpenAPI).
		Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("store: route spec %q: %w", routeID, err)
	}
	return []byte(content), true, nil
}

// DeleteRouteSpec drops a route's deposited file.
func (s *Store) DeleteRouteSpec(ctx context.Context, routeID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM route_specs WHERE route_id = ? AND kind = ?`, routeID, SpecKindOpenAPI)
	if err != nil {
		return fmt.Errorf("store: delete route spec %q: %w", routeID, err)
	}
	return nil
}

// RouteSpecContents returns every deposited spec, by route id. One query for
// the router's reload and for an export - the two callers that need them all,
// and the reason the content is not in ListRoutes.
func (s *Store) RouteSpecContents(ctx context.Context) (map[string][]byte, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT route_id, content FROM route_specs WHERE kind = ?`, SpecKindOpenAPI)
	if err != nil {
		return nil, fmt.Errorf("store: route specs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string][]byte{}
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			return nil, fmt.Errorf("store: route specs: %w", err)
		}
		out[id] = []byte(content)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: route specs: %w", err)
	}
	return out, nil
}
