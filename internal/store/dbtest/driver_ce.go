//go:build !ee

package dbtest

// No driver here: this image runs on the embedded database, so there is
// nothing for these helpers to connect to and the tests that need one skip.
const haveDriver = false
