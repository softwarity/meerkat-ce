package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/config"
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
			Name: "write_language", Allow: administersIdentity, Title: "Write a language",
			Description: "Write wordings into one language, creating it if the product does not ship it. " +
				"Pass entries as key to wording, from read_language's keys - a key the catalogue does " +
				"not have is refused by name rather than stored where nothing would read it. " +
				"By default what is passed is merged into what is there; replace: true makes this the " +
				"whole language. A wording equal to the shipped one is not stored, so that string keeps " +
				"following later fixes. " +
				"THE WHOLE LANGUAGE IS THE POINT: nobody writes two hundred and seventy strings by hand, " +
				"and the console's editor then shows each one beside the screen that renders it, for a " +
				"person who speaks the language to review what you wrote.",
			Schema: object(map[string]any{
				"code":    str("A BCP 47 tag: fr, pt-BR, zh-Hans."),
				"entries": map[string]any{"type": "object", "description": "The wordings, key to text, from read_language's keys."},
				"replace": map[string]any{"type": "boolean", "description": "Replace the language entirely rather than merging into it."},
			}, "code", "entries"),
			Call: a.toolWriteLanguage,
		},
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
				"tenants, settings - and keep it under a name. It changes nothing: it is a copy, " +
				"kept beside the running gateway. " +
				"This is what to do when someone says 'save the current configuration before you start': " +
				"it gives a named point to come back to, on top of the automatic restore points. " +
				"A name already taken is refused, unless replace is true: that configuration is then " +
				"refreshed from the running state - the way to bring a saved configuration up to date " +
				"before push_configuration.",
			Schema: object(map[string]any{
				"name":        str("What to call it, e.g. 'before the billing route'."),
				"description": str("Optional: a line about why it was taken."),
				"replace": map[string]any{"type": "boolean",
					"description": "True to refresh the configuration of that name from the running state, " +
						"replacing what it held. Its git binding is kept."},
			}, "name"),
			Call: a.toolSaveConfiguration,
		},
		{
			Name: "list_configurations", Allow: isRoot, Title: "List the saved configurations", ReadOnly: true,
			Description: "The configurations saved under a name, and whether what the gateway runs right now " +
				"still matches one of them. Use activate_configuration to switch to one.",
			Schema: noArgs(),
			Call:   a.toolListConfigurations,
		},
		{
			Name: "activate_configuration", Allow: isRoot, Title: "Switch to a saved configuration",
			Description: "Make a saved configuration the running one, by name (list_configurations gives the " +
				"names). It replaces the WHOLE state - what the configuration does not carry is removed, " +
				"which is what makes it a switch rather than an import - and it applies at once. " +
				"Take a snapshot first with save_configuration if the current state is worth keeping: " +
				"a restore point is recorded either way, but a named one is easier to ask for later.",
			Schema: object(map[string]any{
				"name": str("The configuration's name, as list_configurations gives it."),
			}, "name"),
			Call: a.toolActivateConfiguration,
		},
		{
			Name: "import_configuration", Allow: isRoot, Title: "Apply a configuration document",
			Description: "Apply a configuration document - the YAML or JSON export_configuration answers, or " +
				"a file somebody wrote - to this gateway. This is how an installation is set up from a " +
				"file: routes, roles, tenants, groups, authorities, OIDC clients, themes, settings and " +
				"the branding all at once. An OIDC client carrying a secretHash keeps that secret; one without " +
				"keeps the secret it has here, or is created with none until a person draws one. " +
				"\n\nBY DEFAULT IT MERGES: what the document does not mention is left alone, so an import " +
				"cannot quietly delete a route it never knew about. prune: true aligns the gateway ON the " +
				"document instead - anything absent from a section the document carries is removed. " +
				"\n\nAsk for dryRun: true first when the document did not come from this gateway: the answer " +
				"is the same plan, listing what would be added, updated and removed, and nothing is touched. " +
				"\n\nTwo things a document never carries: accounts, which hold credentials, and the VALUES " +
				"behind vault references - an entry the document names but this gateway does not hold is " +
				"reserved empty and reported in `missing`, for a person to fill. Pictures ride as data URIs " +
				"when the document carries them; the console's package (a ZIP) is the form for those, and " +
				"it is a person's path rather than a tool's.",
			Schema: object(map[string]any{
				"document": str("The configuration, as YAML or JSON."),
				"prune": map[string]any{"type": "boolean",
					"description": "Remove what the document does not mention, section by section. Default false (merge)."},
				"dryRun": map[string]any{"type": "boolean",
					"description": "Report what it WOULD do and change nothing. Default false."},
			}, "document"),
			Call: a.toolImportConfiguration,
		},
		{
			Name: "compose_configuration", Allow: isRoot, Title: "Compose one configuration from two",
			Description: "Build one configuration out of two documents and a recipe of choices, then " +
				"preview it, apply it or save it. The LEFT side is where the result starts (usually " +
				"what runs); the RIGHT side is what it is compared with (a saved configuration, a restore " +
				"point, a file). Nothing moves without a pick: a right-only object is in the result only " +
				"when picked, an object on both sides keeps the left's fields unless a pick says otherwise. " +
				"\n\nA pick names a path and what to do at it: `route:r-x` with from left|right|none (the " +
				"object whole, or out of the result); `route:r-x/upstream` with from left|right|unset (one " +
				"field, nested keys with dots); `route:r-x/access.roles[ROLE_Y]` with op add|drop (one " +
				"element of a list; an object element is named by its canonical JSON); `route:r-x/predicates` " +
				"with order (the element keys in the wanted order). A setting is `setting:tls/names[...]`, " +
				"the relay is `mailRelay/host`, a role is `role:ROLE_X`. A pair says that right object " +
				"`r-9f0` IS left object `r-123` under another id: the result keeps the left id. " +
				"\n\nWhat the result names and does not hold is PULLED from the side that has it (a route " +
				"taken from the right brings its roles) and reported in `pulled`; a reference nobody " +
				"declares is reported in `missing`, and only a hard one (a role's parent, a group's tenant) " +
				"blocks applying. Paths that resolve nowhere are listed in `unresolved`, never passed " +
				"under silence. Start with then: preview - it answers the object-level comparison of the " +
				"sides, the report, and the plan of applying the result to this gateway, and changes " +
				"nothing. A git location is not a source here: pull_configuration keeps it as a saved " +
				"configuration, which is then a side like any other; to send a result to git, then: save " +
				"and push_configuration.",
			Schema: object(map[string]any{
				"left":  composeSourceSchema("The side the result starts from."),
				"right": composeSourceSchema("The side it is compared with."),
				"pairs": map[string]any{"type": "array", "description": "Right objects that ARE left objects under another id.",
					"items": object(map[string]any{
						"kind":  str("route, tenant, group, authProvider or theme."),
						"left":  str("The left object's id, kept in the result."),
						"right": str("The right object's id."),
					}, "kind", "left", "right")},
				"picks": map[string]any{"type": "array", "description": "The choices, path by path.",
					"items": object(map[string]any{
						"path": str("kind:id, kind:id/field.path, kind:id/field.path[element] or mailRelay/field."),
						"from": str("left, right, none or unset - for an object or a field."),
						"op":   str("add or drop - for one element of a list."),
						"order": map[string]any{"type": "array", "items": map[string]any{"type": "string"},
							"description": "The element keys of a list, in the wanted order."},
					}, "path")},
				"then": str("preview (default: say what it would do, change nothing), apply (make the result " +
					"the running configuration, a switch), or save (keep it under `name`, applied to nothing)."),
				"name": str("What to call the result when then is save."),
				"replace": map[string]any{"type": "boolean",
					"description": "With then: save, true to replace the document of the configuration of that " +
						"name; refused otherwise when the name is taken. Its git binding is kept."},
			}, "left", "right"),
			Call: a.toolComposeConfiguration,
		},
	}
}

