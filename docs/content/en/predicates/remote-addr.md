---
title: remote-addr
section: Predicates
order: 47
summary: Matches the client address against CIDR ranges.
---

# remote-addr

Matches the address the connection comes from against CIDR ranges. This is how
an administration path is kept to the office network, or a partner route to one
machine.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `cidrs` | list of strings | yes | The ranges accepted, e.g. `10.0.0.0/8`, `192.168.1.10/32`. Several act as OR. |
| `useForwarded` | boolean | no | Trust the **first** `X-Forwarded-For` entry instead of the connection. Default: `false`. Only behind a trusted proxy. |

A bad CIDR is refused when the route is saved, naming the value.

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /admin/**
  - type: remote-addr
    args:
      cidrs:
        - 10.0.0.0/8
        - 192.168.1.10/32
```

## Notes

Behind another proxy the connection address is the **proxy's**, so every request
looks like it comes from one machine. Either turn `useForwarded` on, or use
[x-forwarded-remote-addr](/#/docs/predicates/x-forwarded-remote-addr), which
reads the address the last proxy reported.

> [!WARNING]
> A client can send any `X-Forwarded-For` it likes. `useForwarded` trusts the
> first entry of that header, so switch it on only when a proxy you control
> rewrites it.

A single address is a `/32` range (`/128` in IPv6).
