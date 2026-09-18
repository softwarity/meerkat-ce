package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// jsonOf renders a tool's answer the way a reader sees it. Marshal escapes < and
// > into \u003c/\u003e - correct JSON, and decoded back by whatever parses it,
// but it turns every assertion here into a test of the escaping rather than of
// the answer.
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// A 1x1 PNG, the smallest thing that is honestly an image.
const pngPixel = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// The whole point of opening these to an agent: the SETTINGS are readable and
// the PICTURES are not carried. A background is the one thing here that can
// weigh a megabyte, and base64 of it is a third longer again - returned as
// bytes it would cost more of the conversation than every other answer put
// together, to convey an image the reader cannot see.
func TestTheBrandingReadsWithoutItsPictures(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()

	b := store.DefaultBranding()
	b.AppName = "Acme"
	b.Logo = pngPixel
	b.Background = store.Background{
		Image: pngPixel, Fit: "cover", Dim: 30,
		ImageDark: pngPixel, FitDark: "tile", DimDark: 50,
	}
	if err := store.SanitizeBranding(&b); err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if err := f.api.st.SetSetting(ctx, store.SettingBranding, b); err != nil {
		t.Fatalf("put branding: %v", err)
	}

	out, err := f.api.toolGetBranding(ctx, nil)
	if err != nil {
		t.Fatalf("get_branding: %v", err)
	}
	body := jsonOf(t, out)

	// Not one byte of the picture, in any field.
	if strings.Contains(body, "iVBORw0KGgo") {
		t.Errorf("the image itself came back:\n%s", body)
	}
	// But everything AROUND it, which is what an agent acts on.
	for _, want := range []string{
		`"appName":"Acme"`, `"logo":"<png,`, `"image":"<png,`, `"imageDark":"<png,`,
		`"fit":"cover"`, `"dim":30`, `"both":false`, `"fitDark":"tile"`, `"dimDark":50`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in:\n%s", want, body)
		}
	}
}

// The two-layer background is the setting a reader has to be able to tell
// apart: one picture for both schemes, or one each. With "both" on, the dark
// slot is empty and saying so IS the answer.
func TestOneBackgroundForBothSchemesReadsAsOne(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()

	b := store.DefaultBranding()
	b.Background = store.Background{Image: pngPixel, Fit: "contain", Dim: 10}
	if err := store.SanitizeBranding(&b); err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if !b.Background.Both {
		t.Fatalf("a lone image should mean both schemes, got %+v", b.Background)
	}
	if err := f.api.st.SetSetting(ctx, store.SettingBranding, b); err != nil {
		t.Fatalf("put branding: %v", err)
	}

	out, err := f.api.toolGetBranding(ctx, nil)
	if err != nil {
		t.Fatalf("get_branding: %v", err)
	}
	body := jsonOf(t, out)
	if !strings.Contains(body, `"both":true`) || !strings.Contains(body, `"imageDark":""`) {
		t.Errorf("a single picture should read as both-schemes with an empty dark slot:\n%s", body)
	}
}

// The portal is why settings had to open at all - and its icons are the same
// trap as a background: a module keeps the SVG itself, so a bar of ten would
// be ten kilobytes of path data in the middle of the answer.
func TestTheSettingsCarryThePortalButNotItsDrawings(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()

	if err := f.api.st.SetSetting(ctx, store.SettingPortal, store.PortalConfig{
		Enabled: true, Layout: store.PortalRail, Side: "right", Display: store.PortalDisplayIcon,
		Parents: []store.ModuleParent{{
			RouteID: "sales", Label: "Sales",
			Icon:     `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960"><path d="M840-519v339q0 24-18 42t-42 18H179Z"/></svg>`,
			Children: []store.ModuleChild{{RouteID: "orders", Label: "Orders", Icon: `<svg viewBox="0 -960 960 960"><path d="M240-80Z"/></svg>`}},
		}},
	}); err != nil {
		t.Fatalf("put portal: %v", err)
	}

	out, err := f.api.toolGetSettings(ctx, nil)
	if err != nil {
		t.Fatalf("get_settings: %v", err)
	}
	body := jsonOf(t, out)

	if strings.Contains(body, "<path") || strings.Contains(body, "viewBox") {
		t.Errorf("the icons came back as drawings:\n%s", body)
	}
	for _, want := range []string{
		`"enabled":true`, `"layout":"rail"`, `"side":"right"`, `"display":"icon"`,
		`"routeId":"sales"`, `"label":"Sales"`, `"routeId":"orders"`, `"icon":"<svg,`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in:\n%s", want, body)
		}
	}
}

// A palette is text - hex tokens - so it travels whole: "what is the primary
// colour here" is a question an agent can answer, unlike "what does the logo
// look like".
func TestTheThemesTravelWholeBecauseTheyAreText(t *testing.T) {
	f := setupBare(t)
	out, err := f.api.toolListThemes(context.Background(), nil)
	if err != nil {
		t.Fatalf("list_themes: %v", err)
	}
	body := jsonOf(t, out)
	for _, want := range []string{`"themes":[`, `"primary":"#`, `"active":`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in:\n%s", want, body)
		}
	}
}

// describeImage says what a thing is and how big, from the data URI alone. The
// size is the DECODED one: base64 is what we hold, not what the picture weighs.
func TestAnImageIsDescribedNotSent(t *testing.T) {
	for _, tc := range []struct {
		name, uri, want string
	}{
		{"nothing stays nothing", "", ""},
		{"a png pixel", pngPixel, "<png, 70 bytes>"},
		{"an svg data uri", "data:image/svg+xml;base64," + strings.Repeat("A", 4096), "<svg+xml, 3 KiB>"},
		{"not a data uri at all", "https://example.test/logo.png", "<image, 29 bytes>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := describeImage(tc.uri); got != tc.want {
				t.Errorf("describeImage(%.40s) = %q, want %q", tc.uri, got, tc.want)
			}
		})
	}
	if got := describeSVG(""); got != "" {
		t.Errorf("no icon should describe as nothing, got %q", got)
	}
	if got := describeSVG(strings.Repeat("x", 2048)); got != "<svg, 2 KiB>" {
		t.Errorf("describeSVG = %q", got)
	}
}
