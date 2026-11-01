package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// COMPOSING A CONFIGURATION FROM TWO (CFG-08). A plan is applied whole, and a
// line of it can be kept out (keep.go); what neither offers is "this route,
// but with the upstream it has here", or "this role list, with one more
// name". Compose takes two documents - the LEFT one is where the result
// starts, the RIGHT one is what it is compared with - and a recipe of
// choices, and builds one ordinary Document out of them. It applies nothing:
// the result goes through Switch, the shelf, the export or a push like any
// other document, with the same checks.
//
// Granularities, in the words a path uses:
//
//	route:r-x                       the object whole
//	route:r-x/upstream              a field, nested keys with dots
//	route:r-x/access.roles[ROLE_Y]  one element of a list
//	setting:tls/names[api.acme.io]  a setting is an object like another
//	mailRelay/host                  the relay has no id
//
// Generic over the document's JSON, like Compare: a section added tomorrow
// composes without anyone remembering to add it here. The result starts as
// the left side; a right-only object is in it only when picked; an object on
// both sides takes its fields from the left unless a pick says otherwise; a
// list is its elements, each in or out, in the order the recipe gives or the
// result holds.
//
// Dependencies are pulled, not refused: a route taken from the right names a
// role the left does not have, so the role comes along from the side that has
// it, said in the report, and the operator can still leave it out - then the
// reference is said as missing, and only a HARD one (a role's parent, a
// group's tenant, which the store refuses absent) blocks applying.

// Pick is one choice of the recipe: a path, and what to do at it.
type Pick struct {
	Path string `json:"path"`
	// From is the side the value comes from: left, right, none (the object or
	// field is not in the result), unset (a field cleared).
	From string `json:"from,omitempty"`
	// Op is add or drop, for a path that names one element of a list.
	Op string `json:"op,omitempty"`
	// Order is the keys of a list's elements, in the order wanted; the
	// elements it does not name keep their place after the ones it does.
	Order []string `json:"order,omitempty"`
}

// Pair says that the right object Right IS the left object Left under
// another id: the result keeps the left id, so the gateway updates the object
// instead of removing one and adding another.
type Pair struct {
	Kind  string `json:"kind"`
	Left  string `json:"left"`
	Right string `json:"right"`
}

// Recipe is what the console sends, and what an agent can replay on another
// gateway where the paths resolve.
type Recipe struct {
	Pairs []Pair `json:"pairs,omitempty"`
	Picks []Pick `json:"picks,omitempty"`
}

// Pulled is an object the composition added by itself, because another one
// in the result names it and the result did not have it.
type Pulled struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	// By names the objects that name it; From the side it was taken from.
	By   []string `json:"by"`
	From string   `json:"from"`
	Hard bool     `json:"hard,omitempty"`
}

// MissingDep is a reference the result makes that nothing in it declares.
type MissingDep struct {
	Kind string   `json:"kind"`
	Key  string   `json:"key"`
	By   []string `json:"by"`
	Via  string   `json:"via"`
	// Hard says the store would refuse the object without it; a soft one is
	// said and let through, the rule simply never matches (as oidcBindings).
	Hard bool `json:"hard"`
	// LeftOut says a side had it and the recipe said none: the cure is to
	// take it back rather than to declare it.
	LeftOut bool `json:"leftOut,omitempty"`
}

// PairProposal is a left-only and a right-only object of the same kind and
// name: probably the same thing under two generated ids. Proposed, never
// assumed.
type PairProposal struct {
	Kind  string `json:"kind"`
	Left  string `json:"left"`
	Right string `json:"right"`
	Label string `json:"label"`
}

// Report is what the composition has to say beside the document.
type Report struct {
	Pulled     []Pulled       `json:"pulled"`
	Missing    []MissingDep   `json:"missing"`
	Unresolved []string       `json:"unresolved"`
	Pairs      []PairProposal `json:"pairs"`
}

// Blocked reports whether a hard dependency is missing: the result cannot be
// applied as it stands, and says which object to take back.
func (r *Report) Blocked() error {
	for _, m := range r.Missing {
		if m.Hard {
			return fmt.Errorf("config: %s cannot be saved without %s %s, which the result does not hold: take it back, or leave %s out too",
				strings.Join(m.By, ", "), m.Kind, m.Key, strings.Join(m.By, ", "))
		}
	}
	return nil
}

