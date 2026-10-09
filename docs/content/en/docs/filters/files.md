---
title: files
section: Filters
order: 67
summary: Serves files uploaded on the route instead of proxying - the font, stylesheet, script or image a UI asks for when nothing behind the gateway serves it.
---

# files

Lets the route answer by itself with **files uploaded on it**. The case it is
for: a UI that needs a resource nothing behind the gateway serves - a font, a
stylesheet, a script, an image - typically **offline**, where the CDN it names is
out of reach. You declare the route that takes that path, choose the **Files**
mode under Target, and upload the files.

This is a **terminal** filter: nothing is proxied and no upstream is called.

![A route in the Files mode: a stylesheet and two fonts uploaded under /fonts, each with the name it is served as and its type, then the index, the browser cache and CORS](img/console/route-editor-files.webp)

## Parameters

| Name | Type | Required | What it does |
| --- | --- | --- | --- |
| `index` | string | no | The file answered at the route's own path, without a name after it - the `index.html` of a small site. Empty: that path answers 404, only the files' own addresses answer. |
| `cors` | boolean | no | Let pages of any origin read the files. Default: `true`. |
| `maxAge` | integer | no | Seconds a browser keeps a file before revalidating it. Default: `3600`. |

The files themselves are not parameters: they are uploaded on the route, under
Target, and live beside it.

## Example

A route on `/fonts/**` in the Files mode, with `inter.css` and
`files/inter-latin.woff2` uploaded on it:

```yaml
predicates:
  - type: path
    args:
      patterns: ["/fonts/**"]
filters:
  - type: files
    args:
      index: inter.css
      cors: true
```

`/fonts/files/inter-latin.woff2` answers the font, `/fonts` answers `inter.css`,
any other path under `/fonts` answers 404.

## Notes

- A file is served at the route's path followed by **the name it is served
  as**, which may hold folders (`files/inter-latin.woff2`) so that a
  stylesheet's relative URLs keep working as they were written. That name starts
  as the uploaded file's own, cleaned for an address (`My file - v2.pdf` becomes
  `My-file-v2.pdf`), and is **edited in the list**: renaming does not upload the
  file again, and an index naming it follows.
- Its **type** is read from its name (`.woff2` is `font/woff2`, `.css` is
  `text/css`, `.js` is `text/javascript`...), then from its bytes. It is edited
  in the list too, and a warning marks the type the gateway could only guess.
- Each file's link opens it **on the data plane**, the origin the applications
  answer on - not this console's.
- Each answer carries an **ETag**, the file's hash: after `maxAge` a browser asks
  again and gets a `304` when nothing changed.
- **CORS** matters for fonts and modules: a browser refuses a font loaded by a
  page of another origin unless the answer says it may read it.
- **Limits**: 10 MiB per file, 32 MiB per route - the files travel in a
  configuration package, and what nobody can carry around is not configuration.
- An **export** carries the files beside the YAML, under
  `assets/files/<route id>/`; an import puts them back. A files route imported
  without its files is named in the plan, since it would answer 404 to
  everything.

> [!TIP]
> A UI that names `fonts.googleapis.com` will not pass through the route by
> itself. Either point the UI at the gateway, or - offline - resolve that name to
> the gateway in your DNS and give it a certificate from your own authority in
> the TLS pool: the route then answers the host the UI asks for.
