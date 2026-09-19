---
title: Operations
section: Operations
order: 200
summary: Where to look when something is wrong, what the gateway records, and what a backup actually holds.
---

# Operations

Meerkat sits on the path of every request, so the questions an operator brings to
it are always the same few: is traffic arriving, is it getting through, and who
changed something. Each has one screen, and this page says which.

## Where to look

| The question | Where it is answered |
|---|---|
| Is traffic arriving, and how fast | [The traffic screen](/#/docs/operations/traffic), the **Metrics** entry of the rail |
| Is a service still answering | [Upstream health](/#/docs/operations/upstream-health), and the Routes list marks what fails |
| Why was that call refused with `429` | [Rate limits](/#/docs/operations/rate-limits) |
| Who changed this, and when | [The audit trail](/#/docs/operations/audit) |
| Is this node ready for traffic | [Health probes](/#/docs/operations/health) |
| When does that certificate expire | [TLS and certificates](/#/docs/operations/tls) |
| What is actually in a backup | [Backup and restore](/#/docs/operations/backup-restore) |
| Where does this password live | [The vault](/#/docs/operations/vault) |

## What the gateway records, and for how long

| What | Where it lives | For how long |
|---|---|---|
| Traffic counters | memory, on each node | one hour, lost on restart |
| Endpoint history | memory, on each node | seventy minutes, one point a minute |
| Audit trail | the database | one year, then purged |
| Restore points | the database | kept, never pruned |
| Logs | standard error, structured | whatever collects them |

Nothing about traffic is written to disk, and that is deliberate: an app gateway
is not a time-series database, and an installation that wants a year of curves
scrapes them into the one it already runs
([metrics](/#/docs/operations/metrics)).

Logs are structured but plain: there is no configurable level and no request log
yet (OBS-03).

## A backup and a configuration export are not the same thing

A **snapshot** is the whole database: accounts, sessions, the vault, the audit
trail, the certificates. It is what restores *this* installation.

A **configuration export** is routes, roles, authorities, themes and settings, as
a file a person can read and diff. It reproduces a gateway *elsewhere*. It
carries no account, no certificate and no secret value, so it is not a backup.

## What is per node and what is shared

| Kept by each node | Shared through the database |
|---|---|
| traffic counters (summed for the screen) | the audit trail and the restore points |
| the circuit breaker's verdict on an upstream | routes, certificates, settings, the vault |
| rate-limit counters | the brute-force counter on sign-ins |
| the live channel's own subscribers (messages are relayed) | sessions and API tokens |

> [!NOTE] Enterprise edition
> Running several gateways against one PostgreSQL database is the Enterprise
> image: the driver that opens the connection and waits on notifications lives
> there (STORE-03, PERF-03). The community image tells you what it can do instead
> of failing on a driver.

> [!WARNING]
> The vault's master key is a **file** beside the data directory, and it stays one
> even with an external database. Every node must carry the same key, or one node
> cannot open what another sealed - certificates included. See
> [the vault](/#/docs/operations/vault).
