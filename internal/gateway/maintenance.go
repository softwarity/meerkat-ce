package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"html/template"
	"net/http"
	"net/url"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The global maintenance switch (LIFE-05).
//
// It answers 503 in place of THE ROUTES, and that boundary is the whole
// design:
//
//   - the gateway's OWN pages keep working. Sign-in, the second factor, the
//     profile: they are mounted beside this router, not inside it, so they are
//     untouched by construction. They have to be - the switch is lifted by
//     somebody who first has to sign in, and a maintenance that locks the door
//     it is opened from is a maintenance nobody can end.
//   - the CONTROL PLANE is a different port and a different mux. Nothing here
//     reaches it.
//   - whoever administers or develops here can GO THROUGH, to check the
//     application is really back before reopening it to everyone. But they go
//     through DELIBERATELY, and that is not a detail: the bypass used to be
//     automatic, so the one person who could not see the maintenance was the
//     one who had just turned it on - and a switch whose effect is invisible
//     to whoever flipped it stops being believed. They now get the same page
//     as a visitor, which is the truth of what a visitor gets, with a door at
//     the bottom.

// bypassCookie marks an administrator who has chosen to go through. It is only
// an INTENT: the session is still what grants it, so a copied cookie opens
// nothing.
//
// Its value is bound to the SESSION that took the door, not to a clock. A
// duration would be a number somebody picked - an hour of silent passage after
// one click, which is the automatic bypass this whole design replaced, just
// slower. Signing out and back in is a new session, so the page comes back
// with its door, and the decision is made again by whoever is making it now.
const bypassCookie = "meerkat_maintenance_bypass"

// bypassParam is the link on the page. A GET, because it changes nothing that
// outlives the browser that asked.
const bypassParam = "meerkat-through-maintenance"

// bypassFor is the cookie's value for one session: an HMAC of its token hash,
// so the mark cannot be moved to another session and the hash itself never
// leaves the server. The key is the gateway's own, shared between the nodes,
// which is what lets the door survive a load balancer changing its mind.
func (rt *Router) bypassFor(tokenHash string) string {
	mac := hmac.New(sha256.New, rt.simKey())
	mac.Write([]byte("maintenance|" + tokenHash))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))[:22]
}

// underMaintenance decides what this request gets: the page, the page with a
// door, or nothing at all.
func (rt *Router) underMaintenance(w http.ResponseWriter, req *http.Request) (*http.Request, maintenanceAnswer, bool) {
	rt.mu.RLock()
	m, page := rt.maintenance, rt.maintenancePage
	rt.mu.RUnlock()
	if !m.Enabled {
		return req, maintenanceAnswer{}, false
	}
	tokenHash, privileged := rt.maintenanceActor(req)
	if privileged && rt.hasBypass(req, tokenHash) {
		// Marked, so every HTML answer on the way back carries the stripe: an
		// administrator browsing an application that is down for everyone else
		// has to be reminded, on every page, or the door is just the old silent
		// bypass with an extra click.
		return marked(req), maintenanceAnswer{}, false
	}
	if carriesBypass(req) {
		// The mark no longer answers for this session - a sign-out, a new
		// login, a capability taken away. Dropped rather than left lying
		// around, so nothing can adopt it later.
		http.SetCookie(w, &http.Cookie{Name: bypassCookie, Path: "/", MaxAge: -1, HttpOnly: true})
	}
	if privileged {
		// The door, only for someone the session already privileges. A visitor
		// is never shown a way through, because there is none for them.
		return req, maintenanceAnswer{
			Reason:   m.Reason,
			Until:    m.Until,
			Continue: bypassURL(req),
			Fallback: routing.MaintenancePageWith(bypassDoor(req)),
		}, true
	}
	return req, maintenanceAnswer{Reason: m.Reason, Until: m.Until, Fallback: page}, true
}

// maintenanceAnswer is what this request gets: which of the reasons to give
// and when service is expected back, the administrator's way through (empty
// for a visitor), and the self-contained page for a router with no flow chrome
// wired.
type maintenanceAnswer struct {
	Reason   string
	Until    int64
	Continue string
	Fallback []byte
}

// htmlAttr escapes a URL for an href. The path comes from the request, so it
// is attacker-shaped by definition.
func htmlAttr(s string) string { return template.HTMLEscapeString(s) }

// bypassMark rides the request context so the response filter on every route
// can tell, without asking the database again, that this answer is being shown
// to somebody who went through the door.
type bypassMark struct{}

func marked(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), bypassMark{}, true))
}

func bypassing(ctx context.Context) bool {
	v, _ := ctx.Value(bypassMark{}).(bool)
	return v
}

