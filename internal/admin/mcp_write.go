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
				"file: routes, roles, organisations, groups, themes, settings and the branding all at once. " +
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
				"('headers', 'jwt', 'signed-jwt'), the attributes, ttl, and algorithm for signed-jwt " +
				"(ES256 by default, or EdDSA, RS256). Read it with get_route and send it back whole."},
		"ui": map[string]any{"type": "object",
			"description": "What a UI route injects: the colour scheme it drives, the roles and user info " +
				"it writes on the page, the user button, the custom CSS and script."},
		"locales": map[string]any{"type": "object",
			"description": "How this application takes a language: mechanism, the languages it speaks, " +
				"and where the locale lives in the URL."},
		"api": map[string]any{"type": "object",
			"description": "Its OpenAPI declaration and the per-endpoint security read from it."},
		"timeouts": map[string]any{"type": "object", "description": "The waits for this route, overriding the installation's."},
		"breaker":  map[string]any{"type": "object", "description": "The circuit breaker for this route."},
		"limits":   map[string]any{"type": "array", "description": "The rate limits for this route.", "items": map[string]any{"type": "object"}},
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
	if in.DryRun {
		plan, err := config.Preview(ctx, a.st, doc, in.Prune)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"dryRun": true, "prune": in.Prune,
			"changes": plan.Changes, "missing": plan.Missing, "missingFiles": plan.MissingFiles,
			"note": "Nothing was changed. Call again without dryRun to apply this.",
		}, nil
	}
	plan, err := a.applyDocument(ctx, doc, in.Prune, mcpActor(ctx))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"prune":   in.Prune,
		"changes": plan.Changes, "missing": plan.Missing, "missingFiles": plan.MissingFiles,
		"note": "Applied. A restore point was recorded before it, and the trail names the token that did it.",
	}, nil
}
