package openapi

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The shape nestjs-granted and springdoc write: a global bearer requirement,
// and on the guarded operations the same scheme with the roles as scopes.
const declaredV3 = `{
  "openapi": "3.0.0",
  "info": {"title": "demo", "version": "1"},
  "security": [{"bearer": []}],
  "components": {"securitySchemes": {"bearer": {"type": "http", "scheme": "bearer"}}},
  "paths": {
    "/health": {"get": {"security": [], "responses": {"200": {"description": "ok"}}}},
    "/whoami": {"get": {"responses": {"200": {"description": "ok"}}}},
    "/admin": {"get": {"security": [{"bearer": ["ROLE_ADMIN"]}], "responses": {"200": {"description": "ok"}}}},
    "/either": {"get": {"security": [{"bearer": ["ROLE_USER"]}, {"bearer": ["ROLE_ADMIN"]}], "responses": {"200": {"description": "ok"}}}},
    "/optional": {"get": {"security": [{}, {"bearer": ["ROLE_USER"]}], "responses": {"200": {"description": "ok"}}}}
  }
}`

func TestDeclaredSecurityV3(t *testing.T) {
	spec, err := Parse([]byte(declaredV3))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]*Upstream{
		"/health":   {Access: UpstreamPublic},
		"/whoami":   {Access: UpstreamAuthenticated},
		"/admin":    {Access: UpstreamRoles, Roles: []string{"ROLE_ADMIN"}},
		"/either":   {Access: UpstreamRoles, Roles: []string{"ROLE_ADMIN", "ROLE_USER"}},
		"/optional": {Access: UpstreamPublic},
	}
	for _, op := range spec.Operations {
		if !reflect.DeepEqual(op.Upstream, want[op.Path]) {
			t.Errorf("%s %s: upstream %+v, want %+v", op.Method, op.Path, op.Upstream, want[op.Path])
		}
	}
	if !reflect.DeepEqual(spec.Roles, []string{"ROLE_ADMIN", "ROLE_USER"}) {
		t.Errorf("declared roles %v, want ROLE_ADMIN and ROLE_USER", spec.Roles)
	}
}

// A document that says nothing about security says nothing: no mark, no role.
func TestNoSecurityDeclaresNothing(t *testing.T) {
	spec, err := Parse([]byte(`{"openapi":"3.0.0","info":{"title":"t","version":"1"},
	  "paths":{"/a":{"get":{"responses":{"200":{"description":"ok"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Operations[0].Upstream != nil || spec.Roles != nil {
		t.Errorf("upstream %+v roles %v, want neither", spec.Operations[0].Upstream, spec.Roles)
	}
}

// Swagger 2.0: the high model forgets an operation's `security: []`, which is
// the one statement that opens it under a global requirement.
func TestDeclaredSecurityV2(t *testing.T) {
	spec, err := Parse([]byte(`{
	  "swagger": "2.0", "info": {"title": "t", "version": "1"},
	  "securityDefinitions": {"oauth": {"type": "oauth2", "flow": "implicit", "authorizationUrl": "https://x/auth", "scopes": {"admin": "a"}}},
	  "security": [{"oauth": []}],
	  "paths": {
	    "/open": {"get": {"security": [], "responses": {"200": {"description": "ok"}}}},
	    "/any": {"get": {"responses": {"200": {"description": "ok"}}}},
	    "/admin": {"get": {"security": [{"oauth": ["admin"]}], "responses": {"200": {"description": "ok"}}}}
	  }}`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]*Upstream{
		"/open":  {Access: UpstreamPublic},
		"/any":   {Access: UpstreamAuthenticated},
		"/admin": {Access: UpstreamRoles, Roles: []string{"admin"}},
	}
	for _, op := range spec.Operations {
		if !reflect.DeepEqual(op.Upstream, want[op.Path]) {
			t.Errorf("%s %s: upstream %+v, want %+v", op.Method, op.Path, op.Upstream, want[op.Path])
		}
	}
}

// The developer's swagger: the roles land in the description, with the
// catalogue role that grants each one when the route maps it.
func TestAnnotateDeclared(t *testing.T) {
	out, err := AnnotateDeclared([]byte(declaredV3), map[string]string{"ROLE_ADMIN": "demo-ROLE_ADMIN"})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	desc := func(path string) string {
		op := doc["paths"].(map[string]any)[path].(map[string]any)["get"].(map[string]any)
		d, _ := op["description"].(string)
		return d
	}
	for path, want := range map[string]string{
		"/admin":    "**Required by the application:** `ROLE_ADMIN` (Meerkat role `demo-ROLE_ADMIN`).",
		"/either":   "**Required by the application:** one of `ROLE_ADMIN` (Meerkat role `demo-ROLE_ADMIN`), `ROLE_USER`.",
		"/whoami":   "**Required by the application:** a signed-in caller.",
		"/health":   "",
		"/optional": "",
	} {
		if got := desc(path); got != want {
			t.Errorf("%s: description %q, want %q", path, got, want)
		}
	}
}
