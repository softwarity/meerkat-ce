---
title: cookie-attributes
section: Filters
order: 65
summary: Forces attributes on the cookies an upstream sets.
---

# cookie-attributes

Adds the attributes a cookie should have carried. The application runs on plain
HTTP behind the gateway and sets its cookies accordingly; the browser, which
reached the gateway over TLS, expects better.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `secure` | boolean | no | Add `Secure`. Default: `true`. |
| `httpOnly` | boolean | no | Add `HttpOnly`. Default: `false`. |
| `sameSite` | string | no | `Lax`, `Strict` or `None`. Empty leaves what the application set. |

## Example

```yaml
filters:
  - type: cookie-attributes
    args:
      secure: true
      httpOnly: true
      sameSite: Lax
```

## Notes

`Set-Cookie: id=abc; Path=/` becomes `Set-Cookie: id=abc; Path=/; Secure`. Every
cookie of the answer is rewritten, and what the application already set on them
is kept.

`SameSite=None` forces `Secure`, whatever `secure` says: without it every current
browser drops the cookie, so asking for one is asking for both.
