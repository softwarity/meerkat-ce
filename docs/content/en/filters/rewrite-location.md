---
title: rewrite-location
section: Filters
order: 82
summary: Brings the Location of an upstream redirect back into the public space.
---

# rewrite-location

The service redirects to its own address and the browser leaves the gateway. This
rewrites the `Location` so it points back at the name the caller used:
`http://billing.internal:8080/login` becomes `https://shop.example.com/login`.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `from` | string | no | Prefix to replace. Empty means the upstream's own origin. |
| `to` | string | no | What replaces it. Empty means the origin the caller used. |

With both left out, the filter swaps the upstream's origin for the public one,
which is what it is for. Set them to rewrite a path prefix as well.

## Example

```yaml
filters:
  - type: rewrite-location
    args:
      from: http://billing.internal:8080/app
      to: https://shop.example.com/billing
```

## Notes

Only a `Location` starting with `from` is rewritten; anything else is left alone,
and so is an answer that carries no `Location`.

The upstream's origin is taken from the request **as it was actually sent**, not
as the route was configured, so a redirect chain lands on the host that answered.

The public origin is read from the forwarding headers the gateway itself sets
(`X-Forwarded-Host`, `X-Forwarded-Proto`). When they are missing there is nothing
reliable to point at, and the header is left as it is: a `Location` rewritten
towards a host nobody asked for is worse than one left alone.
