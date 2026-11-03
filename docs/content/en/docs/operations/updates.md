---
title: Update check
section: Operations
order: 216
summary: Off by default. What the gateway reads, what it sends, and what the console says before you install a newer version.
---

# Update check

A gateway reaches out to nobody on its own. The update check is the one exception,
and it stays off until root turns it on: an installation with no way out has
nothing to do.

## Turning it on

On **Meerkat > Release notes**, root turns on **Check for updates**. The gateway
reads the list at once, then once a day, on one instance only. A restart does not
read it again. Turning it off forgets what it found.

The switch is recorded in the audit trail, and it stays on this gateway: it does
not travel in a configuration export.

## What leaves the gateway

One `GET` of a public file, `https://meerkat.softwarity.dev/releases.json`. The
request carries no version, no identifier and nothing about your installation. It
follows `HTTP_PROXY` and `HTTPS_PROXY` when they are set.

The file is served by GitHub Pages, which sees the address the request comes from,
as it does for any visitor of the site. Softwarity receives nothing.

The address cannot be changed from the console or the API. To keep the check
inside your network, publish a copy of the file and give its address to the
gateway with `MEERKAT_UPDATE_FEED`.

## What the console shows

When a newer version exists, root and gateway administrators see an indicator on
the **Meerkat** entry of the rail and on **Release notes**, on every open console,
with no reload. Other accounts see nothing: they cannot act on an update.

**Release notes** then opens on the newer versions, marked **Not installed**, each
with what installing it would do:

| The console says | What it means |
|---|---|
| Built after the updates of your license ended | Installed, this version runs as an evaluation: the notice shows on the pages your users see and the license ceilings stop applying. Traffic is not touched, and renewing the license keeps it in force. |
| This version migrates the database | An older version no longer opens the database afterwards. Take a [backup](/docs/operations/backup-restore) before you update. |

When a version says both, it does not migrate on its own: see below.

Nothing is ever installed from here: the console has no install button.

## A version past the license does not migrate on its own

Running a version built after the updates of your license ended can be undone:
go back to the one before, and the license is in force again. Migrating the
database cannot: the version before no longer opens it, and the way back is a
backup, if there is one.

So a version that is outside the license, and would migrate the database, stops
before it does, writes nothing, and says why in its log:

```
this version was built on 2028-01-10, after the updates of the license of ACME (...) ended on 2027-12-31,
and it has to migrate the database (schema v102 to v105). [...] Run a version built by 2027-12-31,
or renew the license and start this one again. To migrate anyway and run as an evaluation,
take a backup first, then set MEERKAT_MIGRATE_PAST_LICENSE=true
```

The version you ran before starts again on the same database, as it was. On
Kubernetes with several instances, and on Swarm with the stack we publish, the
instances already running keep serving while the new one refuses. On a single
Kubernetes instance, on Swarm without `order: start-first`, and on Compose, the
gateway is down until you put the previous image back.

What ends it:

- **Renewing the license.** The new license covers the version, and it
  migrates on its next start. A license mounted from a Secret is read at
  start; a pasted one goes in from the console of the version still running.
- **Going ahead anyway.** Take a [backup](/docs/operations/backup-restore), then
  start with `MEERKAT_MIGRATE_PAST_LICENSE=true`. The database is migrated, the
  version runs as an evaluation, and the license stays as it is: renewing later
  puts it back in force. The audit trail records it (`license.migrate`).

The check is made only when there is something to lose: a license in force and
a migration waiting. No license, an ended test license, a fresh database or a
version that changes nothing in the database start as they always do.

## When the check fails

A gateway that cannot reach the list says so on **Release notes**, with the time
and the reason, and writes one informative line to its log. There is no alert, and
it tries again the next day. What an earlier check found stays shown.
