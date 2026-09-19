---
title: What the gateway injects
section: Concepts
order: 24
summary: The user button, the navigation bar, the page agent and the colour scheme are added to proxied pages by the gateway, so the application needs no library.
---

# What the gateway injects

A route marked **UI** serves pages a browser renders, and the gateway adds a few
things to them on the way back: a user button, a navigation bar, a small script,
and the visitor's light or dark preference.

It **adds** them. The application is not asked to embed a library, to call an
endpoint, to speak a protocol or to be rebuilt. That is the point: an internal
application that has been running for six years gets a sign-out menu and a
language picker without anyone opening its source.

## What gets added

| | What it is | Turned on by |
|---|---|---|
| Page agent | `/meerkat/page.js`, which installs `window.meerkatPage` | the route being a UI route |
| User button | a `meerkat-user-button` Web Component: who you are, organisation, language, sign out | a switch on the route |
| Navigation portal | a `meerkat-portal-nav` bar listing the applications this person may open | a global switch; it replaces the standalone button |
| Identity stamp | roles and user fields written into the page's own markup, server-side | a switch on the route, per field |
| Custom CSS and JS | whatever you wrote in the route's Injections section | non-empty |
| Locale hook | a function the gateway calls when the visitor changes language | a route whose locales travel by script |

The user button also carries the entry points to what the gateway holds for that
person: their applications, their profile, and **Report a problem** when the
issue-reporting switch is on.

## The page agent

`window.meerkatPage` is the small API the injected chrome runs on, and the page
may use it too:

- `data()` - what the gateway knows about this visitor, the same payload the button draws from.
- `applyScheme`, `pickScheme` - read and change light or dark.
- `applyLanguage`, `pickLanguage`, `resolvedLanguage` - the same for the language.
- `onEvent`, `onLanguage` - subscribe to what the gateway announces.
- `signedOut` - what to do when the session is gone.

The agent also watches the session deadline, through a readable companion
cookie, and sends the page to the sign-in flow when it lapses - rather than
letting the person type into a form that will be refused.

> [!NOTE]
> The agent is a global script, not a Web Component. The user button is one.

## Light and dark

The visitor's choice lives in a cookie for a year **and** on their account, so a
browser that has never seen them still gets it right after sign-in. On the page
itself, the gateway does three things:

1. sets `color-scheme` and `data-meerkat-scheme` on the root element - enough for any application that respects the CSS property;
2. drives the application's **own** mechanism, if it has one: a named attribute, two bare attributes, or a class, on whichever tag you name;
3. writes the application's own `localStorage` key, in the application's own vocabulary - and writes it **before** the application boots, from a small inline script placed ahead of everything else.

The third is what makes a framework that reads its theme from storage on startup
come up in the right one, instead of flashing the wrong one and correcting
itself.

> [!TIP]
> The theme palette is **not** injected into your page. It travels in the JSON
> the button and the bar fetch, and is applied inside their shadow roots, so
> nothing the gateway adds can restyle your application by accident.

## What the page is told about the caller

Two paths, and they answer different needs.

**The stamp** is written server-side into the HTML, with no script and no
round-trip. Roles arrive as class tokens, or as one attribute, or as a `<meta>`.
User fields arrive the same way - `username`, `email`, `tenant`, `tenantid`,
`locale` and whatever fields your installation defined. Everything is escaped on
the way in.

This is what makes role-driven CSS work from the very first paint: a page can
hide a button for everyone who does not hold a role, with a stylesheet and
nothing else.

**The JSON**, at `/meerkat/user-button.json`, is what a script reads when it
wants the whole picture: the identity, the organisations this person may switch
to, the applications they may open, their roles and groups. It is served
`no-store`.

Neither of these is how an **upstream service** learns who is calling. That is a
separate mechanism - headers or a signed JWT, configured in the route's Identity
section - and the gateway purges any inbound value of those headers first, so a
caller cannot claim to be somebody.

## Where the assets come from

Everything under `/meerkat/...` is served by the gateway engine itself, not by a
route: `page.js`, `user-button.js`, `portal.js` and their JSON companions. The
scripts are cached for five minutes; anything carrying an identity is
`no-store`.

> [!WARNING]
> `/meerkat/` is reserved. A route whose predicates cover it will never be
> reached for those paths.

## When nothing is injected

Rewriting a response means buffering it, so the gateway is deliberately narrow
about when it does. Nothing is injected when:

- the response is not `text/html`;
- the body is not a **whole document** - an HTML fragment returned to an XHR has no `<head>` and is handed back untouched, because prepending anything to a template breaks the framework that asked for it;
- the status is 204, 304 or 206;
- the connection is an upgrade, or the body is a server-sent-event stream;
- the response says `Cache-Control: no-transform`;
- the body is larger than the rewrite ceiling - 20 MiB by default, settable between 1 and 256;
- the body is compressed with something the gateway cannot re-encode. Identity, gzip and brotli round-trip; **zstd does not**, and a zstd response is passed through as it is.

A skipped injection is silent: no error, no header, no log line. If the button is
missing from a page, one of the conditions above is the first place to look.

A rewritten response also loses its `ETag`, since the bytes are no longer the
ones the upstream hashed, and a response carrying an identity is marked
`no-store, private` so no shared cache can hand one person's page to another.
