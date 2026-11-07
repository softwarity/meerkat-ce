// Cloudflare Pages: the one thing a file host cannot do for this site - serve
// a page in the language its address names.
//
// The site has ONE address per page, and ?lg= names the language it is read
// in: /docs/index/?lg=fr. GitHub Pages ignored the query and answered the
// English file, so a crawler saw the French page as a duplicate of the English
// one and never indexed it. Here, ?lg=fr answers the French file the build
// writes for every page (/fr/docs/index/), at the same address - the
// application does not change, and the address the reader sees does not
// either.
//
// The /en/... and /fr/... addresses of before were sent and indexed: they
// answer a permanent redirect to the address that replaced them, which is
// what passes their standing on. A JavaScript redirect did that only for a
// crawler that runs the page.
//
// Only pages reach this code: _routes.json (written by
// scripts/static-pages.mjs) keeps the bundle, the content and the pictures on
// the static path, where a request costs nothing.
const LANGS = ['en', 'fr'];
// The language every page exists in, served at the address that names none.
const FIRST = 'en';

export async function onRequest({ request, next, env }) {
  if (request.method !== 'GET' && request.method !== 'HEAD') return next();
  const url = new URL(request.url);

  const old = url.pathname.match(/^\/(en|fr)(\/.*)?$/);
  if (old) {
    const to = new URL(url);
    to.pathname = slashed(old[2] || '/');
    to.searchParams.set('lg', old[1]);
    return Response.redirect(to.toString(), 301);
  }
  // A section's first page is a page named index (/docs/index/). Without its
  // slash, the host reads "index" as the directory's own file and sends the
  // reader to /docs/, where there is none: put the slash back first.
  if (/\/index$/.test(url.pathname)) {
    const to = new URL(url);
    to.pathname = slashed(url.pathname);
    return Response.redirect(to.toString(), 301);
  }

  const lg = url.searchParams.get('lg');
  if (lg && lg !== FIRST && LANGS.includes(lg)) {
    const file = new URL(url);
    file.pathname = `/${lg}${url.pathname}`;
    file.search = '';
    const res = await env.ASSETS.fetch(new Request(file.toString(), request));
    // A page written in one language only: the address still answers, in the
    // language it exists in.
    if (res.ok) return res;
  }
  const res = await next();
  // A section's own address (/docs/) has no page: its first one has.
  if (res.status === 404 && url.pathname.endsWith('/')) {
    const first = new URL(url);
    first.pathname = `${url.pathname}index/`;
    first.search = '';
    if ((await env.ASSETS.fetch(new Request(first.toString(), request))).ok) {
      first.search = url.search;
      return Response.redirect(first.toString(), 301);
    }
  }
  return res;
}

// The address of a page is a directory: with its trailing slash.
const slashed = (path) => (path.endsWith('/') ? path : `${path}/`);
