//go:build ee

package main

// The Enterprise packages, linked in for their side effect: each registers
// what it implements - a sign-in driver, the page arrangements. THIS FILE is
// the seam between the two images. The community build does not compile it,
// so that code is not in the binary at all, and the compiler proves the trunk
// never depended on it.
import (
	_ "github.com/softwarity/meerkat/ee/devplug"
	_ "github.com/softwarity/meerkat/ee/directories"
	_ "github.com/softwarity/meerkat/ee/layouts"
)
