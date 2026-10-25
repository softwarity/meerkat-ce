package admin

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/admin/apidocs"
)

// What the reference says an answer is, the answer is (DOCS-01).
//
// cmd/meerkat/apidocs_test.go holds the documents to the endpoints a plane
// mounts; this holds the control plane's document to what its reads ANSWER.
// Every read without a parameter is called on a fresh gateway, as root, and
// its JSON checked against the schema the document gives its 200: a field
// the answer carries and the schema does not declare, a declared type the
// value does not have, a required field missing. A fresh gateway answers its
// lists empty, so what this proves is the shape of every object it returns,
// which is where a document drifts first - a field added to a Go type.

// specSchemas is the admin document's schemas and its reads' answer schemas.
type specDoc struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]schema `json:"schemas"`
	} `json:"components"`
}

type schema struct {
	Ref                  string            `json:"$ref"`
	Type                 string            `json:"type"`
	Nullable             bool              `json:"nullable"`
	Properties           map[string]schema `json:"properties"`
	Required             []string          `json:"required"`
	AdditionalProperties json.RawMessage   `json:"additionalProperties"`
	Items                *schema           `json:"items"`
	Enum                 []any             `json:"enum"`
	OneOf                []schema          `json:"oneOf"`
	AnyOf                []schema          `json:"anyOf"`
	AllOf                []schema          `json:"allOf"`
}

type checker struct {
	schemas map[string]schema
	errs    []string
}

func (c *checker) resolve(s schema) schema {
	for i := 0; s.Ref != "" && i < 20; i++ {
		s = c.schemas[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
	}
	return s
}

// flatten is one schema out of an allOf: the properties and the required
// fields of every branch, so that a field one branch declares is not
// "undocumented" for the others.
func (c *checker) flatten(s schema) schema {
	out := schema{Type: s.Type, Nullable: s.Nullable, Properties: map[string]schema{}, Required: s.Required,
		AdditionalProperties: s.AdditionalProperties}
	for k, p := range s.Properties {
		out.Properties[k] = p
	}
	for _, sub := range s.AllOf {
		b := c.resolve(sub)
		if len(b.AllOf) > 0 {
			b = c.flatten(b)
		}
		if out.Type == "" {
			out.Type = b.Type
		}
		out.Nullable = out.Nullable || b.Nullable
		out.Required = append(out.Required, b.Required...)
		for k, p := range b.Properties {
			out.Properties[k] = p
		}
		if len(b.AdditionalProperties) > 0 {
			out.AdditionalProperties = b.AdditionalProperties
		}
	}
	return out
}

// matches reports whether v fits s, without recording anything: what oneOf
// and anyOf need to try each branch.
func (c *checker) matches(s schema, v any) bool {
	sub := &checker{schemas: c.schemas}
	sub.check(s, v, "")
	return len(sub.errs) == 0
}

func (c *checker) fail(at, format string, args ...any) {
	c.errs = append(c.errs, at+": "+fmt.Sprintf(format, args...))
}

func (c *checker) check(s schema, v any, at string) {
	s = c.resolve(s)
	if v == nil {
		if !s.Nullable && s.Type != "" {
			c.fail(at, "null, and the schema says %s without nullable", s.Type)
		}
		return
	}
	if len(s.AllOf) > 0 {
		c.check(c.flatten(s), v, at)
		return
	}
	if len(s.OneOf) > 0 || len(s.AnyOf) > 0 {
		if !slices.ContainsFunc(append(s.OneOf, s.AnyOf...), func(b schema) bool { return c.matches(b, v) }) {
			c.fail(at, "fits none of the alternatives")
		}
		return
	}
	if len(s.Enum) > 0 && !slices.Contains(s.Enum, v) {
		c.fail(at, "%v is not one of %v", v, s.Enum)
	}
	switch s.Type {
	case "object", "":
		obj, ok := v.(map[string]any)
		if !ok {
			if s.Type == "object" {
				c.fail(at, "a %T, the schema says object", v)
			}
			return
		}
		for _, name := range s.Required {
			if _, ok := obj[name]; !ok {
				c.fail(at, "required field %q is absent", name)
			}
		}
		var extra *schema
		open := false
		if len(s.AdditionalProperties) > 0 {
			var b bool
			if json.Unmarshal(s.AdditionalProperties, &b) == nil {
				open = b
			} else {
				extra = &schema{}
				_ = json.Unmarshal(s.AdditionalProperties, extra)
			}
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if p, ok := s.Properties[k]; ok {
				c.check(p, obj[k], at+"."+k)
			} else if extra != nil {
				c.check(*extra, obj[k], at+"."+k)
			} else if !open && (s.Properties != nil || s.Type == "object") {
				c.fail(at, "field %q is answered and not documented", k)
			}
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			c.fail(at, "a %T, the schema says array", v)
			return
		}
		if s.Items != nil {
			for i, x := range arr {
				c.check(*s.Items, x, fmt.Sprintf("%s[%d]", at, i))
			}
		}
	case "string":
		if _, ok := v.(string); !ok {
			c.fail(at, "a %T, the schema says string", v)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			c.fail(at, "a %T, the schema says boolean", v)
		}
	case "integer":
		if f, ok := v.(float64); !ok || f != float64(int64(f)) {
			c.fail(at, "%v, the schema says integer", v)
		}
	case "number":
		if _, ok := v.(float64); !ok {
			c.fail(at, "a %T, the schema says number", v)
		}
	}
}

// answerSchema is the schema the document gives a read's 200 JSON answer.
func answerSchema(doc specDoc, path string) (schema, bool) {
	var op struct {
		Responses map[string]struct {
			Content map[string]struct {
				Schema *schema `json:"schema"`
			} `json:"content"`
		} `json:"responses"`
	}
	raw, ok := doc.Paths[path]["get"]
	if !ok || json.Unmarshal(raw, &op) != nil {
		return schema{}, false
	}
	for ct, c := range op.Responses["200"].Content {
		if strings.Contains(ct, "json") && c.Schema != nil {
			return *c.Schema, true
		}
	}
	return schema{}, false
}

func TestEveryReadAnswersWhatTheReferenceSays(t *testing.T) {
	var doc specDoc
	if err := json.Unmarshal(apidocs.AdminSpec, &doc); err != nil {
		t.Fatal(err)
	}
	f := setup(t)
	checked := 0
	for _, pattern := range controlPlaneSurface() {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok || method != "GET" || strings.Contains(path, "{") || !strings.HasPrefix(path, "/api/") {
			continue
		}
		s, ok := answerSchema(doc, path)
		if !ok {
			continue // not JSON, or not documented: the plane's own test says which
		}
		code, body := f.call(t, method, path, "", f.rootC)
		if code != 200 {
			continue // refused in this edition, or a hub the fixture does not start
		}
		var v any
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Errorf("GET %s: the document says JSON, the answer is not: %.80s", path, body)
			continue
		}
		c := &checker{schemas: doc.Components.Schemas}
		c.check(s, v, "GET "+path)
		for _, e := range c.errs {
			t.Error(e)
		}
		checked++
	}
	if checked < 50 {
		t.Errorf("only %d reads checked: the document gives too few of them a JSON answer", checked)
	}
}
