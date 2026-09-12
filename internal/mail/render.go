package mail

import (
	"bytes"
	"html"
	"html/template"
	"strings"
)

// One look for every e-mail the gateway sends (NOTIF-01).
//
// A mailed message is the same identity as the sign-in page - it carries the
// application's name and its colours - but it cannot be built the same way. An
// e-mail client loads no stylesheet, honours no <style> block reliably, and
// second-guesses a dark background, so the page CSS cannot be reused: the
// colours are read from the active theme's LIGHT palette and written INLINE, on
// a table, which is the one layout every client agrees on.
//
// This lives in the mail package, not in auth, because it is not auth's: the
// daily expiry digest (internal/expiry) wears the same clothes, and a renderer
// each would drift the day one is touched. The package takes PLAIN inputs - a
// brand, a palette, a spec - so it depends on neither the store nor auth, and
// the caller maps its own theme onto them.

// Brand is the identity a message wears: the name, an optional logo (a data
// URI), and whether this is Meerkat's own - its mark never rides in mail, an
// integrator's does.
type Brand struct {
	AppName string
	LogoURL string
	Meerkat bool
}

// Button is a call to action pointing at a gateway link.
type Button struct {
	Label string
	URL   string
}

// Group is a titled list - the digest's "losing access within N days" and the
// names under it. A message with no groups simply omits the section.
type Group struct {
	Title string
	Items []string
}

// Spec is one message, already localized: the strings are pulled from the
// caller's catalogue, because only it knows which keys its case uses. The
// shapes are additive - a message is a button, or a code, or a list, or a plain
// notice, or any combination - so a new case is a call, not a new design.
type Spec struct {
	Subject string
	// Preheader is the one line a client shows beside the subject in the list,
	// before the body is opened. Kept out of sight in the body itself.
	Preheader string
	Heading   string
	// Intro is what to say before the action, Outro after it - the reassurance
	// that a message nobody asked for can be ignored belongs after.
	Intro  []string
	Button *Button
	// Code and CodeNote render a second factor: the digits, large, and the line
	// under them (how long they last).
	Code     string
	CodeNote string
	Groups   []Group
	Outro    []string
}

// Compose turns a spec into a Message with both a plain-text and an HTML body:
// a client that refuses HTML, and a person who prefers text, both get a
// readable message rather than a wall of markup.
func Compose(to string, brand Brand, palette map[string]string, spec Spec) Message {
	return Message{
		To:      []string{to},
		Subject: spec.Subject,
		Text:    renderText(brand, spec),
		HTML:    renderHTML(brand, palette, spec),
	}
}

