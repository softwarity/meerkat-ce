package config

import (
	"encoding/json"
	"sort"
	"strconv"
)

// Compare says what changes going from one configuration document to another
// (CFG-04), without looking at what runs. The switch preview answers "what
// would this do to the gateway"; this answers "how do these two differ" -
// the question asked of two saved configurations, a customer's and the
// template it was made from, before touching either.
//
// Object by object, in the words the import uses: a route, a role, an
// authority, a theme, an organisation, a group by its id; a setting by its
// key; the mail relay as one object. An update names the fields that moved.
// Generic over the document's JSON: a section added to the document later is
// compared without anyone remembering to add it here.
func Compare(from, to *Document) []Change {
	a, b := sections(from), sections(to)
	var out []Change
	for _, key := range sortedKeys(a, b) {
		kind := sectionKind[key]
		if kind == "" {
			kind = key
		}
		out = append(out, compareSection(kind, a[key], b[key])...)
	}
	return out
}

// sectionKind names each section's objects the way a plan does.
var sectionKind = map[string]string{
	"routes": "route", "roles": "role", "authProviders": "authProvider", "themes": "theme",
	"tenants": "tenant", "groups": "group", "settings": "setting", "mailRelay": "mailRelay",
}

// sections is the document as JSON, section by section, with the version left
// out: it says which format the file is in, not what it configures.
func sections(doc *Document) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	if doc == nil {
		return out
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	delete(out, "version")
	return out
}

// compareSection compares one section: a list of objects with an id, a map of
// keys (the settings), or one object (the relay).
func compareSection(kind string, a, b json.RawMessage) []Change {
	var la, lb []map[string]json.RawMessage
	if isList(a) || isList(b) {
		_ = json.Unmarshal(orNull(a), &la)
		_ = json.Unmarshal(orNull(b), &lb)
		return compareObjects(kind, byID(la), byID(lb))
	}
	if kind == "setting" {
		var ma, mb map[string]json.RawMessage
		_ = json.Unmarshal(orNull(a), &ma)
		_ = json.Unmarshal(orNull(b), &mb)
		var out []Change
		for _, key := range sortedKeys(ma, mb) {
			out = append(out, compareOne(kind, key, key, ma[key], mb[key]))
		}
		return out
	}
	return []Change{compareOne(kind, kind, kind, a, b)}
}

func compareObjects(kind string, a, b map[string]map[string]json.RawMessage) []Change {
	ids := map[string]bool{}
	for id := range a {
		ids[id] = true
	}
	for id := range b {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	var out []Change
	for _, id := range sorted {
		oa, ob := a[id], b[id]
		label := labelOf(ob)
		if label == "" {
			label = labelOf(oa)
		}
		if label == "" {
			label = id
		}
		switch {
		case oa == nil:
			out = append(out, Change{Kind: kind, ID: id, Label: label, Action: ActionAdd})
		case ob == nil:
			out = append(out, Change{Kind: kind, ID: id, Label: label, Action: ActionRemove})
		default:
			fields := differingKeys(oa, ob)
			action := ActionSame
			if len(fields) > 0 {
				action = ActionUpdate
			}
			out = append(out, Change{Kind: kind, ID: id, Label: label, Action: action, Fields: fields})
		}
	}
	return out
}

func compareOne(kind, id, label string, a, b json.RawMessage) Change {
	switch {
	case isAbsent(a) && isAbsent(b):
		return Change{Kind: kind, ID: id, Label: label, Action: ActionSame}
	case isAbsent(a):
		return Change{Kind: kind, ID: id, Label: label, Action: ActionAdd}
	case isAbsent(b):
		return Change{Kind: kind, ID: id, Label: label, Action: ActionRemove}
	case sameJSON(a, b):
		return Change{Kind: kind, ID: id, Label: label, Action: ActionSame}
	}
	return Change{Kind: kind, ID: id, Label: label, Action: ActionUpdate, Fields: changedFields(a, b)}
}

// byID indexes a list of objects by their id, falling back to a code or a
// name for the objects that carry no id.
func byID(list []map[string]json.RawMessage) map[string]map[string]json.RawMessage {
	out := map[string]map[string]json.RawMessage{}
	for i, o := range list {
		id := ""
		for _, k := range []string{"id", "code", "name"} {
			if v, ok := o[k]; ok {
				_ = json.Unmarshal(v, &id)
				if id != "" {
					break
				}
			}
		}
		if id == "" {
			id = "#" + strconv.Itoa(i)
		}
		out[id] = o
	}
	return out
}

func labelOf(o map[string]json.RawMessage) string {
	var name string
	if v, ok := o["name"]; ok {
		_ = json.Unmarshal(v, &name)
	}
	return name
}

func differingKeys(a, b map[string]json.RawMessage) []string {
	var out []string
	for _, k := range sortedKeys(a, b) {
		if !sameJSON(orNull(a[k]), orNull(b[k])) {
			out = append(out, k)
		}
	}
	return out
}

func sortedKeys[V any](maps ...map[string]V) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range maps {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

func isList(raw json.RawMessage) bool {
	for _, c := range raw {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			return true
		}
		return false
	}
	return false
}

func isAbsent(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

func orNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}
