---
title: remove-query-param
section: Filters
order: 75
summary: Removes a query parameter from the proxied request.
---

# remove-query-param

Takes a parameter out of the query string before the service sees it - a tracking
tag, a debug switch the caller should not be able to turn on.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The parameter removed. |

## Example

```yaml
filters:
  - type: remove-query-param
    args:
      name: debug
```

## Notes

Every value of that parameter goes; the rest of the query string is unchanged.

Nothing happens when the parameter is absent.
