---
title: Configuration
section: The console
order: 162
summary: Moving a gateway's setup around: named configurations, restore points, and the whole database backed up, restored and moved.
---

# Configuration

**Meerkat > Configuration**, root only. Three tabs, because three different
questions were sharing one page.

What travels in a configuration: **routes, roles, tenants and their groups,
authorities, OIDC clients, mail relay, themes and gateway settings**. An OIDC
client's secret travels as its sha256 only (`secretHash`), never in clear: the
client imported elsewhere keeps the same secret, and the application keeps working
with nothing to change. What stays behind: **users and who belongs where,
sessions, and the values of the vault** (a configuration names the entries it
uses, never what they hold) - they are not configuration, they are what this
gateway lives with.

| Tab | Answers |
|---|---|
| **Compose** | What differs between two configurations, and what the result should be |
| **History** | Going back to any moment of this gateway's configuration |
| **Database** | Backing up, restoring and moving the whole database |

Each tab is a route of its own, so a bookmark comes back to it.

## Compose

![The Compose tab: what runs on the left, a saved configuration on the right, the result between them, each difference framed in red](img/console/configuration.webp)

Three columns: **what you start from** on the left, in blue; **the result** in the
middle, in green; **what you compare it with** on the right, in orange. Nothing moves
until a result is applied or saved: this screen is a workbench, not a switch.

The screen opens with what runs on both sides, so it says the two are identical.
Choose something else in the header of a column:

| Source | What it is |
|---|---|
| **Running** | What this gateway serves |
| **A saved configuration** | One of the copies kept on the shelf |
| **A git location** | The directory of a repository, read at this moment (*Enterprise*) |
| **A file** | A YAML, JSON or package (`.zip`) you pick or drop on the page. It lands on the right and is read, not kept |

### Deciding

**Every difference is framed in red**, and the frame goes once you have touched it.
A ribbon joins what feeds the result to its place in the middle. To decide:

- **An object that exists on one side only** has a radio on each side: taken, or
  left out.
- **An object on both sides that differs** opens into its fields, down to the leaves:
  `timeouts.response` from one side and `upstream` from the other is one result. A
  radio on each value says which one the result holds, and the **x** on a result line
  leaves the value unset.
- **A list** is its elements as chips: a filled chip is in the result, a pale dashed
  one is out. Click to bring one in or take it out, drag to reorder.
- **An object the two sides share** has *in the result* ticked: untick it to leave it
  out anyway.
- **Code** - a custom code block, an `onChange` hook - shows a **code** button
  beside the value: it opens the two sides line by line. A block that only moved
  (from the end of the body to the head, say) is not code that changed: it says
  where it went, and offers no comparison when the code is the same.
- **The files beside a route** - those it serves, its OpenAPI spec - are compared
  by content: a **files** line names those that differ, are added or removed, and
  a spec that differs is said too.

![The code button of a rewritten CSS block: the two sides line by line, the changes marked](img/console/configuration-code.webp)

The double tick in a column's header takes that side for everything still
undecided, and **Reset** beside the result forgets every choice. **Only differences**
hides what is the same, and the chips above the columns filter by kind of object.

An object's *order* in the routing is never offered: two routes on the same rank
would make routing ambiguous, so the result keeps what the left side says.

**Dependencies follow.** A route taken from the right brings along the roles it
names, from the side that has them. Leave such a role out and the reference is said
to be missing; only a role's parent or a group's tenant blocks applying, since the
gateway cannot hold the object without them.

**Two objects with the same name under two ids** are offered as a pair, never
paired for you. Paired, the result keeps the left id and the gateway updates the
object instead of removing one and adding the other.

### The portal, drawn

![The portal setting: under its fields, a miniature of the bar each side describes](img/console/configuration-portal.webp)

A catalogue reads badly as a list, so under the fields of the portal setting each
column shows a **miniature of the real bar**. Click an entry in a miniature to open
it: a container shows its modules, which is how the sub-routes are compared.

### What the result would lack

A banner above the columns, and a count in the header of the result, say what is
missing **before** anything is applied:

- the vault entries the document names and this gateway does not hold (they are
  reserved empty on applying, to fill in on the [vault](/docs/console/vault) screen);
- the specs and files a plain file does not carry, when the source is one;
- what this edition does not apply;
- the OIDC bindings left in suspense, and the dependencies a choice left out.

### What to do with the result

![The Save menu of the result: apply it as current, keep it on the shelf, save it on the filesystem](img/console/configuration-save.webp)

**Save** in the header of the result:

