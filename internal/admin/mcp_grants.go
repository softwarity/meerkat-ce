package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
)

// Granting, as far as an agent reaches it (RBAC-02/03).
//
// The catalogue says what a role IS; this says who holds one. The chain has
// three links - a group in an organisation names roles, a person is a member of
// that organisation, that member holds groups - and none of them was reachable,
// so an application could arrive with its roles and nobody could be given them.
// "Who gets what is a decision, not a task" was the reason, and it confused two
// things: deciding is the operator's, and they decide by SAYING it. Typing it in
// four screens afterwards is the task.
//
// Everything here works by NAME - organisations, groups, roles, people - because
// an id is this installation's own hash and an agent has no way to know one. And
// save_member is deliberately the WHOLE act: joining somebody and setting what
// they hold is one decision, and it took two endpoints in a fixed order, which
// is exactly the kind of sequence an agent gets half-right.
func (a *API) grantTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_groups", Allow: administersIdentity, Title: "List an organisation's groups", ReadOnly: true,
			Description: "The groups of one organisation and the roles each one grants, by name. A " +
				"group is how a role becomes somebody's: the catalogue holds roles, an organisation " +
				"holds groups, a member holds groups.",
			Schema: object(map[string]any{
				"tenant": str("The organisation, by name (or its id). Omit it in a single-organisation installation."),
			}),
			Call: a.toolListGroups,
		},
		{
			Name: "save_group", Allow: administersIdentity, Title: "Add a group, or change what it grants",
			Description: "Add a group to an organisation, or change its description and the roles it " +
				"grants. It works BY NAME inside that organisation: a name it does not hold is a new " +
				"group, a name it holds is that group. Roles are named as list_roles spells them - a " +
				"name nobody holds is refused rather than saved as a grant of nothing. What you pass " +
				"is what changes: a field left out keeps its value.",
			Schema: object(map[string]any{
				"tenant":      str("The organisation, by name or id."),
				"name":        str("The group's name inside that organisation."),
				"newName":     str("Rename it to this. Nothing else points at a group by name, so this moves nothing else."),
				"description": str("What it is for."),
				"roles": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
					"description": "The roles it grants, by name. Replaces the list."},
			}, "name"),
			Call: a.toolSaveGroup,
		},
		{
			Name: "delete_group", Allow: administersIdentity, Title: "Remove a group",
			Description: "Remove a group from an organisation. Refused while members still hold it - " +
				"deleting it would take their roles away without saying whose. The refusal counts them; " +
				"list_members says who.",
			Schema: object(map[string]any{
				"tenant": str("The organisation, by name or id."),
				"name":   str("The group's name."),
			}, "name"),
			Call: a.toolDeleteGroup,
		},
		{
			Name: "list_members", Allow: administersIdentity, Title: "List who is in an organisation", ReadOnly: true,
			Description: "The people in one organisation: their username, whether the membership is " +
				"enabled, whether they administer it, and the groups they hold there - which is what " +
				"decides the roles they carry INSIDE that organisation and nowhere else.",
			Schema: object(map[string]any{
				"tenant": str("The organisation, by name or id."),
			}),
			Call: a.toolListMembers,
		},
		{
			Name: "save_member", Allow: administersIdentity, Title: "Put somebody in an organisation, with what they hold",
			Description: "Join somebody to an organisation and set the groups they hold there, in one " +
				"call: the account must exist, the membership is created if it is missing, and the " +
				"groups REPLACE what they held. This is one decision - who holds what, where - and it " +
				"took two endpoints in a fixed order, which is how a half-done grant happens. Roles " +
				"are not set here: a group grants them, so that a second person gets the same thing by " +
				"holding the same group.",
			Schema: object(map[string]any{
				"tenant":   str("The organisation, by name or id."),
				"username": str("The account, by username."),
				"groups": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
					"description": "The groups they hold there, by name. Replaces what they held; an empty list takes them all away and keeps the membership."},
				"admin":   map[string]any{"type": "boolean", "description": "They administer this organisation. Left out, an existing membership keeps what it had and a new one is an ordinary member."},
				"enabled": map[string]any{"type": "boolean", "description": "The membership itself. Left out, a new one is enabled."},
			}, "username"),
			Call: a.toolSaveMember,
		},
	}
}

