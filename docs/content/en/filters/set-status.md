---
title: set-status
section: Filters
order: 92
summary: Overrides the upstream response status code.
---

# set-status

Replaces the status code of the answer. Its honest use is a service that answers
the wrong code for a case it got right, and which cannot be changed today.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `status` | integer | yes | The status sent to the client, between `100` and `599`. |

A status outside that range is refused when the route is saved.

## Example

```yaml
filters:
  - type: set-status
    args:
      status: 404
```

## Notes

The body is untouched, so a `200` set over an error page hides the error from
anything that only reads the code - monitoring included, and the route's own
metrics with it.

The route's metrics and its circuit breaker both read the status that reaches the
client, not the one the service sent. A `200` posed over a `502` therefore hides
the failure from them as well: the breaker counts a success and never opens.