// composeSourceSchema describes one side of a composition to a model.
func composeSourceSchema(desc string) map[string]any {
	s := object(map[string]any{
		"source":   str("running (what the gateway serves), configuration (a saved one, by id), point (a restore point, by id), or file (the document itself)."),
		"id":       str("The configuration's or the restore point's id."),
		"document": str("The file, as YAML or JSON, when source is file."),
		"package":  str("Instead of document: a package (ZIP), in base64, when source is file."),
	}, "source")
	s["description"] = desc
	return s
}

// toolComposeConfiguration composes two documents and previews, applies or
// saves the result: the console's compose endpoints, for an agent.
func (a *API) toolComposeConfiguration(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Left  composeSource `json:"left"`
		Right composeSource `json:"right"`
		config.Recipe
		Then    string `json:"then"`
		Name    string `json:"name"`
		Replace bool   `json:"replace"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	if in.Left.Source == "remote" || in.Right.Source == "remote" {
		return nil, fmt.Errorf("a git location is not a source here: pull_configuration puts it on the shelf, " +
			"then name that configuration")
	}
	left, lname, _, err := a.sideDocument(ctx, in.Left, "left")
	if err != nil {
		return nil, err
	}
	right, rname, _, err := a.sideDocument(ctx, in.Right, "right")
	if err != nil {
		return nil, err
	}
	result, report, err := config.Compose(left, right, in.Recipe)
	if err != nil {
		return nil, err
	}
	c := &composed{left: left, right: right, lname: lname, rname: rname, result: result, report: report}
	c.body.Recipe, c.body.Name = in.Recipe, in.Name
	out := map[string]any{
		"changes": config.Compare(left, right),
		"report":  report,
	}
	switch in.Then {
	case "", "preview":
		if plan, err := config.PreviewSwitch(ctx, a.st, result); err != nil {
			out["planError"] = err.Error()
		} else {
			out["plan"] = plan
		}
		out["note"] = "Nothing was applied: this is what the result would do to this gateway."
		return out, nil
	case "apply":
		if err := report.Blocked(); err != nil {
			return nil, err
		}
		preview, err := config.PreviewSwitch(ctx, a.st, result)
		if err != nil {
			return nil, err
		}
		if err := a.agentMayApply(preview); err != nil {
			return nil, err
		}
		plan, err := config.Switch(ctx, a.st, result)
		if err != nil {
			return nil, err
		}
		a.sm.TokenChanged("*")
		a.auditEvent(ctx, mcpActor(ctx), "configuration.compose", "config", "", "", "",
			c.describe()+"; "+summarise(plan))
		if err := a.reloadRouting(ctx); err != nil {
			return nil, fmt.Errorf("applied, but the routing table could not be reloaded: %w", err)
		}
		out["plan"] = plan
		out["note"] = "This gateway now serves the composed configuration. It is applied, not staged."
		return out, nil
	case "save":
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
		if taken && !in.Replace {
			return nil, fmt.Errorf("a configuration called %s already exists: pass replace: true to "+
				"replace its document", name)
		}
		file, err := config.Marshal(result)
		if err != nil {
			return nil, err
		}
		saved := store.Configuration{ID: newID(), Name: name}
		if taken {
			named, err := a.configurationByName(ctx, name)
			if err != nil {
				return nil, err
			}
			if saved, err = a.st.GetConfiguration(ctx, named.ID); err != nil {
				return nil, err
			}
		}
		saved.Document = string(file)
		if err := a.st.SaveConfiguration(ctx, &saved); err != nil {
			return nil, err
		}
		a.auditEvent(ctx, mcpActor(ctx), "configuration.create", "configuration", saved.ID, saved.Name, "",
			fmt.Sprintf("%s, %d bytes, applied to nothing", c.describe(), len(file)))
		out["saved"] = saved.Name
		out["note"] = "Saved on the shelf, applied to nothing: activate_configuration makes it current."
		return out, nil
	}
	return nil, fmt.Errorf("then is one of preview, apply or save, not %q", in.Then)
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
			"description": "Who may reach it: level 'auth' (signed in), 'tenant' (with a tenant chosen), " +
				"'tenants' with a list, 'deny', or absent to let the upstream decide. 'roles' and 'users' refine it."},
		// The nested objects are DECLARED, with their shape left to get_route.
		// They were left out on the grounds that a read-modify-write carries
		// them unchanged and that additionalProperties welcomes them - and that
		// was wrong in the one way that matters: an undeclared object reaches
		// the server as a STRING, and the save is refused with "cannot
		// unmarshal string into Go struct field Route.identity". So get_route
		// could render them and save_route could not take them: every one of
		// these settings was readable and unwritable by an agent. Naming the
		// TYPE is what makes the value survive the trip; naming every field
		// inside it is still not this schema's job.
		"identity": map[string]any{"type": "object",
			"description": "What the route forwards about the caller to the service: mechanism " +
				"('headers', 'jwt', 'signed-jwt', 'oidc'), the attributes, ttl, and algorithm for signed-jwt " +
				"(ES256 by default, or EdDSA, RS256). 'audience' is the aud of the jwt and signed-jwt " +
				"tokens, the route's name when absent (any text up to 255 characters). 'issuer' replaces the iss of " +
				"the jwt, signed-jwt and oidc tokens; absent, the gateway's internal issuer (the internal plane's full " +
				"name, what the internal discovery names), or the public one for an oidc token asked on the " +
				"application plane - set it only for a service that expects a value of its own. With 'oidc' nothing is forwarded: the application " +
				"signs its people in against this gateway as an OpenID provider, through 'client' (an id " +
				"from list_oidc_clients), and 'claims' say what its tokens carry - each {name, field} " +
				"naming one of the attributes, or {name, value} fixed. The provider's own claims (iss, " +
				"aud, exp, iat, nbf, nonce, azp, auth_time, at_hash, jti, client_id, scope) are refused; " +
				"sub may be set. The route's access rule decides who may sign in. Enterprise edition " +
				"only. Read it with get_route and send it back whole."},
		"ui": map[string]any{"type": "object",
			"description": "What a UI route injects: the colour scheme it drives, the roles and user info " +
				"it writes on the page, the user button, the custom CSS and script."},
		"locales": map[string]any{"type": "object",
			"description": "How this application takes a language: mechanism, the languages it speaks, " +
				"and where the locale lives in the URL."},
		"api": map[string]any{"type": "object",
			"description": "Its OpenAPI declaration, the per-endpoint security read from it, and roleMap: " +
				"each role the application declares in its spec (its own name) tied to the catalogue role " +
				"that grants it. With a roleMap, the roles expression {{.Roles | .Declared | join \",\"}} " +
				"(the default when none is written) forwards the declared roles the caller holds, under " +
				"the application's names. list_operations shows what each operation declares."},
		"timeouts": map[string]any{"type": "object", "description": "The waits for this route, overriding the installation's."},
		"breaker":  map[string]any{"type": "object", "description": "The circuit breaker for this route."},
		"limits":   map[string]any{"type": "array", "description": "The rate limits for this route.", "items": map[string]any{"type": "object"}},
		// The scalars too, for the same reason: undeclared, a number or a
		// boolean reaches the server as a string, and a route read with
		// get_route and sent back whole was refused on rev or telemetry.
		// TestEveryRouteFieldIsDeclared keeps this list whole.
		"rev": map[string]any{"type": "integer",
			"description": "The revision get_route read; a save is refused when the route moved on since. Omit it to write regardless."},
		"telemetry": map[string]any{"type": "boolean",
			"description": "Whether the gateway reports on what this route answers. Absent follows the installation."},
		"telemetryUi": map[string]any{"type": "boolean",
			"description": "Whether the pages of this UI route report from the browser too."},
	}, "id", "name", "predicates")
	// Anything this build does not name still travels: a route read from a
	// newer gateway must not lose a field on its way back.
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
		Replace     bool   `json:"replace"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	if in.Replace {
		named, err := a.configurationByName(ctx, in.Name)
		if err != nil {
			return nil, err
		}
		// The list leaves the documents out; the whole row, as the console's
		// Capture reads it.
		if named, err = a.st.GetConfiguration(ctx, named.ID); err != nil {
			return nil, err
		}
		c, err := a.recapture(ctx, mcpActor(ctx), named)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"name": c.Name, "bytes": len(c.Document), "replaced": true,
			"note": "Refreshed from the gateway as it is now, and named the current one. Nothing was applied.",
		}, nil
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
		return nil, fmt.Errorf("a configuration called %s already exists: pick another name, or pass replace: true to refresh it from the running state", name)
	}
	// The same capture as the console's, pictures in: a copy saved here
	// without its logo would push to git without one.
	_, file, err := a.runningDocument(ctx)
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
	running, _, err := a.runningDocument(ctx)
	if err != nil {
		return nil, err
	}
	// The console's current card asks the same question with the same
	// function: comparing this listing's fingerprint (pictures out) with a
	// capture's (pictures in) answered false straight after every capture on
	// a gateway that wears a logo.
	matching, list, err := a.matchesRunning(ctx, running)
	if err != nil {
		return nil, err
	}
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
			MatchesRunning: matching[c.ID],
		})
	}
	return map[string]any{"configurations": out}, nil
}

