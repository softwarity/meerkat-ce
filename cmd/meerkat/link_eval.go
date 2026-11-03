//go:build ee

package main

// The evaluation notice, in one import. ee/eval's init() fills
// internal/evalmark, and every surface that draws a mark draws it because the
// string is no longer empty.
//
// It is part of every Enterprise build: there are two images, the community
// one, which has neither the Enterprise code nor this, and softwarity/meerkat,
// which has both. That one is an evaluation until a license says it is Team
// or Enterprise (EVAL-06); nothing else takes the notice away, and no build
// exists without it.
import _ "github.com/softwarity/meerkat/ee/eval"
