package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Several configurations, one active (CFG-01/02).
//
// The whole feature rests on one sentence - a saved configuration is a COPY,
// never the running state - and every test below is that sentence seen from a
// different door: saving changes nothing, deleting changes nothing, and only
// activating touches what is served.
func TestSeveralConfigurations(t *testing.T) {
	f := setup(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello from "+r.URL.Path)
	}))
	t.Cleanup(upstream.Close)

	serves := func(path string) int {
		t.Helper()
		res, err := http.Get(f.appSrv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		return res.StatusCode
	}
	id := func(body string) string {
		t.Helper()
		var c store.Configuration
		if err := json.Unmarshal([]byte(body), &c); err != nil {
			t.Fatalf("not a configuration: %s", body)
		}
		return c.ID
	}

	// A gateway serving one route, saved under a name.
	route := strings.Replace(validRoute, "%s", upstream.URL, 1)
	if code, out := f.call(t, "PUT", "/api/routes/r1", route, f.rootC); code != http.StatusOK {
		t.Fatalf("put route: %d %s", code, out)
	}
	code, body := f.call(t, "POST", "/api/configurations", `{"name":"Acme","description":"one route"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("capture: %d %s", code, body)
	}
	acme := id(body)

	// Saving changed nothing about what is SERVED - and it named it: capturing
	// the running state under a name is what says "what this gateway serves is
	// called Acme", so that name holds the mark from now on. Without this the
	// current-configuration card reads "not saved under any name" straight
	// after someone saved it under a name.
	if s := serves("/api/x"); s != http.StatusOK {
		t.Fatalf("after capture the route stopped being served: %d", s)
	}
	if _, list := f.call(t, "GET", "/api/configurations", "", f.rootC); strings.Count(list, `"active":true`) != 1 {
		t.Fatalf("capturing the running state did not name it: %s", list)
	}

	// "Saved" is a comparison, not a flag. This is the assertion that stops the
	// screen from telling a comfortable lie: save, change one thing, and the
	// running state is no longer the one that was saved.
	t.Run("saved is a comparison, and drift shows", func(t *testing.T) {
		type ref struct{ Name string }
		type currentState struct {
			Digest  string `json:"digest"`
			SavedAs *ref   `json:"savedAs"`
			Active  *ref   `json:"active"`
		}
		// A FRESH value every time: json.Unmarshal leaves fields the payload
		// does not mention untouched, so a reused struct would keep a savedAs
		// the server has just stopped sending - which is the very thing being
		// tested, and it passed for the wrong reason once already.
		read := func() currentState {
			t.Helper()
			code, body := f.call(t, "GET", "/api/configurations/current", "", f.rootC)
			if code != http.StatusOK {
				t.Fatalf("current: %d %s", code, body)
			}
			var out currentState
			if err := json.Unmarshal([]byte(body), &out); err != nil {
				t.Fatal(err)
			}
			return out
		}

		saved := read()
		if saved.Digest == "" || saved.SavedAs == nil || saved.SavedAs.Name != "Acme" {
			t.Fatalf("right after capturing, the running state must read as saved: %+v", saved)
		}

		// One route more, and it is not that configuration any anymore - while
		// the mark has not moved, which is exactly the state to warn about.
		drift := strings.Replace(validRoute, "%s", upstream.URL, 1)
		drift = strings.Replace(drift, `"name": "api"`, `"name": "drifted"`, 1)
		drift = strings.Replace(drift, "/api/**", "/drift/**", 1)
		if code, out := f.call(t, "PUT", "/api/routes/drift", drift, f.rootC); code != http.StatusOK {
			t.Fatalf("put route: %d %s", code, out)
		}
		drifted := read()
		if drifted.SavedAs != nil {
			t.Fatalf("a changed gateway still claimed to be saved: %+v", drifted)
		}
		if drifted.Active == nil || drifted.Active.Name != "Acme" {
			t.Fatalf("the mark must stay put so the screen can say what it drifted from: %+v", drifted)
		}
		if drifted.Digest == saved.Digest {
			t.Fatal("the fingerprint did not move with the configuration")
		}

		// And back to the saved shape, it reads as saved again: the comparison
		// has no memory of having disagreed.
		if code, _ := f.call(t, "DELETE", "/api/routes/drift", "", f.rootC); code != http.StatusNoContent {
			t.Fatal("clean up the drift")
		}
		if back := read(); back.SavedAs == nil {
			t.Fatalf("undoing the change must read as saved again: %+v", back)
		}
	})

	t.Run("a name is asked for, and asked for only once", func(t *testing.T) {
		if code, _ := f.call(t, "POST", "/api/configurations", `{"name":"  "}`, f.rootC); code != http.StatusUnprocessableEntity {
			t.Fatalf("unnamed: %d, want 422", code)
		}
		if code, body := f.call(t, "POST", "/api/configurations", `{"name":"Acme"}`, f.rootC); code != http.StatusConflict {
			t.Fatalf("duplicate name: %d %s, want 409", code, body)
		}
	})

	// A second configuration: the same gateway, minus the route.
	if code, _ := f.call(t, "DELETE", "/api/routes/r1", "", f.rootC); code != http.StatusNoContent {
		t.Fatal("delete route")
	}
	code, body = f.call(t, "POST", "/api/configurations", `{"name":"Empty"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("capture second: %d %s", code, body)
	}
	empty := id(body)

	t.Run("the plan says what a switch would do, and does nothing", func(t *testing.T) {
		code, plan := f.call(t, "GET", "/api/configurations/"+acme+"/plan", "", f.rootC)
		if code != http.StatusOK {
			t.Fatalf("plan: %d %s", code, plan)
		}
		if !strings.Contains(plan, `"action":"add"`) {
			t.Fatalf("the plan does not mention the route it would bring back: %s", plan)
		}
		if s := serves("/api/x"); s != http.StatusNotFound {
			t.Fatalf("the plan applied itself: %d", s)
		}
	})

	t.Run("activating switches, and switching back is the rollback", func(t *testing.T) {
		if code, out := f.call(t, "POST", "/api/configurations/"+acme+"/activate", "", f.rootC); code != http.StatusOK {
			t.Fatalf("activate: %d %s", code, out)
		}
		if s := serves("/api/x"); s != http.StatusOK {
			t.Fatalf("the route did not come back: %d", s)
		}
		// The mark moved, and only one holds it - the second capture had taken
		// it, activating brings it back.
		_, list := f.call(t, "GET", "/api/configurations", "", f.rootC)
		if strings.Count(list, `"active":true`) != 1 {
			t.Fatalf("exactly one configuration must be active: %s", list)
		}
		// And back. Activating prunes what the document does not carry, which
		// is what makes this a switch rather than a merge.
		if code, out := f.call(t, "POST", "/api/configurations/"+empty+"/activate", "", f.rootC); code != http.StatusOK {
			t.Fatalf("activate back: %d %s", code, out)
		}
		if s := serves("/api/x"); s != http.StatusNotFound {
			t.Fatalf("switching away left the route behind: %d", s)
		}
	})

	t.Run("duplicating, renaming, exporting", func(t *testing.T) {
		code, body := f.call(t, "POST", "/api/configurations/"+acme+"/duplicate", `{"name":"Acme staging"}`, f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("duplicate: %d %s", code, body)
		}
		copyID := id(body)
		if code, out := f.call(t, "PUT", "/api/configurations/"+copyID,
			`{"name":"Acme preprod","description":"same, other upstream"}`, f.rootC); code != http.StatusOK {
			t.Fatalf("rename: %d %s", code, out)
		}
		// The export is the document itself, byte for byte - what would be
		// applied, not a re-rendering of it.
		code, file := f.call(t, "GET", "/api/configurations/"+copyID+"/export", "", f.rootC)
		if code != http.StatusOK || !strings.Contains(file, "version: 1") {
			t.Fatalf("export: %d %s", code, file)
		}
		_, source := f.call(t, "GET", "/api/configurations/"+acme, "", f.rootC)
		var got struct {
			Document string `json:"document"`
		}
		if err := json.Unmarshal([]byte(source), &got); err != nil {
			t.Fatal(err)
		}
		if got.Document != file {
			t.Fatal("a duplicate's export differs from what it was copied from")
		}
	})

	t.Run("a file can be taken in without being applied", func(t *testing.T) {
		file := "version: 1\nroles:\n  - id: r-taken\n    name: taken\n"
		code, body := f.call(t, "POST", "/api/configurations/import?name=From%20a%20file", file, f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("import: %d %s", code, body)
		}
		if _, roles := f.call(t, "GET", "/api/roles", "", f.rootC); strings.Contains(roles, "taken") {
			t.Fatal("storing a file applied it")
		}
	})

	// Importing under a name that is taken is an OVERWRITE, and a different
	// call from creating: the console asks first, and the audit trail says
	// which of the two happened.
	t.Run("a file can replace what a name already holds", func(t *testing.T) {
		code, body := f.call(t, "POST", "/api/configurations/import?name=Replaceable",
			"version: 1\nroles:\n  - id: r-first\n    name: first\n", f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("import: %d %s", code, body)
		}
		target := id(body)
		if code, body := f.call(t, "POST", "/api/configurations/import?name=Replaceable",
			"version: 1\nroles: []\n", f.rootC); code != http.StatusConflict {
			t.Fatalf("re-importing under the same name = %d %s, want 409", code, body)
		}
		if code, body := f.call(t, "PUT", "/api/configurations/"+target+"/document",
			"version: 1\nroles:\n  - id: r-second\n    name: second\n", f.rootC); code != http.StatusOK {
			t.Fatalf("replace: %d %s", code, body)
		}
		_, after := f.call(t, "GET", "/api/configurations/"+target, "", f.rootC)
		if !strings.Contains(after, "second") || strings.Contains(after, "first") {
			t.Fatalf("the document was not replaced: %s", after)
		}
		// And it applied nothing, like every other write on this screen.
		if _, roles := f.call(t, "GET", "/api/roles", "", f.rootC); strings.Contains(roles, "second") {
			t.Fatal("replacing a document applied it")
		}
	})

	// Media are not defined by editing (CFG-05): the text a person reads never
	// carries a picture, and saving that text back must not be read as "remove
	// the logo" - which is what would happen if the document were taken
	// literally, with nothing on screen ever having mentioned it.
	t.Run("images stay out of the text and survive an edit", func(t *testing.T) {
		logo := "data:image/png;base64,iVBORw0KGgo="
		branding := `{"appName":"Acme","logo":"` + logo + `","tagline":"t"}`
		if code, out := f.call(t, "PUT", "/api/branding", branding, f.rootC); code != http.StatusOK {
			t.Fatalf("set branding: %d %s", code, out)
		}
		// A plain export is TEXT: no picture in it, so it stays readable and
		// diffable. The package is what to take when the pictures have to
		// travel - one rule, two forms.
		if _, file := f.call(t, "GET", "/api/config/export", "", f.rootC); strings.Contains(file, logo) {
			t.Fatal("a plain export inlined a base64 image")
		}
		code, pkg := f.call(t, "GET", "/api/config/export?format=zip", "", f.rootC)
		if code != http.StatusOK || !strings.HasPrefix(pkg, "PK") {
			t.Fatalf("the package: %d, %q", code, pkg[:min(4, len(pkg))])
		}
		// The document to read does not.
		code, text := f.call(t, "GET", "/api/config/document", "", f.rootC)
		if code != http.StatusOK {
			t.Fatalf("document: %d %s", code, text)
		}
		if strings.Contains(text, logo) {
			t.Fatalf("the document handed to the editor carries a base64 image: %s", text)
		}
		// And applying that very text back keeps the logo where it was.
		if code, out := f.call(t, "POST", "/api/config/import?prune=true", text, f.rootC); code != http.StatusOK {
			t.Fatalf("apply the edited document: %d %s", code, out)
		}
		// Read from the branding itself: the plain export no longer carries a
		// picture, so asking IT would prove nothing either way.
		if _, b := f.call(t, "GET", "/api/branding", "", f.rootC); !strings.Contains(b, logo) {
			t.Fatal("saving an edited document erased the logo it never showed")
		}
	})

	t.Run("deleting removes the copy, never the running state", func(t *testing.T) {
		if code, out := f.call(t, "POST", "/api/configurations/"+acme+"/activate", "", f.rootC); code != http.StatusOK {
			t.Fatalf("activate: %d %s", code, out)
		}
		if code, _ := f.call(t, "DELETE", "/api/configurations/"+acme, "", f.rootC); code != http.StatusNoContent {
			t.Fatal("delete the active one")
		}
		if s := serves("/api/x"); s != http.StatusOK {
			t.Fatalf("deleting a configuration changed what is served: %d", s)
		}
		if code, _ := f.call(t, "DELETE", "/api/configurations/"+acme, "", f.rootC); code != http.StatusNotFound {
			t.Fatal("re-delete must be a 404")
		}
	})
}

// What the community image buys is the SIZE of the shelf, not the right to
// use it. Every action works on both images; three configurations fit at a
// time here, and the refusal is a count rather than a feature name.
func TestTheCommunityShelfHoldsThree(t *testing.T) {
	f := communityFixture(t)

	for i := 1; i <= FreeConfigurations; i++ {
		code, body := f.call(t, "POST", "/api/configurations",
			fmt.Sprintf(`{"name":"Customer %d"}`, i), f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("save %d of %d: %d %s", i, FreeConfigurations, code, body)
		}
	}
	code, body := f.call(t, "POST", "/api/configurations", `{"name":"One too many"}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("one past the cap: %d %s, want 422", code, body)
	}
	if !strings.Contains(body, "Enterprise") || !strings.Contains(body, "3") {
		t.Fatalf("the refusal must say how many fit and what lifts it: %s", body)
	}

	// Everything else is free, and that is the whole point of a cap rather
	// than a lock: three configurations one cannot switch between would be
	// three useless configurations.
	var first store.Configuration
	_, list := f.call(t, "GET", "/api/configurations", "", f.rootC)
	var all []store.Configuration
	if err := json.Unmarshal([]byte(list), &all); err != nil {
		t.Fatal(err)
	}
	first = all[0]
	if code, out := f.call(t, "POST", "/api/configurations/"+first.ID+"/activate", "", f.rootC); code != http.StatusOK {
		t.Fatalf("set as current on the community image: %d %s", code, out)
	}
	if code, out := f.call(t, "GET", "/api/configurations/"+first.ID+"/export", "", f.rootC); code != http.StatusOK {
		t.Fatalf("export: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/configurations/"+first.ID+"/capture", "", f.rootC); code != http.StatusOK {
		t.Fatalf("save the running state into it: %d %s", code, out)
	}

	// And deleting frees a place: the cap is on how many are held, not on how
	// many were ever made.
	if code, _ := f.call(t, "DELETE", "/api/configurations/"+first.ID, "", f.rootC); code != http.StatusNoContent {
		t.Fatal("delete")
	}
	if code, body := f.call(t, "POST", "/api/configurations", `{"name":"In its place"}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("after freeing a place: %d %s", code, body)
	}

	// The one-configuration loop is untouched, as it always was.
	if code, _ := f.call(t, "GET", "/api/config/export", "", f.rootC); code != http.StatusOK {
		t.Fatal("the community image must still export its configuration")
	}
	// So is going back in time: undoing is a safety function, and an image
	// that cannot undo is a more dangerous image, not a cheaper one.
	if code, body := f.call(t, "GET", "/api/config/history", "", f.rootC); code != http.StatusOK {
		t.Fatalf("history on the community image: %d %s", code, body)
	}
}
