package devtunnel

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/store"
	"golang.org/x/crypto/ssh"
)

// The sessions of the tunnel (DEV-13): who is plugged in now, and the one way
// to end it from the gateway.
//
// A session is one plug run - `plug -s checkout`, `plug -c` - with all its
// connections. Behind one Service those connections land on whichever replica
// the Service picks, and each replica's agent reports its own part of the run
// as a session of its own. So, as with the served names, a node's list travels
// to the others (store.TopicPlugSessions) and the list a page reads is merged
// by id: a session is open while any node holds part of it. What is told ONCE -
// the trail - is decided by the store, where the first hold opens the session
// and the last release ends it (store.HoldPlugSession).
//
// Ending one is said to every node (store.TopicPlugClose), unconditionally:
// each closes what it holds, and an agent that never saw the session still
// refuses it for a while, so a client that missed the notice cannot come back
// through another replica.

// Session is one plug run, as one node's agent holds it.
type Session struct {
	ID      string    `json:"id"`
	Who     string    `json:"who,omitempty"`
	Key     string    `json:"key,omitempty"` // the key's SHA256 fingerprint
	From    string    `json:"from,omitempty"`
	Version string    `json:"version,omitempty"`
	Since   time.Time `json:"since"`
}

// Control is the agent's hand, registered by the Enterprise package while its
// agent runs.
type Control interface {
	CloseSession(id, reason string) bool
	CloseKey(fingerprint, reason string) int
}

// Wiring is what the registry needs from the gateway around it to tell the
// trail and the other nodes. Set once by main; a registry without it (tests,
// the community image) keeps its lists and tells nobody.
type Wiring struct {
	Store *store.Store
	// Node is this gateway's id on the change bus, the one the others file
	// its reports under.
	Node string
	// Audit writes one event through the administrative funnel, so the
	// console's live channel hears about it.
	Audit func(context.Context, store.AuditEvent)
	// Close tells the other nodes to end a session ("session", id) or what a
	// key holds ("key", fingerprint).
	Close func(kind, target, reason string)
}

type remoteSessions struct {
	sessions []Session
	at       time.Time
}

// Close kinds, as they travel.
const (
	CloseBySession = "session"
	CloseByKey     = "key"
)

// sessionsKept is how long an ended session's row outlives it: long past the
// last report of any replica, and the trail keeps the story.
const sessionsKept = time.Hour

// Wire connects the registry to the trail and to the other nodes.
func (r *Registry) Wire(w Wiring) {
	r.mu.Lock()
	r.wiring = w
	r.mu.Unlock()
}

// RelaySessions is Relay for the sessions: publish carries this node's list.
func (r *Registry) RelaySessions(publish func([]Session)) {
	r.mu.Lock()
	r.publishSess = publish
	r.mu.Unlock()
}

// ReportSessions records what one OTHER node says it holds: its whole list.
func (r *Registry) ReportSessions(node string, sessions []Session, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteSessions[node] = remoteSessions{sessions: sessions, at: at}
}

// SetControl registers the running agent's hand, nil when it stops.
func (r *Registry) SetControl(c Control) {
	r.mu.Lock()
	r.control = c
	r.mu.Unlock()
}

// MySessions is this node's own list, which is what travels.
func (r *Registry) MySessions() []Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) tellSessions() {
	r.mu.RLock()
	pub := r.publishSess
	r.mu.RUnlock()
	if pub != nil {
		pub(r.MySessions())
	}
}

// Started is said by the agent as it starts: whatever this node held before -
// a previous process under the same name, killed - is let go, and the sessions
// nobody else holds end in the trail.
func (r *Registry) Started(ctx context.Context) {
	w := r.wired()
	if w.Store == nil {
		return
	}
	ended, err := w.Store.ReleaseNodePlugHolds(ctx, w.Node, store.PlugEndShutdown)
	if err != nil {
		slog.Warn("developer tunnel: the previous sessions of this node could not be let go", "err", err)
	}
	for _, ps := range ended {
		r.auditEnded(ctx, ps)
	}
}