| Entry | What it does |
|---|---|
| **Apply as current** | Asks first, in figures and in names, with what would be removed and what is missing. Then the gateway serves the result, and a [restore point](#history) keeps what it served before |
| **On the shelf** | Keeps the result under a name, applying nothing. A name that exists asks to replace it |
| **On the filesystem** | Opens a window to choose the shape, and says what it leaves behind |
| **Push to...** | Offered when a git location is loaded on the right: see [Git repository](#git-repository) |

Running on both sides and **On the shelf** names what runs. A saved configuration on
the left and **On the filesystem** exports it.

![Saving on the filesystem: a file or a package, and what the chosen shape leaves behind](img/console/configuration-file.webp)

The window offers two shapes, and lists what the chosen one leaves behind:

- **As a file** - plain YAML, text only. Images, specs and the files of a route stay
  behind; importing it keeps the images in place.
- **As a package with its assets** - a zip: the configuration reads and diffs as text,
  its assets sit beside it as files. Offered first when there is something to carry,
  and greyed out when there is not.

Neither carries the values of the vault entries it names, nor a secret held in clear on
this gateway: the window names them, so that they can be moved or copied before the file
leaves.

### The shelf

The shelf is the list of saved configurations: copies, that change nothing about what
is served until one is applied. To load one, choose it in the header of a column. The
**More** button of that column then acts on it:

| Entry | What it does |
|---|---|
| **Read the file** | Opens it in the drawer, to read and correct. Saving replaces its document and applies nothing. On Running, the file reads but does not change: what runs changes on this screen, where a change shows what it does first |
| **Rename** | Renames the copy |
| **Delete** | Deletes the copy; the column goes back to Running |

> [!NOTE]
> Every gesture works in both editions. What differs is the size of the shelf: the
> community edition keeps **three** saved configurations at a time and says where you
> are in the Save menu (*2 of 3 saved*), before the cap is reached; Enterprise keeps as
> many as you like.

### From the API

The screen composes on the server at each click, so an agent replays the same
recipe. `POST /api/configurations/compose` previews, and
`/compose/apply`, `/compose/save`, `/compose/export` and `/compose/push` act on the
result. The [agent](/docs/agent/overview) tool is `compose_configuration`.

For a script that does not need a recipe, two plain calls move a whole document:
`GET /api/config/export` reads what runs, and `POST /api/config/import` applies a file
(`?prune=true` removes what it does not carry, `?keep=kind:id` keeps a line as the gateway
holds it). `POST /api/configurations` names what runs.

## Git repository

*Enterprise edition.* The configurations can live in a git repository: a history
with names on it, review before a change, and one source of truth across several
installations.

An export is **public by construction** - a declared secret field leaves as its
`${name}` reference or not at all - which is what makes putting one in a
repository a reasonable thing to do. And its bytes are deterministic, so two
exports of the same state produce the same file and a diff shows only what moved.

### Locations, not branches

A **git location** is a repository, a branch and a **directory** inside it. Several
locations share one repository, one directory per platform:

```
platforms/acme/meerkat.yaml
platforms/acme/assets/logo.png
platforms/foo/meerkat.yaml
```

That layout is the one an export already produces, so there is no file to pick
and nothing to agree on.

> [!TIP]
> Prefer one directory per platform on one branch over one branch per platform.
> A branch is for a change that intends to merge; platforms are parallel for
> good. Branch-per-platform means cherry-picking every fix into fourteen
> branches, forever. Keep branches for a **proposed** change - that is a pull
> request - and tags for *what Acme was running on the 2nd*.

### Pull is loading, push is resolving

**To read a location**, choose it in the header of a column. It is read at that
moment, nothing is stored and **nothing is applied**: the gateway goes on serving what
it served. From there it is a side like any other: compose, apply, keep on the shelf.

This is deliberate and it is not negotiable: whoever can write to that branch must
not be able to reconfigure a gateway. There is no continuous reconciliation, and no
branch is watched.

**To write a location**, load it **on the right** and decide what differs: **Save >
Push to...** then appears, naming that location and no other. A push replaces what the
location's directory holds with the result, so it goes only where you have just
looked, at the revision you read. If somebody pushed since, it is refused and asks
you to read the right side again: there is no force. Once pushed, the right side is
read again, and the two read as the repository now holds them.

A push writes the repository and **saves nothing here**. The commit is **attributed to
the operator who clicked**. That is the point of doing this in the product rather than
in a script: a repository whose history reads *meerkat* for every change answers
nothing six months later. Nothing is lost either: what a push replaced is the previous
commit of the history. Only the location's directory is written; another platform's
directory in the same repository is left alone.

### The token

HTTPS with a token. The token is a [vault](/docs/console/vault) reference, never
a literal - a location is a row the console reads and a snapshot carries.

What to grant, and the **username to send beside the token**, differ per forge -
and they all report a wrong username as the same *authentication failed* a wrong
token gives, which is how a perfectly good token costs somebody an afternoon. So
the form starts with the **forge**: GitHub, GitLab, Bitbucket Cloud, Azure DevOps,
Gitea / Forgejo, or another git server. A known forge fills in its host - change it
for a self-hosted GitLab or Forgejo - and you type only the repository
(`owner/repository`); the URL is made from the two. Under it, the steps that make the
token in that forge's own menus, with a link to the token page of this very
repository, and the username it sends - asked only where the forge leaves it to you:

| Forge | Username | What the token needs |
|---|---|---|
| **GitHub** | ignored | A **fine-grained** token, this repository only, *Contents: Read and write*. Not a classic token with `repo`: that is full control of every private repository the account can reach |
| **GitLab** | `oauth2` | A project access token with `write_repository` and the Maintainer role (Developer where the branch is not protected) |
| **Bitbucket Cloud** | `x-token-auth` | A repository access token with `repository:write` |
| **Azure DevOps** | ignored | A personal access token with *Code: Read & Write* |
| **Gitea / Forgejo** | the token's account | An access token with `write:repository` |

**Check** proves the repository answers, the credential is accepted and the branch
exists, and writes nothing - so a wrong token is found on the screen that sets it.

### From an agent

`list_git_locations`, `save_git_location` (create or change a location; the token stays a vault reference), `pull_configuration` and `push_configuration`
([agents](/docs/agent/overview)). An agent works on saved configurations: a pull
refreshes a saved copy, a push commits one to the location it names, and the gateway
remembers which location and which revision for that copy. An agent that asks the
gateway to push holds no credential and needs no checkout: the token in the vault,
scoped to one repository, is the one doing the work.

## History

The gateway keeps its own tape: **one restore point per change** that moves the
configuration's fingerprint, newest first, grouped by day. Rows read as time and
person first, then the words the [audit trail](/docs/console/audit-and-issues)
used for the same change at the same second.

Open a point to read its file. **Restore** shows its plan, like any other change,
before going back, and leaves a point of its own: the tape never loses the state
it was asked to leave. A point can also be saved onto the shelf under a name,
which is how *what it looked like on Tuesday* becomes a configuration you keep.

A saved configuration and a restore point are different objects on purpose: one
is intentional and named (*the Acme setup*), the other automatic and timestamped
(*what it looked like at 14:32*).

## Database

![The Database tab: the pause, the database this gateway runs on and the platform it is deployed with, the download and the steps to restore](img/console/database.webp)

A coherent copy of the **whole database**, taken while the gateway runs: routes
and vault, but also users, tenants, sessions and the audit trail. This is
what a backup restores; a configuration export is what a second gateway
reproduces. The tab was called *Snapshot* until it also moved the database; the
old address still leads here.

Meerkat takes the snapshot itself, because copying a live database with `cp` can
catch it mid-write and nothing says so until the day you restore it. Scheduling,
retention and shipping it off the host belong to your backup tool.

**There is no restore button, deliberately.** A database cannot be swapped under
the process holding it open. The tab prints the exact commands instead, with this
installation's own paths, **for the platform it runs on** - the runtime says
which: a Kubernetes or OpenShift Deployment and its Helm release, a Swarm
service, a compose project, a container. *Written for*
switches to another, for a runtime the gateway could not ask. The old file is
kept aside until the new one has proven itself.

> [!WARNING]
> If the master key sits next to the database, backing them up together undoes
> the encryption at rest - exactly as storing a vault file beside its passphrase
> would. Keep the key in a secret manager, or supply it through
> `MEERKAT_VAULT_KEY`; the tab says which of the two this gateway does.

The same tab **moves** the database, between the embedded one and PostgreSQL,
into a file or straight into a server. A move is **one numbered procedure**,
shown from the start: create the database and its user (the SQL a database
administrator runs), test the server, pause, copy, switch the deployment, check
a route and a sign-in, and the way back - each step ticked as it is done.

![The move to PostgreSQL before the copy: the target, the SQL that creates it, the steps still to do](img/console/database-migrate.webp)

The gateway remembers the copy - where it went and how much it carried, never
the password - so a reload in the middle of a migration, on any node, comes back
to the same procedure. **Dismiss** forgets it; it goes by itself once the
gateway runs on that database. See
[moving the database](/docs/operations/backup-restore#moving-the-database-embedded-and-postgresql).

## Traps

- **An export does not carry the vault.** Take the
  [vault file](/docs/console/vault) too, or the second gateway comes up with
  references pointing at nothing.
- **A configuration is not a backup.** Users and sessions are not in it. That is
  what the Database tab is for.
- **The community image leaves the Enterprise parts out.** A configuration
  exported from an Enterprise gateway imports into a community one without its
  working hours, layout, hidden mark, OpenTelemetry export, directories, OIDC
  clients and ACME authorities and orders (the HTTPS redirect and HSTS stay); the
  plan lists what it left out. A route bound to an OIDC client keeps its mechanism,
  unserved.
- **A route does not lose its OIDC client in an import.** A client a route still
  signs people into is kept, even by a pruning document that does not list it,
  so a configuration exported before clients travelled still activates. A route
  naming a client that neither the document nor the gateway declares imports as
  it is, and the plan says so: declare the client under **OIDC / JWT**.
- **Reading a git location changes nothing.** It is one side of the screen. The
  gateway switches when somebody applies a result, not when the repository moves.
- **A location belongs to this installation, not to the document.** It is never
  exported: a configuration that could repoint the gateway at another repository
  would overwrite another customer's directory on the next push.
