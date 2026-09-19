---
title: Endpoint security and rate limits
section: The console
order: 154
summary: Setting who may call each operation of an API, and how much it may carry, from the route's OpenAPI spec.
---

# Endpoint security and rate limits

**Infra > Endpoint security** and **Infra > Endpoint rate limits** are two
entries onto one inventory: the operations the gateway reads from a route's
OpenAPI spec. Security asks *who may call this*, rate limits ask *how much*. They
are two menu entries because those are two questions people arrive with, and a
menu naming neither is a menu where neither is found.

## Before you start

The screen only lists routes that expose an OpenAPI spec. If none does, it says
so and points you at Routes: declare the spec on the route's **Target** section,
either published by the service or deposited as a file, then come back.

## What you do here

1. **Pick the route** in the selector at the top. Its title, format and version
   appear beside it. Arriving from a route's editor preselects it.
2. **Find the operation.** The table lists method, path, tags and description.
   The method and tag column headers are filters; the path column sorts. The
   footer counts what is covered: *N operations, M secured* (or *M bounded*).
3. **Click the row.** The operation opens in a drawer, with its operation id and
   the section this page is about.
4. **Write the rule.** There is no Save button: the footer says *Saving...* then
   *All changes saved*.

## On the security page

The drawer carries one switch, **Override the route config**.

- Left alone, the operation says **Inherits the route config**, and the sentence
  links to the route's own Security section. An operation you never touch keeps
  being secured by the backend itself.
- Turned on, you get the same access editor the route uses. Two axes, both of
  which must be satisfied: the **belonging level**, and a **role filter**
  evaluated in the caller's active organisation.

| Level | What it requires |
|---|---|
| **Delegated** | Nothing. Everyone through, signed in or not; the service decides |
| **Signed in** | Any account, including one that belongs to no organisation yet |
| **In an organisation** | An organisation must be active on the session |
| **In one of these organisations** | The active organisation must be one of those named |
| **Nobody** | Refused before the service is called |

![The same access editor, on a route's Security section: a level, a list of roles, and an exception for named users](img/console/route-editor-security.webp)

The same editor, here on the *Billing* route: the level, the roles (any one
grants access, held in the active organisation), and the exception for named
users. An operation's drawer shows exactly this once the override is on.

**Named users are an exception, not a level**: whoever is listed passes whatever
the level requires. That is how a service account or a support login gets
through a rule written for everyone else.

> [!NOTE]
> Whatever you choose, the service still applies its own rules. This screen adds
> conditions, it never removes any - which is why the open end is called
> *delegated* and not *public*.

## On the rate limits page

The drawer holds the operation's own bounds, **on top of** the route's, which
still apply. A bound is a counter, and you choose what it is keyed on:

| Key | One budget per |
|---|---|
| **The whole operation** | everything it carries, whoever is calling |
| **Each user** | signed-in account (anonymous callers are not covered) |
| **Each API token** | token, so an integration is bounded without bounding its owner |
| **Each organisation** | organisation - a quota sold to a customer |
| **Each address** | client address, the only key an anonymous caller has |

The second half of a rule says **to whom** it applies, and it is the same access
shape as above: a role becomes a pricing tier without a new concept. Several
bounds are true at once, and the first one exceeded answers 429.

Write these for the few operations that need one. A bound on every operation of
a large inventory is a counter per operation and per caller.

## Traps

- **A spec published by the service follows the service.** When the service
  renames a path, the rule written against the old one has nothing left to
  attach to. A deposited file is a snapshot and does not move under you.
- **Untouched means unguarded by Meerkat**, not unguarded: the backend is still
  in charge. Deciding to centralise access control means overriding
  deliberately, operation by operation.
- **The route's own rule is not on this screen.** It takes part in choosing the
  route at all, so it lives with the route; the *Inherits* sentence is the way
  there.
- **An address key is the attacker's choice of budget.** It is read from the
  connection, never from a forwarded header, but it still means one budget per
  address.

See also [Routes](/#/docs/console/routes) for the route-wide rule, and
[Roles](/#/docs/console/roles) for the catalogue these rules name.
