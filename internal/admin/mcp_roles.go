package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
)

// The role catalogue, as far as an agent reaches it (RBAC-01).
//
// It was out of reach at first, for a reason worth keeping in view: every
// access rule names roles BY NAME, and so does the JWT a route forwards, so
// renaming one silently changes who reaches what. The answer is not to keep the
// catalogue shut - an application arrives with the roles it compares against,
// and somebody has to put them there - but to take the rename out of the tool's
// hands: save_role works BY NAME. A name it does not know is a new role; a name
// it knows is that role, and the only things it can change are what the role
// says about itself. Renaming stays a console act, in front of the rules that
// point at it.
//
// Granting is not here either, and that is a different reason: a role becomes
// somebody's through a group, per organisation, and who gets what is a policy
// decision rather than a task.
func (a *API) roleTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_roles", Allow: administersIdentity, Title: "List the role catalogue", ReadOnly: true,
			Description: "The global role catalogue: each role's name, what holding it also grants, " +
				"and its tags. The NAME is the contract - an access rule on a route names roles by " +
				"name, and the services behind the gateway compare those strings as they arrive. " +
				"Call this before writing a rule or adding a role: names are unique.",
			Schema: noArgs(),
			Call:   a.toolListRoles,
		},
		{
			Name: "save_role", Allow: administersIdentity, Title: "Add a role, or edit what it says about itself",
			Description: "Add a role to the catalogue, or change what it says about itself - its " +
				"description, its tags, what implies it. It works BY NAME: a name this catalogue " +
				"does not hold is a new role, a name it holds is that role. What you pass is what " +
				"changes: a field left out keeps its value. \n\nA ROLE CANNOT BE RENAMED. Its name " +
				"is its identity, and it is what the services behind this gateway read out of a " +
				"token - renaming it here would rewrite this gateway's own rules and nothing they " +
				"hold, which looks complete and is not. Changing a name is declaring the new role " +
				"and deleting the old one once the rules naming it have been changed; " +
				"list_role_references says which those are. \n\nGranting a role to somebody is a " +
				"group's business, per organisation, and no tool does it: who gets what is a " +
				"decision, not a task.",
			Schema: object(map[string]any{
				"name": str("The role's name, exactly as the rules and the services spell it, e.g. ROLE_ADMIN."),
				"grantedBy": str("The role that also grants this one, by name. A role implies its " +
					"descendants, so \"ROLE_ADMIN implies ROLE_USER\" is written on ROLE_USER as " +
					"grantedBy: ROLE_ADMIN. Empty makes it top-level."),
				"description": str("What it is for, for whoever reads the catalogue."),
				"tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
					"description": "Classification, e.g. the application the role belongs to."},
			}, "name"),
			Call: a.toolSaveRole,
		},
		{
			Name: "list_role_references", Allow: administersIdentity, Title: "Where a role is named", ReadOnly: true,
			Description: "Every rule that names this role: the routes whose access rule names it, the " +
				"per-endpoint policies inside them, and the scheduled calls that ask for it. Each " +
				"answer carries the route's id, so the next call is get_route and the one after is " +
				"save_route - which is how an agent FIXES what a role change would break, rather " +
				"than finding out from a refusal.\n\nGroups are absent on purpose: they hold roles " +
				"by id, so nothing a rename does can reach them. And a name this catalogue does NOT " +
				"hold is a fair question too - the answer is then the rules that grant nobody.",
			Schema: object(map[string]any{
				"name": str("The role's name, exactly as the rules spell it."),
			}, "name"),
			Call: a.toolRoleReferences,
		},
		{
			Name: "delete_role", Allow: administersIdentity, Title: "Remove a role from the catalogue",
			Description: "Remove a role, by name. Refused while any rule still names it - a rule " +
				"naming a role nobody holds grants nobody, and it would do so later, on a route " +
				"somebody else owns, with nothing to say why. The refusal lists what to change first.",
			Schema: object(map[string]any{
				"name": str("The role's name, as list_roles spells it."),
			}, "name"),
			Call: a.toolDeleteRole,
		},
	}
}

