---
title: add-request-header
section: Filters
order: 62
summary: Adds a request header value; ifNotPresent skips when the client already sent one.
---

# add-request-header

Adds a header value to the request sent upstream, next to the values already
there. With `ifNotPresent` it becomes a default: the route fills the header in
only when the caller did not.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header to add. |
| `value` | string | yes | The value added. |
| `ifNotPresent` | boolean | no | Add nothing when the caller already sent that header. Default: `false`. |

## Example

```yaml
filters:
  - type: add-request-header
    args:
      name: Accept-Language
      value: en-GB
      ifNotPresent: true
```

## Notes

Adds a value next to the existing ones. With `ifNotPresent`, nothing is added when
the caller already sent that header.

To replace what the caller sent, use
[set-request-header](/#/docs/filters/set-request-header) instead: two values on a
header a service reads as single-valued is a bug waiting for the day the service
picks the other one.
