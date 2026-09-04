package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// Several configurations, one active (CFG-01/02) - root only, Enterprise only.
//
// Root for the same reason import and export are (a document crosses both
// planes at once), and Enterprise because keeping several is what a test
// instance juggling one shape per customer buys. The COLLECTION is what is
// sold, not the format: a community installation exports, imports and versions
// its configuration exactly as before, it just holds one at a time.
//
// The rule the whole screen rests on: a saved configuration is a COPY, never
// the running state. Saving one changes nothing, deleting one changes nothing,
// and the only act that touches what the gateway serves is Activate - which is
// an import of that copy, with the same preview, the same vault holes and the
// same reload as any other.

func (a *API) registerConfigurations(mux Mux) {
	mux.Handle("GET /api/configurations", a.rootOnly(a.listConfigurations))
	mux.Handle("POST /api/configurations", a.rootOnly(a.captureConfiguration))
	mux.Handle("POST /api/configurations/import", a.rootOnly(a.storeConfigurationFile))
	mux.Handle("GET /api/configurations/current", a.rootOnly(a.currentConfiguration))
	mux.Handle("GET /api/configurations/{id}", a.rootOnly(a.getConfiguration))
	mux.Handle("PUT /api/configurations/{id}", a.rootOnly(a.renameConfiguration))
	mux.Handle("DELETE /api/configurations/{id}", a.rootOnly(a.deleteConfiguration))
	mux.Handle("POST /api/configurations/{id}/duplicate", a.rootOnly(a.duplicateConfiguration))
	mux.Handle("POST /api/configurations/{id}/capture", a.rootOnly(a.recaptureConfiguration))
	mux.Handle("PUT /api/configurations/{id}/document", a.rootOnly(a.replaceConfigurationDocument))
	mux.Handle("POST /api/configurations/{id}/activate", a.rootOnly(a.activateConfiguration))
	mux.Handle("GET /api/configurations/{id}/plan", a.rootOnly(a.configurationPlan))
	mux.Handle("GET /api/configurations/{id}/export", a.rootOnly(a.exportConfiguration))
	mux.Handle("GET /api/configurations/{id}/document", a.rootOnly(a.readConfigurationDocument))
}

// FreeConfigurations is how many the community image holds at once.
//
// A LIMIT, not a lock, and the difference is the whole packaging: every action
// works on both images - save, duplicate, set as current, export, restore -
// and what is bought is the SIZE of the shelf. A community installation keeps
// its production and its staging side by side and never meets a refusal; an
// instance juggling one configuration per customer is the one that pays.
//
// The number is shown before it is reached ("2 of 3"), because a cap
// discovered by being refused is a trap, and a cap announced is a price.
const FreeConfigurations = 3

// roomForAnother refuses only the act that would GROW the shelf past the cap.
// Everything else - and every act on what is already there - is free, on both
// images: a downgrade must never strand an installation with configurations it
// can read but not use.
func (a *API) roomForAnother(ctx context.Context) error {
	if edition.Enterprise {
		return nil
	}
	list, err := a.st.ListConfigurations(ctx)
	if err != nil {
		return err
	}
	if len(list) < FreeConfigurations {
		return nil
	}
	return fmt.Errorf(
		"this image keeps %d configurations at a time and holds %d: delete one, or the Enterprise edition keeps as many as you like",
		FreeConfigurations, len(list))
}

// configurationView is what the list and the detail answer. The document rides
// along only on the detail, and even there as the file itself - the console
// shows it, diffs it and downloads it, it never parses it.
type configurationView struct {
	store.Configuration
	// Contents is the inventory of the document: how many routes, roles,
	// authorities... - what a name alone cannot tell you when three
	// configurations sit side by side.
	Contents []config.Section `json:"contents,omitempty"`
	// HasImage says the document carries a picture inline, which is the one
	// thing that makes it heavy and unreadable - and the reason the export
	// offers a package.
	HasImage bool `json:"hasImage,omitempty"`
}

