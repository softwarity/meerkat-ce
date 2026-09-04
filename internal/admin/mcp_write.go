package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// What an agent may CHANGE, and it changes it for real.
//
// The first cut made every write go through a draft that a person then
// activated in the console. That was a ceremony nobody asked for: an
// administrator who connects an agent wants the work done, not a second screen
// to visit - no more than a GitHub agent asks you to go and confirm the pull
// request it just opened. The safety is elsewhere, and it was already there:
//
//   - every successful change on this plane records a RESTORE POINT (CFG-06),
//     automatically, labelled with the words the audit trail just wrote, so
//     going back is a click and needs nobody's foresight;
//   - the audit names the acting TOKEN beside the account (MCP-03);
//   - the token's perimeter decides what is even offered (MCP-02).
//
// What a careful administrator still does is ask, in words, for a named
// snapshot before a big change - which is a TOOL (save_configuration), not a
// rite. The difference matters: a rite is paid by everyone on every change,
// and a tool is paid by whoever wants it.
//
// A route is applied the moment it is saved, through the same function the
// console's own endpoint calls. There is no second set of rules about what a
// route may be, so the agent cannot store what the console would have refused.

func (a *API) writeTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_route_bricks", Allow: administersRouting, Title: "List predicates and filters", ReadOnly: true,
			Description: "The vocabulary a route is written in: every predicate (what a route matches on) and " +
				"every filter (what it does to the request, the response, or instead of them), with their " +
				"parameters. Read this BEFORE writing a route - the types are a closed list, and one that " +
				"does not exist is refused at save time.",
			Schema: object(map[string]any{
				"kind": str("Optional: 'predicate' or 'filter' to halve the answer."),
				"type": str("Optional: one brick's name, for its parameters alone."),
			}),
			Call: a.toolRouteBricks,
		},
		{
			Name: "save_route", Allow: administersRouting, Title: "Create or update a route",
			Description: "Write a route and apply it. It takes effect immediately - the gateway reloads - and " +
				"a restore point is recorded, so the change can be undone from the console. " +
				"The shape is the one get_route returns, so the way to change a route is to read it, change " +
				"the field, and send it back whole: what you leave out is removed. " +
				"Call list_route_bricks first if you are composing predicates or filters.",
			Schema: routeSchema(),
			Call:   a.toolSaveRoute,
		},
		{
			Name: "delete_route", Allow: administersRouting, Title: "Delete a route",
			Description: "Remove a route and apply it. It stops answering immediately. A restore point is " +
				"recorded first, so this is undoable from the console.",
			Schema: object(map[string]any{
				"id": str("The route id, as given by list_routes."),
			}, "id"),
			Call: a.toolDeleteRoute,
		},
		{
			Name: "save_configuration", Allow: isRoot, Title: "Save the current state under a name",
			Description: "Take the whole state of the gateway as it is right now - routes, roles, " +
				"organisations, settings - and keep it under a name. It changes nothing: it is a copy, " +
				"kept beside the running gateway. " +
				"This is what to do when someone says 'save the current configuration before you start': " +
				"it gives a named point to come back to, on top of the automatic restore points.",
			Schema: object(map[string]any{
				"name":        str("What to call it, e.g. 'before the billing route'."),
				"description": str("Optional: a line about why it was taken."),
			}, "name"),
			Call: a.toolSaveConfiguration,
		},
		{
			Name: "list_configurations", Allow: isRoot, Title: "List the saved configurations", ReadOnly: true,
			Description: "The configurations saved under a name, and whether what the gateway runs right now " +
				"still matches one of them. Activating one is done by a person in the console - it replaces " +
				"the whole state, which is a different act from the changes made here.",
			Schema: noArgs(),
			Call:   a.toolListConfigurations,
		},
	}
}

// routeSchema describes a route to a model. Deliberately not exhaustive: the
// full object has thirty fields and get_route hands them over already shaped.
// What is spelled out is what an agent composes from a sentence.
func routeSchema() map[string]any {
	brick := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"type": str("The brick's name, from list_route_bricks."),
				"args": map[string]any{"type": "object", "description": "Its parameters."},
			},
			"required": []any{"type"},
		},
	}
	s := object(map[string]any{
		"id":   str("A stable identifier, lower-case with dashes. An existing one updates that route."),
		"name": str("What a human calls it."),
		"order": map[string]any{"type": "integer",
			"description": "Matching order, ascending. The first route that matches wins, so a catch-all sits last."},
		"enabled":    map[string]any{"type": "boolean", "description": "Absent or false means the route is not served."},
		"upstream":   str("The service to proxy to, e.g. http://billing.svc:8080. Leave empty when a terminal filter answers instead."),
		"predicates": brick,
		"filters":    brick,
		"isUi": map[string]any{"type": "boolean",
			"description": "This route serves pages to a browser rather than an API. It is what turns on the " +
				"injected user button, the page decorations and the per-route CSS."},
		"access": map[string]any{"type": "object",
			"description": "Who may reach it: level 'auth' (signed in), 'tenant' (with an organisation chosen), " +
				"'tenants' with a list, 'deny', or absent to let the upstream decide. 'roles' and 'users' refine it."},
	}, "id", "name", "predicates")
	// The rest of a route (identity forwarding, UI options, locales, the
	// OpenAPI declaration, per-endpoint security) travels unchanged on a
	// read-modify-write, which is why unknown fields are welcome here.
	s["additionalProperties"] = true
	return s
}

