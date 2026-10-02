package auth

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/softwarity/meerkat/internal/mfa"
	"github.com/softwarity/meerkat/internal/store"
)

// The flow pages as the theme editor renders them (THEME-07).
//
// Twenty-nine pages wear a palette and the editor used to show one composite,
// so a tenant chooser that looked wrong was discovered in production. Each
// entry below pairs a template with the values it needs to draw - a fixture -
// and the picker offers exactly what is listed here.
//
// NOTHING IN THIS FILE CALLS A HANDLER, and that is the rule the whole feature
// rests on. A handler resolves a session, reads the store and sometimes
// writes: routed through one, a preview of "account confirmed" would spend a
// token, and a preview of the API tokens page would put somebody's real
// tokens in a screenshot. The template is rendered against made-up values, or
// it is not offered.
//
// The values are deliberately ORDINARY - "alice", "Acme", two passkeys - and
// deliberately not empty. An empty list renders the empty state, which is a
// real screen but not the one somebody tuning a palette wants to judge.

// PreviewPage is one flow page the editor can render: a key for the URL, a
// label for the picker, and how to build it.
type PreviewPage struct {
	Key string
	// Category groups the picker's filter toggles - two that cannot be told
	// apart by an icon are two that should have been one.
	Category string
	Label    string
	// title is the catalogue key of the page's own title.
	title string
	// build returns the template and the value to execute it with. The chrome
	// is handed in already themed, so a fixture only supplies its own half.
	build func(c flowChrome) (*template.Template, any)
}

// The categories the picker filters on. Not one per screen: two notions too
// close to carry distinct icons are two notions that should have been one, so
// the password and registration journeys live inside the sign-in one - they are
// the same journey seen from outside.
const (
	cSignIn    = "signin"
	cAccount   = "account"
	cDeveloper = "developer"
	cRefused   = "refused"
	// CategoryMessages is the mails' category. They are not pages, so they are
	// not in the list below - the admin package tags them with this.
	CategoryMessages = "messages"
	// CategoryPortal is the navigation bar. Not a page either: it is injected
	// into somebody else's HTML and draws itself in a shadow DOM, which is why
	// it sits beside the mails rather than among the pages. Before them,
	// because it HAS a dark half and a message does not.
	CategoryPortal = "portal"
)

// PreviewCategories is the closed list, in the order the toggles show it.
var PreviewCategories = []struct{ Key, Label, Icon string }{
	{cSignIn, "Signing in", "login"},
	{cAccount, "Account", "account_circle"},
	{cDeveloper, "Developer", "code"},
	{cRefused, "Refused", "block"},
	{CategoryPortal, "Portal", "dashboard"},
	{CategoryMessages, "Messages", "mail"},
}

// dress puts the page's own title on the chrome. Shared with the key map, which
// renders these same pages against a catalogue of markers: without it the
// twenty-four title strings came out of no screen at all, and a page's own
// title is the first thing somebody translating it would look for.
// all decides how many refusals go on the page. ONE while a palette or an
// arrangement is being judged - the error block is the one colour on these
// pages that is not the primary, so it has to be there, and four of them are
// four red boxes in the way of the question being asked. ALL on the Locale
// tab, where the wordings ARE the question.
func (p PreviewPage) dress(c flowChrome, all bool) flowChrome {
	if s := c.T[p.title]; s != "" {
		c.Title = s
	}
	keys := previewErrors[p.Key]
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if s := c.T[k]; s != "" {
			out = append(out, s)
			if !all {
				break
			}
		}
	}
	c.PreviewErrors = strings.Join(out, "\n")
	return c
}

