---
title: query
section: Predicates
order: 46
summary: Matches when a query parameter is present, against a list of values or a regexp.
---

# query

Matches when the named query parameter is present, and optionally when its value
is one of a list or matches a shape. Useful for a switch a caller can put in a
link, such as a preview or a channel.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The parameter to look for. |
| `values` | list of strings | no | The parameter value must be one of them. |
| `regexp` | string | no | Full-match regexp on the value. Use it for a shape, not for a list. |

Give `values` or `regexp`, not both. With neither, presence of the parameter is
enough.

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /catalogue/**
  - type: query
    args:
      name: channel
      values:
        - mobile
        - tablet
```

## Notes

Presence counts even with an empty value: `?preview` and `?preview=` both match a
`query` predicate that names `preview` and no value.

Only the **first** value is read when the parameter appears several times.
