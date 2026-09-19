---
title: security-headers
section: Filters
order: 86
summary: Adds the response headers a browser hardens on.
---

# security-headers

Poses the headers a browser hardens on, for an application that sets none. One
brick instead of four `set-response-header`, and with the two decisions that are
easy to get wrong already taken.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `referrerPolicy` | string | no | `Referrer-Policy`. Default: `strict-origin-when-cross-origin`. Empty leaves what the application set. |
| `frameOptions` | string | no | `X-Frame-Options`: `DENY` or `SAMEORIGIN`. Default: `SAMEORIGIN`. Empty leaves what the application set. |
| `hstsMaxAge` | integer | no | `Strict-Transport-Security` lifetime in seconds. Default: `0`, which leaves the header alone. |
| `contentSecurityPolicy` | string | no | Sent as-is when set. A CSP is written per application, never guessed. |

## Example

```yaml
filters:
  - type: security-headers
    args:
      frameOptions: DENY
      hstsMaxAge: 31536000
```

## Notes

`X-Content-Type-Options: nosniff` is always sent. The referrer policy, the frame
policy and the CSP are sent when set. Each is **set**, not added: these are
decisions, and two of them is none.

HSTS is sent **only over TLS**, whatever `hstsMaxAge` says. A browser remembers it
for months and would then refuse plain HTTP, which is how a development gateway
becomes unreachable. It is sent with `includeSubDomains`.

`X-XSS-Protection` and the other two IE-era headers are not sent at all: current
browsers ignore them.

> [!TIP]
> Leave `contentSecurityPolicy` empty until you have one written for that
> application. A guessed CSP either blocks the page or allows everything.
