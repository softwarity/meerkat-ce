---
title: maintenance
section: Filters
order: 68
summary: Answers 503 with the gateway's unavailable page instead of proxying.
---

# maintenance

Takes one route out of service without touching the rest. The route keeps
matching, so no other route takes its traffic while its service is down, and
visitors get the installation's unavailable page rather than a `502`.

This is a **terminal** filter: nothing is proxied and the service is never called.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `reason` | string | no | What to say, from a closed list: empty (say nothing), `maintenance`, `upgrade`, `incident`. |

## Example

```yaml
filters:
  - type: maintenance
    args:
      reason: upgrade
```

## Notes

The answer is the installation's unavailable page, in the visitor's language, with
`503` and `Cache-Control: no-store`.

The reason is a key, not a sentence: the page is read in twenty languages, and a
sentence typed here would be a sentence in one of them. An empty reason says
nothing, which is sometimes the honest answer; a value outside the list says
nothing either.

> [!NOTE]
> For the whole gateway at once, there is a switch rather than a filter:
> maintenance mode takes every route off the air with nothing to edit and nothing
> to undo.
