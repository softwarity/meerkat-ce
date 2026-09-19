import { Component, computed, effect, inject, signal, CUSTOM_ELEMENTS_SCHEMA } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatIconModule, MatIconRegistry } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { NavigationEnd, Router, RouterLink, RouterOutlet } from '@angular/router';
import {
  RailnavComponent,
  RailnavContainerComponent,
  RailnavContentComponent,
  RailnavItemComponent,
  RailnavSpacerComponent,
} from '@softwarity/rail-nav';
import { filter, map, startWith } from 'rxjs';
import { Lang, LANG_NAMES, LANGS, WORDS } from './i18n';
import { Hit, Nav, NavArea, Scheme, SiteService } from './site.service';
import { VersionService } from '../version.service';

// The frame every page is read in: the Softwarity bar on top, the rail down
// the left, and the page itself in between.
//
// ONE level of site navigation, not three. The rail says which part of the
// site you are in - the product, the gallery, the documentation, the company,
// the project. A contextual drawer on a rail item lists what is inside it,
// without leaving the page you are reading. Everything finer than that -
// which page of a section, which heading of a page - belongs to the page and
// lives there, see page.component.
@Component({
  selector: 'app-shell',
  imports: [
    RouterOutlet,
    RouterLink,
    MatIconModule,
    MatMenuModule,
    RailnavComponent,
    RailnavContainerComponent,
    RailnavContentComponent,
    RailnavItemComponent,
    RailnavSpacerComponent,
  ],
  schemas: [CUSTOM_ELEMENTS_SCHEMA],
  templateUrl: './shell.component.html',
  styleUrl: './shell.component.scss',
})
export class ShellComponent {
  private readonly site = inject(SiteService);
  private readonly router = inject(Router);

  protected readonly langs = LANGS;
  protected readonly langNames = LANG_NAMES;
  protected readonly schemes: Scheme[] = ['system', 'light', 'dark'];
  protected readonly scheme = this.site.scheme;

  // The address is the truth: language and page both come from it, so a link
  // someone was sent opens on what it says.
  private readonly url = toSignal(
    this.router.events.pipe(
      filter((e): e is NavigationEnd => e instanceof NavigationEnd),
      map((e) => e.urlAfterRedirects),
      startWith(this.router.url),
    ),
    { initialValue: this.router.url },
  );

  protected readonly lang = computed<Lang>(() => {
    const first = this.url().split('?')[0].split('/')[1];
    return (LANGS as readonly string[]).includes(first) ? (first as Lang) : 'en';
  });
  protected readonly words = computed(() => WORDS[this.lang()]);

  protected readonly slug = computed(() => {
    const path = this.url().split('?')[0].split('#')[0];
    const rest = path.split('/').slice(2).join('/');
    return rest || 'index';
  });

  protected readonly nav = signal<Nav | null>(null);
  protected readonly areas = computed<NavArea[]>(() => this.nav()?.areas ?? []);
  protected readonly currentArea = computed(() => {
    const slug = this.slug();
    for (const area of this.areas()) {
      if (area.id === 'meerkat' && (slug === 'index' || slug.startsWith('product/'))) return area.id;
      if (slug === area.id || slug.startsWith(`${area.id}/`)) return area.id;
    }
    return 'meerkat';
  });

  // One drawer template for the whole rail: the item being pointed at sets
  // this, and the template follows. The library re-targets the same overlay
  // when the cursor moves from one item to the next, so a second template per
  // area would only be a second copy of the same markup.
  protected readonly drawerArea = signal<NavArea | null>(null);

  // The release the site describes, resolved at build time. There is no chip
  // before the first release: "v0.0.0" says less than nothing.
  private readonly version = inject(VersionService);
  protected readonly tag = computed(() => {
    const v = this.version.tag();
    return v && v !== '0.0.0' ? v : '';
  });

  protected readonly searchOpen = signal(false);
  protected readonly query = signal('');
  protected readonly hits = signal<Hit[]>([]);

  constructor(iconRegistry: MatIconRegistry) {
    iconRegistry.setDefaultFontSetClass('material-symbols-outlined');
    effect(() => {
      const lang = this.lang();
      this.site.rememberLang(lang);
      document.documentElement.lang = lang;
      void this.site.nav(lang).then((nav) => this.nav.set(nav));
    });
  }

  // A rail entry opens a drawer when there is something to list in it: a
  // single page is reached by clicking the entry itself.
  protected hasDrawer(area: NavArea): boolean {
    const pages = area.sections.reduce((n, s) => n + s.pages.length, 0);
    return pages > 1;
  }

  // "/" opens the search the way it does everywhere else, and Escape closes
  // it. Both are ignored while the reader is typing in a field.
  protected onKey(event: KeyboardEvent): void {
    if (event.key === 'Escape' && this.searchOpen()) {
      this.closeSearch();
      return;
    }
    const target = event.target as HTMLElement | null;
    const typing = !!target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName);
    if (typing || event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === '/') {
      event.preventDefault();
      this.openSearch();
    }
  }

  // Listed by section rather than page to page. The line is where a list stops
  // being a menu: four sections is a menu, eleven times ten pages is a book.
  protected bySection(area: NavArea): boolean {
    return area.sections.length > 3;
  }

  protected areaLabel(id: string): string {
    return this.areas().find((a) => a.id === id)?.label ?? '';
  }

  protected path(slug: string): string {
    return this.site.pathFor(this.lang(), slug);
  }

  // The same page in the other language. Content is written page for page in
  // both, so the address holds - and when it does not, the language's home is
  // a better answer than a dead link.
  protected async switchLang(lang: Lang): Promise<void> {
    const slug = this.slug();
    this.site.rememberLang(lang);
    try {
      await this.site.page(lang, slug);
      void this.router.navigateByUrl(this.site.pathFor(lang, slug));
    } catch {
      void this.router.navigateByUrl(`/${lang}`);
    }
  }

  protected setScheme(scheme: Scheme): void {
    this.site.setScheme(scheme);
  }

  protected openSearch(): void {
    this.searchOpen.set(true);
    setTimeout(() => document.getElementById('site-search')?.focus(), 0);
  }

  protected closeSearch(): void {
    this.searchOpen.set(false);
    this.query.set('');
    this.hits.set([]);
  }

  protected onQuery(value: string): void {
    this.query.set(value);
    void this.runSearch(value);
  }

  protected async goto(slug: string): Promise<void> {
    this.closeSearch();
    void this.router.navigateByUrl(this.path(slug));
  }

  private async runSearch(value: string): Promise<void> {
    const nav = this.nav();
    if (!nav) return;
    const version = this.site.versionOf(this.slug(), nav);
    const hits = await this.site.search(this.lang(), version, nav, value);
    // The reader may have typed on while this was in flight.
    if (this.query() === value) this.hits.set(hits);
  }
}
