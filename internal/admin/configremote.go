package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/confrepo"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// A configuration in a git repository (CFG-07) - root only, Enterprise only.
//
// GIT IS A TRANSPORT. Everything an operator does with a pulled configuration
// already existed before this file: a document arrives, it is SHELVED as a
// saved configuration and applied to nothing, then it is planned, compared and
// activated by someone who decided to. A pull is the same act as uploading a
// file, with the bytes coming from somewhere else; a push is the same act as
// downloading one. That is why nothing here applies, reloads or prunes.
//
// WHAT THE REMOTE IS NOT: part of the configuration document. A document that
// could repoint this gateway at another repository would turn activating a
// customer's configuration into overwriting that customer's directory on the
// next push. So locations live in their own table, like the vault, and they
// never travel.
//
// THE BINDING IS REMEMBERED, and that is the one thing this adds beyond a
// transport: a saved configuration knows which location it came from and at
// which revision. Without it, "has the repository moved since I read this?" has
// no answer and every push is blind.

func (a *API) registerConfigRemotes(mux Mux) {
	mux.Handle("GET /api/config-remotes", a.rootOnly(a.listConfigRemotes))
	mux.Handle("GET /api/config-remotes/forges", a.rootOnly(a.listForges))
	mux.Handle("POST /api/config-remotes", a.rootOnly(a.createConfigRemote))
	mux.Handle("PUT /api/config-remotes/{id}", a.rootOnly(a.updateConfigRemote))
	mux.Handle("DELETE /api/config-remotes/{id}", a.rootOnly(a.deleteConfigRemote))
	mux.Handle("POST /api/config-remotes/{id}/check", a.rootOnly(a.checkConfigRemote))
	// The two acts, on the collection they feed.
	mux.Handle("POST /api/configurations/pull", a.rootOnly(a.pullNewConfiguration))
	mux.Handle("POST /api/configurations/{id}/pull", a.rootOnly(a.pullConfiguration))
	mux.Handle("POST /api/configurations/{id}/push", a.rootOnly(a.pushConfiguration))
	mux.Handle("PUT /api/configurations/{id}/remote", a.rootOnly(a.bindConfiguration))
}

// gitAvailable is the one refusal every act here starts with, and it answers in
// the order an operator can act on: the edition first (a sentence about what
// was bought), then the driver (a build that should not exist).
func gitAvailable() error {
	if err := edition.Require("keeping a configuration in a git repository"); err != nil {
		return err
	}
	if !confrepo.Available() {
		return errors.New("this build carries no git driver")
	}
	return nil
}

// remoteView is a location as the console reads it. It is the row itself: the
// token is a ${name} reference, which is public, so there is nothing to hide
// and nothing to recompute.
type remoteView struct {
	store.ConfigRemote
	// Bound names the configurations pointing at this location, so that the
	// screen can say what deleting it would unbind.
	Bound []string `json:"bound,omitempty"`
}

