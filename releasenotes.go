// Package meerkat holds what the binary carries from the repository's root.
package meerkat

import _ "embed"

// ReleaseNotes is RELEASE_NOTES.md as it stood when the binary was built. The
// image is built before its version is chosen (docker/Dockerfile.stamp), so
// the notes of the release it becomes may still sit under NEXT RELEASE here:
// internal/admin/releasenotes.go knows it.
//
//go:embed RELEASE_NOTES.md
var ReleaseNotes string
