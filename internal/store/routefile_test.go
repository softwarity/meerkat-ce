package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestARouteFileStaysInsideItsRoute(t *testing.T) {
	for _, bad := range []string{"", "../x.css", "a/../../b", "a//b", `a\b`, "x?y", "./a"} {
		if _, err := CheckRouteFileName(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if n, err := CheckRouteFileName("/fonts/inter.woff2"); err != nil || n != "fonts/inter.woff2" {
		t.Errorf("a leading slash: %q %v", n, err)
	}
}

func TestRouteFilesRoundTripAndAreBounded(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.SaveRoute(ctx, Route{ID: "r", Name: "r", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	f, err := s.SetRouteFile(ctx, "r", "app.js", []byte("console.log(1)"))
	if err != nil || f.ContentType != "text/javascript; charset=utf-8" || f.SHA == "" {
		t.Fatalf("%+v %v", f, err)
	}
	all, err := s.RouteFileContents(ctx)
	if err != nil || len(all["r"]) != 1 || string(all["r"][0].Data) != "console.log(1)" {
		t.Fatalf("contents: %+v %v", all, err)
	}
	if _, err := s.SetRouteFile(ctx, "r", "big.bin", make([]byte, MaxRouteFileBytes+1)); err == nil || !strings.Contains(err.Error(), "the limit is") {
		t.Errorf("an oversized file: %v", err)
	}
	if gone, _ := s.DeleteRouteFile(ctx, "r", "app.js"); !gone {
		t.Error("the file was not deleted")
	}
	// The route goes, its files go with it.
	if _, err := s.SetRouteFile(ctx, "r", "x.css", []byte("a{}")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteRoute(ctx, "r"); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.RouteFileContents(ctx); len(all["r"]) != 0 {
		t.Errorf("files outlived their route: %+v", all["r"])
	}
}

// A taken name is refused in words, not by the database's constraint.
func TestARouteNameIsUniqueInWords(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.SaveRoute(ctx, Route{ID: "a", Name: "fonts", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	err := s.SaveRoute(ctx, Route{ID: "b", Name: "fonts", Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "another route already has this name") {
		t.Fatalf("a duplicate name: %v", err)
	}
	if err := s.SaveRoute(ctx, Route{ID: "a", Name: "fonts", Enabled: false}); err != nil {
		t.Errorf("saving the same route again: %v", err)
	}
}

// A file is renamed and retyped without being uploaded again; a taken name or
// a type that is not one is refused.
func TestARouteFileIsRenamedAndRetyped(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	if err := s.SaveRoute(ctx, Route{ID: "r", Name: "docs", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRouteFile(ctx, "r", "Agreement proposal - Google Docs.pdf", []byte("%PDF-1.7 x")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRouteFile(ctx, "r", "other.bin", []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	f, err := s.UpdateRouteFile(ctx, "r", "Agreement proposal - Google Docs.pdf", "agreement.pdf", "")
	if err != nil || f.Name != "agreement.pdf" || f.ContentType != "application/pdf" {
		t.Fatalf("rename: %+v %v", f, err)
	}
	if got, ok, _ := s.RouteFileContent(ctx, "r", "agreement.pdf"); !ok || string(got.Data) != "%PDF-1.7 x" {
		t.Errorf("the content did not follow the name: %v %q", ok, got.Data)
	}
	if f, err := s.UpdateRouteFile(ctx, "r", "other.bin", "", "application/x-thing"); err != nil || f.ContentType != "application/x-thing" {
		t.Errorf("retype: %+v %v", f, err)
	}
	if _, err := s.UpdateRouteFile(ctx, "r", "other.bin", "agreement.pdf", ""); err == nil || !strings.Contains(err.Error(), "already has a file") {
		t.Errorf("a taken name: %v", err)
	}
	if _, err := s.UpdateRouteFile(ctx, "r", "other.bin", "", "pdf"); err == nil {
		t.Error("a type without a subtype was accepted")
	}
	if _, err := s.UpdateRouteFile(ctx, "r", "missing", "x", ""); !errors.Is(err, ErrNoRouteFile) {
		t.Errorf("a missing file: %v", err)
	}
}

// What a page cannot carry is refused, and a block without a place goes to
// the end of the head.
func TestInjectionsAreChecked(t *testing.T) {
	list := []Injection{{Kind: "css", Code: "a{}"}}
	if err := CheckInjections(list); err != nil || list[0].Position != InjectHeadEnd {
		t.Fatalf("%v %+v", err, list)
	}
	for _, bad := range []Injection{
		{Kind: "html", Code: "x"},
		{Kind: "css", Code: "a{}</style><script>", Position: "head-end"},
		{Kind: "js", Code: "x", Position: "footer"},
		{Kind: "js", Code: "x", Load: "defer"},
		{Kind: "css", File: "a.css", Load: "module"},
		{Kind: "js", File: "../a.js"},
		{Kind: "js", File: "a.js", Code: "x"},
	} {
		if err := CheckInjections([]Injection{bad}); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
}
