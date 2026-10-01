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

The notation is the **Ant-style** path pattern of the Spring family
([PathPattern](https://docs.spring.io/spring-framework/docs/current/javadoc-api/org/springframework/web/util/pattern/PathPattern.html)),
in a strict subset: no `?`, no `*` wildcard, no `{*name}`. A pattern must start
with `/` and is matched **segment by segment**:

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

## Excluding a path

There is no "everything but" in a pattern, and no regular expression either:
the gateway reads the prefix of a path pattern in several places (the
per-endpoint security that maps a request back to its OpenAPI operation, the
portal's link to an application, the language redirect, the endpoint metrics),
and a regular expression has no prefix to read.

To keep a path out, give it a **route of its own**, ordered above the others,
that always refuses - access *Nobody*, or a fixed 404. It covers every route
that would have caught that path, the catch-all included, where an exclusion
written inside one route would have to be repeated in each of them, and would
be forgotten in the next one.
