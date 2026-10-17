// Package openapi reads an OpenAPI / Swagger specification and projects it down
// to the minimal shape Meerkat needs: the list of operations (method + path)
// that endpoint-level security (RBAC-07) binds to, and the served swagger-ui
// hangs off. Swagger 2.0 and OpenAPI 3.0/3.1 are both understood through
// libopenapi; callers see one version-agnostic Operation slice and never touch
// OpenAPI's complexity (schemas, $refs, parameters are deliberately dropped).
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v4"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// maxSpecBytes caps a fetched spec: enough for large real-world documents,
// small enough that a hostile or broken upstream cannot exhaust memory.
const maxSpecBytes = 12 << 20 // 12 MiB

// Operation is one method+path pair exposed by a spec, reduced to what the
// console shows and what a per-endpoint policy binds to.
type Operation struct {
	Method      string   `json:"method"` // upper-case HTTP verb
	Path        string   `json:"path"`   // spec path, e.g. /users/{id}
	OperationID string   `json:"operationId,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	// Upstream is what the DOCUMENT says the application itself requires to
	// call this operation - absent when it says nothing. Read-only knowledge:
	// the application enforces it, Meerkat does not, and a rule Meerkat poses
	// is still written apart, on the route.
	Upstream *Upstream `json:"upstream,omitempty"`
}

// The three things a document can say about an operation's security.
const (
	UpstreamPublic        = "public"        // explicitly open: `security: []`, or an empty `{}` alternative
	UpstreamAuthenticated = "authenticated" // a scheme with no role: any valid credential
	UpstreamRoles         = "roles"         // a scheme with roles: one of them is required
)

// Upstream is the security a document declares for one operation. The roles
// are the SCOPES of its requirements - the standard place, which libraries such
// as nestjs-granted or springdoc fill from the code's own role checks, whatever
// the scheme (bearer, oauth2, openIdConnect).
type Upstream struct {
	Access string   `json:"access"`
	Roles  []string `json:"roles,omitempty"`
}

// Spec is the parsed projection of an OpenAPI / Swagger document: API metadata
// plus the flat, sorted list of operations.
type Spec struct {
	Title      string      `json:"title,omitempty"`
	Version    string      `json:"version,omitempty"` // the API version (info.version)
	Format     string      `json:"format"`            // spec version, e.g. "2.0", "3.0.3", "3.1.0"
	Operations []Operation `json:"operations"`
	// Base is the path the document says it lives under - basePath in 2.0, the
	// path of the first server in 3.x - and it is part of every operation's
	// ADDRESS: a client calls base + path, never path alone. Operation.Path
	// carries it already, and this field is the piece on its own, for a reader
	// who has to compare with something else.
	Base string `json:"base,omitempty"`
	// Roles is every role the document declares, over all its operations,
	// sorted: what an application asks of the people calling it, in its words.
	Roles []string `json:"roles,omitempty"`
}

// cleanBase normalises what a document says about where it lives: "/api/v3",
// "api/v3/" and "/" all mean the same thing, and "" means the root. What comes
// back is either empty or a path with a leading slash and no trailing one, so
// joining it to an operation path needs no second thought.
func cleanBase(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// Parse reads a raw spec (JSON or YAML) and returns its operation projection.
// The spec version is auto-detected: "2.x" builds the Swagger model, "3.x" the
// OpenAPI one.
func Parse(data []byte) (*Spec, error) {
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return nil, fmt.Errorf("openapi: not a readable spec: %w", err)
	}
	v := strings.TrimSpace(doc.GetVersion())
	switch {
	case strings.HasPrefix(v, "2"):
		return parseV2(doc, data)
	case strings.HasPrefix(v, "3"):
		return parseV3(doc)
	default:
		return nil, fmt.Errorf("openapi: unsupported spec version %q (want swagger 2.x or openapi 3.x)", v)
	}
}

func parseV2(doc libopenapi.Document, data []byte) (*Spec, error) {
	m, err := doc.BuildV2Model()
	if err != nil {
		return nil, fmt.Errorf("openapi: build swagger 2.0 model: %w", err)
	}
	sw := &m.Model
	// Swagger 2.0 splits the address in two: host and basePath. With no host,
	// the document describes an API answering where it was fetched from, so the
	// basePath is part of the public address - the same reading as a relative
	// server in 3.x. With a host, the pair is the service's own address and the
	// route's upstream carries it.
	out := &Spec{Format: orDefault(sw.Swagger, "2.0")}
	if strings.TrimSpace(sw.Host) == "" {
		out.Base = cleanBase(sw.BasePath)
	}
	if sw.Info != nil {
		out.Title, out.Version = sw.Info.Title, sw.Info.Version
	}
	opened := openedV2(data)
	if sw.Paths != nil {
		for pp := orderedmap.First(sw.Paths.PathItems); pp != nil; pp = pp.Next() {
			path := pp.Key()
			for op := orderedmap.First(pp.Value().GetOperations()); op != nil; op = op.Next() {
				o := op.Value()
				// Both of libopenapi's 2.0 models drop an operation's
				// `security: []` - precisely the one that says "open" - so it
				// is read from the document itself.
				reqs, explicit := o.Security, o.Security != nil || opened[strings.ToLower(op.Key())+" "+path]
				if !explicit {
					reqs, explicit = sw.Security, sw.Security != nil
				}
				out.Operations = append(out.Operations, Operation{
					Method: strings.ToUpper(op.Key()), Path: out.Base + path,
					OperationID: o.OperationId, Summary: o.Summary, Tags: o.Tags,
					Upstream: upstreamOf(reqs, explicit),
				})
			}
		}
	}
	sortOps(out.Operations)
	out.Roles = declaredRoles(out.Operations)
	return out, nil
}

func parseV3(doc libopenapi.Document) (*Spec, error) {
	m, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("openapi: build openapi 3.x model: %w", err)
	}
	d := &m.Model
	out := &Spec{Format: orDefault(d.Version, "3.0")}
	// The first server's PATH, when the server is RELATIVE ("/api/v3",
	// "/otel-demo"): the document then says its operations answer under that
	// prefix ON THE HOST THAT SERVED IT, so the prefix is part of the address a
	// client calls and part of the coordinate a rule names.
	//
	// An ABSOLUTE server (https://api.example.com/v1) says something else: the
	// path belongs to the address of the service itself, which is what the
	// route's upstream already carries. Prefixing operations with it would name
	// coordinates no request ever has.
	if len(d.Servers) > 0 && d.Servers[0] != nil {
		if u, err := url.Parse(d.Servers[0].URL); err == nil && u.Host == "" && u.Scheme == "" {
			out.Base = cleanBase(u.Path)
		}
	}
	if d.Info != nil {
		out.Title, out.Version = d.Info.Title, d.Info.Version
	}
	if d.Paths != nil {
		for pp := orderedmap.First(d.Paths.PathItems); pp != nil; pp = pp.Next() {
			path := pp.Key()
			for op := orderedmap.First(pp.Value().GetOperations()); op != nil; op = op.Next() {
				o := op.Value()
				// nil = the operation says nothing and the document's own
				// requirement applies; an EMPTY list is its own statement.
				reqs, explicit := o.Security, o.Security != nil
				if !explicit {
					reqs, explicit = d.Security, d.Security != nil
				}
				out.Operations = append(out.Operations, Operation{
					Method: strings.ToUpper(op.Key()), Path: out.Base + path,
					OperationID: o.OperationId, Summary: o.Summary, Tags: o.Tags,
					Upstream: upstreamOf(reqs, explicit),
				})
			}
		}
	}
	sortOps(out.Operations)
	out.Roles = declaredRoles(out.Operations)
	return out, nil
}

// Fetch retrieves a spec over HTTP server-side and parses it. The caller
// supplies the client so timeouts and transport stay under the gateway's
// control. The raw bytes are returned alongside the projection: the served
// swagger-ui needs them (after Rewrite), the console only needs the Spec.
func Fetch(ctx context.Context, client *http.Client, url string) (*Spec, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("openapi: build request for %q: %w", url, err)
	}
	req.Header.Set("Accept", "application/json, application/yaml;q=0.9, text/yaml;q=0.9, */*;q=0.1")
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("openapi: fetch %q: %w", url, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("openapi: fetch %q: upstream returned %s", url, res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxSpecBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("openapi: read %q: %w", url, err)
	}
	spec, err := Parse(body)
	if err != nil {
		return nil, nil, err
	}
	// What comes back is JSON, always: the callers that edit it cannot do so
	// in YAML, and the ones that only read it do not care which it was.
	body, err = Normalize(body)
	if err != nil {
		return nil, nil, err
	}
	return spec, body, nil
}

// Normalize returns a spec as JSON, whatever it was written in.
//
// The specification admits exactly two representations, JSON and YAML, and
// they describe the same object - so this loses nothing a reader needs. What
// it BUYS is that everything downstream works: Rewrite and InjectSimulation
// only know how to edit JSON, and a YAML spec used to travel through them
// untouched, silently costing an upstream that publishes YAML both its
// retargeting to the gateway and its simulation Authorize.
//
// One conversion, one place, for a spec fetched from an upstream and for a
// deposited file alike. It does not go through libopenapi: its JSON renderer
// exists for OpenAPI 3 only and refuses Swagger 2.0, while YAML decoded into
// any re-encodes as JSON for both - including the response codes YAML reads as
// integers, which come back out as the string keys the format requires.
func Normalize(raw []byte) ([]byte, error) {
	if trimmed := strings.TrimLeft(string(raw), " \t\r\n"); strings.HasPrefix(trimmed, "{") {
		return raw, nil
	}
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("openapi: this is neither JSON nor YAML: %w", err)
	}
	out, err := json.MarshalIndent(stringKeys(doc), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("openapi: converting the YAML spec to JSON: %w", err)
	}
	return out, nil
}

// stringKeys makes the mapping keys strings, which is what JSON has and YAML
// does not: `200:` in a responses block is an INTEGER key, and json.Marshal
// refuses a map that is not keyed by a string.
//
// Doing it here rather than trusting the decoder, because the decoder is not
// something to trust with this. The same version of the same YAML library
// returned map[string]any under one Go release and map[any]any under the one
// before it, and the difference only showed up on a build machine: every test
// was green on a workstation a minor version ahead. Whether a customer's
// specification loads must not depend on which toolchain compiled the gateway.
func stringKeys(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = stringKeys(val)
		}
		return t
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = stringKeys(val)
		}
		return out
	case []any:
		for i, val := range t {
			t[i] = stringKeys(val)
		}
		return t
	default:
		return v
	}
}

// Rewrite adjusts a raw spec so a swagger-ui served BEHIND the gateway calls
// operations through the exposed base (UIF-07). A relative base keeps the
// current origin: Swagger 2.0 gets its basePath set (host and schemes dropped);
// OpenAPI 3.x gets a single relative server. An absolute base
// (scheme://host[/path]) points at ANOTHER origin - the API-docs page serves
// from the admin plane while the routes answer on the data plane - so 2.0 gets
// it decomposed into host/schemes/basePath and 3.x carries it verbatim. The
// spec is treated as JSON; a YAML-only spec is returned unchanged (swagger-ui
// still loads it, only gateway targeting is lost) with a non-nil error the
// caller may log.
func Rewrite(raw []byte, exposedBase string) ([]byte, error) {
	if exposedBase == "" {
		exposedBase = "/"
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw, fmt.Errorf("openapi: rewrite expects JSON: %w", err)
	}
	if _, isV2 := doc["swagger"]; isV2 {
		// The spec's own base path survives: the route forwards paths, so the
		// public path is <route prefix> + <spec base> (httpbin: "/", petstore:
		// "/api/v3").
		ownBase, _ := doc["basePath"].(string)
		delete(doc, "host")
		delete(doc, "schemes")
		if u, err := url.Parse(exposedBase); err == nil && u.Scheme != "" && u.Host != "" {
			doc["host"] = u.Host
			doc["schemes"] = []any{u.Scheme}
			doc["basePath"] = joinBasePaths(u.Path, ownBase)
		} else {
			doc["basePath"] = joinBasePaths(exposedBase, ownBase)
		}
	} else {
		// Same idea for 3.x: keep the PATH of the spec's first server (its
		// url is often relative, e.g. "/api/v3") under the exposed base.
		var ownPath string
		if servers, _ := doc["servers"].([]any); len(servers) > 0 {
			if first, _ := servers[0].(map[string]any); first != nil {
				if s, _ := first["url"].(string); s != "" {
					if u, err := url.Parse(s); err == nil {
						ownPath = u.Path
					}
				}
			}
		}
		base := strings.TrimSuffix(exposedBase, "/")
		if p := strings.Trim(ownPath, "/"); p != "" {
			base += "/" + p
		}
		if base == "" {
			base = "/"
		}
		doc["servers"] = []any{map[string]any{"url": base}}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return raw, fmt.Errorf("openapi: rewrite marshal: %w", err)
	}
	return out, nil
}

// AnnotateDeclared writes, at the head of each operation's description, what
// the APPLICATION requires to call it: swagger-ui shows a padlock for a bearer
// requirement but never its roles, and the roles are what a developer trying
// the operation needs to know. roleMap adds the catalogue role that grants
// each one, which is the name the page's simulation speaks. Serve-time only; a
// YAML spec comes back unchanged with a non-nil error.
func AnnotateDeclared(raw []byte, roleMap map[string]string) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw, fmt.Errorf("openapi: annotate expects JSON: %w", err)
	}
	global, hasGlobal := doc["security"].([]any)
	paths, _ := doc["paths"].(map[string]any)
	changed := false
	for _, item := range paths {
		ops, _ := item.(map[string]any)
		for method, v := range ops {
			op, _ := v.(map[string]any)
			if op == nil || !httpMethods[strings.ToLower(method)] {
				continue
			}
			reqs, explicit := op["security"].([]any)
			if !explicit {
				reqs, explicit = global, hasGlobal
			}
			line := declaredLine(upstreamOfJSON(reqs, explicit), roleMap)
			if line == "" {
				continue
			}
			if d, _ := op["description"].(string); d != "" {
				line += "\n\n" + d
			}
			op["description"] = line
			changed = true
		}
	}
	if !changed {
		return raw, nil
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return raw, fmt.Errorf("openapi: annotate marshal: %w", err)
	}
	return out, nil
}

var httpMethods = map[string]bool{"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true}

// upstreamOfJSON is upstreamOf on a document read as plain JSON.
func upstreamOfJSON(reqs []any, explicit bool) *Upstream {
	conv := make([]*base.SecurityRequirement, 0, len(reqs))
	for _, r := range reqs {
		m, _ := r.(map[string]any)
		req := &base.SecurityRequirement{Requirements: orderedmap.New[string, []string]()}
		for scheme, scopes := range m {
			list, _ := scopes.([]any)
			names := make([]string, 0, len(list))
			for _, sc := range list {
				if name, ok := sc.(string); ok {
					names = append(names, name)
				}
			}
			req.Requirements.Set(scheme, names)
		}
		conv = append(conv, req)
	}
	return upstreamOf(conv, explicit)
}

// declaredLine says it in one line of Markdown, or nothing for an operation
// the document says nothing about or leaves open.
func declaredLine(up *Upstream, roleMap map[string]string) string {
	if up == nil {
		return ""
	}
	switch up.Access {
	case UpstreamAuthenticated:
		return "**Required by the application:** a signed-in caller."
	case UpstreamRoles:
		parts := make([]string, 0, len(up.Roles))
		for _, r := range up.Roles {
			part := "`" + r + "`"
			if m := roleMap[r]; m != "" {
				part += " (Meerkat role `" + m + "`)"
			}
			parts = append(parts, part)
		}
		if len(parts) == 1 {
			return "**Required by the application:** " + parts[0] + "."
		}
		return "**Required by the application:** one of " + strings.Join(parts, ", ") + "."
	}
	return ""
}

// InjectSimulation declares the gateway's identity-simulation headers on a
// spec served by the API-docs page: swagger-ui's Authorize can then input a
// user and roles (2.0 gets securityDefinitions, 3.x components.securitySchemes)
// and every operation shows the padlock - the appended global requirement is
// OR-ed with whatever the spec already demands. Serve-time only, the stored
// spec is untouched. A YAML spec is returned unchanged with a non-nil error.
func InjectSimulation(raw []byte) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw, fmt.Errorf("openapi: inject expects JSON: %w", err)
	}
	schemes := map[string]any{
		"MeerkatSimulateUser": map[string]any{
			"type": "apiKey", "in": "header", "name": "X-Meerkat-Simulate-User",
			"description": "Try the route AS this username - no account needed. Honored only for signed-in gateway testers (root, infra-admin, dev).",
		},
		"MeerkatSimulateRoles": map[string]any{
			"type": "apiKey", "in": "header", "name": "X-Meerkat-Simulate-Roles",
			"description": "Comma-separated roles of the simulated identity (e.g. auditor,sales).",
		},
		"MeerkatTestToken": map[string]any{
			"type": "apiKey", "in": "header", "name": "Authorization",
			"description": "An ephemeral test token minted in the page header, prefixed: `Bearer mksim_...`. Carries its own identity and roles - no session needed.",
		},
	}
	if _, isV2 := doc["swagger"]; isV2 {
		defs, _ := doc["securityDefinitions"].(map[string]any)
		if defs == nil {
			defs = map[string]any{}
		}
		for k, v := range schemes {
			defs[k] = v
		}
		doc["securityDefinitions"] = defs
	} else {
		comps, _ := doc["components"].(map[string]any)
		if comps == nil {
			comps = map[string]any{}
		}
		ss, _ := comps["securitySchemes"].(map[string]any)
		if ss == nil {
			ss = map[string]any{}
		}
		for k, v := range schemes {
			ss[k] = v
		}
		comps["securitySchemes"] = ss
		doc["components"] = comps
	}
	sec, _ := doc["security"].([]any)
	doc["security"] = append(sec,
		map[string]any{"MeerkatSimulateUser": []any{}, "MeerkatSimulateRoles": []any{}},
		map[string]any{"MeerkatTestToken": []any{}})
	out, err := json.Marshal(doc)
	if err != nil {
		return raw, fmt.Errorf("openapi: inject marshal: %w", err)
	}
	return out, nil
}

// joinBasePaths joins the route's exposed path and the spec's own base path
// into one clean base ("" and "/" collapse; the result is never empty).
func joinBasePaths(exposed, own string) string {
	e := strings.TrimSuffix(exposed, "/")
	if o := strings.Trim(own, "/"); o != "" {
		e += "/" + o
	}
	if e == "" {
		return "/"
	}
	return e
}

// methodRank orders operations by the conventional REST verb sequence for a
// stable, readable listing.
func methodRank(m string) int {
	switch m {
	case "GET":
		return 0
	case "POST":
		return 1
	case "PUT":
		return 2
	case "PATCH":
		return 3
	case "DELETE":
		return 4
	case "HEAD":
		return 5
	case "OPTIONS":
		return 6
	default:
		return 7
	}
}

// sortOps gives a deterministic order: by path, then by verb rank. The console
// regroups by tag on top; tests rely on the determinism.
// openedV2 names the operations of a 2.0 document that set `security: []`, as
// "method path". A document this cannot read opens nothing.
func openedV2(data []byte) map[string]bool {
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return nil
	}
	out := map[string]bool{}
	for path, item := range doc.Paths {
		for method, op := range item {
			o, _ := op.(map[string]any)
			if sec, ok := o["security"].([]any); ok && len(sec) == 0 {
				out[strings.ToLower(method)+" "+path] = true
			}
		}
	}
	return out
}

// upstreamOf reads the requirements that apply to one operation. They are
// ALTERNATIVES (any one satisfied lets the call in), so an empty alternative
// makes the whole operation open, and the roles are the union of what the
// alternatives name. explicit separates "said nothing" from "said []".
func upstreamOf(reqs []*base.SecurityRequirement, explicit bool) *Upstream {
	if !explicit {
		return nil
	}
	if len(reqs) == 0 {
		return &Upstream{Access: UpstreamPublic}
	}
	seen := map[string]bool{}
	var roles []string
	for _, req := range reqs {
		if req == nil || req.ContainsEmptyRequirement || req.Requirements == nil || req.Requirements.Len() == 0 {
			return &Upstream{Access: UpstreamPublic}
		}
		for r := orderedmap.First(req.Requirements); r != nil; r = r.Next() {
			for _, role := range r.Value() {
				if role = strings.TrimSpace(role); role != "" && !seen[role] {
					seen[role] = true
					roles = append(roles, role)
				}
			}
		}
	}
	if len(roles) == 0 {
		return &Upstream{Access: UpstreamAuthenticated}
	}
	sort.Strings(roles)
	return &Upstream{Access: UpstreamRoles, Roles: roles}
}

// declaredRoles is every role the operations name, once each, sorted.
func declaredRoles(ops []Operation) []string {
	seen := map[string]bool{}
	var out []string
	for _, op := range ops {
		if op.Upstream == nil {
			continue
		}
		for _, r := range op.Upstream.Roles {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	sort.Strings(out)
	return out
}

func sortOps(ops []Operation) {
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return methodRank(ops[i].Method) < methodRank(ops[j].Method)
	})
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
