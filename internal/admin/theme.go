package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/fonts"
	"github.com/softwarity/meerkat/internal/m3color"
	"github.com/softwarity/meerkat/internal/store"
)

// Theme administration (THEME-04): several saved themes, one active; the
// editor previews any of them as the REAL login page before activating.
// APPLICATION plane (RBAC-05): a theme is the product's visual identity - its
// name, tagline, logo and colours. The gateway merely SERVES those pages; who
// serves them is not who owns them.
func (a *API) registerThemes(mux Mux) {
	mux.Handle("GET /api/themes", a.appAdmin(a.listThemes))
	mux.Handle("GET /api/themes/presets", a.appAdmin(a.listPresets))
	mux.Handle("POST /api/themes", a.appAdmin(a.createTheme))
	mux.Handle("PUT /api/themes/{id}", a.appAdmin(a.updateTheme))
	mux.Handle("DELETE /api/themes/{id}", a.appAdmin(a.deleteTheme))
	mux.Handle("POST /api/themes/{id}/activate", a.appAdmin(a.activateTheme))
	mux.Handle("GET /api/themes/{id}/preview", a.appAdmin(a.previewTheme))
	mux.Handle("GET /api/themes/templates", a.appAdmin(a.listTemplates))
	mux.Handle("GET /api/themes/fonts", a.appAdmin(a.listFonts))
	mux.Handle("GET /api/branding", a.appAdmin(a.getBranding))
	mux.Handle("PUT /api/branding", a.appAdmin(a.putBranding))
}

// Branding (THEME-02) is GLOBAL - one application identity whatever theme is
// active - but it is edited on the same Theme screen.
func (a *API) getBranding(w http.ResponseWriter, r *http.Request, _ store.User) {
	b := store.DefaultBranding()
	if err := a.st.GetSetting(r.Context(), store.SettingBranding, &b); err != nil {
		b = store.DefaultBranding()
	}
	writeJSON(w, http.StatusOK, b)
}

func (a *API) putBranding(w http.ResponseWriter, r *http.Request, actor store.User) {
	old := store.DefaultBranding()
	_ = a.st.GetSetting(r.Context(), store.SettingBranding, &old)
	var b store.Branding
	if err := decodeStrict(r, &b); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed branding: "+err.Error())
		return
	}
	if err := store.SanitizeBranding(&b); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// Removing the mark is what the white-label feature grants. Refused here
	// rather than ignored: a switch that saves and does nothing is worse than
	// one that says why it cannot.
	if err := edition.Require("removing the Meerkat mark"); b.HideMark && err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SetSetting(r.Context(), store.SettingBranding, b); err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "theme.branding", "theme", "", "branding", "", old, b)
	writeJSON(w, http.StatusOK, b)
}

// listFonts is the typefaces a theme may choose (THEME-09) and the faces to
// draw them with: the editor sets each choice in its own font, which it can
// only do with the faces declared in the console's page.
func (a *API) listFonts(w http.ResponseWriter, _ *http.Request, _ store.User) {
	type family struct {
		Family   string `json:"family"`
		Kind     string `json:"kind"`
		Category string `json:"category"`
	}
	var names []string
	out := struct {
		Families []family `json:"families"`
		CSS      string   `json:"css"`
	}{Families: []family{}}
	for _, f := range fonts.Families() {
		out.Families = append(out.Families, family{f.Family, f.Kind, f.Category})
		names = append(names, f.Family)
	}
	out.CSS = fonts.FaceCSS(names...)
	writeJSON(w, http.StatusOK, out)
}

// listPresets returns the built-in starting palettes (THEME-04) - the console
// offers them under the "+" button so a deleted preset can be recreated.
func (a *API) listPresets(w http.ResponseWriter, _ *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, store.PresetThemes())
}

func (a *API) listThemes(w http.ResponseWriter, r *http.Request, _ store.User) {
	themes, err := a.st.ListThemes(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	if themes == nil {
		themes = []store.Theme{}
	}
	writeJSON(w, http.StatusOK, themes)
}

func (a *API) createTheme(w http.ResponseWriter, r *http.Request, actor store.User) {
	var t store.Theme
	if err := decodeStrict(r, &t); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed theme: "+err.Error())
		return
	}
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "theme name is required")
		return
	}
	t.ID = newID()
	t.Active = false // activation is an explicit, separate act
	// Nothing to make it from: it starts as the default does, from colours.
	if !t.Generated() && len(t.Dark) == 0 && len(t.Light) == 0 {
		base := store.DefaultTheme()
		t.Colors, t.Contrast, t.ColorMatch = base.Colors, base.Contrast, base.ColorMatch
	}
	if err := a.st.SaveTheme(r.Context(), t); err != nil {
		if conflict(w, err) {
			return
		}
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	saved, err := a.st.GetTheme(r.Context(), t.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "theme.create", "theme", saved.ID, saved.Name, "", "")
	writeJSON(w, http.StatusCreated, saved)
}

