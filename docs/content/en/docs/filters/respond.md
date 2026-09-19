---
title: respond
section: Filters
order: 81
summary: Answers from a template instead of proxying, with the signed-in caller available to it.
---

# respond

Lets the route answer by itself from a template, with the signed-in caller
available to it. Two uses: exposing the identity endpoint a hosted application
expects **in its own shape**, and serving a small fixed document - a `robots.txt`,
a public configuration - with no service behind it.

This is a **terminal** filter: nothing is proxied and no upstream is called.

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `body` | string | yes | A Go `text/template`. See below for what it can reach. |
| `contentType` | string | no | Content-Type of the answer. Default: `application/json; charset=utf-8`. |
| `status` | integer | no | HTTP status of the answer. Default: `200`. |

## Example

```yaml
filters:
  - type: respond
    args:
      body: '{"name": {{json .Username}}, "authorities": {{json (wrap "authority" .Roles)}}}'
      contentType: application/json; charset=utf-8
      status: 200
```

An application asking `/user` gets `{"name":"jsmith","authorities":[{"authority":"BILLING"}]}`.

## Notes

The caller is available as `{{.Username}}`, `{{.UserID}}`, `{{.Fullname}}`,
`{{.Email}}`, `{{.Tenant}}`, `{{.TenantID}}`, `{{.Timezone}}`, `{{.Roles}}`, plus
`{{.SignedIn}}`, which is `false` when nobody is signed in. Nothing else is
reachable: no store, no filesystem, no other account.

Three functions come with it:

- `json` renders a value as JSON, quotes and escaping included.
- `join` flattens a list, e.g. `{{join "," .Roles}}`.
- `wrap` turns a list into one-key objects: `{{json (wrap "authority" .Roles)}}` gives `[{"authority":"A"}]`, which is the shape half the applications out there expect for roles.

> [!WARNING]
> Write `"name": {{json .Username}}` and **not** `"name": "{{.Username}}"`. The
> second looks right and breaks the day a name holds a quote - and names come
> from directories, not from you.

The template is parsed **and run once** when the route is saved, against a witness
caller: a field that does not exist (`{{.Usernme}}`) is caught there rather than
in production. The console previews the rendered answer as you type, through the
same code.

The answer carries `Cache-Control: no-store`, since what it says depends on who is
asking.

The template is taken verbatim: a `$` in it is never read as a vault reference.
