---
title: copy-request-header
section: Filters
order: 66
summary: Copies a request header under a second name, leaving the original in place.
---

# copy-request-header

Puts the same value under two names. This is the migration filter: one service
already reads the new header name, another still needs the old one, and the route
feeds both.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `from` | string | yes | The header read. |
| `to` | string | yes | The header written. |

## Example

```yaml
filters:
  - type: copy-request-header
    args:
      from: X-Request-Id
      to: X-Correlation-Id
```

## Notes

Copies every value to the second name and keeps the first. Nothing happens when
`from` is absent.

The target is **replaced**, not appended to: copying twice would otherwise pile
the same value up. Use
[rename-request-header](/docs/filters/rename-request-header) when the old name
has to disappear.
