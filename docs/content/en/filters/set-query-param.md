---
title: set-query-param
section: Filters
order: 89
summary: Sets a query parameter on the proxied request.
---

# set-query-param

Decides a query parameter at the route, replacing whatever the caller sent. Use it
when the value is the route's business and not the caller's - a tenant, an API
key, a fixed format.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The parameter set. |
| `value` | string | yes | The value written. |

## Example

```yaml
filters:
  - type: set-query-param
    args:
      name: format
      value: json
```

## Notes

Replaces any value the caller sent, and adds the parameter when it was absent.
The value is encoded on the way out, so it needs no escaping of its own.

To keep what the caller sent and add another value, use
[add-query-param](/#/docs/filters/add-query-param).
