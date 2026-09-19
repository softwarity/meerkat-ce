---
title: Predicates
section: Predicates
order: 40
summary: How a route decides that a request is for it, and how the eleven predicates combine.
---

# Predicates

A **predicate** is a condition on the incoming request. A route carries a list of
them and takes the request only when **every one** of them accepts it: predicates
are ANDed, never ORed. A route with no predicate at all takes everything.

To express an OR, write two routes - or use a predicate that already accepts a
list. Several patterns, hosts, methods or values inside one predicate act as OR.

```yaml
routes:
  - id: orders
    name: Orders API
    enabled: true
    order: 100
    upstream: http://orders.internal:8080
    predicates:
      - type: path
        args:
          patterns:
            - /orders/**
      - type: method
        args:
          methods:
            - GET
            - POST
```

That route answers a `GET` or a `POST` under `/orders`, and nothing else.

## Which route answers

Routes are tried in ascending **order** (ties broken by name) and the **first**
route whose predicates all match answers. Nothing matched at all is a plain
`404`.

A route's access rule takes part in the choice: a caller the rule turns away
falls through to the next route that matches, and is refused by the first route
that turned them away only if no other one answers. A rule set to **deny** is the
exception - a closed door stays closed.

A disabled route is not loaded, so it never matches.

## The catch-all

A catch-all is not a setting: it is an ordinary route whose path predicate is
`/**`, ordered last. It catches what no other route claimed, which is how an
installation answers something better than a bare `404`.

> [!TIP]
> Leave a gap in the order numbers - `100`, `200`, `300` - so a route can be
> inserted between two others without renumbering the table.

## Trying it before shipping

The console's route tester composes a fictitious request (method, path, host,
header, cookie, query, client address, clock) and shows **each predicate's
verdict**, one line per predicate, rather than a single yes or no. The clock is
part of it, so a `time-window` can be tried on a date that has not arrived yet.

## The eleven predicates

| Type | What it matches |
| --- | --- |
| [cookie](/docs/predicates/cookie) | A cookie is present, against a list of values or a regexp. |
| [header](/docs/predicates/header) | A header is present, against a list of values or a regexp. |
| [host](/docs/predicates/host) | The request Host, exact names or `*.suffix` wildcards. |
| [method](/docs/predicates/method) | The HTTP method. |
| [path](/docs/predicates/path) | The request path, against one or more patterns. |
| [query](/docs/predicates/query) | A query parameter is present, against a list of values or a regexp. |
| [remote-addr](/docs/predicates/remote-addr) | The client address, against CIDR ranges. |
| [time-window](/docs/predicates/time-window) | The request was made inside a time window. |
| [version](/docs/predicates/version) | An API version range read from a header, a query parameter or the path. |
| [weight](/docs/predicates/weight) | A share of the traffic of a group, for a canary. |
| [x-forwarded-remote-addr](/docs/predicates/x-forwarded-remote-addr) | The rightmost `X-Forwarded-For` address, against CIDR ranges. |

Anything a route does to the request once it has matched is a
[filter](/docs/filters/overview).