// currentState is what the first row of the management screen needs, and the
// question it answers is a COMPARISON, not a flag.
//
// "Saved" cannot be a fact recorded once: save a configuration, add a route,
// and the running state is no longer the one that was saved - a screen still
// saying "saved as Acme" would be lying, and the operator would find out by
// switching away and losing an afternoon. So the running document is
// fingerprinted on every read and matched against the saved digests.
type currentState struct {
	Digest string `json:"digest"`
	// SavedAs names the saved configuration this state MATCHES, when one does.
	// Nil means what is running exists nowhere else.
	SavedAs *configurationRef `json:"savedAs,omitempty"`
	// Active is the one holding the mark, matching or not. When it is set and
	// SavedAs is nil, the state has DRIFTED from it - which is the case the
	// warning is for, and the only one where the name is worth showing beside
	// it ("modified since Acme was saved").
	Active *configurationRef `json:"active,omitempty"`
}

type configurationRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *API) currentConfiguration(w http.ResponseWriter, r *http.Request, _ store.User) {
	file, ok := a.liveDocument(w, r)
	if !ok {
		return
	}
	out := currentState{Digest: store.DigestOf(file)}
	list, err := a.st.ListConfigurations(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	for _, c := range list {
		ref := &configurationRef{ID: c.ID, Name: c.Name}
		if c.Active {
			out.Active = ref
		}
		// The ACTIVE one wins when several hold the same document: it is the
		// one the screen is about. Any other match still counts as saved -
		// what matters to an operator is whether this state exists somewhere,
		// not which row the mark happens to sit on.
		if c.Digest == out.Digest && (out.SavedAs == nil || c.Active) {
			out.SavedAs = ref
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) listConfigurations(w http.ResponseWriter, r *http.Request, _ store.User) {
	list, err := a.st.ListConfigurations(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) getConfiguration(w http.ResponseWriter, r *http.Request, _ store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	view := configurationView{Configuration: c}
	if doc, err := config.Unmarshal([]byte(c.Document)); err == nil {
		view.Contents = config.Inventory(doc)
		view.HasImage = config.HasImage(doc)
	}
	writeJSON(w, http.StatusOK, view)
}

// captureConfiguration saves the state the gateway is running RIGHT NOW under
// a name. That is the honest primitive: there is no document editor, so the
// way to build a configuration is to configure the gateway and then say "this
// one, call it Acme".
func (a *API) captureConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.roomForAnother(r.Context()); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	name, ok := a.freeName(w, r, body.Name, "")
	if !ok {
		return
	}
	file, ok := a.liveDocument(w, r)
	if !ok {
		return
	}
	c := store.Configuration{ID: newID(), Name: name, Description: body.Description, Document: file}
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	// And it becomes the current one, because it IS: naming the running state
	// is exactly what says "what this gateway serves is called Acme". Nothing
	// is applied - the gateway already serves it - so this moves a mark and
	// changes not one route.
	if err := a.st.MarkConfigurationActive(r.Context(), c.ID); err != nil {
		a.internal(w, err)
		return
	}
	c.Active = true
	a.auditEvent(r.Context(), actor, "configuration.create", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("captured from the running gateway and named as the current one, %d bytes", len(file)))
	writeJSON(w, http.StatusCreated, c)
}

// recaptureConfiguration refreshes an existing one from the running gateway:
// the loop is activate, change things in the console, save them back.
func (a *API) recaptureConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	file, ok := a.liveDocument(w, r)
	if !ok {
		return
	}
	if file == c.Document && c.Active {
		// Nothing to save AND already the current one: answering the row as it
		// stands beats a write that pretends something happened. (A document
		// that matches but does NOT hold the mark still goes through: the mark
		// is the half being claimed.)
		writeJSON(w, http.StatusOK, c)
		return
	}
	c.Document = file
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	// Same as a first capture: this name now describes what is running, so it
	// is the current one - even if another held the mark a second ago.
	if err := a.st.MarkConfigurationActive(r.Context(), c.ID); err != nil {
		a.internal(w, err)
		return
	}
	c.Active = true
	a.auditEvent(r.Context(), actor, "configuration.capture", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("refreshed from the running gateway and named as the current one, %d bytes", len(file)))
	writeJSON(w, http.StatusOK, c)
}

// replaceConfigurationDocument overwrites one configuration's document with an
// uploaded file, applying nothing.
//
// This is what "import under a name that already exists" means, and it is a
// separate act from creating: the console asks before calling it, because the
// name is all anyone sees in the list and losing the wrong one to a re-import
// is a mistake with no undo.
func (a *API) replaceConfigurationDocument(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	doc, ok := a.readDocument(w, r)
	if !ok {
		return
	}
	file, err := config.Marshal(doc)
	if err != nil {
		a.internal(w, err)
		return
	}
	c.Document = string(file)
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "configuration.replace", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("replaced from a file, %d bytes, applied to nothing", len(file)))
	writeJSON(w, http.StatusOK, c)
}

// storeConfigurationFile takes an uploaded document into the collection
// WITHOUT applying it - the difference with /api/config/import, which applies.
// A file that lands unapplied can be inspected, planned and compared first,
// which is the whole point of holding several.
func (a *API) storeConfigurationFile(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := a.roomForAnother(r.Context()); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	doc, ok := a.readDocument(w, r)
	if !ok {
		return
	}
	name, ok := a.freeName(w, r, r.URL.Query().Get("name"), "")
	if !ok {
		return
	}
	file, err := config.Marshal(doc)
	if err != nil {
		a.internal(w, err)
		return
	}
	c := store.Configuration{ID: newID(), Name: name, Document: string(file)}
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "configuration.create", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("imported from a file, %d bytes, applied to nothing", len(file)))
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) duplicateConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	source, ok := a.configuration(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.roomForAnother(r.Context()); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		body.Name = source.Name + " (copy)"
	}
	name, ok := a.freeName(w, r, body.Name, "")
	if !ok {
		return
	}
	copied := store.Configuration{
		ID: newID(), Name: name, Description: source.Description, Document: source.Document,
	}
	if err := a.st.SaveConfiguration(r.Context(), &copied); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "configuration.duplicate", "configuration", copied.ID, copied.Name, "",
		"copied from "+source.Name)
	writeJSON(w, http.StatusCreated, copied)
}

