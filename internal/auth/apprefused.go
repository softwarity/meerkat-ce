package auth

import (
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

type appRefusedData struct {
	flowChrome
	// Reason is the chosen sentence, already translated.
	Reason string
}

const appRefusedBody = `    <form onsubmit="return false">
      <p class="lead">{{.T.appRefusedLead}}</p>
      <p class="hint">{{.Reason}}</p>
      <p class="hint">{{.T.appRefusedHint}}</p>
    </form>
`

var appRefusedPage = flowPage("app-refused", appRefusedBody)

// ServeAppRefused writes the page for one of the reasons above: 503 while the
// gateway cannot sign anyone in to an application, 400 for a request it will
// not answer. An unknown reason says the generic sentence rather than a key.
func (h *Handler) ServeAppRefused(w http.ResponseWriter, r *http.Request, why string) {
	chrome := h.flowData(r, "titleAppRefused")
	status := http.StatusBadRequest
	key := "appRefusedUnreadable"
	switch why {
	case AppRefusedUnknown:
		key = "appRefusedUnknown"
	case AppRefusedReturn:
		key = "appRefusedReturn"
	case AppRefusedUnavailable:
		key, status = "appRefusedUnavailable", http.StatusServiceUnavailable
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	writeFlow(w, appRefusedPage, appRefusedData{flowChrome: chrome, Reason: chrome.T[key]}, status)
}
