package meerkat

import (
	_ "embed"
	"strings"
)

// Features is FEATURES.md as it stood when the binary was built: the product's
// contract, one row per feature, with the edition each belongs to. The
// console's License screen lists the Enterprise ones from it (CONSOLE-14), so
// the list cannot drift from the contract - there is no second copy to forget.
//
//go:embed FEATURES.md
var Features string

// Feature is one row of the contract, as much of it as a screen needs.
type Feature struct {
	ID string `json:"id"`
	// Status is done, partial, planned or retired ([x], [~], [ ], [-]).
	Status string `json:"status"`
	// Edition is CE, EE, or CE/EE for a feature split between the two.
	Edition string `json:"edition"`
}

// EnterpriseFeatures are the rows the Enterprise edition carries, in the
// order the contract lists them, retired ones left out.
func EnterpriseFeatures() []Feature {
	var out []Feature
	for _, f := range ParseFeatures(Features) {
		if strings.Contains(f.Edition, "EE") && f.Status != "retired" {
			out = append(out, f)
		}
	}
	return out
}

// ParseFeatures reads the rows of the contract's table:
// | [x] | ID | **Title** | description | missing | edition |
func ParseFeatures(md string) []Feature {
	statuses := map[string]string{"[x]": "done", "[~]": "partial", "[ ]": "planned", "[-]": "retired"}
	var out []Feature
	for _, line := range strings.Split(md, "\n") {
		if !strings.HasPrefix(line, "| [") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), " | ")
		if len(cells) < 4 {
			continue
		}
		status, ok := statuses[strings.TrimSpace(cells[0])]
		if !ok {
			continue
		}
		out = append(out, Feature{
			ID:      strings.TrimSpace(cells[1]),
			Status:  status,
			Edition: strings.TrimSpace(cells[len(cells)-1]),
		})
	}
	return out
}
