---
title: rewrite-path
section: Filters
order: 83
summary: Rewrites the path with a regexp replacement.
---

# rewrite-path

Reshapes the path when removing or adding a prefix is not enough - reordering
segments, dropping one in the middle, folding two shapes into one.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `pattern` | string | yes | The regexp matched against the path. |
| `replacement` | string | yes | What replaces it. Captures are `$1`, `$2`. |

A pattern that does not compile is refused when the route is saved.

## Example

```yaml
filters:
  - type: rewrite-path
    args:
      pattern: ^/shop/v2/(.*)$
      replacement: /api/$1
```

`/shop/v2/orders/8814` reaches the service as `/api/orders/8814`.

## Notes

Anchor the pattern with `^` unless you mean to match anywhere: an unanchored
pattern rewrites **every** occurrence in the path, which is rarely what was meant.

A capture written `$1` followed by a letter reads as the group named `1x`, which
does not exist, and the replacement silently loses it. Write `${1}` whenever a
letter or a digit follows.

The regexp syntax is Go's RE2: no backreferences and no lookaround. A pattern
borrowed from another gateway may need rewriting.

For the two everyday cases there are simpler bricks:
[strip-prefix](/#/docs/filters/strip-prefix) and
[prefix-path](/#/docs/filters/prefix-path). Unlike `strip-prefix`, this filter
announces nothing: a service that builds its own links will not know it lives
under a prefix.
