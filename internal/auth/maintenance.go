package auth

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// The unavailable page (LIFE-05), as a page this gateway SERVES rather than a
// special case.
//
// It used to be a self-contained block of HTML in internal/routing, and every
// argument for branding, theme, layout and language on the sign-in page holds
// here word for word - more so, even: this is the page every visitor meets
// during an outage, and the one they will judge the installation on. So it
// goes through the same chrome as the rest of the flow, which is also how it
// gains the language and colour-scheme controls without a line of its own.
//
// The gateway cannot reach this package (internal/auth already imports
// internal/routing, so the other direction is a cycle). main hands the router
// this function instead, the way it hands it every other cross-package wire.

var maintenancePage = flowPage("maintenance", maintenanceBody)

// The headline states the FACT - nothing is being served - and never the
// reason. "Under maintenance" was a guess about why: an installation may be
// closed for a migration, an incident, a licence that lapsed or a decision
// taken this morning, and the gateway knows none of that. The reason is the
// operator's message below it, in their own words, and it is the only place
// a cause belongs.
type maintenanceData struct {
	flowChrome
	// Reason is the CHOSEN one, already translated. Free text used to sit here
	// and it was the wrong shape: this page is read in twenty languages, and a
	// sentence typed in a console is a sentence in one of them. The operator
	// picks from a closed list; the product says it in the visitor's language.
	Reason string
	// When is how long service is expected to be away, in the visitor's
	// language: within the hour, within a few hours, within a day or so.
	//
	// A ROUNDED duration and never a moment, for two reasons. The output would
	// otherwise be more precise than the input - an operator types "six
	// hours", an estimate, and a page answering "04:43" turns it into a
	// promise. And a promise to the minute is one a maintenance window
	// famously breaks: at 04:44 the page is lying to everybody. A duration
	// also needs no timezone and no script, and it shrinks on its own as the
	// page is served again.
	When string
	// Continue is where an administrator goes to look anyway, or empty for
	// everybody else, who has nowhere to go.
	Continue template.URL
	// SignedIn offers the way out. Only to somebody who HAS a session -
	// showing "sign out" to a visitor who never signed in is an invitation to
	// wonder what they are signed into.
	SignedIn bool
}

const maintenanceBody = `    <p class="maint-lead">{{.T.maintenanceLead}}</p>
    {{if .Reason}}<p class="maint-msg">{{.Reason}}</p>{{end}}
    <p class="maint-when">{{.When}}</p>
    <div class="maint-dot" aria-hidden="true"></div>
    {{if .Continue}}
    <p class="maint-note">{{.T.maintenanceAdmin}}</p>
    <p class="maint-go"><a href="{{.Continue}}">{{.T.maintenanceContinue}}</a></p>
    {{end}}
    {{if .SignedIn}}
    <form method="post" action="/logout" class="leave">
      <button type="submit">{{.T.signOut}}</button>
    </form>
    {{end}}
`

// reasonKey maps a stored reason to its catalogue entry. An unknown one says
// nothing rather than showing a key: a value written by a newer build, or by
// hand in the database, must not turn into debris on a visitor's screen.
func reasonKey(reason string) string {
	switch reason {
	case store.ReasonMaintenance:
		return "reasonMaintenance"
	case store.ReasonUpgrade:
		return "reasonUpgrade"
	case store.ReasonIncident:
		return "reasonIncident"
	}
	return ""
}

