---
title: time-window
section: Predicates
order: 48
summary: Matches requests made inside a time window.
---

# time-window

Matches while the clock is inside the window. It opens or closes a route at a
date, without anybody being awake: a migration that starts on Monday, an offer
that ends at midnight.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `from` | string | no | Start of the window, RFC 3339, e.g. `2026-10-05T00:00:00+02:00`. The offset travels with it. |
| `to` | string | no | End of the window, RFC 3339. Must be after `from`. |

Give `from`, `to`, or both. A window open at both ends matches every moment,
which is what having no time predicate already does, so it is refused.

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /promotions/**
  - type: time-window
    args:
      from: 2026-11-27T00:00:00+01:00
      to: 2026-12-01T00:00:00+01:00
```

## Notes

Outside the window the route does not match, so the next matching route
answers - or nobody, and that is a `404`. Put the route that answers the rest
of the time underneath.

Both bounds are exclusive: the window is open strictly after `from` and strictly
before `to`.

A date that is not `RFC 3339` is refused when the route is saved, not silently at
request time. The offset is part of the value, so `+01:00` and `Z` mean what
they say wherever the gateway runs.

The console's route tester can pin the clock, which is how a window is tried on a
date that has not arrived.

## Reference

[RFC 3339](https://www.rfc-editor.org/rfc/rfc3339) - date and time on the
Internet.
