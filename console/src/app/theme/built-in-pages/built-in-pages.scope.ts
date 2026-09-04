import { computed, inject, Injectable, signal } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ApiService, Background, LogoSize, PageLayout, Settings, Theme } from '../../api.service';
import { CSS_VARS } from '../theme-tokens';

type Fit = 'cover' | 'contain' | 'tile';

// Shared state between the Built-in pages layout and its three tabs, on the
// model of TenantScope. Provided by BuiltInPagesComponent - one instance per
// visit.
//
// It exists because the PREVIEW is one and the tabs are three. The colours,
// the arrangement and the identity all describe the same page, they are
// pushed into the same two frames, and the frames must survive a tab change:
// three components each holding their own copy of the theme would each start
// by re-fetching it, and the preview would blink on every click.
@Injectable()
export class BuiltInPagesScope {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  readonly loading = signal(true);
  readonly saving = signal(false);
  // Bumped after every write: the frames re-fetch what the gateway now serves
  // instead of keeping the copy that was pushed into them.
  readonly version = signal(0);

  // ── themes ────────────────────────────────────────────────────────────────
  readonly themes = signal<Theme[]>([]);
  readonly presets = signal<Theme[]>([]);
  readonly selectedId = signal('');
  readonly name = signal('');
  readonly flat = signal(false);
  readonly dark = signal<Record<string, string>>({});
  readonly light = signal<Record<string, string>>({});
  readonly selected = computed(() => this.themes().find((t) => t.id === this.selectedId()) ?? null);

  // Hovered token (Theme tab) -> the CSS var the preview blinks.
  private readonly hoverKey = signal('');
  readonly highlightVar = computed(() => (this.hoverKey() ? CSS_VARS[this.hoverKey()] : ''));

  // ── branding ──────────────────────────────────────────────────────────────
  readonly appName = signal('');
  readonly tagline = signal('');
  readonly logo = signal('');
  readonly logoSize = signal<LogoSize>('');
  readonly favicon = signal('');
  readonly background = signal('');
  readonly backgroundFit = signal<Fit>('cover');
  readonly backgroundDim = signal(0);
  readonly hideMark = signal(false);
  readonly bg = computed<Background>(() => ({
    image: this.background(),
    fit: this.backgroundFit(),
    dim: this.backgroundDim(),
  }));

  // ── the pages' own settings ───────────────────────────────────────────────
  // Both ride on /api/settings, whose PUT takes the WHOLE payload: the loaded
  // object is kept to send back with one field changed, or a partial body
  // would quietly reset the rest.
  readonly pagesScheme = signal<'' | 'light' | 'dark'>('');
  readonly layout = signal<PageLayout>({ name: 'centered' });
  private settings: Settings | null = null;

  constructor() {
    this.loadThemes();
    this.api.listPresets().subscribe({ next: (p) => this.presets.set(p) });
    this.api.settings().subscribe({
      next: (s) => {
        this.settings = s;
        this.pagesScheme.set(s.pagesScheme ?? '');
        this.layout.set(s.pageLayout ?? { name: 'centered' });
      },
      error: () => undefined,
    });
    this.api.branding().subscribe({
      next: (b) => {
        this.appName.set(b.appName);
        this.tagline.set(b.tagline);
        this.logo.set(b.logo);
        this.logoSize.set(b.logoSize ?? '');
        this.favicon.set(b.favicon ?? '');
        this.background.set(b.background?.image ?? '');
        this.backgroundFit.set(b.background?.fit ?? 'cover');
        this.backgroundDim.set(b.background?.dim ?? 0);
        this.hideMark.set(b.hideMark ?? false);
      },
    });
  }

  hover(key: string): void {
    this.hoverKey.set(key);
  }

  // ── themes ────────────────────────────────────────────────────────────────

  select(t: Theme): void {
    this.selectedId.set(t.id);
    this.name.set(t.name);
    this.flat.set(!!t.flat);
    this.dark.set({ ...t.dark });
    this.light.set({ ...t.light });
  }

