package filters

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// A custom element inside <head> closes it where it stands: everything written
// after it - charset, title, and <base href> - lands in the body, where a base
// is ignored. An Angular build served under a prefix then resolves from the
// root and its router answers its own 404. NEO's fpl-ui did exactly that.
func TestBodyInjectionLeavesTheHeadIntact(t *testing.T) {
	const page = `<!doctype html><html><head>` +
		`<meta charset="utf-8"><title>t</title><base href="/app/en/">` +
		`</head><body class="x"><app-root></app-root></body></html>`

	res := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/html"}},
		Body:   io.NopCloser(strings.NewReader(page)),
	}
	if err := InjectAtBodyStart(`<my-button></my-button>`)(res); err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(res.Body)
	got := string(out)

	head := strings.Index(got, "<base href=")
	button := strings.Index(got, "<my-button>")
	if head < 0 || button < 0 {
		t.Fatalf("missing pieces: %s", got)
	}
	if button < head {
		t.Errorf("the button was placed before <base>, truncating the head:\n%s", got)
	}
	if !strings.Contains(got, `<body class="x"><my-button>`) {
		t.Errorf("not at the top of the body:\n%s", got)
	}
}

func TestBodyInjectionSkipsFragments(t *testing.T) {
	res := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/html"}},
		Body:   io.NopCloser(strings.NewReader(`<div class="map"></div>`)),
	}
	if err := InjectAtBodyStart(`<x/>`)(res); err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(res.Body)
	if strings.Contains(string(out), "<x/>") {
		t.Errorf("a partial was touched: %s", out)
	}
}
