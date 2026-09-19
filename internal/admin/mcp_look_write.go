package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
)

// Writing what the gateway LOOKS like: the navigation portal, and the branding
// around the pictures.
//
// Two tools rather than one save_settings, and the line between them is what a
// caller is actually asking for. The portal is a CATALOGUE - modules, their
// order, the route each one opens - and composing it is the kind of work a
// sentence describes well ("add the invoices app after sales"). The rest of the
// global settings are security policy - password rules, MFA, session lifetime -
// where the console shows what a change affects and an agent would not.
//
// Both go through the same validation the console does: SanitizePortalConfig
// against the routes as they are now, SanitizeBranding for the rest. A second
// implementation of those rules is how the two halves of a product start
// disagreeing about what is legal.
func (a *API) lookWriteTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "save_portal", Allow: administersIdentity, Title: "Write the navigation portal",
			Description: "Set the portal - the bar the proxied applications wear (PORTAL-01): whether it is on, " +
				"header or rail, which side, what an entry shows, and the modules it lists. " +
				"It takes the shape get_settings returns under \"portal\", so read it, change it, and send it " +
				"back whole: what you leave out is removed. " +
				"Every module binds to an ENABLED UI route by its id - list_routes gives them - and a module " +
				"may carry children, shown in the bar's second surface. " +
				"An icon is named, not drawn: pass a Material Symbols name like \"storefront\" and the gateway " +
				"stores the drawing. It takes effect at once, on every UI route.",
			Schema: portalSchema(),
			Call:   a.toolSavePortal,
		},
		{
			Name: "save_branding", Allow: administersIdentity, Title: "Write the branding",
			Description: "Set the identity the built-in pages wear: the application's name and tagline, the " +
				"logo size, and the page background - its fit, its dim, and whether light and dark share one " +
				"picture or have their own. It takes the shape get_branding returns, read-modify-write. " +
				"IMAGES: an image field may be left out or repeated as the summary get_branding gave " +
				"(\"<png, 45 KiB>\"), and the stored picture is kept. To CHANGE one, pass an https URL - the " +
				"gateway fetches it, so the bytes never cross this conversation - or a data URI for something " +
				"small. To remove one, pass \"none\".",
			Schema: brandingSchema(),
			Call:   a.toolSaveBranding,
		},
	}
}

func portalSchema() map[string]any {
	module := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"routeId":     str("The id of an enabled UI route, from list_routes. Required."),
			"label":       str("What the bar shows. Empty uses the route's own name."),
			"icon":        str("A Material Symbols name, e.g. storefront, receipt_long, apartment."),
			"description": str("The entry's tooltip."),
			"homeLabel":   str("Parents only: what the 'back to this module' row reads when it has children."),
			"disabled":    map[string]any{"type": "boolean", "description": "Kept in the catalogue but not served."},
			"children":    map[string]any{"type": "array", "description": "Sub-modules, same shape without children.", "items": map[string]any{"type": "object"}},
		},
		"required": []any{"routeId"},
	}
	return object(map[string]any{
		"enabled": map[string]any{"type": "boolean", "description": "Off leaves every route its own user button, as before."},
		"layout":  str("Which surface carries the top-level modules: \"header\" (tabs) or \"rail\"."),
		"side":    str("Which edge the rail takes: \"left\" or \"right\"."),
		"display": str("What an entry shows: \"both\", \"icon\" or \"label\". The rail always shows both."),
		"showAppName": map[string]any{"type": "boolean",
			"description": "Write the branding name beside the logo in the bar."},
		"parents": map[string]any{"type": "array", "description": "The top-level modules, in display order.", "items": module},
	})
}

func brandingSchema() map[string]any {
	image := "Leave out or repeat the summary to keep it; an https URL or a data URI to change it; \"none\" to remove it."
	return object(map[string]any{
		"appName":  str("The application's name, on every built-in page."),
		"tagline":  str("The line under it."),
		"logoSize": str("How big the mark is drawn: \"\" (normal), \"large\" or \"xlarge\"."),
		"logo":     str("The mark. " + image),
		"favicon":  str("The browser-tab icon; empty falls back to the logo. " + image),
		"hideMark": map[string]any{"type": "boolean",
			"description": "Remove the \"powered by Meerkat\" line. Enterprise only - refused without it."},
		"background": map[string]any{"type": "object", "description": "The picture behind the built-in pages.",
			"properties": map[string]any{
				"image":     str("The light scheme's picture, and both when \"both\" is on. " + image),
				"fit":       str("How it meets the screen: \"cover\", \"contain\" or \"tile\"."),
				"dim":       map[string]any{"type": "integer", "description": "0..100, how much surface colour is laid over it so the card stays readable."},
				"both":      map[string]any{"type": "boolean", "description": "Use the one picture in both schemes. Off gives the dark scheme its own."},
				"imageDark": str("The dark scheme's own picture, when \"both\" is off. " + image),
				"fitDark":   str("Its fit."),
				"dimDark":   map[string]any{"type": "integer", "description": "Its dim, 0..100."},
			}},
	})
}

