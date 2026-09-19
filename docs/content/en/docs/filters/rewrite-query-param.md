---
title: rewrite-query-param
section: Filters
order: 84
summary: Rewrites a query parameter's value with a regexp replacement.
---

# rewrite-query-param

Cleans a parameter the service should not receive as sent. The example that
matters is a return URL: `?redirect=https://evil.test/back` reaching the service
as `?redirect=/back`.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The parameter rewritten. |
| `pattern` | string | yes | The regexp matched against the value. |
| `replacement` | string | no | What replaces it. Empty removes what matched. Captures are `$1`, `$2`. |

A pattern that does not compile is refused when the route is saved.

## Example

```yaml
filters:
  - type: rewrite-query-param
    args:
      name: redirect
      pattern: ^https?://[^/]+
      replacement: ''
```

## Notes

Every value of the parameter is rewritten when it appears several times. Nothing
happens when the parameter is absent.

The regexp syntax is Go's RE2: no backreferences and no lookaround.