func (a *API) listConfigRemotes(w http.ResponseWriter, r *http.Request, _ store.User) {
	list, err := a.st.ListConfigRemotes(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	configs, err := a.st.ListConfigurations(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	out := make([]remoteView, 0, len(list))
	for _, remote := range list {
		view := remoteView{ConfigRemote: remote}
		for _, c := range configs {
			if c.RemoteID == remote.ID {
				view.Bound = append(view.Bound, c.Name)
			}
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, out)
}

// listForges serves what is known about the forges (confrepo.Providers), so the
// form says what the token needs and which username this one wants BEFORE
// anybody clicks check.
//
// Served rather than written into the console because the same table answers a
// refused credential on the server, and two copies of it would drift the day a
// forge changes its mind.
func (a *API) listForges(w http.ResponseWriter, _ *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, map[string]any{
		"forges":  confrepo.Providers,
		"generic": confrepo.GenericProvider,
	})
}

// remoteBody is what the console sends for a location.
type remoteBody struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Branch      string `json:"branch"`
	Dir         string `json:"dir"`
	TokenRef    string `json:"tokenRef"`
	TokenUser   string `json:"tokenUser"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
}

// into validates the body onto a row, naming what is wrong and what is allowed.
//
// HTTPS WITH A TOKEN IS THE WHOLE OF IT, deliberately. SSH would need a private
// key in the vault and a host-key policy, which is a second decision about
// trust rather than a second field on a form - so it is refused by name here
// rather than half-supported.
func (b remoteBody) into(row *store.ConfigRemote) error {
	name, err := store.SanitizeConfigRemoteName(b.Name)
	if err != nil {
		return err
	}
	url := strings.TrimSpace(b.URL)
	switch {
	case url == "":
		return errors.New("a git location needs the repository URL")
	case !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://"):
		return fmt.Errorf("%q is not supported: a repository is reached over https, with a token "+
			"(ssh keys are not supported)", url)
	}
	branch := strings.TrimSpace(b.Branch)
	if branch == "" {
		return errors.New("a git location needs a branch")
	}
	// A literal here would be a credential stored in clear in a table the
	// console reads and a snapshot carries. The vault is not a convenience for
	// this field, it is the only form it has.
	ref := strings.TrimSpace(b.TokenRef)
	if ref != "" && !vault.IsRef(ref) {
		return errors.New("the access token must be a vault reference (${name}), never the token itself: " +
			"put it in the vault and refer to it")
	}
	row.Name, row.URL, row.Branch = name, url, branch
	row.Dir = confrepo.CleanDir(b.Dir)
	row.TokenRef = ref
	row.TokenUser = strings.TrimSpace(b.TokenUser)
	row.AuthorName = strings.TrimSpace(b.AuthorName)
	row.AuthorEmail = strings.TrimSpace(b.AuthorEmail)
	return nil
}

func (a *API) createConfigRemote(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	var body remoteBody
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	row := store.ConfigRemote{ID: newID()}
	if err := body.into(&row); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if !a.remoteNameFree(w, r, row.Name, "") {
		return
	}
	if err := a.st.SaveConfigRemote(r.Context(), &row); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "config-remote.create", "config-remote", row.ID, row.Name, "",
		fmt.Sprintf("%s (%s) in %s", row.URL, row.Branch, dirOrRoot(row.Dir)))
	writeJSON(w, http.StatusCreated, remoteView{ConfigRemote: row})
}

func (a *API) updateConfigRemote(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	row, ok := a.configRemote(w, r)
	if !ok {
		return
	}
	var body remoteBody
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	before := row
	if err := body.into(&row); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if !a.remoteNameFree(w, r, row.Name, row.ID) {
		return
	}
	if err := a.st.SaveConfigRemote(r.Context(), &row); err != nil {
		a.internal(w, err)
		return
	}
	// A location that now points somewhere else makes every revision it handed
	// out meaningless: the rows bound to it have read a different branch, or a
	// different directory, and keeping their revision would let the next push
	// believe it knew where this one stood.
	if before.URL != row.URL || before.Branch != row.Branch || before.Dir != row.Dir {
		if err := a.st.ForgetConfigRemoteRevisions(r.Context(), row.ID); err != nil {
			a.internal(w, err)
			return
		}
	}
	a.auditUpdate(r.Context(), actor, "config-remote.update", "config-remote", row.ID, row.Name, "",
		remoteMeta(before), remoteMeta(row))
	writeJSON(w, http.StatusOK, remoteView{ConfigRemote: row})
}

// deleteConfigRemote forgets a location. The configurations bound to it STAY -
// they are copies of a state, and losing a destination is not losing them - and
// the store unbinds them in the same transaction.
func (a *API) deleteConfigRemote(w http.ResponseWriter, r *http.Request, actor store.User) {
	row, ok := a.configRemote(w, r)
	if !ok {
		return
	}
	bound, err := a.st.ConfigRemoteInUse(r.Context(), row.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	if err := a.st.DeleteConfigRemote(r.Context(), row.ID); err != nil {
		a.internal(w, err)
		return
	}
	detail := "no configuration pointed at it"
	if len(bound) > 0 {
		detail = "unbound " + strings.Join(bound, ", ") + ", which stay as saved copies"
	}
	a.auditEvent(r.Context(), actor, "config-remote.delete", "config-remote", row.ID, row.Name, "", detail)
	w.WriteHeader(http.StatusNoContent)
}

// checkConfigRemote proves the repository answers and the credential is
// accepted, and WRITES NOTHING - so a wrong token is found on the screen that
// sets it rather than during the first push of a real change.
func (a *API) checkConfigRemote(w http.ResponseWriter, r *http.Request, _ store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	row, ok := a.configRemote(w, r)
	if !ok {
		return
	}
	driver, remote, ok := a.remote(w, r, row)
	if !ok {
		return
	}
	if err := driver.Check(r.Context(), remote); err != nil {
		// The message is handed over as it stands. The driver already adds what
		// the forge wants WHEN THE CREDENTIAL WAS THE PROBLEM, and only then: a
		// first version pinned the token paperwork to every failure, so a typo
		// in a branch name came back advising a different token - which sends
		// the operator to the one screen that cannot help.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// Whether a document is already there decides which half of the screen is
	// useful: a location with nothing in it is waiting for a push, not broken.
	_, rev, err := driver.Fetch(r.Context(), remote)
	out := map[string]any{"ok": true, "holds": err == nil, "revision": rev}
	if err != nil && !errors.Is(err, confrepo.ErrAbsent) {
		out["holds"], out["error"] = false, err.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

// ── pull ─────────────────────────────────────────────────────────────────────

// pullNewConfiguration takes a document from a location into the collection
// under a new name, and APPLIES NOTHING.
//
// This is the entry point for a platform this gateway has never held: the row
// arrives already bound, so the next pull and push need no dialog, and the
// answer carries the plan - what activating it WOULD change - because the only
// useful next question is that one.
func (a *API) pullNewConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	var body struct {
		RemoteID string `json:"remoteId"`
		Name     string `json:"name"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.roomForAnother(r.Context()); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, ok := a.configRemoteByID(r.Context(), w, body.RemoteID)
	if !ok {
		return
	}
	// The location's name is the obvious default: it is what the operator named
	// the platform, and it is already unique.
	if strings.TrimSpace(body.Name) == "" {
		body.Name = row.Name
	}
	name, ok := a.freeName(w, r, body.Name, "")
	if !ok {
		return
	}
	file, rev, ok := a.fetchDocument(w, r, row)
	if !ok {
		return
	}
	c := store.Configuration{ID: newID(), Name: name, Document: file}
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.st.BindConfiguration(r.Context(), c.ID, row.ID); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.st.MarkConfigurationSynced(r.Context(), c.ID, rev); err != nil {
		a.internal(w, err)
		return
	}
	c.RemoteID, c.RemoteRev = row.ID, rev
	a.auditEvent(r.Context(), actor, "configuration.pull", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("pulled from %s at %s, %d bytes, applied to nothing", row.Name, rev, len(file)))
	a.answerPull(w, r, c)
}

// pullConfiguration refreshes a bound row from its location, and APPLIES
// NOTHING. What was in the row is replaced; what the gateway serves is not
// touched until somebody activates it.
func (a *API) pullConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	row, ok := a.boundRemote(w, r, c)
	if !ok {
		return
	}
	file, rev, ok := a.fetchDocument(w, r, row)
	if !ok {
		return
	}
	was := c.Digest
	c.Document = file
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.st.MarkConfigurationSynced(r.Context(), c.ID, rev); err != nil {
		a.internal(w, err)
		return
	}
	c.RemoteRev = rev
	detail := fmt.Sprintf("pulled from %s at %s, %d bytes, applied to nothing", row.Name, rev, len(file))
	if was == c.Digest {
		detail = "pulled from " + row.Name + " at " + rev + ": the same document, nothing moved"
	}
	a.auditEvent(r.Context(), actor, "configuration.pull", "configuration", c.ID, c.Name, "", detail)
	a.answerPull(w, r, c)
}

