// Command catalogdump prints the routing catalogue - every predicate and
// filter, with its phase, its documentation and its parameters - as JSON.
//
// It exists for the documentation: the reference pages under docs/content for
// the forty-four bricks are written from THIS, not from someone reading the
// registries and retyping them. When a brick gains a parameter, the dump says
// so and the page can be brought back in step.
//
//	go run ./tools/catalogdump | jq '.[] | select(.type == "strip-prefix")'
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/softwarity/meerkat/internal/routing"
)

func main() {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(routing.Catalog()); err != nil {
		fmt.Fprintln(os.Stderr, "catalogdump:", err)
		os.Exit(1)
	}
}
