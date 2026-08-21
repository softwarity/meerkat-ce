//go:build ee

// Package edition is the ONE place the two artifacts differ - see the
// community file (edition_ce.go) for the whole story.
package edition

// Name is what the startup line reports.
const Name = "ee"

// Enterprise is true here: see the community file for why this is a constant
// and not a licence lookup.
//
// This package stays a LEAF - it imports nothing. The Enterprise packages are
// linked in by cmd/meerkat/link_ee.go instead: internal/auth asks this
// constant about the mark, so an import of ee/layouts from here would close
// the circle auth -> edition -> ee/layouts -> auth.
const Enterprise = true