// maintenanceStripe is the fallback reminder for a router with no chrome
// wired - the real one is translated, and lives with the catalogue in
// internal/auth. Fixed, thin, and impossible to mistake for the application's
// own: it borrows nothing from the page it lands on, because it lands on pages
// nobody here wrote.
const maintenanceStripe = `<style>
#mk-maint{position:fixed;top:0;left:0;right:0;z-index:2147483647;` +
	`display:flex;align-items:center;gap:10px;padding:5px 12px;` +
	`background:#b3261e;color:#fff;font:500 12px/1.4 system-ui,sans-serif;` +
	`box-shadow:0 1px 6px rgba(0,0,0,.35)}
#mk-maint a{color:#fff;text-decoration:underline}
#mk-maint .g{flex:1}
</style>
<div id="mk-maint">
<span>Under maintenance. Visitors get the unavailable page - you are through because you administer this gateway.</span>
<span class="g"></span>
<a href="?` + bypassParam + `=0">Leave</a>
</div>`

func (rt *Router) hasBypass(req *http.Request, tokenHash string) bool {
	c, err := req.Cookie(bypassCookie)
	if err != nil || c.Value == "" {
		return false
	}
	return hmac.Equal([]byte(c.Value), []byte(rt.bypassFor(tokenHash)))
}

// carriesBypass reports a cookie being present at all, whoever it belongs to.
// What it answers is "should this be cleared", not "should this open".
func carriesBypass(req *http.Request) bool {
	c, err := req.Cookie(bypassCookie)
	return err == nil && c.Value != ""
}

// bypassDoor is the footer offered to an administrator: what is happening, and
// the link that lets them look anyway.
func bypassDoor(req *http.Request) string {
	return `<p class="door">You administer this gateway, so you can look at the application anyway. ` +
		`Everyone else is seeing this page. <a href="` + htmlAttr(bypassURL(req)) + `">Continue anyway</a></p>`
}

// bypassURL is where the door leads: here, plus the parameter.
func bypassURL(req *http.Request) string {
	q := req.URL.Query()
	q.Set(bypassParam, "1")
	return (&url.URL{Path: req.URL.Path, RawQuery: q.Encode()}).String()
}

// takeBypass answers the door's link: it records the choice and sends the
// caller back to where they were, without the parameter - a URL carrying it
// would be pasted into a chat and handed to somebody it does not privilege
// anyway.
func (rt *Router) takeBypass(w http.ResponseWriter, req *http.Request) bool {
	want := req.URL.Query().Get(bypassParam)
	if want == "" {
		return false
	}
	// Leaving needs no privilege: anybody holding the cookie may drop it, and
	// refusing to let go of a mark is never the safe side.
	tokenHash, privileged := rt.maintenanceActor(req)
	if want != "0" && !privileged {
		return false
	}
	c := &http.Cookie{
		Name:     bypassCookie,
		Value:    rt.bypassFor(tokenHash),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// No MaxAge: it lives as long as the browser does, and no longer than
		// the session it is bound to.
	}
	if want == "0" {
		c.Value, c.MaxAge = "", -1
	}
	http.SetCookie(w, c)
	q := req.URL.Query()
	q.Del(bypassParam)
	back := (&url.URL{Path: req.URL.Path, RawQuery: q.Encode()}).String()
	http.Redirect(w, req, back, http.StatusSeeOther)
	return true
}

// maintenanceActor reports whether this request comes from someone who
// administers or develops here.
//
// The same shape as the simulation actor (simulate.go): an admin-plane session
// with a control-plane capability, or a data-plane session whose user
// administers or develops. Read live rather than cached in the compiled
// snapshot, because a capability revoked mid-maintenance has to bite at once.
func (rt *Router) maintenanceActor(req *http.Request) (tokenHash string, ok bool) {
	privileged := func(u store.User) bool {
		return u.Enabled && (u.Root || u.InfraAdmin || u.AppAdmin || u.Dev)
	}
	for _, sm := range []*session.Manager{rt.AdminSessions, rt.sm} {
		if sm == nil {
			continue
		}
		sess, err := sm.Resolve(req.Context(), req)
		if err != nil || sess.Pending != "" {
			continue
		}
		if u, err := rt.st.GetUserByID(req.Context(), sess.UserID); err == nil && privileged(u) {
			return sess.TokenHash, true
		}
	}
	return "", false
}

// serveMaintenance writes the unavailable page.
//
// The real one comes from the flow chrome (internal/auth): it is a page this
// gateway SERVES, so it wears the installation's theme, layout, mark and the
// visitor's language, exactly like the sign-in page beside it. This package
// cannot reach that one - internal/auth already imports internal/routing - so
// main hands it over, and what stays here is the fallback for a router wired
// without it, which is every test and nothing that ships.
func (rt *Router) serveMaintenance(w http.ResponseWriter, req *http.Request, a maintenanceAnswer) {
	if rt.Pages != nil {
		w.Header().Set("Cache-Control", "no-store")
		rt.Pages(w, req, a.Reason, a.Until, a.Continue)
		return
	}
	page := a.Fallback
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	// no-store, or a cache in front keeps serving the unavailable page after
	// the switch is off - which is how a maintenance outlives itself.
	h.Set("Cache-Control", "no-store")
	h.Set("Retry-After", "300")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write(page)
}

// renderMaintenance builds the visitor's page once, at reload, rather than per
// request. The administrator's carries a URL, so it is built per request.
func renderMaintenance(m store.Maintenance) []byte {
	if !m.Enabled {
		return nil
	}
	return routing.MaintenancePage()
}
