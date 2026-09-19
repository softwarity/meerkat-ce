import { Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { SiteService } from './site.service';

// The door for an address with no language in it: / , /docs/..., an old link.
// It keeps the path and prepends the language the reader asked for, so a
// bookmark made before languages existed still lands on its page.
@Component({
  selector: 'app-enter',
  template: '',
})
export class EnterComponent {
  constructor() {
    const site = inject(SiteService);
    const router = inject(Router);
    const lang = site.preferredLang();
    const path = router.url.split('?')[0].split('#')[0].replace(/^\//, '');
    void router.navigateByUrl(`/${lang}${path ? `/${path}` : ''}`, { replaceUrl: true });
  }
}