// pullAnswer is what a pull hands back: the row as it now stands, and what
// activating it would change.
//
// The plan rides along because a pull whose answer is "ok" would leave the
// operator to click twice to learn the only thing they wanted to know. It is
// the same traversal the import preview and the activation plan use, so what
// this shows is what a switch would do.
type pullAnswer struct {
	Configuration store.Configuration `json:"configuration"`
	Plan          *config.Plan        `json:"plan,omitempty"`
	// PlanError says why the plan is missing: a document from a repository can
	// be one a future Meerkat wrote, and refusing to describe it is not a
	// reason to lose the pull that already succeeded.
	PlanError string `json:"planError,omitempty"`
}

func (a *API) answerPull(w http.ResponseWriter, r *http.Request, c store.Configuration) {
	out := pullAnswer{Configuration: c}
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		out.PlanError = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	plan, err := config.PreviewSwitch(r.Context(), a.st, doc)
	if err != nil {
		out.PlanError = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	out.Plan = plan
	writeJSON(w, http.StatusOK, out)
}

// fetchDocument reads a location and turns its files into one document, as the
// bytes a saved configuration holds.
func (a *API) fetchDocument(w http.ResponseWriter, r *http.Request, row store.ConfigRemote) (string, string, bool) {
	driver, remote, ok := a.remote(w, r, row)
	if !ok {
		return "", "", false
	}
	files, rev, err := driver.Fetch(r.Context(), remote)
	if errors.Is(err, confrepo.ErrAbsent) {
		writeErr(w, http.StatusNotFound, fmt.Sprintf(
			"%s holds no configuration yet (looked for %s on %s): push one there first",
			row.Name, confrepo.Join(row.Dir, config.BundleName), row.Branch))
		return "", "", false
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, row.Name+": "+err.Error())
		return "", "", false
	}
	doc, err := config.Assemble(files)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, row.Name+": "+err.Error())
		return "", "", false
	}
	file, err := config.Marshal(doc)
	if err != nil {
		a.internal(w, err)
		return "", "", false
	}
	return string(file), rev, true
}

