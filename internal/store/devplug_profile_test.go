package store

import (
	"regexp"
	"strings"
	"testing"
)

// What plug accepts as a profile name: anything else and the agent emits no
// install script at all.
var plugProfileRule = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// The application's name, however it was written, becomes a name plug takes.
func TestThePlugProfileIsTheApplicationsName(t *testing.T) {
	cases := map[string]string{
		"Weather Hub":           "weather-hub",
		"MY APP":                "my-app",
		"Météo Brest":           "meteo-brest",
		"  -Ops_Console.v2  ":   "ops_console.v2",
		"Ça & Là":               "ca---la",
		"":                      DefaultPlugProfile,
		"***":                   DefaultPlugProfile,
		"日本":                    DefaultPlugProfile,
		strings.Repeat("a", 80): strings.Repeat("a", 63),
	}
	for in, want := range cases {
		got := PlugProfile(in)
		if got != want {
			t.Errorf("PlugProfile(%q) = %q, want %q", in, got, want)
		}
		if !plugProfileRule.MatchString(got) {
			t.Errorf("PlugProfile(%q) = %q, which plug refuses", in, got)
		}
	}
}
