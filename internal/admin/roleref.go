package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// What POINTS AT a role, and how a rename follows it.
//
// A role is held by an id everywhere it is GRANTED - a group names roleIds - so
// a rename never breaks that side. What it breaks is the other one: an access
// rule names roles BY NAME (store.Access.Roles, on a route and on each endpoint
// policy), and so does a scheduled call. Those are strings written next to the
// name at a moment in time, and nothing in the database ties them back.
//
// That was the reason the catalogue stayed out of an agent's reach, and it was
// the wrong answer to the right observation: the console renames a role with
// the same consequence, so what was missing was not a closed door but this - a
// rename that follows its references, and a deletion that says what still names
// the role rather than leaving rules that grant nobody.
//
// Names are compared EXACTLY. The gateway compares them exactly too, so a rule
// naming "role_user" is not a rule naming "ROLE_USER" and must not be rewritten
// as though it were.

// roleReference is one place a role name is written, said the way an operator
// would look for it.
type roleReference struct {
	Kind  string `json:"kind"`  // route | endpoint | schedule
	ID    string `json:"id"`    // the route's or schedule's id - what get_route takes
	Label string `json:"label"` // what to look for on screen
	// Method and Path name the endpoint policy inside the route, for the two
	// kinds where "the route" is not precise enough to go and change it.
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
}

func (r roleReference) String() string { return r.Kind + " " + r.Label }

// roleReferences lists what names this role. It reads; it changes nothing.
func (a *API) roleReferences(ctx context.Context, name string) ([]roleReference, error) {
	if name == "" {
		return nil, nil
	}
	var out []roleReference
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	for _, rt := range routes {
		if namesRole(rt.Access.Roles, name) {
			out = append(out, roleReference{Kind: "route", ID: rt.ID, Label: rt.Name})
		}
		if rt.API == nil || rt.API.Security == nil {
			continue
		}
		for _, e := range rt.API.Security.Endpoints {
			if namesRole(e.Roles, name) {
				out = append(out, roleReference{Kind: "endpoint", ID: rt.ID,
					Label:  fmt.Sprintf("%s: %s %s", rt.Name, e.Method, e.Path),
					Method: e.Method, Path: e.Path})
			}
		}
	}
	schedules, err := a.st.ListSchedules(ctx, store.ScheduleFilter{})
	if err != nil {
		return nil, err
	}
	for _, s := range schedules {
		if namesRole(s.Roles, name) {
			out = append(out, roleReference{Kind: "schedule", ID: s.ID, Label: s.Name})
		}
	}
	return out, nil
}

// renameRoleReferences rewrites every rule that named from so it names to, and
// answers how many places moved. The routing is reloaded when anything did:
// a rule the gateway still holds in its compiled form would go on refusing the
// people the rename was supposed to keep.
func (a *API) renameRoleReferences(ctx context.Context, from, to string) (int, error) {
	if from == "" || to == "" || from == to {
		return 0, nil
	}
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, rt := range routes {
		touched := false
		if replaceRole(&rt.Access.Roles, from, to) {
			touched, moved = true, moved+1
		}
		if rt.API != nil && rt.API.Security != nil {
			for i := range rt.API.Security.Endpoints {
				if replaceRole(&rt.API.Security.Endpoints[i].Roles, from, to) {
					touched, moved = true, moved+1
				}
			}
		}
		if !touched {
			continue
		}
		if err := a.st.SaveRoute(ctx, rt); err != nil {
			return moved, fmt.Errorf("route %q: %w", rt.Name, err)
		}
	}
	schedules, err := a.st.ListSchedules(ctx, store.ScheduleFilter{})
	if err != nil {
		return moved, err
	}
	for _, s := range schedules {
		if !replaceRole(&s.Roles, from, to) {
			continue
		}
		moved++
		if err := a.st.UpdateSchedule(ctx, s); err != nil {
			return moved, fmt.Errorf("schedule %q: %w", s.Name, err)
		}
	}
	if moved > 0 {
		if err := a.reloadRouting(ctx); err != nil {
			return moved, fmt.Errorf("rules rewritten, but the reload failed: %w", err)
		}
	}
	return moved, nil
}

func namesRole(list []string, name string) bool {
	for _, r := range list {
		if strings.TrimSpace(r) == name {
			return true
		}
	}
	return false
}

// replaceRole rewrites the name in place and says whether anything moved.
func replaceRole(list *[]string, from, to string) bool {
	changed := false
	for i, r := range *list {
		if strings.TrimSpace(r) == from {
			(*list)[i] = to
			changed = true
		}
	}
	return changed
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