// Tidy renders doc as the JSON a screen that composes reads: every object
// the way an export writes it, pictures out, blanks and stamps dropped - so a
// list is never null, a default spelled out on one side is not a difference,
// and an element's canonical JSON is the same here and in the browser.
func Tidy(doc *Document) (json.RawMessage, error) {
	stripped, err := WithoutImages(exported(doc))
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(stripped)
	if err != nil {
		return nil, err
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	return plainJSON(drop(strip(tree, stampedKeys)))
}

// plainJSON encodes without the HTML escaping json.Marshal applies: a key
// computed from "<" here and in a browser must be the same bytes.
func plainJSON(v any) (json.RawMessage, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(strings.TrimSuffix(b.String(), "\n")), nil
}

// Compose builds one document from two and a recipe. Neither input is
// changed. Never nil on success: a recipe that resolves nowhere answers the
// left document and says so in Unresolved.
func Compose(left, right *Document, recipe Recipe) (*Document, *Report, error) {
	c := &composer{
		left:   newSides(left),
		right:  newSides(right),
		report: &Report{Pulled: []Pulled{}, Missing: []MissingDep{}, Unresolved: []string{}, Pairs: []PairProposal{}},
		picks:  map[string][]Pick{},
		whole:  map[string]string{},
	}
	c.pair(recipe.Pairs)
	c.index(recipe.Picks)
	c.build()
	c.pull()
	c.propose()
	doc, err := c.document(left, right)
	if err != nil {
		return nil, nil, err
	}
	return doc, c.report, nil
}

// composer holds one composition in flight.
type composer struct {
	left, right *sideDoc
	report      *Report
	// picks are the recipe's field picks, by "kind:id"; whole are the picks
	// made on an object whole, by the same key.
	picks map[string][]Pick
	whole map[string]string
	// result is what is being built: by kind, the objects by id, in order.
	result map[string]*objects
	// origin says which side each result object came from.
	origin map[string]string
}

// sideDoc is one side read as JSON objects, kind by kind.
type sideDoc struct {
	kinds map[string]*objects
}

// objects is one section's objects by id, with their order.
type objects struct {
	ids  []string
	byID map[string]map[string]any
}

func newObjects() *objects { return &objects{byID: map[string]map[string]any{}} }

func (o *objects) put(id string, obj map[string]any) {
	if _, ok := o.byID[id]; !ok {
		o.ids = append(o.ids, id)
	}
	o.byID[id] = obj
}

func (o *objects) del(id string) {
	delete(o.byID, id)
	o.ids = slices.DeleteFunc(o.ids, func(x string) bool { return x == id })
}

// sectionOf maps a kind back to its section in the document.
var sectionOf = func() map[string]string {
	out := map[string]string{}
	for section, kind := range sectionKind {
		out[kind] = section
	}
	return out
}()

// kindOrder is the order the kinds are composed and written in: what names
// something comes after what it names, so a pull reads a finished section.
var kindOrder = []string{"setting", "mailRelay", "theme", "role", "tenant", "group", "authProvider", "oidcClient", "route"}

// newSides reads a document as JSON objects by kind, the way an export writes
// them (sections). Numbers stay numbers: UseNumber, so a port written 587
// comes back 587 and not 5.87e2.
func newSides(doc *Document) *sideDoc {
	out := &sideDoc{kinds: map[string]*objects{}}
	for section, raw := range sections(doc) {
		kind := sectionKind[section]
		if kind == "" {
			kind = section
		}
		objs := newObjects()
		switch {
		case isList(raw):
			var list []map[string]any
			_ = decodeNumbers(raw, &list)
			for i, o := range list {
				objs.put(objectID(o, i), o)
			}
		case kind == "setting":
			var m map[string]map[string]any
			_ = decodeNumbers(raw, &m)
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				objs.put(k, m[k])
			}
		default:
			var o map[string]any
			if decodeNumbers(raw, &o) == nil && o != nil {
				objs.put(kind, o)
			}
		}
		out.kinds[kind] = objs
	}
	return out
}

