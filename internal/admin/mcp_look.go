package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/softwarity/meerkat/internal/mcp"
	"github.com/softwarity/meerkat/internal/store"
)

// What the gateway LOOKS like, for an agent: the global settings (the portal
// among them), the branding, and the colour palettes.
//
// These were out of reach on the grounds that "an agent has no eyes, and a
// base64 image would flood the conversation". The second half was right and the
// first was not: a portal's modules, a page layout, a dim percentage or a
// stored-theme key are text, and an agent that cannot read them cannot answer
// the simplest question about this installation. What had to stay out is the
// PICTURE, not the settings around it - so every image travels as a line
// saying what it is (see describeImage), and never as its bytes.
func (a *API) lookTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "get_settings", Allow: a.administersSomething, Title: "Read the global settings", ReadOnly: true,
			Description: "The settings that apply to the whole installation: the navigation portal and its " +
				"modules, the built-in pages' arrangement and imposed scheme, the session and password " +
				"policies, the locale pool, self-registration, rate limits. " +
				"This is where the portal lives - what it shows, in what order, and which UI route each " +
				"module binds to.",
			Schema: noArgs(),
			Call:   a.toolGetSettings,
		},
		{
			Name: "get_branding", Allow: a.administersSomething, Title: "Read the branding", ReadOnly: true,
			Description: "The identity the built-in pages wear: the application's name and tagline, the logo " +
				"size, and the page background - its fit, its dim, and whether light and dark share one " +
				"picture or have their own. " +
				"The images themselves are described rather than sent (\"png, 45 KiB\"): their bytes would " +
				"fill this conversation and say nothing an agent can act on.",
			Schema: noArgs(),
			Call:   a.toolGetBranding,
		},
		{
			Name: "list_themes", Allow: a.administersSomething, Title: "List the colour themes", ReadOnly: true,
			Description: "The colour palettes this installation holds, and which one is active. Each theme " +
				"carries its light and dark tokens as hex values - readable, comparable, and what a " +
				"question like 'what is the primary colour here' is asking for.",
			Schema: noArgs(),
			Call:   a.toolListThemes,
		},
	}
}

func (a *API) toolGetSettings(ctx context.Context, _ json.RawMessage) (any, error) {
	p, err := a.loadSettingsPayload(ctx)
	if err != nil {
		return nil, fmt.Errorf("the settings could not be read: %w", err)
	}
	// A portal module keeps its icon as the SVG itself - a kilobyte of path
	// data each, and a bar of ten modules would be ten kilobytes of drawing
	// instructions in the middle of the settings. Said in a line instead; what
	// an agent needs to know is that a module HAS an icon, and save_settings
	// takes the Material Symbols name rather than the drawing anyway.
	for i := range p.Portal.Parents {
		parent := &p.Portal.Parents[i]
		parent.Icon = describeSVG(parent.Icon)
		for j := range parent.Children {
			parent.Children[j].Icon = describeSVG(parent.Children[j].Icon)
		}
	}
	return p, nil
}

// describeSVG is describeImage for a stored drawing: the icons a portal module
// carries are SVG source, not a data URI.
func describeSVG(svg string) string {
	if svg == "" {
		return ""
	}
	return fmt.Sprintf("<svg, %s>", byteSize(len(svg)))
}

// brandingView is the branding with its pictures named instead of carried.
type brandingView struct {
	AppName    string `json:"appName"`
	Tagline    string `json:"tagline"`
	LogoSize   string `json:"logoSize,omitempty"`
	Logo       string `json:"logo"`
	Favicon    string `json:"favicon"`
	HideMark   bool   `json:"hideMark"`
	Background struct {
		Image     string `json:"image"`
		Fit       string `json:"fit,omitempty"`
		Dim       int    `json:"dim,omitempty"`
		Both      bool   `json:"both"`
		ImageDark string `json:"imageDark"`
		FitDark   string `json:"fitDark,omitempty"`
		DimDark   int    `json:"dimDark,omitempty"`
	} `json:"background"`
}

func (a *API) toolGetBranding(ctx context.Context, _ json.RawMessage) (any, error) {
	b := store.DefaultBranding()
	if err := a.st.GetSetting(ctx, store.SettingBranding, &b); err != nil {
		return nil, fmt.Errorf("the branding could not be read: %w", err)
	}
	var v brandingView
	v.AppName, v.Tagline, v.LogoSize, v.HideMark = b.AppName, b.Tagline, b.LogoSize, b.HideMark
	v.Logo, v.Favicon = describeImage(b.Logo), describeImage(b.Favicon)
	v.Background.Image = describeImage(b.Background.Image)
	v.Background.Fit, v.Background.Dim = b.Background.Fit, b.Background.Dim
	v.Background.Both = b.Background.Both
	v.Background.ImageDark = describeImage(b.Background.ImageDark)
	v.Background.FitDark, v.Background.DimDark = b.Background.FitDark, b.Background.DimDark
	return v, nil
}

func (a *API) toolListThemes(ctx context.Context, _ json.RawMessage) (any, error) {
	themes, err := a.st.ListThemes(ctx)
	if err != nil {
		return nil, fmt.Errorf("the themes could not be read: %w", err)
	}
	if themes == nil {
		themes = []store.Theme{}
	}
	return map[string]any{"themes": themes}, nil
}

// describeImage turns a stored data URI into a line an agent can reason about:
// what it is and how big. "" stays "", which is the answer to "is there one".
//
// The bytes are never returned. A background can weigh a megabyte, and base64
// of it is a third longer again - it would cost more of the conversation than
// everything else this tool says, to convey a picture the reader cannot see.
func describeImage(uri string) string {
	if uri == "" {
		return ""
	}
	kind := "image"
	if rest, ok := strings.CutPrefix(uri, "data:"); ok {
		if mediaType, _, found := strings.Cut(rest, ";"); found {
			kind = strings.TrimPrefix(mediaType, "image/")
		}
	}
	// The payload is base64: four characters carry three bytes, and the
	// padding at the end stands for bytes that are not there.
	size := len(uri)
	if _, payload, ok := strings.Cut(uri, ";base64,"); ok {
		size = len(payload) / 4 * 3
		size -= strings.Count(payload[max(0, len(payload)-2):], "=")
	}
	return fmt.Sprintf("<%s, %s>", kind, byteSize(size))
}

func byteSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KiB", n/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}