// tenantByName resolves what an agent typed - a name, an id, or nothing at all
// in an installation with one organisation.
func (a *API) tenantByName(ctx context.Context, given string) (store.Tenant, error) {
	tenants, err := a.st.ListTenants(ctx)
	if err != nil {
		return store.Tenant{}, err
	}
	given = strings.TrimSpace(given)
	if given == "" {
		if len(tenants) == 1 {
			return tenants[0], nil
		}
		names := make([]string, 0, len(tenants))
		for _, t := range tenants {
			names = append(names, t.Name)
		}
		return store.Tenant{}, fmt.Errorf("which organisation: this installation has %d (%s)",
			len(tenants), strings.Join(names, ", "))
	}
	for _, t := range tenants {
		if t.Name == given || t.ID == given {
			return t, nil
		}
	}
	names := make([]string, 0, len(tenants))
	for _, t := range tenants {
		names = append(names, t.Name)
	}
	return store.Tenant{}, fmt.Errorf("no organisation is called %q: this installation has %s",
		given, strings.Join(names, ", "))
}

func (a *API) toolListGroups(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Tenant string `json:"tenant"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	tenant, err := a.tenantByName(ctx, in.Tenant)
	if err != nil {
		return nil, err
	}
	groups, err := a.st.ListGroups(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	roles, err := a.st.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	name := map[string]string{}
	for _, r := range roles {
		name[r.ID] = r.Name
	}
	type line struct {
		Name        string   `json:"name"`
		Description string   `json:"description,omitempty"`
		Roles       []string `json:"roles"`
	}
	out := make([]line, 0, len(groups))
	for _, g := range groups {
		rs := make([]string, 0, len(g.RoleIDs))
		for _, id := range g.RoleIDs {
			if n := name[id]; n != "" {
				rs = append(rs, n)
			}
		}
		sort.Strings(rs)
		out = append(out, line{Name: g.Name, Description: g.Description, Roles: rs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return map[string]any{"tenant": tenant.Name, "groups": out}, nil
}

func (a *API) toolSaveGroup(ctx context.Context, args json.RawMessage) (any, error) {
	var given map[string]json.RawMessage
	if err := json.Unmarshal(args, &given); err != nil {
		return nil, fmt.Errorf("this is not a group: %w", err)
	}
	var in struct {
		Tenant      string   `json:"tenant"`
		Name        string   `json:"name"`
		NewName     string   `json:"newName"`
		Description string   `json:"description"`
		Roles       []string `json:"roles"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	tenant, err := a.tenantByName(ctx, in.Tenant)
	if err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("a group needs a name")
	}
	groups, err := a.st.ListGroups(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	var group store.Group
	existed := false
	for _, g := range groups {
		if g.Name == in.Name {
			group, existed = g, true
			break
		}
	}
	if !existed {
		group = store.Group{ID: newID(), TenantID: tenant.ID, Name: in.Name}
	}
	if _, ok := given["description"]; ok {
		group.Description = in.Description
	}
	if _, ok := given["roles"]; ok {
		roles, err := a.st.ListRoles(ctx)
		if err != nil {
			return nil, err
		}
		byName := map[string]string{}
		for _, r := range roles {
			byName[r.Name] = r.ID
		}
		ids := make([]string, 0, len(in.Roles))
		for _, want := range in.Roles {
			id, ok := byName[strings.TrimSpace(want)]
			if !ok {
				return nil, fmt.Errorf("no role is called %q: call list_roles for the catalogue, "+
					"or save_role to add it - a group granting a role nobody holds grants nothing", want)
			}
			ids = append(ids, id)
		}
		group.RoleIDs = ids
	}
	if _, ok := given["newName"]; ok {
		next := strings.TrimSpace(in.NewName)
		if next == "" {
			return nil, fmt.Errorf("newName is empty")
		}
		if !existed {
			return nil, fmt.Errorf("no group is called %q here, so there is nothing to rename", in.Name)
		}
		group.Name = next
	}
	if err := a.st.SaveGroup(ctx, group); err != nil {
		return nil, err
	}
	action := "group.update"
	if !existed {
		action = "group.create"
	}
	a.auditEvent(ctx, mcpActor(ctx), action, "group", group.ID, group.Name, tenant.ID, "")
	return map[string]any{
		"saved": true, "created": !existed,
		"tenant": tenant.Name, "name": group.Name, "roles": len(group.RoleIDs),
	}, nil
}

