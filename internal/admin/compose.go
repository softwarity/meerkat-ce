package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/softwarity/meerkat/internal/confrepo"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/store"
)

// Composing a configuration from two (CFG-08) - root only, like everything
// that reads or writes a whole document.
//
// The preview is a READ: two documents in, one out, with what the composition
// has to say and what applying it would do to the gateway; nothing stored.
// What follows is the ordinary doors, handed the composed document: Apply is a
// switch, Save puts it on the shelf, Export downloads it, Push saves then
// pushes - one assembler, four existing destinations, the same checks.
//
// Five patterns rather than one with a verb in it, because the mux refuses the
// ambiguity "compose/{then}" would make with "{id}/push".

func (a *API) registerCompose(mux Mux) {
	mux.Handle("POST /api/configurations/compose", a.rootOnly(a.composePreview))
	mux.Handle("POST /api/configurations/compose/apply", a.rootOnly(a.composeApply))
	mux.Handle("POST /api/configurations/compose/save", a.rootOnly(a.composeSave))
	mux.Handle("POST /api/configurations/compose/export", a.rootOnly(a.composeExport))
	mux.Handle("POST /api/configurations/compose/push", a.rootOnly(a.composePush))
}

// composeSource names one side: what runs, a saved configuration, a restore
// point, a git location, or a file.
type composeSource struct {
	Source string `json:"source"`
	ID     string `json:"id,omitempty"`
	// Document is the file itself, YAML or JSON, when Source is "file": the
	// console holds the bytes it was handed and has no reason to park them
	// anywhere first.
	Document string `json:"document,omitempty"`
	// Package is the same for a ZIP, in base64: a file looked at is not a file
	// saved, whatever its shape.
	Package string `json:"package,omitempty"`
}

// composeBody is the recipe with its two sources - what the console sends at
// each click, and what an agent replays.
type composeBody struct {
	Left  composeSource `json:"left"`
	Right composeSource `json:"right"`
	config.Recipe
	// Name is what Save and Push call the result; RemoteID the location Push
	// goes to; Format "zip" asks Export for the package with its assets.
	Name     string `json:"name,omitempty"`
	RemoteID string `json:"remoteId,omitempty"`
	// RemoteRev is the revision of the git location the right side was read
	// at: a push names it, and is refused if the location moved since.
	RemoteRev string `json:"remoteRev,omitempty"`
	Format    string `json:"format,omitempty"`
	// Replace lets Save refresh the row that already carries the name,
	// instead of refusing it: "save it again" is the commonest wish here.
	Replace bool `json:"replace,omitempty"`
}

// composeAnswer is the preview: both sides and the result as the console
// shows them (pictures out, the way an export writes them, so a value differs
// only when it differs), the object-level comparison of the sides, the
// composition's report, and the plan of applying the result.
type composeAnswer struct {
	Left      json.RawMessage `json:"left"`
	Right     json.RawMessage `json:"right"`
	Result    json.RawMessage `json:"result"`
	LeftName  string          `json:"leftName"`
	RightName string          `json:"rightName"`
	Changes   []config.Change `json:"changes"`
	Report    *config.Report  `json:"report"`
	// Media says which routes carry a deposited spec and how many files, on
	// each side and in the result: the documents above leave them out (they
	// ride beside the YAML, never in it), and a route taken from the other
	// side brings its own without a word otherwise.
	Media map[string]map[string]mediaInfo `json:"media"`
	// Leaves says what the result does not carry once it is on a filesystem.
	Leaves *composeLeaves `json:"leaves"`
	// RightRev is the revision the right side was read at, when it is a git
	// location: what a push must name to be allowed.
	RightRev string       `json:"rightRev,omitempty"`
	Plan     *config.Plan `json:"plan,omitempty"`
	// PlanError says why the result could not be planned against the gateway
	// (an empty result, a document the checks refuse): the preview still
	// answers, with the reason, so the screen shows what to fix.
	PlanError string `json:"planError,omitempty"`
}

