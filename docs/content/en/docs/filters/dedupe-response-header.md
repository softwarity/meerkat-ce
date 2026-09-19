---
title: dedupe-response-header
section: Filters
order: 67
summary: Drops repeated values of a response header.
---

# dedupe-response-header

Keeps one value where two arrived. The gateway and the service both set the same
header, the browser refuses the pair, and the page fails for a reason nothing in
the logs explains. CORS is where this happens most.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `names` | list of strings | yes | The headers to clean, e.g. `Access-Control-Allow-Origin`. |
| `keep` | string | no | Which value survives: `first`, `last` or `unique`. Default: `first`. |

## Example

```yaml
filters:
  - type: dedupe-response-header
    args:
      names:
        - Access-Control-Allow-Origin
        - Access-Control-Allow-Credentials
      keep: first
```

## Notes

`first` keeps the service's value, `last` keeps the one added afterwards, and
`unique` keeps each distinct value once.

`unique` keeps the **order of first appearance**: a browser reads these lists
positionally, and a set would shuffle them differently on every response.

A header carrying a single value is left alone.
