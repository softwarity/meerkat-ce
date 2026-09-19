---
title: method
section: Predicates
order: 44
summary: Matches the HTTP method.
---

# method

Matches the HTTP verb. Use it to send reads and writes of the same path to two
different places, or to publish only the verbs an application should receive.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `methods` | list of strings | yes | The verbs accepted, e.g. `GET`, `POST`. Several act as OR. |

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /catalogue/**
  - type: method
    args:
      methods:
        - GET
        - HEAD
```

## Notes

Verbs are compared upper-cased, so `get` and `GET` are the same thing here.

A verb left out does not get a `405`: the route simply does not match, and the
next one that does answers - or nobody, and that is a `404`. Add a route
answering the rest if a refusal has to be explicit.
