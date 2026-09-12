package mail

import (
	"strings"
	"testing"
)

var testPalette = map[string]string{
	"surfaceContainer": "#111111", "surface": "#222222", "onSurface": "#eeeeee",
	"onSurfaceVariant": "#999999", "outline": "#444444", "primary": "#8839ef", "onPrimary": "#ffffff",
}

// The shapes one renderer draws: a button (link), a code (second factor), a
// titled list (the digest), and a plain notice. Each renders both a text and an
// HTML body, and the HTML carries the theme's colours inline so no client has
// to load anything.
func TestRenderShapes(t *testing.T) {
	brand := Brand{AppName: "Acme"}

	// A link message: the button label AND the raw URL both appear (a text
	// client cannot hide a URL behind a label).
	link := Spec{
		Subject: "Confirm", Heading: "Welcome", Intro: []string{"Confirm your address."},
		Button: &Button{Label: "Confirm my address", URL: "https://gw.example/confirm?token=abc"},
		Outro:  []string{"If you did not create this account, ignore this message."},
	}
	txt := renderText(brand, link)
	for _, want := range []string{"Welcome", "Confirm your address.", "https://gw.example/confirm?token=abc", "Acme"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text body missing %q:\n%s", want, txt)
		}
	}
	htmlOut := renderHTML(brand, testPalette, link)
	for _, want := range []string{"#8839ef", "https://gw.example/confirm?token=abc", "Confirm my address", "Welcome"} {
		if !strings.Contains(htmlOut, want) {
			t.Errorf("html body missing %q", want)
		}
	}

	// A code message: the digits show large, no button.
	code := Spec{Subject: "Code", Heading: "Your sign-in code", Code: "428913", CodeNote: "valid 10 minutes"}
	if txt := renderText(brand, code); !strings.Contains(txt, "428913") {
		t.Errorf("code missing from text: %s", txt)
	}
	h := renderHTML(brand, testPalette, code)
	if !strings.Contains(h, "428913") || strings.Contains(h, "<a href") {
		t.Errorf("code body wrong (want digits, no link): %s", h)
	}

	// A list message (the digest): the group title and each item appear, in
	// both bodies.
	digest := Spec{
		Subject: "Expiring", Heading: "Expiring accounts",
		Groups: []Group{{Title: "Losing access within 7 days", Items: []string{"alice (Ada) - last day 2026-09-14", "bob - last day 2026-09-15"}}},
		Outro:  []string{"Change a window under Users."},
	}
	dt := renderText(brand, digest)
	for _, want := range []string{"Losing access within 7 days", "alice (Ada) - last day 2026-09-14", "bob - last day 2026-09-15"} {
		if !strings.Contains(dt, want) {
			t.Errorf("digest text missing %q:\n%s", want, dt)
		}
	}
	dh := renderHTML(brand, testPalette, digest)
	if !strings.Contains(dh, "Losing access within 7 days") || !strings.Contains(dh, "Ada") {
		t.Errorf("digest html missing its list:\n%s", dh)
	}

	// An integrator's logo rides; Meerkat's own mark never does.
	if !strings.Contains(renderHTML(Brand{AppName: "Acme", LogoURL: "data:image/png;base64,AAAA"}, testPalette, link), "data:image/png;base64,AAAA") {
		t.Error("an integrator logo should appear in the header")
	}
	if strings.Contains(renderHTML(Brand{AppName: "Meerkat", Meerkat: true, LogoURL: "data:image/png;base64,AAAA"}, testPalette, link), "AAAA") {
		t.Error("Meerkat's own mark must not ride in mail")
	}
}

// A hand-written theme that dropped a token still renders: every colour has a
// default, so a message is never built against an empty string.
func TestRenderToleratesAThinPalette(t *testing.T) {
	out := renderHTML(Brand{AppName: "Acme"}, map[string]string{}, Spec{Heading: "Hi", Intro: []string{"x"}})
	if !strings.Contains(out, "#") {
		t.Errorf("no colour written from an empty palette:\n%s", out)
	}
	if strings.Contains(out, "background:;") || strings.Contains(out, "color:;") {
		t.Errorf("an empty colour slipped through:\n%s", out)
	}
}