// previewErrors is every refusal a page can give, page by page, as its HANDLER
// gives it - h.tr wherever that page is rendered again with a
// message.
//
// A served page shows one refusal at a time, and a preview shows the page once,
// so a wording that is not here has no screen it can be read on: of thirty
// error strings this product carries, two were reachable. The preview stacks
// them, the way the sign-in page already stacked its two.
//
// A wording with a %s keeps it: the value is a date here and a name there, so
// no filler would be right twice - and a translator reading the preview needs
// to know a value lands in that sentence.
//
// It is a table rather than a field on the page because the pages are written
// as positional literals; and it cannot go stale quietly -
// TestEveryRefusalHasAScreen reads the handlers and refuses a key that names no
// page here.
var previewErrors = map[string][]string{
	"login": {"errInvalidCreds", "errPasskey", "errTooManyAttempts", "errOutsideHours",
		"errNotInvited", "errSignInExpired", "errSignInUnavailable",
		// The access window closes on the sign-in page too, date and all.
		"errAccessEnded", "errAccessNotOpen", "errTemporaryExpired"},
	"totp":            {"errBadCode", "errTooManyAttempts"},
	"totp-enroll":     {"errBadCodeRetry"},
	"signin-code":     {"errBadEmail", "errTooManyAttempts"},
	"signin-sent":     {"errSigninCode", "errTooManyAttempts"},
	"select-tenant":   {"errTenantForbidden", "errTenantRefused"},
	"select-group":    {"errGroupForbidden"},
	"update-password": {"errPwPolicy", "errPwMismatch", "errPwReused"},
	// The same three, given through a helper that takes the key - which is why
	// the scan reads literals rather than h.tr call sites.
	"reset-password":      {"errPwPolicy", "errPwMismatch", "errPwReused"},
	"forgot-password":     {"errBadEmail", "errTooManyAttempts"},
	"register":            {"errRegisterMissing", "errPwMismatch", "errBadEmail", "errTooManyAttempts", "errCaptcha"},
	"profile":             {"errBadEmail", "errEmailTaken", "errBadTimezone", "errAvatarSize", "errAvatarType"},
	"profile-password":    {"errPwCurrentWrong", "errPwPolicy", "errPwMismatch", "errPwReused"},
	"profile-authorities": {"errSignInUnavailable", "errIdentityTaken", "errLastWayIn"},
	"profile-tokens":      {"errTokenName"},
	"profile-dev-key":     {"errBadKey"},
	"profile-mfa":         {"errMfaRequiredOff"},
}

// errorsWithNoPage are refusals answered as PLAIN TEXT, not as a page: a
// confirmation or reset link that has expired is answered by http.Error, so
// there is no screen to put them on and no preview that could show them.
var errorsWithNoPage = map[string]bool{
	"errConfirmExpired":     true,
	"errResetExpired":       true,
	"errEmailChangeExpired": true,
}

