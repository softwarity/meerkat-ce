import {
  Component,
  ElementRef,
  computed,
  effect,
  inject,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';
import { Background, LogoSize, PageLayout, ThemeFonts } from '../../api.service';
import { cssVar } from '../theme-tokens';

// The live preview: the gateway-rendered flow-page specimen, dark and light
// SIDE BY SIDE, each scaled from its logical 1280×800 viewport to fit - the
// point is to see the whole page. Palette/branding edits and token highlights
// are pushed into both frames over postMessage, no reload.
@Component({
  selector: 'app-theme-preview',
  templateUrl: './theme-preview.component.html',
  styleUrl: './theme-preview.component.scss',
})
export class ThemePreviewComponent {
  readonly themeId = input.required<string>();
  readonly version = input.required<number>();
  readonly dark = input.required<Record<string, string>>();
  readonly light = input.required<Record<string, string>>();
  readonly brandName = input.required<string>();
  readonly brandTagline = input.required<string>();
  readonly brandLogo = input.required<string>();
  // The size the mark is drawn at, pushed like the rest: the frame turns it
  // into a class, so trying one is not a reload.
  readonly brandLogoSize = input<LogoSize>('');
  // The background as it is being edited: the console holds the picked image
  // long before it is saved, so the panes show it from the drop.
  readonly background = input<Background>({});
  readonly flat = input(false); // flat design -> --mk-glow 0, effects off
  // The typefaces, by family name: the frame maps a name to the stack the
  // gateway wrote into it, so only a family it ships can be set.
  readonly fonts = input<ThemeFonts>({});
  // Which template the panes render. "specimen" is the flow-page composite;
  // "mail:<kind>" is a sample message, and those have no dark half - a mail is
  // built from light colours inline, because an e-mail client second-guesses a
  // dark background. One pane there is the truth, not a degraded preview.
  readonly template = input('specimen');
  readonly locale = input('en');
  // Stack every refusal the page can give, rather than one. The Locale tab
  // asks for it - the wordings are what it is about - and the other tabs do
  // not: four red boxes stand in front of an arrangement one is trying to
  // judge, while one of them is the error colour the palette needs on screen.
  readonly allErrors = input(false);
  protected readonly isMail = computed(() => this.template().startsWith('mail:'));
  // The CSS vars to blink: one role, or every token a source colour drives.
  readonly highlight = input<string[]>([]);
  // The colours on screen and not saved, as the gateway's preview takes them.
  // A page takes colours live over postMessage; a mail (inline styles) and
  // the portal bar (a shadow DOM fed a payload) cannot, so THEIR frame is
  // reloaded with the draft - already debounced by the scope.
  readonly draft = input('');
  private readonly reloads = computed(() => this.isMail() || this.template().startsWith('portal:'));
  // Which schemes the built-in pages offer, read-only here: the pane of a
  // scheme nobody will be served is dimmed, so the screen never shows a look
  // that cannot happen.
  readonly pagesScheme = input<'' | 'light' | 'dark'>('');
  // The arrangement being tried. The specimen carries every layout's CSS, so
  // this travels as a class to swap - not as a URL to reload, which would
  // blank the panes at each candidate, exactly while they are being compared.
  readonly layout = input<PageLayout>({ name: 'centered' });
  protected readonly darkOffered = computed(() => !this.isMail() && this.pagesScheme() !== 'light');
  protected readonly lightOffered = computed(() => this.isMail() || this.pagesScheme() !== 'dark');

  private readonly sanitizer = inject(DomSanitizer);
  private readonly frames = new Set<HTMLIFrameElement>();

  private readonly hostWidth = signal(0);
  private readonly host = viewChild<ElementRef<HTMLDivElement>>('duo');
  private observed?: HTMLDivElement;

  // Stacked panes, deliberately kept SCALED DOWN (capped width): the point is
  // an at-a-glance look, not a full-size page.
  protected readonly scale = computed(() => {
    const pane = Math.min(this.hostWidth(), 860);
    return pane > 0 ? pane / 1280 : 0.3;
  });
  protected readonly paneWidth = computed(() => Math.round(1280 * this.scale()));
  protected readonly paneHeight = computed(() => Math.round(640 * this.scale()));

  // No dark half for a mail: not dimmed, ABSENT. A scheme the pages do not
  // offer is still a real page, so it stays on screen greyed; a dark mail
  // is not a thing that exists, and showing one greyed would claim it is.
  protected readonly darkUrl = computed(() => (this.isMail() ? null : this.url('dark')));
  protected readonly lightUrl = computed(() => this.url('light'));

  constructor() {
    effect(() => {
      const el = this.host()?.nativeElement;
      if (!el || el === this.observed) return;
      this.observed = el;
      this.hostWidth.set(el.clientWidth);
      new ResizeObserver(() => this.hostWidth.set(el.clientWidth)).observe(el);
    });
    effect(() =>
      this.push(
        this.dark(),
        this.light(),
        this.brandName(),
        this.brandTagline(),
        this.brandLogo(),
        this.brandLogoSize(),
        this.flat(),
        this.background(),
        this.layout(),
        this.fonts(),
      ),
    );
    effect(() => this.pushHighlight(this.highlight()));
  }

  protected frameReady(ev: Event): void {
    this.frames.add(ev.target as HTMLIFrameElement);
    this.push(
      this.dark(),
      this.light(),
      this.brandName(),
      this.brandTagline(),
      this.brandLogo(),
      this.brandLogoSize(),
      this.flat(),
      this.background(),
      this.layout(),
      this.fonts(),
    );
  }

  private url(scheme: 'dark' | 'light'): SafeResourceUrl | null {
    const id = this.themeId();
    if (!id) return null;
    const q = new URLSearchParams({
      scheme,
      v: String(this.version()),
      template: this.template(),
      locale: this.locale(),
    });
    if (this.allErrors()) q.set('errors', 'all');
    if (this.reloads() && this.draft()) q.set('draft', this.draft());
    const raw = `/api/themes/${encodeURIComponent(id)}/preview?${q}`;
    return this.sanitizer.bypassSecurityTrustResourceUrl(raw);
  }

  private post(message: Record<string, unknown>): void {
    for (const frame of this.frames) {
      if (!frame.isConnected) {
        this.frames.delete(frame);
        continue;
      }
      frame.contentWindow?.postMessage({ type: 'meerkat-theme', ...message }, location.origin);
    }
  }

  private push(
    dark: Record<string, string>,
    light: Record<string, string>,
    name: string,
    tagline: string,
    logo: string,
    logoSize: LogoSize,
    flat: boolean,
    background: Background,
    layout: PageLayout,
    fonts: ThemeFonts,
  ): void {
    const vars: Record<string, string> = {};
    for (const key of Object.keys(light)) {
      // Palettes come complete; a token missing on one side is left to the
      // frame's current value rather than invented.
      if (!light[key] || !dark[key]) continue;
      vars[cssVar(key)] = `light-dark(${light[key]}, ${dark[key]})`;
    }
    // The flat-design switch: 0 collapses every decorative effect at once.
    vars['--mk-glow'] = flat ? '0' : '1';
    this.post({
      vars,
      brand: { name, tagline, logo, logoSize },
      background,
      layout,
      fonts: { display: fonts.display ?? '', body: fonts.body ?? '', code: fonts.code ?? '' },
    });
  }

  private pushHighlight(vars: string[]): void {
    this.post({ highlight: vars });
  }
}