func (a *API) toolRouteBricks(_ context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Kind string `json:"kind"`
		Type string `json:"type"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	var out []routing.CatalogEntry
	for _, entry := range routing.Catalog() {
		if in.Kind != "" && !strings.EqualFold(entry.Kind, in.Kind) {
			continue
		}
		if in.Type != "" && !strings.EqualFold(entry.Type, in.Type) {
			continue
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no brick matches kind=%q type=%q: call this tool with no arguments for the whole list", in.Kind, in.Type)
	}
	return map[string]any{"bricks": out}, nil
}

func (a *API) toolSaveRoute(ctx context.Context, args json.RawMessage) (any, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("nothing to save: pass the route")
	}
	var route store.Route
	// NOT DisallowUnknownFields, unlike the arguments of every other tool: a
	// route read by get_route and handed back carries fields this schema does
	// not spell out, and refusing them would make read-modify-write - the way
	// to change one thing - impossible.
	if err := json.Unmarshal(args, &route); err != nil {
		return nil, fmt.Errorf("this is not a route: %w", err)
	}
	existed := false
	if _, err := a.st.GetRoute(ctx, route.ID); err == nil {
		existed = true
	}
	saved, err := a.saveRoute(ctx, mcpActor(ctx), route)
	if err != nil {
		return nil, err
	}
	verb := "created"
	if existed {
		verb = "updated"
	}
	return map[string]any{
		"route": saved, "applied": true, "what": verb,
		"note": "Live now, and a restore point was recorded: this can be undone from the console.",
	}, nil
}

func (a *API) toolDeleteRoute(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, fmt.Errorf("which route: pass the id given by list_routes")
	}
	if err := a.dropRoute(ctx, mcpActor(ctx), in.ID); err != nil {
		return nil, err
	}
	return map[string]any{
		"deleted": in.ID, "applied": true,
		"note": "Gone now, and a restore point was recorded: this can be undone from the console.",
	}, nil
}

func (a *API) toolSaveConfiguration(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	if err := a.roomForAnother(ctx); err != nil {
		return nil, err
	}
	name, err := store.SanitizeConfigurationName(in.Name)
	if err != nil {
		return nil, err
	}
	taken, err := a.st.ConfigurationNameTaken(ctx, name, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, fmt.Errorf("a configuration called %s already exists: pick another name", name)
	}
	file, err := a.liveDocumentText(ctx)
	if err != nil {
		return nil, err
	}
	c := store.Configuration{ID: newID(), Name: name, Description: in.Description, Document: file}
	if err := a.st.SaveConfiguration(ctx, &c); err != nil {
		return nil, err
	}
	// It becomes the current one, because it IS: naming the running state is
	// what says "what this gateway serves is called that". Nothing is applied.
	if err := a.st.MarkConfigurationActive(ctx, c.ID); err != nil {
		return nil, err
	}
	a.auditEvent(ctx, mcpActor(ctx), "configuration.create", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("captured from the running gateway, %d bytes", len(file)))
	return map[string]any{
		"name": c.Name, "bytes": len(file),
		"note": "A copy of the gateway as it is now. Nothing changed; it is a point to come back to.",
	}, nil
}

func (a *API) toolListConfigurations(ctx context.Context, _ json.RawMessage) (any, error) {
	list, err := a.st.ListConfigurations(ctx)
	if err != nil {
		return nil, err
	}
	running, err := a.liveDocumentText(ctx)
	if err != nil {
		return nil, err
	}
	digest := store.DigestOf(running)
	type line struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Active      bool   `json:"active"`
		// MatchesRunning is the honest form of "saved": a configuration saved
		// yesterday and a route added since are two different states, and only
		// the two fingerprints can say so.
		MatchesRunning bool `json:"matchesRunning"`
	}
	out := make([]line, 0, len(list))
	for _, c := range list {
		out = append(out, line{
			Name: c.Name, Description: c.Description, Active: c.Active,
			MatchesRunning: c.Digest == digest,
		})
	}
	return map[string]any{"configurations": out}, nil
}