// PreviewPages is the catalogue, in the order the picker walks it: the sign-in
// flow first, then what an account holder sees, then the refusals.
var PreviewPages = []PreviewPage{
	// WITH the refusal showing. The same template renders the page without it,
	// so a second entry for that would be the same screen minus one line - and
	// a palette has to be judged on the line, which is the one colour on this
	// page that is not the primary.
	{"login", cSignIn, "Sign in", "titleSignIn", func(c flowChrome) (*template.Template, any) {
		// BOTH refusals, stacked. This page can give two - the credentials
		// were wrong, or the passkey ceremony failed - and the second is
		// written by script when it happens, so it is never on a served page
		// at the same time as the first. Showing one leaves the other with no
		// screen it can be read on, which is the same reason the report form
		// previews every message it can print.
		return loginPage, sampleLogin(c, c.PreviewErrors)
	}},
	{"totp", cSignIn, "Two-factor challenge", "titleTwoFactor", func(c flowChrome) (*template.Template, any) {
		return totpChallengePage, totpChallengeData{
			flowChrome: c, Error: c.PreviewErrors, AllowTrust: true,
			TrustLabel: fmt.Sprintf(c.T["trustDays"], 30),
			EmailOTP:   true,
		}
	}},
	{"totp-enroll", cSignIn, "Two-factor setup", "titleSetupTwoFactor", func(c flowChrome) (*template.Template, any) {
		return totpEnrollPage, totpEnrollData{
			flowChrome: c, Error: c.PreviewErrors,
			// A REAL code, for a made-up secret. A placeholder rectangle here
			// read as a QR that had failed to render - which is the one thing
			// somebody judging this page must not have to wonder about - and
			// the generator is offline anyway, so the honest version costs
			// nothing.
			QR:     sampleQR(c.Brand, sampleSecret),
			Secret: sampleSecret,
			Action: "/totp-enroll",
		}
	}},
	{"totp-scratch", cSignIn, "Two-factor backup codes", "titleSetupTwoFactor", func(c flowChrome) (*template.Template, any) {
		return totpEnrollPage, totpEnrollData{
			flowChrome: c, Action: "/totp-enroll", Mandatory: true,
			Scratch: []string{
				"4F2K-8QW1", "9XZC-2M7T", "B6RD-1HSP", "KJ40-77VE",
				"QA83-LM2N", "ZT51-6PWX", "3HGY-9DCK", "E7NB-4UFR",
			},
		}
	}},
	{"signin-code", cSignIn, "Sign-in code", "titleSigninCode", func(c flowChrome) (*template.Template, any) {
		return signinCodePage, signinCodeData{flowChrome: c, Next: "/", Error: c.PreviewErrors}
	}},
	{"signin-sent", cSignIn, "Sign-in code sent", "titleSigninCode", func(c flowChrome) (*template.Template, any) {
		return signinSentPage, signinSentData{
			flowChrome: c, Next: "/", Error: c.PreviewErrors,
			Hint: fmt.Sprintf(c.T["signinCodeSentHint"], minutesLabel(c.T, signinCodeTTL)),
		}
	}},
	{"select-tenant", cSignIn, "Organisation chooser", "titleChooseTenant", func(c flowChrome) (*template.Template, any) {
		return selectTenantPage, selectTenantData{
			flowChrome: c, Error: c.PreviewErrors,
			// Why the chooser is being shown, all three of them: it says one,
			// chosen from what refused the caller, and a preview of one leaves
			// the other two on no screen.
			Why: stack(c, "whyRoles", "whyOtherTenant", "whyNamed"),
			Tenants: []store.UserTenant{
				{TenantID: "acme", TenantName: "Acme Corporation", Type: "ADMIN", Enabled: true},
				{TenantID: "globex", TenantName: "Globex", Type: "USER", Enabled: true},
				{TenantID: "initech", TenantName: "Initech", Type: "USER", Enabled: true},
			},
		}
	}},
	{"select-group", cSignIn, "Group chooser", "titleChooseGroup", func(c flowChrome) (*template.Template, any) {
		return selectGroupPage, selectGroupData{
			flowChrome: c, Next: "/", Active: "g-support", Error: c.PreviewErrors,
			Groups: []store.Group{
				{ID: "g-support", Name: "Support"},
				{ID: "g-sales", Name: "Sales"},
				{ID: "g-ops", Name: "Operations"},
			},
		}
	}},
	{"update-password", cSignIn, "Password change required", "titleUpdatePassword", func(c flowChrome) (*template.Template, any) {
		d := updatePasswordData{flowChrome: c, Error: c.PreviewErrors}
		d.PwRules = samplePwRules(c)
		return updatePasswordPage, d
	}},
	{"forgot-password", cSignIn, "Forgotten password", "titleForgot", func(c flowChrome) (*template.Template, any) {
		return forgotPage, forgotData{flowChrome: c, Error: c.PreviewErrors}
	}},
	{"forgot-sent", cSignIn, "Reset link sent", "titleForgot", func(c flowChrome) (*template.Template, any) {
		return forgotSentPage, struct{ flowChrome }{c}
	}},
	{"reset-password", cSignIn, "Choose a new password", "titleReset", func(c flowChrome) (*template.Template, any) {
		d := resetData{flowChrome: c, Token: "SAMPLE", Error: c.PreviewErrors}
		// The served page calls withPasswordRules like the other two that ask
		// for a NEW password (reset.go). The fixture did not, so the one page
		// somebody reaches from a mail - with no other page to compare against -
		// previewed without the rules it really shows.
		d.PwRules = samplePwRules(c)
		return resetPage, d
	}},
	{"reset-done", cSignIn, "Password reset", "titleReset", func(c flowChrome) (*template.Template, any) {
		return resetDonePage, struct{ flowChrome }{c}
	}},
	// WITH the anti-robot check and the password checklist. Both are branches an
	// installation turns on, and a preview that takes neither left nine
	// wordings on no screen - which is how a checklist nobody could read ends
	// up shipped.
	{"register", cSignIn, "Create an account", "titleRegister", func(c flowChrome) (*template.Template, any) {
		d := registerData{
			flowChrome: c, Error: c.PreviewErrors, Username: "alice", Email: "alice@acme.example",
			Fullname:   "Alice Martin",
			CaptchaID:  "sample",
			CaptchaImg: sampleCaptcha(),
		}
		d.PwRules = samplePwRules(c)
		return registerPage, d
	}},
	{"register-sent", cSignIn, "Confirmation sent", "titleRegister", func(c flowChrome) (*template.Template, any) {
		return registerSentPage, struct{ flowChrome }{c}
	}},
	{"confirmed", cSignIn, "Account confirmed", "titleConfirmed", func(c flowChrome) (*template.Template, any) {
		return confirmedPage, struct{ flowChrome }{c}
	}},
	{"account-pending", cSignIn, "Waiting room", "titlePending", func(c flowChrome) (*template.Template, any) {
		return pendingPage, pendingData{flowChrome: c, Public: samplePublicLinks()}
	}},
	{"profile", cAccount, "Profile", "titleProfile", func(c flowChrome) (*template.Template, any) {
		return profilePage, profileData{
			flowChrome: c, Error: c.PreviewErrors, Initials: "AM", Username: "alice",
			Fullname: "Alice Martin", Email: "alice@acme.example",
			TenantName: "Acme Corporation", Timezone: "Europe/Paris",
			IsDev: true, Console: true, Apps: samplePublicLinks(),
		}
	}},
	{"profile-security", cAccount, "Security", "titleSecurity", func(c flowChrome) (*template.Template, any) {
		return profileSecurityPage, profileSecurityData{
			flowChrome: c, MFAEnrolled: true, PasskeysAllowed: true, APITokens: true,
			Passkeys:       samplePasskeys(),
			Authorities:    sampleAuthorities(),
			ConnectedCount: 1,
		}
	}},
	{"profile-password", cAccount, "Change password", "titleChangePassword", func(c flowChrome) (*template.Template, any) {
		d := profilePasswordData{flowChrome: c, Error: c.PreviewErrors}
		d.PwRules = samplePwRules(c)
		return profilePasswordPage, d
	}},
	// The same page where an ORGANISATION IMPOSES two-factor: it then says so
	// twice and drops the way out, which is three wordings the optional case
	// never shows. A screen of its own rather than a second state squeezed
	// into the first, the way the created token has one.
	{"profile-mfa-required", cAccount, "Two-factor imposed", "titleTwoFactor", func(c flowChrome) (*template.Template, any) {
		return profileMFAManagePage, profileMFAManageData{
			flowChrome:  c,
			StatusLine:  fmt.Sprintf(c.T["mfaStatus"], 6, 8),
			Required:    true,
			ScratchLeft: 6, AllowTrust: true,
			Trusted: []trustedView{
				{ID: "t1", Label: "Chrome on macOS", UntilLabel: fmt.Sprintf(c.T["untilDate"], "2026-10-24"), Current: true},
			},
		}
	}},
	{"profile-passkeys", cAccount, "Passkeys", "titlePasskeys", func(c flowChrome) (*template.Template, any) {
		return profilePasskeysPage, profilePasskeysData{flowChrome: c, Passkeys: samplePasskeys()}
	}},
	{"profile-mfa", cAccount, "Two-factor settings", "titleTwoFactor", func(c flowChrome) (*template.Template, any) {
		return profileMFAManagePage, profileMFAManageData{
			flowChrome: c, Error: c.PreviewErrors,
			// The status line as the page builds it, and OPTIONAL: that is the
			// common case, and the only one that offers to turn two-factor off.
			StatusLine:  fmt.Sprintf(c.T["mfaStatus"], 6, 8),
			ScratchLeft: 6, AllowTrust: true,
			Trusted: []trustedView{
				{ID: "t1", Label: "Chrome on macOS", UntilLabel: fmt.Sprintf(c.T["untilDate"], "2026-10-24"), Current: true},
				{ID: "t2", Label: "Safari on iPhone", UntilLabel: fmt.Sprintf(c.T["untilDate"], "2026-10-02")},
			},
		}
	}},
	{"profile-tokens", cAccount, "API tokens", "titleTokens", func(c flowChrome) (*template.Template, any) {
		// A DISABLED token beside a working one: the page says "disabled" and
		// offers "Enable" only in that state, and a list of two working tokens
		// shows neither.
		//
		// The just-created dialog is NOT shown, though it is the only place
		// "Copy your token now" and its warning appear. It is a modal over a
		// blurred page: showing it puts two strings on screen and hides eight,
		// which is the wrong trade on a page whose list is the subject.
		return apiTokensPage, apiTokensData{
			flowChrome: c, Error: c.PreviewErrors,
			// WITHOUT an active organisation: the page then names the context
			// "no active tenant" and warns what such a token carries. Nothing
			// is lost by showing that state rather than the other - the list
			// is the same - and two wordings had no screen at all.
			Context: c.T["tokenContextNone"], NoTenant: true,
			Durations: sampleDurations(c),
			Tokens: []apiTokenView{
				{ID: "t1", Name: "Nightly export", Prefix: "mk_7fQ2", Context: "Acme Corporation",
					Tenant: "Acme Corporation", Enabled: true,
					Expiry: c.T["until"] + " 2026-12-31", LastUsed: c.T["lastUsed"] + " 2026-09-23"},
				{ID: "t3", Name: "Old importer", Prefix: "mk_3zR8", Context: "Acme Corporation",
					Tenant: "Acme Corporation", Enabled: true, Expiry: c.T["expired"], Expired: true},
				{ID: "t2", Name: "Staging probe", Prefix: "mk_11bD", Context: c.T["tokenContextNone"],
					Enabled: false},
			},
		}
	}},
	// The token JUST CREATED, which is a modal over the list rather than a
	// state of it: four wordings live only there, and showing it on the entry
	// above would put two on screen and hide eight. A screen of its own, the
	// way the report form has one.
	{"profile-tokens-created", cAccount, "API token created", "titleTokens", func(c flowChrome) (*template.Template, any) {
		return apiTokensPage, apiTokensData{
			flowChrome: c, Context: "Acme Corporation",
			Created:   "mk_7fQ2_9TkP4wZs2XmA8bR1vQ6yE3nH5jL0dC",
			Durations: sampleDurations(c),
			Tokens: []apiTokenView{
				{ID: "t1", Name: "Nightly export", Prefix: "mk_7fQ2", Context: "Acme Corporation",
					Tenant: "Acme Corporation", Enabled: true, Expiry: "2026-12-31", LastUsed: c.T["lastUsed"] + " 2026-09-23"},
			},
		}
	}},
	{"profile-authorities", cAccount, "Connected accounts", "titleAuthorities", func(c flowChrome) (*template.Template, any) {
		return profileAuthoritiesPage, profileAuthoritiesData{
			flowChrome: c, Error: c.PreviewErrors, Authorities: sampleAuthorities(),
		}
	}},
	{"profile-sessions", cAccount, "Active sessions", "titleSessions", func(c flowChrome) (*template.Template, any) {
		return profileSessionsPage, profileSessionsData{flowChrome: c, Others: true, Sessions: []sessionView{
			{ID: "a", Label: "Chrome - macOS", Meta: "203.0.113.24", Since: "2026-09-29 08:12", Current: true},
			{ID: "b", Label: "Safari - iPhone", Meta: "198.51.100.7", Since: "2026-09-27 19:40"},
		}}
	}},
	{"profile-leave", cAccount, "Leave an organisation", "titleLeave", func(c flowChrome) (*template.Template, any) {
		return profileLeavePage, profileLeaveData{flowChrome: c, TenantName: "Acme Corp"}
	}},
	{"profile-history", cAccount, "Sign-in history", "titleHistory", func(c flowChrome) (*template.Template, any) {
		// ONE LINE PER METHOD, and the names come from the catalogue rather
		// than typed in English here. Eight ways in exist and this page is the
		// only one that names them, so a fixture with three hand-written
		// labels left five wordings on no screen at all - and made the other
		// three look translated when they were not.
		return profileHistoryPage, profileHistoryData{
			flowChrome: c,
			Events: []loginEventView{
				{Label: "Signed in", Method: c.T["methodTotp"],
					Meta: "203.0.113.24 - France", When: "today, 09:14", Current: true},
				{Label: "Signed in", Method: c.T["methodPasskey"],
					Meta: "203.0.113.24 - France", When: "yesterday, 18:02"},
				{Label: "Signed in", Method: c.T["methodPasskeyTotp"],
					Meta: "203.0.113.24 - France", When: "yesterday, 08:40"},
				{Label: "Signed in", Method: c.T["methodEmailCode"],
					Meta: "198.51.100.7 - Germany", When: "2 days ago, 21:05"},
				{Label: "Signed in", Method: c.T["methodEmailCodeTotp"],
					Meta: "198.51.100.7 - Germany", When: "2 days ago, 20:58"},
				{Label: "Signed in", Method: c.T["methodExternal"],
					Meta: "203.0.113.24 - France", When: "3 days ago, 11:22"},
				{Label: "Signed in", Method: c.T["methodExternalTotp"],
					Meta: "203.0.113.24 - France", When: "3 days ago, 09:03"},
				{Label: "Refused", Method: c.T["methodPassword"],
					Meta: "198.51.100.7 - Germany", When: "3 days ago, 02:41"},
			},
		}
	}},
	{"profile-dev", cDeveloper, "Developer", "", func(c flowChrome) (*template.Template, any) {
		c.Title = "Developer - Meerkat"
		return profileDevPage, profileDevData{
			flowChrome: c,
			Keys: []devKeyRow{
				{ID: "k1", Fingerprint: "SHA256:8Xq1oS7m2VbN4cR0pK9tLwEjYh3dFzAu6QsPgIrMxTk", Comment: "alice@workstation", Added: "2026-09-01"},
				{ID: "k2", Fingerprint: "SHA256:Qm3vT7dLp0cW9sFh2YxN6kJrB1uEaZoG4iRnHyXe5wA", Comment: "alice@laptop", Added: "2026-09-20"},
			},
		}
	}},
	{"profile-dev-key", cDeveloper, "Developer key", "", func(c flowChrome) (*template.Template, any) {
		c.Title = "plug key - Meerkat"
		// A plausible published address: the commands are the point of the
		// page, and without one they read "get@ install".
		return profileDevKeyPage, profileDevData{
			flowChrome: c, Error: c.PreviewErrors,
			PlugHost: "gateway.acme.io", PlugPort: store.DefaultPlugPort,
		}
	}},
	// Approving an AGENT (MCP-07): the page a browser opens when somebody
	// points Claude, or any MCP client, at this gateway. It was the one page
	// the picker did not offer, on the grounds that it needs a registered
	// client - which is true of the FLOW and false of the template. Nine
	// strings had no screen for the difference.
	{"oauth-consent", cDeveloper, "Agent approval", "", func(c flowChrome) (*template.Template, any) {
		d := consentData{
			flowChrome: c,
			ClientName: "Claude Code",
			// Carried through the form untouched by the real page, and never
			// dereferenced. A plausible one, so the field is not empty in a
			// view-source.
			Query: template.URL("response_type=code&client_id=mk-sample&code_challenge=E9M...&" +
				"redirect_uri=http%3A%2F%2F127.0.0.1%3A7391%2Fcallback"), //nolint:gosec // a fixture, never followed
			Scopes: []consentChoice{
				{Value: store.ScopeReadOnly, Label: "Read only", Detail: "Look at the gateway and run the testers. Change nothing.", Checked: true},
				{Value: store.ScopeFull, Label: "Read and change", Detail: "Everything you can do. Each change is recorded and can be undone."},
			},
		}
		d.List = true
		d.Title = "Connect an agent - Meerkat"
		return consentPage, d
	}},
	{"plugged", cDeveloper, "Served from a workstation", "titlePlugged", func(c flowChrome) (*template.Template, any) {
		return pluggedPage, pluggedData{
			flowChrome: c, Next: "/",
			Served: []pluggedGroup{
				{Who: "alice", Names: []string{"orders-api", "orders-ui"}},
				{Who: "bruno", Names: []string{"billing-ui"}},
			},
		}
	}},
	// EVERY reason, stacked. This page prints one of four sentences depending on
	// what turned the caller away, so a preview of one leaves three on no
	// screen - the same call as the sign-in page's two refusals.
	{"refused", cRefused, "Refused", "titleRefused", func(c flowChrome) (*template.Template, any) {
		return refusedPage, refusedData{
			flowChrome: c,
			Reason:     stack(c, "refusedRoles", "refusedTenant", "refusedUser", "refusedOther"),
			Public:     samplePublicLinks(),
		}
	}},
	// Same for the delay: the page picks one of five by how far away the end of
	// the window is, and five is the whole vocabulary of this line.
	{"maintenance", cRefused, "Unavailable", "titleMaintenance", func(c flowChrome) (*template.Template, any) {
		return maintenancePage, maintenanceData{
			flowChrome: c,
			Reason:     stack(c, "reasonMaintenance", "reasonIncident", "reasonUpgrade"),
			When:       stack(c, "backSoon", "backWithinHour", "backWithinHours", "backWithinDay", "backWithinDays"),
			// As an ADMINISTRATOR sees it: the two lines offering to look at
			// the application anyway exist only for them.
			Continue: "/", SignedIn: true,
		}
	}},
}

