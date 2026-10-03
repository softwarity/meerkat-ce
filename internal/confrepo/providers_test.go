package confrepo

import "testing"

// TestTheForgePickedDecidesTheUsername: a company GitLab answers to no known
// host, so only the forge picked when it was set up knows it wants oauth2.
func TestTheForgePickedDecidesTheUsername(t *testing.T) {
	selfHosted := Remote{URL: "https://git.acme.internal/ops/meerkat.git"}
	if got := BasicUser(selfHosted); got != GenericProvider.User {
		t.Fatalf("an unknown host with no forge picked: %q", got)
	}
	selfHosted.Provider = "gitlab"
	if got := BasicUser(selfHosted); got != "oauth2" {
		t.Fatalf("picked as GitLab, it wants oauth2: %q", got)
	}
	if got := BasicUser(Remote{URL: "https://bitbucket.org/acme/cfg.git"}); got != "x-token-auth" {
		t.Fatalf("a location from before the choice still goes by its host: %q", got)
	}
	for _, p := range append(Providers, GenericProvider) {
		if len(p.Steps) == 0 {
			t.Errorf("%s says nothing of how to make its token", p.ID)
		}
		if p.Host == "" && p.Path != "" {
			t.Errorf("%s has a path to type with no host to put it after", p.ID)
		}
	}
}
