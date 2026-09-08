package auth

import (
	"net/http"
)

// The refusal page (RBAC-06): signed in, and turned away anyway.
//
// The gateway used to answer that with http.Error - a naked line of text on a
// blank page, in a browser, for someone who did nothing wrong. The two
// refusals that HAD an answer already had a page: the organisation chooser
// when switching would help, the waiting room when there is no organisation at
// all. This is the third, and it says the same thing they do - what was
// wanted, and what this session can open instead.
//
// It is a page rather than a rendered 403 for the reason the other two are:
// the gateway redirects, internal/auth draws. A themed page needs the theme,
// the branding, the locale and the user button, and none of that belongs on
// the proxy path.
type refusedData struct {
	flowChrome
	// Why, as one sentence already chosen from the code the gateway sent.
	Reason string
	Public []publicLink
}

const refusedBody = `    <form onsubmit="return false">
      <p class="lead">{{.T.refusedLead}}</p>
      <p class="hint">{{.Reason}}</p>
      <p class="hint">{{.T.refusedHint}}</p>
    </form>
    {{if .Public}}<div class="public">
      <p class="lead">{{.T.continueWithout}}</p>
      <nav class="public-links">
        {{range .Public}}<a href="{{.Href}}">{{.Name}}</a>{{end}}
      </nav>
    </div>{{end}}
    <p class="back"><a href="/profile">{{.T.backToProfile}}</a></p>
`

var refusedPage = flowPage("refused", refusedBody)

// showRefused draws it. The reason arrives as a CODE, never as a sentence: a
// message in the query string is a message anyone can rewrite, and this page
// is reached by following a link.
func (h *Handler) showRefused(w http.ResponseWriter, r *http.Request) {
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil {
		// Not signed in: this page has nothing to tell them that the sign-in
		// page does not tell them better.
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	chrome := h.flowData(r, "titleRefused")
	reason := chrome.T["refusedOther"]
	switch r.URL.Query().Get("why") {
	case "tenant":
		reason = chrome.T["refusedTenant"]
	case "roles":
		reason = chrome.T["refusedRoles"]
	case "user":
		reason = chrome.T["refusedUser"]
	}
	writeFlow(w, refusedPage, refusedData{
		flowChrome: chrome,
		Reason:     reason,
		Public:     h.reachableLinks(r.Context(), sess),
	}, http.StatusForbidden)
}
