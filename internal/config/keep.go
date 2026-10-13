package config

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// KEEPING PART OF WHAT IS THERE. A plan is applied whole, and an operator
// reading it sometimes wants all of it but one line: a route the configuration
// would delete that is still in use, an authority they changed by hand since.
// Each line of the plan can be kept out, and keeping it out means one thing
// whatever its action: what this gateway holds NOW stays as it is.
//
//   - a removal kept out: the object stays;
//   - an update kept out: the current version stays;
//   - an addition kept out: the object is not created.
//
// It works on the DOCUMENT, before it is applied, not inside each section: the
// current version of a kept object is put back into the document (or the
// object taken out of it when there is none), and the document then goes
// through the one path every import and switch already takes, its checks
// included. A kept object that the rest of the document cannot live with is
// refused there, with what is wrong, and nothing is applied half.
//
// What the gateway runs afterwards is no longer exactly the saved
// configuration, and the configuration screen already says so: the gateway has
// drifted from it, and saving it again records what was chosen.

// Ref names one line of a plan: its kind and its id.
type Ref struct {
	Kind string
	ID   string
}

// keepable are the kinds a line can be kept out for. The deposited files of a
// route follow their route: keeping a route keeps its spec and its files.
var keepable = []string{"route", "role", "group", "authProvider", "theme", "tenant", "mailRelay", "setting"}

// ParseRefs reads "kind:id" items - the form the API takes them in. The mail
// relay has no id: "mailRelay" alone.
func ParseRefs(raw []string) ([]Ref, error) {
	out := make([]Ref, 0, len(raw))
	for _, item := range raw {
		kind, id, _ := strings.Cut(strings.TrimSpace(item), ":")
		if !slices.Contains(keepable, kind) {
			return nil, fmt.Errorf("config: %q cannot be kept out: a line is one of %s, as kind:id",
				item, strings.Join(keepable, ", "))
		}
		if id == "" && kind != "mailRelay" {
			return nil, fmt.Errorf("config: %q names no %s: write it kind:id", item, kind)
		}
		out = append(out, Ref{Kind: kind, ID: id})
	}
	return out, nil
}

// Keep returns a copy of doc in which the lines named by keep are not applied:
// for each, the gateway's current object, or nothing when it has none.
// The caller's document is never changed - the same one is previewed and then
// applied.
func Keep(ctx context.Context, st *store.Store, doc *Document, keep []Ref) (*Document, error) {
	if len(keep) == 0 {
		return doc, nil
	}
	current, _, err := Export(ctx, st)
	if err != nil {
		return nil, fmt.Errorf("config: read what this gateway holds now: %w", err)
	}
	out := *doc
	out.Settings = maps.Clone(doc.Settings)
	out.Specs = maps.Clone(doc.Specs)
	out.Files = maps.Clone(doc.Files)
	for _, ref := range keep {
		switch ref.Kind {
		case "route":
			out.Routes = keepOne(out.Routes, current.Routes, ref.ID, func(r store.Route) string { return r.ID })
			if spec, ok := current.Specs[ref.ID]; ok {
				if out.Specs == nil {
					out.Specs = map[string][]byte{}
				}
				out.Specs[ref.ID] = spec
			} else {
				delete(out.Specs, ref.ID)
			}
			if files, ok := current.Files[ref.ID]; ok {
				if out.Files == nil {
					out.Files = map[string][]store.RouteFile{}
				}
				out.Files[ref.ID] = files
			} else {
				delete(out.Files, ref.ID)
			}
		case "role":
			out.Roles = keepOne(out.Roles, current.Roles, ref.ID, func(r store.Role) string { return r.Name })
		case "group":
			out.Groups = keepOne(out.Groups, current.Groups, ref.ID, func(g store.Group) string { return g.ID })
		case "authProvider":
			out.AuthProviders = keepOne(out.AuthProviders, current.AuthProviders, ref.ID, func(p store.AuthProvider) string { return p.ID })
		case "theme":
			out.Themes = keepOne(out.Themes, current.Themes, ref.ID, func(t store.Theme) string { return t.ID })
		case "tenant":
			out.Tenants = keepOne(out.Tenants, current.Tenants, ref.ID, func(t store.Tenant) string { return t.ID })
		case "mailRelay":
			// A single object: absent from the document, it is left alone.
			out.MailRelay = nil
		case "setting":
			// Same: a setting the document does not carry is not touched -
			// a switch completes the lists, never the settings.
			delete(out.Settings, ref.ID)
		}
	}
	return &out, nil
}

// keepOne puts the current object named id in place of the document's, or
// appends it when the document has none (a removal kept out), or takes the
// document's out when the gateway has none (an addition kept out). A nil list
// stays nil when nothing is to be kept in it: a section the document does not
// carry must not appear from nowhere.
func keepOne[T any](list, current []T, id string, key func(T) string) []T {
	at := slices.IndexFunc(list, func(x T) bool { return key(x) == id })
	now := slices.IndexFunc(current, func(x T) bool { return key(x) == id })
	out := slices.Clone(list)
	switch {
	case now >= 0 && at >= 0:
		out[at] = current[now]
	case now >= 0:
		out = append(out, current[now])
	case at >= 0:
		out = slices.Delete(out, at, at+1)
		if out == nil {
			out = []T{}
		}
	}
	return out
}
