package fonts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every face of the catalogue is a file the binary carries, and every family
// draws the scripts it is there for - a family offered for a choice that did
// not draw Vietnamese would leave a Vietnamese page half in the system font.
func TestTheCatalogueIsWhole(t *testing.T) {
	if len(Families()) != 14 {
		t.Errorf("%d families offered, want the 14 chosen", len(Families()))
	}
	for _, f := range catalogue {
		need := []string{"latin", "latin-ext", "vietnamese", "cyrillic"}
		if f.Kind == KindCompanion {
			need = nil
		}
		have := map[string]bool{}
		for _, face := range f.Faces {
			have[face.Subset] = true
			if _, err := assets.ReadFile("files/" + face.File); err != nil {
				t.Errorf("%s: %s is not embedded", f.Family, face.File)
			}
		}
		for _, s := range need {
			if !have[s] {
				t.Errorf("%s does not draw %s", f.Family, s)
			}
		}
		if _, err := assets.ReadFile("licenses/" + f.Slug + ".txt"); err != nil {
			t.Errorf("%s travels without its licence", f.Family)
		}
	}
}

func TestAStackEndsOnTheSystem(t *testing.T) {
	s := Stack("Inter")
	for _, want := range []string{"'Inter'", "'Noto Sans Arabic'", "'Noto Sans Thai'", "system-ui"} {
		if !strings.Contains(s, want) {
			t.Errorf("stack %q lacks %s", s, want)
		}
	}
	if m := Stack("JetBrains Mono"); strings.Contains(m, "Noto") || !strings.HasSuffix(m, "monospace") {
		t.Errorf("a monospace stack: %q", m)
	}
	css := FaceCSS("Lora")
	if !strings.Contains(css, "font-family: 'Lora'") || !strings.Contains(css, "'Noto Sans Hebrew'") || strings.Contains(css, "'Inter'") {
		t.Errorf("faces for Lora:\n%s", css)
	}
	if FaceCSS() != "" {
		t.Error("no family chosen declares no face")
	}
}

func TestACheckNamesWhatIsAllowed(t *testing.T) {
	if err := Check("body", "Inter", KindSans, KindSerif); err != nil {
		t.Error(err)
	}
	err := Check("code", "Inter", KindMono)
	if err == nil || !strings.Contains(err.Error(), "allowed are JetBrains Mono, Roboto Mono") {
		t.Errorf("Inter as code: %v", err)
	}
}

func TestTheFilesAreServedForAYear(t *testing.T) {
	f := Families()[0].Faces[0].File
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Prefix+f, nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "font/woff2" ||
		!strings.Contains(rec.Header().Get("Cache-Control"), "immutable") || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("%s: %d %v", f, rec.Code, rec.Header())
	}
	for _, bad := range []string{"../catalogue.json", "catalogue.json", "licenses/../files/x.woff2", "x.otf"} {
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Prefix+bad, nil))
		if rec.Code != 404 {
			t.Errorf("%s answered %d", bad, rec.Code)
		}
	}
}
