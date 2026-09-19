---
title: remove-request-header
section: Filters
order: 77
summary: Removes a request header before proxying.
---

# remove-request-header

Stops a header from reaching the service - something the caller has no business
sending, or a name the application would read as an instruction.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header removed. |

## Example

```yaml
filters:
  - type: remove-request-header
    args:
      name: X-Internal-Debug
```

## Notes

Every value under that name goes. Header names are case-insensitive, so
`x-internal-debug` and `X-Internal-Debug` are the same header.

To remove a cookie, use
[remove-request-cookie](/#/docs/filters/remove-request-cookie): cookies all share
one header, and deleting it would take the session with it.
