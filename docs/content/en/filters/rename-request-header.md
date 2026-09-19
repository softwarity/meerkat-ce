---
title: rename-request-header
section: Filters
order: 79
summary: Moves a request header to another name, values and all.
---

# rename-request-header

The caller sends `X-User` and the service expects `REMOTE_USER`. Moves the value
to the name the application reads.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `from` | string | yes | The header read, and removed. |
| `to` | string | yes | The header written. |

## Example

```yaml
filters:
  - type: rename-request-header
    args:
      from: X-User
      to: REMOTE_USER
```

## Notes

Every value moves and the old name is gone. Anything already under `to` is
replaced.

Nothing happens when `from` is absent. Use
[copy-request-header](/#/docs/filters/copy-request-header) when the old name has
to survive.
