//go:build eval

package main

// The evaluation image, in one import. THIS FILE is the whole difference
// between what a customer runs and what an evaluator runs: ee/eval's init()
// fills internal/evalmark, and every surface that draws a mark draws it
// because the string is no longer empty.
//
// Neither of the two shipped images compiles this file, so neither carries the
// marks nor a flag that hides them. Building without `eval` is what removes
// them, which is the only thing that can.
import _ "github.com/softwarity/meerkat/ee/eval"
