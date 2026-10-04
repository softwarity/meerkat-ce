import { Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { LANG_PARAM } from './site.service';

// The door for an address written when the language was its first segment:
// /fr/docs/start/install. Those addresses were sent, indexed and bookmarked,
// so they keep opening - on the same page, in the language they named, at the
// address that page has now (/docs/start/install?lg=fr).
@Component({
  selector: 'app-enter',
  template: '',
})
export class EnterComponent {
  constructor() {
    const router = inject(Router);
    const tree = router.parseUrl(router.url);
    const [lang, ...rest] = (tree.root.children['primary']?.segments ?? []).map((s) => s.path);
    void router.navigate(['/', ...rest], {
      queryParams: { ...tree.queryParams, [LANG_PARAM]: lang },
      replaceUrl: true,
    });
  }
}
