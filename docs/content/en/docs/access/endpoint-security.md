---
title: Per-endpoint security
section: Access control
order: 136
summary: Rules per operation of an API route, written on the inventory its OpenAPI spec provides.
---

# Per-endpoint security

A route's rule covers the whole route. When one API route exposes forty
operations and three of them must be closed, the rule to write is per operation.

**Infra > Endpoint security.** Pick a route that exposes an OpenAPI spec: Meerkat
fetches and parses the spec on the server, lists its operations - method, path,
summary, tags - and lets you pose a rule on any of them. Swagger 2.0 and OpenAPI
3.x are both understood, in JSON or in YAML.

The spec is either **published by your service**, in which case it is a live
reference re-read whenever the screen is opened, or a **file deposited here**, in
which case it is a snapshot that changes only when you deposit another. The
screen says which.

## The two situations it exists for

**You cannot change the application.** A bought product, a service another team
owns, a binary whose sources you do not have. Its authorisation is whatever its
author decided, and you have no way to add a rule inside it. The rule goes in
front of it instead: Meerkat refuses the call before the service is ever
reached, so the application does not need to know it is protected.

**You found a hole in production.** An operation that should never have been
open - a reindex, a purge, an export - answers anyone who knows its path,
because the backend never checked. You do not deploy. You open this screen,
pose the rule, and it holds on the next request.

![The operations of a route, each with the rule that governs it](img/console/endpoint-security.webp)

The list comes straight from the route's OpenAPI description: method, path,
summary and tags, exactly as the service declares them. The badge on the left
is the rule in force, and the icons next to it say whether roles, named users
or rate limits are attached.

Here, two operations carry a rule of their own. `DELETE /orders/{id}` is
restricted to a role the backend knows nothing about - that is roles being
prototyped in front of a service rather than wired into it. And
`POST /admin/reindex` is closed to everybody, with one exception.

![Closing one operation, with a named exception](img/console/endpoint-rule.webp)

That is the production hole, shut: **Nobody** may call it, refused before the
service is called, except the one operator who still has to run it. No
deployment, no ticket to the team that owns the backend, no change in the
application at all.

## What a rule is

The same rule as a route's - level, organisations, roles, named users - plus a
method and a path:

| Part | Values |
|---|---|
| Method | `GET`, `PUT`, `POST`, `DELETE`, `OPTIONS`, `HEAD`, `PATCH`, `TRACE`, or `*` for any |
| Path | the path **as the spec writes it**: exact segments, `{id}` for one segment, `**` as a last segment for everything below |
| The rule | see [A route's access rule](/docs/access/route-access) |

A rule may also carry rate limits of its own, and those are evaluated **before**
the access rule - a flood is refused without a session being looked up.

> [!NOTE]
> The path is the one in the spec, not the one that arrives at the gateway. If
> the route strips a prefix, Meerkat puts it back before matching, so
> `/orders/{id}` in the document is what you write, whatever the public path is.

## Precedence: order, not specificity

**The first rule in the list that matches wins.** There is no most-specific-wins
ranking. A rule for `*` on `/**` placed first would swallow everything under it,
so put the general rules last.

An operation matched by **no** rule falls back to the **route's own rule**.

> [!WARNING]
> There is **no deny-by-default switch**. An operation you forgot is not closed:
> it is governed by the route's rule, which may be delegated. Leaving an
> incomplete list of endpoint rules on a delegated route protects nothing.

If you want an inventory where only what you listed is reachable, write the
refusal yourself, as the **last** rule: method `*`, path `/**`, level *Nobody*.
Everything not matched earlier lands on it. That is today's answer to
deny-by-default, and FEATURES.md still lists the switch as missing.

## Rules can open as well as close

An endpoint rule replaces the route's rule for that operation, in both
directions. An override with nothing set is *delegated*, so it **reopens** an
operation on a route that requires a session - which is how a public health check
lives on an otherwise closed API.

Because of that, a route carrying endpoint rules does not apply its own rule
while routes are being chosen: it would refuse a caller an override was about to
let in. The route's rule is not lost - it is the fallback inside, for every
operation no rule matched.

## The spec itself is not hidden

When the spec is a file you deposited, the gateway serves it on the route, and it
is served **outside** the endpoint rules: it carries the route's own rule
instead. Per-operation rules decorate a contract, they do not conceal it - and a
`curl`, Postman or a client generator all read it from the same URL.
