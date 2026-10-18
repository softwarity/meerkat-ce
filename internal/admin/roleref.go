package admin

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// What POINTS AT a role.
//
// An access rule names roles BY NAME (store.Access.Roles, on a route and on
// each endpoint policy), and so do a route's role map (the catalogue role
// each declared role is tied to) and a scheduled call. Those are strings
// written next to the name at a moment in time, and nothing in the database
// ties them back - no foreign key reaches inside a route's JSON.
//
// So a deletion says what still names the role rather than leaving rules that
// grant nobody. There used to be a rename here too, that rewrote them all; it
// is gone with the rename itself, because what it rewrote was only this
// gateway's half - the services that read the name out of a token, and the
// customers who hold it in a repository, were never in reach.
//
// Names are compared EXACTLY. The gateway compares them exactly too, so a rule
// naming "role_user" is not a rule naming "ROLE_USER".

// roleReference is one place a role name is written, said the way an operator
// would look for it.
type roleReference struct {
	Kind  string `json:"kind"`  // route | endpoint | roleMap | schedule
	ID    string `json:"id"`    // the route's or schedule's id - what get_route takes
	Label string `json:"label"` // what to look for on screen
	// Method and Path name the endpoint policy inside the route, for the two
	// kinds where "the route" is not precise enough to go and change it.
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
	// Owner is the route's or schedule's name, without the detail Label adds.
	Owner string `json:"-"`
}

func (r roleReference) String() string { return r.Kind + " " + r.Label }

// roleReferences lists what names this role. It reads; it changes nothing.
func (a *API) roleReferences(ctx context.Context, name string) ([]roleReference, error) {
	if name == "" {
		return nil, nil
	}
	var out []roleReference
	err := a.eachRoleReference(ctx, func(role string, ref roleReference) {
		if role == name {
			out = append(out, ref)
		}
	})
	return out, err
}

// roleUse is what a role is used BY, as the catalogue shows it: a route or a
// scheduled call, once, however many of its rules name the role.
type roleUse struct {
	Kind string `json:"kind"` // route | schedule
	ID   string `json:"id"`
	Name string `json:"name"`
}

// roleUsage says, for every role named somewhere, what names it - read off
// the routes and the schedules in one pass, never stored. A tag is what an
// operator WRITES about a role; this is what the configuration says of it, so
// it cannot drift from it and nobody has to keep it up to date.
func (a *API) roleUsage(ctx context.Context) (map[string][]roleUse, error) {
	usage := map[string][]roleUse{}
	seen := map[string]bool{}
	err := a.eachRoleReference(ctx, func(role string, ref roleReference) {
		use := roleUse{Kind: "route", ID: ref.ID, Name: ref.Owner}
		if ref.Kind == "schedule" {
			use.Kind = "schedule"
		}
		key := role + "\x00" + use.Kind + "\x00" + use.ID
		if seen[key] {
			return
		}
		seen[key] = true
		usage[role] = append(usage[role], use)
	})
	for _, uses := range usage {
		slices.SortFunc(uses, func(x, y roleUse) int {
			return cmp.Or(strings.Compare(x.Kind, y.Kind), strings.Compare(x.Name, y.Name), strings.Compare(x.ID, y.ID))
		})
	}
	return usage, err
}

// eachRoleReference walks every place a role name is written, and calls fn
// with the name and the place.
func (a *API) eachRoleReference(ctx context.Context, fn func(role string, ref roleReference)) error {
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return err
	}
	for _, rt := range routes {
		for _, role := range roleNames(rt.Access.Roles) {
			fn(role, roleReference{Kind: "route", ID: rt.ID, Label: rt.Name, Owner: rt.Name})
		}
		// A role the application declares, tied to this one: the route
		// forwards it under the application's name, and a deletion would
		// leave the declared role granted by nobody.
		roleMap := rt.RoleMap()
		for _, declared := range slices.Sorted(maps.Keys(roleMap)) {
			if role := strings.TrimSpace(roleMap[declared]); role != "" {
				fn(role, roleReference{Kind: "roleMap", ID: rt.ID, Owner: rt.Name,
					Label: fmt.Sprintf("%s: declared role %s", rt.Name, declared)})
			}
		}
		if rt.API == nil || rt.API.Security == nil {
			continue
		}
		for _, e := range rt.API.Security.Endpoints {
			for _, role := range roleNames(e.Roles) {
				fn(role, roleReference{Kind: "endpoint", ID: rt.ID, Owner: rt.Name,
					Label:  fmt.Sprintf("%s: %s %s", rt.Name, e.Method, e.Path),
					Method: e.Method, Path: e.Path})
			}
		}
	}
	schedules, err := a.st.ListSchedules(ctx, store.ScheduleFilter{})
	if err != nil {
		return err
	}
	for _, s := range schedules {
		for _, role := range roleNames(s.Roles) {
			fn(role, roleReference{Kind: "schedule", ID: s.ID, Label: s.Name, Owner: s.Name})
		}
	}
	return nil
}

// roleNames is a rule's list of roles as the gateway compares them: trimmed,
// each once, blanks left out.
func roleNames(list []string) []string {
	out := make([]string, 0, len(list))
	for _, r := range list {
		if r = strings.TrimSpace(r); r != "" && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

func namesRole(list []string, name string) bool {
	for _, r := range list {
		if strings.TrimSpace(r) == name {
			return true
		}
	}
	return false
}

// refLabels is what a refusal or an answer shows of a reference list: a few,
// then how many more. A role named by forty endpoints must not answer with
// forty lines.
func refLabels(refs []roleReference, shown int) string {
	labels := make([]string, 0, shown)
	for i, r := range refs {
		if i == shown {
			labels = append(labels, fmt.Sprintf("and %d more", len(refs)-shown))
			break
		}
		labels = append(labels, r.String())
	}
	return strings.Join(labels, ", ")
}

// roleByName finds a role by the name an agent typed, or answers false.
func (a *API) roleByName(ctx context.Context, name string) (store.Role, bool, error) {
	roles, err := a.st.ListRoles(ctx)
	if err != nil {
		return store.Role{}, false, err
	}
	for _, r := range roles {
		if r.Name == name {
			return r, true, nil
		}
	}
	return store.Role{}, false, nil
}
