---
title: Backup and restore
section: Operations
order: 215
summary: What a snapshot holds, why there is no restore button, and why a configuration export is not a backup.
---

# Backup and restore

Three different things get confused here, so they live on three tabs of the
**Infra > Configuration** screen and they answer three different questions.

| What | What it holds | What it is for |
|---|---|---|
| A snapshot | the whole database | restoring *this* installation |
| A configuration export | routes, roles, authorities, themes, settings | reproducing a gateway *elsewhere* |
| A vault file | the secret values themselves, encrypted with a passphrase | bootstrapping or moving an environment |

![The configuration screen](img/console/configuration.webp)

## The snapshot

The one piece of backup only the gateway can do is a **coherent** copy taken while
it runs. Copying a live database file with `cp` can catch it mid-write or out of
step with its write-ahead log, and nothing says so: the copy looks fine and fails
the day it is restored, which is the worst possible day.

So the gateway writes the copy itself, refuses to overwrite an existing file,
finishes it **before** answering - a failure is then an error an admin reads, not a
truncated download discovered months later - and names it with its date.

Everything else about backups is deliberately absent: scheduling, retention,
rotation, encryption of the archives, shipping them off the host, alerting when one
fails. Mature tools do all of that, and half-rebuilding it inside an app gateway
would serve nobody.

A snapshot holds accounts, sessions, the vault, the audit trail, the restore points
and the certificates. It does **not** hold the vault's master key.

> [!WARNING]
> The hot snapshot is a copy of the **embedded** database. With an external
> PostgreSQL, back the database up with its own tools (`pg_dump`, PITR) - that is
> the same shape of decision as the cluster itself: the external database brings its
> own backup story.

## There is no restore button, and that is not an omission

A database cannot be replaced underneath the process holding it open. Worse, the
sessions and the accounts a restore brings back are the very ones the request doing
it depends on - and accepting an arbitrary database as trusted state would turn one
borrowed admin session into permanent control of the gateway.

Restoring therefore happens with the service stopped, and the console prints the
exact commands with **this** installation's paths, so the procedure is a paste and
not a puzzle:

```bash
# 1. stop meerkat
# 2. keep the current database aside
mv /data/meerkat.db /data/meerkat.db.before-restore
# 3. put the snapshot in its place
cp meerkat-YYYY-MM-DD.db /data/meerkat.db
# 4. start meerkat, then check a route and a sign-in
```

The old file is kept: a restore one regrets must have a way back.

> [!WARNING]
> The master key sits next to the database unless it comes from
> `MEERKAT_VAULT_KEY`. Backing the two up together undoes the encryption at rest,
> exactly as storing a vault file beside its passphrase would. Keep the key in a
> secret manager, or supply it through the environment - the console says which of
> the two your installation does.

## The configuration export

One document, in YAML: routes, the role catalogue, organisations and their groups,
the authorities people sign in through, the mail relay, the themes and the gateway
settings (CONSOLE-05, CFG-05).

What it does **not** carry is as much of the design as what it does:

- **no account**, no membership, no session - they carry credentials, second-factor
  secrets and passkeys;
- **no certificate**, no signing key, no simulation key - they are generated where
  they are used;
- **no secret value.** A declared secret field travels as its `$name` reference or
  not at all.

So an export is **public by construction**. It goes into a ticket, a mail to
support or a git repository without anyone having to wonder what is inside - and the
day an export may hold a secret, nobody dares share one and the feature dies of its
own caution.

The counterpart is assumed: a document alone does not start an environment. Its
references have to exist in [the vault](/docs/operations/vault), which is a
separate file with a separate life.

Two forms differ by one thing: a plain YAML never carries a picture, and says what
it left behind; a `.zip` package carries the logos and the deposited OpenAPI specs,
for when the pictures have to travel. An import carrying no image leaves the ones in
place alone.

## Restore points: the tape

The gateway keeps its own tape (CFG-06). A point is written whenever a change moves
the configuration's **fingerprint** - not whenever an endpoint is called, which is
what makes it affordable and complete at the same time:

- an endpoint added later is covered without anyone remembering to;
- a save that changes nothing writes nothing;
- what is not configuration - creating an account, filling the vault, opening a
  session - leaves no trace here at all.

A point has an hour, an author and the sentence the audit trail wrote at the same
second. Nobody asks for one and nobody names one: that is the whole difference with
a saved configuration.

A run of changes by the **same** person inside two minutes folds into one point:
dragging a colour picker writes twenty times for one decision. Landing on a state
the tape already knows is never folded, because that is what going back *is*.

The tape is **not** bounded, and it is the same in both editions. Going back is a
safety function: shortening the free image's memory would not sell the paid one, it
would only make the product riskier where it is used most.

## Named configurations

Several configurations coexist and exactly one is active (CFG-01, CFG-02). The
console diffs them - objects added, removed, changed - and shows that same diff when
a file is imported, before anything is written. An agent can file the current state
under a name in one call, which is what a careful admin asks for in words before a
large change.