// SessionOpened records part of a run held here, and says the run opened when
// no other node had said so.
func (r *Registry) SessionOpened(s Session) {
	r.mu.Lock()
	r.sessions[s.ID] = s
	r.mu.Unlock()
	slog.Info("a developer plugged in", "session", s.ID, "who", s.Who, "from", s.From, "version", s.Version)
	r.tellSessions()
	w := r.wired()
	if w.Store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, err := w.Store.HoldPlugSession(ctx, store.PlugSession{
		ID: s.ID, Who: s.Who, Key: s.Key, From: s.From, Version: s.Version, Since: s.Since.Unix(),
	}, w.Node)
	if err != nil {
		slog.Warn("developer tunnel: a session could not be recorded", "session", s.ID, "err", err)
		return
	}
	if first {
		r.audit(ctx, "plug.session.open", s.Who, s.ID, r.workstation(ctx, s.Who, s.Key), s.From,
			fmt.Sprintf("from %s, plug %s, key %s", orDash(s.From), orDash(s.Version), orDash(s.Key)))
	}
}

// SessionClosed records that this node no longer holds part of a run, and
// says the run ended when it was the last to.
func (r *Registry) SessionClosed(s Session, end, reason string) {
	r.mu.Lock()
	delete(r.sessions, s.ID)
	r.mu.Unlock()
	slog.Info("a developer's session ended here", "session", s.ID, "who", s.Who, "end", end, "reason", reason)
	r.tellSessions()
	w := r.wired()
	if w.Store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ps, last, err := w.Store.ReleasePlugSession(ctx, s.ID, w.Node, end, reason)
	if err != nil {
		slog.Warn("developer tunnel: a session's end could not be recorded", "session", s.ID, "err", err)
		return
	}
	if last {
		r.auditEnded(ctx, ps)
	}
}

// sweepHolds lets go of the holds of the nodes that went quiet - a replica
// that died with sessions open - and of the ended sessions' rows.
func (r *Registry) sweepHolds(ctx context.Context) {
	w := r.wired()
	if w.Store == nil {
		return
	}
	now := r.now()
	r.mu.RLock()
	alive := []string{w.Node}
	cutoff := now.Add(-r.StaleAfter)
	for node, rs := range r.remoteSessions {
		if !rs.at.Before(cutoff) {
			alive = append(alive, node)
		}
	}
	r.mu.RUnlock()
	ended, err := w.Store.ReleasePlugHolds(ctx, alive, cutoff.Unix(), store.PlugEndDropped)
	if err != nil {
		slog.Warn("developer tunnel: the holds of a silent node could not be let go", "err", err)
	}
	for _, ps := range ended {
		r.auditEnded(ctx, ps)
	}
	if err := w.Store.PurgePlugSessions(ctx, now.Add(-sessionsKept).Unix()); err != nil {
		slog.Warn("developer tunnel: ended sessions could not be purged", "err", err)
	}
}

// SessionView is one run as a page shows it: merged over the nodes, with the
// names it serves.
type SessionView struct {
	Session
	// Names are the cluster names this run serves; empty for `plug -c`, a
	// client that consumes without replacing anything.
	Names []Served `json:"names"`
	// Nodes is how many gateway replicas hold part of it.
	Nodes int `json:"nodes"`
}

