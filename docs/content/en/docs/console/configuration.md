---
title: Configuration
section: The console
order: 162
summary: Moving a gateway's setup around: named configurations, restore points, and a full database snapshot.
---

# Configuration

**Infra > Configuration**, root only. Three tabs, because three different
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
*Import a file* is capped Enterprise.

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
> Enterprise edition: keeping several configurations and switching between them
> (save, import, duplicate, set as current). Exporting is in both editions.

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

## Traps

- **An export does not carry the vault.** Take the
  [vault file](/docs/console/vault) too, or the second gateway comes up with
  references pointing at nothing.
- **Replace prunes, Add does not.** Read the plan; it is the difference between a
  merge and a wipe.
- **A configuration is not a backup.** Users and sessions are not in it. That is
  what the Snapshot tab is for.
