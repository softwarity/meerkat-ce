---
title: path
section: Predicates
order: 45
summary: Matches the request path against one or more patterns.
---

# path

Matches the request path against one or more patterns. It is the predicate
almost every route starts with: the path is what tells one application from
another.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `patterns` | list of strings | yes | The patterns the path is tried against, e.g. `/api/users/{id}`, `/static/**`. Several act as OR. |

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /orders/**
        - /invoices/**
```

## Notes

A pattern must start with `/` and is matched **segment by segment**:

- `{name}` takes exactly one segment, whatever it holds: `/orders/{id}` matches `/orders/8814` and not `/orders/8814/lines`.
- `**` matches the rest of the path, and is allowed as the **last segment only**. `/orders/**` matches `/orders`, `/orders/8814` and `/orders/8814/lines`.
- anything else is a literal segment.

Boundaries are real: `/demo/**` matches `/demo` and `/demo/x`, never
`/demolition`. A single trailing slash is ignored, so `/orders/` and `/orders`
are the same route.

> [!WARNING]
> A single `*` is **not** a wildcard. `/orders/*` matches the literal path
> `/orders/*` and nothing else. Write `{id}` for one segment and `**` for a tail.

`{name}` matches a segment but captures nothing usable elsewhere: no filter can
read it back. Use [rewrite-path](/docs/filters/rewrite-path) when the value has
to be moved around.
