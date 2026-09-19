---
title: preserve-host
section: Filters
order: 72
summary: Sends the caller's own Host upstream instead of the upstream's.
---

# preserve-host

The service builds its links from the name it was called by. Without this filter
it sees the internal name, and every link, redirect and cookie domain it writes
leads there.

## Parameters

This filter takes no argument.

## Example

```yaml
filters:
  - type: preserve-host
```

`args` is omitted because there is nothing to pass.

## Notes

Both the `Host` header and the value Go puts on the wire are set, which is the
point: set one and the two disagree, and that is the virtual-host bug that takes
an afternoon.

Two pitfalls worth knowing:

- An upstream that picks a site **by name** gets the public name instead of the one it knows, and answers its default site. That is what [set-host](/docs/filters/set-host) is for.
- Keeping the Host is not enough for an application published under a prefix: it also needs to know the prefix, which [strip-prefix](/docs/filters/strip-prefix) announces as `X-Forwarded-Prefix`.
