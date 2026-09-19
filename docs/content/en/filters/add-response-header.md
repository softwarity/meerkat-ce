---
title: add-response-header
section: Filters
order: 63
summary: Adds a response header value.
---

# add-response-header

Adds a header value to the answer on its way back, next to the ones the service
sent. Its use is the headers that are lists by design.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header to add. |
| `value` | string | yes | The value added. |

## Example

```yaml
filters:
  - type: add-response-header
    args:
      name: Vary
      value: Accept-Language
```

## Notes

Adds a value next to the ones the service sent. Use it on multi-valued headers
such as `Vary` or `Set-Cookie`; two values on a single-valued header are
ambiguous, and it is the browser that decides which one wins.

For a single decision, use
[set-response-header](/#/docs/filters/set-response-header). For a header that
arrived twice, use
[dedupe-response-header](/#/docs/filters/dedupe-response-header).
