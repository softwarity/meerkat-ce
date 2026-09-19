---
title: Open source
section: Softwarity
order: 4
summary: The components, libraries and tools we publish, because we needed them and carrying a copy from project to project is how a codebase ages.
---

# Open source

Everything below exists because we needed it on a real project. Publishing it
costs a README and a release pipeline; not publishing it costs the same
component, slightly different, in every repository we touch. We picked the
first.

## Angular

The furniture of an administration console - the parts nobody wants to write
again and everybody writes again. Built on Angular Material, on the current
major, signal-first.

| Package | What it is |
| --- | --- |
| [`rail-nav`](https://github.com/softwarity/rail-nav) | The Material Design 3 navigation rail, with its contextual drawer. The rail on the left of this site is it. |
| [`row-actions`](https://github.com/softwarity/row-actions) | Row actions for a Material table, in a toolbar that collapses instead of a column of icons. |
| [`loading-indicator`](https://github.com/softwarity/loading-indicator) | The Material 3 expressive loading indicator, with its morphing animation. |
| [`split-button`](https://github.com/softwarity/split-button) | A split button directive: the default action, and the menu beside it. |
| [`timezone-select`](https://github.com/softwarity/timezone-select) | A timezone picker that navigates by UTC offset rather than by an alphabetical list of four hundred names. |
| [`store`](https://github.com/softwarity/store) | Persist what the reader chose - visible columns, sort, page size, filters - in the browser, with one decorator and no server. |

## Angular and i18n

Translating an Angular application is where a lot of time goes, so it has its
own shelf.

| Package | What it is |
| --- | --- |
| [`polyglot`](https://github.com/softwarity/polyglot) | Serves every locale of an i18n application at once behind a single dev port, read from `angular.json`. It removes the "one locale per `ng serve`" tax. |
| [`angular-i18n-cli`](https://github.com/softwarity/angular-i18n-cli) | Sets up and maintains the i18n configuration of a project rather than making you remember it. |
| [Xliff translator](https://xliff.softwarity.io/en/) | A hosted editor for XLIFF catalogues: the translation step, without a spreadsheet. |

## NestJS

| Package | What it is |
| --- | --- |
| [`nestjs-granted`](https://github.com/softwarity/nestjs-granted) | RBAC for NestJS endpoints: decorator-based authorisation with composable boolean expressions. |
| [`nestjs-amqp`](https://github.com/softwarity/nestjs-amqp) | AMQP 1.0 for NestJS, on rhea, with decorator-based publishers and consumers. |

## Live data, and web components

| Package | What it is |
| --- | --- |
| [`livewire`](https://github.com/softwarity/livewire) | Live query synchronisation across NestJS, Go and Angular: subscribe to a query over one WebSocket, get its answer and every answer after it. Virtual-scroll data source included. |
| [`interactive-code`](https://github.com/softwarity/interactive-code) | Syntax-highlighted, click-to-edit code with collapsible sections, copy and download. Framework-agnostic, zero dependencies. |

## Aeronautical meteorology

A domain we have worked in for years, and the tools that came out of it. All of
them are framework-agnostic web components or libraries: they carry the ICAO
and WMO rules, not a UI opinion.

| Project | What it is |
| --- | --- |
| [Sigmet Draw](https://github.com/softwarity/sigmet-draw) | Draw and edit ICAO SIGMET hazard areas on a map, on MapLibre, OpenLayers or Leaflet. |
| [Sigwx Draw](https://github.com/softwarity/sigwx-draw) | Draw ICAO SIGWX and WAFS significant-weather charts, on the same three. |
| [TAC editor](https://github.com/softwarity/tac-editor) | Edit the Traditional Alphanumeric Codes of aviation meteorology, with highlighting and validation. |
| [TTAAii provider](https://github.com/softwarity/ttaaii-provider) | The data and logic of WMO TTAAii bulletin headings - completion, validation, decoding - with no UI attached. |
| [GeoJSON editor](https://github.com/softwarity/geojson-editor) | Edit GeoJSON features with highlighting, collapsible nodes and a colour picker. |

## Tools and images

| Project | What it is |
| --- | --- |
| [plug](https://github.com/softwarity/plug) | Run a local process as if it were inside your cluster: cluster DNS and services reachable, no application configuration. It is the engine behind [Dev mode](/product/dev-mode). |
| [PDFBox service](https://github.com/softwarity/pdfbox) | A small self-contained Spring Boot service that turns HTML into PDF, PDF/A included. Push HTML, get the binary back. |

## Products

- **[Meerkat](https://github.com/softwarity/meerkat-ce)** - the app-gateway
  this site is about. Its core is under the Functional Source License and
  becomes Apache 2.0 two years after each release; see
  [Editions](/product/editions).