func (a *API) toolListRoles(ctx context.Context, _ json.RawMessage) (any, error) {
	roles, err := a.st.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	type line struct {
		Name string `json:"name"`
		// GrantedBy is the role ABOVE - and it is a name like everything else
		// here, because a role IS its name.
		GrantedBy   string   `json:"grantedBy,omitempty"`
		Description string   `json:"description,omitempty"`
		Tags        []string `json:"tags,omitempty"`
		System      bool     `json:"system,omitempty"`
	}
	out := make([]line, 0, len(roles))
	for _, r := range roles {
		out = append(out, line{
			Name: r.Name, GrantedBy: r.Parent,
			Description: r.Description, Tags: r.Tags, System: r.System,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return map[string]any{"roles": out}, nil
}

func (a *API) toolSaveRole(ctx context.Context, args json.RawMessage) (any, error) {
	// Which KEYS were passed, not only their values: "what you pass is what
	// changes" needs to tell a field left out from a field emptied on purpose -
	// and clearing grantedBy, which makes a role top-level, is a real edit.
	var given map[string]json.RawMessage
	if err := json.Unmarshal(args, &given); err != nil {
		return nil, fmt.Errorf("this is not a role: %w", err)
	}
	var in struct {
		Name        string   `json:"name"`
		GrantedBy   string   `json:"grantedBy"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("a role needs a name: it is what every access rule and every service " +
			"behind the gateway spells out, e.g. ROLE_ADMIN")
	}

	roles, err := a.st.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]store.Role, len(roles))
	for _, r := range roles {
		byName[r.Name] = r
	}
	role, existed := byName[in.Name]
	if !existed {
		role = store.Role{Name: in.Name}
	}
	if role.System {
		return nil, fmt.Errorf("role %q is one of this product's own: it is not an installation's to edit", in.Name)
	}
	if _, ok := given["description"]; ok {
		role.Description = in.Description
	}
	if _, ok := given["tags"]; ok {
		role.Tags = in.Tags
	}
	if _, ok := given["grantedBy"]; ok {
		parent := strings.TrimSpace(in.GrantedBy)
		if parent == "" {
			// Cleared on purpose: the role becomes top-level.
			role.Parent = ""
		} else {
			if _, ok := byName[parent]; !ok {
				return nil, fmt.Errorf("no role is called %q: call list_roles for the names this "+
					"catalogue holds, and remember grantedBy is the role ABOVE - the one that also "+
					"grants this one", parent)
			}
			role.Parent = parent
		}
	}
	if err := a.st.SaveRole(ctx, role); err != nil {
		return nil, err
	}
	action := "role.update"
	if !existed {
		action = "role.create"
	}
	a.auditEvent(ctx, mcpActor(ctx), action, "role", role.Name, role.Name, "", "")
	return map[string]any{
		"saved":     true,
		"created":   !existed,
		"name":      role.Name,
		"grantedBy": role.Parent,
	}, nil
}

func (a *API) toolRoleReferences(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("which role: pass its name, as list_roles spells it")
	}
	refs, err := a.roleReferences(ctx, name)
	if err != nil {
		return nil, err
	}
	_, known, err := a.roleByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if refs == nil {
		refs = []roleReference{}
	}
	return map[string]any{
		"name": name,
		// A name no catalogue entry holds, named by rules: those rules grant
		// nobody. Said here rather than left for the reader to notice.
		"inCatalogue": known,
		"references":  refs,
	}, nil
}

func (a *API) toolDeleteRole(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("which role: pass its name, as list_roles spells it")
	}
	role, found, err := a.roleByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no role is called %q", name)
	}
	// What still names it. Deleting anyway fails CLOSED - a rule naming a role
	// nobody holds grants nobody - but it fails closed later, on somebody
	// else's route, and nothing would say why.
	refs, err := a.roleReferences(ctx, role.Name)
	if err != nil {
		return nil, err
	}
	if len(refs) > 0 {
		return nil, fmt.Errorf("role %q is still named by %d rule(s): %s. Call list_role_references "+
			"for all of them with the route ids to change, or nobody will pass them",
			role.Name, len(refs), refLabels(refs, 5))
	}
	ok, err := a.st.DeleteRole(ctx, role.Name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("role %q was not deleted", role.Name)
	}
	a.auditEvent(ctx, mcpActor(ctx), "role.delete", "role", role.Name, role.Name, "", "")
	return map[string]any{"deleted": true, "name": role.Name}, nil
}
