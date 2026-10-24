---
title: The agent endpoint
section: Operations
order: 230
summary: Meerkat speaks MCP, so an assistant can read and change the gateway with tools rather than by guessing REST calls.
---

# The agent endpoint

Meerkat exposes its control plane to an AI assistant over **MCP** (Model Context
Protocol). The assistant does not learn a REST API: it gets a list of tools with
names, descriptions and argument schemas, and calls them.

## Turning it on

The endpoint is **off by default**. Switch it on in the console under
**Infra > MCP**, then mint a control-plane token (**Infra > Access tokens**).

The endpoint answers at `/mcp` on the **control plane** port, and only with a
Bearer token:

```bash
curl -X POST https://meerkat.internal:9090/mcp \
  -H "Authorization: Bearer mk_..." \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

> [!NOTE]
> It refuses calls that carry a browser origin. A control-plane token is not a
> session, and a page in a browser has no business holding one.

## What a token may do

A token carries a **perimeter**, and the tools a caller sees are the tools that
perimeter opens - a read-only token is not shown the writing ones. The same
guards apply as in the console: whoever administers a domain gets that domain's
tools, and nobody gets more by asking an assistant instead of clicking.

## The tools

| Tool | What it answers |
|---|---|
| `describe_gateway` | Which edition, which version, how much of everything. Call it first. |
| `list_routes`, `get_route` | The routes, then one of them in full. |
| `test_routing` | Which route a given request would reach, and why. |
| `list_operations`, `call_route` | What a route's service offers, then a real call to it through the gateway. |
| `save_route`, `delete_route` | Write a route and apply it at once. |
| `list_route_bricks` | The catalogue of predicates and filters, with their parameters. |
| `list_users`, `list_tenants` | The accounts and the organisations. |
| `list_services` | What the gateway sees running around it. |
| `list_schedules` | The scheduled calls: what runs at night, how often, as whom, and how the last run ended. |
| `pause_schedule`, `run_schedule` | Stop a scheduled call, let it fire again, or bring its next turn forward to now. |
| `describe_schedule` | Write what a scheduled call is for, when its service did not say. |
| `cancel_schedule_run` | Stop the run a scheduled call has in flight: closed at once, the service told at its next report. |
| `read_traffic` | What is passing through right now. |
| `read_logs` | The gateway's own last lines, on the node that answered. |
| `read_audit` | The audit trail, within the caller's own scope. |
| `get_settings`, `save_portal` | The global settings, and the application catalogue (its `portal` mode, the bar, on the Enterprise image only). |
| `get_branding`, `save_branding` | The identity the built-in pages wear. |
| `list_themes` | The colour palettes, and which is active. |
| `export_configuration` | The whole configuration, as a readable document. |
| `save_configuration`, `list_configurations` | Named snapshots to come back to; `replace` refreshes one from the running state, before a push. |
| `list_oidc_clients`, `save_oidc_client`, `try_oidc_sign_in` | The applications signing in through the gateway, and what one would receive for a given person. |
| `get_signing_keys`, `set_issuer`, `renew_signing_keys`, `preview_identity` | The issuer and the public keys, and what a draft identity would send. |

## Everything the console does

The tools above answer the frequent questions well. For everything else,
three tools reach the **whole control plane** - the API the console itself
calls:

| Tool | What it does |
|---|---|
| `list_admin_api` | The sections, then the endpoints of one, with a summary when the API's documentation has one. |
| `read_admin_api` | A GET, as the console would. |
| `call_admin_api` | Any request: create an organisation, undo a change from the history, compare two configurations, edit a theme or a setting. |

They go through the same endpoints as the console, with the agent's own token:
the endpoint's guard decides, a read-only token stays read-only, and the audit
trail names the token. Nothing is reimplemented, so nothing is out of step.

What stays closed is short, and each refusal gives its reason:

- the **certificates** and their private keys, the **ACME** accounts (opening one
  accepts an authority's terms) and the **trusted authorities**;
- the **vault**: an agent works with `$names`, a secret is typed by a person;
- minting **control-plane tokens**;
- the **backups** (a file to download, a database copy);
- the **TLS doors and ports** and the switch that lets agents in: a change there can
  cut the console off. A configuration an agent imports, activates or restores is
  refused if it changes them.

## Calling the services behind the routes

`list_operations` reads a route's OpenAPI spec: every operation with the
**public path** to call - the route's prefix added when the route strips it -
and, for one operation, its parameters, body and responses with the schemas
they reference.
Each operation also says what the **application** requires to call it, when
its spec says so (public, a signed-in caller, or roles), and the route's
declared roles come with the catalogue roles they are tied to (see
[Per-endpoint security](/docs/access/endpoint-security#the-roles-the-application-declares)):
the agent knows which roles to pass to `call_route` to be let in, or kept out.

`call_route` sends a real request through the route and returns the status,
the headers and the body (cut at 64 KiB). It is handed to the application plane
in process, so everything a client meets applies: the route's access rule, the
per-endpoint security, the filters, the quotas, and the identity the route
forwards to the service.

- **As whom.** By default the caller is `agent`, holding **every role** of the
  catalogue - in the identity the service receives too - so an administrator's
  agent is not stopped by rights it would have to manage. Pass `as` and `roles`
  to call as somebody shaped otherwise, and `tenant` for a route reserved to
  organisations.
- **Why a refusal.** A `403` says why in its body: the gateway names the role
  or the organisation missing, and a service's own reason comes back as the
  service wrote it.
- **For real.** A `POST`, `PUT` or `DELETE` changes the service's data. The call
  is offered to a full-access token, never to a read-only one, and every call
  is written to the audit trail with the token that made it. The service
  receives the marker headers of a simulated call, so its own log tells a test
  from real traffic.

## Pictures are described, never sent

A logo or a page background is stored as a data URI, and a megabyte of base64
would cost more of a conversation than everything else an answer says. So the
reading tools return a **line about the image** instead of its bytes:

```json
"background": { "image": "<png, 45 KiB>", "fit": "cover", "dim": 30 }
```

This matters when writing back. An assistant reads, changes one field, and
sends the whole object again - so a summary in an image field means **keep what
is stored**. To change a picture, pass an `https` URL and the gateway fetches it
itself; to remove one, pass `"none"`.

> [!TIP]
> The same applies to a portal module's icon: it is named, not drawn. Pass
> `"storefront"` and the gateway stores the drawing.