func (a *API) renameConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	name, ok := a.freeName(w, r, body.Name, c.ID)
	if !ok {
		return
	}
	before := c
	c.Name, c.Description = name, body.Description
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "configuration.update", "configuration", c.ID, c.Name, "",
		configurationMeta(before), configurationMeta(c))
	writeJSON(w, http.StatusOK, c)
}

// Deleting is NOT gated, unlike everything else that writes here. It neither
// grows the collection nor switches between its members - the two things the
// Enterprise image adds - and gating it would strand an installation that came
// DOWN from Enterprise with rows it can see, export, and never tidy away.
func (a *API) deleteConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	if err := a.st.DeleteConfiguration(r.Context(), c.ID); err != nil {
		a.internal(w, err)
		return
	}
	detail := "the saved copy only: the gateway goes on serving what it serves"
	if c.Active {
		detail = "the ACTIVE one: what is running is unchanged, it is simply no longer named"
	}
	a.auditEvent(r.Context(), actor, "configuration.delete", "configuration", c.ID, c.Name, "", detail)
	w.WriteHeader(http.StatusNoContent)
}

// configurationPlan answers what activating this one would change, and changes
// nothing (CFG-04). It is the same traversal the import preview uses, so what
// the screen shows before a switch is what the switch does.
func (a *API) configurationPlan(w http.ResponseWriter, r *http.Request, _ store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	plan, err := config.PreviewSwitch(r.Context(), a.st, doc)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// activateConfiguration switches to it: apply the document, move the mark,
// reload.
//
// PRUNE, unlike an ordinary import. Importing a file is a merge because the
// file may legitimately be a fragment; switching to a saved configuration is
// the opposite promise - after it, the gateway IS that configuration, so what
// the document does not carry goes. Only in the sections the document has, as
// everywhere else: a configuration without themes does not wipe the themes.
//
// What survives regardless: every living object (accounts, memberships,
// sessions, tokens, the vault, the audit trail). That is the CFG-01 line, and
// it is what makes re-activating the previous configuration a real rollback
// rather than a restore.
func (a *API) activateConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	plan, err := config.Switch(r.Context(), a.st, doc)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.MarkConfigurationActive(r.Context(), c.ID); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "configuration.activate", "configuration", c.ID, c.Name, "",
		summarise(plan))
	// Saving IS applying. A reload that fails means a route in this
	// configuration no longer compiles: say so rather than report a clean
	// switch - the previous snapshot keeps serving in the meantime.
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("activated, but the routing table could not be reloaded: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// exportConfiguration downloads a SAVED one - the plain file, byte for byte
// what would be applied, named after the configuration so a directory of them
// stays readable.
func (a *API) exportConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	name, mime := "meerkat-"+slugify(c.Name)+".yaml", "application/yaml; charset=utf-8"
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// The same two forms as the live export, and the same rule: a YAML never
	// carries a picture, a package does.
	var file []byte
	if r.URL.Query().Get("format") == "zip" {
		if !config.HasImage(doc) {
			writeErr(w, http.StatusUnprocessableEntity,
				"this configuration carries no image: download it as a plain file")
			return
		}
		if file, err = config.MarshalBundle(doc); err != nil {
			a.internal(w, err)
			return
		}
		name, mime = "meerkat-"+slugify(c.Name)+".zip", "application/zip"
	} else {
		stripped, sErr := config.WithoutImages(doc)
		if sErr != nil {
			a.internal(w, sErr)
			return
		}
		if file, err = config.Marshal(stripped); err != nil {
			a.internal(w, err)
			return
		}
	}
	a.auditEvent(r.Context(), actor, "configuration.export", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("%d bytes", len(file)))
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(file)
}

