---
title: set-response-header
section: Filters
order: 91
summary: Sets a response header (replacing any upstream value).
---

# set-response-header

Decides a header on the answer, replacing whatever the service sent. The generic
brick for a header no dedicated filter covers.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header set. |
| `value` | string | yes | The value written. |

## Example

```yaml
filters:
  - type: set-response-header
    args:
      name: X-Frame-Options
      value: DENY
```

## Notes

Every value the service sent under that name is replaced.

Headers that **describe the body**, such as `Content-Type` or `Content-Encoding`,
are not enforced: setting one does not change the bytes, it only makes the answer
lie about them.

For the hardening headers there is
[security-headers](/docs/filters/security-headers), and for caching
[cache-control](/docs/filters/cache-control): both take the decisions that are
easy to get wrong one header at a time.