// loginView is the login page's data under a name. The handler declares it
// anonymously, which is fine where it is built once - but a preview has to
// fill the same shape, and an anonymous struct cannot be named twice. The two
// must stay in step: a field added there and missed here renders a login page
// with a missing half, which is precisely what a preview exists to reveal.
type loginView struct {
	flowChrome
	Next        string
	Error       string
	Public      []publicLink
	Passkeys    bool
	Register    bool
	Forgot      bool
	SigninCode  bool
	Providers   []externalProvider
	Credentials bool
	Shut        bool
}

// sampleLogin is the sign-in page with everything a gateway can offer turned
// on: an installation shows a subset, and a palette has to be judged against
// the busiest version, not the emptiest.
func sampleLogin(c flowChrome, errMsg string) loginView {
	return loginView{
		flowChrome: c, Next: "/", Error: errMsg,
		Public:     samplePublicLinks(),
		Passkeys:   true,
		Register:   true,
		Forgot:     true,
		SigninCode: true,
		Providers: []externalProvider{
			{ID: "acme-sso", Name: "Acme SSO", Kind: "oidc"},
			{ID: "github", Name: "GitHub", Kind: "github"},
		},
		Credentials: true,
	}
}

// sampleSecret is a made-up TOTP secret - valid base32, tied to no account.
// It is what the preview's QR encodes, so scanning it enrols an authenticator
// against nothing, which is the right outcome for a picture in an editor.
const sampleSecret = "JBSWY3DPEHPK3PXP"