  loadThemes(keepSelection = false): void {
    this.loading.set(true);
    this.api.listThemes().subscribe({
      next: (themes) => {
        this.themes.set(themes);
        const wanted = keepSelection ? this.selectedId() : '';
        const pick = themes.find((t) => t.id === wanted) ?? themes.find((t) => t.active) ?? themes[0];
        if (pick) this.select(pick);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  saveTheme(): void {
    const t = this.selected();
    if (!t) return;
    this.saving.set(true);
    this.api
      .updateTheme({
        ...t,
        name: this.name().trim(),
        flat: this.flat(),
        dark: this.dark(),
        light: this.light(),
      })
      .subscribe({
        next: () => {
          this.saving.set(false);
          this.version.update((v) => v + 1);
          this.loadThemes(true);
        },
        error: (err) => {
          this.saving.set(false);
          this.fail(err);
        },
      });
  }

  activate(t: Theme): void {
    this.api.activateTheme(t.id).subscribe({
      next: () => {
        this.snack.open(
          $localize`:@@Theme_is_now_live:This theme is now live on the flow pages`,
          undefined,
          { duration: 3000 },
        );
        this.loadThemes(true);
      },
      error: (err) => this.fail(err),
    });
  }

  // Derive a new theme from the selected one, auto-named. It inherits the
  // source's createdAt so it sorts right NEXT TO it (ListThemes orders by
  // createdAt then name; "X copy" > "X").
  createFrom(): void {
    const base = this.selected();
    if (!base) return;
    this.create(
      this.uniqueName(`${base.name} copy`),
      { ...this.dark() },
      { ...this.light() },
      this.flat(),
      base.createdAt,
    );
  }

  createFromPreset(p: Theme): void {
    this.create(this.uniqueName(p.name), { ...p.dark }, { ...p.light }, p.flat);
  }

  removeTheme(t: Theme): void {
    this.api.deleteTheme(t.id).subscribe({
      next: () => this.loadThemes(),
      error: (err) => this.fail(err),
    });
  }

  private create(
    name: string,
    dark: Record<string, string>,
    light: Record<string, string>,
    flat = false,
    createdAt?: number,
  ): void {
    this.api.createTheme({ name, dark, light, flat, ...(createdAt ? { createdAt } : {}) }).subscribe({
      next: (created) => {
        this.selectedId.set(created.id);
        this.loadThemes(true);
      },
      error: (err) => this.fail(err),
    });
  }

  private uniqueName(base: string): string {
    const taken = new Set(this.themes().map((t) => t.name));
    if (!taken.has(base)) return base;
    for (let i = 2; ; i++) {
      const candidate = `${base} ${i}`;
      if (!taken.has(candidate)) return candidate;
    }
  }

  // ── branding ──────────────────────────────────────────────────────────────

  private timer?: ReturnType<typeof setTimeout>;

  // Debounced, so typing a name does not send one call per letter. 700ms is
  // long enough to finish a word and short enough that leaving the screen right
  // after typing still saves.
  brandingChanged(): void {
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this.saveBranding(), 700);
  }

  private saveBranding(): void {
    const image = this.background();
    this.api
      .saveBranding({
        appName: this.appName().trim(),
        tagline: this.tagline().trim(),
        logo: this.logo(),
        logoSize: this.logoSize(),
        favicon: this.favicon(),
        // No picture, no framing: the settings that described it would be
        // exported as decisions about something that is not there.
        background: image ? { image, fit: this.backgroundFit(), dim: this.backgroundDim() } : {},
        hideMark: this.hideMark(),
      })
      .subscribe({
        // Silent on success: the preview already showed it.
        next: () => this.version.update((v) => v + 1),
        error: (err) => this.fail(err),
      });
  }

  // The size of the mark is edited on the LAYOUT tab - it is a question about
  // the page, and one arrangement (banner) answers it itself - but it is
  // stored with the branding, where the picture is: it must survive changing
  // arrangement, and travel with the logo in an exported configuration.
  // Written on the click, like the layout beside it: a toggle IS the setting.
  setLogoSize(size: LogoSize): void {
    this.logoSize.set(size);
    this.saveBranding();
  }

  // ── scheme and layout ─────────────────────────────────────────────────────

  // Both are written as soon as they are clicked: a checkbox and a picked
  // thumbnail ARE the setting, so a Save button beside them would be asking
  // twice.
  setPagesScheme(value: '' | 'light' | 'dark'): void {
    this.pagesScheme.set(value);
    this.pushSettings({ pagesScheme: value });
  }

  setLayout(next: PageLayout): void {
    const before = this.layout();
    this.layout.set(next);
    this.pushSettings({ pageLayout: next }, () => this.layout.set(before));
  }

  private pushSettings(patch: Partial<Settings>, rollback?: () => void): void {
    const current = this.settings;
    if (!current) return;
    this.api.saveSettings({ ...current, ...patch }).subscribe({
      next: (s) => {
        this.settings = s;
        this.version.update((v) => v + 1);
      },
      error: (err) => {
        rollback?.();
        this.fail(err);
      },
    });
  }

  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
      undefined,
      { duration: 4000 },
    );
  }
}
