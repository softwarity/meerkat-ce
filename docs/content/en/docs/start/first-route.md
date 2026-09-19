---
title: Your first route
section: Getting started
order: 3
summary: Put one of your own services behind the gateway, from the console, and check that the right route answers.
---

# Your first route

A route answers one question - *is this request for me* - and then says what to
do with it. This page creates one that proxies an internal service, and checks
it before letting anyone near it.

Say the service is an invoicing application, reachable inside your cluster at
`billing:8080`, and you want it served on `/billing`.

## Open the editor

In the console, go to **Infra > Routes** and click **New route**. The editor
opens in a drawer, with the sections of a route down the left. Two of them
carry a star, which means the route cannot be saved without them: **Target**
and **Predicates**.

Name the route at the top - `billing` will do. The name is what you will look
for in the table, in the traffic curves and in the audit trail.

![The route editor, open on Target](img/console/route-editor-target.webp)

## Target: what answers

**Target** decides how the route answers. Four modes, and only the first one
calls a service:

| Mode | What it does |
|---|---|
| Proxy | fetches from an upstream and hands back what it said |
| Redirect | sends the browser somewhere else |
| Maintenance | serves the built-in unavailable page |
| Respond | answers from a template, calling nothing |

Keep **Proxy** and type the upstream: `http://billing:8080`. Plain `http` is
the default on purpose - inside a cluster, TLS usually ends at the gateway and
the hop to the service is cleartext. `https` and `h2c` are the other two
choices.

> [!TIP]
> Where the gateway can read its runtime - a Docker or Swarm socket, or a
> Kubernetes namespace from the inside - the services it found are offered in
> that field as you focus it. Free text stays free: an upstream outside the
> cluster is typed.

The same section holds how long this upstream may take (connect, then first
answer) and whether to stop calling it when it stops answering. Both inherit
from the installation until you say otherwise, so leave them alone for now.

## Predicates: which requests

**Predicates** is the *is this for me* half. Add a **path** predicate with the
pattern `/billing/**`.

A route may carry several predicates, and they are combined with AND: a path
plus a host plus a method is one route that wants all three. Predicates
available today include path, host, header, cookie, method, query,
remote-addr, x-forwarded-remote-addr, time-window, version and weight.

![Three predicates on one route](img/console/route-editor-predicates.webp)

## Strip the prefix

The gateway matched `/billing/**`, but the invoicing application knows nothing
about that prefix - it serves `/invoices`, not `/billing/invoices`. Open
**Incoming** and add a **strip-prefix** filter with `1` part.

Incoming filters are the request-side transformations: headers, query
parameters, path, host. **Outgoing** does the same on the way back.

![The incoming filters of a route](img/console/route-editor-filters.webp)

## Who may pass

**Security** carries the route's access rule. Leave it empty and the route is
open - the gateway proxies without asking who is calling, and the service
decides for itself. Set it to require a signed-in account, or named roles, or
named accounts, and a caller who does not qualify never reaches the upstream.

Details are in [Access control](/docs/concepts/access-control).

![The security section of a route](img/console/route-editor-security.webp)

## Save

**Save** applies at once - the routing table is recompiled and, in a cluster,
the other nodes are told. There is nothing to restart and no file to reload.

A new route is created with order `0`, which puts it at the **top** of the
table. That matters: the first route whose predicates match is the one that
answers, so a new route is tried before everything already there.

## Check it before your users do

Two ways, and use the first one.

**Routing test**, on the Routes screen, composes a request that is never sent:
method, path, host, headers, cookies, client address, clock, and an identity to
be judged as. It then reports, route by route, which one takes the request and
why the others did not - not matched, matched but the rule turned this caller
away, or not evaluated because a route above matched first.

Then the real thing:

```bash
curl -i http://localhost:8080/billing/invoices
```

If nothing matches, the gateway answers 404. If a route matched but its rule
turned you away, a UI route lands on a page naming the rule that refused; a
service route gets a plain 403.

## Next

- What a route is made of, in full: [Routes](/docs/concepts/routes)
- What the gateway adds to a proxied page: [What the gateway injects](/docs/concepts/data-plane-chrome)
