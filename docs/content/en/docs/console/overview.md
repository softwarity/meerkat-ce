---
title: The console
section: The console
order: 150
summary: What the administration console is for, how it is laid out, and where each screen lives.
---

# The console

The console is Meerkat's administration application. It is served by the gateway
itself, on the **admin port** (the control plane), and it is the same binary: there
is nothing extra to deploy, and nothing to keep in step with the version that
routes your traffic.

It is an operator's tool, served in English.

![The console on the Users screen, with the rail on the left and the Application plane's sections beside it](img/console/users.webp)

On the far left, the rail: the two planes and the transverse screens. Beside it,
the sections of the plane you are in. The rest of the width is the screen itself.

## Two planes, and what sits across them

The left rail holds the two planes and the transverse screens.

| Rail entry | URL | What it answers |
|---|---|---|
| **Infra** | `/infra/...` | How requests travel: routes, endpoints, authorities, TLS, the relay |
| **Application** | `/application/...` | The product your users see: identity, roles, pages, portal, policies |
| **Tenants** | `/tenants/:id/...` | One organisation at a time (several-organisations mode only) |
| **Vault** | `/vault` | Every named value and secret the configuration points at |
| **Data plane** | `/data-plane/...` | What the applications are doing: their sign-ins (Audit), who is signed in to them (Sessions), the calls made to them on a schedule (Scheduler), what the gateway served (Metrics), what your users reported (Issues) |
| **Meerkat** | `/system/...` | The gateway itself: who changed what and who signed in to the console (Audit), what it says about itself, live (Logs), who holds the console (Sessions), the whole installation as a document or a database (Configuration) - and, at the foot, the control plane's REST reference (API reference), what this version brought (Release notes) and the edition (License) |

The split is not cosmetic. **Infra** is about the installation: an upstream, a
certificate, an SMTP server, a directory. **Application** is about the product
that installation serves: who your users are, what they may do, what your
sign-in page looks like. The same person often does both, and holds both
capabilities, but the questions are asked in two places.

> [!NOTE]
> Meerkat lives at `/system`, not `/meerkat`: outside `/api`, the paths of the
> control plane belong to the product (`/meerkat/...` serves the gateway's own
> scripts), so the console keeps clear of them.

## What you see depends on who you are

The console shows what your capabilities allow, and the admin API enforces the
same scopes on every call.

| Capability | Opens |
|---|---|
| `root` | Everything, including Configuration |
| `infra admin` | The Infra plane (with Access tokens and MCP), Metrics, Logs, Audit and Issues, the vault's infra scope |
| `app admin` | The Application plane (with Access tokens), Sessions, Scheduler, Audit and Issues, the vault's application scope |
| `tenant admin` | The organisations they administer, Sessions, Audit and Issues scoped to them |
| `tenant creator` | Creating an organisation from the Tenants drawer |
| `dev` | The developer tooling on the served applications, not a console screen |

Signing in lands you on the first section you may use: Infra on Routes for an
infra admin, Application on General for an app admin, Tenants otherwise (License
when there is a single organisation). Capabilities are granted per account on
[Users](/docs/console/users).

## The habits of the screens

Learn these five and the console stops surprising you.

- **A list, then a drawer on the right.** Clicking a row opens the object; the
  list stays where it was. On Routes, Users, Roles, Authentication, Issues and
  Configuration the drawer is in the URL, so a refresh or a bookmark comes back
  to exactly what was open.
- **Row actions live in the last column** of the table, and appear on the row
  you are pointing at.
- **Saving.** Some screens save on the click (a switch, a checkbox in a matrix),
  others have a Save button and say what is still missing. Where it matters, the
  screen says which it is.
- **Enterprise features are marked.** On the Enterprise image they carry a
  small `EE` badge (tooltip *Enterprise edition feature*). On the community
  image they are locked and dimmed, with an `Enterprise` badge that says what
  they buy and links to License. Nothing is hidden.
- **Secrets go through the vault.** A sensitive field offers to store its value
  in the [vault](/docs/console/vault) and refuses to be saved as a literal.

## The screens, by group

### Infra

- **[Routes](/docs/console/routes)** - the routing table, in order, and the route editor.
- **[Endpoint security and rate limits](/docs/console/endpoints)** - per operation, from a route's OpenAPI spec.
- **[Endpoint audit](/docs/operations/audit#auditing-a-routes-operations)** - which operations are recorded in the audit trail.
- **[Authentication](/docs/console/authentication)** - the authorities people may sign in through.
- **[Mail relay](/docs/console/mail-relay)** - the SMTP server, and the daily digest.
- **[TLS](/docs/console/tls)** - one name, one certificate, and ACME.
- **[OpenTelemetry](/docs/operations/tracing)** - traces, metrics, audit and logs sent to your collector.
- **[Plug](/docs/operations/plug)** - the developer tunnel.
- **[Access tokens, MCP and API](/docs/console/access-and-agents)** - driving Meerkat without a browser.
- **Model** - the fields an account carries, documented with [Users](/docs/console/users).

### Application

- **[General and Security](/docs/console/application)** - what this installation is, and its policies.
- **[Roles](/docs/console/roles)** - the global role catalogue.
- **[Users](/docs/console/users)** - accounts, capabilities, and their fields.
- **[Groups, Members and Group rules](/docs/console/organisation)** - who is in which group.
- **[Built-in pages](/docs/console/built-in-pages)** - theme, layout and branding of the pages the gateway serves.
- **[Portal](/docs/console/portal)** - the navigation bar the proxied applications wear.
- **[Sessions](/docs/auth/sessions#seeing-the-sessions)** - who is signed in, and signing a session out.
- **[Access tokens](/docs/console/access-and-agents)** - console tokens, and everyone's application tokens.

### Across both

- **[Tenants](/docs/console/tenants)** - one organisation's own administration.
- **[Vault](/docs/console/vault)** - secrets and values, referenced as `$name`.
- **[Metrics](/docs/console/traffic)** - traffic, latency, the ranking of routes.
- **[Scheduler](/docs/operations/scheduler)** - the scheduled calls and their runs.
- **[Audit and Issues](/docs/console/audit-and-issues)** - the trail of changes, and the reports.
- **[Logs](/docs/operations/logs#in-the-console)** - the gateway's own lines, live, and its level. One node at a time.
- **[Configuration](/docs/console/configuration)** (under Meerkat, root only) - configurations, restore points, snapshots and migration: the whole installation, which crosses both planes.
- **License** (under Meerkat, and open to everybody) - which edition answered, and every Enterprise feature: what it
  does, how far it is built, and the screen it lives on, or that it has none
  (the active/active cluster, the deployment files). The list is read from the
  product's own feature contract, so it cannot drift from it. It is the only
  screen that talks about editions: everywhere else a locked control carries its
  badge and links here.

![The License screen: the edition, then every Enterprise feature with how far it is built and the screen it lives on](img/console/license.webp)

## Your own account

The bottom of the rail is you. The line with your name opens `/profile` - the
gateway's own profile pages, the same ones your users get: photo, password,
second factor, passkeys, personal API tokens. Sign out is underneath, and it
signs out every console tab at once.

## Version and release notes

![The release notes, open on what the next release brings](img/console/release-notes.webp)

**Meerkat, Release notes** says which version runs, and which edition, in its
title: **Meerkat 1.0.1 EE** (or **CE**). Underneath, the release notes, newest
first, from that version down to its minor release - the same text as the
GitHub release. A development build shows the last release it
carries, with what is coming under **Next release** on top; an empty section
says *Missing information*.

Right under it, **License** opens the edition's screen: every Enterprise
feature, how far it is built, and where it lives. Both are open to everybody
holding the console.
