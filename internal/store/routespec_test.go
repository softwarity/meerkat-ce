package store

import (
	"strings"
	"testing"
)

// The declaration is normalized where normalizing is obvious, and refused
// where a choice was made badly - with the allowed values named, which is what
// tells an admin what to type next.
func TestSanitizeRouteSpec(t *testing.T) {
	cases := []struct {
		name     string
		in       RouteSpec
		wantErr  string // substring, "" when it must pass
		wantPath string
	}{
		{
			name: "an upstream spec keeps its url",
			in:   RouteSpec{Type: SpecUpstream, Path: "/v3/api-docs"}, wantPath: "/v3/api-docs",
		},
		{
			name: "an unknown source names what is allowed",
			in:   RouteSpec{Type: "somewhere", Path: "x"}, wantErr: "allowed are upstream, file",
		},
		{
			name: "an upstream spec without a url says which one it wants",
			in:   RouteSpec{Type: SpecUpstream}, wantErr: "where the service publishes it",
		},
		{
			name: "a deposited spec needs the name it came from",
			in:   RouteSpec{Type: SpecFile, Path: "openapi.json"}, wantErr: "name of the file",
		},
		{
			// It is served inside the route's own prefix. A path that can climb
			// out of it is a path that can answer for another route.
			name:    "a served path is one file name",
			in:      RouteSpec{Type: SpecFile, Path: "../../etc/openapi.json", Filename: "o.yaml"},
			wantErr: "no directory and no",
		},
		{
			name:     "a leading slash is dropped rather than refused",
			in:       RouteSpec{Type: SpecFile, Path: "/openapi.json", Filename: "o.yaml"},
			wantPath: "openapi.json",
		},
		{
			// JSON is what leaves, whatever was deposited, so the name says so.
			name:     "the served name follows what is served",
			in:       RouteSpec{Type: SpecFile, Path: "contract.yaml", Filename: "contract.yaml"},
			wantPath: "contract.json",
		},
		{
			name: "a deposited spec with no path takes the obvious one",
			in:   RouteSpec{Type: SpecFile, Filename: "orders.yaml"}, wantPath: "openapi.json",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.in
			err := SanitizeRouteSpec(&spec)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("no error, want one mentioning %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spec.Path != tc.wantPath {
				t.Fatalf("path = %q, want %q", spec.Path, tc.wantPath)
			}
		})
	}
}

// An upstream declaration carries no file name: it would name a file nobody
// stored, and the first person to read it would look for it.
func TestAnUpstreamSpecDropsTheFileName(t *testing.T) {
	spec := RouteSpec{Type: SpecUpstream, Path: "/v3/api-docs", Filename: "left-over.yaml"}
	if err := SanitizeRouteSpec(&spec); err != nil {
		t.Fatal(err)
	}
	if spec.Filename != "" {
		t.Fatalf("filename = %q, want it dropped", spec.Filename)
	}
}