// toolActivateConfiguration switches to a saved configuration, by name.
//
// By NAME and not by id, like every other configuration tool here: an id is a
// value an agent would have to carry between two calls, and the name is what
// the person asking said out loud.
func (a *API) toolActivateConfiguration(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	named, err := a.configurationByName(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	// Read again by id: a LIST does not carry the documents - one row per saved
	// state, each holding an installation, is not what a list is for - so the
	// row this resolved a name into has everything except the thing to apply.
	c, err := a.st.GetConfiguration(ctx, named.ID)
	if err != nil {
		return nil, err
	}
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		return nil, err
	}
	preview, err := config.PreviewSwitch(ctx, a.st, doc)
	if err != nil {
		return nil, err
	}
	if err := a.agentMayApply(preview); err != nil {
		return nil, err
	}
	plan, err := config.Switch(ctx, a.st, doc)
	if err != nil {
		return nil, err
	}
	// A configuration carries the gateway-wide token policy: every cached
	// token decision is read again.
	a.sm.TokenChanged("*")
	if err := a.st.MarkConfigurationActive(ctx, c.ID); err != nil {
		return nil, err
	}
	a.auditEvent(ctx, mcpActor(ctx), "configuration.activate", "configuration", c.ID, c.Name, "",
		summarise(plan))
	// Saving IS applying. A reload that fails means a route in this
	// configuration no longer compiles: say so rather than report a clean
	// switch - the previous plan keeps serving in the meantime.
	if err := a.reloadRouting(ctx); err != nil {
		return nil, fmt.Errorf("activated, but the routing table could not be reloaded: %w", err)
	}
	return map[string]any{
		"activated": c.Name,
		"changes":   plan.Changes,
		"missing":   plan.Missing,
		"note":      "This gateway now serves that configuration. It is applied, not staged.",
	}, nil
}