// renderText is the plain-text body: no colours, no markup, the same words in
// the same order. A link is shown in full - a text client cannot hide it behind
// a label - and a code stands on its own line.
func renderText(brand Brand, spec Spec) string {
	var b strings.Builder
	if spec.Heading != "" {
		b.WriteString(spec.Heading)
		b.WriteString("\n\n")
	}
	for _, p := range spec.Intro {
		b.WriteString(p)
		b.WriteString("\n\n")
	}
	if spec.Code != "" {
		b.WriteString("    " + spec.Code + "\n")
		if spec.CodeNote != "" {
			b.WriteString(spec.CodeNote + "\n")
		}
		b.WriteString("\n")
	}
	if spec.Button != nil {
		b.WriteString(spec.Button.Label + ":\n")
		b.WriteString(spec.Button.URL + "\n\n")
	}
	for _, g := range spec.Groups {
		if g.Title != "" {
			b.WriteString(g.Title + "\n")
		}
		for _, it := range g.Items {
			b.WriteString("  - " + it + "\n")
		}
		b.WriteString("\n")
	}
	for _, p := range spec.Outro {
		b.WriteString(p)
		b.WriteString("\n\n")
	}
	b.WriteString("-- \n")
	b.WriteString(brand.AppName)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// shell is the HTML skeleton: a table on a tinted page, a card in the middle,
// the application's name (its logo when it has one) at the top. Every colour is
// a placeholder filled from the theme; every style is inline.
var shell = template.Must(template.New("mail").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:{{.Page}};color:{{.Text}};font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
<span style="display:none!important;opacity:0;color:transparent;height:0;width:0;overflow:hidden;">{{.Preheader}}</span>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:{{.Page}};">
  <tr><td align="center" style="padding:32px 16px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:480px;background:{{.Card}};border:1px solid {{.Outline}};border-radius:14px;overflow:hidden;">
      <tr><td style="background:{{.Primary}};padding:20px 28px;">
        {{if .LogoURL}}<img src="{{.LogoURL}}" alt="{{.AppName}}" height="28" style="height:28px;display:block;border:0;">
        {{else}}<span style="color:{{.OnPrimary}};font-size:18px;font-weight:600;letter-spacing:.02em;">{{.AppName}}</span>{{end}}
      </td></tr>
      <tr><td style="padding:28px;">
        {{if .Heading}}<h1 style="margin:0 0 16px;font-size:20px;font-weight:600;color:{{.Text}};">{{.Heading}}</h1>{{end}}
        {{range .Intro}}<p style="margin:0 0 14px;font-size:15px;line-height:1.5;color:{{$.Text}};">{{.}}</p>{{end}}
        {{if .Code}}
        <div style="margin:22px 0;text-align:center;">
          <div style="display:inline-block;background:{{.Page}};border:1px solid {{.Outline}};border-radius:10px;padding:14px 22px;font-family:'SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace;font-size:30px;font-weight:700;letter-spacing:.28em;color:{{.Text}};">{{.Code}}</div>
          {{if .CodeNote}}<p style="margin:10px 0 0;font-size:13px;color:{{.Muted}};">{{.CodeNote}}</p>{{end}}
        </div>
        {{end}}
        {{if .Button}}
        <table role="presentation" cellpadding="0" cellspacing="0" style="margin:22px 0;"><tr><td style="border-radius:10px;background:{{.Primary}};">
          <a href="{{.Button.URL}}" style="display:inline-block;padding:12px 24px;font-size:15px;font-weight:600;color:{{.OnPrimary}};text-decoration:none;border-radius:10px;">{{.Button.Label}}</a>
        </td></tr></table>
        <p style="margin:0 0 14px;font-size:12px;color:{{.Muted}};word-break:break-all;">{{.Button.URL}}</p>
        {{end}}
        {{range .Groups}}
        <p style="margin:20px 0 8px;font-size:13px;font-weight:600;text-transform:uppercase;letter-spacing:.05em;color:{{$.Muted}};">{{.Title}}</p>
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
          {{range .Items}}<tr><td style="padding:7px 0;border-top:1px solid {{$.Outline}};font-size:14px;color:{{$.Text}};">{{.}}</td></tr>{{end}}
        </table>
        {{end}}
        {{range .Outro}}<p style="margin:14px 0 0;font-size:13px;line-height:1.5;color:{{$.Muted}};">{{.}}</p>{{end}}
      </td></tr>
    </table>
    <p style="margin:16px 0 0;font-size:12px;color:{{.Muted}};">{{.AppName}}</p>
  </td></tr>
</table>
</body></html>`))

// renderHTML fills the shell. Every string that came from a person - the app
// name, the heading, the paragraphs, the list items - is escaped by the
// template; the colours come from the palette, with a safe default for any
// token a hand-written theme might have dropped.
func renderHTML(brand Brand, palette map[string]string, spec Spec) string {
	color := func(key, def string) string {
		if v := strings.TrimSpace(palette[key]); v != "" {
			return v
		}
		return def
	}
	data := struct {
		Spec
		AppName                                              string
		LogoURL                                              template.URL
		Page, Card, Text, Muted, Outline, Primary, OnPrimary string
	}{
		Spec:      spec,
		AppName:   brand.AppName,
		Page:      color("surfaceContainer", "#e6e9ef"),
		Card:      color("surface", "#eff1f5"),
		Text:      color("onSurface", "#4c4f69"),
		Muted:     color("onSurfaceVariant", "#6c6f85"),
		Outline:   color("outline", "#acb0be"),
		Primary:   color("primary", "#8839ef"),
		OnPrimary: color("onPrimary", "#ffffff"),
	}
	// An integrator's logo is a sanitized data: URI; Meerkat's own mark is not
	// carried in mail (a mailed sentinel is Meerkat lore, not the app's).
	if !brand.Meerkat && brand.LogoURL != "" {
		data.LogoURL = template.URL(brand.LogoURL) //nolint:gosec // store-sanitized data URI
	}
	var b bytes.Buffer
	if err := shell.Execute(&b, data); err != nil {
		// The template is a constant and the data is strings, so this cannot
		// happen - but a mail that falls back to text beats a mail that panics.
		return "<pre>" + html.EscapeString(renderText(brand, spec)) + "</pre>"
	}
	return b.String()
}