// ── push ─────────────────────────────────────────────────────────────────────

// pushConfiguration commits a saved configuration to its location.
//
// It pushes the SAVED copy, never the running state: a row is what somebody
// decided to keep, and "save the running state, then push" are two acts the
// screen already has. A branch that moved is a refusal, never a force - the
// console offers a pull, which is a comparison an operator can read.
func (a *API) pushConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	row, ok := a.boundRemote(w, r, c)
	if !ok {
		return
	}
	driver, remote, ok := a.remote(w, r, row)
	if !ok {
		return
	}
	doc, err := config.Unmarshal([]byte(c.Document))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	body, assets, err := config.Split(doc)
	if err != nil {
		a.internal(w, err)
		return
	}
	files := map[string][]byte{config.BundleName: body}
	for name, content := range assets {
		files[name] = content
	}
	// The commit says WHO, in git's own terms. That is the point of carrying an
	// author at all: the audit trail already knows, and a repository whose
	// history reads "meerkat" for every change answers no question later.
	author, email := commitAuthor(row, actor)
	remote.AuthorName, remote.AuthorEmail = author, email
	msg := pushMessage(c, author)
	rev, err := driver.Commit(r.Context(), remote, msg, files, c.RemoteRev)
	if errors.Is(err, confrepo.ErrMoved) {
		writeErr(w, http.StatusConflict, fmt.Sprintf(
			"%s has moved since %s was last read from it: pull it into a copy and compare before pushing",
			row.Name, c.Name))
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, row.Name+": "+err.Error())
		return
	}
	if err := a.st.MarkConfigurationSynced(r.Context(), c.ID, rev); err != nil {
		a.internal(w, err)
		return
	}
	c.RemoteRev = rev
	a.auditEvent(r.Context(), actor, "configuration.push", "configuration", c.ID, c.Name, "",
		fmt.Sprintf("pushed to %s (%s, %s) at %s, %d files", row.Name, row.Branch,
			confrepo.Join(row.Dir, config.BundleName), rev, len(files)))
	writeJSON(w, http.StatusOK, c)
}

// pushMessage is the commit subject: what changed, and who said so.
func pushMessage(c store.Configuration, who string) string {
	return fmt.Sprintf("Update %s from Meerkat\n\nSaved configuration %q, pushed by %s.\n",
		c.Name, c.Name, who)
}

// commitAuthor is the person the commit is attributed to: the operator who
// clicked, with the location's configured author as the fallback.
//
// THE OPERATOR WINS, and that is the whole reason this feature is worth having
// in the product rather than in a script: the audit trail already records who
// changed what here, and a repository whose history reads "meerkat" for every
// commit answers nothing six months later.
//
// An account with no address of its own gets one that CANNOT be anybody: .invalid
// is reserved (RFC 2606) and can never be a real domain, so the history says
// "root, who has no address here" instead of guessing at one that might belong
// to a stranger. The first version refused outright, which was a dead end - a
// fresh installation's root has no email, and nobody gives one to the account
// just to be allowed to push.
func commitAuthor(row store.ConfigRemote, actor store.User) (string, string) {
	name := strings.TrimSpace(actor.Fullname)
	if name == "" {
		name = actor.Username
	}
	if email := strings.TrimSpace(actor.Email); email != "" {
		return name, email
	}
	if row.AuthorEmail != "" {
		who := row.AuthorName
		if who == "" {
			who = name
		}
		return who, row.AuthorEmail
	}
	return name, actor.Username + "@meerkat.invalid"
}

