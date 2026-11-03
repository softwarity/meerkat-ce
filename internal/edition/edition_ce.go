//go:build !ee

// Package edition is the ONE place the two artifacts differ.
//
// Meerkat ships as two images built from the same commit, both on Docker Hub:
// softwarity/meerkat-ce, the community one, and softwarity/meerkat, which a
// license turns into Team or Enterprise. What
// separates them is not a flag read at runtime - it is what the linker put in
// the binary, decided by the `ee` build tag.
//
//	go build ./cmd/meerkat            -> community
//	go build -tags ee ./cmd/meerkat   -> Enterprise
package edition

// Name is what the startup line reports.
const Name = "ce"

// Enterprise is the ONE question the product asks about the image, answered at
// COMPILE time. There is no per-feature key. What separates Team from
// Enterprise, on the image that holds both, is a license (internal/license).
//
// Most Enterprise code is simply absent here (ee/directories, ee/layouts), and
// absent code refuses by itself. This constant exists for the handful of
// decisions whose code is NECESSARILY shared - a second organisation, opening
// hours, the Meerkat mark - where something has to say no.
const Enterprise = false
