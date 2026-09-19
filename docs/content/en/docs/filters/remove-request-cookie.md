---
title: remove-request-cookie
section: Filters
order: 76
summary: Removes one cookie from the request.
---

# remove-request-cookie

Removes one cookie on the way to the service and keeps the rest - typically a
session cookie the application would mistake for its own.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The cookie removed. |

## Example

```yaml
filters:
  - type: remove-request-cookie
    args:
      name: meerkat_session
```

## Notes

The `Cookie` header is one string carrying every cookie, so this filter rebuilds
it rather than deleting it: removing the header outright would take the
application's own session with it. That is also why
[remove-request-header](/docs/filters/remove-request-header) is the wrong tool
for one cookie.
