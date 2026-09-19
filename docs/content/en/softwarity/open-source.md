---
title: Open source
section: Softwarity
order: 4
summary: The components and tools we publish, because we needed them and carrying a copy from project to project is how a codebase ages.
---

# Open source

Everything below exists because we needed it on a real project. Publishing it
costs a README and a release pipeline; not publishing it costs the same
component, slightly different, in every repository we touch. We picked the
first.

## Angular components

Built on Angular Material, on the current major, signal-first. They are the
furniture of an administration console - the parts nobody wants to write again
and everybody writes again.

| Package | What it is |
| --- | --- |
| [`@softwarity/rail-nav`](https://github.com/softwarity/rail-nav) | The Material Design 3 navigation rail, with the contextual drawer. The rail on the left of this site is it. |
| [`@softwarity/row-actions`](https://github.com/softwarity/row-actions) | Row actions for a Material table, in a toolbar that collapses instead of a column of icons. |
| [`@softwarity/loading-indicator`](https://github.com/softwarity/loading-indicator) | The Material 3 expressive loading indicator, with its morphing animation. |
| [`@softwarity/split-button`](https://github.com/softwarity/split-button) | A split button directive: the default action, and the menu beside it. |
| [`@softwarity/timezone-select`](https://github.com/softwarity/timezone-select) | A timezone picker that navigates by UTC offset rather than by an alphabetical list of four hundred names. |
| [`@softwarity/livewire`](https://github.com/softwarity/livewire) | Live query synchronisation: subscribe to a query over one WebSocket, get its answer and every answer after it. Virtual-scroll data source included. |
| [`@softwarity/projects`](https://github.com/softwarity/projects) | The menu in the top bar of this site: a web component built from Markdown. |

## Tools

- **[plug](https://github.com/softwarity/plug)** - a developer's workstation
  joins a cluster, and a service running locally answers under its cluster
  name. It is a product of its own, and the engine behind
  [Dev mode](/product/dev-mode).

## Products

- **[Meerkat](https://github.com/softwarity/meerkat-ce)** - the app-gateway
  this site is about. Its core is under the Functional Source License and
  becomes Apache 2.0 two years after each release; see
  [Editions](/product/editions).
