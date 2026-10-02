// Package confrepotest is a repository that lives in memory, so that the
// control plane's git endpoints are tested without a git server.
//
// It is a separate package rather than a file next to the driver because a fake
// has no business in the binary: nothing outside a test imports this, and the
// compiler says so.
package confrepotest

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/softwarity/meerkat/internal/confrepo"
)

// Fake is one branch of one repository, keyed by FULL path - so a test can
// check that two remotes sharing a repository really did write to two
// directories.
type Fake struct {
	mu sync.Mutex
	// Files is the tree, by full path from the repository root.
	Files map[string][]byte
	// Rev is the current revision; it moves on every commit.
	Rev int
	// Fail, when set, is what every call answers. A wrong token and an
	// unreachable host are the same shape of failure to everything above.
	Fail error
	// Commits counts the pushes, and Messages keeps what they said: the audit
	// story is that the operator's name travels, so a test reads it here.
	Commits  int
	Messages []string
}

// New returns an empty repository at revision 1.
func New() *Fake { return &Fake{Files: map[string][]byte{}, Rev: 1} }

// Register makes this fake the driver for the rest of the test, and restores
// what was there when the test ends.
func (f *Fake) Register(t interface{ Cleanup(func()) }) {
	confrepo.Register(f)
	t.Cleanup(func() { confrepo.Register(nil) })
}

func (f *Fake) rev() string { return "rev" + strconv.Itoa(f.Rev) }

// Check answers Fail, or nothing.
func (f *Fake) Check(_ context.Context, _ confrepo.Remote) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Fail
}

// Fetch reads the remote's directory, rebasing the paths on it.
func (f *Fake) Fetch(_ context.Context, r confrepo.Remote) (map[string][]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		return nil, "", f.Fail
	}
	out := map[string][]byte{}
	for path, body := range f.Files {
		name, inside := under(r.Dir, path)
		// Only what a Split puts there: a remote names one configuration, not
		// everything that happens to sit below it.
		if !inside || (name != "meerkat.yaml" && !strings.HasPrefix(name, "assets/")) {
			continue
		}
		out[name] = body
	}
	if len(out) == 0 {
		return nil, "", confrepo.ErrAbsent
	}
	return out, f.rev(), nil
}

// under reports whether a full path sits in dir, and its name relative to it.
func under(dir, path string) (string, bool) {
	prefix := confrepo.Join(dir, "")
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	return strings.TrimPrefix(path, prefix), true
}

// Commit replaces the remote's directory with files, refusing a stale expect.
func (f *Fake) Commit(
	_ context.Context, r confrepo.Remote, msg string, files map[string][]byte, expect string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Fail != nil {
		return "", f.Fail
	}
	if expect != "" && expect != f.rev() {
		return "", fmt.Errorf("%w (at %s, you had %s)", confrepo.ErrMoved, f.rev(), expect)
	}
	for path := range f.Files {
		if name, inside := under(r.Dir, path); inside &&
			(name == "meerkat.yaml" || strings.HasPrefix(name, "assets/")) {
			delete(f.Files, path)
		}
	}
	for name, body := range files {
		f.Files[confrepo.Join(r.Dir, name)] = body
	}
	f.Rev++
	f.Commits++
	f.Messages = append(f.Messages, msg)
	return f.rev(), nil
}

// Put seeds a file at a full path, as a repository somebody else pushed to.
func (f *Fake) Put(path string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Files[path] = body
}

// Move advances the revision without touching the files, which is how a test
// reproduces "somebody else pushed while you were reading".
func (f *Fake) Move() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Rev++
}

// ErrDenied is a credential the repository refuses, for the Fail field.
var ErrDenied = errors.New("authentication required")
