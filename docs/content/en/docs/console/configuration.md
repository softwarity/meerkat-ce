---
title: Configuration
section: The console
order: 162
summary: Moving a gateway's setup around: named configurations, restore points, and a full database snapshot.
---

# Configuration

**Meerkat > Configuration**, root only. Three tabs, because three different
questions were sharing one page.

What travels in a configuration: **routes, roles, authorities, mail relay, themes
and gateway settings**. What stays behind: **users, organisations, sessions and
the vault** - they are not configuration, they are what this gateway lives with.

| Tab | Answers |
|---|---|
| **Management** | What is running, what is on the shelf, and moving files in and out |
| **History** | Going back to any moment of this gateway's configuration |
| **Snapshot** | Backing up and restoring the whole database |

Each tab is a route of its own, so a bookmark comes back to it.

![The Configuration screen on the Management tab: the current configuration on the first row, two saved ones under it](img/console/configuration.webp)

The current configuration on the first row, marked as matching *Known-good
baseline*, and two shelf rows under it - one of which carries the *Current* chip.
*Import a file* works in both editions; *Import from git* carries the
Enterprise mark.

## Management

The **first row is what this gateway serves**. The rows under it are copies on a
shelf. Saving, duplicating or deleting one of those changes nothing about what is
served.

The icon on the first row is a comparison, and it is the most useful thing on the
screen:

| Icon | What runs |
|---|---|
| green | is byte for byte a saved configuration - its name is beside it |
| amber | has drifted from the one it was saved as |
| grey | was never saved under any name |

The shelf row it was set from carries the same verdict as a chip: *Current*, or
*Current, changed*.

**Actions.** The current row saves under a name and exports. A shelf row can be
set as current, duplicated, exported or deleted. Clicking any row opens its file
in the drawer, to read.

Only **Set as current** crosses from the shelf to the gateway, so it is the only
action that asks - and it asks with **the list of what it would change**, plus a
warning first when what is running was never saved: that is the one move with
nothing to come back to.

### Exporting

The dialog says what the file carries and what it does not, then offers two
shapes:

- **Plain YAML** - text only. Images stay behind, and an import keeps whatever is
  in place.
- **Package** - a zip: the configuration reads and diffs as text, images sit
  beside it as files.

### Importing

**Import a file** takes `.yaml`, `.yml`, `.json` or `.zip`, and asks where it
should land:

1. **Save it under a name** - onto the shelf, touching nothing.
2. **Replace the current one** - what is not in the file goes away.
3. **Add to the current one** - what is in the file is added or updated, and
   nothing is removed.

The two that touch the gateway show their plan first, object by object. If the
file refers to vault entries this gateway does not have, a second dialog lists
them so you can fill them in straight away.

> [!NOTE]
> Every action works in both editions - save, import, duplicate, set as
> current, export. What differs is the size of the shelf: the community edition
> keeps **three** saved configurations at a time and says where you are beside
> the Import button (*2 of 3 saved*), before the cap is reached; Enterprise keeps
> as many as you like.

### Comparing two saved configurations

With two or more on the shelf, a saved row offers **Compare with another saved
configuration**: choose the other one, and the dialog lists what changes going
from the first to the second - added, updated, removed, object by object, with
the fields that moved inside an update. What runs is left out: it is the question
asked of a customer's configuration and the template it was made from, before
touching either. `GET /api/configurations/{id}/compare/{other}`.

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

### The loop: pull, read, activate

**Import from git** reads a location into a saved configuration and **applies
nothing**. The gateway goes on serving what it served; the answer is the plan -
what activating it would add, change and remove, plus the vault entries it
expects and this installation has not got. You read that, then **Set as current**
when it says what you expected, and read the plan again on the way.

This is deliberate and it is not negotiable: whoever can write to that branch
must not be able to reconfigure a gateway. There is no continuous
reconciliation, and no branch is watched.

**Export to git** commits the **saved copy** - not the running state, so save it
under a name first if that is what you want - and the commit is **attributed to
the operator who clicked**. That is the point of doing this in the product rather
than in a script: a repository whose history reads *meerkat* for every change
answers nothing six months later.

The two directions are plain: **a pull replaces** the saved configuration with what
the repository holds, **a push replaces** what the location's directory holds with
the saved configuration - whatever somebody else put there. Nothing is lost: what a
push replaced is the previous commit of the history. Only the location's directory is
written; another platform's directory in the same repository is left alone. To see
the repository's version before choosing, pull it into a new configuration (*Import
from git*) and compare.

A row says where it stands: *synced* with the date of the last push or pull,
*changed here since*, or *never pushed*. "Synced" is what this gateway knows at its
last exchange - whether somebody changed the repository since is a pull away.

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

`list_git_locations`, `pull_configuration` and `push_configuration`
([agents](/docs/agent/overview)). An agent that asks the gateway to push holds no
credential and needs no checkout: the token in the vault, scoped to one
repository, is the one doing the work.

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

## Snapshot

A coherent copy of the **whole database**, taken while the gateway runs: routes
and vault, but also users, organisations, sessions and the audit trail. This is
what a backup restores; a configuration export is what a second gateway
reproduces.

Meerkat takes the snapshot itself, because copying a live database with `cp` can
catch it mid-write and nothing says so until the day you restore it. Scheduling,
retention and shipping it off the host belong to your backup tool.

**There is no restore button, deliberately.** A database cannot be swapped under
the process holding it open. The tab prints the exact commands instead, with this
installation's own paths, and reminds you to keep the old file until the new one
has proven itself.

> [!WARNING]
> If the master key sits next to the database, backing them up together undoes
> the encryption at rest - exactly as storing a vault file beside its passphrase
> would. Keep the key in a secret manager, or supply it through
> `MEERKAT_VAULT_KEY`; the tab says which of the two this gateway does.

The same tab **moves** the database, between the embedded one and PostgreSQL,
into a file or straight into a server, with the **Pause** switch that makes the
copy whole: see [moving the database](/docs/operations/backup-restore#moving-the-database-embedded-and-postgresql).

## Traps

- **An export does not carry the vault.** Take the
  [vault file](/docs/console/vault) too, or the second gateway comes up with
  references pointing at nothing.
- **Replace prunes, Add does not.** Read the plan; it is the difference between a
  merge and a wipe.
- **A configuration is not a backup.** Users and sessions are not in it. That is
  what the Snapshot tab is for.
- **The community image leaves the Enterprise parts out.** A configuration
  exported from an Enterprise gateway imports into a community one without its
  working hours, layout, hidden mark, OpenTelemetry export, directories and ACME
  authorities and orders (the HTTPS redirect and HSTS stay); the plan lists what
  it left out.
- **Pulling from git changes nothing.** It shelves a document and shows you the
  plan. The gateway switches when somebody activates it, not when the repository
  moves.
- **A location belongs to this installation, not to the document.** It is never
  exported: a configuration that could repoint the gateway at another repository
  would overwrite another customer's directory on the next push.
