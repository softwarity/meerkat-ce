package config_test

import (
	"testing"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/confrepo"
)

// The layout of a configuration's files is agreed by two packages: this one
// produces it (Split), and a git driver walks a tree looking for it. Two
// constants are two chances to disagree, so they are held equal here - a rename
// on either side fails this rather than making every pull answer "there is no
// meerkat.yaml here".
func TestTheGitLayoutIsTheBundleLayout(t *testing.T) {
	if confrepo.DocumentName != config.BundleName {
		t.Errorf("a git location looks for %q and a bundle is called %q",
			confrepo.DocumentName, config.BundleName)
	}
	if got := config.AssetDirName(); confrepo.AssetDir != got {
		t.Errorf("a git location walks %q and the assets live in %q", confrepo.AssetDir, got)
	}
}
