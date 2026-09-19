---
title: max-request-headers
section: Filters
order: 70
summary: Refuses a request whose headers weigh more than this, with 431.
---

# max-request-headers

Caps the header block, for a caller sending hundreds of cookies or roles. Like
[max-request-body](/#/docs/filters/max-request-body) it is a **gate**: it decides
before anything is transformed and answers the caller itself.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `size` | string | yes | The cap, e.g. `16KB`, or a number of bytes. |

Units are `KB`, `MB`, `GB` (or `K`, `M`, `G`, `B`); no unit means bytes. The size
must be greater than zero.

## Example

```yaml
filters:
  - type: max-request-headers
    args:
      size: 16KB
```

## Notes

The refusal is a `431`, which tells the caller which half to shrink, and it
carries the arithmetic: what arrived and what this route accepts.

What is weighed is the request line plus each header as it would be written on
the wire, `Host` included - the same way Go's own server weighs it. What the
gateway adds afterwards, such as the forwarding or identity headers, is not
counted.
