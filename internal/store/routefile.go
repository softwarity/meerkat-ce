package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// Files uploaded on a route (ROUTE-22): what a route in the "files" mode
// serves under its own path - the font an offline UI asks for, a stylesheet,
// a script, an image - when nothing behind the gateway can.
//
// Kept out of the route on purpose, like a deposited spec (route_specs): the
// route is read in full by every listing and every reload, and a file is the
// thing big enough to make listing names cost megabytes. The router reads
// them all once per reload and serves them from memory.

// RouteFile is one file of a route. Data is empty in a listing.
type RouteFile struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
	// SHA is the content hash: the file's ETag, and how an import tells an
	// unchanged file from a new one.
	SHA       string `json:"sha"`
	UpdatedAt int64  `json:"updatedAt"`
	Data      []byte `json:"-"`
}

// The limits come from the configuration export, which carries the files: a
// route's files travel inside a package, and what nobody can carry around is
// not configuration any more.
const (
	MaxRouteFileBytes  = 10 << 20
	MaxRouteFilesBytes = 32 << 20
)

// CheckRouteFileName accepts a relative path of plain segments - a stylesheet
// may well reference fonts/x.woff2 - and nothing that climbs out of the route.
func CheckRouteFileName(name string) (string, error) {
	n := strings.TrimLeft(strings.TrimSpace(name), "/")
	if n == "" {
		return "", errors.New("route file: a name is required")
	}
	if len(n) > 200 || strings.ContainsAny(n, "\\?#\x00") {
		return "", fmt.Errorf("route file %q: a relative path of plain segments, without \\ ? or #", name)
	}
	for _, seg := range strings.Split(n, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("route file %q: a relative path of plain segments, without empty, . or .. segments", name)
		}
	}
	return n, nil
}

// RouteFileType is the Content-Type a file is served with: from its
// extension, else from its bytes.
func RouteFileType(name string, data []byte) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".map", ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	}
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t
	}
	return http.DetectContentType(data)
}

// SetRouteFile stores a file on a route, replacing one of the same name.
func (s *Store) SetRouteFile(ctx context.Context, routeID, name string, data []byte) (RouteFile, error) {
	n, err := CheckRouteFileName(name)
	if err != nil {
		return RouteFile{}, err
	}
	if len(data) == 0 {
		return RouteFile{}, fmt.Errorf("route file %q: the file is empty", n)
	}
	if len(data) > MaxRouteFileBytes {
		return RouteFile{}, fmt.Errorf("route file %q: %d bytes, the limit is %d", n, len(data), MaxRouteFileBytes)
	}
	var others int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0) FROM route_files WHERE route_id = ? AND name <> ?`, routeID, n).Scan(&others); err != nil {
		return RouteFile{}, fmt.Errorf("store: route files %q: %w", routeID, err)
	}
	if others+len(data) > MaxRouteFilesBytes {
		return RouteFile{}, fmt.Errorf("route file %q: the route's files would weigh %d bytes, the limit is %d", n, others+len(data), MaxRouteFilesBytes)
	}
	sum := sha256.Sum256(data)
	f := RouteFile{Name: n, ContentType: RouteFileType(n, data), Size: len(data),
		SHA: hex.EncodeToString(sum[:]), UpdatedAt: time.Now().Unix(), Data: data}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO route_files (route_id, name, content_type, size, sha, content, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(route_id, name) DO UPDATE SET content_type = excluded.content_type,
		   size = excluded.size, sha = excluded.sha, content = excluded.content, updated_at = excluded.updated_at`,
		routeID, f.Name, f.ContentType, f.Size, f.SHA, base64.StdEncoding.EncodeToString(data), f.UpdatedAt)
	if err != nil {
		return RouteFile{}, fmt.Errorf("store: save route file %q: %w", n, err)
	}
	return f, nil
}

// ErrNoRouteFile says the route has no file of that name.
var ErrNoRouteFile = errors.New("no such file on this route")

