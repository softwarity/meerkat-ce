import { Routes, UrlMatcher, UrlSegment } from '@angular/router';
import { LANGS } from './site/i18n';

// An address names a page and nothing else: /docs/filters/respond. The
// language it is read in is the reader's - their browser's, or the one they
// picked - unless the address forces one with ?lg=fr.
//
// The version of the documentation is a path segment - /docs/1.3/filters/respond -
// and only the documentation has one. The current version wears no segment at
// all, so its addresses never move when a release ships. Neither is a route
// here: the whole path is one slug, resolved against the content the build
// wrote.

// The old shape, /en/... and /fr/...: kept as a door, never written any more.
const legacyLang: UrlMatcher = (segments: UrlSegment[]) => {
  const first = segments[0]?.path;
  return first && (LANGS as readonly string[]).includes(first) ? { consumed: segments } : null;
};

export const routes: Routes = [
  {
    matcher: legacyLang,
    loadComponent: () => import('./site/enter.component').then((m) => m.EnterComponent),
  },
  {
    path: '',
    loadComponent: () => import('./site/shell.component').then((m) => m.ShellComponent),
    children: [
      {
        path: '**',
        loadComponent: () => import('./site/page.component').then((m) => m.PageComponent),
      },
    ],
  },
];
