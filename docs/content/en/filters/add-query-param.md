---
title: add-query-param
section: Filters
order: 61
summary: Adds a query parameter.
---

# add-query-param

Adds a value to the query string sent upstream, next to what the caller already
sent under that name. Use it when a service reads a flag from the query and the
route, not the caller, is what should decide it.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The parameter to add. |
| `value` | string | yes | The value added. |

## Example

```yaml
filters:
  - type: add-query-param
    args:
      name: source
      value: gateway
```

## Notes

The value is added and what the caller already sent under that name is kept. Use
[set-query-param](/#/docs/filters/set-query-param) to replace it instead.

The query string is re-encoded on the way out, so the value needs no escaping of
its own.