// dirOrRoot names a location's directory for a sentence a person reads.
func dirOrRoot(dir string) string {
	if dir == "" {
		return "the repository root"
	}
	return dir
}

// ── binding ──────────────────────────────────────────────────────────────────

// bindConfiguration points a row at a location, or at none.
//
// Changing the destination clears the revision (the store does it), because a
// revision is a fact about one branch and means nothing on another.
func (a *API) bindConfiguration(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := gitAvailable(); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	c, ok := a.configuration(w, r)
	if !ok {
		return
	}
	var body struct {
		RemoteID string `json:"remoteId"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	detail := "no longer bound to a git location"
	if body.RemoteID != "" {
		row, found := a.configRemoteByID(r.Context(), w, body.RemoteID)
		if !found {
			return
		}
		detail = "bound to " + row.Name
	}
	if err := a.st.BindConfiguration(r.Context(), c.ID, body.RemoteID); err != nil {
		a.internal(w, err)
		return
	}
	c.RemoteID, c.RemoteRev, c.RemoteAt = body.RemoteID, "", 0
	a.auditEvent(r.Context(), actor, "configuration.bind", "configuration", c.ID, c.Name, "", detail)
	writeJSON(w, http.StatusOK, c)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// configRemote resolves the {id} and answers the 404 itself.
func (a *API) configRemote(w http.ResponseWriter, r *http.Request) (store.ConfigRemote, bool) {
	return a.configRemoteByID(r.Context(), w, r.PathValue("id"))
}

func (a *API) configRemoteByID(ctx context.Context, w http.ResponseWriter, id string) (store.ConfigRemote, bool) {
	row, err := a.st.GetConfigRemote(ctx, id)
	if errors.Is(err, store.ErrConfigRemoteNotFound) {
		writeErr(w, http.StatusNotFound, "no such git location")
		return store.ConfigRemote{}, false
	}
	if err != nil {
		a.internal(w, err)
		return store.ConfigRemote{}, false
	}
	return row, true
}

// boundRemote resolves the location a configuration points at, refusing the
// unbound one with the cure rather than a 404: bind it, then pull.
func (a *API) boundRemote(w http.ResponseWriter, r *http.Request, c store.Configuration) (store.ConfigRemote, bool) {
	if c.RemoteID == "" {
		writeErr(w, http.StatusUnprocessableEntity,
			c.Name+" is not bound to a git location: choose one first")
		return store.ConfigRemote{}, false
	}
	row, err := a.st.GetConfigRemote(r.Context(), c.RemoteID)
	if errors.Is(err, store.ErrConfigRemoteNotFound) {
		writeErr(w, http.StatusUnprocessableEntity,
			c.Name+" points at a git location that no longer exists: choose one again")
		return store.ConfigRemote{}, false
	}
	if err != nil {
		a.internal(w, err)
		return store.ConfigRemote{}, false
	}
	return row, true
}

// remote is gitRemote with the refusal written to the response: the resolution
// itself lives once, in mcp_git.go, so a handler and a tool reach a repository
// through the same code.
func (a *API) remote(
	w http.ResponseWriter, r *http.Request, row store.ConfigRemote,
) (confrepo.Driver, confrepo.Remote, bool) {
	driver, target, err := a.gitRemote(r.Context(), row)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return nil, confrepo.Remote{}, false
	}
	return driver, target, true
}

func (a *API) remoteNameFree(w http.ResponseWriter, r *http.Request, name, exceptID string) bool {
	taken, err := a.st.ConfigRemoteNameTaken(r.Context(), name, exceptID)
	if err != nil {
		a.internal(w, err)
		return false
	}
	if taken {
		writeErr(w, http.StatusConflict, "a git location called "+name+" already exists")
		return false
	}
	return true
}

// remoteMeta is what the audit diffs. The token REFERENCE is in it - it names
// a vault entry, which is public - and there is no value anywhere to leave out.
func remoteMeta(row store.ConfigRemote) map[string]string {
	return map[string]string{
		"name": row.Name, "url": row.URL, "branch": row.Branch, "dir": row.Dir,
		"tokenRef": row.TokenRef, "tokenUser": row.TokenUser,
		"authorName": row.AuthorName, "authorEmail": row.AuthorEmail,
	}
}
