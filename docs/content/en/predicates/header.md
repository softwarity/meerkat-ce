---
title: header
section: Predicates
order: 42
summary: Matches when a header is present, against a list of values or a regexp.
---

# header

Matches when the named header is present, and optionally when its value is one
of a list or matches a shape. Use it to split traffic by something the caller
declares: a client name, an environment, an API key shape.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header to look for. |
| `values` | list of strings | no | The header value must be one of them, e.g. `staging`, `prod`. |
| `regexp` | string | no | Full-match regexp on the value. Use it for a shape, not for a list. |

Give `values` or `regexp`, not both. With neither, presence of the header is
enough.

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /orders/**
  - type: header
    args:
      name: X-Client
      values:
        - mobile-app
```

## Notes

Header names are case-insensitive, values are not. Only the **first** value of a
repeated header is read.
