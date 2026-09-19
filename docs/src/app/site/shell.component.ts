import { Component, computed, effect, inject, signal, CUSTOM_ELEMENTS_SCHEMA } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatIconModule, MatIconRegistry } from '@angular/material/icon';
import { DomSanitizer } from '@angular/platform-browser';
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
import { Hit, Nav, NavArea, Scheme, Shot, SiteService } from './site.service';
import { VersionService } from '../version.service';

const MEERKAT_MARK = `
<svg viewBox="0 0 44 64" xmlns="http://www.w3.org/2000/svg">
  <path d="M29 43c8.2 1.8 11.4 8.8 8.1 17.1-.4 1-1.9 1-2.5.1-1.8-2.6-2.4-5.2-2.4-7.9 0-3.7-1.5-7.1-4.4-9.4z" fill="currentColor" opacity=".85"/>
  <path d="M22 2c-4.8 0-8.6 3.8-8.6 8.6 0 2.5 1 4.7 2.7 6.3-3.5 3-5.7 7.9-5.7 14.6 0 13 5.1 23.1 11.6 23.1s11.6-10.1 11.6-23.1c0-6.7-2.2-11.6-5.7-14.6 1.7-1.6 2.7-3.8 2.7-6.3C30.6 5.8 26.8 2 22 2z" fill="currentColor"/>
  <circle cx="15.4" cy="6.4" r="3.1" fill="currentColor"/>
  <circle cx="28.6" cy="6.4" r="3.1" fill="currentColor"/>
  <ellipse cx="18.3" cy="10.3" rx="1.8" ry="2.5" fill="var(--meerkat-eye)"/>
  <ellipse cx="25.7" cy="10.3" rx="1.8" ry="2.5" fill="var(--meerkat-eye)"/>
  <path d="M22 14l2.5 2.1-2.5 1.3-2.5-1.3z" fill="var(--meerkat-eye)"/>
</svg>`;

// An area owns an address when one of its prefixes is the whole slug or its
// first segments. `index` is a prefix like any other: it owns the home page
// and nothing else.
const owns = (area: { prefixes: string[] }, slug: string): boolean =>
  area.prefixes.some((p) => slug === p || slug.startsWith(`${p}/`));

// The frame every page is read in: the Softwarity bar on top, the rail down
// the left, and the page itself in between.
//
// ONE menu, not two, and ONE entry lit at a time. The rail says which part of
// the site you are in - the product, the gallery, the documentation, the
// company, the project - and a click goes to its first page. What is INSIDE
// that part is listed by the page itself, in the column beside it: see
// page.component. The home page is not one of those parts: it is the Meerkat
// button in the bar, and the rail is dark while it is read.
//
// There was a contextual drawer on these entries, and it had to go. The
// library opens it on click as well as on hover, which is the only path a
// touch device has; ours also navigate, so the drawer stayed open on top of
// the page just reached - on top of that page's own column, which is how one
// menu came to look like two stacked on each other.
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
    // The trailing slash is GitHub Pages': it answers /en with a 301 to /en/,
    // because every page of this site is a directory holding an index.html.
    const rest = path.split('/').slice(2).join('/').replace(/\/$/, '');
    return rest || 'index';
  });

  protected readonly nav = signal<Nav | null>(null);
  private readonly allAreas = computed<NavArea[]>(() => this.nav()?.areas ?? []);
  // The rail shows the areas that have an entry. The home page has an area of
  // its own without one: it is reached by the Meerkat button in the bar.
  protected readonly areas = computed<NavArea[]>(() => this.allAreas().filter((a) => !a.hidden));

  // ONE entry is lit, or none. Which addresses an entry owns is declared in
  // content/areas.json and carried by nav.json - the shell does not guess it,
  // and the page beside it reads the same list.
  protected readonly currentArea = computed(() => {
    const slug = this.slug();
    return this.allAreas().find((a) => owns(a, slug))?.id ?? '';
  });
  protected readonly atHome = computed(() => this.currentArea() === 'home');

  // The release the site describes, resolved at build time. There is no chip
  // before the first release: "v0.0.0" says less than nothing.
  private readonly version = inject(VersionService);
  protected readonly tag = computed(() => {
    const v = this.version.tag();
    return v && v !== '0.0.0' ? v : '';
  });

  // The picture viewer. The page being read finds the pictures and hands them
  // to the service; the overlay is drawn HERE, at the shell's root, because
  // the rail's container is a stacking context and nothing inside it can be
  // painted over the bar - the search panel is next door for the same reason.
  protected readonly shots = this.site.shots;
  protected readonly at = this.site.at;
  protected readonly shot = computed<Shot | null>(() => this.shots()[this.at()] ?? null);

  protected readonly searchOpen = signal(false);
  protected readonly query = signal('');
  protected readonly hits = signal<Hit[]>([]);

  constructor(iconRegistry: MatIconRegistry, sanitizer: DomSanitizer) {
    iconRegistry.setDefaultFontSetClass('material-symbols-outlined');
    // The product's own mark, for the rail entry that is the product. The bar
    // above stays Softwarity: the company is the brand, this is one of the
    // things it makes.
    iconRegistry.addSvgIconLiteral('meerkat', sanitizer.bypassSecurityTrustHtml(MEERKAT_MARK));
    effect(() => {
      const lang = this.lang();
      document.documentElement.lang = lang;
      void this.site.nav(lang).then((nav) => this.nav.set(nav));
    });

  }

  // "/" opens the search the way it does everywhere else, and Escape closes
  // it. Both are ignored while the reader is typing in a field.
  protected onKey(event: KeyboardEvent): void {
    // The viewer is above the search panel and answers Escape itself.
    if (this.at() >= 0) return;
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

  // An area names either a Material symbol or, with an svg: prefix, one of the
  // marks registered above.
  protected svgIcon(area: NavArea): string {
    return area.icon.startsWith('svg:') ? area.icon.slice(4) : '';
  }

  protected areaLabel(id: string): string {
    return this.allAreas().find((a) => a.id === id)?.label ?? '';
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

  protected closeShot(): void {
    this.site.closeShot();
  }

  protected stepShot(by: number): void {
    this.site.stepShot(by);
  }

  protected onZoomKey(event: KeyboardEvent): void {
    if (this.at() < 0) return;
    if (event.key === 'Escape') this.closeShot();
    else if (event.key === 'ArrowLeft') this.stepShot(-1);
    else if (event.key === 'ArrowRight') this.stepShot(1);
    else return;
    event.preventDefault();
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
