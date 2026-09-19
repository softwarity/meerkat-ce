---
title: The agent endpoint
section: Automation
order: 280
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
| `save_route`, `delete_route` | Write a route and apply it at once. |
| `list_route_bricks` | The catalogue of predicates and filters, with their parameters. |
| `list_users`, `list_tenants` | The accounts and the organisations. |
| `read_traffic` | What is passing through right now. |
| `read_audit` | The audit trail, within the caller's own scope. |
| `get_settings`, `save_portal` | The global settings, and the navigation portal. |
| `get_branding`, `save_branding` | The identity the built-in pages wear. |
| `list_themes` | The colour palettes, and which is active. |
| `export_configuration` | The whole configuration, as a readable document. |
| `save_configuration`, `list_configurations` | Named snapshots to come back to. |

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
