---
title: Application administrator
section: Administration
order: 142
summary: What the app admin capability opens in the console - the people, their roles, the served pages - and what stays out of reach.
---

# Application administrator

An account carrying the **app admin** capability runs the **application's
identity**: who the accounts are, the roles they can hold, the policies they
sign in under, and how the gateway's own pages look. It does not decide where
requests go - that is the [infrastructure administrator](/docs/access/infra-admin).

The capability is a flag on the account, set from **Application > Users** by
another application administrator or root. See
[Administration capabilities](/docs/access/capabilities).

![The console of an application administrator: Application, Vault, Data plane and Meerkat in the rail, the Application menu open on Users](img/console/capability-app-admin.webp)

## What you see

The console opens on **Application > General**. The rail holds four entries:
**Application**, **Vault**, **Data plane** and **Meerkat**.

| Section | Screens |
|---|---|
| Application | General, Roles, Users, Built-in pages, Portal, Security, Access tokens - and Plug while the developer tunnel is open |
| Vault | the entries of the application |
| Data plane | Audit, Sessions, Scheduler, Notifications, Issues |
| Meerkat | Audit, API reference, Release notes, License |

On an installation with a single tenant, its **Groups**, **Members** and
**Group rules** are in the Application menu too: there is one tenant, and nobody
has to choose it. With several, they are each tenant's, under **Tenants**.

## What you do there

- **[General and Security](/docs/console/application)**: what the installation is,
  the languages it speaks, the password policy, two-factor, passkeys, the
  session lifetime.
- **[Users](/docs/console/users)**: the accounts and the capabilities they carry -
  except root, which only root grants.
- **[Roles](/docs/console/roles)**: the global role catalogue routes ask for.
- **[Built-in pages](/docs/console/built-in-pages)** and **[Portal](/docs/console/portal)**:
  the look of the sign-in and account pages, and the bar that moves people
  between applications.
- **[Access tokens](/docs/console/access-and-agents)**: a token for a script,
  within your own capability.
- **[Vault](/docs/console/vault)**: the values and secrets the application refers to.
- **[Audit and Issues](/docs/console/audit-and-issues)**: who signed in, who changed
  what in the application, what users reported. **Sessions**: who is signed in,
  and ending a session. **[Scheduler](/docs/operations/scheduler)** and
  **[Notifications](/docs/console/notifications)**: the calls services schedule,
  and what they told their users.

## What is not yours

| You do not see | It belongs to |
|---|---|
| Routes, authorities, TLS, the mail relay, the model, metrics, logs | the [infrastructure administrator](/docs/access/infra-admin) |
| A tenant's own screens, on an installation with several | the [tenant's administrators](/docs/access/tenant-admin) |
| Configuration (export, import, backup) and the console's own sessions | root |

A screen you may not open sends you back to General, even typed into the
address bar, and the admin API answers `403` on its calls.
