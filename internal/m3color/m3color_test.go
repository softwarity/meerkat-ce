package m3color

import (
	"encoding/json"
	"os"
	"testing"
)

// golden.json is written by the console's generator (console/scripts/
// m3-golden.mjs, on @material/material-color-utilities 0.3.0, the release that
// reproduces Material Theme Builder's exports). The two generators must agree
// to the last role: the console draws the live preview, the gateway stores
// what it saves, and the moment they differ an operator sees one colour and
// visitors get another.
type goldenCase struct {
	Core       Core                         `json:"core"`
	ColorMatch bool                         `json:"colorMatch"`
	Schemes    map[string]map[string]string `json:"schemes"`
	Palettes   map[string]map[string]string `json:"palettes"`
}

var schemeNames = []struct {
	name     string
	dark     bool
	contrast float64
}{
	{"light", false, ContrastStandard}, {"light-medium-contrast", false, ContrastMedium},
	{"light-high-contrast", false, ContrastHigh}, {"dark", true, ContrastStandard},
	{"dark-medium-contrast", true, ContrastMedium}, {"dark-high-contrast", true, ContrastHigh},
}

func TestTheGoPortDrawsWhatTheConsoleDraws(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("golden.json holds no case")
	}
	for _, c := range cases {
		for _, s := range schemeNames {
			got, err := Scheme(c.Core, s.dark, s.contrast, c.ColorMatch)
			if err != nil {
				t.Fatalf("%+v: %v", c.Core, err)
			}
			for _, r := range Roles {
				if want := c.Schemes[s.name][r]; got[r] != want {
					t.Errorf("%+v colorMatch=%v %s %s: got %s, want %s", c.Core, c.ColorMatch, s.name, r, got[r], want)
				}
			}
		}
		p, err := Palettes(c.Core)
		if err != nil {
			t.Fatal(err)
		}
		for k, tones := range c.Palettes {
			for tone, want := range tones {
				if p[k][tone] != want {
					t.Errorf("%+v palette %s tone %s: got %s, want %s", c.Core, k, tone, p[k][tone], want)
				}
			}
		}
	}
}

// A scheme is the source colour's: an unset colour is derived, a set one
// changes its own group and nothing else.
func TestASetColourMovesOnlyItsGroup(t *testing.T) {
	base, err := Scheme(Core{Primary: "#6750A4"}, false, ContrastStandard, false)
	if err != nil {
		t.Fatal(err)
	}
	withSecondary, err := Scheme(Core{Primary: "#6750A4", Secondary: "#B33B15"}, false, ContrastStandard, false)
	if err != nil {
		t.Fatal(err)
	}
	if base["secondary"] == withSecondary["secondary"] {
		t.Error("setting the secondary left the secondary role where it was")
	}
	for _, r := range []string{"primary", "tertiary", "surface", "outline", "error"} {
		if base[r] != withSecondary[r] {
			t.Errorf("setting the secondary moved %s from %s to %s", r, base[r], withSecondary[r])
		}
	}
}

func TestAColourThatIsNotOneIsNamed(t *testing.T) {
	if _, err := Scheme(Core{Primary: "#6750A4", Neutral: "grey"}, false, 0, false); err == nil {
		t.Error("a neutral of \"grey\" was accepted")
	} else if want := `neutral: "grey" is not a colour: allowed are #rgb and #rrggbb`; err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}
