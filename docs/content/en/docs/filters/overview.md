---
title: Filters
section: Filters
order: 60
summary: What a filter is, the four phases one can run in, and the thirty-three of them.
---

# Filters

A **filter** is a brick a route poses on the traffic it has accepted. Where a
[predicate](/docs/predicates/overview) decides whether a request is for this
route, a filter decides what happens to it: refuse it, change it on the way in,
change the answer on the way back, or answer it outright.

They are all written the same way, a `type` and its `args`:

```yaml
filters:
  - type: strip-prefix
    args:
      parts: 1
  - type: set-request-header
    args:
      name: X-Tenant
      value: northwind
  - type: security-headers
    args:
      frameOptions: DENY
```

An unknown argument, a missing required one or a value of the wrong type is
refused when the route is saved, naming what is allowed.

## The four phases

A filter's phase is not a choice: it comes with the type, and it says **when** the
brick runs.

| Phase | What it does |
| --- | --- |
| **gate** | Accepts or refuses, before anything is transformed. It answers the caller itself. |
| **request** | Changes the request on its way to the service. |
| **response** | Changes the answer on its way back to the caller. |
| **terminal** | The route answers by itself, and nothing is proxied. |

For one request, in order:

1. the **gates**, in the order they are written. One that refuses answers on the spot, with a status the caller can act on, and nothing further runs.
2. the **request** filters, in the order they are written. Then the upstream is called: its own base path is appended after the filters have had their say, and identity forwarding runs last, so it wins on the headers it writes itself.
3. the **response** filters, in the order they are written, on the way back.

A gate is not a predicate. A predicate that does not match lets the **next route**
try; a gate that refuses ends the request there. Too large is not "not for this
route", it is no.

## What terminal means

A terminal filter answers the route itself, so the upstream is never called:
[redirect](/docs/filters/redirect), [respond](/docs/filters/respond) and
[maintenance](/docs/filters/maintenance) are the three.

- **One per route.** A second terminal on the same route is refused.
- **Request filters are dropped** and the gateway logs how many: there is no proxied request left for them to change.
- **Response filters still apply.** What a terminal answers is a response like any other, and a `Cache-Control` or a security header on it is as legitimate as on a proxied one.
- **Gates still apply.** A route answering by itself has as much reason to refuse an oversized body as one that proxies.

> [!TIP]
> Any argument may hold a vault reference, written `$name`, resolved when the
> route is loaded - which is how a shared secret stays out of the exported
> configuration. Two arguments are taken verbatim and never expanded: the
> [respond](/docs/filters/respond) template and the
> [version](/docs/predicates/version) pattern, because both write a `$` of
> their own.

## Gates

| Type | What it does |
| --- | --- |
| [max-request-body](/docs/filters/max-request-body) | Refuses a request body over this size, with `413`. |
| [max-request-headers](/docs/filters/max-request-headers) | Refuses a request whose headers weigh more than this, with `431`. |

## Request filters

| Type | What it does |
| --- | --- |
| [add-query-param](/docs/filters/add-query-param) | Adds a query parameter. |
| [add-request-header](/docs/filters/add-request-header) | Adds a request header value, optionally only when the caller sent none. |
| [copy-request-header](/docs/filters/copy-request-header) | Copies a request header under a second name, leaving the original in place. |
| [prefix-path](/docs/filters/prefix-path) | Prepends a prefix to the path before proxying. |
| [preserve-host](/docs/filters/preserve-host) | Sends the caller's own Host upstream instead of the upstream's. |
| [remove-query-param](/docs/filters/remove-query-param) | Removes a query parameter from the proxied request. |
| [remove-request-cookie](/docs/filters/remove-request-cookie) | Removes one cookie from the request. |
| [remove-request-header](/docs/filters/remove-request-header) | Removes a request header before proxying. |
| [rename-request-header](/docs/filters/rename-request-header) | Moves a request header to another name, values and all. |
| [rewrite-path](/docs/filters/rewrite-path) | Rewrites the path with a regexp replacement. |
| [rewrite-query-param](/docs/filters/rewrite-query-param) | Rewrites a query parameter's value with a regexp replacement. |
| [set-host](/docs/filters/set-host) | Sets the Host sent upstream. |
| [set-path](/docs/filters/set-path) | Replaces the whole path sent upstream. |
| [set-query-param](/docs/filters/set-query-param) | Sets a query parameter, replacing any value the caller sent. |
| [set-request-header](/docs/filters/set-request-header) | Sets a request header, replacing any client value. |
| [strip-prefix](/docs/filters/strip-prefix) | Removes the first segments of the path before proxying. |

## Response filters

| Type | What it does |
| --- | --- |
| [add-response-header](/docs/filters/add-response-header) | Adds a response header value. |
| [cache-control](/docs/filters/cache-control) | Sets `Cache-Control` on the response. |
| [cookie-attributes](/docs/filters/cookie-attributes) | Forces attributes on the cookies an upstream sets. |
| [dedupe-response-header](/docs/filters/dedupe-response-header) | Drops repeated values of a response header. |
| [remove-json-fields](/docs/filters/remove-json-fields) | Removes fields from a JSON answer, by name or by dotted path. |
| [remove-response-header](/docs/filters/remove-response-header) | Removes a response header before it reaches the client. |
| [rename-response-header](/docs/filters/rename-response-header) | Moves a response header to another name, values and all. |
| [rewrite-location](/docs/filters/rewrite-location) | Brings the `Location` of an upstream redirect back into the public space. |
| [rewrite-response-header](/docs/filters/rewrite-response-header) | Rewrites a response header's value with a regexp replacement. |
| [security-headers](/docs/filters/security-headers) | Adds the response headers a browser hardens on. |
| [set-response-header](/docs/filters/set-response-header) | Sets a response header, replacing any upstream value. |
| [set-status](/docs/filters/set-status) | Overrides the upstream response status code. |

## Terminal filters

| Type | What it does |
| --- | --- |
| [maintenance](/docs/filters/maintenance) | Answers `503` with the gateway's unavailable page instead of proxying. |
| [redirect](/docs/filters/redirect) | Answers with a redirect instead of proxying. |
| [respond](/docs/filters/respond) | Answers from a template, with the signed-in caller available to it. |
