---
title: Tenant administrator
section: Access control
order: 143
summary: What administering a tenant opens in the console - that tenant's groups, members and rules, its audit and sessions - and nothing of the others.
---

# Tenant administrator

Administering a tenant is not a capability on the account: somebody administers
a tenant when they **own** it, or hold an enabled **ADMIN** membership in it -
see [Tenants](/docs/access/tenants). The console works it out from that, and
shows the tenants concerned and nothing else.

This is the Enterprise edition's: the community image serves a single tenant,
whose groups and members the [application administrator](/docs/access/app-admin)
manages.

![The console of a tenant administrator: Tenants, Data plane and Meerkat in the rail, the tenant's menu open on Members](img/console/capability-tenant-admin.webp)

## What you see

The console opens on your tenant. The rail holds three entries: **Tenants**,
**Data plane** and **Meerkat**.

| Section | Screens |
|---|---|
| Tenants | for each tenant you administer: General, Groups, Members, Group rules, Notifications, Danger zone |
| Data plane | Audit, Sessions, Issues - of your tenants |
| Meerkat | Audit, Release notes, License |

## What you do there

- **[General](/docs/console/tenants)**: the tenant's name, its working hours, the
  session lifetime its members get.
- **[Groups, Members and Group rules](/docs/console/organisation)**: who belongs to
  the tenant, the groups that carry their roles, and the rules that fill those
  groups from a directory.
- **[Notifications](/docs/console/notifications)**: whether the tenant's people
  receive them, and what was sent to them.
- **Danger zone**: the acts that cannot be taken back.
- **[Audit and Issues](/docs/console/audit-and-issues)** and **Sessions**: who
  signed in to your tenant, what changed in it, what its people reported, who
  is signed in now.

## What is not yours

| You do not see | It belongs to |
|---|---|
| Another tenant, even by its address | its own administrators |
| The roles a group may hold: the global catalogue | the [application administrator](/docs/access/app-admin) |
| Users as accounts, the served pages, the security policies | the application administrator |
| Routes, authorities, TLS, metrics, logs | the [infrastructure administrator](/docs/access/infra-admin) |
| The Vault | root and the two administrators above |

Another tenant's screens send you back to yours, even typed into the address
bar, and the admin API answers `403` on its calls.

## An account that administers nothing

Signed in to the console with no capability and no tenant to administer, an
account sees only **Meerkat**: the release notes and the license.

![The console of an account that administers nothing: Meerkat alone in the rail, the License screen](img/console/capability-none.webp)