func (a *API) toolSavePortal(ctx context.Context, args json.RawMessage) (any, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("nothing to save: pass the portal, in the shape get_settings returns under \"portal\"")
	}
	var portal store.PortalConfig
	if err := json.Unmarshal(args, &portal); err != nil {
		return nil, fmt.Errorf("this is not a portal: %w", err)
	}
	before := a.st.Portal(ctx)
	routes, err := a.st.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	// The same guard the console gets: every module binds to an enabled UI
	// route, and an icon NAME becomes the stored drawing (icons.Resolve).
	if err := store.SanitizePortalConfig(&portal, routes); err != nil {
		return nil, err
	}
	if err := a.st.SetSetting(ctx, store.SettingPortal, portal); err != nil {
		return nil, err
	}
	// The bar is injected by the data plane, which decides at reload whether a
	// route wears it: saved and not reloaded is a portal nobody sees.
	if err := a.reloadRouting(ctx); err != nil {
		return nil, fmt.Errorf("saved, but the reload failed: %w", err)
	}
	a.auditUpdate(ctx, mcpActor(ctx), "settings.update", "settings", "", "portal", "", before, portal)
	return map[string]any{
		"saved":   true,
		"enabled": portal.Enabled,
		"modules": len(portal.Parents),
	}, nil
}

func (a *API) toolSaveBranding(ctx context.Context, args json.RawMessage) (any, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("nothing to save: pass the branding, in the shape get_branding returns")
	}
	old := store.DefaultBranding()
	_ = a.st.GetSetting(ctx, store.SettingBranding, &old)

	// Start from what is stored, so a field left out keeps its value - the
	// read-modify-write every other write tool here follows.
	b := old
	var in struct {
		AppName    *string `json:"appName"`
		Tagline    *string `json:"tagline"`
		LogoSize   *string `json:"logoSize"`
		Logo       *string `json:"logo"`
		Favicon    *string `json:"favicon"`
		HideMark   *bool   `json:"hideMark"`
		Background *struct {
			Image     *string `json:"image"`
			Fit       *string `json:"fit"`
			Dim       *int    `json:"dim"`
			Both      *bool   `json:"both"`
			ImageDark *string `json:"imageDark"`
			FitDark   *string `json:"fitDark"`
			DimDark   *int    `json:"dimDark"`
		} `json:"background"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("this is not a branding: %w", err)
	}
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&b.AppName, in.AppName)
	set(&b.Tagline, in.Tagline)
	set(&b.LogoSize, in.LogoSize)
	if in.HideMark != nil {
		b.HideMark = *in.HideMark
	}
	var err error
	if b.Logo, err = resolveImage(ctx, "logo", in.Logo, old.Logo); err != nil {
		return nil, err
	}
	if b.Favicon, err = resolveImage(ctx, "favicon", in.Favicon, old.Favicon); err != nil {
		return nil, err
	}
	if bg := in.Background; bg != nil {
		set(&b.Background.Fit, bg.Fit)
		set(&b.Background.FitDark, bg.FitDark)
		if bg.Dim != nil {
			b.Background.Dim = *bg.Dim
		}
		if bg.DimDark != nil {
			b.Background.DimDark = *bg.DimDark
		}
		if bg.Both != nil {
			b.Background.Both = *bg.Both
		}
		if b.Background.Image, err = resolveImage(ctx, "background", bg.Image, old.Background.Image); err != nil {
			return nil, err
		}
		if b.Background.ImageDark, err = resolveImage(ctx, "dark background", bg.ImageDark, old.Background.ImageDark); err != nil {
			return nil, err
		}
	}
	if err := store.SanitizeBranding(&b); err != nil {
		return nil, err
	}
	if err := edition.Require("removing the Meerkat mark"); b.HideMark && err != nil {
		return nil, err
	}
	if err := a.st.SetSetting(ctx, store.SettingBranding, b); err != nil {
		return nil, err
	}
	a.auditUpdate(ctx, mcpActor(ctx), "theme.branding", "theme", "", "branding", "", old, b)
	out, err := a.toolGetBranding(ctx, nil)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// resolveImage decides what an image field means, given what is stored.
//
// The hard case is the one a read-modify-write creates: get_branding described
// the picture rather than sending it, so the value coming back is "<png, 45
// KiB>" - and taking that literally would replace the logo with a piece of
// prose. A summary means "unchanged", which is what the caller meant by not
// touching it.
func resolveImage(ctx context.Context, what string, in *string, current string) (string, error) {
	if in == nil {
		return current, nil
	}
	v := strings.TrimSpace(*in)
	switch {
	case v == "", strings.HasPrefix(v, "<"):
		// Absent, or the summary handed back untouched.
		return current, nil
	case strings.EqualFold(v, "none"):
		return "", nil
	case strings.HasPrefix(v, "data:"):
		return v, nil
	case strings.HasPrefix(v, "https://"), strings.HasPrefix(v, "http://"):
		return fetchImage(ctx, what, v)
	}
	return "", fmt.Errorf("%s %q: pass an https URL, a data URI, \"none\" to remove it, or leave it out to keep it", what, v)
}

// maxFetchedImage bounds what the gateway will pull in. The branding's own
// limits are stricter still (SanitizeBranding refuses a background past ~1.4
// MB); this one stops a download before it is held in memory at all.
const maxFetchedImage = 4 << 20

// fetchImage downloads a picture and turns it into the data URI the branding
// stores. The gateway fetches it, not the agent: an image that travelled
// through the conversation would cost tens of thousands of tokens to carry
// bytes nobody reads.
func fetchImage(ctx context.Context, what, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("%s %q: %w", what, url, err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: %s could not be fetched: %w", what, url, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s answered %s", what, url, res.Status)
	}
	mediaType, _, _ := strings.Cut(res.Header.Get("Content-Type"), ";")
	mediaType = strings.TrimSpace(mediaType)
	if !strings.HasPrefix(mediaType, "image/") {
		return "", fmt.Errorf("%s: %s is %q, not an image", what, url, mediaType)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxFetchedImage+1))
	if err != nil {
		return "", fmt.Errorf("%s: reading %s: %w", what, url, err)
	}
	if len(body) > maxFetchedImage {
		return "", fmt.Errorf("%s: %s is over %s", what, url, byteSize(maxFetchedImage))
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}
