---
title: cache-control
section: Filters
order: 64
summary: Sets Cache-Control on the response.
---

# cache-control

Decides what may be cached, for a service that says nothing about it. This is the
filter that keeps a personal page out of a shared cache, and the one that lets a
public catalogue be kept for an hour.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `value` | string | yes | The `Cache-Control` sent, e.g. `public, max-age=3600` or `no-store`. |

## Example

```yaml
filters:
  - type: cache-control
    args:
      value: no-store
```

## Notes

`no-store` for personal data, `public, max-age=3600` for shared content.

`Expires` and `Pragma` are removed at the same time: they could say the opposite,
and the older header wins in some caches. Saying one thing twice is how a page
gets cached that should not have been.
