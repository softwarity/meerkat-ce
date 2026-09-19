---
title: CSS and JavaScript per route
section: Customising
order: 258
summary: What the gateway already injects into a proxied page, and where your own CSS and JavaScript land.
---

# CSS and JavaScript per route

Meerkat rewrites the HTML of the UI routes it proxies, to add the little of itself the visitor needs -
and to add whatever you write on the route (UIF-02, UIF-06, SAUTH-03).

Only HTML answers are touched, and only on **UI** routes. An API route's JSON is never rewritten.

## What the gateway already injects

| What | Where it lands | When |
|---|---|---|
| The page agent, `/meerkat/page.js` | top of `<body>` | on every UI route |
| The user button, or the [portal](/#/docs/customise/portal) bar | top of `<body>`, with the agent | depending on the route and the portal |
| Your extra CSS | after `<head>` | when the route carries some |
| Your extra JavaScript | after `<head>` | when the route carries some |
| The language hook | after `<head>` | when the route's language mechanism is a script |
| The maintenance stripe | after `<head>` | only for the administrator who went through the door |
| The scheme override for the application's own storage | before the agent | when the route declares one |

The agent carries the visitor's language, their light or dark choice, and the session watch. **No
route opts out of it**: a page with no agent is a page that goes on looking signed in for hours after
the session ended.

The agent and the button are **one** injection, in that order, and deliberately: both scripts are
deferred, so they run in document order, and the button's element must not upgrade before the agent
has defined what it reads. Two separate insertions would each land right after `<body>` and put the
second one first.

Everything the gateway adds goes at the top of the **body**, never inside the head. A custom element
inside `<head>` closes it where it stands, and everything after it - charset, title,
and `<base href>` - lands in the body, where a base is ignored. An application served
under a prefix would then resolve every URL from the root.

## Your own CSS and JavaScript

Two code editors on the route, under its UI options. The CSS rides a `<style>` tag, the JavaScript a
`<script>` tag, both verbatim.

| Rule | Why |
|---|---|
| At most `64 KiB` each | plenty for page tweaks; more is a bundle, and a bundle belongs to the application |
| `</style` and `</script` are refused on save | either would break out of the tag it travels in |

They are injected after `<head>`, so they come **before** the application's own stylesheets and
scripts in document order. For CSS that means the application's own rules win on equal specificity -
write with that in mind rather than reaching for `!important` first.

## Styling on the visitor's roles

The session's **effective roles** can be stamped onto the served HTML, **server-side**: as classes,
as one attribute on a tag of your choice, or as a `<meta>`. No client JavaScript, no call home. Your
CSS can then hide or show elements by role with nothing but a selector.

The same mechanism can stamp the user's own facts - username, id, full name, e-mail, organisation,
time zone, locale - each under an attribute or meta name you choose.

> [!WARNING]
> A page stamped this way belongs to **one person**, so the gateway makes it non-storable: `no-store`,
> and the headers that could say otherwise - `Expires`, `Pragma`, `Last-Modified`, `ETag` - are
> removed, whatever the application behind says. Without that, a `public, max-age=300` - the default of
> every static file server - would have a CDN, a corporate proxy or a shared browser serve Alice's page
> to Bob.

An anonymous request is not stamped and keeps the application's own caching.

## The cost

Rewriting a body means holding it in memory. The ceiling is in **Routes > Global**, between one and
two hundred and fifty-six MiB, and twenty by default. Past it the answer is forwarded **intact and
whole**: nothing breaks, the injection simply does not apply. The cost is per request being served at
that moment, so a large ceiling multiplies by however many arrive together.

The role stamping is gated on a cheap session check, so anonymous requests never buffer anything.

## What is missing

- **Rewriting `<base href>`** to match a stripped prefix (UIF-01): nothing rewrites it, which is why
  the injection is careful never to close the head.
- **A post-authentication hook** and presets for common tools (SAUTH-03): the JavaScript block is
  there, the hook that would run at sign-in is not.
- **Rewriting OpenAPI specs in proxied answers** (UIF-07): only the developer documentation portal
  rewrites the spec it serves.
