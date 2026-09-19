---
title: set-request-header
section: Filters
order: 90
summary: Sets a request header (replacing any client value).
---

# set-request-header

Decides a header at the route. This is the filter for what a service should be
told and a caller should not be able to claim.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header set. |
| `value` | string | yes | The value written. |

## Example

```yaml
filters:
  - type: set-request-header
    args:
      name: X-Tenant
      value: northwind
```

## Notes

Every value the caller sent under that name is replaced.

The value is **fixed**: nothing is taken from the path, the query or the caller.
For the signed-in user, use the route's identity forwarding rather than this
filter - it runs after the filters and overwrites the headers it writes itself, so
a `set-request-header` on one of those names has no effect.

A vault reference (`$name`) is expanded here, which is how a shared API key
reaches a service without being written into the exported configuration.
