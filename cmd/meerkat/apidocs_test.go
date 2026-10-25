package main

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/admin"
	"github.com/softwarity/meerkat/internal/admin/apidocs"
	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/notifier"
	"github.com/softwarity/meerkat/internal/oidcprovider"
)

// The API reference is true, plane by plane (DOCS-01).
//
// The three specifications are written by hand: nothing generates an OpenAPI
// document from a Go handler that would say what a person needs to read. So
// what keeps them true is this test, over the same mount functions main calls
// (planes.go):
//
//   - every pattern a plane mounts is documented, or excluded below with the
//     reason it is not an API;
//   - every documented operation is mounted - an Enterprise one only in the
//     Enterprise binary, where its code is linked.
//
// The community binary mounts the Enterprise paths it answers with a refusal
// (the admin API of the notifications answers 404), so those are documented
// and checked in both.

// recorder collects the patterns a plane mounts.
type recorder struct{ patterns []string }

func (p *recorder) Handle(pattern string, _ http.Handler) { p.patterns = append(p.patterns, pattern) }
func (p *recorder) HandleFunc(pattern string, _ func(http.ResponseWriter, *http.Request)) {
	p.patterns = append(p.patterns, pattern)
}

// planeAPI is an admin API as main wires it, for what it mounts.
func planeAPI() *admin.API {
	api := admin.New(nil, nil, nil)
	api.OIDC = oidcprovider.New(oidcprovider.Deps{})
	api.Notify = notifier.New(notifier.Deps{})
	return api
}

func ready(http.ResponseWriter, *http.Request) {}

func dataSurface() []string {
	rec := &recorder{}
	mountData(rec, ready, auth.New(nil, nil), &gateway.Router{},
		oidcprovider.New(oidcprovider.Deps{}), notifier.New(notifier.Deps{}))
	return rec.patterns
}

func adminSurface() []string {
	rec := &recorder{}
	mountAdmin(rec, ready, auth.NewAdmin(nil, nil), planeAPI())
	return rec.patterns
}

func internalSurface() []string {
	rec := &recorder{}
	mountInternal(rec, planeAPI())
	return rec.patterns
}

// notAnAPI are the patterns a plane mounts and its specification leaves out,
// each with its reason. Keyed by apidocs.Key.
var notAnAPI = map[string]map[string]string{
	"admin": {
		"GET /apidocs":                             "this reference itself: the page",
		"GET /apidocs/":                            "this reference itself: the page",
		"GET /apidocs/assets/{}":                   "this reference itself: swagger-ui's files",
		"GET /apidocs/specs.json":                  "this reference itself: the list of its documents",
		"GET /apidocs/specs/meerkat-admin.json":    "this reference itself: a document it shows",
		"GET /apidocs/specs/meerkat-internal.json": "this reference itself: a document it shows",
		"GET /apidocs/specs/meerkat-data.json":     "this reference itself: a document it shows",
		"* /apidocs/call/{}/{}":                    "this reference itself: its Try it out on the internal and data planes",
		"GET /meerkat/links-preview":               "the console's preview of the account button's Applications menu, in a frame of the Portal screen",
		"GET /meerkat/portal-preview":              "Enterprise: the console's preview of the portal bar, in a frame of the Portal screen",
	},
	"internal": {},
	// The forms of the sign-in and profile pages: each is posted by the page
	// of the same path, and answers a page or a redirect.
	"data": {
		"POST /confirm/code":            "the code form of the registration's confirmation page",
		"POST /forgot-password":         "the form of the page GET /forgot-password",
		"POST /login":                   "the form of the page GET /login",
		"POST /login/code":              "the form of the page GET /login/code",
		"POST /login/code/verify":       "the code form the page GET /login/code leads to",
		"POST /logout":                  "the sign-out button of the profile, the account button and the page GET /logout; an application signs out through GET /logout",
		"POST /plugged":                 "the Continue button of the page GET /plugged",
		"POST /profile/avatar":          "the photo form of the page GET /profile",
		"POST /profile/dev-key":         "the form of the page GET /profile/dev/key",
		"POST /profile/disconnect/{}":   "the Disconnect button of the page GET /profile/authorities",
		"POST /profile/email":           "the address form of the page GET /profile",
		"POST /profile/email/code":      "the code form of the address change, on the profile",
		"POST /profile/leave":           "the confirmation of the page GET /profile/leave",
		"POST /profile/mfa":             "the forms of the page GET /profile/mfa (enrol, renew, disable, trusted browsers)",
		"POST /profile/passkeys/delete": "the Remove button of the passkeys pages",
		"POST /profile/password":        "the form of the page GET /profile/password",
		"POST /profile/sessions":        "the close buttons of the page GET /profile/sessions",
		"POST /profile/timezone":        "the time-zone form of the page GET /profile",
		"POST /profile/tokens":          "the form of the page GET /profile/tokens (mint, disable, delete a token)",
		"POST /register":                "the form of the page GET /register",
		"POST /reset-password":          "the form of the page GET /reset-password",
		"POST /reset-password/code":     "the code form of the forgotten-password flow",
		"POST /select-group":            "the chooser GET /select-group posts it, and the account button's group switch",
		"POST /select-tenant":           "the chooser GET /select-tenant posts it, and the account button's organisation switch",
		"POST /totp":                    "the form of the page GET /totp",
		"POST /totp-enroll":             "the form of the page GET /totp-enroll",
		"POST /totp/email":              "the 'mail me a code' button of the page GET /totp",
		"POST /update-password":         "the form of the page GET /update-password",
	},
}

