package admin

import (
	"encoding"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// No answer of the control plane carries a list as null.
//
// The console reads a list as a list: `p.missing.length` on a null threw, and
// the activation dialog came up empty (2026-10-14). Go writes a nil slice as
// null, so the mistake is one forgotten make() away in any handler - which is
// why the guard sits on writeJSON, through which every answer goes, rather than
// on the handful of endpoints that once got it wrong. Every test of this
// package that reaches an endpoint checks it; TestMain fails the run if any
// did, and TestEveryReadAnswersListsNotNull walks every read of a fresh
// gateway, where empty lists are the rule.

var (
	nullMu   sync.Mutex
	nullSeen = map[string]bool{}
)

func TestMain(m *testing.M) {
	checkAnswer = func(v any) {
		found := nullLists(v)
		if len(found) == 0 {
			return
		}
		nullMu.Lock()
		defer nullMu.Unlock()
		for _, f := range found {
			nullSeen[f] = true
		}
	}
	code := m.Run()
	if seen := nullsSeen(); len(seen) > 0 {
		fmt.Fprintln(os.Stderr, "the control plane answered a list as null (make the slice, or tag it omitempty):")
		for _, s := range seen {
			fmt.Fprintln(os.Stderr, "  "+s)
		}
		code = 1
	}
	os.Exit(code)
}

func nullsSeen() []string {
	nullMu.Lock()
	defer nullMu.Unlock()
	out := make([]string, 0, len(nullSeen))
	for s := range nullSeen {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

var (
	jsonMarshaler = reflect.TypeFor[json.Marshaler]()
	textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()
)

// nullLists names every list in v that encoding/json would write as null: a
// nil slice in a field without omitempty, a map value, or v itself. A type
// that writes itself (MarshalJSON, MarshalText) is its own business, and a
// []byte is a string on the wire.
func nullLists(v any) []string {
	root := fmt.Sprintf("%T", v)
	seen := map[string]bool{}
	var walk func(rv reflect.Value, path string, depth int)
	isNullList := func(rv reflect.Value) bool {
		return rv.Kind() == reflect.Slice && rv.IsNil() && rv.Type().Elem().Kind() != reflect.Uint8 &&
			!writesItself(rv)
	}
	walk = func(rv reflect.Value, path string, depth int) {
		if !rv.IsValid() || depth > 32 || writesItself(rv) {
			return
		}
		switch rv.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !rv.IsNil() {
				walk(rv.Elem(), path, depth+1)
			}
		case reflect.Struct:
			t := rv.Type()
			for i := range t.NumField() {
				f := t.Field(i)
				if !f.IsExported() && !f.Anonymous {
					continue
				}
				name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
				if name == "-" && opts == "" {
					continue
				}
				fv := rv.Field(i)
				if f.Anonymous && name == "" {
					walk(fv, path, depth+1)
					continue
				}
				if name == "" {
					name = f.Name
				}
				omitted := slices.Contains(strings.Split(opts, ","), "omitempty") ||
					slices.Contains(strings.Split(opts, ","), "omitzero")
				if isNullList(fv) {
					if !omitted {
						seen[path+"."+name] = true
					}
					continue
				}
				walk(fv, path+"."+name, depth+1)
			}
		case reflect.Slice, reflect.Array:
			if rv.Type().Elem().Kind() == reflect.Uint8 {
				return
			}
			for i := range rv.Len() {
				walk(rv.Index(i), path+"[]", depth+1)
			}
		case reflect.Map:
			iter := rv.MapRange()
			for iter.Next() {
				if isNullList(iter.Value()) {
					seen[path+"{}"] = true
					continue
				}
				walk(iter.Value(), path+"{}", depth+1)
			}
		}
	}
	rv := reflect.ValueOf(v)
	if isNullList(rv) {
		seen[""] = true
	} else {
		walk(rv, "", 0)
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, root+p)
	}
	slices.Sort(out)
	return out
}

func writesItself(rv reflect.Value) bool {
	t := rv.Type()
	if t.Implements(jsonMarshaler) || t.Implements(textMarshaler) {
		return true
	}
	if t.Kind() != reflect.Pointer {
		pt := reflect.PointerTo(t)
		return pt.Implements(jsonMarshaler) || pt.Implements(textMarshaler)
	}
	return false
}

// The guard itself: what it must catch, and what it must leave alone.
func TestTheNullListGuard(t *testing.T) {
	type inner struct {
		Names []string `json:"names"`
	}
	type answer struct {
		Lines    []string          `json:"lines"`
		Optional []string          `json:"optional,omitempty"`
		Raw      json.RawMessage   `json:"raw"`
		Bytes    []byte            `json:"bytes"`
		Inner    inner             `json:"inner"`
		Many     []inner           `json:"many"`
		ByName   map[string][]int  `json:"byName"`
		Hidden   []string          `json:"-"`
		Nested   *answer           `json:"nested"`
		Any      any               `json:"any"`
		Fine     map[string]string `json:"fine"`
	}
	got := nullLists(answer{
		Many:   []inner{{Names: []string{"a"}}, {}},
		ByName: map[string][]int{"x": nil},
		Any:    inner{},
	})
	want := []string{
		"admin.answer.any.names",
		"admin.answer.byName{}",
		"admin.answer.inner.names",
		"admin.answer.lines",
		"admin.answer.many[].names",
	}
	if !slices.Equal(got, want) {
		t.Errorf("guard found\n  %v\nwant\n  %v", got, want)
	}
	if got := nullLists([]string(nil)); len(got) != 1 {
		t.Errorf("a nil list answered whole: %v", got)
	}
	if got := nullLists(map[string]any{"routes": []string{}}); len(got) != 0 {
		t.Errorf("an empty list is not null: %v", got)
	}
}

// Every read the control plane mounts, on a fresh gateway and as root: the
// state where every list is empty, which is exactly when a nil slice goes out
// as null. A read that needs an id is reached by the tests of its section.
func TestEveryReadAnswersListsNotNull(t *testing.T) {
	f := setup(t)
	before := nullsSeen()
	reached := 0
	for _, pattern := range controlPlaneSurface() {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok || method != "GET" || strings.Contains(path, "{") || !strings.HasPrefix(path, "/api/") {
			continue
		}
		path = strings.TrimSuffix(path, "/")
		code, body := f.call(t, method, path, "", f.rootC)
		// The live channel is a hub this fixture does not start; it says so.
		if code >= 500 && path != "/api/live" {
			t.Errorf("GET %s on a fresh gateway: %d %s", path, code, body)
		}
		reached++
	}
	if reached < 50 {
		t.Errorf("only %d reads reached: the surface did not list them", reached)
	}
	var fresh []string
	for _, s := range nullsSeen() {
		if !slices.Contains(before, s) {
			fresh = append(fresh, s)
		}
	}
	if len(fresh) > 0 {
		t.Errorf("a fresh gateway answers lists as null: %v", fresh)
	}
}
