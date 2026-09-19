---
title: Routes
section: Concepts
order: 21
summary: A route answers two questions - is this request for me, and what do I do with it - and the order they are tried in decides everything.
---

# Routes

A route is the unit of configuration in Meerkat. It carries everything: which
requests it wants, what to do to them, where they go, who may pass, how long
the upstream may take, how much it may carry. There is no separate service
object to create first - a route with an upstream URL is a complete route.

Routes live in the database and are edited from the console. Saving one applies
it at once.

![The routes screen](img/console/routes-list.webp)

## The four parts

**Predicates** answer *is this request for me*. They are the matching half, and
they are combined with **AND**: a route with a path and a host predicate wants
both.

**Filters** transform. A request filter rewrites what goes upstream, a response
filter rewrites what comes back, a gate refuses a request outright, and a
terminal filter answers instead of proxying.

**The upstream** is where the request goes - `http://`, `https://` or `h2c://`
for HTTP/2 over cleartext, which is what a gRPC service inside a cluster
speaks.

**Access** is the route's rule: who may pass. Empty means the route asks
nothing and the upstream decides for itself.

## What a route can match on

| Predicate | Matches on |
|---|---|
| `path` | the request path, with `*` and `**` patterns |
| `host` | the Host header |
| `method` | the HTTP verb |
| `header`, `cookie`, `query` | a named value, present or equal to something |
| `remote-addr` | the peer address, by CIDR |
| `x-forwarded-remote-addr` | the same, read from the forwarding header |
| `time-window` | a period, for a route that only exists at certain hours |
| `version` | an API version range, read from a header, a query parameter or the path |
| `weight` | a share of the traffic, drawn per request, for a canary release |

> [!NOTE]
> A language predicate, regex exclusion and a trailing-slash option are
> described but not built. The list above is what exists.

## What a route can do to a request

Filters come in phases, and the console groups them the way they run:

- **Gates** refuse before anything else: `max-request-body`, `max-request-headers`.
- **Incoming** rewrites the request: `strip-prefix`, `prefix-path`, `rewrite-path`, `set-path`, `set-host`, `preserve-host`, and the whole family of header, query-parameter and cookie operations - set, add, remove, rename, copy, rewrite.
- **Outgoing** rewrites the response: the same header family, plus `set-status`, `cache-control`, `security-headers`, `cookie-attributes`, `dedupe-response-header`, `rewrite-location`, `remove-json-fields`.
- **Terminal** filters mean the route answers by itself: `redirect`, `maintenance`, `respond`.

A route carrying a terminal filter proxies nothing, so its incoming filters and
its identity forwarding are dropped - the console disables those sections rather
than letting you write settings that are thrown away.

## The order, and why it is the whole story

Routes are tried in **ascending order**, and the **first one that matches
answers**. Ties are broken by name, so the same configuration always compiles
to the same table.

Two things about that order surprise people, and both are deliberate.

**Security is part of matching.** Two routes may cover the same paths and
differ only by who they are for. A caller whose rule turns them away **falls
through to the next matching route**. This is what lets a `/reports/**` route
for the finance roles sit above a `/reports/**` route for everyone else.

**A `deny` rule never falls through.** The one rule written to shut a path shuts
it. Anything else would turn it into a redirect.

If every matching route turned the caller away, the **first** one that did
produces the refusal - with the reason it knows, and the organisation switch it
can offer. A bare 404 there would be the one answer nobody can act on.

If nothing matched at all, the gateway answers 404 and counts it
gateway-wide - a rising number of unmatched requests is a misconfiguration
somebody should see.

## The catch-all

A fresh installation has a route named `trap`: a `/**` path predicate ordered
last. It is an ordinary route with no privilege - it just happens to be tried
after everything else, so it catches whatever was not matched, `/` included.

That is what makes a new gateway answer something on every path. Delete it, or
point it at your own landing page, once you have your own routes.

> [!TIP]
> A new route is created with order `0`, which puts it at the **top** of the
> table - it is tried before everything already there. Drag the rows on the
> Routes screen to change that.

## Two kinds of route

Every route is a service route. **UI** is a flag on top, and it unlocks the
options that only make sense for something a browser renders: the injected user
button, the colour scheme, the identity stamped on the page. A refusal on a UI
route lands on a page; on a service route it stays a 403, because nobody reads
a 403 in a browser.

See [What the gateway injects](/docs/concepts/data-plane-chrome).

## What else a route carries

| | What, and where it falls back to |
|---|---|
| Timeouts | connect and first answer, per route, else the installation, else 5 s / 15 s. The body is never bounded: a download or a websocket that has started runs as long as it needs |
| Circuit breaker | off by default. After N consecutive failures the route stops calling and serves the unavailable page, letting one request through after the cooldown. A 500 does not count - a service answering 500 is up and has a bug |
| Rate limits | several at once, each keyed on something different - the route, a user, a token, an organisation, an address. The first one exceeded answers |
| Identity forwarding | what the upstream is told about the caller - headers, or a signed JWT |
| Locales | the route's language offer, and how the language travels to the upstream |
| OpenAPI spec | published by the service or deposited here; it is what per-endpoint rules are posed on |

## Testing before traffic

The **Routing test** on the Routes screen composes a request that is never
sent - method, path, host, headers, cookies, client address, clock, and an
identity to be judged as - and reports route by route which one takes it. The
answers it gives are the four that matter: takes this request, refused by the
predicates, matched but the rule turned this caller away, or not evaluated
because a route above matched first.
