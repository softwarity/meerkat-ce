package store

import "testing"

// What an administrator types becomes the link the pages draw: an e-mail
// address a mailto:, a web address an https one when it says nothing else.
// Any other scheme is refused - the link is drawn on pages served to anyone.
func TestTheContactIsAnAddressOrNothing(t *testing.T) {
	for in, want := range map[string]string{
		"":                             "",
		"  it@acme.example ":           "mailto:it@acme.example",
		"mailto:it@acme.example":       "mailto:it@acme.example",
		"help.acme.example/ask":        "https://help.acme.example/ask",
		"https://help.acme.example":    "https://help.acme.example",
		"http://intranet.local/helpme": "http://intranet.local/helpme",
	} {
		got, err := CleanContact(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v - want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"javascript:alert(1)", "data:text/html,hi", "mailto:", "mailto:not an address", "ftp://files.acme.example", "https://"} {
		if got, err := CleanContact(in); err == nil {
			t.Errorf("%q was accepted as %q", in, got)
		}
	}
}
