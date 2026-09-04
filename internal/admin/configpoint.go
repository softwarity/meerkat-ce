package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/store"
)

// The tape (CFG-06): a restore point whenever a change moves the
// configuration's fingerprint.
//
// The condition is the fingerprint, not the endpoint, and that is the whole
// design. Two things follow, and both are what makes this affordable to keep:
//
//   - an endpoint added later is covered without anyone remembering it, and a
//     write that changes nothing (a PUT of identical values) writes nothing;
//   - what is NOT configuration - creating an account, filling the vault,
//     opening a session - leaves no trace here at all, because none of it moves
//     the fingerprint. The tape talks about configuration by construction
//     rather than by discipline.

func (a *API) registerConfigPoints(mux Mux) {
	mux.Handle("GET /api/config/history", a.rootOnly(a.listConfigPoints))
	mux.Handle("GET /api/config/history/{id}", a.rootOnly(a.readConfigPoint))
	mux.Handle("GET /api/config/history/{id}/plan", a.rootOnly(a.configPointPlan))
	mux.Handle("POST /api/config/history/{id}/restore", a.rootOnly(a.restoreConfigPoint))
	mux.Handle("POST /api/config/history/{id}/save", a.rootOnly(a.saveConfigPoint))
}

// markConfigPoint is called after every authenticated admin request. It costs
// one export on a WRITE that succeeded, and nothing at all otherwise.
func (a *API) markConfigPoint(r *http.Request, actor store.User, status int) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || status >= 300 {
		return
	}
	// A background context: the point belongs to a change that has ALREADY
	// happened, so a client hanging up must not be what decides whether the
	// gateway can be rolled back to it.
	ctx := context.WithoutCancel(r.Context())
	if _, err := config.Record(ctx, a.st, actor.ID, a.lastAuditLabel(ctx, actor)); err != nil {
		a.pointFailed(err)
	}
}

// pointFailed: a tape that cannot be written must not break the change that
// was just made. It is logged, loudly, and the request that succeeded stays
// succeeded.
func (a *API) pointFailed(err error) {
	slog.Error("restore point not written", "err", err)
}

// lastAuditLabel borrows the words the audit trail has just written for this
// actor, so a point and its audit line never tell two stories. Empty when
// there is nothing to borrow - the point still exists, it is simply unlabelled.
func (a *API) lastAuditLabel(ctx context.Context, actor store.User) string {
	events, err := a.st.ListAuditEvents(ctx, store.AuditFilter{
		ActorID: actor.ID, Since: time.Now().Add(-time.Minute).Unix(), Limit: 1,
	})
	if err != nil || len(events) == 0 {
		return ""
	}
	ev := events[0]
	if ev.TargetName != "" {
		return ev.Action + " " + ev.TargetName
	}
	return ev.Action
}

// pointView adds what a list of points needs and a point does not hold: the
// name of the saved configuration whose document is byte for byte this one.
// It is how the tape shows where "TEST" was taken.
type pointView struct {
	store.ConfigPoint
	SavedAs string `json:"savedAs,omitempty"`
	// Current marks the LATEST point holding what is being served - where the
	// gateway is now. Live marks EVERY point holding it, which is a different
	// question and the one a Restore button has to ask: going back to a moment
	// the gateway is already in changes nothing, and offering it is offering a
	// button that does nothing.
	Current bool `json:"current,omitempty"`
	Live    bool `json:"live,omitempty"`
	// SameAs is when this exact state was first recorded, on a point that is
	// not the first of its kind. Going back produces one by definition, and two
	// identical lines with nothing to tell them apart is what makes a tape look
	// broken: this is what lets the screen say "back to 14:32" instead.
	SameAs int64 `json:"sameAs,omitempty"`
}

