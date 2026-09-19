---
title: weight
section: Predicates
order: 50
summary: Splits traffic between the routes of a group (canary): a route takes weight/total of the requests.
---

# weight

Splits the traffic of a group of routes by share. This is the canary: the same
path, two upstreams, most of the traffic on the version in production and a
little on the new one.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `group` | string | yes | The name the routes of one split share. |
| `weight` | integer | yes | This route's share of the group. Must be greater than zero. |

A route takes `weight` divided by the **total of the group** of the requests. The
numbers are shares, not percentages: `8` and `2` is the same split as `80` and
`20`.

## Example

```yaml
routes:
  - id: checkout
    name: Checkout
    enabled: true
    order: 100
    upstream: http://checkout-1.internal:8080
    predicates:
      - type: path
        args:
          patterns:
            - /checkout/**
      - type: weight
        args:
          group: checkout
          weight: 8
  - id: checkout-next
    name: Checkout (next)
    enabled: true
    order: 110
    upstream: http://checkout-2.internal:8080
    predicates:
      - type: path
        args:
          patterns:
            - /checkout/**
      - type: weight
        args:
          group: checkout
          weight: 2
```

One request in five goes to the new version.

## Notes

The draw happens **per request**, so one person sees both versions during a
session. Add a [cookie](/docs/predicates/cookie) predicate on a route of the
group when someone has to stay on one side.

Shares are computed over the routes actually loaded, so **disabling** a route of
the group hands its share to the others rather than dropping that traffic.

The other predicates still apply: a route only takes its share of the requests
its own path, host and method already accepted.
