---
title: redirect
section: Filters
order: 73
summary: Answers with a redirect instead of proxying.
---

# redirect

Answers a redirect without calling anything. For a path that has moved, a
shortcut that should land elsewhere, or an old entry point kept alive after a
migration.

This is a **terminal** filter: nothing is proxied and no upstream is called.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `location` | string | yes | Where the caller is sent, absolute or relative. |
| `status` | integer | no | The redirect status, `3xx`. Default: `302`. |

A status outside `3xx` is refused when the route is saved.

## Example

```yaml
filters:
  - type: redirect
    args:
      location: https://shop.example.com/catalogue
      status: 301
```

## Notes

`301` is permanent and browsers remember it for a long time, which is a decision
you cannot take back for the people who already got it. `307` is temporary and
keeps the method, so a `POST` stays a `POST`.

A route with a redirect needs no upstream. Its request filters are dropped, since
nothing is proxied; response filters still apply.
