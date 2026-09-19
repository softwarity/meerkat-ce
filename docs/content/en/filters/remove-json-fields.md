---
title: remove-json-fields
section: Filters
order: 74
summary: Removes fields from a JSON answer, by name or by dotted path.
---

# remove-json-fields

The service answers with more than its callers should see. Removes fields from the
JSON on its way out, by name or by path, without the service being changed.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `fields` | list of strings | yes | Field names or dotted paths, e.g. `password`, `user.email`. |

## Example

```yaml
filters:
  - type: remove-json-fields
    args:
      fields:
        - password
        - passwordHash
        - user.email
```

## Notes

A name alone (`password`) is removed at the top level; a dotted path
(`user.email`) follows the document down. Both work **inside arrays**, which is
the case the filter exists for: a service answering a list of users would
otherwise keep every password it was asked to drop.

The answer has to declare JSON (`application/json` or a `+json` media type). What
is not JSON, or what claims to be and does not parse, passes through untouched: a
gateway that mangles what it cannot read is worse than one that forwards it.

Some answers are never rewritten, by design: event streams and anything marked
`no-transform`, `204`, `304` and `206` responses, upgrades, a body compressed with
something other than gzip or brotli, and a body over the installation's rewriting
ceiling (`20 MiB` unless an operator changed it). A bigger answer passes through
whole rather than truncated.
