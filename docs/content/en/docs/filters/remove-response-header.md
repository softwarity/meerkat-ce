---
title: remove-response-header
section: Filters
order: 78
summary: Removes a response header before it reaches the client.
---

# remove-response-header

Takes a header out of the answer. Its everyday use is what a service says about
itself: its framework, its version, the name of the machine that answered.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `name` | string | yes | The header removed. |

## Example

```yaml
filters:
  - type: remove-response-header
    args:
      name: X-Powered-By
```

## Notes

Every value under that name goes before the answer reaches the client.

When the value is the problem rather than the header, use
[rewrite-response-header](/docs/filters/rewrite-response-header): removing
`Server` outright tells an attentive reader as much as leaving it.
