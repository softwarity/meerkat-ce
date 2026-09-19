---
title: host
section: Predicates
order: 43
summary: Matches the request Host against exact names or *.suffix wildcards.
---

# host

Matches the `Host` the caller used. This is the predicate for a gateway serving
several sites, or for the same application published under a customer name.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `hosts` | list of strings | yes | Exact names or `*.suffix` wildcards, e.g. `shop.example.com`, `*.example.com`. Several act as OR. |

## Example

```yaml
predicates:
  - type: host
    args:
      hosts:
        - shop.example.com
        - '*.shop.example.com'
  - type: path
    args:
      patterns:
        - /**
```

## Notes

The port is ignored: `shop.example.com` matches `shop.example.com:8443`.

A wildcard covers **subdomains only**: `*.example.com` matches
`app.example.com`, and not `example.com`. Name the bare domain as well when you
want both.
