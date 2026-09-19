---
title: Architecture
section: Concepts
order: 20
summary: One binary listening on two planes - the applications on one port, their administration on another - and why that line is not negotiable.
---

# Architecture

Meerkat is one process. It listens on **two planes**, and they are two
different jobs served by two different ports.

| | Data plane | Control plane |
|---|---|---|
| Default port | `:8080` | `:9090` |
| HTTPS port | `:8443` | `:9443` |
| Serves | your applications, and the pages the gateway serves itself | the admin API and the console |
| Belongs on | the network your users reach | an internal network, never the public one |

## Where it sits

One binary in front of the services your application is made of. What reaches
it is a browser or an API client; what it forwards is a request that has
already been decided on.

::: figure mesh
The same gateway in front of both halves of an application: the interfaces
your users open, and the APIs everything else calls.
:::

The line down the middle of the services is the one that matters. A **UI
route** is a page a person looks at, so the gateway dresses it: the navigation
portal, the account button, the theme, the light and dark scheme, all injected
into the HTML on the way out. The application ships no library for any of it
and does not know it is being dressed.

An **API route** is called by a program, so there is nothing to dress. What it
gets instead is a signed token in a header - who is calling, their roles, their
organisation - and it authenticates nobody. That is the whole point: your
services stop carrying a login page and a user table.

A service can be both, and often is: the same application serving its pages and
its own API behind two routes.

## Why they are separate

Because a mistake on one must not be a mistake on the other.

- The console is **never** reachable on the data plane. No route can expose it, because it is not mounted there at all.
- Each plane has its own session cookie - `MEERKAT_SESSION` and `MEERKAT_ADMIN_SESSION` - and every stored session is stamped with the plane it belongs to. A token copied from one cookie into the other is refused: the two ports never share a browser session.
- API tokens carry a plane too. A data-plane token never opens the admin port, and the other way round.
- One consequence is worth knowing before you need it: the control plane's plain HTTP port stays plain, for good. It is what a broken certificate gets repaired from. Only the data plane's plain port redirects to HTTPS.

## What lives on the data plane

Your routes, and the pages the gateway serves in its own name:

| Path | What |
|---|---|
| `/login`, `/logout` | sign-in, and the steps after it: `/update-password`, `/totp`, `/totp-enroll`, `/select-tenant`, `/select-group` |
| `/register`, `/confirm`, `/forgot-password`, `/reset-password` | account entry points, each governed by a setting - self-registration ships closed |
| `/profile/...` | what a person may do about their own account: password, MFA, passkeys, API tokens, sign-in history |
| `/refused`, `/account-pending` | why a request was turned away, and what to do next |
| `/meerkat/...` | the assets the gateway injects into proxied pages, served by the engine rather than by a route |
| `/healthz`, `/readyz` | liveness and readiness |

> [!WARNING]
> Those prefixes are the gateway's. A route whose predicates cover `/login` or
> `/meerkat/` will not be reached for them: the engine's own handlers are
> registered first, and everything else falls through to the router.

## What lives on the control plane

- `/api/...` - the admin API, about a hundred and fifty endpoints.
- `/` - the console, an Angular application embedded in the binary. English only, with no locale segment: it is an operator's tool.
- `/login`, `/logout` - the console's own sign-in, so that origin is self-sufficient.
- `/mcp` - the endpoint an agent connects to.
- `/metrics` - the Prometheus exposition, when it is switched on.
- `/healthz`, `/readyz`.

> [!NOTE]
> Enterprise edition, for `/metrics`. The counters themselves, and the
> dashboards the console draws from them, are in both images.

## One binary, no dependency

The storage is embedded - a file in the directory given by `-data`. The console
is compiled into the binary. There is no CGO, no sidecar, no message broker and
no cache to run beside it.

An external PostgreSQL database is an **option**, and it buys one thing: several
gateways serving one installation. It was chosen for what it carries beyond
storage - `LISTEN`/`NOTIFY`, which is how one gateway tells the others that
something changed, and advisory locks, which is the one exclusion the product
needs.

> [!NOTE]
> Enterprise edition. The PostgreSQL driver is only in the Enterprise binary.

## How a change takes effect

Routes are stored, not configured in a file. Saving one recompiles the whole
routing table into a snapshot and swaps it atomically - in-flight requests
finish on the table they started with. Nothing restarts, and nothing is
reloaded from disk.

In a cluster the write is announced on the change bus, and every node
recompiles. `SIGHUP` does the same thing locally, for the cases where something
outside the console wrote to the database.

A route whose vault references resolve to nothing is **left out** of the
compiled table, with a warning line, rather than failing the whole reload. That
is the normal state of a gateway just seeded from a configuration file whose
vault is not filled in yet.

## Probes

`/healthz` is liveness and answers UP unconditionally. That is the correct
answer, not a lazy one: liveness decides whether to **kill** the process, and a
probe failing on an unreachable database would restart every node at once for a
fault none of them can fix by dying.

`/readyz` is readiness and answers 503 with a reason when the store is not
answering, or when the routing table has not been compiled yet. Point your load
balancer at this one.

## Next

- What a route is made of: [Routes](/docs/concepts/routes)
- Who is calling: [Identity](/docs/concepts/identity)
