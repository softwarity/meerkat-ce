---
title: prefix-path
section: Filters
order: 71
summary: Prepends a prefix to the path before proxying.
---

# prefix-path

The caller asks `/orders` and the service receives `/api/orders`. For an
application mounted under a path the gateway does not show.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `prefix` | string | yes | What is prepended to the path, e.g. `/api`. |

## Example

```yaml
filters:
  - type: prefix-path
    args:
      prefix: /api
```

## Notes

The prefix is prepended verbatim, so write the leading slash and no trailing one.
It is not a template: `{language}` or `{id}` in a prefix is prepended as those
characters.

The upstream's own base path is appended **after** the filters have run. When the
upstream is already written `http://orders.internal:8080/api`, there is nothing to
prefix here.

[strip-prefix](/docs/filters/strip-prefix) is the reverse.
