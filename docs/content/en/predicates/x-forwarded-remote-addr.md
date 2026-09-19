---
title: x-forwarded-remote-addr
section: Predicates
order: 51
summary: Matches the rightmost X-Forwarded-For address against CIDR ranges - the address the last proxy reports.
---

# x-forwarded-remote-addr

Matches the **last** address of `X-Forwarded-For` against CIDR ranges. Use it
when a front proxy or a load balancer sits in front of Meerkat: the connection
address is then the proxy's, and this is the one it reported.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `cidrs` | list of strings | yes | The ranges accepted, e.g. `10.0.0.0/8`, `192.168.1.10/32`. Several act as OR. |

A bad CIDR is refused when the route is saved, naming the value.

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /partner/**
  - type: x-forwarded-remote-addr
    args:
      cidrs:
        - 203.0.113.0/24
```

## Notes

The **rightmost** entry is read, which is the one the last proxy in the chain
appended - the only entry a caller cannot choose. A request with no
`X-Forwarded-For` does not match.

> [!WARNING]
> A client can send any `X-Forwarded-For` it likes. Use this predicate only when
> a trusted proxy is the one writing that header; with nothing in front of the
> gateway, use [remote-addr](/#/docs/predicates/remote-addr).
