package icons

import "testing"

func TestBankLoaded(t *testing.T) {
	if Count() < 1000 {
		t.Fatalf("the embedded bank looks empty: %d icons", Count())
	}
	if svg, ok := Lookup("dashboard"); !ok || len(svg) < 20 {
		t.Fatalf("dashboard should resolve to an svg, got ok=%v len=%d", ok, len(svg))
	}
}

func TestSearchMatchesEveryToken(t *testing.T) {
	res := Search("shopping cart", 50)
	if len(res) == 0 {
		t.Fatal("expected matches for 'shopping cart'")
	}
	for _, i := range res {
		if !contains(i.Name, "shopping") || !contains(i.Name, "cart") {
			t.Errorf("%q matched but lacks a token", i.Name)
		}
	}
	// The cap holds.
	if len(Search("", 10)) != 10 {
		t.Errorf("empty query should return exactly the cap")
	}
}

func TestResolveNameAndSanitizePaste(t *testing.T) {
	// A bare name resolves to its catalogue svg.
	if got := Resolve("point_of_sale"); len(got) < 20 || got[:4] != "<svg" {
		t.Fatalf("a known name should resolve to svg, got %.20q", got)
	}
	// An unknown name resolves to nothing.
	if got := Resolve("definitely_not_an_icon_xyz"); got != "" {
		t.Errorf("an unknown name should resolve to empty, got %q", got)
	}
	// A pasted svg is stripped to viewBox + path (no handler survives).
	paste := `<svg viewBox="0 0 24 24" onload="x()"><path d="M1 1h2z" onclick="hack()"/><script>bad()</script></svg>`
	got := Resolve(paste)
	if contains(got, "onclick") || contains(got, "onload") || contains(got, "script") {
		t.Errorf("sanitized svg must carry no active content: %q", got)
	}
	if !contains(got, `viewBox="0 0 24 24"`) || !contains(got, `d="M1 1h2z"`) {
		t.Errorf("sanitized svg lost its geometry: %q", got)
	}
	// Idempotent: re-resolving its own output is unchanged.
	if Resolve(got) != got {
		t.Errorf("Resolve should be idempotent")
	}
	// Junk resolves to nothing.
	if Resolve("<svg>no viewbox no path</svg>") != "" {
		t.Errorf("markup with no viewBox/path should resolve to empty")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