// sampleQR draws the enrolment code the real page draws, for sampleSecret.
func sampleQR(brand brandView, secret string) template.URL {
	issuer := brand.AppName
	if issuer == "" {
		issuer = "Meerkat"
	}
	data, err := mfa.QRDataURI(mfa.ProvisioningURI(issuer, "alice", secret))
	if err != nil {
		return ""
	}
	return template.URL(data)
}

func samplePublicLinks() []publicLink {
	return []publicLink{{Name: "Billing", Href: "/billing"}, {Name: "Docs portal", Href: "/docs"}}
}

func samplePasskeys() []passkeyView {
	return []passkeyView{
		{ID: "k1", Label: "iPhone", Created: "2026-03-14", Current: true},
		{ID: "k2", Label: "YubiKey 5C", Created: "2025-11-02"},
	}
}

func sampleAuthorities() []authorityLink {
	return []authorityLink{
		{ID: "acme-sso", Name: "Acme SSO", Connected: true, Sole: true},
		{ID: "github", Name: "GitHub"},
	}
}

// WritePagePreview renders one catalogued page with an arbitrary theme, the way
// WriteThemePreview does for the specimen. ok is false for a key that is not in
// the catalogue - which is how an unknown template answers 404 rather than a
// blank frame.
func WritePagePreview(
	w http.ResponseWriter, key string, t store.Theme, b store.Branding, scheme string,
	l store.PageLayout, locale string, allErrors bool,
) bool {
	for _, p := range PreviewPages {
		if p.Key != key {
			continue
		}
		page, data := p.build(p.dress(previewChrome(t, b, scheme, l, locale), allErrors))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = page.Execute(w, data)
		return true
	}
	return false
}

