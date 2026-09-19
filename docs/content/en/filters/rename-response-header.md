---
title: rename-response-header
section: Filters
order: 80
summary: Moves a response header to another name, values and all.
---

# rename-response-header

Renames a header in the answer, for a client that reads a different name than the
service writes.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `from` | string | yes | The header read, and removed. |
| `to` | string | yes | The header written. |

## Example

```yaml
filters:
  - type: rename-response-header
    args:
      from: X-Request-Id
      to: X-Correlation-Id
```

## Notes

Every value moves to the new name and the old name is gone. Anything already
under `to` is replaced.

Nothing happens when `from` is absent.