// readConfigurationDocument serves a configuration as TEXT to read and edit -
// the same file, with the pictures taken out.
//
// Not the same thing as the export, and the difference is the point: an export
// is a file that has to stand on its own somewhere else, so it carries
// everything; this is what a person looks at, and a megabyte of base64 on one
// line is what stops them looking. What it drops, the import puts back.
func (a *API) readConfigurationDocument(w http.ResponseWriter, r *http.Request, _ store.User) {
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	writeDocumentText(w, a, []byte(c.Document))
}

// ── helpers ──────────────────────────────────────────────────────────────────

// writeDocumentText answers a document with its images taken out.
func writeDocumentText(w http.ResponseWriter, a *API, file []byte) {
	doc, err := config.Unmarshal(file)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	stripped, err := config.WithoutImages(doc)
	if err != nil {
		a.internal(w, err)
		return
	}
	out, err := config.Marshal(stripped)
	if err != nil {
		a.internal(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(out)
}

// configuration resolves the {id} and answers the 404 itself.
func (a *API) configuration(w http.ResponseWriter, r *http.Request) (store.Configuration, bool) {
	c, err := a.st.GetConfiguration(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrConfigurationNotFound) {
		writeErr(w, http.StatusNotFound, "no such configuration")
		return store.Configuration{}, false
	}
	if err != nil {
		a.internal(w, err)
		return store.Configuration{}, false
	}
	return c, true
}

// liveDocument renders what the gateway is running as a portable file - the
// same bytes the export hands over, secrets left behind as $names.
func (a *API) liveDocument(w http.ResponseWriter, r *http.Request) (string, bool) {
	doc, _, err := config.Export(r.Context(), a.st)
	if err != nil {
		a.internal(w, err)
		return "", false
	}
	file, err := config.Marshal(doc)
	if err != nil {
		a.internal(w, err)
		return "", false
	}
	return string(file), true
}

// freeName validates the name and refuses a collision with a sentence. Two
// configurations called "Acme" is the one mistake this screen cannot recover
// from: the list is how one is chosen.
func (a *API) freeName(w http.ResponseWriter, r *http.Request, raw, exceptID string) (string, bool) {
	name, err := store.SanitizeConfigurationName(raw)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return "", false
	}
	taken, err := a.st.ConfigurationNameTaken(r.Context(), name, exceptID)
	if err != nil {
		a.internal(w, err)
		return "", false
	}
	if taken {
		writeErr(w, http.StatusConflict, "a configuration called "+name+" already exists")
		return "", false
	}
	return name, true
}

// configurationMeta is what the audit diffs: the name and the description. The
// document is deliberately out - a field-level diff of two YAML files would be
// one enormous before/after in the trail, and what activating changes is
// already recorded, in the words of a plan.
func configurationMeta(c store.Configuration) map[string]any {
	return map[string]any{"name": c.Name, "description": c.Description}
}

// slugify turns a configuration name into a file name that survives a
// download folder, a chat and a git repository.
func slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
