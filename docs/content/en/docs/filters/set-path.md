---
title: set-path
section: Filters
order: 88
summary: Replaces the whole path sent upstream.
---

# set-path

Sends everything the route matches to one fixed path, typically a health endpoint
published under a friendlier name.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `path` | string | yes | The path sent upstream. Absolute, e.g. `/health`. |

A path that does not start with `/` is refused when the route is saved.

## Example

```yaml
filters:
  - type: set-path
    args:
      path: /actuator/health
```

## Notes

The whole path is replaced, whatever the caller asked for: a route carrying this
filter has exactly one destination.

The query string is untouched. Use
[remove-query-param](/docs/filters/remove-query-param) if it should not travel.