func (a *API) updateTheme(w http.ResponseWriter, r *http.Request, actor store.User) {
	var t store.Theme
	if err := decodeStrict(r, &t); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed theme: "+err.Error())
		return
	}
	t.ID = r.PathValue("id")
	current, err := a.st.GetTheme(r.Context(), t.ID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "theme not found")
		return
	}
	t.Active = current.Active // active-ness only changes through /activate
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "theme name is required")
		return
	}
	if err := a.st.SaveTheme(r.Context(), t); err != nil {
		if conflict(w, err) {
			return
		}
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	saved, err := a.st.GetTheme(r.Context(), t.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	a.auditUpdate(r.Context(), actor, "theme.update", "theme", saved.ID, saved.Name, "", current, saved)
	writeJSON(w, http.StatusOK, saved)
}

func (a *API) deleteTheme(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	theme, _ := a.st.GetTheme(r.Context(), id) // capture the name before deletion
	existed, err := a.st.DeleteTheme(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if !existed {
		writeErr(w, http.StatusNotFound, "theme not found")
		return
	}
	a.auditEvent(r.Context(), actor, "theme.delete", "theme", id, theme.Name, "", "")
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) activateTheme(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := a.st.ActivateTheme(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "theme not found")
			return
		}
		a.internal(w, err)
		return
	}
	t, err := a.st.GetActiveTheme(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "theme.activate", "theme", t.ID, t.Name, "", "")
	writeJSON(w, http.StatusOK, t)
}

// previewTheme renders the flow-page specimen in the given theme, one scheme
// forced (?scheme=dark|light) - the console's editor iframes it twice.
func (a *API) previewTheme(w http.ResponseWriter, r *http.Request, _ store.User) {
	t, err := a.previewSubject(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	// The colours on screen, before they are saved. A page takes them live
	// over postMessage; a mail and the portal bar cannot - the first is inline
	// styles, the second a shadow DOM fed by a payload - so their frame is
	// reloaded with the draft, and generated here by the generator a save
	// would use.
	if raw := r.URL.Query().Get("draft"); raw != "" {
		if err := applyDraft(&t, raw); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
	}
	b := store.DefaultBranding()
	if err := a.st.GetSetting(r.Context(), store.SettingBranding, &b); err != nil {
		b = store.DefaultBranding()
	}
	// The layout in force, unless the console names another: the Layout tab
	// has to show an arrangement BEFORE it is saved, which is the whole point
	// of picking one from a gallery.
	l := store.DefaultPageLayout()
	if err := a.st.GetSetting(r.Context(), store.SettingPageLayout, &l); err != nil {
		l = store.DefaultPageLayout()
	}
	if q := r.URL.Query().Get("layout"); q != "" {
		l = store.PageLayout{Name: q, Side: r.URL.Query().Get("side")}
	}
	// The language every template below speaks. One parameter for the pages and
	// the mails alike: a preview is of ONE language, chosen outside the frame,
	// the same way each pane is of one scheme.
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "en"
	}
	// A TEMPLATE other than the flow specimen: a mailed message, rendered with
	// the palette on screen rather than the one in force. The editor is looking
	// at a theme that may be neither saved nor active, and a preview of the
	// active theme would answer a question nobody asked.
	//
	// What is rendered is the SAMPLE - fake values, real rendering - never the
	// real message: previewing "account confirmation" must not mint a token,
	// and previewing a list must not read anybody's data.
	if kind, ok := strings.CutPrefix(r.URL.Query().Get("template"), "mail:"); ok {
		msg, found := auth.SampleMailWith(r.Context(), a.st, kind, locale, originOf(r), t.MailPalette())
		if !found {
			writeErr(w, http.StatusNotFound, "unknown mail template "+kind+
				" (known: "+strings.Join(mailTemplateKeys(), ", ")+")")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(msg.HTML))
		return
	}
	// The portal bar. Not a page and not a message: the REAL component, in the
	// preview mode the portal editor already drives it with, against made-up
	// applications - never this installation's catalogue, which would put it in
	// a palette screenshot.
	if key := r.URL.Query().Get("template"); strings.HasPrefix(key, auth.PortalPreviewPrefix) {
		if auth.WritePortalPreview(w, key, t, b, r.URL.Query().Get("scheme"), locale) {
			return
		}
		writeErr(w, http.StatusNotFound, "unknown portal template "+key)
		return
	}
	// A named flow page rather than the specimen. Same rule as the mails: the
	// TEMPLATE against a fixture, never the handler.
	if key := r.URL.Query().Get("template"); key != "" && key != "specimen" {
		// errors=all stacks every refusal the page can give. The Locale tab asks
		// for it, because the wordings are what it is about; the other tabs get
		// one, so the error colour is on the page without four red boxes
		// standing in front of the arrangement.
		allErrors := r.URL.Query().Get("errors") == "all"
		if auth.WritePagePreview(w, key, t, b, r.URL.Query().Get("scheme"), l, locale, allErrors) {
			return
		}
		writeErr(w, http.StatusNotFound, "unknown template "+key)
		return
	}
	auth.WriteThemePreview(w, t, b, r.URL.Query().Get("scheme"), l, locale)
}

