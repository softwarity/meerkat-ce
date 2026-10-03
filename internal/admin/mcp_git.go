package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/confrepo"
	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// The git side of a configuration, for an agent (CFG-07).
//
// WHY THESE EXIST AT ALL, given an agent can already export a configuration and
// run git itself: because then the agent needs a checkout and a credential of
// its own, and the gateway's own token - the one in the vault, scoped to one
// repository - is the credential that should be doing this. An agent that asks
// the gateway to push holds nothing.
//
// Everything names things by NAME, never by id, because that is what somebody
// typed in a prompt: "pull Acme and tell me what would change".

func (a *API) gitTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "list_git_locations", Allow: isRoot, Title: "List the git locations", ReadOnly: true,
			Description: "The named git locations this gateway can read a configuration from and write " +
				"one to - a repository, a branch and a directory each - with which saved " +
				"configurations are bound to them and whether each is up to date. " +
				"Enterprise edition. Use these names with pull_configuration and push_configuration.",
			Schema: noArgs(),
			Call:   a.toolListGitLocations,
		},
		{
			Name: "pull_configuration", Allow: isRoot, Title: "Pull a configuration from git",
			Description: "Read a configuration from a git location and KEEP IT BESIDE the running one. " +
				"It applies NOTHING: the document lands as a saved configuration and the gateway goes " +
				"on serving what it serves. The answer is the plan - what activating it WOULD add, " +
				"change and remove - plus the vault entries it expects and this installation has not " +
				"got. \n\nThat is the whole loop: pull, read the plan, then activate_configuration when " +
				"it says what you expected. Never report a pull as a deployment. \n\nGive the location " +
				"name alone to pull into a new saved configuration, or a configuration name to refresh " +
				"one already bound to its location. Enterprise edition.",
			Schema: object(map[string]any{
				"location": str("The git location's name, as list_git_locations gives it. " +
					"Required when pulling into a new configuration."),
				"configuration": str("Optional: the saved configuration to refresh, by name. " +
					"When given, its own location is used and `location` is not needed."),
				"name": str("Optional: what to call the new configuration. Defaults to the location's name."),
			}),
			Call: a.toolPullConfiguration,
		},
		{
			Name: "push_configuration", Allow: isRoot, Title: "Push a configuration to git",
			Description: "Commit a SAVED configuration to the git location it is bound to, and push. " +
				"The commit is attributed to the account this token belongs to. \n\nIt pushes the saved " +
				"copy, not the running state: if you want what the gateway is running right now, call " +
				"save_configuration first and push that. \n\nA push REPLACES what the location's " +
				"directory holds - as a pull replaces the saved configuration - and only that directory: " +
				"what it replaced stays in the git history. Enterprise edition.",
			Schema: object(map[string]any{
				"configuration": str("The saved configuration's name, as list_configurations gives it."),
				"location": str("Optional: a git location name to bind it to first, when it has none " +
					"or should go somewhere else."),
			}, "configuration"),
			Call: a.toolPushConfiguration,
		},
	}
}

func (a *API) toolListGitLocations(ctx context.Context, _ json.RawMessage) (any, error) {
	if err := gitAvailable(); err != nil {
		return nil, err
	}
	list, err := a.st.ListConfigRemotes(ctx)
	if err != nil {
		return nil, err
	}
	configs, err := a.st.ListConfigurations(ctx)
	if err != nil {
		return nil, err
	}
	type bound struct {
		Configuration string `json:"configuration"`
		// Revision is where that configuration last met this location. Empty
		// means it has never been read from it or pushed to it, so the next
		// push is its first.
		Revision string `json:"revision,omitempty"`
	}
	type line struct {
		Name      string  `json:"name"`
		URL       string  `json:"url"`
		Branch    string  `json:"branch"`
		Directory string  `json:"directory,omitempty"`
		Bound     []bound `json:"bound,omitempty"`
	}
	out := make([]line, 0, len(list))
	for _, remote := range list {
		l := line{Name: remote.Name, URL: remote.URL, Branch: remote.Branch, Directory: remote.Dir}
		for _, c := range configs {
			if c.RemoteID == remote.ID {
				l.Bound = append(l.Bound, bound{Configuration: c.Name, Revision: c.RemoteRev})
			}
		}
		out = append(out, l)
	}
	return map[string]any{"locations": out}, nil
}

