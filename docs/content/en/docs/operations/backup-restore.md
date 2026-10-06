---
title: Backup and restore
section: Operations
order: 215
summary: What a snapshot holds, why there is no restore button, and why a configuration export is not a backup.
---

# Backup and restore

Three different things get confused here, so they live on three tabs of the
**Meerkat > Configuration** screen and they answer three different questions.

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

## Moving the database: embedded and PostgreSQL

The same tab moves the database. A copy of the **same kind** as the one the
gateway runs on is a backup; of the **other kind**, it moves the gateway:

| From | To | What you get |
|---|---|---|
| embedded | embedded | `meerkat.db`, the snapshot above |
| embedded | PostgreSQL | `meerkat.sql`, a dump in `pg_dump`'s plain format, or a copy straight into a server |
| PostgreSQL | PostgreSQL | `meerkat.sql`, or a copy into another server |
| PostgreSQL | embedded | `meerkat.db`, built from the database |

PostgreSQL is the Enterprise image's: the target is locked on the community one.

**Pause first** - the screen insists: a copy to the other kind, file or server,
waits for it, and so does the API. A backup of the same kind does not: it is
made to be taken hot. The **Pause** switch, at the top of the tab, puts every
application on the maintenance page and stops every write, on every node. A copy
taken while the gateway writes would lose what is written before the switch -
sessions, the audit trail, the audited calls. The pause is never stored: a
restart ends it, and the gateway that starts on the new database works at once.
While it lasts, the console stays open for reading, and refuses changes.

**Into a server**, the tab takes the target one field at a time - host, port,
database, user, password, SSL mode - for an empty database. In Kubernetes it
offers the PostgreSQL services it finds beside the gateway, the operator's
primary first (CrunchyData's `-primary`, CloudNativePG's `-rw`); a pooler such
as pgbouncer and a replica are listed and refused, since the cluster needs
`LISTEN/NOTIFY` and advisory locks, which a transaction pooler breaks and a
replica cannot take. **Test** asks the server before anything moves, and writes
nothing there: its version, the SSL mode that connects (tried encrypted first,
and kept for the copy), whether the database is empty, and whether the user may
create tables - which PostgreSQL 15 and later no longer grant on `public` by
default. A target that fails one of these is not offered the copy. It copies
every table in one transaction, counting each on both sides before it
commits. A target that already holds accounts or routes is refused: a copy
never lands over a gateway. The URL is used for the copy and kept nowhere; the
trail records its host, in the new database, which starts its history by saying
where it came from.

**Into a file**, the dump loads with `psql -v ON_ERROR_STOP=1 -f meerkat.sql`
into an empty database. It is one transaction: a dump cut short is refused whole.

**The switch is the deployment's.** The gateway cannot change its own
`MEERKAT_DATABASE_URL`, so once the copy is done the tab prints what to change,
filled with the target it was given: a Secret with the
database URL and the vault key, then `helm upgrade` with
`database.existingSecret`, `vault.existingSecret` and the number of replicas - or
the same two variables for Compose and Swarm.

> [!WARNING]
> The vault key never travels in the copy. Without the same key, every secret of
> the vault is unreadable on the new database: carry it into the Secret, as the
> procedure says.

The same moves, for a script or a Kubernetes Job run before an upgrade:

```bash
meerkat db dump -format postgres -out meerkat.sql    # a dump, for psql
meerkat db dump -format sqlite -out meerkat.db       # a database file
meerkat db copy -to postgres://meerkat:PASSWORD@host:5432/meerkat
```

They read the source from `-data` and `-database-url`, like the gateway. Stop the
gateway, or pause it from the console, first.

## The configuration export

One document, in YAML: routes, the role catalogue, organisations and their groups,
the authorities people sign in through, the mail relay, the themes and the gateway
settings.

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

The trade-off is deliberate: a document alone does not start an environment. Its
references have to exist in [the vault](/docs/operations/vault), which is a
separate file with a separate life.

Two forms differ by one thing: a plain YAML never carries a picture, and says what
it left behind; a `.zip` package carries the logos and the deposited OpenAPI specs,
for when the pictures have to travel. An import carrying no image leaves the ones in
place alone.

## Restore points: the tape

The gateway keeps its own tape. A point is written whenever a change moves
the configuration's **fingerprint** - not whenever an endpoint is called, which is
what makes it affordable and complete at the same time:

- an endpoint added later is covered without anyone remembering to;
- a save that changes nothing writes nothing;
- what is not configuration - creating an account, filling the vault, opening a
 session - leaves no trace here at all.

A point has a time, an author and the sentence the audit trail wrote at the same
second. Nobody asks for one and nobody names one: that is the whole difference with
a saved configuration.

A run of changes by the **same** person inside two minutes folds into one point:
dragging a colour picker writes twenty times for one decision. Landing on a state
the tape already knows is never folded, because that is what going back *is*.

The tape is **not** bounded, and it is the same in both editions. Going back is a
safety function: shortening the free image's memory would not sell the paid one, it
would only make the product riskier where it is used most.

## Named configurations

Several configurations coexist and exactly one is active. The
console diffs them - objects added, removed, changed - and shows that same diff when
a file is imported, before anything is written. An agent can file the current state
under a name in one call, which is what a careful admin asks for in words before a
large change.