// UpdateRouteFile renames a file and sets the type it is served with - what
// the upload named it is the uploader's file name, which is rarely what a page
// should ask for, and a type read from the bytes is a guess the operator may
// have to correct. Empty keeps the current value. The content is untouched,
// so the ETag is too.
func (s *Store) UpdateRouteFile(ctx context.Context, routeID, name, newName, contentType string) (RouteFile, error) {
	f, ok, err := s.RouteFileContent(ctx, routeID, name)
	if err != nil {
		return RouteFile{}, err
	}
	if !ok {
		return RouteFile{}, fmt.Errorf("route file %q: %w", name, ErrNoRouteFile)
	}
	to := f.Name
	if strings.TrimSpace(newName) != "" {
		if to, err = CheckRouteFileName(newName); err != nil {
			return RouteFile{}, invalidf(err)
		}
	}
	ct := f.ContentType
	if t := strings.TrimSpace(contentType); t != "" {
		if _, _, err := mime.ParseMediaType(t); err != nil || !strings.Contains(t, "/") {
			return RouteFile{}, invalidf(fmt.Errorf("route file %q: %q is not a media type - type/subtype, like font/woff2 or application/pdf", f.Name, t))
		}
		ct = t
	}
	if to != f.Name {
		var taken int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM route_files WHERE route_id = ? AND name = ?`, routeID, to).Scan(&taken); err != nil {
			return RouteFile{}, fmt.Errorf("store: route files %q: %w", routeID, err)
		}
		if taken > 0 {
			return RouteFile{}, invalidf(fmt.Errorf("route file %q: this route already has a file named %q - remove it first", f.Name, to))
		}
	}
	f.UpdatedAt = time.Now().Unix()
	if _, err := s.db.ExecContext(ctx,
		`UPDATE route_files SET name = ?, content_type = ?, updated_at = ? WHERE route_id = ? AND name = ?`,
		to, ct, f.UpdatedAt, routeID, f.Name); err != nil {
		return RouteFile{}, fmt.Errorf("store: update route file %q: %w", f.Name, err)
	}
	f.Name, f.ContentType, f.Data = to, ct, nil
	return f, nil
}

// PruneRouteFiles drops the files of a route that keep does not name, and
// says which.
func (s *Store) PruneRouteFiles(ctx context.Context, routeID string, keep map[string]bool) ([]string, error) {
	files, err := s.ListRouteFiles(ctx, routeID)
	if err != nil {
		return nil, err
	}
	var gone []string
	for _, f := range files {
		if keep[f.Name] {
			continue
		}
		if _, err := s.DeleteRouteFile(ctx, routeID, f.Name); err != nil {
			return gone, err
		}
		gone = append(gone, f.Name)
	}
	return gone, nil
}

// DeleteRouteFile drops one file; false when there was none.
func (s *Store) DeleteRouteFile(ctx context.Context, routeID, name string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM route_files WHERE route_id = ? AND name = ?`, routeID, name)
	if err != nil {
		return false, fmt.Errorf("store: delete route file %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListRouteFiles is a route's files without their content, by name.
func (s *Store) ListRouteFiles(ctx context.Context, routeID string) ([]RouteFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, content_type, size, sha, updated_at FROM route_files WHERE route_id = ? ORDER BY name`, routeID)
	if err != nil {
		return nil, fmt.Errorf("store: route files %q: %w", routeID, err)
	}
	defer func() { _ = rows.Close() }()
	out := []RouteFile{}
	for rows.Next() {
		var f RouteFile
		if err := rows.Scan(&f.Name, &f.ContentType, &f.Size, &f.SHA, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: route files %q: %w", routeID, err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RouteFileContents is every route's files WITH their content, by route id:
// one query for the router's reload and for an export, the two callers that
// need them all.
func (s *Store) RouteFileContents(ctx context.Context) (map[string][]RouteFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT route_id, name, content_type, size, sha, content, updated_at FROM route_files ORDER BY route_id, name`)
	if err != nil {
		return nil, fmt.Errorf("store: route files: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string][]RouteFile{}
	for rows.Next() {
		var id, content string
		var f RouteFile
		if err := rows.Scan(&id, &f.Name, &f.ContentType, &f.Size, &f.SHA, &content, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: route files: %w", err)
		}
		if f.Data, err = base64.StdEncoding.DecodeString(content); err != nil {
			return nil, fmt.Errorf("store: route file %q of %q: %w", f.Name, id, err)
		}
		out[id] = append(out[id], f)
	}
	return out, rows.Err()
}

// RouteFileContent is one file with its content.
func (s *Store) RouteFileContent(ctx context.Context, routeID, name string) (RouteFile, bool, error) {
	var f RouteFile
	var content string
	err := s.db.QueryRowContext(ctx,
		`SELECT name, content_type, size, sha, content, updated_at FROM route_files WHERE route_id = ? AND name = ?`,
		routeID, name).Scan(&f.Name, &f.ContentType, &f.Size, &f.SHA, &content, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RouteFile{}, false, nil
	}
	if err != nil {
		return RouteFile{}, false, fmt.Errorf("store: route file %q: %w", name, err)
	}
	if f.Data, err = base64.StdEncoding.DecodeString(content); err != nil {
		return RouteFile{}, false, fmt.Errorf("store: route file %q: %w", name, err)
	}
	return f, true, nil
}