// composeLeaves is what the result leaves behind on a filesystem, said before
// the download: the moment the operator can still act on it. Whatever the
// shape, a file holds the names of the vault entries and never their values,
// nor a secret held in clear here. Images are what a plain file loses and a
// package carries, with the specs and files of Media.
type composeLeaves struct {
	Refs     []string         `json:"refs"`
	Literals []config.Literal `json:"literals"`
	Images   bool             `json:"images"`
}

// composed is one composition read from the request: the body, both sides
// resolved, the result and its report.
type composed struct {
	body        composeBody
	left, right *config.Document
	lname       string
	rname       string
	rrev        string
	result      *config.Document
	report      *config.Report
}

// compose reads the body, resolves the sides and composes; the refusals are
// written to the response.
func (a *API) compose(w http.ResponseWriter, r *http.Request) (*composed, bool) {
	var body composeBody
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	left, lname, _, ok := a.composeSide(w, r, body.Left, "left")
	if !ok {
		return nil, false
	}
	right, rname, rrev, ok := a.composeSide(w, r, body.Right, "right")
	if !ok {
		return nil, false
	}
	result, report, err := config.Compose(left, right, body.Recipe)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return nil, false
	}
	return &composed{body: body, left: left, right: right, lname: lname, rname: rname, rrev: rrev, result: result, report: report}, true
}

// composeSide resolves one source into a document and a name for the
// sentences, the refusal written to the response. A git location is the one
// source that reaches out, and it is resolved here, the HTTP way (fetchDocument);
// the others are sideDocument, shared with the agent's tool.
func (a *API) composeSide(w http.ResponseWriter, r *http.Request, src composeSource, side string) (*config.Document, string, string, bool) {
	if src.Source == "remote" {
		if err := gitAvailable(); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return nil, "", "", false
		}
		row, ok := a.configRemoteByID(r.Context(), w, src.ID)
		if !ok {
			return nil, "", "", false
		}
		file, rev, ok := a.fetchDocument(w, r, row)
		if !ok {
			return nil, "", "", false
		}
		doc, err := config.Unmarshal([]byte(file))
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, row.Name+": "+err.Error())
			return nil, "", "", false
		}
		return doc, row.Name, rev, true
	}
	doc, name, status, err := a.sideDocument(r.Context(), src, side)
	if err != nil {
		if status == http.StatusInternalServerError {
			a.internal(w, err)
		} else {
			writeErr(w, status, err.Error())
		}
		return nil, "", "", false
	}
	return doc, name, "", true
}