// configurationByName resolves what a person said into the row.
func (a *API) configurationByName(ctx context.Context, name string) (store.Configuration, error) {
	name = strings.TrimSpace(name)
	list, err := a.st.ListConfigurations(ctx)
	if err != nil {
		return store.Configuration{}, err
	}
	var names []string
	for _, c := range list {
		if strings.EqualFold(c.Name, name) {
			return c, nil
		}
		names = append(names, c.Name)
	}
	if len(names) == 0 {
		return store.Configuration{}, fmt.Errorf("this gateway has no saved configuration yet: " +
			"save_configuration takes one from the running state")
	}
	return store.Configuration{}, fmt.Errorf("no configuration is called %q: this gateway holds %s",
		name, strings.Join(names, ", "))
}

// toolImportConfiguration applies a document, or says what it would do.
func (a *API) toolImportConfiguration(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Document string `json:"document"`
		Prune    bool   `json:"prune"`
		DryRun   bool   `json:"dryRun"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Document) == "" {
		return nil, fmt.Errorf("document is empty: pass the YAML or JSON to apply")
	}
	if len(in.Document) > maxConfigBytes {
		return nil, fmt.Errorf("this document is %d bytes, and a configuration is limited to %d MB",
			len(in.Document), maxConfigBytes>>20)
	}
	// A package (ZIP) cannot travel through a tool call, and saying so beats
	// letting the YAML parser report that the first byte is not a key.
	if config.IsBundle([]byte(in.Document)) {
		return nil, fmt.Errorf("this is a package (a ZIP), and a tool call carries text: " +
			"pass the meerkat.yaml inside it, or import the package from the console, " +
			"which is where its pictures come with it")
	}
	doc, err := config.Unmarshal([]byte(in.Document))
	if err != nil {
		return nil, err
	}
	// What a document may not change for an agent is asked before anything
	// is applied, dry run or not.
	plan, err := config.Preview(ctx, a.st, doc, in.Prune)
	if err != nil {
		return nil, err
	}
	if err := a.agentMayApply(plan); err != nil {
		return nil, err
	}
	if in.DryRun {
		return map[string]any{
			"dryRun": true, "prune": in.Prune,
			"changes": plan.Changes, "missing": plan.Missing, "missingFiles": plan.MissingFiles,
			"note": "Nothing was changed. Call again without dryRun to apply this.",
		}, nil
	}
	plan, err = a.applyDocument(ctx, doc, in.Prune, mcpActor(ctx), nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"prune":   in.Prune,
		"changes": plan.Changes, "missing": plan.Missing, "missingFiles": plan.MissingFiles,
		"note": "Applied. A restore point was recorded before it, and the trail names the token that did it.",
	}, nil
}
