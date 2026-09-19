---
title: cookie
section: Predicates
order: 41
summary: Matches when a cookie is present, against a list of values or a regexp.
---

# cookie

Matches when the named cookie is present, and optionally when its value is one
of a list or matches a shape. A cookie lasts the whole session, so this is what
pins one person to one side of a canary or to a beta.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The cookie to look for. |
| `values` | list of strings | no | The cookie value must be one of them. |
| `regexp` | string | no | Full-match regexp on the value. Use it for a shape, not for a list. |

Give `values` or `regexp`, not both: a list matches whole values, a regexp
matches a shape, and two answers to the same question is one too many. With
neither, presence of the cookie is enough.

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /checkout/**
  - type: cookie
    args:
      name: checkout_variant
      values:
        - new
```

## Notes

The cookie has to be there. A caller without it does not match, so the route
that serves everyone else goes **underneath** this one.
