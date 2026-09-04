# CLAUDE.md

Guidance for Claude Code sessions working on this repository.

## What this is

**Meerkat** is an app-gateway (Go) with an Angular admin console, by Softwarity.
What the product does, and how far each piece is built, lives in **`FEATURES.md`**
(French) - read it before any structural change. One table, one line per feature, a
stable ID (AUTH-xx, ROUTE-xx...) that is never reused, and the state read from the code.
**Shipping something means ticking its box in the same commit.**

**Session handoff**: read **`memory.md`** first (current state, known pitfalls, next
milestones) and update it before ending a work session that changed the state.

## Layout

| Path | What |
|---|---|
| `cmd/meerkat/` | single binary entry point (data plane :8080 + control plane :9090) |
| `internal/` | Go core (FSL license) - routing, gateway, store, session, auth, admin |
| `ee/` | Enterprise code, commercial license. Linked in by the `ee` build tag (`internal/edition`), which is what makes the two images two products |
| `console/` | Angular admin console (served through the admin port) |
| `docs/` | doc site (GitHub Pages) |

## Dev loop

See "Development" in README.md: `npm start` (console, `ng serve` on :4200)
+ `MEERKAT_* env... make dev` (gateway, air hot reload). Browse the **admin port**.

## Hard rules

- **Go**: version from `go.mod` (latest stable). `make fmt lint test` green before commit.
  Pure Go only (no CGO). Errors must name what failed and list what is allowed.
- **Angular**: LATEST major only - verify with `npm view @angular/core dist-tags` before
  touching versions; zero deprecated APIs (no @angular/animations, no ngModel, no
  constructor injection). Signal-first everywhere: `signal/computed/input()/output()/model()`,
  **Signal Forms** (`@angular/forms/signals`, `[formField]`), zoneless, standalone, fine-grained
  components, minimal custom CSS on `--mat-sys-*` tokens. Follow the Angular team's
  best-practices resource (served by the `angular` MCP server configured in `.mcp.json`).
- **Softwarity ecosystem first**: before writing any generic component/tool, check the
  GitHub org `softwarity` - rail-nav, row-actions, loading-indicator, split-button,
  timezone-select, release-flow... When unsure of an npm package name, read the
  repo's README (package names don't always match repo intuition).
- **The console is English only** - an operator's tool, served at the root with no
  locale segment: no catalogues, no per-locale build, nothing to extract. But keep
  MARKING new strings (`i18n="@@Cancel"`, `$localize`): the marks cost an attribute
  and they are what would make a future translation a translation job rather than a
  refactoring. Stop marking and the head start rots one screen at a time.
  The **data plane** pages (sign-in, MFA, user button) ARE translated: their
  catalogue is `internal/auth/i18n.go`, a Go map, and a new language is a new entry.
- **Commits**: author is the repo owner's git identity (set `user.name`/`user.email`
  locally per repo - the harness may reset the global config). English messages,
  imperative subject.
- **A new control-plane endpoint answers one question**: can an agent do this? Either add its
  tool to `admin.tools()` and name it in `agentCovers`, or say in `agentIgnores` why an agent
  has no business there - and classify it in `readsNothing` or `writesSomething` so a read-only
  token's perimeter stays true. `internal/admin/mcp_coverage_test.go` refuses silence.
- **Never commit secrets** - no exceptions (lesson from V1).
- Validate work by **running it** (the smoke chain: login -> route via data plane ->
  admin API), not only by unit tests.
