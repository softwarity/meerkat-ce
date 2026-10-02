package confrepo

import (
	"net/url"
	"strings"
)

// What each forge wants, because they do not agree and the differences are not
// cosmetic.
//
// THE USERNAME IS THE TRAP. A token travels over HTTP basic auth, which has two
// halves, and every forge decided the first one differently: GitHub ignores it,
// GitLab wants "oauth2", Bitbucket REFUSES anything but "x-token-auth" for an
// access token. Get it wrong and the answer is "authentication failed" - the
// same sentence a wrong token gives - so an operator with a perfectly good
// token spends an afternoon on it. Hence a table: the username is a field with
// a default per host, not something the admin has to know.
//
// The SCOPES are the other half. "repo" on GitHub is full control of every
// private repository the account can reach, where a fine-grained token with
// Contents: Read and write on one repository is all this needs. Saying so where
// the field is filled in is the difference between a least-privilege token and
// the one that was quickest to create.
//
// The table is matched on the HOST, and a host nobody listed still works: the
// generic answer is a username of "git" and a sentence saying the token needs
// to be allowed to read and write this repository's contents. A forge this does
// not know is not a forge this refuses.

// Provider is what is known about one forge.
type Provider struct {
	// ID is matched by the console against a URL's host.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Hosts are the domains that identify it. A self-hosted instance answers to
	// none of them, which is why Suffix exists.
	Hosts []string `json:"hosts,omitempty"`
	// User is the HTTP basic username this forge wants beside the token.
	User string `json:"user"`
	// UserFixed says the username is not a choice: the forge refuses any other.
	UserFixed bool `json:"userFixed,omitempty"`
	// Needs is what to grant the token, in the forge's own words.
	Needs string `json:"needs"`
	// Create is where the token is minted, for a link.
	Create string `json:"create,omitempty"`
	// Note is the one extra thing that catches people out.
	Note string `json:"note,omitempty"`
}

// Providers is the table, in the order a list shows them.
var Providers = []Provider{
	{
		ID: "github", Name: "GitHub", Hosts: []string{"github.com"},
		User: "x-access-token",
		Needs: "A fine-grained token, Repository access limited to this repository, " +
			"Repository permissions > Contents: Read and write. Nothing else - and not a " +
			"classic token with repo, which grants full control of every private repository " +
			"the account can reach.",
		Create: "https://github.com/settings/personal-access-tokens",
		Note:   "The username is ignored by GitHub; any non-empty value works.",
	},
	{
		ID: "gitlab", Name: "GitLab", Hosts: []string{"gitlab.com"},
		User: "oauth2", UserFixed: true,
		Needs: "A project access token with the write_repository scope and the Maintainer role " +
			"(Developer is enough when the branch is not protected). A personal access token " +
			"with write_repository works too.",
		Create: "https://gitlab.com/-/user_settings/personal_access_tokens",
		Note:   "GitLab wants the username oauth2 beside the token, whatever the token is called.",
	},
	{
		ID: "bitbucket", Name: "Bitbucket Cloud", Hosts: []string{"bitbucket.org"},
		User: "x-token-auth", UserFixed: true,
		Needs: "A repository access token with the repository:write scope (repository:read is " +
			"not enough to push).",
		Note: "Bitbucket REFUSES any username but x-token-auth for an access token. An app " +
			"password is the other way in, and that one takes your account name as the username.",
	},
	{
		ID: "azure", Name: "Azure DevOps", Hosts: []string{"dev.azure.com", "visualstudio.com"},
		User: "meerkat",
		Needs: "A personal access token with Code: Read & Write, scoped to the project that " +
			"holds this repository.",
		Create: "https://dev.azure.com",
		Note:   "Azure ignores the username; it reads the token from the password.",
	},
	{
		ID: "gitea", Name: "Gitea or Forgejo", Hosts: []string{"codeberg.org"},
		User: "git",
		Needs: "An access token with write:repository. On Gitea and Forgejo the username is the " +
			"account the token belongs to, so put that in the username field if git is refused.",
		Note: "Self-hosted: match it by hand, the host is your own.",
	},
}

// GenericProvider is the answer for a host nobody listed - a self-hosted Gitea,
// a company GitLab, a bare git-http-backend. It is deliberately not a refusal:
// the protocol is the same everywhere, only the paperwork differs.
var GenericProvider = Provider{
	ID: "generic", Name: "Another git server", User: "git",
	Needs: "A token or password allowed to read and write this repository's contents, " +
		"and the username that forge expects beside it.",
}

// ProviderFor matches a repository URL to what is known about its forge.
func ProviderFor(repo string) Provider {
	host := hostOf(repo)
	if host == "" {
		return GenericProvider
	}
	for _, p := range Providers {
		for _, h := range p.Hosts {
			if host == h || strings.HasSuffix(host, "."+h) {
				return p
			}
		}
	}
	return GenericProvider
}

func hostOf(repo string) string {
	u, err := url.Parse(strings.TrimSpace(repo))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// BasicUser is the username to send beside the token: the one the location
// carries, or the forge's default. Never empty - basic auth with an empty
// username is refused by some forges and silently accepted by others, which is
// the worst of both.
func BasicUser(r Remote) string {
	if u := strings.TrimSpace(r.User); u != "" {
		return u
	}
	return ProviderFor(r.URL).User
}

// Paperwork is the sentence to add to a refused credential, so that "403" turns
// into something the operator can act on without leaving the screen.
func Paperwork(repo string) string {
	p := ProviderFor(repo)
	return p.Name + " wants: " + p.Needs
}
