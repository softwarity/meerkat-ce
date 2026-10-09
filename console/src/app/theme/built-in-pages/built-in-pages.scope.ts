import { computed, effect, inject, Injectable, signal } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ApiService, Background, LogoSize, LocaleView, PageLayout, PreviewCategory, PreviewTemplate, Settings, Theme, ThemeColors } from '../../api.service';
import { Contrast, CoreKey, themeSchemes } from '../m3';
import { CORE_DRIVES, cssVar } from '../theme-tokens';
import { PRESET_PREFIX } from '../theme-carousel/theme-carousel.component';

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
  // What the preview renders, and in which language. The catalogue is
  // served: adding a template must not mean editing the console too.
  readonly templates = signal<PreviewTemplate[]>([]);
  readonly categories = signal<PreviewCategory[]>([]);
  // Which categories are being asked for. EMPTY MEANS ALL: nothing is selected
  // until somebody selects something, and selecting nothing is not a way to
  // reach an empty list - it is how one goes back to the whole set. Asking for
  // every category comes to the same thing as asking for none, which is the
  // only reading under which a filter needs no rule about its last toggle.
  readonly only = signal<Set<string>>(new Set());
  readonly shownTemplates = computed(() => {
    const only = this.only();
    return only.size ? this.templates().filter((t) => only.has(t.category)) : this.templates();
  });
  readonly template = signal('');
  readonly previewLocale = signal('en');
  // Every language this gateway can render, with the count of strings it has
  // no wording for. That count is what sends somebody to the editor.
  readonly locales = signal<LocaleView[]>([]);
  readonly selectedId = signal('');
  readonly name = signal('');
  readonly flat = signal(false);
  // What a theme is MADE of (THEME-04): six source colours, a contrast level
  // and the builder's colour match. The palettes are not edited any more,
  // they follow - generated here for the preview, by the generator the
  // gateway runs on save.
  readonly colors = signal<ThemeColors>({ primary: '#6750a4' });
  readonly contrast = signal<Contrast>('standard');
  readonly colorMatch = signal(false);
  // The recipe as it was loaded: what "changed" is measured against.
  private readonly baseline = signal<{ colors: ThemeColors; contrast: Contrast; colorMatch: boolean }>({
    colors: { primary: '' },
    contrast: 'standard',
    colorMatch: false,
  });
  private readonly generated = computed(() => themeSchemes(this.colors(), this.contrast(), this.colorMatch()));
  readonly dark = computed(() => this.generated().dark);
  readonly light = computed(() => this.generated().light);
  // A built-in palette: shown, duplicated, never edited or deleted. Nothing
  // forbids it - there is simply no row to write to. The ring hands it over
  // under a namespaced id, because a copy keeps its source's id and the two
  // must stay two pills.
  readonly readOnly = computed(() => this.selectedId().startsWith(PRESET_PREFIX));

  // The ring carries the presets too, so "selected" has to look in both.
  readonly selected = computed(() => {
    const id = this.selectedId();
    if (id.startsWith(PRESET_PREFIX)) {
      const p = this.presets().find((t) => PRESET_PREFIX + t.id === id);
      return p ? { ...p, id, active: false } : null;
    }
    return this.themes().find((t) => t.id === id) ?? null;
  });

  // What is on screen against what was loaded. Navigating replaces the palette
  // signals, and the arrows now sit two centimetres from the colour pickers -
  // without this, three tweaks and a click on "next" lose the three tweaks
  // without a word.
  readonly dirty = computed(() => {
    const t = this.selected();
    if (!t || this.readOnly()) return false;
    return (
      this.name().trim() !== t.name ||
      this.flat() !== !!t.flat ||
      this.recoloured()
    );
  });

  // The colours on screen differ from the saved ones - which is also when the
  // frames that cannot take colours live (a mail, the portal bar) need them
  // handed over as a draft.
  readonly recoloured = computed(() => {
    const b = this.baseline();
    const a = normalized(this.colors()), c = normalized(b.colors);
    return (
      KEYS.some((k) => (a[k] ?? '') !== (c[k] ?? '')) ||
      this.contrast() !== b.contrast ||
      this.colorMatch() !== b.colorMatch
    );
  });

  // The unsaved recipe, for the frames that are reloaded rather than painted
  // live. Settles a quarter second after the last change: a dragged colour
  // picker fires dozens of values, and each would be a page load.
  readonly draft = signal('');
  private draftTimer?: ReturnType<typeof setTimeout>;
  private readonly draftWatch = effect(() => {
    const flatMoved = this.flat() !== !!this.selected()?.flat;
    const value = this.recoloured() || flatMoved
      ? JSON.stringify({ colors: normalized(this.colors()), contrast: this.contrast(), colorMatch: this.colorMatch(), flat: this.flat() })
      : '';
    clearTimeout(this.draftTimer);
    if (!value) {
      this.draft.set('');
      return;
    }
    this.draftTimer = setTimeout(() => this.draft.set(value), 250);
  });

  // Hovered token or source colour (Theme tab) -> the CSS vars the preview
  // blinks: a role is one, a source colour is everything it drives.
  private readonly hoverKey = signal('');
  readonly highlightVar = computed<string[]>(() => {
    const k = this.hoverKey();
    if (!k) return [];
    const drives = CORE_DRIVES[k as CoreKey];
    return (drives ?? [k]).map(cssVar);
  });

  // A file read in: the recipe replaces what is on screen, and is saved only
  // when the operator says so.
  loadRecipe(colors: ThemeColors, colorMatch: boolean, contrast?: Contrast): void {
    this.colors.set(normalized(colors));
    this.colorMatch.set(colorMatch);
    if (contrast) this.contrast.set(contrast);
  }

  // ── branding ──────────────────────────────────────────────────────────────
  readonly appName = signal('');
  readonly tagline = signal('');
  readonly logo = signal('');
  readonly logoSize = signal<LogoSize>('');
  readonly favicon = signal('');
  readonly background = signal('');
  readonly backgroundFit = signal<Fit>('cover');
  readonly backgroundDim = signal(0);
  // The dark scheme's own picture, used when backgroundBoth is off (THEME-06).
  readonly backgroundBoth = signal(true);
  readonly backgroundDark = signal('');
  readonly backgroundFitDark = signal<Fit>('cover');
  readonly backgroundDimDark = signal(0);
  readonly hideMark = signal(false);
  readonly bg = computed<Background>(() => ({
    image: this.background(),
    fit: this.backgroundFit(),
    dim: this.backgroundDim(),
    both: this.backgroundBoth(),
    imageDark: this.backgroundDark(),
    fitDark: this.backgroundFitDark(),
    dimDark: this.backgroundDimDark(),
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
    this.reloadLocales();
    this.api.previewTemplates().subscribe({
      next: (c) => {
        this.templates.set(c.templates);
        this.categories.set(c.categories);
        // Land on the first one rather than on a name written in the console:
        // the catalogue is served, so what it starts with is its business.
        if (!this.template() && c.templates.length) this.template.set(c.templates[0].key);
      },
    });
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
        // Default to "both" so a single stored picture shows on a dark page too.
        this.backgroundBoth.set(b.background?.both ?? true);
        this.backgroundDark.set(b.background?.imageDark ?? '');
        this.backgroundFitDark.set(b.background?.fitDark ?? 'cover');
        this.backgroundDimDark.set(b.background?.dimDark ?? 0);
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
    const recipe = {
      colors: normalized(t.colors ?? { primary: '#6750a4' }),
      contrast: t.contrast || 'standard',
      colorMatch: !!t.colorMatch,
    };
    this.baseline.set(recipe);
    this.colors.set(recipe.colors);
    this.contrast.set(recipe.contrast);
    this.colorMatch.set(recipe.colorMatch);
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
      .updateTheme({ ...t, name: this.name().trim(), flat: this.flat(), ...this.recipe() })
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
    // A copy of a BUILT-IN takes its name plain: "Forest" rather than "Forest
    // copy", because there is no editable Forest for it to be a copy of.
    const name = this.readOnly() ? this.uniqueName(base.name) : this.uniqueName(`${base.name} copy`);
    this.create({ name, flat: this.flat(), ...this.recipe(), createdAt: base.createdAt });
  }

  // What a save sends: the colours. The palettes are the gateway's to write.
  private recipe(): Partial<Theme> {
    return { colors: normalized(this.colors()), contrast: this.contrast(), colorMatch: this.colorMatch(), dark: {}, light: {} };
  }

  // Delete what is selected, which is the only thing the menu can name. A
  // built-in has no row and the menu item is disabled on it.
  removeSelected(): void {
    const t = this.selected();
    if (t && !this.readOnly() && !t.active) this.removeTheme(t);
  }

  removeTheme(t: Theme): void {
    this.api.deleteTheme(t.id).subscribe({
      next: () => this.loadThemes(),
      error: (err) => this.fail(err),
    });
  }

  private create(theme: Partial<Theme>): void {
    this.api.createTheme(theme).subscribe({
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
    const imageDark = this.backgroundDark();
    this.api
      .saveBranding({
        appName: this.appName().trim(),
        tagline: this.tagline().trim(),
        logo: this.logo(),
        logoSize: this.logoSize(),
        favicon: this.favicon(),
        // No picture, no framing: the settings that described it would be
        // exported as decisions about something that is not there. The server
        // normalizes the rest (clears the dark slot under "both", etc.).
        background:
          image || imageDark
            ? {
                image,
                fit: this.backgroundFit(),
                dim: this.backgroundDim(),
                both: this.backgroundBoth(),
                imageDark,
                fitDark: this.backgroundFitDark(),
                dimDark: this.backgroundDimDark(),
              }
            : {},
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
  // Turning a category off may hide what is on screen; step to the first one
  // still shown rather than leave the panes on something the arrows can no
  // longer reach.
  setCategories(keys: string[]): void {
    this.only.set(new Set(keys));
    const shown = this.shownTemplates();
    if (shown.length && !shown.some((t) => t.key === this.template())) {
      this.template.set(shown[0].key);
    }
  }

  // Re-read the languages: their hole counts and their "edited" marks change
  // every time a wording is saved or reset.
  reloadLocales(): void {
    this.api.locales().subscribe({ next: (l) => this.locales.set(l) });
  }

  // Add a language the binary does not ship. An empty one renders English until
  // it is filled - that is the fallback on the data plane too - so creating one
  // is never creating a broken page.
  //
  // Duplicating copies ONLY what the source actually says. Copying what it
  // RENDERS would copy English into the fourteen keys German has no wording
  // for, stored as though they were German: the copy would declare itself
  // complete while carrying fourteen English sentences, and nothing would ever
  // send anyone to translate them. A hole copied stays a hole, renders English
  // like every other hole, and is counted.
  addLocale(code: string, from: string): void {
    const create = (entries: Record<string, string>) =>
      this.api.saveLocale(code, entries, { full: true }).subscribe({
        next: () => {
          this.reloadLocales();
          this.previewLocale.set(code);
          this.version.update((v) => v + 1);
        },
        error: (e: { error?: { error?: string } }) =>
          this.snack.open(
            e.error?.error ?? $localize`:@@Language_add_failed:The language could not be added.`,
            undefined,
            { duration: 8000 },
          ),
      });
    if (!from) {
      create({});
      return;
    }
    this.api.localeStrings(from).subscribe({
      next: (r) => {
        const entries: Record<string, string> = {};
        for (const s of r.strings) if (s.value) entries[s.key] = s.value;
        create(entries);
      },
      error: () => create({}),
    });
  }

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

const KEYS: CoreKey[] = ['primary', 'secondary', 'tertiary', 'error', 'neutral', 'neutralVariant'];

// Lower case, empty colours dropped: the form the gateway stores, so a theme
// read back compares equal to the one that was saved.
function normalized(c: ThemeColors): ThemeColors {
  const out: ThemeColors = { primary: (c.primary ?? '').toLowerCase() };
  for (const k of KEYS) {
    const v = c[k];
    if (k !== 'primary' && v) out[k] = v.toLowerCase();
  }
  return out;
}