// consoleSignIn are the control plane's sign-in and profile pages: the same
// pages, mounted by the same code, as the data plane's, and documented there.
// What only the control plane mounts of them - the agents' authorisation
// server - is an API, and documented here.
func consoleSignIn() map[string]bool {
	rec := &recorder{}
	auth.NewAdmin(nil, nil).Register(rec)
	data := map[string]bool{}
	for _, p := range dataSurface() {
		data[apidocs.Key(p)] = true
	}
	out := map[string]bool{}
	for _, p := range rec.patterns {
		if k := apidocs.Key(p); data[k] {
			out[k] = true
		}
	}
	return out
}

// preflight is the CORS answer a browser asks before a cross-origin call: it
// is the operation it precedes, already documented, and not one of its own.
func preflight(key string) bool { return strings.HasPrefix(key, "OPTIONS ") }

func TestEveryPlaneIsDocumentedAndOnlyWhatItMounts(t *testing.T) {
	for _, plane := range []struct {
		name    string
		spec    []byte
		surface []string
	}{
		{"admin", apidocs.AdminSpec, adminSurface()},
		{"internal", apidocs.InternalSpec, internalSurface()},
		{"data", apidocs.DataSpec, dataSurface()},
	} {
		t.Run(plane.name, func(t *testing.T) {
			ops, err := apidocs.Operations(plane.spec)
			if err != nil {
				t.Fatal(err)
			}
			excluded := notAnAPI[plane.name]
			var sameAsData map[string]bool
			if plane.name == "admin" {
				sameAsData = consoleSignIn()
			}
			mounted := map[string]bool{}
			var undocumented []string
			for _, p := range plane.surface {
				key := apidocs.Key(p)
				mounted[key] = true
				if preflight(key) {
					continue
				}
				if _, ok := ops[key]; ok {
					if why, both := excluded[key]; both {
						t.Errorf("%s is documented and excluded (%q): decide", key, why)
					}
					continue
				}
				if strings.HasPrefix(key, "* ") && documentsPath(ops, strings.TrimPrefix(key, "* ")) {
					continue
				}
				if _, ok := excluded[key]; !ok && !sameAsData[key] {
					undocumented = append(undocumented, key)
				}
			}
			sort.Strings(undocumented)
			for _, k := range undocumented {
				t.Errorf("%s is mounted on the %s plane and absent from its specification", k, plane.name)
			}
			for key, op := range ops {
				if mounted[key] || (op.Enterprise && !edition.Enterprise) {
					continue
				}
				if method, path, _ := strings.Cut(key, " "); mounted["* "+path] && method != "" {
					continue
				}
				t.Errorf("%s is documented on the %s plane and nothing mounts it", key, plane.name)
			}
			for key, why := range excluded {
				// "Enterprise:" says the pattern is mounted by Enterprise code.
				if !mounted[key] && (edition.Enterprise || !strings.HasPrefix(why, "Enterprise: ")) {
					t.Errorf("%s is excluded from the %s plane, which no longer mounts it", key, plane.name)
				}
				if strings.TrimSpace(why) == "" {
					t.Errorf("%s is excluded with no reason", key)
				}
			}
		})
	}
}

// documentsPath says whether any method of path is documented: a pattern
// without a method serves them all, and documenting the ones it answers is
// enough.
func documentsPath(ops map[string]apidocs.Op, path string) bool {
	for key := range ops {
		if _, p, _ := strings.Cut(key, " "); p == path {
			return true
		}
	}
	return false
}
