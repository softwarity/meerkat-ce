---
title: Access tokens, MCP and API
section: The console
order: 164
summary: Driving Meerkat without a browser - a token for a script, an MCP connection for an assistant, and the API reference.
---

# Access tokens, MCP and API

Three screens, one subject: working on this gateway from outside the console.

- **Access tokens** is on both planes. Under **Infra**, it mints console tokens
  for a script or a pipeline, and internal tokens for a service calling the
  scheduler. Under **Application**, it also lists the application tokens: an
  app admin sees everyone's and can revoke them.
- **Infra > MCP** (infra admins) connects an assistant, and produces no key at
  all.
- **Meerkat, API reference** is the reference for the calls both of them make.

## Access tokens

An admin token opens the console's own API without a browser. It acts **with your
own powers**, narrowed by its perimeter, and it authenticates on the admin port as
`Authorization: Bearer mk_...`.

![The Access tokens screen: two console tokens and two internal ones, with their owner, plane, perimeter and last use](img/console/access-tokens.webp)

Four tokens: two for the console - read only, and full access - and two
internal ones, `billing` and `orders-api`, which services present to the
scheduler; `orders-api` has just written the schedules the Scheduler screen
shows. The filters above the list narrow it to **Console**, **Application** (the
tokens people created on their profile, for an app admin) or **Internal**. A row
opens its drawer.

**Internal** lists the tokens of perimeter `schedules`: they open the
[scheduler's API](/docs/operations/scheduler#the-token) on the internal plane
and nothing on the console's port. Under that filter the button reads **New
internal token**: a name - the service that holds it, offered from the
services the gateway sees, or `global` for one they share - the addresses and
the expiry, the perimeter being what makes it internal. Whether a service needs
one at all is the scheduler's **Access** setting.

Creating one asks four things:

| Field | What it decides |
|---|---|
| **Token name** | What you will recognise in the list and in the audit trail |
| **Perimeter** | *Scheduled calls only* opens `/api/schedules` and nothing else, on the internal plane, and lists the token under Internal; *Read only* reads and runs the testers; *Full access* is everything you can do |
| **Used from** | Comma-separated addresses or CIDR ranges. Judged on the connecting address, never on a forwarded header |
| **Expiry** | Never, 30 days, 90 days, or 1 year |

**A perimeter only takes away**: at most what you are. A read-only token minted
by root reads everything root sees and changes nothing.

The secret is **shown once**. Copy it then; it cannot be retrieved. Keep it in an
environment variable rather than in a file, because a configuration file is a
thing people commit.

Afterwards, each row offers: the enable switch (it stops working, and can be
turned back on), **Edit** to change what it may do without touching the secret,
**New secret** to rotate it, and **Revoke**. The line shows the perimeter, the
prefix, when it was created, when it expires and when it was last used.

> [!WARNING]
> A new secret takes effect immediately. Whatever is using the old one is refused
> until the new one is in place.

## MCP

Meerkat answers the Model Context Protocol on the admin port, so an assistant can
read this gateway and work on it with you. There is no port to open: the endpoint
is where your console already is.

![The MCP screen: the endpoint switch, the client picker with the command to paste, and an empty list of connected agents](img/console/mcp.webp)

The endpoint is on and shows its URL, the client picker has Claude Code selected
with the one-line command ready to copy, and no agent is connected yet.

It ships **off**. The switch at the top turns it on and shows the URL; while it is
off, the rest of the screen is hidden, since there is nothing to connect to.

**Connect an agent** gives the exact command for Claude Code, Gemini CLI, Kimi
CLI, Codex CLI, or a generic JSON block for anything else. Each command installs
the server at the widest scope the client offers, **all your projects**:
`-s user` for Claude Code, `--scope user` for Gemini CLI, and Codex keeps it in
`~/.codex/config.toml` anyway. The first call opens
your browser on this console: you sign in, you choose what the agent may do, and
**no key is ever written into a file**.

**Connected agents** lists what is plugged in, with each one's perimeter and when
it was last used, and disconnects one in a click.

When the console is served over HTTPS with a certificate from a **private
authority** (or a self-signed one), the agent refuses it until it trusts that
authority. The *My certificate comes from a private authority* panel, shown only
over HTTPS, gives the command for the chosen agent and for macOS, Linux or
Windows: Claude Code and Gemini CLI run on Node.js and read `NODE_EXTRA_CA_CERTS`,
which adds your authority to the public ones (on macOS:
`echo 'export NODE_EXTRA_CA_CERTS=/path/to/ca.crt' >> ~/.zshrc && source ~/.zshrc`,
then start the agent again from a new terminal); Codex CLI trusts the system's
authorities, so yours is added to the system store; Kimi CLI documents no setting
of its own, and the system store is the way offered.

For a client that cannot authenticate through a browser, the *My client cannot do
that* panel shows the same command with a bearer header, and links to Access
tokens.

Every change an agent makes is written in the audit trail with the token's name
beside the account's - *admin, via claude-desktop*, not *admin* - and a
[restore point](/docs/console/configuration) is written after it, like any
other change.

## API

![API reference: the control plane's REST reference, its calls tried with the console session](img/console/api-reference.webp)

**Meerkat, API reference** shows the control plane's own REST reference - the
swagger-ui page the gateway serves - and the calls are tried **with your real
session**: being in the console is the authorisation. It is the same surface a
token drives.

## Traps

- **Read only is the default when minting**, and it is usually what you want.
  Widening is one edit; a leaked full-access token is not.
- **A token acts as you.** Deleting the account that owns it takes its powers
  with it.
- **A connected agent is not in the token list.** It is a connection, and it is
  seen and cut off under MCP.
- **The scheduled-calls perimeter exists for backend services**: a credential
  that lives in a deployment manifest is the one most likely to leak and least
  likely to be rotated. See [scheduled calls](/docs/operations/scheduler).
