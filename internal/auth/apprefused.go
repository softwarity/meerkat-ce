package auth

import (
	"fmt"
	"html/template"
	"net/http"
)

// The page a person lands on when an application sent them here to sign in
// and the gateway cannot answer that application.
//
// It is the OpenID provider's page (SAUTH-05), drawn here for the reason the
// unavailable page is: it is a page this gateway SERVES, to somebody who did
// nothing wrong, and it deserves the theme, the brand and their language as
// much as the sign-in page does. The provider is Enterprise code and this
// package is not; main hands it this method, the way it hands the router the
// unavailable page.
//
// What the page says is for the PERSON: which side is wrong, and who fixes it.
// Never what the application sent - the client id, the return address, what
// is declared instead all go to the server log, where the administrator reads
// them. A return address echoed on a page is somebody else's text on this
// gateway's screen, and the visitor can do nothing with it anyway.

// The reasons a sign-in request from an application is refused on this page,
// a closed list: the provider names one, the page says it in the visitor's
// language.
const (
	// AppRefusedUnknown: the application is not one this gateway knows.
	AppRefusedUnknown = "unknown"
	// AppRefusedReturn: the application is known, the address it asked to
	// bring the person back to is not one declared for it.
	AppRefusedReturn = "return"
	// AppRefusedUnavailable: the gateway cannot sign anyone in to any
	// application yet - its public address is not set.
	AppRefusedUnavailable = "unavailable"
	// AppRefusedUnreadable: the request itself could not be read.
	AppRefusedUnreadable = "unreadable"
)

// appRefusedRefs are the references the page hands the person for their
// administrator: one word per reason, the word the documentation's table and
// the server log line are searched with. The fix itself (which screen of the
// console) is the administrator's to read there, never the visitor's.
var appRefusedRefs = map[string]string{
	AppRefusedUnknown:     "unknown-client",
	AppRefusedReturn:      "return-address",
	AppRefusedUnavailable: "no-public-name",
	AppRefusedUnreadable:  "unreadable-request",
}

type appRefusedData struct {
	flowChrome
	// Reason is the chosen sentence, already translated.
	Reason string
	// Ref is the sentence carrying the reference for the administrator.
	Ref string
	// The ways on, as on the not-found page: the applications this visitor
	// may open, the profile or the sign-in, the page they came from.
	Apps    []publicLink
	SignIn  bool
	Profile bool
	Back    template.URL
}

const appRefusedBody = `    <form onsubmit="return false">
      <h2 class="lead">{{.T.appRefusedLead}}</h2>
      <p class="hint">{{.Reason}}</p>
      <p class="hint">{{.T.appRefusedHint}}</p>
      <p class="hint ref">{{.Ref}}</p>
      {{if .Apps}}<div class="public">
        <nav class="public-links">
          {{range .Apps}}{{if .Head}}<span class="group">{{.Group}}</span>{{end}}<a href="{{.Href}}">{{.Name}}</a>{{end}}
        </nav>
      </div>{{end}}
      {{if .SignIn}}<a class="choice" href="/login">{{.T.signIn}}</a>{{end}}
    </form>
    {{if .Brand.Contact}}<p class="back"><a href="{{.Brand.Contact}}">{{.T.contactAdmin}}</a></p>{{end}}
    {{if .Back}}<p class="back"><a href="{{.Back}}">{{.T.back}}</a></p>{{end}}
    {{if .Profile}}<p class="back"><a href="/profile">{{.T.profile}}</a></p>{{end}}
`

var appRefusedPage = flowPage("app-refused", appRefusedBody)

// ServeAppRefused writes the page for one of the reasons above: 503 while the
// gateway cannot sign anyone in to an application, 400 for a request it will
// not answer. An unknown reason says the generic sentence rather than a key.
func (h *Handler) ServeAppRefused(w http.ResponseWriter, r *http.Request, why string) {
	chrome := h.flowData(r, "titleAppRefused")
	status := http.StatusBadRequest
	key := "appRefusedUnreadable"
	ref, ok := appRefusedRefs[why]
	if !ok {
		ref = appRefusedRefs[AppRefusedUnreadable]
	}
	switch why {
	case AppRefusedUnknown:
		key = "appRefusedUnknown"
	case AppRefusedReturn:
		key = "appRefusedReturn"
	case AppRefusedUnavailable:
		key, status = "appRefusedUnavailable", http.StatusServiceUnavailable
	}
	ctx := r.Context()
	sess, err := h.sm.Resolve(ctx, r)
	signedIn := err == nil && sess.Pending == ""
	data := appRefusedData{
		flowChrome: chrome,
		Reason:     chrome.T[key],
		Ref:        fmt.Sprintf(chrome.T["appRefusedRef"], ref),
		Apps:       h.openApps(r, h.visitorGrants(ctx, sess, signedIn)),
		SignIn:     !signedIn,
		Profile:    signedIn,
		Back:       template.URL(backFromReferer(r)), //nolint:gosec // same-host path only, see backFromReferer
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	writeFlow(w, appRefusedPage, data, status)
}
