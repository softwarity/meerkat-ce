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
| **Infra** | `/infra/...` | How requests travel: routes, endpoints, authorities, TLS, the relay, the configuration |
| **Application** | `/application/...` | The product your users see: identity, roles, pages, portal, policies |
| **Tenants** | `/tenants/:id/...` | One organisation at a time (several-organisations mode only) |
| **API** | `/api` | The control plane's own REST reference, tried with your session |
| **Vault** | `/vault` | Every named value and secret the configuration points at |
| **Metrics** | `/traffic` | What the gateway has actually served |
| **Audit** | `/audit` | Who changed what |
| **Issues** | `/issues` | What your users reported |

The split is not cosmetic. **Infra** is about the installation: an upstream, a
certificate, an SMTP server, a directory. **Application** is about the product
that installation serves: who your users are, what they may do, what your
sign-in page looks like. The same person often does both, and holds both
capabilities, but the questions are asked in two places.

> [!NOTE]
> Metrics lives at `/traffic`, not `/metrics`: that path belongs to the
> Prometheus exposition on the same port, so the console could not take it.

## What you see depends on who you are

The console shows what your capabilities allow, and the admin API enforces the
same scopes on every call.

| Capability | Opens |
|---|---|
| `root` | Everything, including Access tokens, MCP and Configuration |
| `infra admin` | The Infra plane, Metrics, the vault's infra scope |
| `app admin` | The Application plane, the vault's application scope |
| `tenant admin` | The organisations they administer, Audit and Issues scoped to them |
| `tenant creator` | Creating an organisation from the Tenants drawer |
| `dev` | The developer tooling on the served applications, not a console screen |

Signing in lands you on the first section you may use: Infra on Routes for an
infra admin, Application on General for an app admin, Tenants otherwise.
Capabilities are granted per account on [Users](/docs/console/users).

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
- **Enterprise controls stay visible.** A control this image cannot honour is
  dimmed and carries an `[Enterprise]` cap that explains what it buys, linking
  to the License screen. Nothing is hidden.
- **Secrets go through the vault.** A sensitive field offers to store its value
  in the [vault](/docs/console/vault) and refuses to be saved as a literal.

## The screens, by group

### Infra

- **[Routes](/docs/console/routes)** - the routing table, in order, and the route editor.
- **[Endpoint security and rate limits](/docs/console/endpoints)** - per operation, from a route's OpenAPI spec.
- **[Authentication](/docs/console/authentication)** - the authorities people may sign in through.
- **[Mail relay](/docs/console/mail-relay)** - the SMTP server, and the daily digest.
- **[TLS](/docs/console/tls)** - one name, one certificate, and ACME.
- **[Access tokens, MCP and API](/docs/console/access-and-agents)** - driving Meerkat without a browser.
- **[Configuration](/docs/console/configuration)** - configurations, restore points, snapshots.
- **Model** - the fields an account carries, documented with [Users](/docs/console/users).

### Application

- **[General, Locales and Security](/docs/console/application)** - what this installation is, and its policies.
- **[Roles](/docs/console/roles)** - the global role catalogue.
- **[Users](/docs/console/users)** - accounts, capabilities, and their fields.
- **[Groups, Members and Group rules](/docs/console/organisation)** - who is in which group.
- **[Built-in pages](/docs/console/built-in-pages)** - theme, layout and branding of the pages the gateway serves.
- **[Portal](/docs/console/portal)** - the navigation bar the proxied applications wear.

### Across both

- **[Tenants](/docs/console/tenants)** - one organisation's own administration.
- **[Vault](/docs/console/vault)** - secrets and values, referenced as `$name`.
- **[Metrics](/docs/console/traffic)** - traffic, latency, the ranking of routes.
- **[Audit and Issues](/docs/console/audit-and-issues)** - the trail of changes, and the reports.
- **License** - which edition answered, and what each Enterprise feature buys. It
  is the only screen that talks about editions: everywhere else a locked control
  carries its cap and links here.

## Your own account

The bottom of the rail is you. The line with your name opens `/profile` - the
gateway's own profile pages, the same ones your users get: photo, password,
second factor, passkeys, personal API tokens. Sign out is underneath, and it
signs out every console tab at once.
