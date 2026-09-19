---
title: max-request-body
section: Filters
order: 69
summary: Refuses a request body over this size, with 413.
---

# max-request-body

Caps what a caller may upload to this route. It is a **gate**: it accepts or
refuses before any other filter transforms anything, and it answers the caller
itself.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `size` | string | yes | The cap, e.g. `2MB`, `512KB`, or a number of bytes. |

Units are `KB`, `MB`, `GB` (or `K`, `M`, `G`, `B`); no unit means bytes. The size
must be greater than zero - removing the filter is how a limit is lifted.

## Example

```yaml
filters:
  - type: max-request-body
    args:
      size: 2MB
```

## Notes

A body whose length is declared is refused **without reading anything**: the
caller said how much was coming, and refusing at that word costs one comparison
instead of a megabyte of transfer. A chunked body, which declares no length, is
cut off as it arrives.

The refusal is a `413` carrying the arithmetic: what was received and what this
route accepts. "Too large" alone is the error that costs an afternoon, since the
limit lives where neither the caller nor the application developer can see it.

WebSocket connections are never capped: a socket has no body, it has a
conversation, and its bytes flow through the same reader.
