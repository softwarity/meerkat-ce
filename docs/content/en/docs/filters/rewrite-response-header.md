---
title: rewrite-response-header
section: Filters
order: 85
summary: Rewrites a response header's value with a regexp replacement.
---

# rewrite-response-header

Takes an internal name out of what the service says about itself:
`billing-3.internal:8080` leaving as `service:8080`.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header rewritten. |
| `pattern` | string | yes | The regexp matched against the value. |
| `replacement` | string | no | What replaces it. Empty removes what matched. Captures are `$1`, `$2`. |

A pattern that does not compile is refused when the route is saved.

## Example

```yaml
filters:
  - type: rewrite-response-header
    args:
      name: Server
      pattern: ^[^.]+\.internal
      replacement: service
```

## Notes

Every value of a repeated header is rewritten. Nothing happens when the header is
absent.

For a `Location`, use [rewrite-location](/docs/filters/rewrite-location): it
knows the origin the caller actually used, which a fixed replacement does not.

The regexp syntax is Go's RE2: no backreferences and no lookaround.