// themeDraft is what the editor holds and has not saved: the source colours
// and the switches that make the palettes.
type themeDraft struct {
	Colors     m3color.Core     `json:"colors"`
	Contrast   string           `json:"contrast"`
	ColorMatch bool             `json:"colorMatch"`
	Flat       bool             `json:"flat"`
	Fonts      store.ThemeFonts `json:"fonts"`
}

func applyDraft(t *store.Theme, raw string) error {
	var d themeDraft
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return fmt.Errorf("malformed draft (allowed: colors, contrast, colorMatch, flat, fonts): %w", err)
	}
	if d.Colors.Primary == "" {
		return errors.New("draft: colors.primary is required")
	}
	t.Colors, t.Contrast, t.ColorMatch, t.Flat, t.Fonts = d.Colors, d.Contrast, d.ColorMatch, d.Flat, d.Fonts
	if err := t.Generate(); err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	return nil
}

// previewSubject resolves what a preview is OF: a stored theme, or one of the
// built-in palettes the console's picker also offers.
//
// The picker hands those over under a namespaced id ("preset:<id>") because a
// copy inherits its source's id and the two must stay two pills - which means
// the id arriving here is not always a row. Resolved rather than refused: a
// built-in is a palette one looks at and copies, and looking at it is the
// point of a preview.
func (a *API) previewSubject(ctx context.Context, id string) (store.Theme, error) {
	if key, ok := strings.CutPrefix(id, presetIDPrefix); ok {
		for _, p := range store.PresetThemes() {
			if p.ID == key {
				return p, nil
			}
		}
		return store.Theme{}, fmt.Errorf("no built-in palette %q", key)
	}
	t, err := a.st.GetTheme(ctx, id)
	if err != nil {
		return store.Theme{}, errors.New("theme not found")
	}
	return t, nil
}

// presetIDPrefix mirrors the console's PRESET_PREFIX: the two ends of one
// contract, and a preview that did not know it answered 404 on every built-in.
const presetIDPrefix = "preset:"

// previewTemplate is one thing the theme editor can render beside a palette.
// Kind tells the console whether to show the pair of panes or a single one: a
// mail has no dark half to show (see SampleMailWith).
type previewTemplate struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`     // "page" | "mail"
	Category string `json:"category"` // the picker's filter groups
}

// previewCategory is one filter toggle: a key, what it is called, and the
// glyph that stands for it.
type previewCategory struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

func mailTemplateKeys() []string {
	out := make([]string, 0, len(auth.MailSampleKinds))
	for _, k := range auth.MailSampleKinds {
		out = append(out, k.Key)
	}
	return out
}

// listTemplates is the catalogue the preview's picker walks. It is served
// rather than hard-coded in the console so that adding a template is a change
// in one place - the day a flow page joins it, the picker grows on its own.
func (a *API) listTemplates(w http.ResponseWriter, _ *http.Request, _ store.User) {
	// The flow-page SPECIMEN is not offered. It was a composite of every
	// element the design system has, and it earned its place while it was the
	// only thing the editor could show; with thirty real pages, it teaches
	// nothing they do not, and it is not a page anybody is ever served.
	out := make([]previewTemplate, 0,
		len(auth.PreviewPages)+len(auth.PortalPreviewKinds)+len(auth.MailSampleKinds))
	for _, p := range auth.PreviewPages {
		out = append(out, previewTemplate{Key: p.Key, Label: p.Label, Kind: "page", Category: p.Category})
	}
	// The bar, before the mails: it is the surface a visitor sees on every
	// screen of every application, and it has a dark half a message does not.
	for _, k := range auth.PortalPreviewKinds {
		out = append(out, previewTemplate{
			Key: k.Key, Label: k.Label, Kind: "page", Category: auth.CategoryPortal,
		})
	}
	for _, k := range auth.MailSampleKinds {
		// An operator's message is out: it wears the console's fixed colours
		// and the Meerkat mark whatever theme is being edited, so previewing it
		// beside a palette would show something that palette never touches.
		// The relay test still sends it - there, the question is whether it
		// arrives, not what it looks like.
		if k.Operator {
			continue
		}
		out = append(out, previewTemplate{
			Key: "mail:" + k.Key, Label: k.Label, Kind: "mail", Category: auth.CategoryMessages,
		})
	}
	writeJSON(w, http.StatusOK, struct {
		Templates  []previewTemplate `json:"templates"`
		Categories []previewCategory `json:"categories"`
	}{out, previewCategories()})
}

// previewCategories mirrors the catalogue's own list, so the console draws the
// toggles it is told about rather than a copy that can drift.
func previewCategories() []previewCategory {
	out := make([]previewCategory, 0, len(auth.PreviewCategories))
	for _, c := range auth.PreviewCategories {
		out = append(out, previewCategory{Key: c.Key, Label: c.Label, Icon: c.Icon})
	}
	return out
}
