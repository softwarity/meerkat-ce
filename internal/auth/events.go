package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/events"
)

// The live channel, mounted on the DATA plane: /meerkat/events.
//
// The endpoint's only job is to decide WHO this connection is talking to, and
// that decision is made here rather than in the events package because it is
// the one part that knows Meerkat's identity model. Everything downstream just
// publishes to a topic.
//
// A page keeps the connection for as long as it is open, so the grant is made
// once, at connect time, from the session as it stands. A capability granted
// while a page is open reaches it at its next load - which is also when its
// fetched state would have changed anyway.
func (h *Handler) registerEvents(mux *http.ServeMux) {
	mux.HandleFunc("GET /meerkat/events", h.events)
	mux.HandleFunc("POST /meerkat/events/echo", h.eventsEcho)
}

// Events is the hub publishers write to. It is on the handler because the
// handler is what the data plane already hands around; whoever produces
// events (today the developer tunnel, tomorrow the session watch) takes it
// from here rather than owning a hub of its own.
func (h *Handler) Events() *events.Hub { return h.hub }

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	// Anonymous pages get the channel too, and that is not an oversight: an
	// application may be public, and what TopicAll carries - a service being
	// served from someone's machine right now - changes what the page in front
	// of them is, whoever they are.
	topics := []string{events.TopicAll}
	if sess, err := h.sm.Resolve(r.Context(), r); err == nil && sess.Pending == "" {
		topics = append(topics, events.UserTopic(sess.UserID))
		if u, err := h.st.GetUserByID(r.Context(), sess.UserID); err == nil && h.st.DevAllowed(r.Context(), u) {
			topics = append(topics, events.TopicDev)
		}
	}

	sub, err := h.hub.Subscribe(topics...)
	if err != nil {
		// At capacity. The page holds the state it fetched at load and tries
		// again later, which is exactly the degradation the channel was
		// designed to be safe under.
		http.Error(w, "the live channel is at capacity", http.StatusServiceUnavailable)
		return
	}
	if err := events.Serve(w, r, sub); err != nil && !errors.Is(err, events.ErrNotWebSocket) {
		slog.Debug("live channel ended", "err", err)
	}
}

// eventsEcho sends a message back to the pages of whoever asked for it. It is
// the channel's own instrument: without it, the only way to see whether a
// socket really reached a browser through some proxy is to wait for a feature
// to have something to say.
//
// It can only reach the CALLER's own pages - the account's room, never the
// broadcast - so the worst it can do is lie to the person holding the keyboard,
// which is the definition of a debug tool. Behind the developer gate all the
// same, the same one the UI test mode uses (DEV-01): the installation offers
// the developer surface AND the account holds the capability.
func (h *Handler) eventsEcho(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		http.Error(w, "sign in with a developer account to use the channel echo", http.StatusUnauthorized)
		return
	}
	u, err := h.st.GetUserByID(r.Context(), sess.UserID)
	if err != nil || !h.st.DevAllowed(r.Context(), u) {
		http.Error(w, "the channel echo requires the dev capability, on an installation where developer mode is on", http.StatusUnauthorized)
		return
	}
	var msg events.Message
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&msg); err != nil {
		http.Error(w, "malformed body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(msg.Type) == "" {
		http.Error(w, "an event needs a type: it is what the page switches on", http.StatusUnprocessableEntity)
		return
	}
	h.hub.Publish(events.UserTopic(sess.UserID), msg)
	w.WriteHeader(http.StatusNoContent)
}