// sideDocument reads one side that needs no network: what runs, a saved
// configuration, a restore point, a file's text. The status says how a
// refusal is answered.
func (a *API) sideDocument(ctx context.Context, src composeSource, side string) (*config.Document, string, int, error) {
	switch src.Source {
	case "running":
		doc, _, err := config.Export(ctx, a.st)
		if err != nil {
			return nil, "", http.StatusInternalServerError, err
		}
		return doc, "Running", 0, nil
	case "configuration":
		c, err := a.st.GetConfiguration(ctx, src.ID)
		if errors.Is(err, store.ErrConfigurationNotFound) {
			return nil, "", http.StatusNotFound, errors.New("the " + side + " side names a configuration that does not exist")
		}
		if err != nil {
			return nil, "", http.StatusInternalServerError, err
		}
		doc, err := config.Unmarshal([]byte(c.Document))
		if err != nil {
			return nil, "", http.StatusUnprocessableEntity, fmt.Errorf("%s: %w", c.Name, err)
		}
		return doc, c.Name, 0, nil
	case "point":
		p, err := a.st.GetConfigPoint(ctx, src.ID)
		if errors.Is(err, store.ErrNoConfigPoint) {
			return nil, "", http.StatusNotFound, errors.New("the " + side + " side names a restore point that does not exist")
		}
		if err != nil {
			return nil, "", http.StatusInternalServerError, err
		}
		doc, err := config.Unmarshal([]byte(p.Document))
		if err != nil {
			return nil, "", http.StatusUnprocessableEntity, fmt.Errorf("%s: %w", pointName(p), err)
		}
		return doc, pointName(p), 0, nil
	case "file":
		if src.Package != "" {
			raw, err := base64.StdEncoding.DecodeString(src.Package)
			if err != nil {
				return nil, "", http.StatusUnprocessableEntity,
					errors.New("the " + side + " side's package is not base64: " + err.Error())
			}
			if len(raw) > maxConfigBytes {
				return nil, "", http.StatusRequestEntityTooLarge,
					fmt.Errorf("a configuration file is limited to %d MB", maxConfigBytes>>20)
			}
			doc, err := config.UnmarshalBundle(raw)
			if err != nil {
				return nil, "", http.StatusUnprocessableEntity, err
			}
			return doc, "the " + side + " package", 0, nil
		}
		if strings.TrimSpace(src.Document) == "" {
			return nil, "", http.StatusUnprocessableEntity, errors.New("the " + side + " side is a file and carries no document")
		}
		if len(src.Document) > maxConfigBytes {
			return nil, "", http.StatusRequestEntityTooLarge,
				fmt.Errorf("a configuration file is limited to %d MB", maxConfigBytes>>20)
		}
		doc, err := config.Unmarshal([]byte(src.Document))
		if err != nil {
			return nil, "", http.StatusUnprocessableEntity, err
		}
		return doc, "the file", 0, nil
	}
	return nil, "", http.StatusUnprocessableEntity, errors.New(
		"the " + side + " side names no source: one of running, configuration, point, remote (with its id) or file (with its document)")
}

// composePreview answers the composition and changes nothing.
func (a *API) composePreview(w http.ResponseWriter, r *http.Request, _ store.User) {
	c, ok := a.compose(w, r)
	if !ok {
		return
	}
	out := composeAnswer{
		LeftName: c.lname, RightName: c.rname, RightRev: c.rrev,
		Changes: config.Compare(c.left, c.right), Report: c.report,
		Media: map[string]map[string]mediaInfo{
			"left": mediaOf(c.left), "right": mediaOf(c.right), "result": mediaOf(c.result),
		},
	}
	var err error
	if out.Leaves, err = a.leavesBehind(r.Context(), c.result); err != nil {
		a.internal(w, err)
		return
	}
	if out.Left, err = config.Tidy(c.left); err != nil {
		a.internal(w, err)
		return
	}
	if out.Right, err = config.Tidy(c.right); err != nil {
		a.internal(w, err)
		return
	}
	if out.Result, err = config.Tidy(c.result); err != nil {
		a.internal(w, err)
		return
	}
	// The plan is what applying the result would do to THIS gateway: read
	// here so the confirmation dialog is the one every switch has.
	if plan, err := config.PreviewSwitch(r.Context(), a.st, c.result); err != nil {
		out.PlanError = err.Error()
	} else {
		out.Plan = plan
	}
	writeJSON(w, http.StatusOK, out)
}

// leavesBehind reads what doc leaves behind. The literals are this gateway's,
// so only those of an object the result holds count.
func (a *API) leavesBehind(ctx context.Context, doc *config.Document) (*composeLeaves, error) {
	refs, err := config.Refs(doc)
	if err != nil {
		return nil, err
	}
	if refs == nil {
		refs = []string{}
	}
	_, literals, err := config.Export(ctx, a.st)
	if err != nil {
		return nil, err
	}
	held := func(l config.Literal) bool {
		switch l.Holder {
		case "authprovider":
			return slices.ContainsFunc(doc.AuthProviders, func(p store.AuthProvider) bool { return p.ID == l.ID })
		case "mailrelay":
			return doc.MailRelay != nil
		}
		_, ok := doc.Settings[l.Holder]
		return ok
	}
	kept := []config.Literal{}
	for _, l := range literals {
		if held(l) {
			kept = append(kept, l)
		}
	}
	return &composeLeaves{Refs: refs, Literals: kept, Images: config.HasPicture(doc)}, nil
}

