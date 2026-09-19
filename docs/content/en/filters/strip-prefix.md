---
title: strip-prefix
section: Filters
order: 93
summary: Removes the first N segments of the path before proxying.
---

# strip-prefix

The gateway publishes `/demo/orders`, the service only knows `/orders`. The most
used filter of the catalogue: it is what lets one application be mounted under a
path of your choosing without the application knowing.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `parts` | integer | no | Number of leading segments removed. Default: `1`. Must be at least `1`. |
| `announcePrefix` | boolean | no | Tell the service where it is published, as `X-Forwarded-Prefix`. Default: `true`. |

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /demo/**
filters:
  - type: strip-prefix
    args:
      parts: 1
```

`/demo/orders/8814` reaches the service as `/orders/8814`, with
`X-Forwarded-Prefix: /demo`.

## Notes

`X-Forwarded-Prefix` is how a service that **builds** its own links learns where it
lives. Without it, an application seeing `/orders` writes `/orders`, the browser
follows it, and it lands outside the route. Spring reads the header through
`ForwardedHeaderFilter`, nginx poses it; a service that only ever answers does not
need it.

Whatever a caller sent under that name is purged, on every route, even one with no
`strip-prefix`: a caller posing their own prefix would make the service write its
links wherever they asked.

Two `strip-prefix` in a row work: the announced prefix widens to what was actually
consumed rather than being overwritten.

An application that builds absolute links usually wants
[preserve-host](/#/docs/filters/preserve-host) as well: the prefix says where, the
Host says under which name.
