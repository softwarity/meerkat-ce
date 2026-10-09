package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/m3color"
	"github.com/softwarity/meerkat/internal/store"
)

// Every template the picker offers must actually render.
//
// The catalogue is served, so the console walks whatever this list says - and a
// fixture that stops matching its template gives a preview of a page that no
// longer exists, or a 500 in an iframe nobody reads the status of. This is the
// cheap guard: render the whole catalogue, fail on the first one that cannot.
//
// It also proves the catalogue is not lying about itself: a key that is listed
// and not resolvable is exactly the drift this catches.
func TestEveryOfferedTemplateRenders(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()
	theme, err := f.api.st.GetActiveTheme(ctx)
	if err != nil {
		t.Fatalf("active theme: %v", err)
	}

	res := f.get(t, "/api/themes/templates", f.rootC)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("catalogue: %d", res.StatusCode)
	}
	var payload struct {
		Templates  []previewTemplate `json:"templates"`
		Categories []previewCategory `json:"categories"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("catalogue: %v", err)
	}
	_ = res.Body.Close()
	catalogue := payload.Templates
	// Every template belongs to a category the toggles offer, or it is a
	// template the filter can hide for good and nobody can bring back.
	known := map[string]bool{}
	for _, c := range payload.Categories {
		known[c.Key] = true
	}
	for _, tpl := range catalogue {
		if !known[tpl.Category] {
			t.Errorf("template %q is in category %q, which no toggle offers", tpl.Key, tpl.Category)
		}
	}
	if len(catalogue) < 2 {
		t.Fatalf("a catalogue of %d is not a catalogue", len(catalogue))
	}

	for _, tpl := range catalogue {
		r := f.get(t, "/api/themes/"+theme.ID+"/preview?scheme=light&template="+tpl.Key, f.rootC)
		body := readAll(t, r)
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Errorf("template %q (%s): %d %s", tpl.Key, tpl.Kind, r.StatusCode, body)
			continue
		}
		// A template that renders to nothing is a template nobody would notice
		// had broken: the iframe would simply be blank.
		if len(body) < 200 || !strings.Contains(strings.ToLower(body), "<html") {
			t.Errorf("template %q rendered %d bytes and no document", tpl.Key, len(body))
		}
	}
}

// A built-in palette is offered by the picker under a namespaced id, and the
// preview has to resolve it: it has no row, and answering 404 on every
// built-in is what the namespace cost us the first time.
func TestAPreviewResolvesABuiltInPalette(t *testing.T) {
	f := setupBare(t)
	preset := store.PresetThemes()[1] // not the default, which IS stored

	r := f.get(t, "/api/themes/preset:"+preset.ID+"/preview?scheme=light", f.rootC)
	body := readAll(t, r)
	_ = r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("built-in %q: %d %s", preset.ID, r.StatusCode, body)
	}
	// And it is THAT palette, not the active one: the editor is looking at a
	// theme it has not saved, and showing the active one would answer a
	// question nobody asked.
	if want := preset.Dark["primary"]; !strings.Contains(body, want) {
		t.Errorf("the preview does not wear the built-in's primary %q", want)
	}
}

// An operator's message wears the console's fixed colours and the Meerkat
// mark whatever theme is being edited, so the theme editor has nothing to show
// for it. The relay test still sends it - there the question is whether it
// arrives, not what it looks like.
func TestTheOperatorDigestIsNotOffered(t *testing.T) {
	f := setupBare(t)
	res := f.get(t, "/api/themes/templates", f.rootC)
	var payload struct {
		Templates []previewTemplate `json:"templates"`
	}
	_ = json.NewDecoder(res.Body).Decode(&payload)
	_ = res.Body.Close()
	for _, tpl := range payload.Templates {
		if strings.HasSuffix(tpl.Key, ":digest") {
			t.Fatalf("the operator digest is in the theme picker: %+v", tpl)
		}
	}
}

// A frame that cannot take colours over postMessage - a mail, the portal bar -
// is reloaded with the editor's draft, and wears THOSE colours: generated
// here, by the generator a save would use, from colours nobody saved yet.
func TestAPreviewWearsItsDraft(t *testing.T) {
	f := setupBare(t)
	theme, err := f.api.st.GetActiveTheme(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	draft := `{"colors":{"primary":"#B33B15"},"contrast":"high","colorMatch":false,"flat":false}`
	want, err := m3color.Scheme(m3color.Core{Primary: "#B33B15"}, false, m3color.ContrastHigh, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tpl := range []string{"mail:confirm", ""} {
		r := f.get(t, "/api/themes/"+theme.ID+"/preview?scheme=light&template="+tpl+"&draft="+url.QueryEscape(draft), f.rootC)
		body := strings.ToLower(readAll(t, r))
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Fatalf("template %q: %d %s", tpl, r.StatusCode, body)
		}
		if !strings.Contains(body, strings.ToLower(want["primary"])) {
			t.Errorf("template %q does not wear the draft's primary %s", tpl, want["primary"])
		}
	}
	// A draft that is not one is refused with what it may hold.
	r := f.get(t, "/api/themes/"+theme.ID+"/preview?scheme=light&draft="+url.QueryEscape(`{"colours":{}}`), f.rootC)
	body := readAll(t, r)
	_ = r.Body.Close()
	if r.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "allowed: colors, contrast, colorMatch, flat") {
		t.Errorf("a malformed draft: %d %s", r.StatusCode, body)
	}
}
