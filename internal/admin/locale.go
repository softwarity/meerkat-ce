package admin

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/store"
)

// Editing the strings of the built-in pages (I18N).
//
// Twenty catalogues ship in the binary, nineteen of them missing fourteen keys,
// and nothing could be done about that from an installation. The editor puts a
// page's strings BESIDE the page, which is the whole point: three hundred keys
// in a list is how a catalogue ends up with holes, and the screen in front of
// you is not.
//
// What is stored is the difference, never a copy - see store.LocaleOverride for
// why. So everything here speaks in terms of "what this installation changed",
// and a reset is a delete.

func (a *API) registerLocaleEditor(mux Mux) {
	mux.Handle("GET /api/locales/{code}", a.appAdmin(a.getLocale))
	mux.Handle("PUT /api/locales/{code}", a.appAdmin(a.putLocale))
	mux.Handle("DELETE /api/locales/{code}", a.appAdmin(a.resetLocale))
	mux.Handle("GET /api/locales", a.appAdmin(a.listLocales))
}

// localeString is one editable string: what the product ships, what this
// installation shows, and the English wording a translator works from.
type localeString struct {
	Key string `json:"key"`
	// Value is what is rendered today - the override if there is one, ours
	// otherwise.
	Value string `json:"value"`
	// Embedded is what the product ships IN THIS LANGUAGE, empty when it ships
	// none. Shown beside an edited value so "put this one back" can be judged
	// before it is done - and empty is the honest answer when there is nothing
	// to put back.
	Embedded string `json:"embedded"`
	// Reference is the English wording: the placeholder a translator fills
	// against, because a key name is not a sentence.
	Reference string `json:"reference"`
	// Overridden marks a string this installation changed.
	Overridden bool `json:"overridden"`
	// Missing marks a key with no wording in this language: the page falls back
	// to English there. Never blank - English is the fallback on the data plane
	// too - so this is "left to translate", not "broken".
	Missing bool `json:"missing"`
	// Screens is how many previewable screens render this string. More than one
	// means correcting it here corrects it there: the catalogue holds one copy,
	// and the screens share it.
	Screens int `json:"screens"`
	// Group is what KIND of string this is - a title, a message, a label, a
	// hint, an error. The list is ordered by it, and the editor puts a heading
	// on each run, because thirty strings in alphabetical order is a list one
	// reads twice to find the button one is looking for.
	Group string `json:"group"`
}

// The five kinds a string can be, in the order a screen is read: its title,
// the sentence under it, the fields and buttons, the help beside them, and
// what it says when something goes wrong.
const (
	groupTitle   = "title"
	groupMessage = "message"
	groupLabel   = "label"
	groupHint    = "hint"
	groupError   = "error"
)

var groupOrder = map[string]int{
	groupTitle: 0, groupMessage: 1, groupLabel: 2, groupHint: 3, groupError: 4,
}

// localeGroup sorts one string into its kind.
//
// From the KEY first, because this catalogue names its strings with real
// discipline - err*, title*, *Hint, *Lead, *Subject - and a convention already
// honoured three hundred times is a better classifier than any guess about the
// text. From the English wording only when the name says nothing: a value that
// ends in a full stop or runs past a handful of words is a sentence, and a
// sentence is not a button.
//
// English and not the language being edited, deliberately: the kind of a string
// is a property of the string, not of its translation, so a screen must not
// reorder itself when one switches language.
func localeGroup(key, english string) string {
	low := strings.ToLower(key)
	for _, suffix := range []string{"failed", "required", "invalid", "expired", "toolarge", "unavailable", "denied"} {
		if strings.HasSuffix(low, suffix) {
			return groupError
		}
	}
	if strings.HasPrefix(low, "err") {
		return groupError
	}
	if strings.HasPrefix(low, "title") || strings.HasSuffix(low, "subject") || strings.HasSuffix(low, "heading") {
		return groupTitle
	}
	if strings.HasSuffix(low, "hint") || strings.HasSuffix(low, "note") || strings.HasPrefix(low, "why") {
		return groupHint
	}
	// A link is a label wherever it leads, and this catalogue says so in the
	// name. Without this "Forgot your password?" reads as a sentence and lands
	// among the messages, two groups from the button beside it.
	if strings.HasSuffix(low, "link") || strings.HasSuffix(low, "cta") {
		return groupLabel
	}
	for _, suffix := range []string{"lead", "intro", "outro", "blurb"} {
		if strings.HasSuffix(low, suffix) {
			return groupMessage
		}
	}
	if strings.HasPrefix(low, "refused") {
		return groupMessage
	}
	if t := strings.TrimSpace(english); t != "" {
		// A full stop, not a question mark: a question is very often a link
		// ("Forgot your password?"), and nine words rather than seven because
		// "Sign in with a code by e-mail" is a button.
		if strings.ContainsAny(t[len(t)-1:], ".!") || len(strings.Fields(t)) >= 9 {
			return groupMessage
		}
	}
	return groupLabel
}