func (s *sideDoc) get(kind, id string) (map[string]any, bool) {
	objs, ok := s.kinds[kind]
	if !ok {
		return nil, false
	}
	o, ok := objs.byID[id]
	return o, ok
}

func decodeNumbers(raw json.RawMessage, into any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	return dec.Decode(into)
}

// objectID is the key an object is known by: its id, code or name, as byID
// reads it for a comparison.
func objectID(o map[string]any, i int) string {
	for _, k := range []string{"id", "code", "name"} {
		if v, ok := o[k].(string); ok && v != "" {
			return v
		}
	}
	return "#" + fmt.Sprint(i)
}

// pair re-keys the right objects the recipe pairs under their left id, so
// the two read as one object that differs.
func (c *composer) pair(pairs []Pair) {
	for _, p := range pairs {
		objs, ok := c.right.kinds[p.Kind]
		if !ok {
			c.report.Unresolved = append(c.report.Unresolved, p.Kind+":"+p.Right+" (pair)")
			continue
		}
		o, ok := objs.byID[p.Right]
		if !ok {
			c.report.Unresolved = append(c.report.Unresolved, p.Kind+":"+p.Right+" (pair)")
			continue
		}
		if _, has := c.left.get(p.Kind, p.Left); !has {
			c.report.Unresolved = append(c.report.Unresolved, p.Kind+":"+p.Left+" (pair)")
			continue
		}
		objs.del(p.Right)
		o = cloneObject(o)
		o["id"] = p.Left
		objs.put(p.Left, o)
	}
}

// index sorts the picks by the object they are about.
func (c *composer) index(picks []Pick) {
	for _, p := range picks {
		kind, id, field, _ := splitPath(p.Path)
		if kind == "" {
			c.report.Unresolved = append(c.report.Unresolved, p.Path)
			continue
		}
		key := kind + ":" + id
		if field == "" && p.Op == "" && len(p.Order) == 0 {
			c.whole[key] = p.From
			continue
		}
		c.picks[key] = append(c.picks[key], p)
	}
}

// splitPath reads "kind:id/field.path[element]". The relay has no id; a
// setting's id is its key.
func splitPath(path string) (kind, id, field, element string) {
	head, rest, _ := strings.Cut(path, "/")
	kind, id, hasID := strings.Cut(head, ":")
	if kind == "mailRelay" && !hasID {
		id = "mailRelay"
	}
	if kind == "" || id == "" {
		return "", "", "", ""
	}
	if i := strings.IndexByte(rest, '['); i >= 0 && strings.HasSuffix(rest, "]") {
		field, element = rest[:i], rest[i+1:len(rest)-1]
	} else {
		field = rest
	}
	return kind, id, field, element
}

// build composes every object of every kind.
func (c *composer) build() {
	c.result = map[string]*objects{}
	c.origin = map[string]string{}
	for _, kind := range c.allKinds() {
		out := newObjects()
		l, r := c.left.kinds[kind], c.right.kinds[kind]
		ids := []string{}
		if l != nil {
			ids = append(ids, l.ids...)
		}
		if r != nil {
			for _, id := range r.ids {
				if l == nil || l.byID[id] == nil {
					ids = append(ids, id)
				}
			}
		}
		for _, id := range ids {
			var lo, ro map[string]any
			if l != nil {
				lo = l.byID[id]
			}
			if r != nil {
				ro = r.byID[id]
			}
			key := kind + ":" + id
			from, said := c.whole[key]
			switch {
			case said && from == "none":
				continue
			case said && from == "right" && ro != nil:
			case said && from == "left" && lo != nil:
			case said && (from == "left" || from == "right"):
				c.report.Unresolved = append(c.report.Unresolved, key+" from "+from)
				from = ""
			case said:
				c.report.Unresolved = append(c.report.Unresolved, key+" from "+from)
				from = ""
			}
			if from == "" {
				if lo == nil {
					continue // right-only, not picked: not in the result
				}
				from = "left"
			}
			var obj map[string]any
			if from == "left" {
				obj = cloneObject(lo)
			} else {
				obj = cloneObject(ro)
			}
			// Field picks apply to an object one side has too: dropping a
			// role the other side never declared, clearing a field, are
			// choices about the result, not about a side.
			c.applyFields(key, obj, lo, ro)
			out.put(id, obj)
			c.origin[key] = from
		}
		c.result[kind] = out
	}
	// A pick on an object neither side has resolves nowhere: said, so a recipe
	// replayed on another gateway does not pass a typo under silence.
	keys := make([]string, 0, len(c.whole)+len(c.picks))
	for key := range c.whole {
		keys = append(keys, key)
	}
	for key := range c.picks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		kind, id, _ := strings.Cut(key, ":")
		if _, l := c.left.get(kind, id); l {
			continue
		}
		if _, r := c.right.get(kind, id); r {
			continue
		}
		if from, ok := c.whole[key]; ok {
			c.report.Unresolved = append(c.report.Unresolved, key+" from "+from)
		}
		for _, p := range c.picks[key] {
			c.report.Unresolved = append(c.report.Unresolved, p.Path)
		}
	}
}

