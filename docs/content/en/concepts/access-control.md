---
title: Access control
section: Concepts
order: 23
summary: Roles, groups and organisations, and how a route's rule reads them to decide whether a request goes through.
---

# Access control

Knowing who is calling is one question; whether they may pass is another. Meerkat
answers the second one with four pieces: a role catalogue, groups that hand
roles out, organisations that scope the groups, and a rule on each route that
reads the result.

## Roles

A role is a name in a **single, gateway-wide catalogue**. Roles form a
hierarchy: a parent role implies its children, so granting `finance-manager`
grants everything under it without listing any of it.

Roles carry tags, which are for classification rather than for permission.
Roles marked as system roles cannot be deleted; deleting an ordinary role
re-parents its children rather than orphaning them.

The catalogue is global on purpose. What differs between organisations is who
holds what - not what the words mean.

## Groups

A group belongs to an **organisation** and bundles role IDs from the catalogue.
People are assigned groups per organisation, which is what lets the same account
be an administrator in one and a reader in another.

Each organisation chooses how its groups add up:

| Mode | Meaning |
|---|---|
| `MULTIPLE` | every group assigned to the member counts, and their roles are cumulated |
| `SINGLE` | one group at a time, chosen at sign-in; the others do not apply until the person signs in again |

`MULTIPLE` is the default. In `SINGLE` mode, a session with no group chosen has
no roles at all.

## Organisations

An organisation - a tenant - groups people and the groups they are in. A
membership is typed `ADMIN` or `USER`. Ownership is a separate field on the
organisation, always set, transferable, and independent of membership: an owner
need not be a member.

A member carries their own overrides on top of the organisation: whether they are
enabled, their access window, their session lifetime.

> [!NOTE]
> Enterprise edition, for more than one organisation. A community installation
> serves exactly one, which the console never names - the screens simply act on
> it.

## The inheritance chain

Two settings resolve **member, then organisation, then installation**, most
specific wins:

- the session lifetime;
- the business access window (days, hours, time zone).

> [!WARNING]
> That chain is exactly two settings long. The second factor is **not** in
> it - it resolves from the account, then the installation, with no
> organisation level, because the MFA step runs before an organisation has been
> chosen. A per-organisation rule would have nothing to read yet.

A third, separate chain exists per authentication authority, for MFA, passkeys
and account auto-creation.

Business access hours are an Enterprise capability, and partially built: there
is no re-check during a session, and the per-member window has no editor in the
console.

## Account capabilities

Some rights are not roles: they are flags on the account, because they are about
administering the gateway rather than about using an application.

| Flag | What it opens |
|---|---|
| `root` | global administration; implies both below, and is the only flag that can mint control-plane tokens |
| `infraAdmin` | the routing side: routes, TLS, authorities, the built-in pages |
| `appAdmin` | the application side: accounts, roles, the global settings |
| `dev` | the developer tooling, where developer mode is on |
| `tenantCreator` | may create organisations |

Administering an organisation is deliberately **not** a flag: it is being its
`ADMIN` member, or its owner.

## The rule on a route

Each route carries one rule, and it is read on **two axes combined with AND**.

The first axis is what belonging is required:

| Level | Passes |
|---|---|
| *(empty)* | everyone - the route asks nothing and the upstream decides |
| `auth` | any signed-in account |
| `tenant` | a signed-in account with an active organisation |
| `tenants` | ...and that organisation is one of the named ones |
| `deny` | nobody |

The second axis is the roles the caller holds **in their active organisation**.

So `roles: [admin]` on its own means an administrator of any organisation, while
`tenants: [acme]` plus `roles: [admin]` means an administrator of Acme. A list of
named accounts can be added, and it is read first: a named person passes
whatever the level asks, `deny` included.

## What a refusal looks like

The gateway answers in the order of what the person can actually do about it:

| Situation | Answer |
|---|---|
| No session, and this is a navigation | the sign-in page, with where they were going |
| No session, and this is an API call | 401 with `WWW-Authenticate: Session` |
| A sign-in step still owed | that step |
| Switching organisation would help | the organisation chooser, saying why |
| No membership at all | the waiting room |
| Anything else, on a UI route | `/refused`, which names the rule that turned them away and offers what this session can open |
| Anything else, on a service route | a plain 403 - nobody reads a 403 in a browser |

Every refusal of a signed-in caller writes one warning line carrying the same
reason code the page shows, so what a person reports and what the log says are
the same thing.

## Per-endpoint rules

A route that exposes an OpenAPI spec can carry rules per **operation** - method
plus path, with `{var}` templates and `*` for any method - on the endpoint
inventory the gateway read from that spec. The first matching override wins.

> [!NOTE]
> An operation with no override falls back to the **route's** rule. There is no
> deny-by-default option yet, so a spec that grew an operation nobody wrote a
> rule for is covered by the route, not refused.

## What is not built

- **Impersonation** - signing in as somebody else to reproduce what they see. Nothing exists: no banner, no double identity in the audit trail, no endpoint.
- **A global RBAC off switch** - a mode where any signed-in account passes. The equivalent is written route by route, which is a convenience rather than a mechanism.