// samplePwRules is the checklist every page that asks for a NEW password
// draws, one line per rule. It is rendered server-side from the policy, so a
// preview that declares no policy draws no checklist - and six wordings then
// live on no screen at all.
func samplePwRules(c flowChrome) []passwordRuleView {
	return []passwordRuleView{
		{Kind: store.PasswordRuleLength, Need: 12, Label: fmt.Sprintf(c.T["pwRuleLength"], 12)},
		{Kind: store.PasswordRuleLower, Need: 1, Label: fmt.Sprintf(c.T["pwRuleLower"], 1)},
		{Kind: store.PasswordRuleUpper, Need: 1, Label: fmt.Sprintf(c.T["pwRuleUpper"], 1)},
		{Kind: store.PasswordRuleDigit, Need: 2, Label: fmt.Sprintf(c.T["pwRuleDigit"], 2)},
		{Kind: store.PasswordRuleSpecial, Need: 1, Label: fmt.Sprintf(c.T["pwRuleSpecial"], 1)},
	}
}

// sampleCaptcha is a drawn-on picture rather than a real challenge: minting one
// would store a one-shot answer in the database every time somebody looked at a
// palette, which is the rule this whole file follows.
func sampleCaptcha() template.URL {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="44">` +
		`<rect width="120" height="44" fill="#eceff4"/>` +
		`<text x="12" y="31" font-family="monospace" font-size="24" fill="#3b4252" ` +
		`transform="rotate(-4 60 22)" letter-spacing="4">4172</text>` +
		`<path d="M4 30 Q40 12 118 26" stroke="#5e81ac" fill="none" stroke-width="2"/></svg>`
	return template.URL("data:image/svg+xml;utf8," + url.PathEscape(svg)) //nolint:gosec // a drawing, built here
}

// stack is every wording a line can carry, one per row.
//
// These pages print ONE sentence chosen from a handful - which refusal, which
// delay, which reason - so a preview of one state leaves the rest on no screen
// at all. The .error and .hint blocks keep newlines for exactly this, and it is
// the same call the sign-in page makes with its two refusals: a layout no
// served page reaches, against wordings nobody could otherwise read.
func stack(c flowChrome, keys ...string) string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if s := c.T[k]; s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "\n")
}

// sampleDurations is every validity the page offers, the labels from the
// catalogue: the list is fixed (apiTokenDurations), and a fixture naming three
// of the five left two wordings on no screen.
func sampleDurations(c flowChrome) []durationOption {
	out := make([]durationOption, 0, len(apiTokenDurations))
	for _, d := range apiTokenDurations {
		out = append(out, durationOption{Days: d.Days, Label: c.T[d.Label]})
	}
	return out
}
