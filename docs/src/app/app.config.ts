import { ApplicationConfig, inject, provideAppInitializer } from '@angular/core';
import { provideRouter, withInMemoryScrolling } from '@angular/router';
import { routes } from './app.routes';
import { SiteService } from './site/site.service';
import { VersionService } from './version.service';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(
      routes,
      // Real paths, not a hash: the language and the documentation version are
      // in the address, and an address is what gets sent, indexed and
      // bookmarked. GitHub Pages serves the shell for any path through a
      // file written for every page - see scripts/static-pages.mjs.
      withInMemoryScrolling({ scrollPositionRestoration: 'top', anchorScrolling: 'enabled' }),
    ),
    // The release tag, resolved at build time, so the snippets show a pinned
    // image rather than the moving :latest.
    provideAppInitializer(() => inject(VersionService).load()),
    provideAppInitializer(() => inject(SiteService).applyScheme()),
  ],
};
