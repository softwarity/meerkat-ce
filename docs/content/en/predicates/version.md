---
title: version
section: Predicates
order: 49
summary: Matches an API version range read from a header, a query parameter or the path.
---

# version

Reads a version out of the request and matches when it falls in a range. Two
routes on the same path, one per API version: `2.x` to the new service, the rest
to the old one.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `source` | string | no | Where the version is read: `header`, `query` or `path`. Default: `header`. |
| `name` | string | no | The header or parameter carrying the version. Default: `X-API-Version`. Unused when the source is the path. |
| `pattern` | string | yes | Regexp extracting the version, e.g. `v?(\d+(?:\.\d+)*)`. |
| `from` | string | no | Lowest version accepted, inclusive, e.g. `2.0`. |
| `to` | string | no | First version **refused**, exclusive, e.g. `3.0`. |

Give `from`, `to`, or both: a range open on both sides matches every version,
which is what having no version predicate already does.

## Example

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /orders/**
  - type: version
    args:
      source: header
      name: X-API-Version
      pattern: v?(\d+(?:\.\d+)*)
      from: '2.0'
      to: '3.0'
```

Quote the bounds in YAML: unquoted, `2.0` is a number and loses its shape.

## Notes

Versions compare as **numbers**, segment by segment, so `1.10` comes after
`1.9`, where a regexp on the raw text reads it as before. Up to three segments,
and a short one is padded: `from: '2'` also takes `2.1`.

The named group `version` wins if the pattern has one, otherwise the first
capturing group, otherwise the whole match is taken as the version.

A request carrying no version, or a version the pattern cannot read, does not
match. Put the route for unversioned callers **underneath** this one.

The bounds are checked when the route is saved: `from` below `to`, both readable
as versions. What a caller sends cannot be validated in advance and simply does
not match.

The console previews this predicate as you type, running the same code the
gateway runs - it shows what was extracted and whether it falls in the range.