// composeApply makes the result the running configuration: a switch, with
// the same checks, reload and trail as activating a saved one.
func (a *API) composeApply(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.compose(w, r)
	if !ok {
		return
	}
	if err := c.report.Blocked(); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.agentMayApplyDoc(r.Context(), c.result, true, true); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	plan, err := config.Switch(r.Context(), a.st, c.result)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.sm.TokenChanged("*")
	a.auditEvent(r.Context(), actor, "configuration.compose", "config", "", "", "",
		c.describe()+"; "+summarise(plan))
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("applied, but the routing table could not be reloaded: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// composeSave puts the result on the shelf under a name, applying nothing.
func (a *API) composeSave(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.compose(w, r)
	if !ok {
		return
	}
	var saved store.Configuration
	var ok2 bool
	if c.body.Replace {
		saved, ok2 = a.saveOrRefreshComposed(w, r, actor, c)
	} else {
		saved, ok2 = a.saveComposed(w, r, actor, c)
	}
	if !ok2 {
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

// saveComposed is the shelf half of Save and Push.
func (a *API) saveComposed(w http.ResponseWriter, r *http.Request, actor store.User, c *composed) (store.Configuration, bool) {
	if err := a.roomForAnother(r.Context()); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return store.Configuration{}, false
	}
	name, ok := a.freeName(w, r, c.body.Name, "")
	if !ok {
		return store.Configuration{}, false
	}
	file, err := config.Marshal(c.result)
	if err != nil {
		a.internal(w, err)
		return store.Configuration{}, false
	}
	saved := store.Configuration{ID: newID(), Name: name, Document: string(file)}
	if err := a.st.SaveConfiguration(r.Context(), &saved); err != nil {
		a.internal(w, err)
		return store.Configuration{}, false
	}
	a.auditEvent(r.Context(), actor, "configuration.create", "configuration", saved.ID, saved.Name, "",
		fmt.Sprintf("%s, %d bytes, applied to nothing", c.describe(), len(file)))
	return saved, true
}

// composeExport downloads the result: a YAML without pictures, or the package.
func (a *API) composeExport(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.compose(w, r)
	if !ok {
		return
	}
	name, mime := "meerkat-composed.yaml", "application/yaml; charset=utf-8"
	var file []byte
	var err error
	if c.body.Format == "zip" {
		if !config.HasImage(c.result) {
			writeErr(w, http.StatusUnprocessableEntity,
				"this configuration carries no image: download it as a plain file")
			return
		}
		if file, err = config.MarshalBundle(c.result); err != nil {
			a.internal(w, err)
			return
		}
		name, mime = "meerkat-composed.zip", "application/zip"
	} else {
		stripped, sErr := config.WithoutImages(c.result)
		if sErr != nil {
			a.internal(w, sErr)
			return
		}
		if file, err = config.Marshal(stripped); err != nil {
			a.internal(w, err)
			return
		}
	}
	a.auditEvent(r.Context(), actor, "configuration.export", "config", "", "", "",
		fmt.Sprintf("%s, %d bytes", c.describe(), len(file)))
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(file)
}

// composePush commits the result to a git location, and saves nothing here.
//
// Only to the location read on the RIGHT, at the revision it was read at:
// what is pushed replaces the directory there, and nobody replaces what they
// have not looked at. A location that moved since the right side was read is
// a refusal with its cure (reload the right side), never a force.
func (a *API) composePush(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	c, ok := a.compose(w, r)
	if !ok {
		return
	}
	if c.body.Right.Source != "remote" || (c.body.RemoteID != "" && c.body.RemoteID != c.body.Right.ID) {
		writeErr(w, http.StatusUnprocessableEntity,
			"a push goes to the git location read on the right side, and nowhere else: load it on the right, resolve the differences, then push")
		return
	}
	if c.body.RemoteRev == "" || c.body.RemoteRev != c.rrev {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"%s moved since the right side was read (it is at %s, the right side was read at %s): reload the right side and resolve again",
			c.rname, short(c.rrev), short(c.body.RemoteRev)))
		return
	}
	row, ok := a.configRemoteByID(r.Context(), w, c.body.Right.ID)
	if !ok {
		return
	}
	// The result goes to the repository and nowhere else: no copy on the
	// shelf, no binding. What runs is Running, the rest is loaded and
	// compared, and a push writes where it went.
	message := func(who string) string {
		return fmt.Sprintf("Update %s from Meerkat\n\nComposed from %s and %s, pushed by %s.\n",
			row.Name, c.lname, c.rname, who)
	}
	rev, n, ok := a.pushDocument(w, r, actor, c.result, row, message, "the result")
	if !ok {
		return
	}
	a.auditEvent(r.Context(), actor, "configuration.push", "config-remote", row.ID, row.Name, "",
		fmt.Sprintf("composed result pushed to %s (%s, %s) at %s, %d files", row.Name, row.Branch,
			confrepo.Join(row.Dir, config.BundleName), rev, n))
	writeJSON(w, http.StatusOK, map[string]any{"name": row.Name, "rev": rev, "files": n})
}

// saveOrRefreshComposed saves the result under the name, or refreshes the
// row that already carries it.
func (a *API) saveOrRefreshComposed(w http.ResponseWriter, r *http.Request, actor store.User, c *composed) (store.Configuration, bool) {
	name, err := store.SanitizeConfigurationName(c.body.Name)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return store.Configuration{}, false
	}
	existing, err := a.configurationByName(r.Context(), name)
	if err != nil {
		// No row of that name (or no row at all): a new one.
		return a.saveComposed(w, r, actor, c)
	}
	row, err := a.st.GetConfiguration(r.Context(), existing.ID)
	if err != nil {
		a.internal(w, err)
		return store.Configuration{}, false
	}
	file, err := config.Marshal(c.result)
	if err != nil {
		a.internal(w, err)
		return store.Configuration{}, false
	}
	row.Document = string(file)
	if err := a.st.SaveConfiguration(r.Context(), &row); err != nil {
		a.internal(w, err)
		return store.Configuration{}, false
	}
	a.auditEvent(r.Context(), actor, "configuration.replace", "configuration", row.ID, row.Name, "",
		fmt.Sprintf("%s, %d bytes, applied to nothing", c.describe(), len(file)))
	return row, true
}

