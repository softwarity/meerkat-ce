import { Component, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { NavigationEnd, Router, RouterLink } from '@angular/router';
import { filter, map, startWith } from 'rxjs';
import { Lang, LANGS, WORDS } from './i18n';
import { Nav, NavArea, NavSection, Page, Shot, SiteService } from './site.service';
import { BenchmarkComponent } from './widgets/benchmark.component';

// One page: what the build made of a Markdown file, plus the two navigations
// that belong to the page rather than to the site - the other pages of its
// section, down the left, and its own headings, down the right.
//
// A page declaring `layout: wide` in its front matter gets neither: a landing
// page is a composition, and a column of links beside it would be furniture.
@Component({
  selector: 'app-page',
  imports: [RouterLink, MatIconModule, MatMenuModule, BenchmarkComponent],
  templateUrl: './page.component.html',
  styleUrl: './page.component.scss',
})
export class PageComponent {
  private readonly site = inject(SiteService);
  private readonly router = inject(Router);
  private readonly sanitizer = inject(DomSanitizer);
  private readonly body = viewChild<ElementRef<HTMLElement>>('body');

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
    // See the shell: /en/ and /en are the same page, and Pages serves the first.
    return path.split('/').slice(2).join('/').replace(/\/$/, '') || 'index';
  });

  protected readonly page = signal<Page | null>(null);
  protected readonly missing = signal(false);
  protected readonly nav = signal<Nav | null>(null);
  protected readonly sections = signal<NavSection[]>([]);

  protected readonly wide = computed(() => this.page()?.layout === 'wide');


  // bypassSecurityTrustHtml is safe HERE and would not be anywhere else: this
  // HTML is not user input, it is the output of scripts/build-site.mjs over
  // Markdown kept in this repository - the same trust one gives a component's
  // own template.
  protected readonly html = computed<SafeHtml>(() =>
    this.sanitizer.bypassSecurityTrustHtml(this.page()?.html ?? ''),
  );

  // The version being read, and the one the site calls current. They differ
  // exactly when a banner is owed to the reader.
  protected readonly version = computed(() => {
    const nav = this.nav();
    return nav ? this.site.versionOf(this.slug(), nav) : '';
  });
  protected readonly isDocs = computed(() => this.slug().startsWith('docs'));
  protected readonly versions = computed(() => this.nav()?.docs.versions ?? []);
  protected readonly current = computed(() => this.nav()?.docs.current ?? '');
  protected readonly stale = computed(() => this.isDocs() && !!this.version() && this.version() !== this.current());

  // The pages beside this one: its own section in the documentation, where a
  // list of all hundred would be a phone book; the whole area everywhere else,
  // where there are five.
  protected readonly siblings = computed<NavSection[]>(() => {
    const page = this.page();
    if (!page || this.wide()) return [];
    return this.sections();
  });

  // Which section the page being read belongs to. The others are shown by
  // name only - a hundred and twenty entries at once is a phone book, and the
  // rail no longer opens a drawer that could hold them.
  protected readonly openSection = computed(() => {
    const page = this.page();
    if (!page) return '';
    return this.sections().find((s) => s.pages.some((p) => p.slug === page.slug))?.name ?? '';
  });

  constructor() {
    effect(() => {
      // BOTH read here, synchronously, or the effect does not depend on them.
      // A signal read inside the `then` below runs after the effect has
      // finished and is never registered: the column was then computed once,
      // on the first page loaded, and every click that followed kept it - the
      // documentation's sections stayed up while the reader was in the
      // product, and the other way round.
      const lang = this.lang();
      const slug = this.slug();
      void this.site.nav(lang).then(async (nav) => {
        this.nav.set(nav);
        const sections = slug.startsWith('docs')
          ? await this.site.sectionsFor(lang, nav, this.site.versionOf(slug, nav))
          : (nav.areas.find((a) => this.inArea(a, slug))?.sections ?? []);
        // The reader may have moved on while this was in flight.
        if (this.slug() === slug) this.sections.set(sections);
      });
    });

    effect(() => {
      const lang = this.lang();
      const slug = this.slug();
      this.missing.set(false);
      void this.site
        .page(lang, slug)
        .then((p) => {
          this.page.set(p);
          // The company is the brand, Meerkat is the product: a page about the
          // product says so in the tab, a page about the company says the company.
          const house = p.area === 'softwarity' || p.area === 'project' ? 'Softwarity' : 'Meerkat';
          document.title = slug === 'index' ? `${p.title} | Softwarity` : `${p.title} | ${house}`;
          const meta = document.querySelector('meta[name="description"]');
          if (meta && p.summary) meta.setAttribute('content', p.summary);
          // The anchors the build wrote point at pages, not at addresses -
          // only the runtime knows the language and the version. Fill them in
          // once the HTML is in the document.
          setTimeout(() => {
            this.wireLinks();
            this.wireShots();
            // A deep link may name a heading (?at=the-heading): the path is
            // the page, so the anchor could not ride in the fragment.
            const at = new URL(location.href).searchParams.get('at');
            if (at) this.jump(at);
          }, 0);
        })
        .catch(() => {
          this.page.set(null);
          this.missing.set(true);
        });
    });
  }

  protected path(slug: string): string {
    return this.site.pathFor(this.lang(), slug);
  }

  protected homePath(): string {
    return `/${this.lang()}`;
  }

  protected label(version: string): string {
    return version === 'next' ? this.words().versionNext : version;
  }

  protected async goVersion(version: string): Promise<void> {
    const nav = this.nav();
    if (!nav) return;
    const target = await this.site.sameIn(this.lang(), this.slug(), nav, version);
    void this.router.navigateByUrl(this.path(target));
  }

  // What a reader sees only on paper: the address, so the document can be
  // found again after it has been forwarded twice.
  protected here(): string {
    return typeof location === 'undefined' ? '' : location.href.split('?')[0];
  }

  protected print(): void {
    window.print();
  }

  protected jump(id: string): void {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  // Which addresses an area owns is declared once, in content/areas.json, and
  // carried by nav.json. The shell reads the same list to light one entry of
  // the rail, so the column beside the page and the rail can never disagree.
  private inArea(area: NavArea, slug: string): boolean {
    return area.prefixes.some((p) => slug === p || slug.startsWith(`${p}/`));
  }

  // Every picture of the page, in document order, made to open on a click. A
  // screenshot at a third of the screen is a thumbnail whatever we call it.
  // The drawings travel as markup: they are inline SVG, and they need the
  // figure's class around them or they lose the colours it declares.
  private wireShots(): void {
    const root = this.body()?.nativeElement;
    if (!root) {
      this.site.setShots([]);
      return;
    }
    const found: Shot[] = [];
    const nodes = Array.from(root.querySelectorAll<HTMLElement>('img, .mk-figure svg'));
    for (const node of nodes) {
      const caption = node.closest('figure')?.querySelector('figcaption')?.textContent?.trim() ?? '';
      const index = found.length;
      if (node instanceof HTMLImageElement) {
        found.push({ src: node.currentSrc || node.src, svg: null, alt: node.alt, caption });
      } else {
        const label = node.getAttribute('aria-label') ?? '';
        found.push({
          src: '',
          svg: this.sanitizer.bypassSecurityTrustHtml(node.outerHTML),
          alt: label,
          caption: caption || label,
        });
      }
      node.classList.add('zoomable');
      node.addEventListener('click', () => this.site.openShot(index));
    }
    this.site.setShots(found);
  }

  // A link in the Markdown names a page. Turn each into a real href - so it
  // can be opened in a new tab, copied, or crawled - and route it on click so
  // the site does not reload itself.
  private wireLinks(): void {
    const root = this.body()?.nativeElement;
    if (!root) return;
    const nav = this.nav();
    const version = nav ? this.site.versionOf(this.slug(), nav) : '';
    const prefix = nav?.docs.versions.find((v) => v.id === version)?.prefix ?? '';
    for (const a of Array.from(root.querySelectorAll<HTMLAnchorElement>('a[data-page]'))) {
      let target = a.dataset['page'] || '';
      // A link written in an older version's page stays in that version.
      if (prefix && target.startsWith('docs/') && !target.startsWith(`docs/${prefix}/`)) {
        target = target.replace('docs/', `docs/${prefix}/`);
      }
      const at = a.dataset['at'];
      const url = this.path(target) + (at ? `?at=${at}` : '');
      a.setAttribute('href', this.router.serializeUrl(this.router.parseUrl(url)));
      a.addEventListener('click', (event) => {
        if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return;
        event.preventDefault();
        void this.router.navigateByUrl(url);
      });
    }
    // An anchor inside the page scrolls instead of navigating.
    for (const a of Array.from(root.querySelectorAll<HTMLAnchorElement>('a[data-at]:not([data-page])'))) {
      a.addEventListener('click', (event) => {
        event.preventDefault();
        this.jump(a.dataset['at'] || '');
      });
    }
  }
}
