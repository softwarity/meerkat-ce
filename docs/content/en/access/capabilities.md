---
title: Administration capabilities
section: Access control
order: 140
summary: The five flags on an account that open the console and the admin API, and exactly what each one unlocks.
---

# Administration capabilities

Roles decide what someone reaches **through** the gateway. Capabilities decide
what they may administer **of** the gateway. They are separate mechanisms: a
capability opens the console and the admin API, and grants nothing on the data
plane.

They are flags on the account, toggled from the row in **Application > Users**.

| Capability | What it administers |
|---|---|
| **root** | the whole gateway. Implies the two below |
| **infra admin** | the routing plane: routes, upstreams, authorities, TLS, the mail relay |
| **app admin** | the application's identity: users, roles, settings, the served pages |
| **dev** | the developer tooling: dev keys, substituting a service |
| **tenant creator** | may create organisations, and owns the ones they create |

*Tenant creator* only appears on installations with more than one organisation:
where there is one and a second cannot be created, the badge would grant nothing.

## What each one opens in the console

**infra admin** - the *Infra* plane:

- Routes, with the whole route editor, the routing tester and upstream health
- Endpoint security and endpoint rate limits
- Authentication: the authorities and their configuration
- Mail relay, TLS and certificates
- The account model: which extra fields an account carries
- Metrics

**app admin** - the *Application* plane:

- General settings and Locales
- Users, and the global role catalogue
- Security: the password policy, the throttle, two-factor, passkeys, tokens, the session lifetime
- Built-in pages: theme, layout, branding - and the navigation portal
- On a single-organisation installation, that organisation's groups, members and group rules

**root only**, on top of both:

- Access tokens for the control plane, and connecting an agent
- The configuration: export, import, restore points, history, backup
- The switch between one and several organisations
- Forcing a password change on every account at once
- Granting or revoking **root**, and editing a root account at all

The last enabled root cannot be demoted or disabled. The gateway keeps one door
open for itself.

## Screens that belong to no plane

Some sections are open to several capabilities, with the **content** scoped
server-side rather than the page being hidden:

| Section | Who may open it | What they see |
|---|---|---|
| Vault | root, infra admin, app admin | the entries of the plane they administer |
| Audit trail | root, infra admin, app admin, an organisation's admin | the events of their own domain |
| Anomalies | the same | the reports of their own domain |
| API docs | root, infra admin, app admin | the specs they may read |
| Metrics | root, infra admin | everything that passed through |
| Organisations | any signed-in console user | the organisations they administer |
| Licence | everybody | the edition, and what it unlocks |

## Administering one organisation is not a capability

There is no *tenant admin* flag. Somebody administers an organisation when they
**own** it, or hold an enabled **ADMIN** membership in it - see
[Organisations](/#/docs/access/tenants). The console computes that server-side and
shows them the organisation sections for the organisations concerned, and nothing
else.

## Navigation is comfort, the API is the contract

The console hides what you may not use - the left navigation is built from the
capabilities the gateway stamps on the page, and a bookmark into a section you
may not open bounces to the first one you may. That is ergonomics.

The rule is enforced again on every call to the admin API, which answers `403`
with the sentence naming what is required - *infrastructure administration
requires root or the infra-admin capability*. A caller who bypasses the console
gains nothing.

## Tokens narrow, they never widen

A control-plane token acts with its owner's capabilities, restricted by its own
perimeter: a token scoped to the routing plane keeps only *infra admin* and
**drops root**, even when root minted it. A domain that left root standing would
confine nothing. See [API tokens](/#/docs/auth/api-tokens).