// mediaInfo is what rides beside a route in a document: its deposited spec,
// its files.
type mediaInfo struct {
	Spec  bool `json:"spec,omitempty"`
	Files int  `json:"files,omitempty"`
	// The content's hashes - the spec's, and each file's by name - so the
	// screen can tell a route whose code changed in a file from one that
	// did not, and name the file.
	SpecSum  string            `json:"specSum,omitempty"`
	FileSums map[string]string `json:"fileSums,omitempty"`
}

// mediaOf lists the routes of doc that carry a spec or files.
func mediaOf(doc *config.Document) map[string]mediaInfo {
	out := map[string]mediaInfo{}
	if doc == nil {
		return out
	}
	for id, spec := range doc.Specs {
		m := out[id]
		m.Spec = true
		m.SpecSum = config.SpecSum(spec)
		out[id] = m
	}
	for id, files := range doc.Files {
		if len(files) == 0 {
			continue
		}
		m := out[id]
		m.Files = len(files)
		m.FileSums = config.FileSums(files)
		out[id] = m
	}
	return out
}

// describe is the trail's line about a composition: the sides, and the
// recipe in numbers.
func (c *composed) describe() string {
	return fmt.Sprintf("composed from %s and %s: %d pairs, %d picks, %d pulled, %d missing",
		c.lname, c.rname, len(c.body.Pairs), len(c.body.Picks), len(c.report.Pulled), len(c.report.Missing))
}

// short is a revision as a sentence quotes it.
func short(rev string) string {
	if rev == "" {
		return "nothing"
	}
	if len(rev) > 8 {
		return rev[:8]
	}
	return rev
}