// allKinds is every kind either side carries, in composition order, then
// the ones this package does not know (a section added tomorrow).
func (c *composer) allKinds() []string {
	seen := map[string]bool{}
	var out []string
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, k := range kindOrder {
		if c.left.kinds[k] != nil || c.right.kinds[k] != nil {
			add(k)
		}
	}
	for _, side := range []*sideDoc{c.left, c.right} {
		keys := make([]string, 0, len(side.kinds))
		for k := range side.kinds {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(k)
		}
	}
	return out
}

// applyFields applies the field picks of one object on both sides: a field
// whole from a side, cleared, an element in or out, a list ordered. Whole
// fields first, then elements, then order, so the three compose.
func (c *composer) applyFields(key string, obj, lo, ro map[string]any) {
	picks := c.picks[key]
	for _, p := range picks {
		_, _, field, element := splitPath(p.Path)
		if element != "" || len(p.Order) > 0 {
			continue
		}
		switch p.From {
		case "left":
			setField(obj, field, getField(lo, field))
		case "right":
			setField(obj, field, getField(ro, field))
		case "unset":
			setField(obj, field, nil)
		default:
			c.report.Unresolved = append(c.report.Unresolved, p.Path)
		}
	}
	for _, p := range picks {
		_, _, field, element := splitPath(p.Path)
		if element == "" {
			continue
		}
		cur := toList(getField(obj, field))
		switch p.Op {
		case "add":
			el, ok := findElement(element, getField(lo, field), getField(ro, field))
			if !ok {
				c.report.Unresolved = append(c.report.Unresolved, p.Path)
				continue
			}
			if indexOfElement(cur, element) < 0 {
				cur = append(cur, el)
			}
		case "drop":
			if i := indexOfElement(cur, element); i >= 0 {
				cur = slices.Delete(cur, i, i+1)
			}
		default:
			c.report.Unresolved = append(c.report.Unresolved, p.Path)
			continue
		}
		setField(obj, field, cur)
	}
	for _, p := range picks {
		_, _, field, _ := splitPath(p.Path)
		if len(p.Order) == 0 {
			continue
		}
		cur := toList(getField(obj, field))
		if cur == nil {
			continue
		}
		rank := func(v any) int {
			if i := slices.Index(p.Order, elementKey(v)); i >= 0 {
				return i
			}
			return len(p.Order)
		}
		sort.SliceStable(cur, func(i, j int) bool { return rank(cur[i]) < rank(cur[j]) })
		setField(obj, field, cur)
	}
}

// pull adds what the result names and does not hold, from the side that has
// it, until nothing moves (a pulled role has a parent); what nobody holds is
// said as missing. An object the recipe said none for is not pulled back:
// the operator left it out, and the report says what that costs.
func (c *composer) pull() {
	for moved := true; moved; {
		moved = false
		for _, kind := range c.allKinds() {
			objs := c.result[kind]
			for _, id := range slices.Clone(objs.ids) {
				obj := objs.byID[id]
				who := label(obj, id)
				for _, ref := range refsOf(kind, obj) {
					if _, has := c.result[ref.kind]; !has {
						c.result[ref.kind] = newObjects()
					}
					if _, ok := c.result[ref.kind].byID[ref.key]; ok {
						continue
					}
					key := ref.kind + ":" + ref.key
					if c.whole[key] == "none" {
						c.missing(ref, who, true)
						continue
					}
					from, src := "", map[string]any(nil)
					if o, ok := c.right.get(ref.kind, ref.key); ok {
						from, src = "right", o
					}
					if o, ok := c.left.get(ref.kind, ref.key); ok {
						from, src = "left", o
					}
					if src == nil {
						c.missing(ref, who, false)
						continue
					}
					c.result[ref.kind].put(ref.key, cloneObject(src))
					c.origin[key] = from
					c.pulled(ref, who, from)
					moved = true
				}
			}
		}
	}
}

