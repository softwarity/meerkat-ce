package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A route's custom code lands where each block says, in the route's order
// within each place, and a file is linked from the gateway's own path - which
// serves only the files a block names.
func TestCustomCodeLandsInOrderAtItsPlaces(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<!doctype html><html><head><link rel="stylesheet" href="app.css"></head><body><p>ok</p></body></html>`)
	}))
	t.Cleanup(up.Close)
	r := pathRoute("ui", "ui", 1, "/app/**", up.URL)
	r.IsUI = true
	r.UI = &store.RouteUI{Injections: []store.Injection{
		{Kind: "js", Code: "early()", Position: "head-start"},
		{Kind: "css", File: "theme/override.css", Position: "head-end"},
		{Kind: "css", Code: "p { color: red }", Position: "head-end"},
		{Kind: "js", File: "lib.js", Position: "body-end", Load: "defer"},
		{Kind: "js", Code: "import './x.js'", Position: "body-end", Load: "module"},
		{Kind: "js", File: "gone.js", Position: "body-end"},
	}}
	rt := newRouter(t, r)
	ctx := context.Background()
	css, err := rt.st.SetRouteFile(ctx, "ui", "theme/override.css", []byte("h1 { color: blue }"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.st.SetRouteFile(ctx, "ui", "lib.js", []byte("window.lib = 1")); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.st.SetRouteFile(ctx, "ui", "private.txt", []byte("not named by any block")); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	_, body := get(t, rt, "/app/")
	cssURL := "/meerkat/route-assets/ui/theme/override.css?v=" + css.SHA[:12]
	order := []string{
		"<head><script>\nearly()\n</script>",
		`<link rel="stylesheet" href="app.css">`,
		`<link rel="stylesheet" href="` + cssURL + `">`,
		"<style>\np { color: red }\n</style>\n</head>",
		"<p>ok</p>",
		`src="/meerkat/route-assets/ui/lib.js?v=`,
		`" defer></script>`,
		"<script type=\"module\">\nimport './x.js'\n</script>",
		"</body>",
	}
	at := 0
	for _, want := range order {
		i := strings.Index(body[at:], want)
		if i < 0 {
			t.Fatalf("%q missing or out of order in\n%s", want, body)
		}
		at += i + len(want)
	}
	if strings.Contains(body, "gone.js") {
		t.Errorf("a block naming a file the route does not have was written:\n%s", body)
	}

	res, served := get(t, rt, cssURL)
	if res.StatusCode != http.StatusOK || served != "h1 { color: blue }" ||
		!strings.Contains(res.Header.Get("Cache-Control"), "immutable") ||
		!strings.HasPrefix(res.Header.Get("Content-Type"), "text/css") {
		t.Errorf("the linked stylesheet: %d %q %v", res.StatusCode, served, res.Header)
	}
	if res, _ := get(t, rt, "/meerkat/route-assets/ui/private.txt"); res.StatusCode != http.StatusNotFound {
		t.Errorf("a file no block names was served: %d", res.StatusCode)
	}
}

// A route saved before the list kept two free blocks; they read as the list's
// first two entries, at the start of the head where they used to land.
func TestTheTwoFreeBlocksReadAsTheList(t *testing.T) {
	var ui store.RouteUI
	if err := ui.UnmarshalJSON([]byte(`{"customCss":"a{}","customJs":"go()","injections":[{"kind":"js","code":"later()","position":"body-end"}]}`)); err != nil {
		t.Fatal(err)
	}
	if len(ui.Injections) != 3 || ui.Injections[0].Code != "a{}" || ui.Injections[0].Position != "head-start" ||
		ui.Injections[1].Kind != "js" || ui.Injections[2].Code != "later()" {
		t.Errorf("%+v", ui.Injections)
	}
}