// Sessions is what a page reads: every node's runs, merged by id.
func (r *Registry) Sessions() []SessionView {
	r.mu.RLock()
	byID := map[string]*SessionView{}
	add := func(s Session) {
		v, ok := byID[s.ID]
		if !ok {
			byID[s.ID] = &SessionView{Session: s, Names: []Served{}, Nodes: 1}
			return
		}
		v.Nodes++
		if s.Since.Before(v.Since) {
			v.Since = s.Since
		}
	}
	for _, s := range r.sessions {
		add(s)
	}
	cutoff := r.now().Add(-r.StaleAfter)
	for _, rs := range r.remoteSessions {
		if rs.at.Before(cutoff) {
			continue
		}
		for _, s := range rs.sessions {
			add(s)
		}
	}
	r.mu.RUnlock()
	for _, n := range r.List() {
		if v, ok := byID[n.Session]; ok && n.Session != "" {
			v.Names = append(v.Names, n)
		}
	}
	out := make([]SessionView, 0, len(byID))
	for _, v := range byID {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Since.Equal(out[j].Since) {
			return out[i].Since.Before(out[j].Since)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// CloseSession ends a run everywhere: here, and on every other node. Reports
// whether a node this one hears from held it.
func (r *Registry) CloseSession(id, reason string) bool {
	known := false
	for _, s := range r.Sessions() {
		if s.ID == id {
			known = true
			break
		}
	}
	r.ApplyClose(CloseBySession, id, reason)
	if w := r.wired(); w.Close != nil {
		w.Close(CloseBySession, id, reason)
	}
	return known
}

// CloseKey ends what a key holds, everywhere.
func (r *Registry) CloseKey(fingerprint, reason string) {
	r.ApplyClose(CloseByKey, fingerprint, reason)
	if w := r.wired(); w.Close != nil {
		w.Close(CloseByKey, fingerprint, reason)
	}
}

// ApplyClose ends, on THIS node, what another node or this one asked to end.
func (r *Registry) ApplyClose(kind, target, reason string) {
	r.mu.RLock()
	c := r.control
	r.mu.RUnlock()
	if c == nil {
		return
	}
	switch kind {
	case CloseBySession:
		c.CloseSession(target, reason)
	case CloseByKey:
		c.CloseKey(target, reason)
	}
}

func (r *Registry) wired() Wiring {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.wiring
}

// auditEnded tells the trail a run ended, how, and how long it lasted.
func (r *Registry) auditEnded(ctx context.Context, ps store.PlugSession) {
	detail := "ended: " + orDash(ps.End)
	if ps.Reason != "" {
		detail += " - " + ps.Reason
	}
	if ps.Since > 0 && ps.Ended >= ps.Since {
		detail += ", lasted " + (time.Duration(ps.Ended-ps.Since) * time.Second).String()
	}
	r.audit(ctx, "plug.session.close", ps.Who, ps.ID, r.workstation(ctx, ps.Who, ps.Key), ps.From, detail)
}

// auditServe tells the trail a name went to, or came back from, a run: the
// line that answers "who was serving checkout on Tuesday at three".
func (r *Registry) auditServe(action string, s Served) {
	if r.wired().Store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	detail := ""
	if action == "plug.serve" {
		parts := []string{}
		if len(s.Ports) > 0 {
			parts = append(parts, "ports "+strings.Join(s.Ports, ", "))
		}
		if s.Parked {
			parts = append(parts, "the deployed service set aside")
		}
		if s.Session != "" {
			parts = append(parts, "session "+s.Session)
		}
		detail = strings.Join(parts, "; ")
	}
	r.audit(ctx, action, s.Who, s.Session, s.Name, "", detail)
}

func (r *Registry) audit(ctx context.Context, action, who, targetID, targetName, ip, detail string) {
	w := r.wired()
	if w.Audit == nil {
		return
	}
	ev := store.AuditEvent{
		ActorName: who, Action: action, Target: "plug",
		TargetID: targetID, TargetName: targetName, IP: hostOf(ip), Detail: detail,
	}
	if who != "" && w.Store != nil {
		if u, err := w.Store.GetUserByUsername(ctx, who); err == nil {
			ev.ActorID = u.ID
		}
	}
	w.Audit(ctx, ev)
}

// workstation names the machine a key was deposited from - the comment of its
// line - else the fingerprint.
func (r *Registry) workstation(ctx context.Context, who, fingerprint string) string {
	w := r.wired()
	if w.Store == nil || who == "" || fingerprint == "" {
		return fingerprint
	}
	u, err := w.Store.GetUserByUsername(ctx, who)
	if err != nil {
		return fingerprint
	}
	keys, err := w.Store.ListDevKeys(ctx, u.ID)
	if err != nil {
		return fingerprint
	}
	for _, k := range keys {
		pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(k.Line))
		if err == nil && ssh.FingerprintSHA256(pub) == fingerprint && comment != "" {
			return comment
		}
	}
	return fingerprint
}

// hostOf is the address without its port, when it carries one.
func hostOf(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