func (c *composer) pulled(ref dep, who, from string) {
	for i := range c.report.Pulled {
		p := &c.report.Pulled[i]
		if p.Kind == ref.kind && p.ID == ref.key {
			if !slices.Contains(p.By, who) {
				p.By = append(p.By, who)
			}
			return
		}
	}
	c.report.Pulled = append(c.report.Pulled, Pulled{Kind: ref.kind, ID: ref.key, By: []string{who}, From: from, Hard: ref.hard})
}

func (c *composer) missing(ref dep, who string, leftOut bool) {
	for i := range c.report.Missing {
		m := &c.report.Missing[i]
		if m.Kind == ref.kind && m.Key == ref.key {
			if !slices.Contains(m.By, who) {
				m.By = append(m.By, who)
			}
			return
		}
	}
	c.report.Missing = append(c.report.Missing, MissingDep{Kind: ref.kind, Key: ref.key, By: []string{who}, Via: ref.via, Hard: ref.hard, LeftOut: leftOut})
}

// dep is one reference an object makes to another.
type dep struct {
	kind, key, via string
	hard           bool
}

// refsOf is the graph of a document, small and closed: what an object names
// in another section. Hard is what the store refuses absent.
func refsOf(kind string, o map[string]any) []dep {
	var out []dep
	switch kind {
	case "route":
		for _, r := range toStrings(getField(o, "access.roles")) {
			out = append(out, dep{kind: "role", key: r, via: "access.roles"})
		}
		for _, t := range toStrings(getField(o, "access.tenants")) {
			out = append(out, dep{kind: "tenant", key: t, via: "access.tenants"})
		}
		if m, _ := getField(o, "identity.mechanism").(string); m == store.IdentityOIDC {
			if client, _ := getField(o, "identity.client").(string); client != "" {
				out = append(out, dep{kind: "oidcClient", key: client, via: "identity.client"})
			}
		}
	case "role":
		if parent, _ := o["parent"].(string); parent != "" {
			out = append(out, dep{kind: "role", key: parent, via: "parent", hard: true})
		}
	case "group":
		if t, _ := o["tenantId"].(string); t != "" {
			out = append(out, dep{kind: "tenant", key: t, via: "tenantId", hard: true})
		}
		for _, r := range toStrings(o["roles"]) {
			out = append(out, dep{kind: "role", key: r, via: "roles"})
		}
	}
	return out
}

// propose names the left-only and right-only objects of one kind that share
// a name: the same thing under two generated ids, most likely. Said in the
// report, never paired by itself.
func (c *composer) propose() {
	for _, kind := range c.allKinds() {
		if kind == "setting" || kind == "mailRelay" || kind == "role" || kind == "oidcClient" {
			continue // keyed by what names them already
		}
		l, r := c.left.kinds[kind], c.right.kinds[kind]
		if l == nil || r == nil {
			continue
		}
		for _, lid := range l.ids {
			if r.byID[lid] != nil {
				continue
			}
			ln := pairName(kind, l.byID[lid])
			if ln == "" {
				continue
			}
			for _, rid := range r.ids {
				if l.byID[rid] != nil || pairName(kind, r.byID[rid]) != ln {
					continue
				}
				c.report.Pairs = append(c.report.Pairs, PairProposal{Kind: kind, Left: lid, Right: rid, Label: label(l.byID[lid], lid)})
			}
		}
	}
}

// pairName is what makes two objects "the same" to an operator: the name,
// and for a group its tenant too, for an authority its kind too.
func pairName(kind string, o map[string]any) string {
	name, _ := o["name"].(string)
	if name == "" {
		return ""
	}
	switch kind {
	case "group":
		t, _ := o["tenantId"].(string)
		return t + "/" + name
	case "authProvider":
		k, _ := o["kind"].(string)
		return k + "/" + name
	}
	return name
}

