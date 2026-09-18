package auth

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// Flow-page localization (I18N): the language and the color scheme are USER
// preferences, persisted in cookies and applied server-side on every page of
// the flow - no flash, no JS framework. The language falls back to
// Accept-Language, the scheme to the system (CSS light-dark()).
const (
	langCookie   = "MEERKAT_LANG"   // a language code the catalogue holds
	schemeCookie = "MEERKAT_SCHEME" // "auto" | "light" | "dark"
)

// prefs is the resolved pair for one request.
type prefs struct {
	Lang   string // catalogue key
	Scheme string // "auto" | "light" | "dark"
}

// prefsOf resolves the request's preferences WITHIN the languages the
// integrator offers (I18N: the entry pages must match the target
// application's languages - configured in Application -> General).
func prefsOf(r *http.Request, offered []string) prefs {
	p := prefs{Lang: "", Scheme: "auto"}
	if c, err := r.Cookie(langCookie); err == nil && contains(offered, c.Value) {
		p.Lang = c.Value
	}
	if p.Lang == "" {
		p.Lang = matchAcceptLanguage(r.Header.Get("Accept-Language"), offered)
	}
	if c, err := r.Cookie(schemeCookie); err == nil {
		switch c.Value {
		case "light", "dark":
			p.Scheme = c.Value
		}
	}
	return p
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// matchAcceptLanguage picks the first OFFERED language of the header - enough
// for a small catalogue, no full RFC 4647 machinery. Nothing matches -> the
// integrator's first language.
func matchAcceptLanguage(header string, offered []string) string {
	for _, part := range strings.Split(header, ",") {
		lang := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if len(lang) >= 2 && contains(offered, lang[:2]) {
			return lang[:2]
		}
	}
	if len(offered) > 0 {
		return offered[0]
	}
	return "en"
}

// offeredLanguages is what the FLOW PAGES speak: the RIGHT JOIN of the
// application's locale pool (SettingLanguages) with the languages Meerkat
// actually embeds (the messages catalogue). An app locale Meerkat does not
// embed (e.g. vi) never reaches the flow pages; an empty pool falls back to
// English. Cached alongside the theme (same 5s staleness budget).
//
// The ADMIN plane is out of it, in English like the console it leads to. The
// pool belongs to the INTEGRATOR's application: letting it decide the language
// of Meerkat's own sign-in page meant an operator could be greeted in a
// language chosen for someone else's end users, one click before a console
// that speaks English anyway.
func (h *Handler) offeredLanguages() []string {
	if h.adminPlane {
		return []string{"en"}
	}
	h.themeMu.Lock()
	defer h.themeMu.Unlock()
	if time.Since(h.langsReadAt) < 5*time.Second && len(h.langsCache) > 0 {
		return h.langsCache
	}
	var appLangs []string
	_ = h.st.GetSetting(context.Background(), store.SettingLanguages, &appLangs)
	out := make([]string, 0, len(appLangs))
	for _, l := range appLangs {
		if _, ok := messages[l]; ok {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		out = []string{"en"}
	}
	h.langsCache = out
	h.langsReadAt = time.Now()
	return out
}

// imposedScheme returns the light/dark the integrator forces on the flow pages,
// or "" when the visitor decides (THEME-05). Cached like the theme.
func (h *Handler) imposedScheme() string {
	h.themeMu.Lock()
	defer h.themeMu.Unlock()
	if time.Since(h.schemeReadAt) < 5*time.Second {
		return h.schemeCache
	}
	var scheme string
	_ = h.st.GetSetting(context.Background(), store.SettingPagesScheme, &scheme)
	if scheme != "light" && scheme != "dark" {
		scheme = ""
	}
	h.schemeCache = scheme
	h.schemeReadAt = time.Now()
	return scheme
}

// passwordPolicy is the gateway-wide policy (AUTH-10), cached beside the theme
// with the same 5-second staleness budget: it is read on every page carrying a
// password field AND on every password submitted, so it must not be a database
// round trip each time.
func (h *Handler) passwordPolicy(ctx context.Context) store.PasswordPolicy {
	h.themeMu.Lock()
	defer h.themeMu.Unlock()
	if h.pwCacheSet && time.Since(h.pwReadAt) < 5*time.Second {
		return h.pwCache
	}
	h.pwCache = h.st.GetPasswordPolicy(ctx)
	h.pwReadAt = time.Now()
	h.pwCacheSet = true
	return h.pwCache
}

// pwRuleLabels maps a rule to its catalogue key. The label is a category and a
// number ("Digits: 2") rather than a sentence: a sentence would have to agree
// with the count, and getting "1 chiffre" and "2 chiffres" right in twenty
// languages is a plural table nobody asked for.
var pwRuleLabels = map[string]string{
	store.PasswordRuleLength:  "pwRuleLength",
	store.PasswordRuleLower:   "pwRuleLower",
	store.PasswordRuleUpper:   "pwRuleUpper",
	store.PasswordRuleDigit:   "pwRuleDigit",
	store.PasswordRuleSpecial: "pwRuleSpecial",
}

// passwordRuleView is one checklist line as the page draws it.
type passwordRuleView struct {
	Kind  string // what the browser script counts
	Need  int    // how many of them
	Label string // translated, with the number in it
}

// flowChrome is the shared model every flow page embeds: theme, branding and
// the request's language/scheme preferences.
type flowChrome struct {
	ThemeCSS template.CSS
	// Layout is the ARRANGEMENT (PAGE-02): its name becomes a class on <body>
	// and LayoutCSS the block that dresses it. Two fields for one thing on
	// purpose - the class is what a preview flips without reloading, the CSS
	// is what makes the class mean something.
	Layout    store.PageLayout
	LayoutCSS template.CSS
	// Preview marks the console's specimen. It is served INSIDE an iframe, and
	// a served page in an iframe goes compact - brand off, background off,
	// arrangement overridden. Here that is wrong twice over: the frame is a
	// scaled-down VIEWPORT, not somebody's portal, and it is the one place
	// whose whole job is showing what a full page looks like. Without this the
	// preview lost its background and every layout looked identical, which is
	// exactly what it was reported as.
	Preview bool
	Brand   brandView
	Title   string
	Lang    string   // <html lang>
	Dir     string   // <html dir>: rtl for Arabic and Hebrew, ltr otherwise
	Langs   []string // the offered languages (switcher hidden when 1)
	// Scheme drives the CSS (:root color-scheme); SchemeSwitch shows the
	// buttons. The ADMIN plane is Meerkat's console: dark only, no choice -
	// the theme/scheme options only ever concern the DATA plane's pages.
	Scheme       string
	SchemeSwitch bool
	// LangNames maps codes to endonyms for the language menu; SchemeIcon /
	// SchemeNext drive the single 3-state scheme button (auto->light->dark).
	LangNames  map[string]string
	SchemeIcon template.HTML
	SchemeNext string
	// UntilCookie names the readable session deadline for THIS plane: the two
	// ports carry different sessions, so a login page reading the other one
	// would leave a form nobody asked it to leave.
	UntilCookie string
	// PwRules is the password checklist (AUTH-10), on every page that asks for
	// a new one. Empty when the policy requires nothing.
	PwRules []passwordRuleView
	// PoweredBy shows the Meerkat mark at the foot of every served page. On
	// unless the white-label feature is licensed AND the integrator asked.
	PoweredBy bool
	// MarkText and MarkURL are what the mark says and where it points, from
	// the constants above - never spelt out in a template.
	MarkText string
	MarkURL  string
	// List widens the page: a sign-in form is two fields and deserves to be
	// narrow, a list of browsers is not and does not.
	List bool
	T    map[string]string // the locale's message catalogue
}

// withPasswordRules adds the checklist to a page's chrome. Called by the pages
// that ask for a NEW password - not by the sign-in page, which asks for the
// one that already exists and would only be telling an attacker the shape of
// what they are guessing.
func (h *Handler) withPasswordRules(ctx context.Context, c flowChrome) flowChrome {
	for _, r := range h.passwordPolicy(ctx).Rules("") {
		c.PwRules = append(c.PwRules, passwordRuleView{
			Kind:  r.Kind,
			Need:  r.Need,
			Label: fmt.Sprintf(c.T[pwRuleLabels[r.Kind]], r.Need),
		})
	}
	return c
}

// flowData assembles the chrome for one request: theme + branding + prefs.
func (h *Handler) flowData(r *http.Request, titleKey string) flowChrome {
	css, brand, layout := h.chrome()
	offered := h.offeredLanguages()
	p := prefsOf(r, offered)
	t := messages[p.Lang]
	chrome := flowChrome{
		ThemeCSS:     css,
		Layout:       layout,
		LayoutCSS:    template.CSS(layoutCSS(layout)), //nolint:gosec // a closed catalogue of built-in blocks
		Brand:        brand,
		Title:        t[titleKey],
		Lang:         p.Lang,
		Dir:          Dir(p.Lang),
		Langs:        offered,
		Scheme:       p.Scheme,
		SchemeSwitch: true,
	}
	chrome.T = t
	chrome.PoweredBy = !h.markHidden()
	chrome.MarkText, chrome.MarkURL = MarkText, MarkURL
	chrome.UntilCookie = session.UntilCookieName
	if h.adminPlane {
		chrome.UntilCookie = session.AdminUntilCookieName
		chrome.Scheme = "dark"
		chrome.SchemeSwitch = false
	} else if imposed := h.imposedScheme(); imposed != "" {
		// The application behind only knows one look: the pages in front of it
		// wear that one, and the button that would let a visitor break the
		// pairing goes away with the choice.
		chrome.Scheme = imposed
		chrome.SchemeSwitch = false
	}
	chrome.LangNames = langNames
	chrome.SchemeIcon = schemeIconSVG(chrome.Scheme)
	chrome.SchemeNext = map[string]string{"auto": "light", "light": "dark", "dark": "auto"}[chrome.Scheme]
	return chrome
}

// schemeIconSVG is the glyph the 3-state scheme button wears. Material Symbols
// rather than the Unicode sun and moon: those are drawn small and off-centre
// inside their own em box, and by how much depends on the system font - the
// same button looked right on one machine and shrunken on the next. The three
// paths are the ones the user button uses (userbtn.go), so the switch is the
// same picture on a flow page and inside the account menu.
func schemeIconSVG(scheme string) template.HTML {
	path, ok := schemePaths[scheme]
	if !ok {
		path = schemePaths["auto"]
	}
	return template.HTML(`<svg viewBox="0 -960 960 960" fill="currentColor" aria-hidden="true"><path d="` + path + `"/></svg>`) //nolint:gosec // built-in constants
}

var schemePaths = map[string]string{
	"auto":  `M480-80q-83 0-156-31.5T197-197q-54-54-85.5-127T80-480q0-83 31.5-156T197-763q54-54 127-85.5T480-880q83 0 156 31.5T763-763q54 54 85.5 127T880-480q0 83-31.5 156T763-197q-54 54-127 85.5T480-80Zm40-83q116-14 198-98.5T800-480q0-134-82-218.5T520-797v634Z`,
	"light": `M480-360q50 0 85-35t35-85q0-50-35-85t-85-35q-50 0-85 35t-35 85q0 50 35 85t85 35Zm0 80q-83 0-141.5-58.5T280-480q0-83 58.5-141.5T480-680q83 0 141.5 58.5T680-480q0 83-58.5 141.5T480-280ZM200-440H40v-80h160v80Zm720 0H760v-80h160v80ZM440-760v-160h80v160h-80Zm0 720v-160h80v160h-80ZM256-650l-101-97 57-59 96 100-52 56Zm492 496-97-101 53-55 101 97-57 59Zm-98-550 97-101 59 57-100 96-56-52ZM154-212l101-97 55 53-97 101-59-57Z`,
	"dark":  `M480-120q-150 0-255-105T120-480q0-150 105-255t255-105q14 0 27.5 1t26.5 3q-41 29-65.5 75.5T444-660q0 90 63 153t153 63q55 0 101-24.5t75-65.5q2 13 3 26.5t1 27.5q0 150-105 255T480-120Z`,
}

// The mark, in ONE place. The wording and the address are product identity,
// not page furniture: they are read by every flow page and by the branding
// preview, so changing them must be changing one line - not hunting through
// templates for a string that happens to be spelt the same in each.
const (
	MarkText = "powered by softwarity/meerkat"
	MarkURL  = "https://softwarity.io/meerkat"
)

// markHidden: the integrator asked for the mark to go AND holds the right to
// ask. Two conditions, because either alone is wrong - a licence that strips
// the mark from someone who never requested it, or a switch that works without
// the licence behind it.
func (h *Handler) markHidden() bool {
	if !edition.Enterprise {
		return false
	}
	var b store.Branding
	if err := h.st.GetSetting(context.Background(), store.SettingBranding, &b); err != nil {
		return false
	}
	return b.HideMark
}

// listChrome marks a page as a LIST: it gets the wider column. A sign-in form
// is two fields and reads better narrow; a table of browsers, dates and
// methods spends that width on an ellipsis instead.
func listChrome(c flowChrome) flowChrome {
	c.List = true
	return c
}

// tr translates one key for the request's language.
func (h *Handler) tr(r *http.Request, key string) string {
	return messages[prefsOf(r, h.offeredLanguages()).Lang][key]
}

// langNames are the endonyms shown by language pickers (never translated).
var langNames = map[string]string{
	"ar":      "العربية",
	"de":      "Deutsch",
	"en":      "English",
	"es":      "Español",
	"fr":      "Français",
	"he":      "עברית",
	"hi":      "हिन्दी",
	"id":      "Bahasa Indonesia",
	"it":      "Italiano",
	"ja":      "日本語",
	"ko":      "한국어",
	"nl":      "Nederlands",
	"pl":      "Polski",
	"pt":      "Português",
	"ru":      "Русский",
	"th":      "ไทย",
	"tr":      "Türkçe",
	"uk":      "Українська",
	"vi":      "Tiếng Việt",
	"zh-Hans": "简体中文",
}

// The flow pages' catalogue: ONE FILE PER LANGUAGE, embedded in the binary.
//
// It used to be a Go map. At two languages that read fine; at twenty it would
// be seven thousand lines in one file, and adding a language would mean
// editing the same lines everyone else edits. A file per language makes
// "speak Japanese" a file, reviewable on its own and diffable against English.
//
// English is the reference: every other catalogue is compared to it at startup
// (see the tests), because a missing key renders as an empty string on a page
// nobody is watching.
//
//go:embed locales/*.json
var localeFiles embed.FS

var messages = loadMessages()

func loadMessages() map[string]map[string]string {
	entries, err := localeFiles.ReadDir("locales")
	if err != nil {
		panic("auth: no locales embedded: " + err.Error())
	}
	out := make(map[string]map[string]string, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := localeFiles.ReadFile("locales/" + name)
		if err != nil {
			panic("auth: locale " + name + ": " + err.Error())
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			panic("auth: locale " + name + ": " + err.Error())
		}
		out[strings.TrimSuffix(name, ".json")] = m
	}
	// A key a catalogue does not carry falls back to English, filled in HERE
	// rather than at every lookup: half this package indexes the map directly
	// (t := messages[lang]; t[key]), so a fallback in one accessor would leave
	// the other half rendering an empty string - which is what a missing key
	// used to do, silently, on the page nobody tested in Thai.
	if en := out["en"]; en != nil {
		for lang, m := range out {
			if lang == "en" {
				continue
			}
			for key, text := range en {
				if _, ok := m[key]; !ok {
					m[key] = text
				}
			}
		}
	}
	return out
}

// rtl are the languages written right to left. The pages carry dir on <html>,
// so the whole layout mirrors - a flow page is text and form fields, which the
// browser lays out correctly on its own once it is told which way to read.
var rtl = map[string]bool{"ar": true, "he": true, "fa": true, "ur": true}

// Dir is the writing direction for a language: "rtl" or "ltr".
func Dir(lang string) string {
	if rtl[lang] {
		return "rtl"
	}
	return "ltr"
}
