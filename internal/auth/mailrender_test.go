package auth

import (
	"strings"
	"testing"
)

// The three shapes one builder draws: a button (link), a code (second factor),
// and a plain notice. Each renders both a text and an HTML body, and the HTML
// carries the theme's colours inline so no client has to load anything.
func TestMailRenderShapes(t *testing.T) {
	brand := brandView{AppName: "Acme"}
	pal := map[string]string{
		"surfaceContainer": "#111111", "surface": "#222222", "onSurface": "#eeeeee",
		"onSurfaceVariant": "#999999", "outline": "#444444", "primary": "#8839ef", "onPrimary": "#ffffff",
	}

	// A link message: the button label AND the raw URL both appear (a text
	// client cannot hide a URL behind a label).
	link := mailSpec{
		Subject: "Confirm", Heading: "Welcome", Intro: []string{"Confirm your address."},
		Button: &mailButton{Label: "Confirm my address", URL: "https://gw.example/confirm?token=abc"},
		Outro:  []string{"If you did not create this account, ignore this message."},
	}
	txt := mailText(brand, link)
	for _, want := range []string{"Welcome", "Confirm your address.", "https://gw.example/confirm?token=abc", "Acme"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text body missing %q:\n%s", want, txt)
		}
	}
	htmlOut := mailHTML(brand, pal, link)
	for _, want := range []string{"#8839ef", "https://gw.example/confirm?token=abc", "Confirm my address", "Welcome"} {
		if !strings.Contains(htmlOut, want) {
			t.Errorf("html body missing %q", want)
		}
	}

	// A code message: the digits show large, no button.
	code := mailSpec{Subject: "Code", Heading: "Your sign-in code", Code: "428913", CodeNote: "valid 10 minutes"}
	if txt := mailText(brand, code); !strings.Contains(txt, "428913") {
		t.Errorf("code missing from text: %s", txt)
	}
	h := mailHTML(brand, pal, code)
	if !strings.Contains(h, "428913") || strings.Contains(h, "<a href") {
		t.Errorf("code body wrong (want digits, no link): %s", h)
	}

	// An integrator's logo rides; Meerkat's own mark never does.
	withLogo := brandView{AppName: "Acme", LogoURL: "data:image/png;base64,AAAA"}
	if !strings.Contains(mailHTML(withLogo, pal, link), "data:image/png;base64,AAAA") {
		t.Error("an integrator logo should appear in the header")
	}
	mk := brandView{AppName: "Meerkat", Meerkat: true, LogoURL: "data:image/png;base64,AAAA"}
	if strings.Contains(mailHTML(mk, pal, link), "AAAA") {
		t.Error("Meerkat's own mark must not ride in mail")
	}
}

// A hand-written theme that dropped a token still renders: every colour has a
// default, so a message is never built against an empty string.
func TestMailRenderToleratesAThinPalette(t *testing.T) {
	out := mailHTML(brandView{AppName: "Acme"}, map[string]string{}, mailSpec{Heading: "Hi", Intro: []string{"x"}})
	if !strings.Contains(out, "#") { // some colour was written
		t.Errorf("no colour written from an empty palette:\n%s", out)
	}
	if strings.Contains(out, "background:;") || strings.Contains(out, "color:;") {
		t.Errorf("an empty colour slipped through:\n%s", out)
	}
}
