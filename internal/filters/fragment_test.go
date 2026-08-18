package filters

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

// text/html is not only pages. An application asks for HTML partials over XHR,
// and prepending a script to one gives it a second root element - AngularJS
// answers [$compile:tplrt] and the screen never renders. NEO's map screen died
// exactly there.
func TestInjectionLeavesFragmentsAlone(t *testing.T) {
	inject := InjectAfterHead(`<script src="/x.js"></script>`)

	for _, tc := range []struct {
		name, body string
		want       bool // injected?
	}{
		{"a partial", `<div class="map"><span>hi</span></div>`, false},
		{"a partial with leading space", "\n  <section>hi</section>", false},
		{"a document with a head", `<!doctype html><html><head><title>t</title></head><body>x</body></html>`, true},
		{"a document without a head", `<html><body>x</body></html>`, true},
		{"a doctype alone", "<!DOCTYPE html>\n<html>x</html>", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := &http.Response{
				Header: http.Header{"Content-Type": []string{"text/html"}},
				Body:   io.NopCloser(strings.NewReader(tc.body)),
			}
			if err := inject(res); err != nil {
				t.Fatal(err)
			}
			out, _ := io.ReadAll(res.Body)
			got := bytes.Contains(out, []byte("/x.js"))
			if got != tc.want {
				t.Errorf("injected=%v, want %v\n%s", got, tc.want, out)
			}
		})
	}
}