type localeView struct {
	Code string `json:"code"`
	// Name is the language's own name, never translated.
	Name string `json:"name"`
	// Embedded says the product ships this language; false means somebody
	// added it here.
	Embedded bool `json:"embedded"`
	// Edited says this installation has changed at least one string in it.
	Edited bool `json:"edited"`
	// Holes counts the keys this language has no wording of its own for, ours or
	// theirs: what is left to translate. Those render in English.
	Holes int `json:"holes"`
}

// listLocales is every language this gateway can render, and how complete each
// one is. The count is what sends somebody to the editor in the first place.
func (a *API) listLocales(w http.ResponseWriter, r *http.Request, _ store.User) {
	layer, err := a.st.LocaleOverrides(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	ref := auth.ReferenceStrings()
	out := make([]localeView, 0, len(auth.KnownLanguages()))
	for _, code := range auth.KnownLanguages() {
		holes := 0
		for key := range ref {
			// OwnString, not EmbeddedString: the loader fills every catalogue
			// with English so no page renders blank, which leaves them all
			// looking complete. Asked that way, every language has zero left to
			// translate - and eighteen of them are missing fourteen strings.
			if auth.OwnString(code, key) == "" && layer[code][key] == "" {
				holes++
			}
		}
		out = append(out, localeView{
			Code:     code,
			Name:     auth.LanguageName(code),
			Embedded: auth.IsEmbeddedLanguage(code),
			Edited:   len(layer[code]) > 0,
			Holes:    holes,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// getLocale is one language's strings, for one template or for all of them.
//
// ?template= narrows it to what that screen renders, which is the point of the
// editor. Without it the whole catalogue comes back, which is what an export
// wants and what somebody filling a new language from scratch needs.
func (a *API) getLocale(w http.ResponseWriter, r *http.Request, _ store.User) {
	code := r.PathValue("code")
	layer, err := a.st.LocaleOverrides(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	keys, scope := localeKeys(r.URL.Query().Get("template"))
	ref := auth.ReferenceStrings()
	out := make([]localeString, 0, len(keys))
	for _, key := range keys {
		// What THIS language says, which is empty for a string it has no wording
		// for. The field is then empty with the English wording as its
		// placeholder, and the page behind renders that same English: the screen
		// and the preview tell the same story.
		own := auth.OwnString(code, key)
		value := own
		over, overridden := layer[code][key]
		if overridden && over != "" {
			value = over
		}
		out = append(out, localeString{
			Key: key, Value: value, Embedded: own, Reference: ref[key],
			Overridden: overridden && over != "",
			Missing:    value == "",
			Group:      localeGroup(key, ref[key]),
			Screens:    auth.ScreenCount(key),
		})
	}
	// By kind, then by name inside a kind. Alphabetical across the whole screen
	// puts the title between two buttons and the error at the top, which is a
	// list one reads twice to find what one came for.
	sort.SliceStable(out, func(i, j int) bool {
		gi, gj := groupOrder[out[i].Group], groupOrder[out[j].Group]
		if gi != gj {
			return gi < gj
		}
		return out[i].Key < out[j].Key
	})
	writeJSON(w, http.StatusOK, struct {
		Code    string         `json:"code"`
		Scope   string         `json:"scope"`
		Strings []localeString `json:"strings"`
	}{code, scope, out})
}

// localeKeys is which strings the editor is being asked for: one screen's, or
// the lot. An unknown template name falls back to everything rather than to an
// empty screen - a list one cannot explain is worse than a long one.
func localeKeys(template string) (keys []string, scope string) {
	if template == "" {
		return sortedStringKeys(auth.ReferenceStrings()), "all"
	}
	own, shell := auth.TemplateKeys(template)
	if len(own) == 0 {
		return sortedStringKeys(auth.ReferenceStrings()), "all"
	}
	out := append(append([]string{}, own...), shell...)
	sort.Strings(out)
	return out, template
}

func sortedStringKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// putLocale saves corrections. The body carries the strings that CHANGED, and
// an empty value resets that one key - so a screen can save what it edited
// without holding the other two hundred and ninety.
//
// full=true replaces the language's whole layer instead, which is what an
// import does: a file is the complete picture of what it carries, and merging
// it would leave behind entries the file deliberately dropped.
func (a *API) putLocale(w http.ResponseWriter, r *http.Request, actor store.User) {
	code := strings.TrimSpace(r.PathValue("code"))
	if !validLanguageTag(code) {
		writeErr(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("%q is not a language tag: expected a BCP 47 code such as fr, pt-BR or zh-Hans", code))
		return
	}
	var body struct {
		Entries map[string]string `json:"entries"`
		Full    bool              `json:"full"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed locale: "+err.Error())
		return
	}
	layer, err := a.st.LocaleOverrides(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	next := map[string]string{}
	if !body.Full {
		for k, v := range layer[code] {
			next[k] = v
		}
	}
	known := auth.ReferenceStrings()
	for k, v := range body.Entries {
		// A key the product does not define is a key no page reads: stored, it
		// would sit in an export forever with nothing to show it.
		if _, ok := known[k]; !ok {
			writeErr(w, http.StatusUnprocessableEntity,
				fmt.Sprintf("%q is not a string this product has: the catalogue is fixed, only its wordings are yours", k))
			return
		}
		if strings.TrimSpace(v) == "" || v == auth.OwnString(code, k) {
			// Equal to what we ship is not an override: keeping it would
			// freeze that string at today's wording and hide the next fix.
			delete(next, k)
			continue
		}
		next[k] = v
	}
	if err := a.st.SetLocaleOverride(r.Context(), code, next); err != nil {
		a.internal(w, err)
		return
	}
	a.applyLocales(r)
	a.auditEvent(r.Context(), actor, "locale.update", "locale", code, code, "",
		fmt.Sprintf("%d strings", len(next)))
	a.getLocale(w, r, actor)
}

// resetLocale puts a language back to what the product ships, whole.
func (a *API) resetLocale(w http.ResponseWriter, r *http.Request, actor store.User) {
	code := r.PathValue("code")
	if err := a.st.DeleteLocaleOverride(r.Context(), code); err != nil {
		a.internal(w, err)
		return
	}
	a.applyLocales(r)
	a.auditEvent(r.Context(), actor, "locale.reset", "locale", code, code, "", "")
	w.WriteHeader(http.StatusNoContent)
}

// applyLocales pushes the whole layer into the data plane. Whole rather than
// per-language because a push follows a read of the table, and a partial one
// would leave a language that was just reset still overridden.
func (a *API) applyLocales(r *http.Request) {
	layer, err := a.st.LocaleOverrides(r.Context())
	if err != nil {
		return
	}
	auth.SetLocaleOverrides(layer)
}

// validLanguageTag is a loose BCP 47 check: letters, then optional subtags of
// letters or digits after a hyphen. Loose on purpose - refusing a tag somebody
// legitimately uses would be worse than accepting an odd one, and nothing here
// resolves the tag against a registry.
func validLanguageTag(code string) bool {
	if len(code) < 2 || len(code) > 35 {
		return false
	}
	for i, part := range strings.Split(code, "-") {
		if part == "" {
			return false
		}
		for _, r := range part {
			letter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			// A digit is legal in a subtag but never in the primary one: "2fr"
			// is not a language, "es-419" is a region.
			digit := r >= '0' && r <= '9' && i > 0
			if !letter && !digit {
				return false
			}
		}
	}
	return true
}