func (a *API) toolDeleteGroup(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Tenant string `json:"tenant"`
		Name   string `json:"name"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	tenant, err := a.tenantByName(ctx, in.Tenant)
	if err != nil {
		return nil, err
	}
	groups, err := a.st.ListGroups(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	var group store.Group
	for _, g := range groups {
		if g.Name == strings.TrimSpace(in.Name) {
			group = g
			break
		}
	}
	if group.ID == "" {
		return nil, fmt.Errorf("no group is called %q in %s", in.Name, tenant.Name)
	}
	members, err := a.st.ListMembers(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	holders := 0
	for _, m := range members {
		ids, err := a.st.MemberGroupIDs(ctx, tenant.ID, m.UserID)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if id == group.ID {
				holders++
			}
		}
	}
	if holders > 0 {
		return nil, fmt.Errorf("group %q is held by %d member(s): deleting it takes their roles away "+
			"without saying whose. Call list_members to see them, and save_member to move them first",
			group.Name, holders)
	}
	if _, err := a.st.DeleteGroup(ctx, group.ID); err != nil {
		return nil, err
	}
	a.auditEvent(ctx, mcpActor(ctx), "group.delete", "group", group.ID, group.Name, tenant.ID, "")
	return map[string]any{"deleted": true, "tenant": tenant.Name, "name": group.Name}, nil
}

func (a *API) toolListMembers(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Tenant string `json:"tenant"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	tenant, err := a.tenantByName(ctx, in.Tenant)
	if err != nil {
		return nil, err
	}
	members, err := a.st.ListMembers(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	type line struct {
		Username string   `json:"username"`
		Fullname string   `json:"fullname,omitempty"`
		Enabled  bool     `json:"enabled"`
		Admin    bool     `json:"admin,omitempty"`
		Groups   []string `json:"groups"`
	}
	out := make([]line, 0, len(members))
	for _, m := range members {
		groups, err := a.st.MemberGroups(ctx, tenant.ID, m.UserID)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(groups))
		for _, g := range groups {
			names = append(names, g.Name)
		}
		sort.Strings(names)
		out = append(out, line{
			Username: m.Username, Fullname: m.Fullname, Enabled: m.Enabled,
			Admin: m.Type == store.MemberAdmin, Groups: names,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return map[string]any{"tenant": tenant.Name, "members": out}, nil
}

func (a *API) toolSaveMember(ctx context.Context, args json.RawMessage) (any, error) {
	var given map[string]json.RawMessage
	if err := json.Unmarshal(args, &given); err != nil {
		return nil, fmt.Errorf("this is not a membership: %w", err)
	}
	var in struct {
		Tenant   string   `json:"tenant"`
		Username string   `json:"username"`
		Groups   []string `json:"groups"`
		Admin    bool     `json:"admin"`
		Enabled  bool     `json:"enabled"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	tenant, err := a.tenantByName(ctx, in.Tenant)
	if err != nil {
		return nil, err
	}
	user, err := a.st.GetUserByUsername(ctx, strings.TrimSpace(in.Username))
	if err != nil {
		return nil, fmt.Errorf("no account is called %q: call list_users, and create the account in "+
			"the console - an agent does not mint credentials", in.Username)
	}

	// The membership first: holding a group in an organisation one does not
	// belong to grants nothing, and that is the half-done grant this call
	// exists to prevent.
	m, joined := store.Membership{UserID: user.ID, TenantID: tenant.ID, Type: store.MemberUser, Enabled: true}, true
	if prev, err := a.st.GetMembership(ctx, user.ID, tenant.ID); err == nil {
		m, joined = prev, false
	}
	if _, ok := given["admin"]; ok {
		m.Type = store.MemberUser
		if in.Admin {
			m.Type = store.MemberAdmin
		}
	}
	if _, ok := given["enabled"]; ok {
		m.Enabled = in.Enabled
	}
	if err := a.st.SaveMembership(ctx, m); err != nil {
		return nil, err
	}

	names := []string{}
	if _, ok := given["groups"]; ok {
		groups, err := a.st.ListGroups(ctx, tenant.ID)
		if err != nil {
			return nil, err
		}
		byName := map[string]store.Group{}
		for _, g := range groups {
			byName[g.Name] = g
		}
		ids := make([]string, 0, len(in.Groups))
		for _, want := range in.Groups {
			g, ok := byName[strings.TrimSpace(want)]
			if !ok {
				return nil, fmt.Errorf("no group is called %q in %s: call list_groups, or save_group "+
					"to add it - the membership is saved, the groups are not", want, tenant.Name)
			}
			ids = append(ids, g.ID)
			names = append(names, g.Name)
		}
		if err := a.st.SetMemberGroups(ctx, tenant.ID, user.ID, ids); err != nil {
			return nil, err
		}
	} else {
		held, err := a.st.MemberGroups(ctx, tenant.ID, user.ID)
		if err != nil {
			return nil, err
		}
		for _, g := range held {
			names = append(names, g.Name)
		}
	}
	sort.Strings(names)

	action := "member.update"
	if joined {
		action = "member.join"
	}
	a.auditEvent(ctx, mcpActor(ctx), action, "membership", user.ID, user.Username, tenant.ID,
		strings.Join(names, ", "))
	// What they now CARRY, which is the question behind the call: a group is
	// the mechanism, the effective roles are the answer.
	effective, err := a.st.EffectiveRoleNames(ctx, memberGroupIDs(ctx, a, tenant.ID, user.ID))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"saved": true, "joined": joined,
		"tenant": tenant.Name, "username": user.Username,
		"admin": m.Type == store.MemberAdmin, "enabled": m.Enabled,
		"groups": names, "roles": effective,
	}, nil
}

func memberGroupIDs(ctx context.Context, a *API, tenantID, userID string) []string {
	ids, err := a.st.MemberGroupIDs(ctx, tenantID, userID)
	if err != nil {
		return nil
	}
	return ids
}

// Accounts, as far as an agent reaches them.
//
// The first credential is the whole question. An agent must not choose a
// password - what it writes it also remembers, in a transcript nobody treats as
// a vault - so it does not: the gateway draws one, marks it TO BE CHANGED at
// the first sign-in, and hands it back once. That is the console's own flow
// (writeResetPassword), and the flag is what bounds it: the secret in the
// answer stops working the moment the person uses it.
//
// Administrative capabilities are NOT here. Creating an account is one act;
// making it an administrator of this gateway is another, and an agent holding a
// control-plane token that mints itself a peer is how a perimeter stops meaning
// anything. Membership of an organisation, and whether that membership
// administers it, is save_member's business.
func (a *API) accountTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "save_user", Allow: administersIdentity, Title: "Add an account, or change what it says",
			Description: "Add a local account, or change its full name, e-mail, language or whether it " +
				"is enabled. It works BY USERNAME: a username this installation does not hold is a " +
				"new account, one it holds is that account.\n\nThe first credential: a NEW account is " +
				"given a password drawn by the gateway and marked to be changed at the first " +
				"sign-in, and the answer carries it once - so it lands in this conversation, which " +
				"is why it only works until the person changes it. Pass password: \"temporary\" on " +
				"an existing account to issue a new one the same way. An agent never chooses a " +
				"password.\n\nAdministrative capabilities are not set here, and neither is what the " +
				"account holds in an organisation - that is save_member.",
			Schema: object(map[string]any{
				"username": str("The account name, as the person will type it."),
				"fullname": str("Their name, for the pages that greet them."),
				"email":    str("Their address, which is what a password reset needs."),
				"locale":   str("The language their pages are served in, e.g. fr."),
				"enabled":  map[string]any{"type": "boolean", "description": "A disabled account cannot sign in. New accounts are enabled."},
				"password": str("Only value: \"temporary\" - issue a new password, to be changed at the next sign-in. A new account always gets one."),
			}, "username"),
			Call: a.toolSaveUser,
		},
	}
}

func (a *API) toolSaveUser(ctx context.Context, args json.RawMessage) (any, error) {
	var given map[string]json.RawMessage
	if err := json.Unmarshal(args, &given); err != nil {
		return nil, fmt.Errorf("this is not an account: %w", err)
	}
	var in struct {
		Username string `json:"username"`
		Fullname string `json:"fullname"`
		Email    string `json:"email"`
		Locale   string `json:"locale"`
		Enabled  bool   `json:"enabled"`
		Password string `json:"password"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" {
		return nil, fmt.Errorf("an account needs a username")
	}
	if p := strings.TrimSpace(in.Password); p != "" && p != "temporary" {
		return nil, fmt.Errorf("password only takes \"temporary\": an agent does not choose one - " +
			"the gateway draws it and marks it to be changed at the first sign-in")
	}

	user, err := a.st.GetUserByUsername(ctx, in.Username)
	created := err != nil
	if created {
		user = store.User{ID: newID(), Username: in.Username, Enabled: true}
	}
	if user.Root {
		return nil, fmt.Errorf("account %q is root: an agent does not edit the account that "+
			"administers this gateway", user.Username)
	}
	if _, ok := given["fullname"]; ok {
		user.Fullname = strings.TrimSpace(in.Fullname)
	}
	if _, ok := given["email"]; ok {
		user.Email = strings.TrimSpace(in.Email)
	}
	if _, ok := given["locale"]; ok {
		user.Locale = strings.TrimSpace(in.Locale)
	}
	if _, ok := given["enabled"]; ok {
		user.Enabled = in.Enabled
	}

	if created {
		if err := a.st.CreateUser(ctx, user); err != nil {
			return nil, err
		}
	} else if err := a.st.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	out := map[string]any{
		"saved": true, "created": created, "username": user.Username, "enabled": user.Enabled,
	}
	// The first credential, or a new one on request. Drawn here, never chosen:
	// see the comment above accountTools.
	if created || strings.TrimSpace(in.Password) == "temporary" {
		password, err := randomSecret()
		if err != nil {
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		if err := a.st.SetUserPassword(ctx, user.ID, string(hash), true); err != nil {
			return nil, err
		}
		a.auditEvent(ctx, mcpActor(ctx), "user.reset-password", "user", user.ID, user.Username, "",
			"temporary password issued")
		out["password"] = password
		out["passwordMustChange"] = true
		out["note"] = "Give this to them by a channel you trust. It works once: the next sign-in asks for a new one."
	}
	action := "user.update"
	if created {
		action = "user.create"
	}
	a.auditEvent(ctx, mcpActor(ctx), action, "user", user.ID, user.Username, "", "")
	return out, nil
}
