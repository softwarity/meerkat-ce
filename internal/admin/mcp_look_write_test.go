package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// A context that carries the actor, the way the agent endpoint builds one
// before it calls a tool: the write tools audit what they change, and an audit
// entry with nobody in it is a change nobody made.
func mcpCtx(u store.User) context.Context {
	return context.WithValue(context.Background(), mcpActorKey{}, u)
}

// rootUser is whoever the fixture seeded as root - the actor these tools run as.
func rootUser(t *testing.T, f fixture) store.User {
	t.Helper()
	u, err := f.api.st.GetUserByID(context.Background(), "root")
	if err != nil {
		t.Fatalf("the seeded root: %v", err)
	}
	return u
}

// The trap a read-modify-write walks into here, and the reason resolveImage
// exists: get_branding DESCRIBES the picture rather than sending it, so an
// agent that reads, changes the app name and sends the whole thing back hands
// us "<png, 70 bytes>" in the logo field. Taken literally, the logo becomes a
// piece of prose - and the mark of the installation is gone.
func TestReadingTheBrandingAndSavingItBackKeepsThePictures(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))

	b := store.DefaultBranding()
	b.AppName = "Before"
	b.Logo = pngPixel
	b.Background = store.Background{Image: pngPixel, Fit: "cover", Dim: 20, ImageDark: pngPixel, FitDark: "tile", DimDark: 40}
	if err := store.SanitizeBranding(&b); err != nil {
		t.Fatal(err)
	}
	if err := f.api.st.SetSetting(ctx, store.SettingBranding, b); err != nil {
		t.Fatal(err)
	}

	// Read it the way an agent does...
	read, err := f.api.toolGetBranding(ctx, nil)
	if err != nil {
		t.Fatalf("get_branding: %v", err)
	}
	payload, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	// ...change ONE thing, and send the whole thing back.
	var round map[string]any
	if err := json.Unmarshal(payload, &round); err != nil {
		t.Fatal(err)
	}
	round["appName"] = "After"
	payload, _ = json.Marshal(round)
	if _, err := f.api.toolSaveBranding(ctx, payload); err != nil {
		t.Fatalf("save_branding: %v", err)
	}

	var got store.Branding
	if err := f.api.st.GetSetting(ctx, store.SettingBranding, &got); err != nil {
		t.Fatal(err)
	}
	if got.AppName != "After" {
		t.Errorf("the name did not change: %q", got.AppName)
	}
	if got.Logo != pngPixel {
		t.Errorf("the logo was replaced by the summary it was read as: %q", got.Logo)
	}
	if got.Background.Image != pngPixel || got.Background.ImageDark != pngPixel {
		t.Errorf("a background was lost on the way back: %+v", got.Background)
	}
	if got.Background.Dim != 20 || got.Background.DimDark != 40 || got.Background.FitDark != "tile" {
		t.Errorf("the framing around the pictures moved: %+v", got.Background)
	}
}

// "none" is how a picture is REMOVED - the one thing an empty string cannot
// mean here, since an empty string is what a summary of "no image" looks like.
func TestAnImageIsRemovedOnlyWhenAsked(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))

	b := store.DefaultBranding()
	b.Logo = pngPixel
	if err := f.api.st.SetSetting(ctx, store.SettingBranding, b); err != nil {
		t.Fatal(err)
	}
	if _, err := f.api.toolSaveBranding(ctx, json.RawMessage(`{"logo":"none"}`)); err != nil {
		t.Fatalf("save_branding: %v", err)
	}
	var got store.Branding
	if err := f.api.st.GetSetting(ctx, store.SettingBranding, &got); err != nil {
		t.Fatal(err)
	}
	if got.Logo != "" {
		t.Errorf(`"none" should remove the logo, got %.30q`, got.Logo)
	}
	// And a value that is neither a picture nor a word we know is refused
	// rather than stored: a logo that is a sentence renders as a broken image.
	if _, err := f.api.toolSaveBranding(ctx, json.RawMessage(`{"logo":"the acme logo"}`)); err == nil {
		t.Error("a logo that is prose should be refused")
	}
}

// The portal is the catalogue an agent is most likely to be asked to compose,
// and its guard is the routes: a module that opens nothing is refused, naming
// what is wrong, rather than saved as a dead entry.
func TestThePortalIsWrittenAgainstTheRoutesThatExist(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))

	if _, err := f.api.toolSavePortal(ctx, json.RawMessage(
		`{"enabled":true,"layout":"header","parents":[{"routeId":"ghost","label":"Ghost"}]}`)); err == nil {
		t.Error("a module bound to no route should be refused")
	} else if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("the refusal should name the route: %v", err)
	}

	// A real UI route, and an icon given by NAME - the gateway stores the
	// drawing, which is what keeps an agent out of the business of SVG.
	if err := f.api.st.SaveRoute(ctx, store.Route{
		ID: "sales", Name: "Sales", Enabled: true, IsUI: true,
		Upstream: "http://sales.test", Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/sales/**"}}}},
	}); err != nil {
		t.Fatalf("seeding a route: %v", err)
	}
	out, err := f.api.toolSavePortal(ctx, json.RawMessage(
		`{"enabled":true,"layout":"rail","side":"right","display":"icon","parents":[{"routeId":"sales","label":"Sales","icon":"storefront"}]}`))
	if err != nil {
		t.Fatalf("save_portal: %v", err)
	}
	if m, ok := out.(map[string]any); !ok || m["modules"] != 1 {
		t.Errorf("the answer should say what was saved, got %#v", out)
	}
	saved := f.api.st.Portal(ctx)
	if !saved.Enabled || saved.Layout != store.PortalRail || saved.Side != "right" {
		t.Errorf("the arrangement was not kept: %+v", saved)
	}
	if len(saved.Parents) != 1 || saved.Parents[0].RouteID != "sales" {
		t.Fatalf("the module was not kept: %+v", saved.Parents)
	}
	if !strings.HasPrefix(saved.Parents[0].Icon, "<svg") {
		t.Errorf("an icon named should be stored as its drawing, got %.40q", saved.Parents[0].Icon)
	}
}

// resolveImage is the whole decision in one place, so it is worth pinning:
// what is kept, what is changed, what is removed, what is refused.
func TestWhatAnImageFieldMeans(t *testing.T) {
	ctx := context.Background()
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		name    string
		in      *string
		current string
		want    string
		wantErr bool
	}{
		{name: "left out keeps it", in: nil, current: pngPixel, want: pngPixel},
		{name: "the summary keeps it", in: str("<png, 70 bytes>"), current: pngPixel, want: pngPixel},
		{name: "empty keeps it", in: str(""), current: pngPixel, want: pngPixel},
		{name: "none removes it", in: str("none"), current: pngPixel, want: ""},
		{name: "a data uri replaces it", in: str("data:image/webp;base64,AAAA"), current: pngPixel, want: "data:image/webp;base64,AAAA"},
		{name: "prose is refused", in: str("the acme logo"), current: pngPixel, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveImage(ctx, "logo", tc.in, tc.current)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected a refusal, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveImage: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %.40q, want %.40q", got, tc.want)
			}
		})
	}
}