func (a *API) toolPullConfiguration(ctx context.Context, args json.RawMessage) (any, error) {
	if err := gitAvailable(); err != nil {
		return nil, err
	}
	var in struct {
		Location      string `json:"location"`
		Configuration string `json:"configuration"`
		Name          string `json:"name"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	actor := mcpActor(ctx)

	// Refreshing one that exists: its own location, so an agent cannot pull
	// Acme's directory into Foo's row by naming the wrong one.
	if strings.TrimSpace(in.Configuration) != "" {
		named, err := a.configurationByName(ctx, in.Configuration)
		if err != nil {
			return nil, err
		}
		c, err := a.st.GetConfiguration(ctx, named.ID)
		if err != nil {
			return nil, err
		}
		if c.RemoteID == "" {
			return nil, fmt.Errorf("%s is not bound to a git location: push_configuration takes one, "+
				"or bind it in the console", c.Name)
		}
		remote, err := a.st.GetConfigRemote(ctx, c.RemoteID)
		if err != nil {
			return nil, err
		}
		file, rev, err := a.pullDocument(ctx, remote)
		if err != nil {
			return nil, err
		}
		c.Document = file
		if err := a.st.SaveConfiguration(ctx, &c); err != nil {
			return nil, err
		}
		if err := a.st.MarkConfigurationSynced(ctx, c.ID, rev); err != nil {
			return nil, err
		}
		a.auditEvent(ctx, actor, "configuration.pull", "configuration", c.ID, c.Name, "",
			fmt.Sprintf("pulled from %s at %s, %d bytes, applied to nothing", remote.Name, rev, len(file)))
		return a.pullOutcome(ctx, c, remote, rev)
	}

	// Pulling a location this gateway does not hold yet.
	remote, err := a.configRemoteByName(ctx, in.Location)
	if err != nil {
		return nil, err
	}
	if err := a.roomForAnother(ctx); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = remote.Name
	}
	name, err = store.SanitizeConfigurationName(name)
	if err != nil {
		return nil, err
	}
	taken, err := a.st.ConfigurationNameTaken(ctx, name, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, fmt.Errorf("a configuration called %s already exists: name the new one, "+
			"or pass configuration: %q to refresh that one from its own location", name, name)
	}
	file, rev, err := a.pullDocument(ctx, remote)
	if err != nil {
		return nil, err
	}
	c := store.Configuration{ID: newID(), Name: name, Document: file}
	if err := a.st.SaveConfiguration(ctx, &c); err != nil {
		return nil, err
	}
	if err := a.st.BindConfiguration(ctx, c.ID, remote.ID); err != nil {
		return nil, err
	}
	if err := a.st.MarkConfigurationSynced(ctx, c.ID, rev); err != nil {
		return nil, err
	}
	a.auditEvent(ctx, actor, "configuration.pull", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("pulled from %s at %s, %d bytes, applied to nothing", remote.Name, rev, len(file)))
	return a.pullOutcome(ctx, c, remote, rev)
}

// pullOutcome is what both halves of a pull answer: where it came from, and what
// activating it would do. The sentence is there because an agent reporting "done"
// after a pull would be reporting a deployment that did not happen.
func (a *API) pullOutcome(
	ctx context.Context, c store.Configuration, remote store.ConfigRemote, rev string,
) (any, error) {
	out := map[string]any{
		"configuration": c.Name,
		"location":      remote.Name,
		"revision":      rev,
		"applied":       false,
		"next": fmt.Sprintf("Nothing was applied: the gateway still serves what it served. "+
			"Read the plan, then activate_configuration %q to switch to it.", c.Name),
	}
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		out["planError"] = err.Error()
		return out, nil
	}
	plan, err := config.PreviewSwitch(ctx, a.st, doc)
	if err != nil {
		out["planError"] = err.Error()
		return out, nil
	}
	out["plan"] = summarise(plan)
	if len(plan.Missing) > 0 {
		missing := make([]string, 0, len(plan.Missing))
		for _, m := range plan.Missing {
			missing = append(missing, m.Name)
		}
		out["missingVaultEntries"] = missing
	}
	return out, nil
}

func (a *API) toolPushConfiguration(ctx context.Context, args json.RawMessage) (any, error) {
	if err := gitAvailable(); err != nil {
		return nil, err
	}
	var in struct {
		Configuration string `json:"configuration"`
		Location      string `json:"location"`
	}
	if err := decode(args, &in); err != nil {
		return nil, err
	}
	named, err := a.configurationByName(ctx, in.Configuration)
	if err != nil {
		return nil, err
	}
	c, err := a.st.GetConfiguration(ctx, named.ID)
	if err != nil {
		return nil, err
	}
	actor := mcpActor(ctx)
	if strings.TrimSpace(in.Location) != "" {
		remote, err := a.configRemoteByName(ctx, in.Location)
		if err != nil {
			return nil, err
		}
		if remote.ID != c.RemoteID {
			if err := a.st.BindConfiguration(ctx, c.ID, remote.ID); err != nil {
				return nil, err
			}
			c.RemoteID, c.RemoteRev = remote.ID, ""
			a.auditEvent(ctx, actor, "configuration.bind", "configuration", c.ID, c.Name, "",
				"bound to "+remote.Name)
		}
	}
	if c.RemoteID == "" {
		return nil, fmt.Errorf("%s is not bound to a git location: name one in `location`, "+
			"and list_git_locations says which exist", c.Name)
	}
	remote, err := a.st.GetConfigRemote(ctx, c.RemoteID)
	if err != nil {
		return nil, err
	}
	driver, target, err := a.gitRemote(ctx, remote)
	if err != nil {
		return nil, err
	}
	author, email := commitAuthor(remote, actor)
	target.AuthorName, target.AuthorEmail = author, email
	files, err := splitConfiguration(c)
	if err != nil {
		return nil, err
	}
	rev, err := driver.Commit(ctx, target, pushMessage(c, author), files)
	if errors.Is(err, confrepo.ErrMoved) {
		return nil, fmt.Errorf("nothing was pushed: %s kept moving while %s was being written - "+
			"somebody else pushing at the same moment. Push again", remote.Name, c.Name)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", remote.Name, err)
	}
	if err := a.st.MarkConfigurationSynced(ctx, c.ID, rev); err != nil {
		return nil, err
	}
	a.auditEvent(ctx, actor, "configuration.push", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("pushed to %s (%s, %s) at %s, %d files", remote.Name, remote.Branch,
			confrepo.Join(remote.Dir, config.BundleName), rev, len(files)))
	return map[string]any{
		"configuration": c.Name, "location": remote.Name, "branch": remote.Branch,
		"path": confrepo.Join(remote.Dir, config.BundleName), "revision": rev,
		"author": author + " <" + email + ">",
	}, nil
}

// ── shared with the HTTP handlers ────────────────────────────────────────────

// splitConfiguration renders a saved configuration as the files a commit holds.
func splitConfiguration(c store.Configuration) (map[string][]byte, error) {
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		return nil, err
	}
	body, assets, err := config.Split(doc)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{config.BundleName: body}
	for name, content := range assets {
		files[name] = content
	}
	return files, nil
}

// pullDocument reads a location into the bytes a saved configuration holds. The
// error-returning twin of fetchDocument, for the callers that have no
// ResponseWriter.
func (a *API) pullDocument(ctx context.Context, remote store.ConfigRemote) (string, string, error) {
	driver, target, err := a.gitRemote(ctx, remote)
	if err != nil {
		return "", "", err
	}
	files, rev, err := driver.Fetch(ctx, target)
	if errors.Is(err, confrepo.ErrAbsent) {
		return "", "", fmt.Errorf("%s holds no configuration yet (looked for %s on %s): "+
			"push one there first", remote.Name, confrepo.Join(remote.Dir, config.BundleName), remote.Branch)
	}
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", remote.Name, err)
	}
	doc, err := config.Assemble(files)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", remote.Name, err)
	}
	file, err := config.Marshal(doc)
	if err != nil {
		return "", "", err
	}
	return string(file), rev, nil
}

// gitRemote resolves a row into what the driver is handed, token expanded.
func (a *API) gitRemote(ctx context.Context, row store.ConfigRemote) (confrepo.Driver, confrepo.Remote, error) {
	driver, err := confrepo.Get()
	if err != nil {
		return nil, confrepo.Remote{}, err
	}
	target := confrepo.Remote{
		Name: row.Name, Provider: row.Provider, URL: row.URL, Branch: row.Branch, Dir: row.Dir, User: row.TokenUser,
		AuthorName: row.AuthorName, AuthorEmail: row.AuthorEmail,
	}
	if row.TokenRef != "" {
		target.Token = a.st.ExpandInfra(ctx, row.TokenRef)
		// ExpandInfra hands back the reference UNCHANGED when the vault has no
		// such entry. Refused here rather than passed on as an empty password:
		// "authentication failed" would send the operator to look at the forge,
		// and the hole is on this side.
		if vault.IsRef(target.Token) {
			return nil, confrepo.Remote{}, fmt.Errorf(
				"the vault holds no %s, which %s refers to for its access token",
				vault.RefName(row.TokenRef), row.Name)
		}
	}
	return driver, target, nil
}

// configRemoteByName resolves a location the way an agent names it, and says
// which exist when it does not.
func (a *API) configRemoteByName(ctx context.Context, name string) (store.ConfigRemote, error) {
	list, err := a.st.ListConfigRemotes(ctx)
	if err != nil {
		return store.ConfigRemote{}, err
	}
	for _, r := range list {
		if strings.EqualFold(strings.TrimSpace(r.Name), strings.TrimSpace(name)) {
			return r, nil
		}
	}
	names := make([]string, 0, len(list))
	for _, r := range list {
		names = append(names, r.Name)
	}
	if len(names) == 0 {
		return store.ConfigRemote{}, errors.New(
			"this gateway has no git location: one is set up in the console, under the configurations")
	}
	return store.ConfigRemote{}, fmt.Errorf("no git location called %q. There is: %s",
		name, strings.Join(names, ", "))
}