// ServeMaintenance writes the unavailable page. reason is one of the closed
// catalogue (empty says nothing), until is when service is expected back in
// Unix seconds (zero says soon), and continueURL is the administrator's way
// through, or "" when there is none to offer.
//
// 503 with the headers that go with it: no-store, or a cache in front keeps
// serving it after the switch is off, and a Retry-After so a well-behaved
// client has something to wait for.
func (h *Handler) ServeMaintenance(w http.ResponseWriter, r *http.Request, reason string, until int64, continueURL string) {
	chrome := h.flowData(r, "titleMaintenance")
	t := messages[chrome.Lang]

	said := ""
	if k := reasonKey(reason); k != "" {
		said = t[k]
	}
	// How long away, rounded, computed at every render - which is what keeps
	// it true: the setting is an INSTANT, so a page served three hours later
	// says "within the hour" without anybody touching anything.
	// The thresholds are set against the durations the console OFFERS, not
	// against the units themselves, and they have to hold on both sides. Too
	// low and eight hours inflates into "a day or two"; too high and a day
	// announced deflates into "a few hours" after one hour has passed, when
	// twenty-three are left. So: everything the console calls hours reads as
	// hours, a day reads as a day for most of its own window, and each phrase
	// only gives way once what is left really has crossed into the smaller
	// one - which is the point of recomputing it at every render.
	when := t["backSoon"]
	if until > 0 {
		switch left := time.Until(time.Unix(until, 0)); {
		case left <= 0:
			// Already past. It says nothing anybody can act on, and it is what
			// a forgotten switch looks like from outside.
		case left < time.Hour:
			when = t["backWithinHour"]
		case left < 12*time.Hour:
			when = t["backWithinHours"]
		case left < 36*time.Hour:
			when = t["backWithinDay"]
		default:
			when = t["backWithinDays"]
		}
	}

	// The action is ABSOLUTE here, unlike the other flow pages: they are served
	// at a fixed path, where a relative "logout" resolves to /logout. This page
	// answers for ANY path, and /some/deep/page would post to
	// /some/deep/logout.
	_, err := h.sm.Resolve(r.Context(), r)
	w.Header().Set("Retry-After", "300")
	writeFlow(w, maintenancePage, maintenanceData{
		flowChrome: chrome,
		Reason:     said,
		When:       when,
		Continue:   template.URL(continueURL), //nolint:gosec // built here from the request path, never carried in
		SignedIn:   err == nil,
	}, http.StatusServiceUnavailable)
}

// MaintenanceStripe is the reminder the gateway injects into every page an
// administrator sees while going through the maintenance (LIFE-05). It lives
// here rather than in internal/gateway for one reason: it lands on the DATA
// plane, whose pages are translated, and the catalogue is here.
//
// Self-contained and inline - it is injected into somebody else's HTML, which
// may have any stylesheet and any z-index, so it brings its own and borrows
// nothing.
func (h *Handler) MaintenanceStripe(r *http.Request) string {
	t := messages[prefsOf(r, h.offeredLanguages()).Lang]
	return `<style>
#mk-maint{position:fixed;top:0;left:0;right:0;z-index:2147483647;` +
		`display:flex;align-items:center;gap:10px;padding:5px 12px;` +
		`background:#b3261e;color:#fff;font:500 12px/1.4 system-ui,sans-serif;` +
		`box-shadow:0 1px 6px rgba(0,0,0,.35)}
#mk-maint a{color:#fff;text-decoration:underline}
#mk-maint .g{flex:1}
</style>
<div id="mk-maint" dir="` + Dir(prefsOf(r, h.offeredLanguages()).Lang) + `">
<span>` + inAnyEncoding(t["maintenanceStripe"]) + `</span>
<span class="g"></span>
<a href="?meerkat-through-maintenance=0">` + inAnyEncoding(t["maintenanceLeave"]) + `</a>
</div>`
}

// inAnyEncoding escapes the text for HTML and then turns everything above
// ASCII into numeric references.
//
// This block is injected into SOMEBODY ELSE'S document, whose encoding is
// theirs and often undeclared: an upstream answering "text/html" with no
// charset leaves the browser on its own default, and a UTF-8 "e-acute" written
// straight in comes out as two wrong letters. Numeric references are ASCII,
// which every encoding a browser will guess agrees on, and they decode to the
// right character whatever it guessed.
//
// It only applies to text WE put in a page we did not write. The pages this
// product serves itself declare their own charset and need none of this.
func inAnyEncoding(s string) string {
	var b strings.Builder
	for _, r := range template.HTMLEscapeString(s) {
		if r < 128 {
			b.WriteRune(r)
			continue
		}
		b.WriteString("&#" + strconv.Itoa(int(r)) + ";")
	}
	return b.String()
}
