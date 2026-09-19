import { Routes, UrlMatcher, UrlSegment } from '@angular/router';
import { LANGS } from './site/i18n';

// The language is a PATH SEGMENT, not a setting: /en/docs/... and
// /fr/docs/... are two addresses, both real, both sendable. A reader who
// arrives with no language is sent to the one they asked their browser for.
//
// The version of the documentation is a path segment too, but further in -
// /en/docs/1.3/filters/respond - and only the documentation has one. The
// current version wears no segment at all, so its addresses never move when a
// release ships. Neither is a route here: everything after the language is one
// slug, resolved against the content the build wrote.

const lang: UrlMatcher = (segments: UrlSegment[]) => {
  const first = segments[0]?.path;
  if (first && (LANGS as readonly string[]).includes(first)) {
    return { consumed: [segments[0]], posParams: { lang: segments[0] } };
  }
  return null;
};

export const routes: Routes = [
  {
    matcher: lang,
    loadComponent: () => import('./site/shell.component').then((m) => m.ShellComponent),
    children: [
      {
        path: '**',
        loadComponent: () => import('./site/page.component').then((m) => m.PageComponent),
      },
    ],
  },
  // No language in the address: keep the path, prepend the reader's own.
  {
    path: '**',
    loadComponent: () => import('./site/enter.component').then((m) => m.EnterComponent),
  },
];
