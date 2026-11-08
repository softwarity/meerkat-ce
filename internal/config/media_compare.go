package config

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"

	"github.com/softwarity/meerkat/internal/store"
)

// What travels BESIDE a route rather than in it - the files uploaded on it
// (the custom code a block links to) and its deposited OpenAPI spec - is not
// in the document's JSON, so the generic comparison never saw it: two
// configurations whose routes differed only by the content of a stylesheet
// compared the same. These are read here, by content.

// FileSums is a route's files by name, each with its content's hash.
func FileSums(files []store.RouteFile) map[string]string {
	if len(files) == 0 {
		return nil
	}
	out := make(map[string]string, len(files))
	for _, f := range files {
		out[f.Name] = fileSum(f)
	}
	return out
}

func fileSum(f store.RouteFile) string {
	if len(f.Data) > 0 || f.SHA == "" {
		sum := sha256.Sum256(f.Data)
		return hex.EncodeToString(sum[:])
	}
	return f.SHA
}

// SpecSum is the hash of a deposited spec, empty when there is none.
func SpecSum(spec []byte) string {
	if len(spec) == 0 {
		return ""
	}
	sum := sha256.Sum256(spec)
	return hex.EncodeToString(sum[:])
}

// mediaChanges names, per route both documents hold, what moved beside it:
// "files" when a file was added, removed or rewritten, "spec" for the spec.
func mediaChanges(from, to *Document) map[string][]string {
	out := map[string][]string{}
	if from == nil || to == nil {
		return out
	}
	inTo := map[string]bool{}
	for _, r := range to.Routes {
		inTo[r.ID] = true
	}
	for _, r := range from.Routes {
		if !inTo[r.ID] {
			continue
		}
		var moved []string
		if !sameSums(FileSums(from.Files[r.ID]), FileSums(to.Files[r.ID])) {
			moved = append(moved, "files")
		}
		if SpecSum(from.Specs[r.ID]) != SpecSum(to.Specs[r.ID]) {
			moved = append(moved, "spec")
		}
		if len(moved) > 0 {
			out[r.ID] = moved
		}
	}
	return out
}

func sameSums(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// withMedia folds what moved beside the routes into the comparison: a route
// the JSON found the same is updated, and the fields name the files or the
// spec.
func withMedia(changes []Change, from, to *Document) []Change {
	moved := mediaChanges(from, to)
	if len(moved) == 0 {
		return changes
	}
	for i, c := range changes {
		extra, ok := moved[c.ID]
		if c.Kind != "route" || !ok || (c.Action != ActionSame && c.Action != ActionUpdate) {
			continue
		}
		for _, f := range extra {
			if !slices.Contains(c.Fields, f) {
				c.Fields = append(c.Fields, f)
			}
		}
		slices.Sort(c.Fields)
		c.Action = ActionUpdate
		changes[i] = c
	}
	return changes
}
