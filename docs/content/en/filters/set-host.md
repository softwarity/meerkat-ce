---
title: set-host
section: Filters
order: 87
summary: Sets the Host sent upstream.
---

# set-host

One machine serves several sites and picks by name. This is the name it gets,
whatever the upstream address is.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `host` | string | yes | The `Host` sent upstream, e.g. `billing.internal`. |

## Example

```yaml
filters:
  - type: set-host
    args:
      host: billing.internal
```

## Notes

Both the `Host` header and the value Go puts on the wire are set. Setting only one
makes the two disagree, which is the virtual-host bug that takes an afternoon.

For the caller's own name rather than a fixed one, use
[preserve-host](/#/docs/filters/preserve-host). Do not pose both on the same
route: the last one written wins, which is a coin toss nobody reads in a list of
filters.
