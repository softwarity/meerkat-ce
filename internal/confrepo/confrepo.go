// Package confrepo is the seam between a configuration and a git repository
// (CFG-07): the questions the control plane asks of a remote, with the answers
// living in ee/gitdriver.
//
// GIT IS A TRANSPORT HERE, NOT A FEATURE. Everything that makes a configuration
// travel safely is already built elsewhere and is not repeated: the export is
// public by construction (a declared secret field leaves as its ${name}
// reference or not at all, internal/config), its bytes are deterministic, the
// shape of a directory is config.Split, and what a pulled document would change
// is config.PreviewSwitch. What this package adds is the three acts a remote
// understands - prove, read, write - and nothing else.
//
// THREE RULES THE DRIVER IS HELD TO, because they are what stops a repository
// from becoming a way to take a gateway over:
//
//   - a Fetch APPLIES NOTHING. It hands back bytes; the caller shelves them as
//     a saved configuration, and a human still has to activate it. Whoever can
//     write to the branch must not be able to reconfigure the gateway.
//   - a Commit NEVER FORCES. It is handed the revision the caller believes the
//     branch is at, and a branch that has moved is a refusal to report, never
//     history to overwrite.
//   - a Check WRITES NOTHING. It exists so that a wrong token is found on the
//     screen that sets it, rather than during the first push.
package confrepo

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// Remote is one named place a configuration lives: a repository, a branch, and
// a directory inside it.
//
// The DIRECTORY rather than a file is what makes several of these share one
// repository - one per customer platform - and it is also what lets the layout
// be config.Split's: the document at <dir>/meerkat.yaml, its pictures under
// <dir>/assets/. A remote therefore names exactly one document, which is what
// a saved configuration can be bound to and compared against.
//
// The token arrives EXPANDED: the caller resolves the vault reference, this
// package never reads the vault, and nothing here keeps a credential past the
// call it was handed to.
type Remote struct {
	Name   string
	URL    string
	Branch string
	Dir    string
	// User is the HTTP basic username sent beside the token. Empty means the
	// forge's default (see BasicUser): every forge decided this differently and
	// two of them refuse anything else.
	User        string
	Token       string
	AuthorName  string
	AuthorEmail string
}

// ErrNoDriver is what the community image answers. It is a sentinel rather than
// a message because the caller turns it into an edition refusal, which is a
// different sentence from a git failure.
var ErrNoDriver = errors.New("confrepo: this build carries no git driver")

// ErrMoved says the branch is not where the caller thought it was, so the
// commit was not pushed. A sentinel because it is the one failure with a cure
// the console can offer: pull first.
var ErrMoved = errors.New("confrepo: the branch has moved since it was last read")

// ErrAbsent says the remote's directory holds no document. Told apart from a
// failure because it is the ordinary state of a remote nothing has been pushed
// to yet, and the console offers a push rather than an error.
var ErrAbsent = errors.New("confrepo: no configuration at this location")

// Driver is what an edition carrying git implements.
type Driver interface {
	// Check proves the repository answers, the credential is accepted and the
	// branch exists. It writes nothing.
	Check(ctx context.Context, r Remote) error
	// Fetch reads the document and its assets at the head of the branch, keyed
	// by their path RELATIVE to the remote's directory - so the caller gets
	// exactly what config.Split produced, whatever directory it was put in.
	// The revision is the commit they were read at.
	Fetch(ctx context.Context, r Remote) (files map[string][]byte, rev string, err error)
	// Commit writes files (relative paths again) into the remote's directory
	// and pushes. Paths the directory holds and files does not are REMOVED, so
	// that a logo taken out of a configuration leaves the repository too.
	//
	// expect is the revision the caller last read; a branch that has moved on
	// gives ErrMoved and nothing is pushed. An empty expect is a first write,
	// not a licence to clobber: the driver still refuses a non-fast-forward.
	Commit(ctx context.Context, r Remote, msg string, files map[string][]byte, expect string) (rev string, err error)
}

var (
	mu     sync.RWMutex
	driver Driver
)

// Register is called from ee/gitdriver's init.
func Register(d Driver) {
	mu.Lock()
	defer mu.Unlock()
	driver = d
}

// Available reports whether this build can talk to a repository at all. The
// console asks so that it can show the feature as present and inert rather
// than hide it.
func Available() bool {
	mu.RLock()
	defer mu.RUnlock()
	return driver != nil
}

// Get returns the driver, or ErrNoDriver.
func Get() (Driver, error) {
	mu.RLock()
	defer mu.RUnlock()
	if driver == nil {
		return nil, ErrNoDriver
	}
	return driver, nil
}

// CleanDir normalises a remote's directory to the form the drivers and the
// paths agree on: no leading or trailing slash, "" for the repository root.
//
// Here rather than in each driver because it decides what a stored remote
// MEANS: "/platforms/acme/" and "platforms/acme" have to be one location, or
// two remotes that look identical on screen write to two directories.
func CleanDir(dir string) string {
	return strings.Trim(strings.TrimSpace(strings.ReplaceAll(dir, "\\", "/")), "/")
}

// DocumentName and AssetDir are the layout INSIDE a location's directory, which
// is config.Split's: one document, its media in a directory beside it.
//
// Declared here because two sides depend on it - the driver walks a tree looking
// for these names, the control plane assembles a document out of them - and a
// layout agreed in two places is a layout that will disagree. A test holds them
// equal to internal/config's own.
const (
	DocumentName = "meerkat.yaml"
	AssetDir     = "assets"
)

// Join places a file of a Split inside a remote's directory.
func Join(dir, name string) string {
	if dir = CleanDir(dir); dir == "" {
		return name
	}
	return dir + "/" + name
}
