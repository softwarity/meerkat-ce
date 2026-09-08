//go:build ee

package store

import "github.com/softwarity/meerkat/ee/pgdriver"

// The wiring, in one place rather than one per binary.
//
// It sits inside the storage layer so that EVERY build carrying the driver -
// the gateway, and each test binary that opens a database - is wired the same
// way. Doing it from cmd/meerkat instead left every test package to repeat it,
// and the ones that forgot got the embedded-only refusal from a build that had
// a driver, which reads as a bug in the seam rather than a missing line.
//
// No cycle: ee/pgdriver imports the driver and nothing of ours.
func init() { RegisterExternalDatabase(pgdriver.Dial, pgdriver.Listen) }