// document writes the result back as a Document, through the same JSON the
// sides were read from. The media beside a route (its spec, its files)
// follow the route, from the side it came from.
func (c *composer) document(left, right *Document) (*Document, error) {
	out := map[string]any{"version": Version}
	for kind, objs := range c.result {
		if len(objs.ids) == 0 {
			continue
		}
		section := sectionOf[kind]
		if section == "" {
			section = kind
		}
		switch kind {
		case "setting":
			m := map[string]any{}
			for _, id := range objs.ids {
				m[id] = objs.byID[id]
			}
			out[section] = m
		case "mailRelay":
			out[section] = objs.byID[objs.ids[0]]
		default:
			list := make([]any, 0, len(objs.ids))
			for _, id := range objs.ids {
				list = append(list, objs.byID[id])
			}
			out[section] = list
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("config: compose: %w", err)
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("config: the composed document does not read as a configuration: %w", err)
	}
	if routes := c.result["route"]; routes != nil {
		for _, id := range routes.ids {
			side := left
			if c.origin["route:"+id] == "right" {
				side = right
			}
			if side == nil {
				continue
			}
			if spec, ok := side.Specs[id]; ok {
				if doc.Specs == nil {
					doc.Specs = map[string][]byte{}
				}
				doc.Specs[id] = spec
			}
			if files, ok := side.Files[id]; ok {
				if doc.Files == nil {
					doc.Files = map[string][]store.RouteFile{}
				}
				doc.Files[id] = files
			}
		}
	}
	return &doc, nil
}

// ── JSON helpers ─────────────────────────────────────────────────────────────

func cloneObject(o map[string]any) map[string]any {
	if o == nil {
		return nil
	}
	return cloneValue(o).(map[string]any)
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = cloneValue(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = cloneValue(x)
		}
		return out
	}
	return v
}

// getField walks a dotted path; nil when absent.
func getField(o map[string]any, path string) any {
	if path == "" {
		return o
	}
	var cur any = o
	for _, k := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[k]
		if !ok {
			return nil
		}
	}
	return cur
}

// setField writes a dotted path, creating the objects on the way; nil
// deletes the leaf, and an object left empty by it goes too.
func setField(o map[string]any, path string, v any) {
	keys := strings.Split(path, ".")
	cur := o
	parents := []map[string]any{}
	for _, k := range keys[:len(keys)-1] {
		next, ok := cur[k].(map[string]any)
		if !ok {
			if v == nil {
				return
			}
			next = map[string]any{}
			cur[k] = next
		}
		parents = append(parents, cur)
		cur = next
	}
	leaf := keys[len(keys)-1]
	if v == nil {
		delete(cur, leaf)
		for i := len(parents) - 1; i >= 0 && len(cur) == 0; i-- {
			delete(parents[i], keys[i])
			cur = parents[i]
		}
		return
	}
	cur[leaf] = cloneValue(v)
}

func toList(v any) []any {
	l, _ := v.([]any)
	return l
}

func toStrings(v any) []string {
	var out []string
	for _, x := range toList(v) {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// elementKey is what names an element of a list: a string is itself, anything
// else its canonical JSON - which is what a path writes between brackets.
func elementKey(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return string(raw)
	}
	// Blanks and stamps out, keys sorted (a map marshals sorted), no HTML
	// escaping: what JSON.stringify of the same element gives in the browser.
	if c, err := plainJSON(drop(strip(tree, stampedKeys))); err == nil {
		return string(c)
	}
	return string(raw)
}

func indexOfElement(list []any, key string) int {
	return slices.IndexFunc(list, func(v any) bool { return elementKey(v) == key })
}

// findElement looks for the element on either side, the left first.
func findElement(key string, sides ...any) (any, bool) {
	for _, side := range sides {
		if i := indexOfElement(toList(side), key); i >= 0 {
			return cloneValue(toList(side)[i]), true
		}
	}
	return nil, false
}

// label is how an object reads in a sentence: its name, else its id.
func label(o map[string]any, id string) string {
	if name, _ := o["name"].(string); name != "" {
		return name
	}
	return id
}