func (a *API) listConfigPoints(w http.ResponseWriter, r *http.Request, _ store.User) {
	points, err := a.st.ListConfigPoints(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	saved, err := a.st.ListConfigurations(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	byDigest := map[string]string{}
	for _, c := range saved {
		byDigest[c.Digest] = c.Name
	}
	live := ""
	if file, err := a.liveDocumentText(r.Context()); err == nil {
		live = store.DigestOf(file)
	}
	// Walked oldest first, so "the first time this state was recorded" is
	// answered by the walk itself rather than by a second query.
	firstSeen := map[string]int64{}
	for i := len(points) - 1; i >= 0; i-- {
		if _, seen := firstSeen[points[i].Digest]; !seen {
			firstSeen[points[i].Digest] = points[i].At
		}
	}
	// "Current" marks ONE line: the LATEST point holding the state being
	// served. After going back, two points hold it - the one restored and the
	// restoring one - and marking both would say the gateway is in two places
	// at once.
	marked := false
	out := make([]pointView, 0, len(points))
	for _, p := range points {
		view := pointView{ConfigPoint: p, SavedAs: byDigest[p.Digest], Live: p.Digest == live}
		if !marked && view.Live {
			view.Current, marked = true, true
		}
		if at := firstSeen[p.Digest]; at != p.At {
			view.SameAs = at
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, out)
}

// readConfigPoint serves a point as the text it is - pictures already out, by
// construction: a point never had any.
func (a *API) readConfigPoint(w http.ResponseWriter, r *http.Request, _ store.User) {
	p, ok := a.configPoint(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write([]byte(p.Document))
}

// configPointPlan says what going back to that moment would change, and
// changes nothing.
func (a *API) configPointPlan(w http.ResponseWriter, r *http.Request, _ store.User) {
	p, ok := a.configPoint(w, r)
	if !ok {
		return
	}
	doc, err := config.Unmarshal([]byte(p.Document))
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

// restoreConfigPoint puts the gateway back to that moment.
//
// A SWITCH, like setting a configuration as current: the point is a whole
// state, not a fragment, so what it does not carry goes. The pictures in place
// stay - a point never carried any, and what a document does not carry, it
// does not destroy.
//
// And going back is itself a change, so it leaves its own point: the tape
// never loses the state someone rolled away from.
func (a *API) restoreConfigPoint(w http.ResponseWriter, r *http.Request, actor store.User) {
	p, ok := a.configPoint(w, r)
	if !ok {
		return
	}
	doc, err := config.Unmarshal([]byte(p.Document))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	plan, err := config.Switch(r.Context(), a.st, doc)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.auditEvent(r.Context(), actor, "config.restore", "config", p.ID, pointName(p), "", summarise(plan))
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("restored, but the routing table could not be reloaded: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// saveConfigPoint moves a moment from the tape to the shelf: the crossing
// between the two, and the only way a point ever gets a name.
func (a *API) saveConfigPoint(w http.ResponseWriter, r *http.Request, actor store.User) {
	p, ok := a.configPoint(w, r)
	if !ok {
		return
	}
	if err := a.roomForAnother(r.Context()); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	name, ok := a.freeName(w, r, r.URL.Query().Get("name"), "")
	if !ok {
		return
	}
	c := store.Configuration{ID: newID(), Name: name, Document: p.Document}
	if err := a.st.SaveConfiguration(r.Context(), &c); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "configuration.create", "configuration", c.ID, c.Name, "",
		"saved from the restore point of "+pointName(p))
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) configPoint(w http.ResponseWriter, r *http.Request) (store.ConfigPoint, bool) {
	p, err := a.st.GetConfigPoint(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNoConfigPoint) {
		writeErr(w, http.StatusNotFound, "no such restore point")
		return store.ConfigPoint{}, false
	}
	if err != nil {
		a.internal(w, err)
		return store.ConfigPoint{}, false
	}
	return p, true
}

// liveDocumentText renders the running configuration the way the tape stores
// it - pictures out - so the two digests are comparable.
func (a *API) liveDocumentText(ctx context.Context) (string, error) {
	doc, _, err := config.Export(ctx, a.st)
	if err != nil {
		return "", err
	}
	stripped, err := config.WithoutImages(doc)
	if err != nil {
		return "", err
	}
	file, err := config.Marshal(stripped)
	if err != nil {
		return "", err
	}
	return string(file), nil
}

// pointName is how a moment reads in an audit line: the moment, and nothing
// else. Appending what the point was labelled made every restore line carry a
// sentence about a DIFFERENT change - the one that produced that state - which
// reads as though that change were being replayed.
func pointName(p store.ConfigPoint) string {
	return time.Unix(p.At, 0).UTC().Format("2006-01-02 15:04")
}
